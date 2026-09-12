package movie

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/scanner"
	"igloo/cmd/internal/scanner/tmdbmatch"
	"igloo/cmd/internal/tmdb"
)

// errStaleEnrichment aborts an enrichment transaction when the catalog row,
// its baseline, or its retry state moved on since the lookup started.
var errStaleEnrichment = errors.New("stale enrichment")

type enrichmentOutcome uint8

const (
	enrichmentSkipped enrichmentOutcome = iota
	enrichmentApplied
	enrichmentUnmatched
)

// persistLocalMovie commits technical metadata, streams, chapters, the file
// baseline, and pending-enrichment state in one transaction.
func (s *Scanner) persistLocalMovie(ctx context.Context, scan *movieScanContext, movie *localMovie) error {
	defer s.logSlow("slow movie persistence", time.Now(), "path", movie.params.FilePath)
	if movie.inspection == nil {
		return fmt.Errorf("missing file inspection")
	}
	txScan := scan.clone()
	var movieID int64
	var tmdbID sql.NullInt64
	var pending bool
	err := s.tx.Run(ctx, func(qtx *database.Queries) error {
		current, err := qtx.GetMovieByPath(ctx, movie.params.FilePath)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		// A deleted or replaced catalog row must never be recreated by in-flight work.
		replaced := current.ID != movie.baseline.ID || current.FilePath != movie.baseline.FilePath
		if replaced {
			return &scanner.FileDeferral{Reason: scanner.FileChanged}
		}
		tmdbID = current.TmdbID
		movieID, err = qtx.UpsertMovie(ctx, movie.params)
		if err != nil {
			return fmt.Errorf("upsert movie failed: %w", err)
		}
		// A confirmed TMDB match survives a technical change. Re-queueing it would
		// re-run applyTmdbMetadata, which overwrites every descriptive field --
		// including anything set through the Edit dialog -- and a new mtime says
		// nothing about which movie the file is. Only an unmatched movie is queued.
		unmatched := !current.TmdbID.Valid
		if unmatched {
			err = qtx.MarkMovieTmdbRetry(ctx, movieID)
			if err != nil {
				return err
			}
		}
		videoStreamCount, err := processMovieStreams(ctx, qtx, movieID, movie.streams)
		if err != nil {
			return fmt.Errorf("process movie streams failed: %w", err)
		}
		if videoStreamCount == 0 {
			return fmt.Errorf("no video stream found - invalid movie file")
		}
		err = processChapters(ctx, qtx, movieID, movie.chapters)
		if err != nil {
			return fmt.Errorf("process chapters failed: %w", err)
		}
		// Technical changes invalidate persisted playback work in the same
		// transaction. Keeping the rows on a metadata-only change would not help:
		// their readers key on movieStreamFingerprintBase, which includes
		// movies.updated_at, so an UpsertMovie here already invalidates them.
		err = qtx.DeleteMovieRemuxSafetyVerdicts(ctx, movieID)
		if err != nil {
			return err
		}
		err = qtx.DeleteMovieKeyframeIndexes(ctx, movieID)
		if err != nil {
			return err
		}
		err = storeMovieFingerprint(ctx, qtx, movie.params.FilePath, movie.inspection.Fingerprint)
		if err != nil {
			return err
		}
		pending, err = qtx.HasMovieTmdbRetry(ctx, movieID)
		if err != nil {
			return err
		}
		err = ctx.Err()
		if err != nil {
			return err
		}
		return movie.inspection.Validate(ctx)
	}, func() {
		// The runtime cache describes the committed row, so it is dropped after
		// commit: evicting earlier lets a concurrent reader republish the
		// pre-rescan file path after the new one commits.
		s.invalidateCommittedMovie(movieID)
	})
	if err != nil {
		return err
	}

	// movieIndex is shared (never written inside the transaction) and is only
	// updated here, after a successful commit, so a movie whose transaction
	// failed is never recorded as scanned/unchanged.
	scan.setEntry(filepath.Clean(movie.params.FilePath), movieScanEntry{
		FileFingerprint: movie.inspection.Fingerprint, ID: movieID, FilePath: movie.params.FilePath,
		TmdbID: tmdbID, PendingRetry: pending, HasFingerprint: true,
	})
	scan.mergeFrom(txScan)
	return nil
}

