package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	spotifyapi "igloo/cmd/internal/spotify"
	"path/filepath"
	"strings"

	spotifylib "github.com/zmb3/spotify/v2"
)

const (
	musicSpotifyEntityAlbum     = "album"
	musicSpotifyEntityMusician  = "musician"
	musicSpotifyStatusMatched   = "matched"
	musicSpotifyStatusFailed    = "failed"
	musicSpotifyStatusUnmatched = "unmatched"
)

func generateMusicianSummary(artist *spotifylib.FullArtist) string {
	var parts []string

	parts = append(parts, artist.Name)

	if len(artist.Genres) > 0 {
		maxGenres := 3
		if len(artist.Genres) < maxGenres {
			maxGenres = len(artist.Genres)
		}
		genreStr := strings.Join(artist.Genres[:maxGenres], ", ")
		parts = append(parts, fmt.Sprintf("known for %s", genreStr))
	}

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

	followers := artist.Followers.Count
	switch {
	case followers >= 10_000_000:
		parts = append(parts, fmt.Sprintf("with over %dM followers on Spotify", followers/1_000_000))
	case followers >= 1_000_000:
		parts = append(parts, fmt.Sprintf("with %.1fM followers on Spotify", float64(followers)/1_000_000))
	case followers >= 100_000:
		parts = append(parts, fmt.Sprintf("with %dK followers on Spotify", followers/1_000))
	case followers >= 1_000:
		parts = append(parts, fmt.Sprintf("with %.1fK followers on Spotify", float64(followers)/1_000))
	default:
		parts = append(parts, fmt.Sprintf("with %d followers on Spotify", followers))
	}

	return strings.Join(parts, " ") + "."
}

func (app *Application) resolveMusician(ctx context.Context, scan *musicScanContext, name, sortName string) (*resolvedMusician, error) {
	cacheKey := normalizedMusicCacheKey(name, sortName)
	if musicianID, ok := scan.musicianIDs[cacheKey]; ok {
		return &resolvedMusician{
			name:          name,
			sortName:      sortName,
			existingID:    musicianID,
			hasExistingID: true,
		}, nil
	}

	resolved := &resolvedMusician{name: name, sortName: sortName}

	existing, found, err := app.findExistingMusician(ctx, name)
	if err != nil {
		return nil, err
	}
	if found {
		resolved.existingID = existing.ID
		resolved.hasExistingID = true

		persisted, matchErr := app.Queries.GetMusicSpotifyMatch(ctx, database.GetMusicSpotifyMatchParams{
			EntityType: musicSpotifyEntityMusician,
			EntityID:   existing.ID,
		})
		if matchErr == nil {
			if persisted.Status == musicSpotifyStatusMatched || persisted.Status == musicSpotifyStatusUnmatched {
				resolved.splitCompoundOnNoMatch = persistedSpotifyMatchSplitsCompound(persisted)
				scan.musicianIDs[cacheKey] = existing.ID
				return resolved, nil
			}
		} else if !errors.Is(matchErr, sql.ErrNoRows) {
			return nil, matchErr
		}
	}

	spotifyKey := normalizedMusicCacheKey(name)
	if cachedMiss, ok := scan.spotifyArtistMisses[spotifyKey]; ok {
		resolved.spotifyMatch = &cachedMiss
		resolved.splitCompoundOnNoMatch = spotifyMatchSplitsCompound(cachedMiss)
		return resolved, nil
	}

	if app.Spotify == nil {
		if found {
			scan.musicianIDs[cacheKey] = existing.ID
		}
		return resolved, nil
	}

	artist, err := app.Spotify.SearchArtistByName(ctx, name)
	if err != nil {
		match := resolvedSpotifyMatchFromError(err)
		scan.spotifyArtistMisses[spotifyKey] = match
		resolved.spotifyMatch = &match
		resolved.splitCompoundOnNoMatch = shouldSplitCompoundArtistCredits(err)
		return resolved, nil
	}

	if artist != nil {
		resolved.spotifyArtist = artist
		match := resolvedSpotifyMatch{
			status:    musicSpotifyStatusMatched,
			spotifyID: sql.NullString{String: artist.ID.String(), Valid: true},
		}
		resolved.spotifyMatch = &match
	}

	return resolved, nil
}

