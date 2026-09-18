package music

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/scanner"
)

func (s *Scanner) persistResolvedTrack(ctx context.Context, scan *musicScanContext, resolved *resolvedTrack) (int64, error) {
	if resolved.inspection == nil {
		return 0, fmt.Errorf("missing file inspection")
	}
	txScan := scan.clone()
	var trackID int64
	err := s.tx.Run(ctx, func(qtx *database.Queries) error {
		var err error
		trackID, err = s.persistResolvedTrackTx(ctx, qtx, txScan, resolved)
		if err != nil {
			return err
		}
		err = storeTrackFingerprint(ctx, qtx, resolved.params.FilePath, resolved.inspection.Fingerprint)
		if err != nil {
			return err
		}
		return resolved.inspection.Validate(ctx)
	}, func() {
		// A rescan can move the file or change its type, so the cached lookup is
		// dropped after the new row is committed.
		s.invalidateCommittedTrack(trackID)
		for id := range txScan.invalidatedTracks {
			s.invalidateCommittedTrack(id)
		}
	})
	if err != nil {
		return 0, err
	}

	// trackIndex is shared (never written inside the transaction) and is only
	// updated here, after a successful commit, so a track whose transaction
	// failed is never recorded as scanned/unchanged.
	scan.trackIndex[filepath.Clean(resolved.params.FilePath)] = resolved.inspection.Fingerprint

	scan.mergeFrom(txScan)

	return trackID, nil
}

func (s *Scanner) persistResolvedTrackTx(ctx context.Context, qtx *database.Queries, scan *musicScanContext, resolved *resolvedTrack) (int64, error) {
	oldArtists, err := qtx.MusicTrackAffectedArtists(ctx, resolved.params.FilePath)
	if err != nil {
		return 0, err
	}
	oldAlbum, err := qtx.MusicTrackAffectedAlbum(ctx, resolved.params.FilePath)
	notFound := errors.Is(err, sql.ErrNoRows)
	if err != nil && !notFound {
		return 0, err
	}
	params := resolved.params
	musicianIDs, err := s.persistMusicians(ctx, qtx, scan, resolved.musicians)
	if err != nil {
		return 0, err
	}
	params.MusicianID = sql.NullInt64{}
	if len(musicianIDs) > 0 {
		params.MusicianID = sql.NullInt64{Int64: musicianIDs[0], Valid: true}
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

	trackID, err := qtx.UpsertTrack(ctx, params)
	if err != nil {
		return 0, fmt.Errorf("upsert track failed: %w", err)
	}

	err = s.syncTrackMusicians(ctx, qtx, trackID, musicianIDs)
	if err != nil {
		return 0, fmt.Errorf("track-musician relationships failed: %w", err)
	}

	if resolved.genreTag == "" {
		err = qtx.DeleteTrackGenres(ctx, trackID)
		if err != nil {
			return 0, fmt.Errorf("delete track genres failed: %w", err)
		}
	} else {
		genreID, err := s.getOrCreateMusicGenreID(ctx, qtx, scan, resolved.genreTag)
		if err != nil {
			return 0, fmt.Errorf("genre failed: %w", err)
		}

		err = qtx.DeleteTrackGenresExcept(ctx, database.DeleteTrackGenresExceptParams{
			TrackID: trackID,
			GenreID: genreID,
		})
		if err != nil {
			return 0, fmt.Errorf("delete stale genres failed: %w", err)
		}

		err = qtx.CreateTrackGenre(ctx, database.CreateTrackGenreParams{TrackID: trackID, GenreID: genreID})
		if err != nil {
			return 0, fmt.Errorf("track-genre relationship failed: %w", err)
		}

	}

	err = qtx.SaveMusicTrackMetadata(ctx, database.SaveMusicTrackMetadataParams{TrackID: trackID, ArtistTag: resolved.artistTag, ArtistKey: scanner.NormalizedScanCacheKey(resolved.artistTag), ArtistSort: resolved.artistSort, AlbumSort: resolved.albumSort})
	if err != nil {
		return 0, err
	}
	err = qtx.DeleteMusicCreditMetadata(ctx, trackID)
	if err != nil {
		return 0, err
	}
	for i, input := range resolved.musicians {
		err = qtx.SaveMusicCreditMetadata(ctx, database.SaveMusicCreditMetadataParams{TrackID: trackID, MusicianID: musicianIDs[i], SortName: input.sortName})
		if err != nil {
			return 0, err
		}
	}

	for _, id := range uniqueIDs(oldArtists, musicianIDs) {
		err = qtx.ReconcileMusicArtistSort(ctx, id)
		if err != nil {
			return 0, err
		}
	}
	for _, id := range uniqueIDs(validIDs(oldAlbum, albumID)) {
		err = reconcileAlbum(ctx, qtx, id)
		if err != nil {
			return 0, err
		}
	}
	return trackID, nil
}

// uniqueIDs concatenates id groups in first-sighting order, dropping repeats.
// A rescan of an unchanged track sees the same artist and the same album in
// both the old and the new set, and every Reconcile* statement re-evaluates a
// correlated vote aggregate, so reconciling twice is not free. It also avoids
// appending into a caller's backing array.
func uniqueIDs(groups ...[]int64) []int64 {
	seen := make(map[int64]bool)
	unique := make([]int64, 0)
	for _, group := range groups {
		for _, id := range group {
			if seen[id] {
				continue
			}
			seen[id] = true
			unique = append(unique, id)
		}
	}
	return unique
}

func validIDs(ids ...sql.NullInt64) []int64 {
	present := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id.Valid {
			present = append(present, id.Int64)
		}
	}
	return present
}