// persistEnrichment applies a TMDB match, or records a definitive miss, for a
// movie whose file and catalog identity are unchanged since the lookup.
// Descriptive updates never invalidate playback data.
func (s *Scanner) persistEnrichment(ctx context.Context, scan *movieScanContext, movie *enrichedMovie) (enrichmentOutcome, error) {
	path := movie.baseline.FilePath
	defer s.logSlow("slow movie enrichment persistence", time.Now(), "path", path)
	txScan := scan.clone()
	attemptedAt := s.now()
	err := s.tx.Run(ctx, func(qtx *database.Queries) error {
		current, err := qtx.GetMovieByPath(ctx, path)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		stale := current.ID != movie.baseline.ID || current.FilePath != movie.baseline.FilePath || current.TmdbID != movie.baseline.TmdbID
		if stale {
			return errStaleEnrichment
		}
		stored, err := qtx.GetMovieFileFingerprint(ctx, current.ID)
		if err != nil {
			return err
		}
		baseline := scanner.FileFingerprint{Size: stored.Size, MtimeNS: stored.MtimeNs, CtimeNS: stored.CtimeNs, Device: stored.Device, Inode: stored.Inode}
		if baseline != movie.baseline.FileFingerprint {
			return errStaleEnrichment
		}
		pending, err := qtx.HasMovieTmdbRetry(ctx, current.ID)
		if err != nil {
			return err
		}
		clearedMeanwhile := movie.baseline.PendingRetry && !pending
		if clearedMeanwhile {
			return errStaleEnrichment
		}
		if movie.tmdbMovie != nil {
			err = applyTmdbMetadata(ctx, qtx, txScan, current.ID, movie.tmdbMovie)
		} else {
			err = qtx.RecordMovieTmdbMiss(ctx, database.RecordMovieTmdbMissParams{MovieID: current.ID, LastAttemptAt: helpers.NullInt64(attemptedAt.Unix())})
		}
		if err != nil {
			return err
		}
		err = ctx.Err()
		if err != nil {
			return err
		}
		return movie.inspection.Validate(ctx)
	}, nil)
	stale := errors.Is(err, errStaleEnrichment)
	if stale {
		return enrichmentSkipped, nil
	}
	if err != nil {
		return enrichmentSkipped, err
	}

	entry := movie.baseline
	outcome := enrichmentUnmatched
	if movie.tmdbMovie != nil {
		outcome = enrichmentApplied
		entry.TmdbID = helpers.NullInt64(int64(movie.tmdbMovie.TmdbID))
		entry.PendingRetry, entry.RetryAttempts, entry.LastAttemptAt = false, 0, sql.NullInt64{}
		scan.enriched++
	} else {
		entry.PendingRetry, entry.RetryAttempts, entry.LastAttemptAt = true, entry.RetryAttempts+1, helpers.NullInt64(attemptedAt.Unix())
	}
	scan.setEntry(filepath.Clean(path), entry)
	scan.mergeFrom(txScan)
	return outcome, nil
}

// ---------------------------------------------------------------------------
// TMDB metadata entities
// ---------------------------------------------------------------------------

// ApplyTmdbMetadata updates descriptions, clears retries, and replaces relationships owned by a
// TMDB match -- production companies, cast, crew, genres and extra videos --
// inside qtx's transaction. It is the single definition of "what TMDB owns on
// a movie", shared by the library scan and the manual identify flow, so the
// two cannot drift.
func ApplyTmdbMetadata(ctx context.Context, qtx *database.Queries, movieID int64, tmdbMovie *tmdb.TmdbMovie) error {
	return applyTmdbMetadata(ctx, qtx, nil, movieID, tmdbMovie)
}