func (app *Application) resolveAlbum(ctx context.Context, scan *musicScanContext, title, sortTitle, albumArtist string) (*resolvedAlbum, error) {
	cacheKey := normalizedMusicCacheKey(title, albumArtist)
	if albumID, ok := scan.albumIDs[cacheKey]; ok {
		return &resolvedAlbum{
			title:         title,
			sortTitle:     sortTitle,
			albumArtist:   albumArtist,
			existingID:    albumID,
			hasExistingID: true,
		}, nil
	}

	resolved := &resolvedAlbum{
		title:       title,
		sortTitle:   sortTitle,
		albumArtist: albumArtist,
	}

	existing, found, err := app.findExistingAlbum(ctx, title, albumArtist)
	if err != nil {
		return nil, err
	}
	if found {
		resolved.existingID = existing.ID
		resolved.hasExistingID = true

		persisted, matchErr := app.Queries.GetMusicSpotifyMatch(ctx, database.GetMusicSpotifyMatchParams{
			EntityType: musicSpotifyEntityAlbum,
			EntityID:   existing.ID,
		})
		if matchErr == nil {
			if persisted.Status == musicSpotifyStatusMatched || persisted.Status == musicSpotifyStatusUnmatched {
				scan.albumIDs[cacheKey] = existing.ID
				return resolved, nil
			}
		} else if !errors.Is(matchErr, sql.ErrNoRows) {
			return nil, matchErr
		}
	}

	spotifyKey := normalizedMusicCacheKey(title, albumArtist)
	if cachedMiss, ok := scan.spotifyAlbumMisses[spotifyKey]; ok {
		resolved.spotifyMatch = &cachedMiss
		return resolved, nil
	}

	if app.Spotify == nil {
		if found {
			scan.albumIDs[cacheKey] = existing.ID
		}
		return resolved, nil
	}

	albumDetails, err := app.Spotify.SearchAndGetAlbumDetails(ctx, title, albumArtist)
	if err != nil {
		match := resolvedSpotifyMatchFromError(err)
		scan.spotifyAlbumMisses[spotifyKey] = match
		resolved.spotifyMatch = &match
		return resolved, nil
	}

	if albumDetails != nil {
		resolved.spotifyAlbum = albumDetails
		match := resolvedSpotifyMatch{
			status:    musicSpotifyStatusMatched,
			spotifyID: sql.NullString{String: albumDetails.ID.String(), Valid: true},
		}
		resolved.spotifyMatch = &match
	}

	return resolved, nil
}

func (app *Application) findExistingMusician(ctx context.Context, name string) (database.Musician, bool, error) {
	musician, err := app.Queries.GetMusicianByName(ctx, name)
	if err == nil {
		return musician, true, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return database.Musician{}, false, nil
	}
	return database.Musician{}, false, err
}

func (app *Application) findExistingAlbum(ctx context.Context, title, albumArtist string) (database.Album, bool, error) {
	album, err := app.Queries.GetAlbumByTitleAndMusician(ctx, database.GetAlbumByTitleAndMusicianParams{
		Title:    title,
		Musician: helpers.NullString(albumArtist),
	})
	if err == nil {
		return album, true, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return database.Album{}, false, nil
	}
	return database.Album{}, false, err
}

func (app *Application) persistResolvedTrack(ctx context.Context, scan *musicScanContext, resolved *resolvedTrack) (int64, error) {
	txScan := scan.clone()

	tx, err := app.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to start music track transaction: %w", err)
	}
	defer tx.Rollback()

	qtx := app.Queries.WithTx(tx)
	trackID, err := app.persistResolvedTrackTx(ctx, qtx, txScan, resolved)
	if err != nil {
		return 0, err
	}

	err = tx.Commit()
	if err != nil {
		return 0, fmt.Errorf("failed to commit music track transaction: %w", err)
	}

	txScan.trackIndex[filepath.Clean(resolved.filePath)] = resolved.fileSize
	scan.mergeFrom(txScan)

	return trackID, nil
}

