package music

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/scanner"

	spotifylib "github.com/zmb3/spotify/v2"
)

type resolvedTrack struct {
	inspection *scanner.FileInspection
	params     database.UpsertTrackParams
	musicians  []resolvedMusician
	album      *resolvedAlbum
	genreTag   string
	artistTag  string
	artistSort string
	albumSort  string
}

type resolvedMusician struct {
	name          string
	sortName      string
	existingID    int64
	hasExistingID bool
	// Persistence refreshes this identity inside the transaction because an
	// earlier credit may have merged its owner.
	existing               *database.GetMusicianBySpotifyIDRow
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
	// Persistence refreshes this identity inside the transaction.
	existing     *database.GetAlbumBySpotifyIDRow
	spotifyAlbum *spotifylib.FullAlbum
	spotifyMatch *resolvedSpotifyMatch
}

// resolvedMusicianMatch and resolvedAlbumMatch read the provider outcome off a
// resolution that may be nil, which is how every error path returns.
func resolvedMusicianMatch(resolved *resolvedMusician) *resolvedSpotifyMatch {
	if resolved == nil {
		return nil
	}
	return resolved.spotifyMatch
}

func resolvedAlbumMatch(resolved *resolvedAlbum) *resolvedSpotifyMatch {
	if resolved == nil {
		return nil
	}
	return resolved.spotifyMatch
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
		} else {
			s.logTagParse(file.Path, "duration", info.Format.Duration, parseErr)
		}
	}

	if tags.Track != "" {
		index, parseErr := helpers.ParseSlashNumber(tags.Track)
		if parseErr == nil {
			params.TrackIndex = index
		} else {
			s.logTagParse(file.Path, "track", tags.Track, parseErr)
		}
	}

	if info.Format.BitRate != "" {
		params.BitRate = helpers.ParseBitRate(info.Format.BitRate)
	}

	if tags.Disc != "" {
		disc, parseErr := helpers.ParseSlashNumber(tags.Disc)
		if parseErr == nil {
			params.Disc = disc
		} else {
			s.logTagParse(file.Path, "disc", tags.Disc, parseErr)
		}
	}

	params.Copyright = helpers.NullString(tags.Copyright)
	params.Composer = helpers.NullString(tags.Composer)

	if tags.Date != "" {
		date, parseErr := helpers.ParseDate(tags.Date)
		if parseErr == nil {
			params.ReleaseDate = sql.NullString{String: date.Format("2006-01-02"), Valid: true}
			params.Year = sql.NullInt64{Int64: int64(date.Year()), Valid: true}
		} else {
			s.logTagParse(file.Path, "date", tags.Date, parseErr)
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

		language := strings.TrimSpace(stream.Tags.Language)
		unknown := language == "" || strings.EqualFold(language, "und")
		if unknown {
			language = strings.TrimSpace(tags.Language)
		}
		unknown = strings.EqualFold(language, "und")
		if !unknown {
			params.Language = helpers.NullString(language)
		}

		break
	}

	if !hasAudio {
		return nil, fmt.Errorf("ffprobe returned no audio stream")
	}

	resolved := &resolvedTrack{
		params:    params,
		genreTag:  strings.TrimSpace(tags.Genre),
		artistTag: tags.Artist, artistSort: tags.SortArtist, albumSort: strings.TrimSpace(tags.SortAlbum),
	}

	artistTag := strings.TrimSpace(tags.Artist)
	if artistTag != "" {
		musicians, resolveErr := s.resolveTrackMusicians(ctx, scan, tags.Artist, tags.SortArtist)
		if resolveErr != nil {
			return nil, resolveErr
		}
		resolved.musicians = musicians
	}

	albumTag := strings.TrimSpace(tags.Album)
	if albumTag != "" {
		sortAlbum := tags.SortAlbum
		if sortAlbum == "" {
			sortAlbum = tags.Album
		}

		effectiveAlbumArtist := strings.TrimSpace(tags.AlbumArtist)
		if effectiveAlbumArtist == "" {
			effectiveAlbumArtist = artistTag
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

	credits := parseCompoundArtistCredits(artistTag)
	splitLocally := shouldSplitCompoundArtistCreditsLocally(credits)
	if !splitLocally {
		musician, err := s.resolveMusician(ctx, scan, artistTag, sortArtist)
		if err != nil {
			return nil, fmt.Errorf("musician failed: %w", err)
		}

		if len(credits.parts) < 2 || !musician.splitCompoundOnNoMatch {
			return []resolvedMusician{*musician}, nil
		}
	}

	musicians := make([]resolvedMusician, 0, len(credits.occurrences))
	sortCredits := parseArtistSortCredits(sortArtist, len(credits.occurrences))
	for i, part := range credits.occurrences {
		explicitSort := ""
		if len(sortCredits) == len(credits.occurrences) {
			explicitSort = sortCredits[i]
		}
		musician, err := s.resolveMusician(ctx, scan, part, explicitSort)
		if err != nil {
			return nil, fmt.Errorf("compound musician failed for %q: %w", part, err)
		}
		musicians = append(musicians, *musician)
	}

	return musicians, nil
}

func (s *Scanner) resolveMusician(ctx context.Context, scan *musicScanContext, name, sortName string) (resolved *resolvedMusician, err error) {
	name = strings.TrimSpace(name)
	sortName = strings.TrimSpace(sortName)
	cacheKey := scanner.NormalizedScanCacheKey(name)
	// Tally the outcome on every return path, cached ones included, so the scan
	// summary counts artists rather than Spotify requests. Once the catalog row
	// is known the tally keys on it, so aliases of one artist count once.
	countKey := musicSpotifyEntityMusician + ":" + cacheKey
	defer func() { scan.countEnrichment(countKey, resolvedMusicianMatch(resolved)) }()
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

	resolved = &resolvedMusician{name: name, sortName: sortName}

	// An outcome already decided for this name in this scan short-circuits ahead
	// of the catalog lookups below: persistMusician re-reads the identity inside
	// its own transaction, so the existing* fields are advisory and repeating
	// the two reads per track buys nothing.
	cachedMiss, ok := scan.spotifyArtistMisses[cacheKey]
	if ok {
		resolved.spotifyMatch = &cachedMiss
		resolved.splitCompoundOnNoMatch = musicSpotifyMatchSplitsCompound(cachedMiss.status, cachedMiss.reason)
		scan.compoundSplits[cacheKey] = resolved.splitCompoundOnNoMatch
		return resolved, nil
	}
	attempted, attemptedBefore := scan.artistAttempts[cacheKey]
	if attemptedBefore {
		resolved.spotifyArtist = attempted.spotifyArtist
		resolved.spotifyMatch = attempted.spotifyMatch
		resolved.splitCompoundOnNoMatch = attempted.splitCompoundOnNoMatch
		return resolved, nil
	}

	existing, found, err := s.findExistingMusician(ctx, name)
	if err != nil {
		return nil, err
	}
	if found {
		countKey = musicSpotifyEntityMusician + ":" + strconv.FormatInt(existing.ID, 10)
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

		byID, seen := scan.artistAttemptsByID[existing.ID]
		if seen {
			resolved.spotifyArtist = byID.spotifyArtist
			resolved.spotifyMatch = byID.spotifyMatch
			resolved.splitCompoundOnNoMatch = byID.splitCompoundOnNoMatch
			return resolved, nil
		}
	}

	if s.spotify == nil {
		if found {
			scan.musicianIDs.Set(cacheKey, existing.ID)
		}
		return resolved, nil
	}

	// resolved is the named return, so an error path leaves it nil; never cache
	// that, or the next credit with this name dereferences it.
	defer func() {
		contextErr := ctx.Err()
		if contextErr == nil && resolved != nil {
			scan.artistAttempts[cacheKey] = resolved
			if found {
				scan.artistAttemptsByID[existing.ID] = resolved
			}
		}
	}()
	artist, searchErr := s.spotify.SearchArtistByName(ctx, name)
	contextErr := ctx.Err()
	if contextErr != nil {
		return nil, contextErr
	}
	if searchErr != nil {
		match := resolvedSpotifyMatchFromError(searchErr)
		scan.spotifyArtistMisses[cacheKey] = match
		resolved.spotifyMatch = &match
		resolved.splitCompoundOnNoMatch = shouldSplitCompoundArtistCredits(searchErr)
		scan.compoundSplits[cacheKey] = resolved.splitCompoundOnNoMatch
		return resolved, nil
	}

	if artist != nil {
		resolved.spotifyArtist = artist
		match := resolvedSpotifyMatch{
			status: musicSpotifyStatusMatched,
		}
		resolved.spotifyMatch = &match
	}

	return resolved, nil
}

func (s *Scanner) resolveAlbum(ctx context.Context, scan *musicScanContext, title, sortTitle, albumArtist string) (resolved *resolvedAlbum, err error) {
	title = strings.TrimSpace(title)
	albumArtist = strings.TrimSpace(albumArtist)
	sortTitle = strings.TrimSpace(sortTitle)
	cacheKey := scanner.NormalizedScanCacheKey(title, albumArtist)
	// Same tally as resolveMusician, keyed on the catalog row once it is known.
	countKey := musicSpotifyEntityAlbum + ":" + cacheKey
	defer func() { scan.countEnrichment(countKey, resolvedAlbumMatch(resolved)) }()
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

	resolved = &resolvedAlbum{
		title:       title,
		sortTitle:   sortTitle,
		albumArtist: albumArtist,
	}

	// Same short-circuit as resolveMusician: persistAlbum re-reads the identity
	// inside its transaction, so an outcome already decided for this
	// (title, artist) in this scan skips both catalog reads.
	cachedMiss, ok := scan.spotifyAlbumMisses[cacheKey]
	if ok {
		resolved.spotifyMatch = &cachedMiss
		return resolved, nil
	}
	attempted, attemptedBefore := scan.albumAttempts[cacheKey]
	if attemptedBefore {
		resolved.spotifyAlbum = attempted.spotifyAlbum
		resolved.spotifyMatch = attempted.spotifyMatch

		return resolved, nil
	}

	existing, found, err := s.findExistingAlbum(ctx, title, albumArtist)
	if err != nil {
		return nil, err
	}
	if found {
		countKey = musicSpotifyEntityAlbum + ":" + strconv.FormatInt(existing.ID, 10)
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

		byID, seen := scan.albumAttemptsByID[existing.ID]
		if seen {
			resolved.spotifyAlbum = byID.spotifyAlbum
			resolved.spotifyMatch = byID.spotifyMatch

			return resolved, nil
		}
	}

	if s.spotify == nil {
		if found {
			scan.albumIDs.Set(cacheKey, existing.ID)
		}
		return resolved, nil
	}

	// Same nil guard as resolveMusician.
	defer func() {
		contextErr := ctx.Err()
		if contextErr == nil && resolved != nil {
			scan.albumAttempts[cacheKey] = resolved
			if found {
				scan.albumAttemptsByID[existing.ID] = resolved
			}
		}
	}()
	albumDetails, searchErr := s.spotify.SearchAndGetAlbumDetails(ctx, title, albumArtist)
	contextErr := ctx.Err()
	if contextErr != nil {
		return nil, contextErr
	}
	if searchErr != nil {
		match := resolvedSpotifyMatchFromError(searchErr)
		scan.spotifyAlbumMisses[cacheKey] = match
		resolved.spotifyMatch = &match
		return resolved, nil
	}

	if albumDetails != nil {
		resolved.spotifyAlbum = albumDetails
		match := resolvedSpotifyMatch{
			status: musicSpotifyStatusMatched,
		}
		resolved.spotifyMatch = &match
	}

	return resolved, nil
}

func (s *Scanner) findExistingMusician(ctx context.Context, name string) (database.GetMusicianBySpotifyIDRow, bool, error) {
	musician, err := s.queries.FindMusicArtistIdentity(ctx, scanner.NormalizedScanCacheKey(name))
	if err == nil {
		return database.GetMusicianBySpotifyIDRow(musician), true, nil
	}
	notFound := errors.Is(err, sql.ErrNoRows)
	if notFound {
		return database.GetMusicianBySpotifyIDRow{}, false, nil
	}
	return database.GetMusicianBySpotifyIDRow{}, false, err
}

func (s *Scanner) findExistingAlbum(ctx context.Context, title, albumArtist string) (database.GetAlbumBySpotifyIDRow, bool, error) {
	album, err := s.queries.FindMusicAlbumIdentity(ctx, database.FindMusicAlbumIdentityParams{
		TitleKey:  scanner.NormalizedScanCacheKey(title),
		ArtistKey: scanner.NormalizedScanCacheKey(albumArtist),
	})
	if err == nil {
		return database.GetAlbumBySpotifyIDRow(album), true, nil
	}
	notFound := errors.Is(err, sql.ErrNoRows)
	if notFound {
		return database.GetAlbumBySpotifyIDRow{}, false, nil
	}
	return database.GetAlbumBySpotifyIDRow{}, false, err
}

// logTagParse records a tag the scanner could not read. The field is left at
// its zero value rather than failing the track, so without this the bad tag
// would disappear silently.
func (s *Scanner) logTagParse(path, field, value string, err error) {
	s.logger.Debug("unreadable audio tag", "path", path, "field", field, "value", value, "error", err)
}
