package music

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/scanner"
	spotifyapi "igloo/cmd/internal/spotify"
)

type resolvedTrack struct {
	inspection *scanner.FileInspection
	params     database.UpsertTrackParams
	musicians  []resolvedMusician
	album      *resolvedAlbum
	// adoptedArtist is the album artist adopted from the album's match when
	// none of the track's credits carries that name; it is persisted with the
	// track.
	adoptedArtist *resolvedMusician
	genreTag      string
	artistTag     string
	artistSort    string
	albumSort     string
}

type resolvedMusician struct {
	name          string
	sortName      string
	existingID    int64
	hasExistingID bool
	// Persistence refreshes this identity inside the transaction because an
	// earlier credit may have merged its owner.
	existing               *database.GetMusicianBySpotifyIDRow
	spotifyArtist          *spotifyapi.FullArtist
	spotifyMatch           *resolvedMatch
	splitCompoundOnNoMatch bool
	adoptionTarget         string
	adoptionDone           bool
}

type resolvedAlbum struct {
	title         string
	sortTitle     string
	albumArtist   string
	existingID    int64
	hasExistingID bool
	// Persistence refreshes this identity inside the transaction.
	existing     *database.GetAlbumBySpotifyIDRow
	spotifyAlbum *spotifyapi.FullAlbum
	spotifyMatch *resolvedMatch
}

// resolvedMusicianMatch and resolvedAlbumMatch read the provider outcome off a
// resolution that may be nil, which is how every error path returns.
func resolvedMusicianMatch(resolved *resolvedMusician) *resolvedMatch {
	if resolved == nil {
		return nil
	}
	return resolved.spotifyMatch
}

func resolvedAlbumMatch(resolved *resolvedAlbum) *resolvedMatch {
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

	var trackTotal int64
	if tags.Track != "" {
		index, total, parseErr := helpers.ParseSlashNumbers(tags.Track)
		if parseErr == nil {
			params.TrackIndex = index
			trackTotal = total
		} else {
			s.logTagParse(file.Path, "track", tags.Track, parseErr)
		}
	}

	params.BitRate = helpers.ParseBitRate(info.Format.BitRate)

	if tags.Disc != "" {
		disc, _, parseErr := helpers.ParseSlashNumbers(tags.Disc)
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
			params.ReleaseDate = sql.NullString{String: date.Format(time.DateOnly), Valid: true}
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

		channels := stream.ChannelLayout
		if channels == "" {
			channels = strconv.Itoa(stream.Channels)
		}
		params.Channels = channels
		params.ChannelLayout = channels

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

		query := albumQuery{title: tags.Album, sortTitle: sortAlbum, artist: effectiveAlbumArtist, totalTracks: int(trackTotal)}
		if params.Year.Valid {
			query.year = int(params.Year.Int64)
		}
		album, resolveErr := s.resolveAlbum(ctx, scan, query)
		if resolveErr != nil {
			return nil, fmt.Errorf("album failed: %w", resolveErr)
		}
		resolved.album = album
		if album.spotifyAlbum != nil {
			adopted, adoptErr := s.adoptAlbumArtist(ctx, scan, album.albumArtist, album.spotifyAlbum)
			if adoptErr != nil {
				return nil, adoptErr
			}
			if adopted != nil && !applyAdoptedArtist(resolved.musicians, adopted) {
				resolved.adoptedArtist = adopted
			}
		}
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
	countKey := musicEntityMusician + ":" + cacheKey
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
		resolved.takeOutcome(attempted)
		return resolved, nil
	}

	existing, found, err := s.findExistingMusician(ctx, name)
	if err != nil {
		return nil, err
	}
	if found {
		countKey = musicEntityMusician + ":" + strconv.FormatInt(existing.ID, 10)
		resolved.existingID = existing.ID
		resolved.hasExistingID = true
		resolved.existing = &existing

		target, pending, err := s.pendingAdoption(ctx, existing.ID)
		if err != nil {
			return nil, err
		}
		if pending {
			if existing.SpotifyID.Valid {
				resolved.adoptionDone = true
				return resolved, nil
			}
			return s.resolveArtistAdoption(ctx, scan, resolved, target)
		}

		persisted, matchErr := s.queries.GetMusicProviderMatch(ctx, database.GetMusicProviderMatchParams{
			EntityType: musicEntityMusician,
			EntityID:   existing.ID,
		})
		if matchErr == nil {
			isFinal := musicMatchIsFinal(persisted.Status)
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
			resolved.takeOutcome(byID)
			return resolved, nil
		}
	}

	if !s.spotifyReady(scan) {
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
	scan.breaker.Observe(searchErr)
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
		match := resolvedMatch{
			status: musicStatusMatched,
		}
		resolved.spotifyMatch = &match
	}

	return resolved, nil
}

