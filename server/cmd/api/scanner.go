package main

import (
	"context"
	"fmt"
	"igloo/cmd/internal/database"
	"log"
	"os"
	"path/filepath"

	"github.com/jackc/pgx/v5/pgtype"
)

// Helper function to ensure spotify image exists with correct relations
func (app *Application) ensureSpotifyImage(ctx context.Context, path string, width, height int32, musicianID, albumID *int32) error {
	// Check if image exists
	exists, err := app.Queries.CheckSpotifyImageExists(ctx, path)
	if err != nil {
		return fmt.Errorf("failed to check if image exists: %w", err)
	}

	if exists {
		// Image exists, check if it has the relationship we want
		existingImage, err := app.Queries.GetSpotifyImageByPath(ctx, path)
		if err != nil {
			return fmt.Errorf("failed to get existing image: %w", err)
		}

		// Check if we need to update the relationships
		needsUpdate := false
		if musicianID != nil && (existingImage.MusicianID.Int32 != *musicianID || !existingImage.MusicianID.Valid) {
			needsUpdate = true
		}
		if albumID != nil && (existingImage.AlbumID.Int32 != *albumID || !existingImage.AlbumID.Valid) {
			needsUpdate = true
		}

		if needsUpdate {
			// Update the image with new relationships
			err = app.Queries.UpdateSpotifyImageRelations(ctx, database.UpdateSpotifyImageRelationsParams{
				Path:       path,
				MusicianID: pgtype.Int4{Int32: *musicianID, Valid: musicianID != nil},
				AlbumID:    pgtype.Int4{Int32: *albumID, Valid: albumID != nil},
			})
			if err != nil {
				return fmt.Errorf("failed to update image relationships: %w", err)
			}
		}
		// Image exists and has correct relationships, no action needed
		return nil
	}

	// Image doesn't exist, create it
	_, err = app.Queries.CreateSpotifyImage(ctx, database.CreateSpotifyImageParams{
		Path:   path,
		Width:  width,
		Height: height,
		MusicianID: pgtype.Int4{
			Int32: *musicianID,
			Valid: musicianID != nil,
		},
		AlbumID: pgtype.Int4{
			Int32: *albumID,
			Valid: albumID != nil,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create image: %w", err)
	}

	return nil
}

func (app *Application) ScanMusicLibrary() {
	entries, err := os.ReadDir(app.Settings.MusicDir)
	if err != nil {
		app.Logger.Error(fmt.Sprintf("fail to scan music directory %s\n%s", app.Settings.MusicDir, err.Error()))
		return
	}

	if len(entries) == 0 {
		app.Logger.Info(fmt.Sprintf("music directory %s appears to be empty", app.Settings.MusicDir))
		return
	}

	ctx := context.Background()

	for _, e := range entries {
		if !e.IsDir() || e.Name() == COMPILATIONS_DIR {
			continue
		}

		musician, err := app.ScanDirForMusician(ctx, e.Name())
		if err != nil {
			app.Logger.Error(err.Error())
			continue
		}

		albumsDir := filepath.Join(app.Settings.MusicDir, e.Name())

		albumDirEntries, err := os.ReadDir(albumsDir)
		if err != nil {
			app.Logger.Error(fmt.Sprintf("fail to read albums directory %s for musician %s\n%s", albumsDir, musician.Name, err.Error()))
			continue
		}

		if len(albumDirEntries) == 0 {
			app.Logger.Info(fmt.Sprintf("album directory %s appears to be empty", albumsDir))
			continue
		}

		for _, ae := range albumDirEntries {
			_, err := app.ScanDirForAlbum(ctx, ae.Name(), musician.ID)
			if err != nil {
				app.Logger.Error(err.Error())
				continue
			}

			// TODO: Use the album variable for additional processing if needed
		}
	}
	log.Println("finished with musci library scan")
}

func (app *Application) ScanDirForMusician(ctx context.Context, name string) (*database.Musician, error) {
	artistList, err := app.Spotify.SearchArtistByName(name)
	if err != nil {
		return nil, err
	}

	exist, err := app.Queries.CheckMusicianExistsBySpotifyID(ctx, artistList.ID.String())
	if err != nil {
		return nil, err
	}

	var musician database.Musician

	if exist {
		musician, err = app.Queries.GetMusicianBySpotifyID(ctx, artistList.ID.String())
		if err != nil {
			return nil, err
		}
	} else {
		artist, err := app.Spotify.GetArtistBySpotifyID(artistList.ID.String())
		if err != nil {
			return nil, err
		}

		musician, err = app.Queries.CreateMusician(ctx, database.CreateMusicianParams{
			Name:              artist.Name,
			SpotifyID:         artist.ID.String(),
			SpotifyPopularity: int32(artist.Popularity),
			SpotifyFollowers:  int32(artist.Followers.Count),
			Summary:           fmt.Sprintf("%s is an artist with %d rating and %d followers on Spotify", artist.Name, artist.Popularity, artist.Followers.Count),
			DirPath:           filepath.Join(app.Settings.MusicDir, name),
		})

		if err != nil {
			return nil, fmt.Errorf("fail to create musician %s in the data base\n%s", artist.Name, err.Error())
		}

		if len(artist.Images) > 0 {
			go func() {
				for _, img := range artist.Images {
					err := app.ensureSpotifyImage(ctx, img.URL, int32(img.Width), int32(img.Height), &musician.ID, nil)
					if err != nil {
						app.Logger.Error(fmt.Sprintf("fail to save image %s for musician %s\n%s", img.URL, musician.Name, err.Error()))
					}
				}
			}()
		}
	}

	return &musician, nil
}

func (app *Application) ScanDirForAlbum(ctx context.Context, dirName string, musicianID int32) (*database.Album, error) {
	albumList, err := app.Spotify.SearchAlbums(dirName, 1)
	if err != nil {
		return nil, fmt.Errorf("fail to get any results from spotify for album %s\n%s", dirName, err.Error())
	}

	if len(albumList) == 0 {
		return nil, fmt.Errorf("fail to get any results from spotify for album %s", dirName)
	}

	exists, err := app.Queries.CheckAlbumExistsBySpotifyID(ctx, albumList[0].ID.String())
	if err != nil {
		return nil, fmt.Errorf("fail to check if album %s with spotify id %s exists in the data base\n%s", albumList[0].Name, albumList[0].ID, err.Error())
	}

	var album database.Album

	if exists {
		album, err = app.Queries.GetAlbumBySpotifyID(ctx, albumList[0].ID.String())
		if err != nil {
			return nil, fmt.Errorf("fail to get existing album %s with spotify id %s from data base\n%s", albumList[0].Name, albumList[0].ID, err.Error())
		}
	} else {
		createAlbum := database.CreateAlbumParams{
			Title:                albumList[0].Name,
			SpotifyID:            albumList[0].ID.String(),
			ReleaseDate:          albumList[0].ReleaseDate,
			SpotifyPopularity:    0,
			TotalTracks:          int32(albumList[0].TotalTracks),
			TotalAvailableTracks: 0,
		}

		albumDetails, err := app.Spotify.GetAlbumBySpotifyID(albumList[0].ID.String())
		if err != nil {
			app.Logger.Error(fmt.Sprintf("fail to get album %s details from spotify\n%s", albumList[0].Name, err.Error()))
		} else {
			createAlbum.SpotifyPopularity = int32(albumDetails.Popularity)
		}

		album, err = app.Queries.CreateAlbum(ctx, createAlbum)
		if err != nil {
			return nil, fmt.Errorf("fail to create album %s\n%s", albumList[0].Name, err.Error())
		}

		if len(albumList[0].Images) > 0 {
			go func() {
				for _, img := range albumList[0].Images {
					err := app.ensureSpotifyImage(ctx, img.URL, int32(img.Width), int32(img.Height), nil, &album.ID)
					if err != nil {
						app.Logger.Error(fmt.Sprintf("fail to create image %s for album %s\n%s", img.URL, album.Title, err.Error()))
					}
				}
			}()
		}
	}

	return &album, nil
}
