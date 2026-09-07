package music

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/scanner"
)

func (s *Scanner) persistResolvedTrack(ctx context.Context, scan *musicScanContext, resolved *resolvedTrack) (int64, error) {
	txScan := scan.clone()

	s.scannerDBMu.Lock()
	defer s.scannerDBMu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to start music track transaction: %w", err)
	}
	defer tx.Rollback()

	qtx := s.queries.WithTx(tx)
	trackID, err := s.persistResolvedTrackTx(ctx, qtx, txScan, resolved)
	if err != nil {
		return 0, err
	}

	err = tx.Commit()
	if err != nil {
		return 0, fmt.Errorf("failed to commit music track transaction: %w", err)
	}

	// A rescan can move the file or change its type, so the cached lookup is
	// dropped here, after the new row is committed.
	s.invalidateCommittedTrack(trackID)

	// trackIndex is shared (never written inside the transaction) and is only
	// updated here, after a successful commit, so a track whose transaction
	// failed is never recorded as scanned/unchanged.
	scan.trackIndex[filepath.Clean(resolved.params.FilePath)] = resolved.params.Size
	scan.mergeFrom(txScan)

	return trackID, nil
}

func (s *Scanner) persistResolvedTrackTx(ctx context.Context, qtx *database.Queries, scan *musicScanContext, resolved *resolvedTrack) (int64, error) {
	params := resolved.params
	musicianIDs := make([]int64, 0, len(resolved.musicians))
	seenMusicianIDs := make(map[int64]struct{}, len(resolved.musicians))

	for _, musicianInput := range resolved.musicians {
		musicianID, err := s.persistMusician(ctx, qtx, scan, musicianInput)
		if err != nil {
			return 0, fmt.Errorf("musician failed: %w", err)
		}
		if !params.MusicianID.Valid {
			params.MusicianID = sql.NullInt64{Int64: musicianID, Valid: true}
		}
		_, exists := seenMusicianIDs[musicianID]
		if exists {
			continue
		}
		seenMusicianIDs[musicianID] = struct{}{}
		musicianIDs = append(musicianIDs, musicianID)
	}

	var albumID sql.NullInt64
	if resolved.album != nil {
		id, err := s.persistAlbum(ctx, qtx, scan, *resolved.album)
		if err != nil {
			return 0, fmt.Errorf("album failed: %w", err)
		}
		albumID = sql.NullInt64{Int64: id, Valid: true}
		params.AlbumID = albumID
	}

	if albumID.Valid {
		for _, musicianID := range musicianIDs {
			err := s.createMusicianAlbumIfNeeded(ctx, qtx, scan, musicianID, albumID.Int64)
			if err != nil {
				return 0, fmt.Errorf("musician-album relationship failed: %w", err)
			}
		}
	}

	track, err := qtx.UpsertTrack(ctx, params)
	if err != nil {
		return 0, fmt.Errorf("upsert track failed: %w", err)
	}

	err = s.syncTrackMusicians(ctx, qtx, track.ID, musicianIDs)
	if err != nil {
		return 0, fmt.Errorf("track-musician relationships failed: %w", err)
	}

	if resolved.genreTag == "" {
		err = qtx.DeleteTrackGenres(ctx, track.ID)
		if err != nil {
			return 0, fmt.Errorf("delete track genres failed: %w", err)
		}
	} else {
		genreID, err := s.getOrCreateMusicGenreID(ctx, qtx, scan, resolved.genreTag)
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

		err = qtx.CreateTrackGenre(ctx, database.CreateTrackGenreParams{TrackID: track.ID, GenreID: genreID})
		if err != nil {
			return 0, fmt.Errorf("track-genre relationship failed: %w", err)
		}

		for _, musicianID := range musicianIDs {
			err = s.createMusicianGenreIfNeeded(ctx, qtx, scan, musicianID, genreID)
			if err != nil {
				return 0, fmt.Errorf("musician-genre relationship failed: %w", err)
			}
		}

		if albumID.Valid {
			err = s.createAlbumGenreIfNeeded(ctx, qtx, scan, albumID.Int64, genreID)
			if err != nil {
				return 0, fmt.Errorf("album-genre relationship failed: %w", err)
			}
		}
	}

	return track.ID, nil
}

