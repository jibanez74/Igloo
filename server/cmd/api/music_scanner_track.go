package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	spotifyapi "igloo/cmd/internal/spotify"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func (app *Application) processTrackFile(ctx context.Context, qtx *database.Queries, path, ext string) error {
	file := trackFile{
		path: path,
		ext:  strings.ToLower(ext),
	}

	fileInfo, err := os.Stat(path)
	if err == nil {
		file.size = fileInfo.Size()
		file.mtime = fileInfo.ModTime().UnixNano()
	}

	return app.processTrackFileWithInfo(ctx, qtx, file)
}

func (app *Application) processTrackFileWithInfo(ctx context.Context, qtx *database.Queries, file trackFile) error {
	info, err := app.Ffprobe.GetMetadata(file.path)
	if err != nil {
		return fmt.Errorf("ffprobe failed: %w", err)
	}

	params := database.UpsertTrackParams{
		FilePath: file.path,
		FileName: filepath.Base(file.path),
	}

	if info.Format.Tags.Title != "" {
		params.Title = info.Format.Tags.Title
	} else {
		params.Title = filepath.Base(file.path)
	}

	if info.Format.Tags.SortName != "" {
		params.SortTitle = info.Format.Tags.SortName
	} else {
		params.SortTitle = params.Title
	}

	params.Container = file.ext

	mimeType, ok := helpers.AudioMimeTypes[file.ext]
	if ok {
		params.MimeType = mimeType
	}

	if file.size > 0 {
		params.Size = file.size
	}
	if info.Format.Size != "" {
		size, err := strconv.ParseInt(info.Format.Size, 10, 64)
		if err == nil {
			params.Size = size
		}
	}

	if info.Format.Duration != "" {
		duration, err := helpers.ParseDurationMs(info.Format.Duration)
		if err == nil {
			params.Duration = duration
		}
	}

	if info.Format.Tags.Track != "" {
		index, err := helpers.ParseSlashNumber(info.Format.Tags.Track)
		if err == nil {
			params.TrackIndex = index
		}
	}

	if info.Format.BitRate != "" {
		params.BitRate = helpers.ParseBitRate(info.Format.BitRate)
	}

	if info.Format.Tags.Disc != "" {
		disc, err := helpers.ParseSlashNumber(info.Format.Tags.Disc)
		if err == nil {
			params.Disc = disc
		}
	}

	params.Copyright = helpers.NullString(info.Format.Tags.Copyright)
	params.Composer = helpers.NullString(info.Format.Tags.Composer)

	trackYear := 0
	if info.Format.Tags.Date != "" {
		date, err := helpers.ParseDate(info.Format.Tags.Date)
		if err == nil {
			params.ReleaseDate = sql.NullString{String: date.Format("2006-01-02"), Valid: true}
			params.Year = sql.NullInt64{Int64: int64(date.Year()), Valid: true}
			trackYear = date.Year()
		}
	}

	var musicianID sql.NullInt64
	var allMusicianIDs []int64
	var albumMusicianIDs []int64

	if info.Format.Tags.Artist != "" {
		sortArtist := info.Format.Tags.SortArtist
		if sortArtist == "" {
			sortArtist = info.Format.Tags.Artist
		}

		artistTag := info.Format.Tags.Artist
		isCompound := compoundArtistCreditLooksSplit(artistTag)

		// Probe compound-looking artist names before creating a possibly bogus combined artist.
		splitArtists := false
		if isCompound && app.Spotify != nil {
			_, err := app.Spotify.SearchArtistByName(ctx, artistTag)
			if err != nil {
				splitArtists = shouldSplitCompoundArtistCredits(err)
			}
		}

		parts := splitCompoundArtistCredits(artistTag)
		if len(parts) < 2 {
			splitArtists = false
		}

		if !splitArtists {
			musician, err := app.getOrCreateMusician(ctx, qtx, artistTag, sortArtist)
			if err != nil {
				return fmt.Errorf("musician failed: %w", err)
			}
			musicianID = sql.NullInt64{Int64: musician.ID, Valid: true}
			allMusicianIDs = appendUniqueInt64(allMusicianIDs, musician.ID)
		} else {
			for _, part := range parts {
				m, err := app.getOrCreateMusician(ctx, qtx, part, part)
				if err != nil {
					app.Logger.Warn("failed to resolve compound artist part", "part", part, "error", err)
					return fmt.Errorf("compound musician failed for %q: %w", part, err)
				}
				if !musicianID.Valid {
					musicianID = sql.NullInt64{Int64: m.ID, Valid: true}
				}
				allMusicianIDs = appendUniqueInt64(allMusicianIDs, m.ID)
			}
		}
	}
	params.MusicianID = musicianID

	var albumID sql.NullInt64

	if info.Format.Tags.Album != "" {
		sortAlbum := info.Format.Tags.SortAlbum
		if sortAlbum == "" {
			sortAlbum = info.Format.Tags.Album
		}

		effectiveAlbumArtist := info.Format.Tags.AlbumArtist
		if effectiveAlbumArtist == "" {
			effectiveAlbumArtist = info.Format.Tags.Artist
		}
		if effectiveAlbumArtist != "" {
			if effectiveAlbumArtist == info.Format.Tags.Artist && musicianID.Valid {
				albumMusicianIDs = appendUniqueInt64(albumMusicianIDs, musicianID.Int64)
			} else {
				albumMusician, err := app.getOrCreateMusician(ctx, qtx, effectiveAlbumArtist, effectiveAlbumArtist)
				if err != nil {
					return fmt.Errorf("album musician failed: %w", err)
				}
				albumMusicianIDs = appendUniqueInt64(albumMusicianIDs, albumMusician.ID)
			}
		}
		for _, mID := range allMusicianIDs {
			albumMusicianIDs = appendUniqueInt64(albumMusicianIDs, mID)
		}

		album, err := app.getOrCreateAlbum(ctx, qtx, albumScanInput{
			Title:            info.Format.Tags.Album,
			SortTitle:        sortAlbum,
			AlbumArtist:      effectiveAlbumArtist,
			Year:             trackYear,
			TrackTitles:      []string{params.Title},
			TrackPaths:       []string{file.path},
			CurrentTrackPath: file.path,
			CurrentMetadata:  info,
		})
		if err != nil {
			return fmt.Errorf("album failed: %w", err)
		}

		albumID = sql.NullInt64{Int64: album.ID, Valid: true}
	}
	params.AlbumID = albumID

	for _, stream := range info.Streams {
		if stream.CodecType == "audio" {
			params.Codec = stream.CodecName
			params.Profile = stream.Profile

			if stream.ChannelLayout != "" {
				params.Channels = stream.ChannelLayout
				params.ChannelLayout = stream.ChannelLayout
			} else {
				params.Channels = strconv.Itoa(stream.Channels)
				params.ChannelLayout = strconv.Itoa(stream.Channels)
			}

			if stream.Tags.Language != "" {
				params.Language = sql.NullString{String: stream.Tags.Language, Valid: true}
			}

			break
		}
	}

	var oldAlbumIDs []int64
	var oldMusicianIDs []int64
	existingTrack, err := qtx.GetTrackByPath(ctx, file.path)
	if err == nil {
		oldAlbumIDs, oldMusicianIDs, err = app.trackRelationshipIDs(ctx, qtx, existingTrack)
		if err != nil {
			return fmt.Errorf("load existing track relationships failed: %w", err)
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("load existing track failed: %w", err)
	}

	track, err := qtx.UpsertTrack(ctx, params)
	if err != nil {
		return fmt.Errorf("upsert track failed: %w", err)
	}

	err = app.replaceTrackMusicians(ctx, qtx, track.ID, allMusicianIDs)
	if err != nil {
		return fmt.Errorf("track musicians failed: %w", err)
	}

	if albumID.Valid {
		for _, mID := range albumMusicianIDs {
			err = qtx.CreateMusicianAlbum(ctx, database.CreateMusicianAlbumParams{
				MusicianID: mID,
				AlbumID:    albumID.Int64,
			})
			if err != nil {
				app.Logger.Warn("failed to create musician-album relationship",
					"error", err,
					"musician_id", mID,
					"album_id", albumID.Int64,
				)
			}
		}
	}

	if info.Format.Tags.Genre != "" {
		genre, err := qtx.GetOrCreateGenre(ctx, database.GetOrCreateGenreParams{
			Tag:       info.Format.Tags.Genre,
			GenreType: "music",
		})

		if err != nil {
			return fmt.Errorf("genre failed: %w", err)
		}

		err = qtx.DeleteTrackGenresExcept(ctx, database.DeleteTrackGenresExceptParams{
			TrackID: track.ID,
			GenreID: genre.ID,
		})

		if err != nil {
			return fmt.Errorf("delete stale genres failed: %w", err)
		}

		err = qtx.CreateTrackGenre(ctx, database.CreateTrackGenreParams{
			TrackID: track.ID,
			GenreID: genre.ID,
		})

		if err != nil {
			return fmt.Errorf("track-genre relationship failed: %w", err)
		}

		if musicianID.Valid {
			err = qtx.UpsertMusicianGenre(ctx, database.UpsertMusicianGenreParams{
				MusicianID: musicianID.Int64,
				GenreID:    genre.ID,
			})
			if err != nil {
				app.Logger.Warn("failed to create musician-genre relationship",
					"error", err,
					"musician_id", musicianID.Int64,
					"genre_id", genre.ID,
				)
			}
		}

		if albumID.Valid {
			err = qtx.UpsertAlbumGenre(ctx, database.UpsertAlbumGenreParams{
				AlbumID: albumID.Int64,
				GenreID: genre.ID,
			})
			if err != nil {
				app.Logger.Warn("failed to create album-genre relationship",
					"error", err,
					"album_id", albumID.Int64,
					"genre_id", genre.ID,
				)
			}
		}
	} else {
		err = qtx.DeleteTrackGenres(ctx, track.ID)
		if err != nil {
			return fmt.Errorf("delete stale genres failed: %w", err)
		}
	}

	if file.mtime > 0 {
		err = qtx.UpsertTrackScanStatus(ctx, database.UpsertTrackScanStatusParams{
			TrackID:   track.ID,
			FilePath:  file.path,
			Size:      file.size,
			FileMtime: file.mtime,
		})
		if err != nil {
			return fmt.Errorf("track scan status failed: %w", err)
		}
	}

	cleanupAlbumIDs := append([]int64{}, oldAlbumIDs...)
	if albumID.Valid {
		cleanupAlbumIDs = appendUniqueInt64(cleanupAlbumIDs, albumID.Int64)
	}

	cleanupMusicianIDs := append([]int64{}, oldMusicianIDs...)
	for _, mID := range allMusicianIDs {
		cleanupMusicianIDs = appendUniqueInt64(cleanupMusicianIDs, mID)
	}
	for _, mID := range albumMusicianIDs {
		cleanupMusicianIDs = appendUniqueInt64(cleanupMusicianIDs, mID)
	}

	return app.cleanupMusicRelationships(ctx, qtx, cleanupAlbumIDs, cleanupMusicianIDs)
}

func (app *Application) trackRelationshipIDs(ctx context.Context, qtx *database.Queries, track database.Track) ([]int64, []int64, error) {
	var albumIDs []int64
	var musicianIDs []int64

	if track.AlbumID.Valid {
		albumIDs = append(albumIDs, track.AlbumID.Int64)
	}
	if track.MusicianID.Valid {
		musicianIDs = appendUniqueInt64(musicianIDs, track.MusicianID.Int64)
	}

	trackMusicianIDs, err := qtx.GetMusicianIDsByTrackID(ctx, track.ID)
	if err != nil {
		return nil, nil, err
	}
	for _, musicianID := range trackMusicianIDs {
		musicianIDs = appendUniqueInt64(musicianIDs, musicianID)
	}

	return albumIDs, musicianIDs, nil
}

func (app *Application) replaceTrackMusicians(ctx context.Context, qtx *database.Queries, trackID int64, musicianIDs []int64) error {
	err := qtx.DeleteTrackMusicians(ctx, trackID)
	if err != nil {
		return err
	}

	for _, musicianID := range dedupeInt64s(musicianIDs) {
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

func (app *Application) cleanupMusicRelationships(ctx context.Context, qtx *database.Queries, albumIDs, musicianIDs []int64) error {
	for _, albumID := range dedupeInt64s(albumIDs) {
		albumMusicianIDs, err := qtx.GetMusicianIDsByAlbumID(ctx, albumID)
		if err != nil {
			return err
		}
		for _, musicianID := range albumMusicianIDs {
			musicianIDs = appendUniqueInt64(musicianIDs, musicianID)
		}

		err = qtx.DeleteAlbumMusiciansWithoutTracks(ctx, albumID)
		if err != nil {
			return err
		}

		err = app.deleteAlbumIfEmpty(ctx, qtx, albumID)
		if err != nil {
			return err
		}
	}

	for _, musicianID := range dedupeInt64s(musicianIDs) {
		err := app.deleteMusicianIfUnused(ctx, qtx, musicianID)
		if err != nil {
			return err
		}
	}

	return nil
}

func appendUniqueInt64(values []int64, value int64) []int64 {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}

	return append(values, value)
}

func dedupeInt64s(values []int64) []int64 {
	out := make([]int64, 0, len(values))
	for _, value := range values {
		out = appendUniqueInt64(out, value)
	}
	return out
}

func shouldSplitCompoundArtistCredits(err error) bool {
	matchErr, ok := spotifyapi.AsMatchError(err)
	if !ok {
		return false
	}

	return matchErr.Info.Reason == "no_results" || matchErr.Info.Reason == "score_below_threshold"
}

func splitCompoundArtistCredits(artistTag string) []string {
	normalized := artistTag
	replacer := strings.NewReplacer(
		" & ", ", ",
		" feat. ", ", ",
		" Feat. ", ", ",
		" FEAT. ", ", ",
		" ft. ", ", ",
		" Ft. ", ", ",
		" FT. ", ", ",
		" featuring ", ", ",
		" Featuring ", ", ",
		" FEATURING ", ", ",
	)
	normalized = replacer.Replace(normalized)
	rawParts := strings.Split(normalized, ", ")
	parts := make([]string, 0, len(rawParts))
	seen := make(map[string]struct{}, len(rawParts))

	for _, rawPart := range rawParts {
		part := strings.TrimSpace(rawPart)
		if part == "" {
			continue
		}

		if _, exists := seen[part]; exists {
			continue
		}

		seen[part] = struct{}{}
		parts = append(parts, part)
	}

	return parts
}

func compoundArtistCreditLooksSplit(artistTag string) bool {
	return strings.Contains(artistTag, " & ") ||
		strings.Contains(artistTag, ", ") ||
		strings.Contains(artistTag, " feat. ") ||
		strings.Contains(artistTag, " Feat. ") ||
		strings.Contains(artistTag, " FEAT. ") ||
		strings.Contains(artistTag, " ft. ") ||
		strings.Contains(artistTag, " Ft. ") ||
		strings.Contains(artistTag, " FT. ") ||
		strings.Contains(artistTag, " featuring ") ||
		strings.Contains(artistTag, " Featuring ") ||
		strings.Contains(artistTag, " FEATURING ")
}
