package main

import (
	"context"
	"database/sql"
	"fmt"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"strings"

	"github.com/zmb3/spotify/v2"
)

// generateMusicianSummary creates a rich, descriptive summary for a musician
// based on their Spotify data including genres, popularity, and follower count.
func generateMusicianSummary(artist *spotify.FullArtist) string {
	var parts []string

	// Base info - artist name
	parts = append(parts, artist.Name)

	// Genres from Spotify (more accurate than file metadata)
	if len(artist.Genres) > 0 {
		// Limit to first 3 genres
		maxGenres := 3
		if len(artist.Genres) < maxGenres {
			maxGenres = len(artist.Genres)
		}
		genreStr := strings.Join(artist.Genres[:maxGenres], ", ")
		parts = append(parts, fmt.Sprintf("known for %s", genreStr))
	}

	// Popularity tier
	pop := artist.Popularity
	switch {
	case pop >= 80:
		parts = append(parts, "is a globally recognized artist")
	case pop >= 60:
		parts = append(parts, "is a popular artist")
	case pop >= 40:
		parts = append(parts, "has a dedicated following")
	case pop >= 20:
		parts = append(parts, "is an emerging artist")
	default:
		parts = append(parts, "is an independent artist")
	}

	// Follower count with human-readable formatting
	followers := artist.Followers.Count
	switch {
	case followers >= 10_000_000:
		parts = append(parts, fmt.Sprintf("with over %dM followers on Spotify", followers/1_000_000))
	case followers >= 1_000_000:
		parts = append(parts, fmt.Sprintf("with %.1fM followers on Spotify", float64(followers)/1_000_000))
	case followers >= 100_000:
		parts = append(parts, fmt.Sprintf("with %dK followers on Spotify", followers/1_000))
	case followers >= 1_000:
		parts = append(parts, fmt.Sprintf("with %dK followers on Spotify", followers/1_000))
	default:
		parts = append(parts, fmt.Sprintf("with %d followers on Spotify", followers))
	}

	return strings.Join(parts, " ") + "."
}

// getOrCreateMusician looks up or creates a musician in the database.
// If Spotify is configured, attempts to enrich the data with Spotify info.
// Falls back to basic metadata if Spotify lookup fails.
func (app *Application) getOrCreateMusician(ctx context.Context, qtx *database.Queries, name, sortName string) (*database.Musician, error) {
	// Try Spotify lookup first if configured
	if app.Spotify != nil {
		artist, err := app.Spotify.SearchArtistByName(name)
		if err == nil && artist != nil {
			// Check if we already have this Spotify artist
			existing, err := qtx.GetMusicianBySpotifyID(ctx, sql.NullString{String: artist.ID.String(), Valid: true})
			if err == nil {
				// Even if musician exists, process Spotify genres to enrich the data
				app.processSpotifyGenres(ctx, qtx, existing.ID, artist.Genres)
				return &existing, nil
			}

			// Build thumb from Spotify artist images
			var thumb sql.NullString
			if len(artist.Images) > 0 {
				thumb = sql.NullString{String: artist.Images[0].URL, Valid: true}
			}

			// Generate enhanced summary
			summary := generateMusicianSummary(artist)

			// Upsert with Spotify data
			musician, err := qtx.UpsertMusician(ctx, database.UpsertMusicianParams{
				Name:              name,
				SortName:          sortName,
				Summary:           sql.NullString{String: summary, Valid: true},
				SpotifyPopularity: helpers.NullFloat64(float64(artist.Popularity)),
				SpotifyFollowers:  helpers.NullInt64(int64(artist.Followers.Count)),
				SpotifyID:         sql.NullString{String: artist.ID.String(), Valid: true},
				Thumb:             thumb,
			})
			if err != nil {
				return nil, err
			}

			// Process Spotify genres for this musician
			app.processSpotifyGenres(ctx, qtx, musician.ID, artist.Genres)

			return &musician, nil
		}
	}

	// Upsert with basic data only
	musician, err := qtx.UpsertMusician(ctx, database.UpsertMusicianParams{
		Name:     name,
		SortName: sortName,
	})
	if err != nil {
		return nil, err
	}
	return &musician, nil
}

// processSpotifyGenres creates genre entries and musician-genre relationships
// for each genre provided by Spotify's artist data.
func (app *Application) processSpotifyGenres(ctx context.Context, qtx *database.Queries, musicianID int64, spotifyGenres []string) {
	for _, genreTag := range spotifyGenres {
		// Get or create the genre
		genre, err := qtx.GetOrCreateGenre(ctx, database.GetOrCreateGenreParams{
			Tag:       genreTag,
			GenreType: "music",
		})
		if err != nil {
			app.Logger.Warn("failed to get/create Spotify genre",
				"error", err,
				"genre", genreTag,
			)
			continue
		}

		// Create musician-genre relationship
		err = qtx.UpsertMusicianGenre(ctx, database.UpsertMusicianGenreParams{
			MusicianID: musicianID,
			GenreID:    genre.ID,
		})
		if err != nil {
			app.Logger.Warn("failed to create musician-genre relationship for Spotify genre",
				"error", err,
				"musician_id", musicianID,
				"genre_id", genre.ID,
				"genre", genreTag,
			)
		}
	}
}

// getOrCreateAlbum looks up or creates an album in the database.
// If Spotify is configured, attempts to enrich the data with Spotify info.
// Falls back to basic metadata if Spotify lookup fails.
func (app *Application) getOrCreateAlbum(ctx context.Context, qtx *database.Queries, title, sortTitle, albumArtist string) (*database.Album, error) {
	// Try Spotify lookup first if configured
	if app.Spotify != nil {
		albumDetails, err := app.Spotify.SearchAndGetAlbumDetails(title)
		if err == nil && albumDetails != nil {
			// Check if we already have this Spotify album
			existing, err := qtx.GetAlbumBySpotifyID(ctx, sql.NullString{String: albumDetails.ID.String(), Valid: true})
			if err == nil {
				return &existing, nil
			}

			// Build params with Spotify data
			params := database.UpsertAlbumParams{
				Title:             title,
				SortTitle:         sortTitle,
				SpotifyID:         sql.NullString{String: albumDetails.ID.String(), Valid: true},
				SpotifyPopularity: helpers.NullFloat64(float64(albumDetails.Popularity)),
				TotalTracks:       helpers.NullInt64(int64(albumDetails.TotalTracks)),
			}

			// Parse release date
			releaseDate := albumDetails.ReleaseDateTime()
			if !releaseDate.IsZero() {
				params.ReleaseDate = sql.NullString{String: releaseDate.Format("2006-01-02"), Valid: true}
				params.Year = sql.NullInt64{Int64: int64(releaseDate.Year()), Valid: true}
			}

			// Album artist
			if albumArtist != "" {
				params.Musician = sql.NullString{String: albumArtist, Valid: true}
			}

			// Store Spotify cover URL (local download planned for later)
			if len(albumDetails.Images) > 0 {
				params.Cover = sql.NullString{String: albumDetails.Images[0].URL, Valid: true}
			}

			album, err := qtx.UpsertAlbum(ctx, params)
			if err != nil {
				return nil, err
			}
			return &album, nil
		}
		// Spotify failed, continue with basic metadata (silent failure as per design)
	}

	// Upsert with basic data only
	params := database.UpsertAlbumParams{
		Title:     title,
		SortTitle: sortTitle,
	}
	if albumArtist != "" {
		params.Musician = sql.NullString{String: albumArtist, Valid: true}
	}

	album, err := qtx.UpsertAlbum(ctx, params)
	if err != nil {
		return nil, err
	}
	return &album, nil
}