func (s *Scanner) persistMusician(ctx context.Context, qtx *database.Queries, scan *musicScanContext, input resolvedMusician) (int64, error) {
	cacheKey := scanner.NormalizedScanCacheKey(input.name)
	existing, lookupErr := qtx.FindMusicArtistIdentity(ctx, cacheKey)
	identityMissing := errors.Is(lookupErr, sql.ErrNoRows)
	if lookupErr == nil {
		input.existingID = existing.ID
		input.hasExistingID = true
		identity := database.GetMusicianBySpotifyIDRow(existing)
		input.existing = &identity
	} else if !identityMissing {
		return 0, lookupErr
	}

	var musician database.GetMusicianBySpotifyIDRow
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
			if input.existing != nil {
				musician = *input.existing
				err = qtx.SetMusicArtistSpotifyID(ctx, database.SetMusicArtistSpotifyIDParams{ID: musician.ID, SpotifyID: spotifyID})
			} else {
				row, writeErr := qtx.UpsertMusician(ctx, database.UpsertMusicianParams{Name: input.name, SortName: input.name, SpotifyID: spotifyID})
				musician = database.GetMusicianBySpotifyIDRow(row)
				err = writeErr
			}
		}

	} else if input.hasExistingID {
		musician.ID = input.existingID
	} else {
		row, writeErr := qtx.UpsertMusician(ctx, database.UpsertMusicianParams{Name: input.name, SortName: input.sortName})
		musician = database.GetMusicianBySpotifyIDRow(row)
		err = writeErr
	}
	if err != nil {
		return 0, err
	}

	if input.hasExistingID && input.existingID != musician.ID {
		err = s.mergeMusicArtist(ctx, qtx, scan, input.existingID, musician.ID)
		if err != nil {
			return 0, err
		}
	}
	err = qtx.SaveMusicArtistIdentity(ctx, database.SaveMusicArtistIdentityParams{IdentityKey: cacheKey, MusicianID: musician.ID})
	if err != nil {
		return 0, err
	}

	if input.spotifyArtist != nil {
		err = qtx.UpdateMusicArtistEnrichment(ctx, database.UpdateMusicArtistEnrichmentParams{ID: musician.ID, Summary: helpers.NullString(generateMusicianSummary(input.spotifyArtist)), SpotifyPopularity: helpers.NullFloat64(float64(input.spotifyArtist.Popularity)), SpotifyFollowers: helpers.NullInt64(int64(input.spotifyArtist.Followers.Count))})
		if err != nil {
			return 0, err
		}
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
	existing, lookupErr := qtx.FindMusicAlbumIdentity(ctx, database.FindMusicAlbumIdentityParams{TitleKey: scanner.NormalizedScanCacheKey(input.title), ArtistKey: scanner.NormalizedScanCacheKey(input.albumArtist)})
	identityMissing := errors.Is(lookupErr, sql.ErrNoRows)
	if lookupErr == nil {
		input.existingID = existing.ID
		input.hasExistingID = true
		identity := database.GetAlbumBySpotifyIDRow(existing)
		input.existing = &identity
	} else if !identityMissing {
		return 0, lookupErr
	}

	var album database.GetAlbumBySpotifyIDRow
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
			if input.existing != nil {
				album = *input.existing
				err = qtx.SetMusicAlbumSpotifyID(ctx, database.SetMusicAlbumSpotifyIDParams{ID: album.ID, SpotifyID: spotifyID})
			} else {
				row, writeErr := qtx.UpsertAlbum(ctx, database.UpsertAlbumParams{Title: input.title, SortTitle: input.title, Musician: helpers.NullString(input.albumArtist), SpotifyID: spotifyID})
				album = database.GetAlbumBySpotifyIDRow(row)
				err = writeErr
			}
		}

	} else if input.hasExistingID {
		album.ID = input.existingID
	} else {
		params := database.UpsertAlbumParams{Title: input.title, SortTitle: input.sortTitle}
		if input.albumArtist != "" {
			params.Musician = sql.NullString{String: input.albumArtist, Valid: true}
		}
		row, writeErr := qtx.UpsertAlbum(ctx, params)
		album = database.GetAlbumBySpotifyIDRow(row)
		err = writeErr
	}
	if err != nil {
		return 0, err
	}

	if input.hasExistingID && input.existingID != album.ID {
		err = s.mergeMusicAlbum(ctx, qtx, scan, input.existingID, album.ID)
		if err != nil {
			return 0, err
		}
	}
	err = qtx.SaveMusicAlbumIdentity(ctx, database.SaveMusicAlbumIdentityParams{TitleKey: scanner.NormalizedScanCacheKey(input.title), ArtistKey: scanner.NormalizedScanCacheKey(input.albumArtist), AlbumID: album.ID})
	if err != nil {
		return 0, err
	}

	if input.spotifyAlbum != nil {
		err = qtx.UpdateMusicAlbumEnrichment(ctx, database.UpdateMusicAlbumEnrichmentParams{ID: album.ID, SpotifyPopularity: helpers.NullFloat64(float64(input.spotifyAlbum.Popularity)), TotalTracks: helpers.NullInt64(int64(input.spotifyAlbum.TotalTracks))})
		if err != nil {
			return 0, err
		}
		album, err = s.updateAlbumCoverIfChanged(ctx, qtx, album, firstImageURL(input.spotifyAlbum.Images))
		if err != nil {
			return 0, err
		}
		date := input.spotifyAlbum.ReleaseDateTime()
		var fallback sql.NullString
		hasDate := !date.IsZero()
		if hasDate {
			fallback = helpers.NullString(date.Format(time.DateOnly))
		}
		err = qtx.SaveMusicAlbumDate(ctx, database.SaveMusicAlbumDateParams{AlbumID: album.ID, SpotifyDate: fallback})
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

func (s *Scanner) updateMusicianThumbIfChanged(ctx context.Context, qtx *database.Queries, musician database.GetMusicianBySpotifyIDRow, thumbURL string) (database.GetMusicianBySpotifyIDRow, error) {
	if thumbURL == "" {
		return musician, nil
	}
	if musician.Thumb.Valid && musician.Thumb.String == thumbURL {
		return musician, nil
	}

	row, err := qtx.UpdateMusicianSpotifyThumb(ctx, database.UpdateMusicianSpotifyThumbParams{
		ID:    musician.ID,
		Thumb: sql.NullString{String: thumbURL, Valid: true},
	})
	return database.GetMusicianBySpotifyIDRow(row), err
}

func (s *Scanner) updateAlbumCoverIfChanged(ctx context.Context, qtx *database.Queries, album database.GetAlbumBySpotifyIDRow, coverURL string) (database.GetAlbumBySpotifyIDRow, error) {
	if coverURL == "" {
		return album, nil
	}
	if album.Cover.Valid && album.Cover.String == coverURL {
		return album, nil
	}

	row, err := qtx.UpdateAlbumSpotifyCover(ctx, database.UpdateAlbumSpotifyCoverParams{
		ID:    album.ID,
		Cover: sql.NullString{String: coverURL, Valid: true},
	})
	return database.GetAlbumBySpotifyIDRow(row), err
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

func reconcileAlbum(ctx context.Context, qtx *database.Queries, id int64) error {
	for _, update := range []func(context.Context, int64) error{qtx.ReconcileMusicAlbumSort, qtx.ReconcileMusicAlbumDate, qtx.ReconcileMusicAlbumYear} {
		err := update(ctx, id)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Scanner) persistMusicians(ctx context.Context, qtx *database.Queries, scan *musicScanContext, inputs []resolvedMusician) ([]int64, error) {
	ids := make([]int64, 0, len(inputs))
	for _, input := range inputs {
		id, err := s.persistMusician(ctx, qtx, scan, input)
		if err != nil {
			return nil, fmt.Errorf("musician failed: %w", err)
		}
		ids = append(ids, id)
	}
	// A later credit can merge an earlier credit's artist into an existing
	// Spotify owner in this same transaction, which leaves the ids collected
	// above pointing at deleted rows -- so re-read the aliases, but only when
	// that actually happened. scan is the transaction-local clone, whose merged
	// flag starts false for every track.
	if !scan.merged {
		return ids, nil
	}
	ids = ids[:0]
	for _, input := range inputs {
		owner, err := qtx.FindMusicArtistIdentity(ctx, scanner.NormalizedScanCacheKey(input.name))
		if err != nil {
			return nil, err
		}
		ids = append(ids, owner.ID)
	}
	return ids, nil
}
