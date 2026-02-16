package main

import (
	"context"
	"fmt"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/tmdb"
	"strings"
)

// processProductionCompanies processes production companies from TMDB data.
func (app *Application) processProductionCompanies(
	ctx context.Context,
	qtx *database.Queries,
	movieID int64,
	companies []struct {
		ID            int    `json:"id"`
		LogoPath      string `json:"logo_path"`
		Name          string `json:"name"`
		OriginCountry string `json:"origin_country"`
	},
	cache *movieScannerCache,
) error {
	// Delete all existing production company links
	if err := qtx.DeleteMovieProductionCompanies(ctx, movieID); err != nil {
		return fmt.Errorf("delete movie production companies failed: %w", err)
	}

	for _, company := range companies {
		// Check cache first
		var dbCompany *database.ProductionCompany
		cached, ok := cache.GetProductionCompany(company.ID)
		if ok {
			dbCompany = cached
		} else {
			// Build logo URL if available
			logoURL := buildTmdbImageURL(company.LogoPath)

			// Upsert production company
			upserted, err := qtx.UpsertProductionCompany(ctx, database.UpsertProductionCompanyParams{
				Name:    company.Name,
				TmdbID:  int64(company.ID),
				Logo:    logoURL,
				Country: helpers.NullString(company.OriginCountry),
			})
			if err != nil {
				return fmt.Errorf("upsert production company failed: %w", err)
			}

			dbCompany = &upserted
			// Cache for reuse
			cache.SetProductionCompany(company.ID, dbCompany)
		}

		// Create movie-production company relationship
		if err := qtx.CreateMovieProductionCompany(ctx, database.CreateMovieProductionCompanyParams{
			MovieID:             movieID,
			ProductionCompanyID: dbCompany.ID,
		}); err != nil {
			return fmt.Errorf("create movie production company relationship failed: %w", err)
		}
	}

	return nil
}

// processCast processes cast members from TMDB data.
func (app *Application) processCast(
	ctx context.Context,
	qtx *database.Queries,
	movieID int64,
	cast []struct {
		ID          int    `json:"id"`
		Name        string `json:"name"`
		Character   string `json:"character"`
		ProfilePath string `json:"profile_path"`
		Order       int    `json:"order"`
	},
	cache *movieScannerCache,
) error {
	for _, castMember := range cast {
		// Get or create artist from cache
		artist, err := app.getOrCreateArtistFromCache(ctx, qtx, castMember.ID, castMember.Name, castMember.ProfilePath, cache)
		if err != nil {
			return fmt.Errorf("get or create artist failed: %w", err)
		}

		// Upsert cast record
		if _, err := qtx.UpsertCast(ctx, database.UpsertCastParams{
			MovieID:   movieID,
			ArtistID:  artist.ID,
			Character: castMember.Character,
			CastOrder: int64(castMember.Order),
		}); err != nil {
			return fmt.Errorf("upsert cast failed: %w", err)
		}
	}

	return nil
}

// processCrew processes crew members from TMDB data.
func (app *Application) processCrew(
	ctx context.Context,
	qtx *database.Queries,
	movieID int64,
	crew []struct {
		ID          int    `json:"id"`
		Name        string `json:"name"`
		Job         string `json:"job"`
		Department  string `json:"department"`
		ProfilePath string `json:"profile_path"`
	},
	cache *movieScannerCache,
) error {
	for _, crewMember := range crew {
		// Get or create artist from cache
		artist, err := app.getOrCreateArtistFromCache(ctx, qtx, crewMember.ID, crewMember.Name, crewMember.ProfilePath, cache)
		if err != nil {
			return fmt.Errorf("get or create artist failed: %w", err)
		}

		// Upsert crew record
		if _, err := qtx.UpsertCrew(ctx, database.UpsertCrewParams{
			MovieID:    movieID,
			ArtistID:   artist.ID,
			Job:        crewMember.Job,
			Department: crewMember.Department,
		}); err != nil {
			return fmt.Errorf("upsert crew failed: %w", err)
		}
	}

	return nil
}

// mapTmdbVideoType maps TMDB video type to extra_videos.type ('trailer', 'special_feature', 'other').
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

// mapTmdbVideoSite maps TMDB site to extra_videos.site ('youtube', 'vimeo', 'other').
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

// processExtraVideos processes trailers and special features from TMDB Videos.Results.
// Deletes existing movie–extra links, then upserts each video and links it to the movie.
func (app *Application) processExtraVideos(
	ctx context.Context,
	qtx *database.Queries,
	movieID int64,
	results []tmdb.TmdbVideoResult,
) error {
	if err := qtx.DeleteMovieExtraVideos(ctx, movieID); err != nil {
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
		if err := qtx.CreateMovieExtraVideo(ctx, database.CreateMovieExtraVideoParams{
			MovieID:      movieID,
			ExtraVideoID: extra.ID,
		}); err != nil {
			return fmt.Errorf("create movie extra video link failed: %w", err)
		}
	}

	return nil
}

// processMovieGenres processes genres from TMDB data.
func (app *Application) processMovieGenres(
	ctx context.Context,
	qtx *database.Queries,
	movieID int64,
	genres []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	},
) error {
	// Delete all existing genre links
	if err := qtx.DeleteMovieGenres(ctx, movieID); err != nil {
		return fmt.Errorf("delete movie genres failed: %w", err)
	}

	for _, genre := range genres {
		// Get or create genre with type "movie"
		dbGenre, err := qtx.GetOrCreateGenre(ctx, database.GetOrCreateGenreParams{
			Tag:       genre.Name,
			GenreType: "movie",
		})
		if err != nil {
			return fmt.Errorf("get or create genre failed: %w", err)
		}

		// Create movie-genre relationship
		if err := qtx.CreateMovieGenre(ctx, database.CreateMovieGenreParams{
			MovieID: movieID,
			GenreID: dbGenre.ID,
		}); err != nil {
			return fmt.Errorf("create movie genre relationship failed: %w", err)
		}
	}

	return nil
}