func (app *Application) persistResolvedTrackTx(ctx context.Context, qtx *database.Queries, scan *musicScanContext, resolved *resolvedTrack) (int64, error) {
	params := resolved.params
	musicianIDs := make([]int64, 0, len(resolved.musicians))
	seenMusicianIDs := make(map[int64]struct{}, len(resolved.musicians))

	for _, musicianInput := range resolved.musicians {
		musicianID, err := app.persistMusician(ctx, qtx, scan, musicianInput)
		if err != nil {
			return 0, fmt.Errorf("musician failed: %w", err)
		}
		if !params.MusicianID.Valid {
			params.MusicianID = sql.NullInt64{Int64: musicianID, Valid: true}
		}
		if _, exists := seenMusicianIDs[musicianID]; exists {
			continue
		}
		seenMusicianIDs[musicianID] = struct{}{}
		musicianIDs = append(musicianIDs, musicianID)
	}

	var albumID sql.NullInt64
	if resolved.album != nil {
		id, err := app.persistAlbum(ctx, qtx, scan, *resolved.album)
		if err != nil {
			return 0, fmt.Errorf("album failed: %w", err)
		}
		albumID = sql.NullInt64{Int64: id, Valid: true}
		params.AlbumID = albumID
	}

	if albumID.Valid {
		for _, musicianID := range musicianIDs {
			err := app.createMusicianAlbumIfNeeded(ctx, qtx, scan, musicianID, albumID.Int64)
			if err != nil {
				app.Logger.Warn("failed to create musician-album relationship",
					"error", err,
					"musician_id", musicianID,
					"album_id", albumID.Int64,
				)
			}
		}
	}

	track, err := qtx.UpsertTrack(ctx, params)
	if err != nil {
		return 0, fmt.Errorf("upsert track failed: %w", err)
	}

	err = app.syncTrackMusicians(ctx, qtx, track.ID, musicianIDs)
	if err != nil {
		return 0, fmt.Errorf("track-musician relationships failed: %w", err)
	}

	if resolved.genreTag == "" {
		err = qtx.DeleteTrackGenres(ctx, track.ID)
		if err != nil {
			return 0, fmt.Errorf("delete track genres failed: %w", err)
		}
	} else {
		genreID, err := app.getOrCreateMusicGenreID(ctx, qtx, scan, resolved.genreTag)
		if err != nil {
			return 0, fmt.Errorf("genre failed: %w", err)
		}

		err = qtx.DeleteTrackGenresExcept(ctx, database.DeleteTrackGenresExceptParams{
			TrackID: track.ID,
			GenreID: genreID,
		})
		if err != nil {
			return 0, fmt.Errorf("delete stale genres failed: %w", err)
		}

		err = app.createTrackGenreIfNeeded(ctx, qtx, scan, track.ID, genreID)
		if err != nil {
			return 0, fmt.Errorf("track-genre relationship failed: %w", err)
		}

		for _, musicianID := range musicianIDs {
			err = app.createMusicianGenreIfNeeded(ctx, qtx, scan, musicianID, genreID)
			if err != nil {
				app.Logger.Warn("failed to create musician-genre relationship",
					"error", err,
					"musician_id", musicianID,
					"genre_id", genreID,
				)
			}
		}

		if albumID.Valid {
			err = app.createAlbumGenreIfNeeded(ctx, qtx, scan, albumID.Int64, genreID)
			if err != nil {
				app.Logger.Warn("failed to create album-genre relationship",
					"error", err,
					"album_id", albumID.Int64,
					"genre_id", genreID,
				)
			}
		}
	}

	return track.ID, nil
}