// applyTmdbMetadata is the scan-aware form: scan memoizes genre and artist ids
// across a library scan and is nil on the manual identify path.
func applyTmdbMetadata(
	ctx context.Context,
	qtx *database.Queries,
	scan *movieScanContext,
	movieID int64,
	tmdbMovie *tmdb.TmdbMovie,
) error {
	err := qtx.UpdateMovieTmdbMetadata(ctx, database.UpdateMovieTmdbMetadataParams{
		ID: movieID, Title: tmdbMovie.Title, TmdbID: helpers.NullInt64(int64(tmdbMovie.TmdbID)),
		ImdbID: helpers.NullString(tmdbMovie.ImdbID), PosterPath: helpers.NullString(tmdbMovie.PosterPath),
		BackdropPath: helpers.NullString(tmdbMovie.BackdropPath), Adult: tmdbMovie.Adult,
		Language: helpers.NullString(tmdbMovie.OriginalLang), Year: helpers.NullInt64(int64(tmdbmatch.ReleaseYear(tmdbMovie.ReleaseDate))),
		ReleaseDate: helpers.NullString(tmdbMovie.ReleaseDate), Overview: helpers.NullString(tmdbMovie.Overview),
		TagLine: helpers.NullString(tmdbMovie.Tagline), Certification: helpers.NullString(tmdbMovie.Certification()),
		CriticRating: helpers.NullFloat64(tmdbMovie.VoteAverage), Revenue: helpers.NullFloat64(float64(tmdbMovie.Revenue)),
		Budget: helpers.NullFloat64(float64(tmdbMovie.Budget)),
	})
	if err != nil {
		return fmt.Errorf("update TMDB metadata failed: %w", err)
	}
	err = qtx.DeleteMovieCast(ctx, movieID)
	if err != nil {
		return fmt.Errorf("delete existing cast failed: %w", err)
	}

	err = qtx.DeleteMovieCrew(ctx, movieID)
	if err != nil {
		return fmt.Errorf("delete existing crew failed: %w", err)
	}

	err = processProductionCompanies(ctx, qtx, movieID, tmdbMovie.ProductionCompanies)
	if err != nil {
		return fmt.Errorf("process production companies failed: %w", err)
	}

	err = processCast(ctx, qtx, scan, movieID, tmdbMovie.Credits.Cast)
	if err != nil {
		return fmt.Errorf("process cast failed: %w", err)
	}

	err = processCrew(ctx, qtx, scan, movieID, tmdbMovie.Credits.Crew)
	if err != nil {
		return fmt.Errorf("process crew failed: %w", err)
	}

	err = processMovieGenres(ctx, qtx, scan, movieID, tmdbMovie.Genres)
	if err != nil {
		return fmt.Errorf("process genres failed: %w", err)
	}

	err = processExtraVideos(ctx, qtx, movieID, tmdbMovie.Videos.Results)
	if err != nil {
		return fmt.Errorf("process extra videos failed: %w", err)
	}

	return qtx.ClearMovieTmdbRetry(ctx, movieID)
}

func processProductionCompanies(
	ctx context.Context,
	qtx *database.Queries,
	movieID int64,
	companies []tmdb.ProductionCompany,
) error {
	err := qtx.DeleteMovieProductionCompanies(ctx, movieID)
	if err != nil {
		return fmt.Errorf("delete movie production companies failed: %w", err)
	}

	for _, company := range companies {
		upserted, err := qtx.UpsertProductionCompany(ctx, database.UpsertProductionCompanyParams{
			Name:   company.Name,
			TmdbID: int64(company.ID),
		})
		if err != nil {
			return fmt.Errorf("upsert production company failed: %w", err)
		}

		err = qtx.CreateMovieProductionCompany(ctx, database.CreateMovieProductionCompanyParams{
			MovieID:             movieID,
			ProductionCompanyID: upserted,
		})
		if err != nil {
			return fmt.Errorf("create movie production company relationship failed: %w", err)
		}
	}

	return nil
}

func processCast(
	ctx context.Context,
	qtx *database.Queries,
	scan *movieScanContext,
	movieID int64,
	cast []tmdb.CastCredit,
) error {
	for _, castMember := range cast {
		artistID, err := getOrCreateArtistID(ctx, qtx, scan, castMember.ID, castMember.Name, castMember.ProfilePath)
		if err != nil {
			return fmt.Errorf("get or create artist failed: %w", err)
		}

		err = qtx.UpsertCast(ctx, database.UpsertCastParams{
			MovieID:   movieID,
			ArtistID:  artistID,
			Character: castMember.Character,
			CastOrder: int64(castMember.Order),
		})

		if err != nil {
			return fmt.Errorf("upsert cast failed: %w", err)
		}
	}

	return nil
}

