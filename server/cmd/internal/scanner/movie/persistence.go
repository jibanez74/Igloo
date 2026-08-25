package movie

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/scanner"
	"igloo/cmd/internal/tmdb"
)

func (s *Scanner) persistResolvedMovie(ctx context.Context, scan *movieScanContext, resolved *resolvedMovie) error {
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

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("failed to commit movie: %w", err)
	}

	// Both caches describe the committed row, so they are dropped here rather
	// than inside the transaction: evicting earlier lets a concurrent reader
	// republish the pre-rescan file path after the new one commits.
	if s.invalidateCommittedMovie != nil {
		s.invalidateCommittedMovie(movieID)
	}

	// movieIndex is shared (never written inside the transaction) and is only
	// updated here, after a successful commit, so a movie whose transaction
	// failed is never recorded as scanned/unchanged.
	scan.movieIndex[filepath.Clean(resolved.params.FilePath)] = resolved.params.Size
	scan.mergeFrom(txScan)

	return nil
}

// persistResolvedMovieTx returns the upserted movie ID so the caller can drop
// the caches keyed on it once the transaction commits.
func (s *Scanner) persistResolvedMovieTx(ctx context.Context, qtx *database.Queries, scan *movieScanContext, resolved *resolvedMovie) (int64, error) {
	movie, err := qtx.UpsertMovie(ctx, resolved.params)
	if err != nil {
		return 0, fmt.Errorf("upsert movie failed: %w", err)
	}

	if resolved.tmdbMovie != nil {
		err = qtx.DeleteMovieCast(ctx, movie.ID)
		if err != nil {
			return 0, fmt.Errorf("delete existing cast failed: %w", err)
		}

		err = qtx.DeleteMovieCrew(ctx, movie.ID)
		if err != nil {
			return 0, fmt.Errorf("delete existing crew failed: %w", err)
		}

		err = processProductionCompanies(ctx, qtx, movie.ID, resolved.tmdbMovie.ProductionCompanies)
		if err != nil {
			return 0, fmt.Errorf("process production companies failed: %w", err)
		}

		err = processCast(ctx, qtx, scan, movie.ID, resolved.tmdbMovie.Credits.Cast)
		if err != nil {
			return 0, fmt.Errorf("process cast failed: %w", err)
		}

		err = processCrew(ctx, qtx, scan, movie.ID, resolved.tmdbMovie.Credits.Crew)
		if err != nil {
			return 0, fmt.Errorf("process crew failed: %w", err)
		}

		err = processMovieGenres(ctx, qtx, scan, movie.ID, resolved.tmdbMovie.Genres)
		if err != nil {
			return 0, fmt.Errorf("process genres failed: %w", err)
		}

		err = processExtraVideos(ctx, qtx, movie.ID, resolved.tmdbMovie.Videos.Results)
		if err != nil {
			return 0, fmt.Errorf("process extra videos failed: %w", err)
		}
	}

	videoStreamCount, err := s.processMovieStreams(ctx, qtx, movie.ID, resolved.streams)
	if err != nil {
		return 0, fmt.Errorf("process movie streams failed: %w", err)
	}
	if videoStreamCount == 0 {
		return 0, fmt.Errorf("no video stream found - invalid movie file")
	}

	err = processChapters(ctx, qtx, movie.ID, resolved.chapters)
	if err != nil {
		return 0, fmt.Errorf("process chapters failed: %w", err)
	}

	return movie.ID, nil
}

// ---------------------------------------------------------------------------
// TMDB metadata entities
// ---------------------------------------------------------------------------

func processProductionCompanies(
	ctx context.Context,
	qtx *database.Queries,
	movieID int64,
	companies []struct {
		ID            int    `json:"id"`
		LogoPath      string `json:"logo_path"`
		Name          string `json:"name"`
		OriginCountry string `json:"origin_country"`
	},
) error {
	err := qtx.DeleteMovieProductionCompanies(ctx, movieID)
	if err != nil {
		return fmt.Errorf("delete movie production companies failed: %w", err)
	}

	for _, company := range companies {
		upserted, err := qtx.UpsertProductionCompany(ctx, database.UpsertProductionCompanyParams{
			Name:    company.Name,
			TmdbID:  int64(company.ID),
			Logo:    helpers.NullString(company.LogoPath),
			Country: helpers.NullString(company.OriginCountry),
		})
		if err != nil {
			return fmt.Errorf("upsert production company failed: %w", err)
		}

		err = qtx.CreateMovieProductionCompany(ctx, database.CreateMovieProductionCompanyParams{
			MovieID:             movieID,
			ProductionCompanyID: upserted.ID,
		})
		if err != nil {
			return fmt.Errorf("create movie production company relationship failed: %w", err)
		}
	}

	return nil
}

