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
	// trackMatch is the outcome of pairing the track with its album's Spotify
	// track list; nil when the list was not at hand this scan, which the retry
	// pass settles.
	trackMatch *resolvedSpotifyMatch
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
	spotifyArtist          *spotifyapi.FullArtist
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
	spotifyAlbum *spotifyapi.FullAlbum
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
			if adopted != nil {
				applyAdoptedArtist(resolved.musicians, adopted)
			}
		}
		linkTrack(scan, resolved, scanner.NormalizedScanCacheKey(album.title, album.albumArtist))
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
		match := resolvedSpotifyMatch{
			status: musicSpotifyStatusMatched,
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
	countKey := musicSpotifyEntityAlbum + ":" + cacheKey
	defer func() { scan.countEnrichment(countKey, resolvedAlbumMatch(resolved)) }()
	// Whichever branch produced it, a resolution carrying the Spotify album
	// leaves its track list for the album's tracks to pair with.
	defer func() {
		if resolved != nil && resolved.spotifyAlbum != nil {
			scan.albumTracks[cacheKey] = resolved.spotifyAlbum.Tracks
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
		Title:       title,
		Artist:      albumArtist,
		ArtistID:    artistID,
		Year:        query.year,
		TotalTracks: query.totalTracks,
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
		match := resolvedSpotifyMatch{
			status: musicSpotifyStatusMatched,
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
	if name == "" || len(album.Artists) == 0 || !s.spotifyReady(scan) {
		return nil, nil
	}
	credited := album.Artists[0]
	if !spotifyapi.ArtistNamesAgree(name, credited.Name) {
		return nil, nil
	}

	cacheKey := scanner.NormalizedScanCacheKey(name)
	var existing *database.GetMusicianBySpotifyIDRow
	attempt, attempted := scan.artistAttempts[cacheKey]
	if attempted {
		if attempt.spotifyArtist != nil {
			return nil, nil
		}
		existing = attempt.existing
	} else {
		row, found, err := s.findExistingMusician(ctx, name)
		if err != nil {
			return nil, err
		}
		// A name no track credits has no musician to adopt for; one with a
		// Spotify id is already matched.
		if !found || row.SpotifyID.Valid {
			return nil, nil
		}
		existing = &row
	}

	artist, fetchErr := s.spotify.GetArtist(ctx, credited.ID)
	contextErr := ctx.Err()
	if contextErr != nil {
		return nil, contextErr
	}
	scan.breaker.Observe(fetchErr)
	if fetchErr != nil {
		s.logger.Warn("spotify album artist lookup failed", "artist", name, "spotify_id", credited.ID, "error", fetchErr)
		return nil, nil
	}

	match := resolvedSpotifyMatch{status: musicSpotifyStatusMatched}
	adopted := &resolvedMusician{name: name, spotifyArtist: artist, spotifyMatch: &match}
	countKey := musicSpotifyEntityMusician + ":" + cacheKey
	if existing != nil {
		adopted.existingID = existing.ID
		adopted.hasExistingID = true
		adopted.existing = existing
		scan.artistAttemptsByID[existing.ID] = adopted
		countKey = musicSpotifyEntityMusician + ":" + strconv.FormatInt(existing.ID, 10)
	}
	scan.artistAttempts[cacheKey] = adopted
	delete(scan.spotifyArtistMisses, cacheKey)
	scan.compoundSplits[cacheKey] = false
	scan.recountEnrichment(countKey, musicSpotifyStatusMatched)
	return adopted, nil
}

// applyAdoptedArtist copies an adoption onto the credits resolved before it,
// which hold values rather than the scan's pointer.
func applyAdoptedArtist(credits []resolvedMusician, adopted *resolvedMusician) {
	key := scanner.NormalizedScanCacheKey(adopted.name)
	for i := range credits {
		if scanner.NormalizedScanCacheKey(credits[i].name) != key {
			continue
		}
		credits[i].spotifyArtist = adopted.spotifyArtist
		credits[i].spotifyMatch = adopted.spotifyMatch
		credits[i].splitCompoundOnNoMatch = false
	}
}

// trackLink is a pairing with a Spotify track, as applied to the catalog row.
type trackLink struct {
	spotifyID   sql.NullString
	explicit    bool
	trackNumber int64
}

// pairTrack matches one local track against its album's Spotify tracks. The
// link is nil on a miss, which the returned outcome explains.
func pairTrack(query spotifyapi.TrackQuery, tracks []spotifyapi.SimpleTrack) (*trackLink, resolvedSpotifyMatch) {
	track, info := spotifyapi.MatchAlbumTrack(query, tracks)
	if track == nil {
		return nil, resolvedSpotifyMatchFromInfo(info)
	}
	link := &trackLink{
		spotifyID:   sql.NullString{String: track.ID.String(), Valid: true},
		explicit:    track.Explicit,
		trackNumber: int64(track.TrackNumber),
	}
	return link, resolvedSpotifyMatch{status: musicSpotifyStatusMatched}
}

// linkTrack pairs the track with its album's Spotify track list when this
// scan has it. A track whose album matched in an earlier scan stays unpaired
// here; the retry pass fetches the list once and links it. Spotify's track
// number only fills in a missing tag.
func linkTrack(scan *musicScanContext, resolved *resolvedTrack, albumKey string) {
	tracks, ok := scan.albumTracks[albumKey]
	if !ok {
		return
	}
	params := &resolved.params
	query := spotifyapi.TrackQuery{Title: params.Title, Disc: int(params.Disc), Number: int(params.TrackIndex), DurationMs: params.Duration}
	link, match := pairTrack(query, tracks)
	resolved.trackMatch = &match
	if link == nil {
		return
	}
	params.SpotifyID = link.spotifyID
	params.Explicit = link.explicit
	if params.TrackIndex == 0 {
		params.TrackIndex = link.trackNumber
	}
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
