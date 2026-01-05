package main

import (
	"context"
	"database/sql"
	"fmt"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
)

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
				return &existing, nil
			}

			// Upsert with Spotify data
			musician, err := qtx.UpsertMusician(ctx, database.UpsertMusicianParams{
				Name:     name,
				SortName: sortName,
				Summary: sql.NullString{
					String: fmt.Sprintf("%s is a musician with %d followers on Spotify", artist.Name, artist.Followers.Count),
					Valid:  true,
				},
				SpotifyPopularity: helpers.NullFloat64(float64(artist.Popularity)),
				SpotifyFollowers:  helpers.NullInt64(int64(artist.Followers.Count)),
				SpotifyID:         sql.NullString{String: artist.ID.String(), Valid: true},
			})
			if err != nil {
				return nil, err
			}
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