// ProcessProductionCompanies updates a movie's production-company links.
func ProcessProductionCompanies(ctx context.Context, qtx *database.Queries, movieID int64, companies []struct {
	ID            int    `json:"id"`
	LogoPath      string `json:"logo_path"`
	Name          string `json:"name"`
	OriginCountry string `json:"origin_country"`
}) error {
	return processProductionCompanies(ctx, qtx, movieID, companies)
}

func processCast(
	ctx context.Context,
	qtx *database.Queries,
	scan *movieScanContext,
	movieID int64,
	cast []struct {
		ID          int    `json:"id"`
		Name        string `json:"name"`
		Character   string `json:"character"`
		ProfilePath string `json:"profile_path"`
		Order       int    `json:"order"`
	},
) error {
	for _, castMember := range cast {
		artistID, err := getOrCreateArtistID(ctx, qtx, scan, castMember.ID, castMember.Name, castMember.ProfilePath)
		if err != nil {
			return fmt.Errorf("get or create artist failed: %w", err)
		}

		_, err = qtx.UpsertCast(ctx, database.UpsertCastParams{
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

// ProcessCast updates a movie's cast without scan-local caching.
func ProcessCast(ctx context.Context, qtx *database.Queries, movieID int64, cast []struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Character   string `json:"character"`
	ProfilePath string `json:"profile_path"`
	Order       int    `json:"order"`
}) error {
	return processCast(ctx, qtx, nil, movieID, cast)
}

func processCrew(
	ctx context.Context,
	qtx *database.Queries,
	scan *movieScanContext,
	movieID int64,
	crew []struct {
		ID          int    `json:"id"`
		Name        string `json:"name"`
		Job         string `json:"job"`
		Department  string `json:"department"`
		ProfilePath string `json:"profile_path"`
	},
) error {
	for _, crewMember := range crew {
		artistID, err := getOrCreateArtistID(ctx, qtx, scan, crewMember.ID, crewMember.Name, crewMember.ProfilePath)
		if err != nil {
			return fmt.Errorf("get or create artist failed: %w", err)
		}

		_, err = qtx.UpsertCrew(ctx, database.UpsertCrewParams{
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

// ProcessCrew updates a movie's crew without scan-local caching.
func ProcessCrew(ctx context.Context, qtx *database.Queries, movieID int64, crew []struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Job         string `json:"job"`
	Department  string `json:"department"`
	ProfilePath string `json:"profile_path"`
}) error {
	return processCrew(ctx, qtx, nil, movieID, crew)
}

func getOrCreateArtist(
	ctx context.Context,
	qtx *database.Queries,
	tmdbID int,
	name string,
	profilePath string,
) (*database.Artist, error) {
	upserted, err := qtx.UpsertArtist(ctx, database.UpsertArtistParams{
		Name:    name,
		TmdbID:  int64(tmdbID),
		Profile: helpers.NullString(profilePath),
	})
	if err != nil {
		return nil, fmt.Errorf("upsert artist failed: %w", err)
	}

	return &upserted, nil
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
		if artistID, ok := scan.artistIDs[int64(tmdbID)]; ok {
			return artistID, nil
		}
	}

	artist, err := getOrCreateArtist(ctx, qtx, tmdbID, name, profilePath)
	if err != nil {
		return 0, err
	}

	if scan != nil {
		scan.artistIDs[int64(tmdbID)] = artist.ID
	}
	return artist.ID, nil
}

func processMovieGenres(
	ctx context.Context,
	qtx *database.Queries,
	scan *movieScanContext,
	movieID int64,
	genres []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	},
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

// ProcessMovieGenres updates a movie's genre links without scan-local caching.
func ProcessMovieGenres(ctx context.Context, qtx *database.Queries, movieID int64, genres []struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}) error {
	return processMovieGenres(ctx, qtx, nil, movieID, genres)
}

func getOrCreateMovieGenreID(ctx context.Context, qtx *database.Queries, scan *movieScanContext, tag string) (int64, error) {
	cacheKey := scanner.NormalizedScanCacheKey(tag, "movie")
	if scan != nil {
		if genreID, ok := scan.genreIDs[cacheKey]; ok {
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
		scan.genreIDs[cacheKey] = dbGenre.ID
	}
	return dbGenre.ID, nil
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
			Official:   v.Official,
		})
		if err != nil {
			return fmt.Errorf("upsert extra video failed: %w", err)
		}

		err = qtx.CreateMovieExtraVideo(ctx, database.CreateMovieExtraVideoParams{
			MovieID:      movieID,
			ExtraVideoID: extra.ID,
		})

		if err != nil {
			return fmt.Errorf("create movie extra video link failed: %w", err)
		}
	}

	return nil
}

// ProcessExtraVideos updates a movie's extra-video links.
func ProcessExtraVideos(ctx context.Context, qtx *database.Queries, movieID int64, results []tmdb.TmdbVideoResult) error {
	return processExtraVideos(ctx, qtx, movieID, results)
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

// ---------------------------------------------------------------------------
// Streams and chapters
// ---------------------------------------------------------------------------
