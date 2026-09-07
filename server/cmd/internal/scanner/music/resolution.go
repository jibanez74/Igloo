package music

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/scanner"

	spotifylib "github.com/zmb3/spotify/v2"
)

type resolvedTrack struct {
	params    database.UpsertTrackParams
	musicians []resolvedMusician
	album     *resolvedAlbum
	genreTag  string
}

type resolvedMusician struct {
	name          string
	sortName      string
	existingID    int64
	hasExistingID bool
	// existing carries the row findExistingMusician already fetched, so the
	// Spotify-matched persist path can skip re-reading it when the spotify_id
	// matches.
	existing               *database.Musician
	spotifyArtist          *spotifylib.FullArtist
	spotifyMatch           *resolvedSpotifyMatch
	splitCompoundOnNoMatch bool
}

type resolvedAlbum struct {
	title         string
	sortTitle     string
	albumArtist   string
	existingID    int64
	hasExistingID bool
	// existing carries the row findExistingAlbum already fetched; see
	// resolvedMusician.existing.
	existing     *database.Album
	spotifyAlbum *spotifylib.FullAlbum
	spotifyMatch *resolvedSpotifyMatch
}

func (s *Scanner) resolveTrackFile(ctx context.Context, scan *musicScanContext, file scanner.ScanFile) (*resolvedTrack, error) {
	info, err := s.ffprobe.GetAudioMetadata(ctx, file.Path)
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w", err)
	}

	contextErr := ctx.Err()
	if contextErr != nil {
		return nil, contextErr
	}

	fileName := filepath.Base(file.Path)
	params := database.UpsertTrackParams{
		FilePath: file.Path,
		FileName: fileName,
		Size:     file.Size,
	}

	tags := info.Format.Tags
	if tags.Title != "" {
		params.Title = tags.Title
	} else {
		params.Title = fileName
	}

	if tags.SortName != "" {
		params.SortTitle = tags.SortName
	} else {
		params.SortTitle = params.Title
	}

	params.Container = file.Ext
	mimeType, ok := helpers.AudioMimeTypes[file.Ext]
	if ok {
		params.MimeType = mimeType
	}

	if info.Format.Duration != "" {
		duration, parseErr := helpers.ParseDurationMs(info.Format.Duration)
		if parseErr == nil {
			params.Duration = duration
		}
	}

	if tags.Track != "" {
		index, parseErr := helpers.ParseSlashNumber(tags.Track)
		if parseErr == nil {
			params.TrackIndex = index
		}
	}

	if info.Format.BitRate != "" {
		params.BitRate = helpers.ParseBitRate(info.Format.BitRate)
	}

	if tags.Disc != "" {
		disc, parseErr := helpers.ParseSlashNumber(tags.Disc)
		if parseErr == nil {
			params.Disc = disc
		}
	}

	params.Copyright = helpers.NullString(tags.Copyright)
	params.Composer = helpers.NullString(tags.Composer)

	if tags.Date != "" {
		date, parseErr := helpers.ParseDate(tags.Date)
		if parseErr == nil {
			params.ReleaseDate = sql.NullString{String: date.Format("2006-01-02"), Valid: true}
			params.Year = sql.NullInt64{Int64: int64(date.Year()), Valid: true}
		}
	}

	hasAudio := false
	for _, stream := range info.Streams {
		if stream.CodecType != "audio" {
			continue
		}

		hasAudio = true
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

	if !hasAudio {
		return nil, fmt.Errorf("ffprobe returned no audio stream")
	}

	resolved := &resolvedTrack{
		params:   params,
		genreTag: tags.Genre,
	}

	if tags.Artist != "" {
		musicians, resolveErr := s.resolveTrackMusicians(ctx, scan, tags.Artist, tags.SortArtist)
		if resolveErr != nil {
			return nil, resolveErr
		}
		resolved.musicians = musicians
	}

	if tags.Album != "" {
		sortAlbum := tags.SortAlbum
		if sortAlbum == "" {
			sortAlbum = tags.Album
		}

		effectiveAlbumArtist := tags.AlbumArtist
		if effectiveAlbumArtist == "" {
			effectiveAlbumArtist = tags.Artist
		}

		album, resolveErr := s.resolveAlbum(ctx, scan, tags.Album, sortAlbum, effectiveAlbumArtist)
		if resolveErr != nil {
			return nil, fmt.Errorf("album failed: %w", resolveErr)
		}
		resolved.album = album
	}

	return resolved, nil
}

func (s *Scanner) resolveTrackMusicians(ctx context.Context, scan *musicScanContext, artistTag, sortArtist string) ([]resolvedMusician, error) {
	if sortArtist == "" {
		sortArtist = artistTag
	}

	credits := parseCompoundArtistCredits(artistTag)
	splitLocally := shouldSplitCompoundArtistCreditsLocally(credits)
	if !splitLocally {
		musician, err := s.resolveMusician(ctx, scan, artistTag, sortArtist)
		if err != nil {
			return nil, fmt.Errorf("musician failed: %w", err)
		}

		if len(credits.parts) < 2 || !credits.hasDelimiter || !musician.splitCompoundOnNoMatch {
			return []resolvedMusician{*musician}, nil
		}
	}

	musicians := make([]resolvedMusician, 0, len(credits.parts))
	for _, part := range credits.parts {
		musician, err := s.resolveMusician(ctx, scan, part, part)
		if err != nil {
			return nil, fmt.Errorf("compound musician failed for %q: %w", part, err)
		}
		musicians = append(musicians, *musician)
	}

	return musicians, nil
}

