package main

import (
	"context"
	"fmt"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/tmdb"
	"strings"
)

func (app *Application) processProductionCompanies(
	ctx context.Context,
	qtx *database.Queries,
	scan *movieScanContext,
	movieID int64,
	companies []struct {
		ID            int    `json:"id"`
		LogoPath      string `json:"logo_path"`
		Name          string `json:"name"`
		OriginCountry string `json:"origin_country"`
	},
) error {
	if err := qtx.DeleteMovieProductionCompanies(ctx, movieID); err != nil {
		return fmt.Errorf("delete movie production companies failed: %w", err)
	}

	for _, company := range companies {
		companyID := int64(0)
		if scan != nil {
			companyID = scan.productionCompanyIDs[company.ID]
		}

		if companyID == 0 {
			upserted, err := qtx.UpsertProductionCompany(ctx, database.UpsertProductionCompanyParams{
				Name:    company.Name,
				TmdbID:  int64(company.ID),
				Logo:    helpers.NullString(company.LogoPath),
				Country: helpers.NullString(company.OriginCountry),
			})
			if err != nil {
				return fmt.Errorf("upsert production company failed: %w", err)
			}
			companyID = upserted.ID
			if scan != nil {
				scan.productionCompanyIDs[company.ID] = companyID
			}
		}

		err := qtx.CreateMovieProductionCompany(ctx, database.CreateMovieProductionCompanyParams{
			MovieID:             movieID,
			ProductionCompanyID: companyID,
		})
		if err != nil {
			return fmt.Errorf("create movie production company relationship failed: %w", err)
		}
	}

	return nil
}

func (app *Application) processCast(
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
		artist, err := app.getOrCreateArtist(ctx, qtx, scan, castMember.ID, castMember.Name, castMember.ProfilePath)
		if err != nil {
			return fmt.Errorf("get or create artist failed: %w", err)
		}

		_, err = qtx.UpsertCast(ctx, database.UpsertCastParams{
			MovieID:   movieID,
			ArtistID:  artist.ID,
			Character: castMember.Character,
			CastOrder: int64(castMember.Order),
		})

		if err != nil {
			return fmt.Errorf("upsert cast failed: %w", err)
		}
	}

	return nil
}

func (app *Application) processCrew(
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
		artist, err := app.getOrCreateArtist(ctx, qtx, scan, crewMember.ID, crewMember.Name, crewMember.ProfilePath)
		if err != nil {
			return fmt.Errorf("get or create artist failed: %w", err)
		}

		_, err = qtx.UpsertCrew(ctx, database.UpsertCrewParams{
			MovieID:    movieID,
			ArtistID:   artist.ID,
			Job:        crewMember.Job,
			Department: crewMember.Department,
		})

		if err != nil {
			return fmt.Errorf("upsert crew failed: %w", err)
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

func (app *Application) processExtraVideos(
	ctx context.Context,
	qtx *database.Queries,
	scan *movieScanContext,
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

		extraID := int64(0)
		if scan != nil {
			extraID = scan.extraVideoIDs[v.ID]
		}

		if extraID == 0 {
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
			extraID = extra.ID
			if scan != nil {
				scan.extraVideoIDs[v.ID] = extraID
			}
		}

		err = qtx.CreateMovieExtraVideo(ctx, database.CreateMovieExtraVideoParams{
			MovieID:      movieID,
			ExtraVideoID: extraID,
		})

		if err != nil {
			return fmt.Errorf("create movie extra video link failed: %w", err)
		}
	}

	return nil
}

func (app *Application) processMovieGenres(
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
		genreID, err := app.getOrCreateMovieGenreID(ctx, qtx, scan, genre.Name)
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

func (app *Application) getOrCreateMovieGenreID(ctx context.Context, qtx *database.Queries, scan *movieScanContext, tag string) (int64, error) {
	cacheKey := normalizedMovieCacheKey(tag, "movie")
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