func (app *Application) persistMusician(ctx context.Context, qtx *database.Queries, scan *musicScanContext, input resolvedMusician) (int64, error) {
	cacheKey := normalizedMusicCacheKey(input.name, input.sortName)
	if musicianID, ok := scan.musicianIDs[cacheKey]; ok {
		return musicianID, nil
	}

	var musician database.Musician
	var err error

	if input.spotifyArtist != nil {
		spotifyID := sql.NullString{String: input.spotifyArtist.ID.String(), Valid: true}
		musician, err = qtx.GetMusicianBySpotifyID(ctx, spotifyID)
		if err == nil {
			musician, err = app.updateMusicianThumbIfChanged(ctx, qtx, musician, firstArtistImageURL(input.spotifyArtist))
			if err != nil {
				return 0, err
			}
			app.processSpotifyGenres(ctx, qtx, scan, musician.ID, input.spotifyArtist.Genres)
			err = app.upsertMusicSpotifyMatch(ctx, qtx, musicSpotifyEntityMusician, musician.ID, input.spotifyMatch)
			if err != nil {
				return 0, err
			}
			scan.musicianIDs[cacheKey] = musician.ID
			return musician.ID, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return 0, err
		}

		params := database.UpsertMusicianParams{
			Name:              input.name,
			SortName:          input.sortName,
			Summary:           sql.NullString{String: generateMusicianSummary(input.spotifyArtist), Valid: true},
			SpotifyPopularity: helpers.NullFloat64(float64(input.spotifyArtist.Popularity)),
			SpotifyFollowers:  helpers.NullInt64(int64(input.spotifyArtist.Followers.Count)),
			SpotifyID:         spotifyID,
			Thumb:             helpers.NullString(firstArtistImageURL(input.spotifyArtist)),
		}
		musician, err = qtx.UpsertMusician(ctx, params)
		if err != nil {
			return 0, err
		}
		app.processSpotifyGenres(ctx, qtx, scan, musician.ID, input.spotifyArtist.Genres)
		err = app.upsertMusicSpotifyMatch(ctx, qtx, musicSpotifyEntityMusician, musician.ID, input.spotifyMatch)
		if err != nil {
			return 0, err
		}
		scan.musicianIDs[cacheKey] = musician.ID
		return musician.ID, nil
	}

	if input.hasExistingID {
		err = app.upsertMusicSpotifyMatch(ctx, qtx, musicSpotifyEntityMusician, input.existingID, input.spotifyMatch)
		if err != nil {
			return 0, err
		}
		scan.musicianIDs[cacheKey] = input.existingID
		return input.existingID, nil
	}

	musician, err = qtx.UpsertMusician(ctx, database.UpsertMusicianParams{
		Name:     input.name,
		SortName: input.sortName,
	})
	if err != nil {
		return 0, err
	}

	err = app.upsertMusicSpotifyMatch(ctx, qtx, musicSpotifyEntityMusician, musician.ID, input.spotifyMatch)
	if err != nil {
		return 0, err
	}

	scan.musicianIDs[cacheKey] = musician.ID
	return musician.ID, nil
}