// albumQuery is a local album as its tracks' tags describe it; year and
// totalTracks are zero when untagged.
type albumQuery struct {
	title, sortTitle, artist string
	year, totalTracks        int
}

func (s *Scanner) resolveAlbum(ctx context.Context, scan *musicScanContext, query albumQuery) (resolved *resolvedAlbum, err error) {
	title := strings.TrimSpace(query.title)
	albumArtist := strings.TrimSpace(query.artist)
	sortTitle := strings.TrimSpace(query.sortTitle)
	cacheKey := scanner.NormalizedScanCacheKey(title, albumArtist)
	// Same tally as resolveMusician, keyed on the catalog row once it is known.
	countKey := musicEntityAlbum + ":" + cacheKey
	defer func() { scan.countEnrichment(countKey, resolvedAlbumMatch(resolved)) }()
	// Whichever branch produced it, a resolution carrying the Spotify album
	// leaves its track list for the album's tracks to pair with.
	defer func() {
		if resolved != nil && resolved.spotifyAlbum != nil {
			scan.albumTracks[resolved.spotifyAlbum.ID.String()] = resolved.spotifyAlbum.Tracks
		}
	}()
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
		countKey = musicEntityAlbum + ":" + strconv.FormatInt(existing.ID, 10)
		resolved.existingID = existing.ID
		resolved.hasExistingID = true
		resolved.existing = &existing

		persisted, matchErr := s.queries.GetMusicProviderMatch(ctx, database.GetMusicProviderMatchParams{
			EntityType: musicEntityAlbum,
			EntityID:   existing.ID,
		})
		if matchErr == nil {
			isFinal := musicMatchIsFinal(persisted.Status)
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

	if !s.spotifyReady(scan) {
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
	artistID, err := s.knownArtistSpotifyID(ctx, scan, albumArtist)
	if err != nil {
		return nil, err
	}
	albumDetails, searchErr := s.spotify.MatchAlbum(ctx, spotifyapi.AlbumQuery{
		Title:         title,
		Artist:        albumArtist,
		ArtistCredits: parseCompoundArtistCredits(albumArtist).parts,
		ArtistID:      artistID,
		Year:          query.year,
		TotalTracks:   query.totalTracks,
	})
	contextErr := ctx.Err()
	if contextErr != nil {
		return nil, contextErr
	}
	scan.breaker.Observe(searchErr)
	if searchErr != nil {
		match := resolvedSpotifyMatchFromError(searchErr)
		scan.spotifyAlbumMisses[cacheKey] = match
		resolved.spotifyMatch = &match
		return resolved, nil
	}

	if albumDetails != nil {
		resolved.spotifyAlbum = albumDetails
		match := resolvedMatch{
			status: musicStatusMatched,
		}
		resolved.spotifyMatch = &match
	}

	return resolved, nil
}

// knownArtistSpotifyID is the album artist's Spotify id when this scan or the
// catalog already matched it, and empty otherwise. A name attempted this scan
// and missed has no id, so its catalog row is not re-read.
func (s *Scanner) knownArtistSpotifyID(ctx context.Context, scan *musicScanContext, name string) (spotifyapi.ID, error) {
	if name == "" {
		return "", nil
	}
	attempt, attempted := scan.artistAttempts[scanner.NormalizedScanCacheKey(name)]
	if attempted {
		if attempt.spotifyArtist != nil {
			return attempt.spotifyArtist.ID, nil
		}
		return "", nil
	}
	existing, found, err := s.findExistingMusician(ctx, name)
	if err != nil || !found || !existing.SpotifyID.Valid {
		return "", err
	}
	return spotifyapi.ID(existing.SpotifyID.String), nil
}

// adoptAlbumArtist takes the artist a matched album credits as the local
// album artist of that name, when that artist's own search missed: the album
// match corroborates the name, and the credit carries the id. Compilations
// never qualify, since "Various Artists" agrees with no local name. The
// adopted resolution replaces the scan's record of the name; nil means nothing
// changed.
func (s *Scanner) adoptAlbumArtist(ctx context.Context, scan *musicScanContext, name string, album *spotifyapi.FullAlbum) (*resolvedMusician, error) {
	name = strings.TrimSpace(name)
	var target spotifyapi.ID
	for _, credit := range album.Artists {
		agrees := credit.ID != "" && spotifyapi.ArtistNamesAgree(name, credit.Name)
		if agrees {
			target = credit.ID
			break
		}
	}
	if target == "" {
		return nil, nil
	}
	key := scanner.NormalizedScanCacheKey(name)
	resolved := &resolvedMusician{name: name}
	attempt := scan.artistAttempts[key]
	if attempt != nil {
		if attempt.spotifyArtist != nil {
			return nil, nil
		}
		*resolved = *attempt
	}
	// Re-read aliases: a previous track can have persisted or merged this artist.
	existing, found, err := s.findExistingMusician(ctx, name)
	if err != nil {
		return nil, err
	}
	if found {
		if existing.SpotifyID.Valid {
			return nil, nil
		}
		resolved.existing = &existing
		resolved.existingID, resolved.hasExistingID = existing.ID, true
		pendingTarget, pending, err := s.pendingAdoption(ctx, existing.ID)
		if err != nil {
			return nil, err
		}
		if pending {
			target = pendingTarget
		}
	} else if attempt == nil {
		return nil, nil
	}
	if resolved.adoptionTarget != "" {
		target = spotifyapi.ID(resolved.adoptionTarget)
	}
	return s.resolveArtistAdoption(ctx, scan, resolved, target)
}

// pendingAdoption is the Spotify artist a failed or deferred adoption left
// for the musician to resume with.
func (s *Scanner) pendingAdoption(ctx context.Context, musicianID int64) (spotifyapi.ID, bool, error) {
	target, err := s.queries.GetMusicSpotifyAdoption(ctx, musicianID)
	missing := errors.Is(err, sql.ErrNoRows)
	if missing {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return spotifyapi.ID(target), true, nil
}

type artistIDResult struct {
	artist *spotifyapi.FullArtist
	err    error
}

// A deferred lookup retains its target but creates no provider outcome. Actual
// ID attempts, including failures, are shared by all aliases for this scan.
func (s *Scanner) resolveArtistAdoption(ctx context.Context, scan *musicScanContext, resolved *resolvedMusician, target spotifyapi.ID) (*resolvedMusician, error) {
	resolved.adoptionTarget = target.String()
	result, attempted := scan.artistIDAttempts[target]
	if !attempted && s.spotifyReady(scan) {
		result.artist, result.err = s.spotify.GetArtist(ctx, target)
		contextErr := ctx.Err()
		if contextErr != nil {
			return nil, contextErr
		}
		scan.breaker.Observe(result.err)
		if result.err != nil {
			s.logger.Warn("spotify album artist lookup failed", "artist", resolved.name, "spotify_id", target, "error", result.err)
		}
		scan.artistIDAttempts[target] = result
		attempted = true
	}
	if attempted {
		if result.err == nil && result.artist != nil {
			resolved.spotifyArtist = result.artist
			resolved.spotifyMatch = &resolvedMatch{status: musicStatusMatched}
			resolved.adoptionDone = true
			resolved.splitCompoundOnNoMatch = false
		} else {
			gone := spotifyNotFound(result.err)
			if gone {
				resolved.spotifyMatch = &resolvedMatch{status: musicStatusUnmatched}
				resolved.adoptionDone = true
			} else {
				resolved.spotifyMatch = &resolvedMatch{status: musicStatusFailed}
			}
		}
	}
	key := scanner.NormalizedScanCacheKey(resolved.name)
	scan.artistAttempts[key] = resolved
	delete(scan.spotifyArtistMisses, key)
	scan.compoundSplits[key] = resolved.splitCompoundOnNoMatch
	countKey := musicEntityMusician + ":" + key
	if resolved.hasExistingID {
		scan.artistAttemptsByID[resolved.existingID] = resolved
		countKey = musicEntityMusician + ":" + strconv.FormatInt(resolved.existingID, 10)
	}
	if attempted {
		scan.recountEnrichment(countKey, resolved.spotifyMatch.status)
	}
	return resolved, nil
}

// takeOutcome copies another resolution's provider outcome for the same
// artist, leaving this one's name and catalog identity alone.
func (r *resolvedMusician) takeOutcome(from *resolvedMusician) {
	r.spotifyArtist = from.spotifyArtist
	r.spotifyMatch = from.spotifyMatch
	r.splitCompoundOnNoMatch = from.splitCompoundOnNoMatch
	r.adoptionTarget = from.adoptionTarget
	r.adoptionDone = from.adoptionDone
}

// applyAdoptedArtist copies an adoption onto the credits resolved before it,
// which hold values rather than the scan's pointer, and reports whether any
// credit carries the adopted name.
func applyAdoptedArtist(credits []resolvedMusician, adopted *resolvedMusician) bool {
	key := scanner.NormalizedScanCacheKey(adopted.name)
	applied := false
	for i := range credits {
		if scanner.NormalizedScanCacheKey(credits[i].name) != key {
			continue
		}
		credits[i].takeOutcome(adopted)
		credits[i].splitCompoundOnNoMatch = false
		applied = true
	}
	return applied
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