func (s *Scanner) resolveMusician(ctx context.Context, scan *musicScanContext, name, sortName string) (*resolvedMusician, error) {
	cacheKey := scanner.NormalizedScanCacheKey(name, sortName)
	musicianID, ok := scan.musicianIDs.Get(cacheKey)
	if ok {
		return &resolvedMusician{
			name:                   name,
			sortName:               sortName,
			existingID:             musicianID,
			hasExistingID:          true,
			splitCompoundOnNoMatch: scan.compoundSplits[cacheKey],
		}, nil
	}

	resolved := &resolvedMusician{name: name, sortName: sortName}

	existing, found, err := s.findExistingMusician(ctx, name)
	if err != nil {
		return nil, err
	}
	if found {
		resolved.existingID = existing.ID
		resolved.hasExistingID = true
		resolved.existing = &existing

		persisted, matchErr := s.queries.GetMusicSpotifyMatch(ctx, database.GetMusicSpotifyMatchParams{
			EntityType: musicSpotifyEntityMusician,
			EntityID:   existing.ID,
		})
		if matchErr == nil {
			isFinal := musicSpotifyMatchStatusIsFinal(persisted.Status)
			if isFinal {
				resolved.splitCompoundOnNoMatch = musicSpotifyMatchSplitsCompound(persisted.Status, persisted.Reason)
				scan.compoundSplits[cacheKey] = resolved.splitCompoundOnNoMatch
				scan.musicianIDs.Set(cacheKey, existing.ID)
				return resolved, nil
			}
		} else {
			notFound := errors.Is(matchErr, sql.ErrNoRows)
			if !notFound {
				return nil, matchErr
			}
		}
	}

	spotifyKey := scanner.NormalizedScanCacheKey(name)
	cachedMiss, ok := scan.spotifyArtistMisses[spotifyKey]
	if ok {
		resolved.spotifyMatch = &cachedMiss
		resolved.splitCompoundOnNoMatch = musicSpotifyMatchSplitsCompound(cachedMiss.status, cachedMiss.reason)
		scan.compoundSplits[cacheKey] = resolved.splitCompoundOnNoMatch
		return resolved, nil
	}

	if s.spotify == nil {
		if found {
			scan.musicianIDs.Set(cacheKey, existing.ID)
		}
		return resolved, nil
	}

	artist, err := s.spotify.SearchArtistByName(ctx, name)
	contextErr := ctx.Err()
	if contextErr != nil {
		return nil, contextErr
	}
	if err != nil {
		match := resolvedSpotifyMatchFromError(err)
		scan.spotifyArtistMisses[spotifyKey] = match
		resolved.spotifyMatch = &match
		resolved.splitCompoundOnNoMatch = shouldSplitCompoundArtistCredits(err)
		scan.compoundSplits[cacheKey] = resolved.splitCompoundOnNoMatch
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

func (s *Scanner) resolveAlbum(ctx context.Context, scan *musicScanContext, title, sortTitle, albumArtist string) (*resolvedAlbum, error) {
	cacheKey := scanner.NormalizedScanCacheKey(title, albumArtist)
	albumID, ok := scan.albumIDs.Get(cacheKey)
	if ok {
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

	existing, found, err := s.findExistingAlbum(ctx, title, albumArtist)
	if err != nil {
		return nil, err
	}
	if found {
		resolved.existingID = existing.ID
		resolved.hasExistingID = true
		resolved.existing = &existing

		persisted, matchErr := s.queries.GetMusicSpotifyMatch(ctx, database.GetMusicSpotifyMatchParams{
			EntityType: musicSpotifyEntityAlbum,
			EntityID:   existing.ID,
		})
		if matchErr == nil {
			isFinal := musicSpotifyMatchStatusIsFinal(persisted.Status)
			if isFinal {
				scan.albumIDs.Set(cacheKey, existing.ID)
				return resolved, nil
			}
		} else {
			notFound := errors.Is(matchErr, sql.ErrNoRows)
			if !notFound {
				return nil, matchErr
			}
		}
	}

	spotifyKey := scanner.NormalizedScanCacheKey(title, albumArtist)
	cachedMiss, ok := scan.spotifyAlbumMisses[spotifyKey]
	if ok {
		resolved.spotifyMatch = &cachedMiss
		return resolved, nil
	}

	if s.spotify == nil {
		if found {
			scan.albumIDs.Set(cacheKey, existing.ID)
		}
		return resolved, nil
	}

	albumDetails, err := s.spotify.SearchAndGetAlbumDetails(ctx, title, albumArtist)
	contextErr := ctx.Err()
	if contextErr != nil {
		return nil, contextErr
	}
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

func (s *Scanner) findExistingMusician(ctx context.Context, name string) (database.Musician, bool, error) {
	musician, err := s.queries.GetMusicianByName(ctx, name)
	if err == nil {
		return musician, true, nil
	}
	notFound := errors.Is(err, sql.ErrNoRows)
	if notFound {
		return database.Musician{}, false, nil
	}
	return database.Musician{}, false, err
}

func (s *Scanner) findExistingAlbum(ctx context.Context, title, albumArtist string) (database.Album, bool, error) {
	album, err := s.queries.GetAlbumByTitleAndMusician(ctx, database.GetAlbumByTitleAndMusicianParams{
		Title:    title,
		Musician: helpers.NullString(albumArtist),
	})
	if err == nil {
		return album, true, nil
	}
	notFound := errors.Is(err, sql.ErrNoRows)
	if notFound {
		return database.Album{}, false, nil
	}
	return database.Album{}, false, err
}