func (app *Application) persistAlbum(ctx context.Context, qtx *database.Queries, scan *musicScanContext, input resolvedAlbum) (int64, error) {
	cacheKey := normalizedMusicCacheKey(input.title, input.albumArtist)
	if albumID, ok := scan.albumIDs[cacheKey]; ok {
		return albumID, nil
	}

	var album database.Album
	var err error

	if input.spotifyAlbum != nil {
		spotifyID := sql.NullString{String: input.spotifyAlbum.ID.String(), Valid: true}
		album, err = qtx.GetAlbumBySpotifyID(ctx, spotifyID)
		if err == nil {
			album, err = app.updateAlbumCoverIfChanged(ctx, qtx, album, firstAlbumImageURL(input.spotifyAlbum))
			if err != nil {
				return 0, err
			}
			app.processSpotifyAlbumGenres(ctx, qtx, scan, album.ID, input.spotifyAlbum.Genres)
			err = app.upsertMusicSpotifyMatch(ctx, qtx, musicSpotifyEntityAlbum, album.ID, input.spotifyMatch)
			if err != nil {
				return 0, err
			}
			scan.albumIDs[cacheKey] = album.ID
			return album.ID, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return 0, err
		}

		params := database.UpsertAlbumParams{
			Title:             input.title,
			SortTitle:         input.sortTitle,
			SpotifyID:         spotifyID,
			SpotifyPopularity: helpers.NullFloat64(float64(input.spotifyAlbum.Popularity)),
			TotalTracks:       helpers.NullInt64(int64(input.spotifyAlbum.TotalTracks)),
			Cover:             helpers.NullString(firstAlbumImageURL(input.spotifyAlbum)),
		}

		releaseDate := input.spotifyAlbum.ReleaseDateTime()
		if !releaseDate.IsZero() {
			params.ReleaseDate = sql.NullString{String: releaseDate.Format("2006-01-02"), Valid: true}
			params.Year = sql.NullInt64{Int64: int64(releaseDate.Year()), Valid: true}
		}
		if input.albumArtist != "" {
			params.Musician = sql.NullString{String: input.albumArtist, Valid: true}
		}

		album, err = qtx.UpsertAlbum(ctx, params)
		if err != nil {
			return 0, err
		}
		app.processSpotifyAlbumGenres(ctx, qtx, scan, album.ID, input.spotifyAlbum.Genres)
		err = app.upsertMusicSpotifyMatch(ctx, qtx, musicSpotifyEntityAlbum, album.ID, input.spotifyMatch)
		if err != nil {
			return 0, err
		}
		scan.albumIDs[cacheKey] = album.ID
		return album.ID, nil
	}

	if input.hasExistingID {
		err = app.upsertMusicSpotifyMatch(ctx, qtx, musicSpotifyEntityAlbum, input.existingID, input.spotifyMatch)
		if err != nil {
			return 0, err
		}
		scan.albumIDs[cacheKey] = input.existingID
		return input.existingID, nil
	}

	params := database.UpsertAlbumParams{
		Title:     input.title,
		SortTitle: input.sortTitle,
	}
	if input.albumArtist != "" {
		params.Musician = sql.NullString{String: input.albumArtist, Valid: true}
	}

	album, err = qtx.UpsertAlbum(ctx, params)
	if err != nil {
		return 0, err
	}

	err = app.upsertMusicSpotifyMatch(ctx, qtx, musicSpotifyEntityAlbum, album.ID, input.spotifyMatch)
	if err != nil {
		return 0, err
	}

	scan.albumIDs[cacheKey] = album.ID
	return album.ID, nil
}

func (app *Application) updateMusicianThumbIfChanged(ctx context.Context, qtx *database.Queries, musician database.Musician, thumbURL string) (database.Musician, error) {
	if thumbURL == "" {
		return musician, nil
	}
	if musician.Thumb.Valid && musician.Thumb.String == thumbURL {
		return musician, nil
	}

	return qtx.UpdateMusicianSpotifyThumb(ctx, database.UpdateMusicianSpotifyThumbParams{
		ID:    musician.ID,
		Thumb: sql.NullString{String: thumbURL, Valid: true},
	})
}

func (app *Application) updateAlbumCoverIfChanged(ctx context.Context, qtx *database.Queries, album database.Album, coverURL string) (database.Album, error) {
	if coverURL == "" {
		return album, nil
	}
	if album.Cover.Valid && album.Cover.String == coverURL {
		return album, nil
	}

	return qtx.UpdateAlbumSpotifyCover(ctx, database.UpdateAlbumSpotifyCoverParams{
		ID:    album.ID,
		Cover: sql.NullString{String: coverURL, Valid: true},
	})
}

func (app *Application) processSpotifyGenres(ctx context.Context, qtx *database.Queries, scan *musicScanContext, musicianID int64, spotifyGenres []string) {
	if len(spotifyGenres) == 0 {
		return
	}
	if _, ok := scan.spotifyMusicianGenresHandled[musicianID]; ok {
		return
	}

	hadError := false
	for _, genreTag := range spotifyGenres {
		genreID, err := app.getOrCreateMusicGenreID(ctx, qtx, scan, genreTag)
		if err != nil {
			hadError = true
			app.Logger.Warn("failed to get/create Spotify genre",
				"error", err,
				"genre", genreTag,
			)
			continue
		}

		err = app.createMusicianGenreIfNeeded(ctx, qtx, scan, musicianID, genreID)
		if err != nil {
			hadError = true
			app.Logger.Warn("failed to create musician-genre relationship for Spotify genre",
				"error", err,
				"musician_id", musicianID,
				"genre_id", genreID,
				"genre", genreTag,
			)
		}
	}

	if !hadError {
		scan.spotifyMusicianGenresHandled[musicianID] = struct{}{}
	}
}