func (s *Scanner) persistMusician(ctx context.Context, qtx *database.Queries, scan *musicScanContext, input resolvedMusician) (int64, error) {
	cacheKey := scanner.NormalizedScanCacheKey(input.name, input.sortName)
	cachedID, ok := scan.musicianIDs.Get(cacheKey)
	if ok {
		return cachedID, nil
	}

	var musician database.Musician
	var err error
	if input.spotifyArtist != nil {
		spotifyID := sql.NullString{String: input.spotifyArtist.ID.String(), Valid: true}
		if input.existing != nil && input.existing.SpotifyID == spotifyID {
			musician = *input.existing
		} else {
			musician, err = qtx.GetMusicianBySpotifyID(ctx, spotifyID)
		}
		notFound := errors.Is(err, sql.ErrNoRows)
		if notFound {
			params := database.UpsertMusicianParams{
				Name:              input.name,
				SortName:          input.sortName,
				Summary:           sql.NullString{String: generateMusicianSummary(input.spotifyArtist), Valid: true},
				SpotifyPopularity: helpers.NullFloat64(float64(input.spotifyArtist.Popularity)),
				SpotifyFollowers:  helpers.NullInt64(int64(input.spotifyArtist.Followers.Count)),
				SpotifyID:         spotifyID,
				Thumb:             helpers.NullString(firstImageURL(input.spotifyArtist.Images)),
			}
			musician, err = qtx.UpsertMusician(ctx, params)
		}
	} else if input.hasExistingID {
		musician.ID = input.existingID
	} else {
		musician, err = qtx.UpsertMusician(ctx, database.UpsertMusicianParams{Name: input.name, SortName: input.sortName})
	}
	if err != nil {
		return 0, err
	}

	if input.spotifyArtist != nil {
		musician, err = s.updateMusicianThumbIfChanged(ctx, qtx, musician, firstImageURL(input.spotifyArtist.Images))
		if err != nil {
			return 0, err
		}
		err = s.processSpotifyGenres(ctx, qtx, scan, musician.ID, input.spotifyArtist.Genres)
		if err != nil {
			return 0, err
		}
	}
	err = s.upsertMusicSpotifyMatchAndCacheID(ctx, qtx, musicSpotifyEntityMusician, musician.ID, input.spotifyMatch, scan.musicianIDs, cacheKey)
	if err != nil {
		return 0, err
	}
	return musician.ID, nil
}

func (s *Scanner) persistAlbum(ctx context.Context, qtx *database.Queries, scan *musicScanContext, input resolvedAlbum) (int64, error) {
	cacheKey := scanner.NormalizedScanCacheKey(input.title, input.albumArtist)
	cachedID, ok := scan.albumIDs.Get(cacheKey)
	if ok {
		return cachedID, nil
	}

	var album database.Album
	var err error
	if input.spotifyAlbum != nil {
		spotifyID := sql.NullString{String: input.spotifyAlbum.ID.String(), Valid: true}
		if input.existing != nil && input.existing.SpotifyID == spotifyID {
			album = *input.existing
		} else {
			album, err = qtx.GetAlbumBySpotifyID(ctx, spotifyID)
		}
		notFound := errors.Is(err, sql.ErrNoRows)
		if notFound {
			params := database.UpsertAlbumParams{
				Title:             input.title,
				SortTitle:         input.sortTitle,
				SpotifyID:         spotifyID,
				SpotifyPopularity: helpers.NullFloat64(float64(input.spotifyAlbum.Popularity)),
				TotalTracks:       helpers.NullInt64(int64(input.spotifyAlbum.TotalTracks)),
				Cover:             helpers.NullString(firstImageURL(input.spotifyAlbum.Images)),
			}

			releaseDate := input.spotifyAlbum.ReleaseDateTime()
			hasReleaseDate := !releaseDate.IsZero()
			if hasReleaseDate {
				params.ReleaseDate = sql.NullString{String: releaseDate.Format("2006-01-02"), Valid: true}
				params.Year = sql.NullInt64{Int64: int64(releaseDate.Year()), Valid: true}
			}
			if input.albumArtist != "" {
				params.Musician = sql.NullString{String: input.albumArtist, Valid: true}
			}

			album, err = qtx.UpsertAlbum(ctx, params)
		}
	} else if input.hasExistingID {
		album.ID = input.existingID
	} else {
		params := database.UpsertAlbumParams{Title: input.title, SortTitle: input.sortTitle}
		if input.albumArtist != "" {
			params.Musician = sql.NullString{String: input.albumArtist, Valid: true}
		}
		album, err = qtx.UpsertAlbum(ctx, params)
	}
	if err != nil {
		return 0, err
	}

	if input.spotifyAlbum != nil {
		album, err = s.updateAlbumCoverIfChanged(ctx, qtx, album, firstImageURL(input.spotifyAlbum.Images))
		if err != nil {
			return 0, err
		}
		err = s.processSpotifyAlbumGenres(ctx, qtx, scan, album.ID, input.spotifyAlbum.Genres)
		if err != nil {
			return 0, err
		}
	}
	err = s.upsertMusicSpotifyMatchAndCacheID(ctx, qtx, musicSpotifyEntityAlbum, album.ID, input.spotifyMatch, scan.albumIDs, cacheKey)
	if err != nil {
		return 0, err
	}
	return album.ID, nil
}

func (s *Scanner) updateMusicianThumbIfChanged(ctx context.Context, qtx *database.Queries, musician database.Musician, thumbURL string) (database.Musician, error) {
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

func (s *Scanner) updateAlbumCoverIfChanged(ctx context.Context, qtx *database.Queries, album database.Album, coverURL string) (database.Album, error) {
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

func (s *Scanner) syncTrackMusicians(ctx context.Context, qtx *database.Queries, trackID int64, musicianIDs []int64) error {
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