func processCrew(
	ctx context.Context,
	qtx *database.Queries,
	scan *movieScanContext,
	movieID int64,
	crew []tmdb.CrewCredit,
) error {
	for _, crewMember := range crew {
		artistID, err := getOrCreateArtistID(ctx, qtx, scan, crewMember.ID, crewMember.Name, crewMember.ProfilePath)
		if err != nil {
			return fmt.Errorf("get or create artist failed: %w", err)
		}

		err = qtx.UpsertCrew(ctx, database.UpsertCrewParams{
			MovieID:    movieID,
			ArtistID:   artistID,
			Job:        crewMember.Job,
			Department: crewMember.Department,
		})

		if err != nil {
			return fmt.Errorf("upsert crew failed: %w", err)
		}
	}

	return nil
}

// getOrCreateArtistID upserts a TMDB person once per scan: the same
// person credited across many movies (or several crew roles of one movie) hits
// the database once per scan. The first sighting still runs the full upsert,
// so name/profile refresh from TMDB once per scan instead of once per credit.
// Nil-tolerant on scan, like getOrCreateMovieGenreID.
func getOrCreateArtistID(
	ctx context.Context,
	qtx *database.Queries,
	scan *movieScanContext,
	tmdbID int,
	name string,
	profilePath string,
) (int64, error) {
	if scan != nil {
		artistID, ok := scan.artistIDs.Get(int64(tmdbID))
		if ok {
			return artistID, nil
		}
	}

	artist, err := qtx.UpsertArtist(ctx, database.UpsertArtistParams{
		Name:    name,
		TmdbID:  int64(tmdbID),
		Profile: helpers.NullString(profilePath),
	})
	if err != nil {
		return 0, fmt.Errorf("upsert artist failed: %w", err)
	}

	if scan != nil {
		scan.artistIDs.Set(int64(tmdbID), artist)
	}
	return artist, nil
}

func processMovieGenres(
	ctx context.Context,
	qtx *database.Queries,
	scan *movieScanContext,
	movieID int64,
	genres []tmdb.Genre,
) error {
	err := qtx.DeleteMovieGenres(ctx, movieID)
	if err != nil {
		return fmt.Errorf("delete movie genres failed: %w", err)
	}

	for _, genre := range genres {
		genreID, err := getOrCreateMovieGenreID(ctx, qtx, scan, genre.Name)
		if err != nil {
			return fmt.Errorf("get or create genre failed: %w", err)
		}

		err = qtx.CreateMovieGenre(ctx, database.CreateMovieGenreParams{
			MovieID: movieID,
			GenreID: genreID,
		})

		if err != nil {
			return fmt.Errorf("create movie genre relationship failed: %w", err)
		}
	}

	return nil
}

func getOrCreateMovieGenreID(ctx context.Context, qtx *database.Queries, scan *movieScanContext, tag string) (int64, error) {
	cacheKey := scanner.NormalizedScanCacheKey(tag, "movie")
	if scan != nil {
		genreID, ok := scan.genreIDs.Get(cacheKey)
		if ok {
			return genreID, nil
		}
	}

	dbGenre, err := qtx.GetOrCreateGenre(ctx, database.GetOrCreateGenreParams{
		Tag:       tag,
		GenreType: "movie",
	})
	if err != nil {
		return 0, err
	}

	if scan != nil {
		scan.genreIDs.Set(cacheKey, dbGenre)
	}
	return dbGenre, nil
}

func processExtraVideos(
	ctx context.Context,
	qtx *database.Queries,
	movieID int64,
	results []tmdb.TmdbVideoResult,
) error {
	err := qtx.DeleteMovieExtraVideos(ctx, movieID)
	if err != nil {
		return fmt.Errorf("delete movie extra videos failed: %w", err)
	}

	for _, v := range results {
		if v.Key == "" || v.ID == "" {
			continue
		}

		title := strings.TrimSpace(v.Name)
		if title == "" {
			title = v.Key
		}

		extra, err := qtx.UpsertExtraVideo(ctx, database.UpsertExtraVideoParams{
			Title:      title,
			ExternalID: helpers.NullString(v.ID),
			Key:        v.Key,
			Type:       v.ExtraVideoType(),
			Site:       v.ExtraVideoSite(),
		})
		if err != nil {
			return fmt.Errorf("upsert extra video failed: %w", err)
		}

		err = qtx.CreateMovieExtraVideo(ctx, database.CreateMovieExtraVideoParams{
			MovieID:      movieID,
			ExtraVideoID: extra,
		})

		if err != nil {
			return fmt.Errorf("create movie extra video link failed: %w", err)
		}
	}

	return nil
}
