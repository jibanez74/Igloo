package main

import (
	"context"
	"database/sql"
	"fmt"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	spotifyapi "igloo/cmd/internal/spotify"
	"path/filepath"
	"strconv"
	"strings"

	spotifylib "github.com/zmb3/spotify/v2"
)

type resolvedTrack struct {
	params    database.UpsertTrackParams
	musicians []resolvedMusician
	album     *resolvedAlbum
	genreTag  string
	filePath  string
	fileSize  int64
}

type resolvedMusician struct {
	name                   string
	sortName               string
	existingID             int64
	hasExistingID          bool
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
	spotifyAlbum  *spotifylib.FullAlbum
	spotifyMatch  *resolvedSpotifyMatch
}

type resolvedSpotifyMatch struct {
	status          string
	spotifyID       sql.NullString
	reason          sql.NullString
	score           sql.NullInt64
	thresholdValue  sql.NullInt64
	candidateName   sql.NullString
	candidateArtist sql.NullString
	searchQuery     sql.NullString
	strategy        sql.NullString
	errorText       sql.NullString
}

func (app *Application) resolveTrackFile(ctx context.Context, scan *musicScanContext, file trackFile) (*resolvedTrack, error) {
	info, err := app.Ffprobe.GetAudioMetadata(file.path)
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w", err)
	}

	params := database.UpsertTrackParams{
		FilePath: file.path,
		FileName: filepath.Base(file.path),
		Size:     file.size,
	}

	tags := info.Format.Tags
	if tags.Title != "" {
		params.Title = tags.Title
	} else {
		params.Title = filepath.Base(file.path)
	}

	if tags.SortName != "" {
		params.SortTitle = tags.SortName
	} else {
		params.SortTitle = params.Title
	}

	params.Container = file.ext
	mimeType, ok := helpers.AudioMimeTypes[file.ext]
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

	for _, stream := range info.Streams {
		if stream.CodecType != "audio" {
			continue
		}

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

	resolved := &resolvedTrack{
		params:   params,
		genreTag: tags.Genre,
		filePath: file.path,
		fileSize: file.size,
	}

	if tags.Artist != "" {
		musicians, resolveErr := app.resolveTrackMusicians(ctx, scan, tags.Artist, tags.SortArtist)
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

		album, resolveErr := app.resolveAlbum(ctx, scan, tags.Album, sortAlbum, effectiveAlbumArtist)
		if resolveErr != nil {
			return nil, fmt.Errorf("album failed: %w", resolveErr)
		}
		resolved.album = album
	}

	return resolved, nil
}

func (app *Application) resolveTrackMusicians(ctx context.Context, scan *musicScanContext, artistTag, sortArtist string) ([]resolvedMusician, error) {
	if sortArtist == "" {
		sortArtist = artistTag
	}

	isCompound := strings.Contains(artistTag, " & ") || strings.Contains(artistTag, ", ")
	splitArtists := false
	var combined *resolvedMusician

	if isCompound {
		musician, err := app.resolveMusician(ctx, scan, artistTag, sortArtist)
		if err != nil {
			return nil, fmt.Errorf("musician failed: %w", err)
		}
		combined = musician
		splitArtists = musician.splitCompoundOnNoMatch
	}

	parts := splitCompoundArtistCredits(artistTag)
	if len(parts) < 2 {
		splitArtists = false
	}

	if !splitArtists {
		if combined != nil {
			return []resolvedMusician{*combined}, nil
		}

		musician, err := app.resolveMusician(ctx, scan, artistTag, sortArtist)
		if err != nil {
			return nil, fmt.Errorf("musician failed: %w", err)
		}
		return []resolvedMusician{*musician}, nil
	}

	musicians := make([]resolvedMusician, 0, len(parts))
	for _, part := range parts {
		musician, err := app.resolveMusician(ctx, scan, part, part)
		if err != nil {
			app.Logger.Warn("failed to resolve compound artist part", "part", part, "error", err)
			return nil, fmt.Errorf("compound musician failed for %q: %w", part, err)
		}
		musicians = append(musicians, *musician)
	}

	return musicians, nil
}

func shouldSplitCompoundArtistCredits(err error) bool {
	matchErr, ok := spotifyapi.AsMatchError(err)
	if !ok {
		return false
	}

	return matchErr.Info.Reason == "no_results" || matchErr.Info.Reason == "score_below_threshold"
}

func splitCompoundArtistCredits(artistTag string) []string {
	normalized := strings.ReplaceAll(artistTag, " & ", ", ")
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