func (app *Application) processSpotifyAlbumGenres(ctx context.Context, qtx *database.Queries, scan *musicScanContext, albumID int64, spotifyGenres []string) {
	if len(spotifyGenres) == 0 {
		return
	}
	if _, ok := scan.spotifyAlbumGenresHandled[albumID]; ok {
		return
	}

	hadError := false
	for _, genreTag := range spotifyGenres {
		genreID, err := app.getOrCreateMusicGenreID(ctx, qtx, scan, genreTag)
		if err != nil {
			hadError = true
			app.Logger.Warn("failed to get/create Spotify genre for album",
				"error", err,
				"genre", genreTag,
			)
			continue
		}

		err = app.createAlbumGenreIfNeeded(ctx, qtx, scan, albumID, genreID)
		if err != nil {
			hadError = true
			app.Logger.Warn("failed to create album-genre relationship for Spotify genre",
				"error", err,
				"album_id", albumID,
				"genre_id", genreID,
				"genre", genreTag,
			)
		}
	}

	if !hadError {
		scan.spotifyAlbumGenresHandled[albumID] = struct{}{}
	}
}

func (app *Application) getOrCreateMusicGenreID(ctx context.Context, qtx *database.Queries, scan *musicScanContext, tag string) (int64, error) {
	cacheKey := normalizedMusicCacheKey(tag, "music")
	if genreID, ok := scan.genreIDs[cacheKey]; ok {
		return genreID, nil
	}

	genre, err := qtx.GetOrCreateGenre(ctx, database.GetOrCreateGenreParams{
		Tag:       tag,
		GenreType: "music",
	})
	if err != nil {
		return 0, err
	}

	scan.genreIDs[cacheKey] = genre.ID
	return genre.ID, nil
}

func (app *Application) createMusicianAlbumIfNeeded(ctx context.Context, qtx *database.Queries, scan *musicScanContext, musicianID, albumID int64) error {
	cacheKey := musicIDPairKey(musicianID, albumID)
	if _, ok := scan.musicianAlbums[cacheKey]; ok {
		return nil
	}

	err := qtx.CreateMusicianAlbum(ctx, database.CreateMusicianAlbumParams{
		MusicianID: musicianID,
		AlbumID:    albumID,
	})
	if err != nil {
		return err
	}

	scan.musicianAlbums[cacheKey] = struct{}{}
	return nil
}

func (app *Application) createMusicianGenreIfNeeded(ctx context.Context, qtx *database.Queries, scan *musicScanContext, musicianID, genreID int64) error {
	cacheKey := musicIDPairKey(musicianID, genreID)
	if _, ok := scan.musicianGenres[cacheKey]; ok {
		return nil
	}

	err := qtx.UpsertMusicianGenre(ctx, database.UpsertMusicianGenreParams{
		MusicianID: musicianID,
		GenreID:    genreID,
	})
	if err != nil {
		return err
	}

	scan.musicianGenres[cacheKey] = struct{}{}
	return nil
}

func (app *Application) createAlbumGenreIfNeeded(ctx context.Context, qtx *database.Queries, scan *musicScanContext, albumID, genreID int64) error {
	cacheKey := musicIDPairKey(albumID, genreID)
	if _, ok := scan.albumGenres[cacheKey]; ok {
		return nil
	}

	err := qtx.UpsertAlbumGenre(ctx, database.UpsertAlbumGenreParams{
		AlbumID: albumID,
		GenreID: genreID,
	})
	if err != nil {
		return err
	}

	scan.albumGenres[cacheKey] = struct{}{}
	return nil
}

