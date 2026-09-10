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
	"igloo/cmd/internal/tmdb"
)

func (s *Scanner) persistResolvedMovie(ctx context.Context, scan *movieScanContext, resolved *resolvedMovie) error {
	started := time.Now()
	defer func() {
		elapsed := time.Since(started)
		if elapsed >= 5*time.Second {
			s.logger.Info("slow movie persistence", "path", resolved.params.FilePath, "metadata_only", resolved.metadataOnly, "elapsed", elapsed)
		}
	}()
	if resolved.inspection == nil {
		return fmt.Errorf("missing file inspection")
	}
	txScan := scan.clone()

	s.scannerDBMu.Lock()
	defer s.scannerDBMu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	qtx := s.queries.WithTx(tx)
	movieID, err := s.persistResolvedMovieTx(ctx, qtx, txScan, resolved)
	if err != nil {
		return err
	}

	if movieID == 0 {
		return nil
	}
	if !resolved.metadataOnly {
		// Technical changes invalidate persisted playback work in the same transaction.
		err = qtx.DeleteMovieRemuxSafetyVerdicts(ctx, movieID)
		if err != nil {
			return err
		}
		err = qtx.DeleteMovieKeyframeIndexes(ctx, movieID)
		if err != nil {
			return err
		}

		err = storeMovieFingerprint(ctx, qtx, resolved.params.FilePath, resolved.inspection.Fingerprint)
		if err != nil {
			return err
		}
	}

	committed, err := qtx.GetMovieByPath(ctx, resolved.params.FilePath)
	if err != nil {
		return err
	}
	pending, err := qtx.HasMovieTmdbRetry(ctx, movieID)
	if err != nil {
		return err
	}
	err = ctx.Err()
	if err != nil {
		return err
	}
	err = resolved.inspection.Validate(ctx)
	if err != nil {
		return err
	}
	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("failed to commit movie: %w", err)
	}

	resolved.applied = true

	// Both caches describe the committed row, so they are dropped here rather
	// than inside the transaction: evicting earlier lets a concurrent reader
	// republish the pre-rescan file path after the new one commits.
	if !resolved.metadataOnly {
		s.invalidateCommittedMovie(movieID)
	}

	// movieIndex is shared (never written inside the transaction) and is only
	// updated here, after a successful commit, so a movie whose transaction
	// failed is never recorded as scanned/unchanged.
	scan.movieIndex[filepath.Clean(resolved.params.FilePath)] = movieScanEntry{
		FileFingerprint: resolved.inspection.Fingerprint, ID: committed.ID, FilePath: committed.FilePath,
		TmdbID: committed.TmdbID, PendingRetry: pending, HasFingerprint: true,
	}
	if resolved.metadataOnly && resolved.tmdbMovie != nil && committed.TmdbID == helpers.NullInt64(int64(resolved.tmdbMovie.TmdbID)) {
		scan.enriched++
	}

	scan.mergeFrom(txScan)

	return nil
}

// persistResolvedMovieTx returns the upserted movie ID so the caller can drop
// the caches keyed on it once the transaction commits.
func (s *Scanner) persistResolvedMovieTx(ctx context.Context, qtx *database.Queries, scan *movieScanContext, resolved *resolvedMovie) (int64, error) {
	current, err := qtx.GetMovieByPath(ctx, resolved.params.FilePath)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	// A deleted or replaced catalog row must never be recreated by in-flight work.
	if current.ID != resolved.observed.ID || current.FilePath != resolved.observed.FilePath {
		if !resolved.metadataOnly {
			return 0, &scanner.FileDeferral{Reason: scanner.FileChanged}
		}
		return 0, nil
	}
	sameIdentity := current.TmdbID == resolved.observed.TmdbID
	if resolved.metadataOnly {
		if !sameIdentity {
			return 0, nil
		}
		baseline, err := qtx.GetMovieFileFingerprint(ctx, current.ID)
		if err != nil {
			return 0, err
		}
		stored := scanner.FileFingerprint{Size: baseline.Size, MtimeNS: baseline.MtimeNs, CtimeNS: baseline.CtimeNs, Device: baseline.Device, Inode: baseline.Inode}
		if stored != resolved.baseline {
			return 0, nil
		}
		pending, err := qtx.HasMovieTmdbRetry(ctx, current.ID)
		if err != nil {
			return 0, err
		}
		if resolved.pending && !pending {
			return 0, nil
		}
	}
	movieID := current.ID
	if !resolved.metadataOnly {
		movieID, err = qtx.UpsertMovie(ctx, resolved.params)
		if err != nil {
			return 0, fmt.Errorf("upsert movie failed: %w", err)
		}
	}
	if !resolved.metadataOnly && sameIdentity {
		err = qtx.MarkMovieTmdbRetry(ctx, movieID)
		if err != nil {
			return 0, err
		}
	}
	if resolved.metadataOnly && sameIdentity {
		if resolved.tmdbMovie != nil {
			err = applyTmdbMetadata(ctx, qtx, scan, movieID, resolved.tmdbMovie)
		} else {
			err = qtx.MarkMovieTmdbRetry(ctx, movieID)
		}
		if err != nil {
			return 0, err
		}
	}
	if resolved.metadataOnly {
		return movieID, nil
	}

	videoStreamCount, err := s.processMovieStreams(ctx, qtx, movieID, resolved.streams)
	if err != nil {
		return 0, fmt.Errorf("process movie streams failed: %w", err)
	}
	if videoStreamCount == 0 {
		return 0, fmt.Errorf("no video stream found - invalid movie file")
	}

	err = processChapters(ctx, qtx, movieID, resolved.chapters)
	if err != nil {
		return 0, fmt.Errorf("process chapters failed: %w", err)
	}

	return movieID, nil
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
		Language: helpers.NullString(tmdbMovie.OriginalLang), Year: helpers.NullInt64(int64(extractYearFromReleaseDate(tmdbMovie.ReleaseDate))),
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

func getOrCreateArtist(
	ctx context.Context,
	qtx *database.Queries,
	tmdbID int,
	name string,
	profilePath string,
) (int64, error) {
	upserted, err := qtx.UpsertArtist(ctx, database.UpsertArtistParams{
		Name:    name,
		TmdbID:  int64(tmdbID),
		Profile: helpers.NullString(profilePath),
	})
	if err != nil {
		return 0, fmt.Errorf("upsert artist failed: %w", err)
	}

	return upserted, nil
}

// getOrCreateArtistID is the scan-cached form of getOrCreateArtist: the same
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

	artist, err := getOrCreateArtist(ctx, qtx, tmdbID, name, profilePath)
	if err != nil {
		return 0, err
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
			Type:       mapTmdbVideoType(v.Type),
			Site:       mapTmdbVideoSite(v.Site),
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

func mapTmdbVideoType(t string) string {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "trailer", "teaser":
		return "trailer"
	case "featurette", "behind the scenes", "clip", "bloopers", "interview":
		return "special_feature"
	default:
		return "other"
	}
}

func mapTmdbVideoSite(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "youtube":
		return "youtube"
	case "vimeo":
		return "vimeo"
	default:
		return "other"
	}
}