func (app *Application) createTrackGenreIfNeeded(ctx context.Context, qtx *database.Queries, scan *musicScanContext, trackID, genreID int64) error {
	cacheKey := musicIDPairKey(trackID, genreID)
	if _, ok := scan.trackGenres[cacheKey]; ok {
		return nil
	}

	err := qtx.CreateTrackGenre(ctx, database.CreateTrackGenreParams{
		TrackID: trackID,
		GenreID: genreID,
	})
	if err != nil {
		return err
	}

	scan.trackGenres[cacheKey] = struct{}{}
	return nil
}

func (app *Application) syncTrackMusicians(ctx context.Context, qtx *database.Queries, trackID int64, musicianIDs []int64) error {
	if len(musicianIDs) == 0 {
		return qtx.DeleteTrackMusicians(ctx, trackID)
	}

	err := qtx.DeleteTrackMusiciansExcept(ctx, database.DeleteTrackMusiciansExceptParams{
		TrackID:     trackID,
		MusicianIds: musicianIDs,
	})
	if err != nil {
		return err
	}

	for _, musicianID := range musicianIDs {
		err = qtx.CreateTrackMusician(ctx, database.CreateTrackMusicianParams{
			TrackID:    trackID,
			MusicianID: musicianID,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func (app *Application) upsertMusicSpotifyMatch(ctx context.Context, qtx *database.Queries, entityType string, entityID int64, match *resolvedSpotifyMatch) error {
	if match == nil {
		return nil
	}

	return qtx.UpsertMusicSpotifyMatch(ctx, database.UpsertMusicSpotifyMatchParams{
		EntityType:      entityType,
		EntityID:        entityID,
		SpotifyID:       match.spotifyID,
		Status:          match.status,
		Reason:          match.reason,
		Score:           match.score,
		ThresholdValue:  match.thresholdValue,
		CandidateName:   match.candidateName,
		CandidateArtist: match.candidateArtist,
		SearchQuery:     match.searchQuery,
		Strategy:        match.strategy,
		Error:           match.errorText,
	})
}

func resolvedSpotifyMatchFromError(err error) resolvedSpotifyMatch {
	match := resolvedSpotifyMatch{
		status:    musicSpotifyStatusFailed,
		errorText: helpers.NullString(err.Error()),
	}

	matchErr, ok := spotifyapi.AsMatchError(err)
	if !ok {
		return match
	}

	info := matchErr.Info
	if info.Reason == "no_results" || info.Reason == "score_below_threshold" || info.Reason == "empty_query" {
		match.status = musicSpotifyStatusUnmatched
		match.errorText = sql.NullString{}
	}

	match.reason = helpers.NullString(info.Reason)
	match.candidateName = helpers.NullString(info.CandidateName)
	match.candidateArtist = helpers.NullString(info.CandidateArtist)
	match.searchQuery = helpers.NullString(info.SearchQuery)
	match.strategy = helpers.NullString(info.Strategy)

	if info.Score > 0 {
		match.score = sql.NullInt64{Int64: int64(info.Score), Valid: true}
	}
	if info.Threshold > 0 {
		match.thresholdValue = sql.NullInt64{Int64: int64(info.Threshold), Valid: true}
	}
	if matchErr.Err != nil && match.status == musicSpotifyStatusFailed {
		match.errorText = helpers.NullString(matchErr.Err.Error())
	}

	return match
}

func spotifyMatchSplitsCompound(match resolvedSpotifyMatch) bool {
	if match.status != musicSpotifyStatusUnmatched || !match.reason.Valid {
		return false
	}

	return match.reason.String == "no_results" || match.reason.String == "score_below_threshold"
}

func persistedSpotifyMatchSplitsCompound(match database.MusicSpotifyMatch) bool {
	if match.Status != musicSpotifyStatusUnmatched || !match.Reason.Valid {
		return false
	}

	return match.Reason.String == "no_results" || match.Reason.String == "score_below_threshold"
}

func firstArtistImageURL(artist *spotifylib.FullArtist) string {
	if artist == nil || len(artist.Images) == 0 {
		return ""
	}

	return artist.Images[0].URL
}

func firstAlbumImageURL(album *spotifylib.FullAlbum) string {
	if album == nil || len(album.Images) == 0 {
		return ""
	}

	return album.Images[0].URL
}
