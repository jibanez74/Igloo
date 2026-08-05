package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	spotifyapi "igloo/cmd/internal/spotify"
	"maps"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	spotifylib "github.com/zmb3/spotify/v2"
)

// ---------------------------------------------------------------------------
// Scan orchestration
// ---------------------------------------------------------------------------

func (app *Application) ScanMusicLibrary() {
	if !app.Settings.MusicDir.Valid || app.Settings.MusicDir.String == "" {
		app.Logger.Info("skipping music library scan: music directory is not configured")
		return
	}

	if !musicScanGuard.TryBegin() {
		app.Logger.Warn("music library scan is already in progress")
		return
	}

	if app.Wait != nil {
		app.Wait.Add(1)
	}
	go app.runMusicScan()
}

func (app *Application) runMusicScan() {
	if app.Wait != nil {
		defer app.Wait.Done()
	}
	defer musicScanGuard.Finish()

	if !app.Settings.MusicDir.Valid || app.Settings.MusicDir.String == "" {
		app.Logger.Info("skipping music library scan: music directory is not configured")
		return
	}

	app.Logger.Info(fmt.Sprintf("scanning music directory: %s", app.Settings.MusicDir.String))

	ctx := app.scanContext()
	errorCount := 0
	tracksScanned := 0
	tracksSkipped := 0
	startTime := time.Now()
	batch := make([]helpers.ScanFile, 0, scannerBatchSize)
	scanIndex, err := app.loadMusicScanIndex(ctx)
	if err != nil {
		app.Logger.Error(fmt.Sprintf("failed to load music scan index: %s", err.Error()))
		return
	}
	scan := newMusicScanContext(scanIndex)
	flushBatch := func() {
		if len(batch) == 0 {
			return
		}

		scanned, skipped, errors := app.processMusicBatch(ctx, scan, batch)
		tracksScanned += scanned
		tracksSkipped += skipped
		errorCount += errors
		batch = batch[:0]
	}

	err = helpers.WalkMediaLibraryContext(
		ctx,
		app.Settings.MusicDir.String,
		helpers.ValidAudioExtensions,
		func(err error) {
			app.Logger.Error(err.Error())
			errorCount++
		},
		func(file helpers.ScanFile) error {
			if scan.trackUnchanged(file.Path, file.Size) {
				tracksSkipped++
				return nil
			}

			batch = append(batch, file)

			if len(batch) >= scannerBatchSize {
				flushBatch()
			}

			return nil
		},
	)

	if err != nil {
		if errors.Is(err, context.Canceled) {
			app.Logger.Info("music library scan canceled")
			return
		}
		app.Logger.Error(fmt.Sprintf("unexpected error walking music directory: %s", err.Error()))
		return
	}

	flushBatch()

	app.Logger.Info(fmt.Sprintf("music scanner completed: %d scanned, %d skipped, %d errors in %s",
		tracksScanned, tracksSkipped, errorCount, helpers.FormatDuration(time.Since(startTime))))
}

func (app *Application) processMusicBatch(ctx context.Context, scan *musicScanContext, files []helpers.ScanFile) (scanned, skipped, errCount int) {
	for _, file := range files {
		if ctx.Err() != nil {
			return scanned, skipped, errCount
		}

		if scan.trackUnchanged(file.Path, file.Size) {
			skipped++
			continue
		}

		resolved, err := app.resolveTrackFile(ctx, scan, file)
		if err != nil {
			errCount++
			continue
		}

		_, err = app.persistResolvedTrack(ctx, scan, resolved)
		if err != nil {
			app.Logger.Warn("failed to persist music track", "path", file.Path, "error", err)
			errCount++
			continue
		}

		scanned++
	}

	return scanned, skipped, errCount
}

func (app *Application) loadMusicScanIndex(ctx context.Context) (map[string]int64, error) {
	rows, err := app.Queries.ListMusicTrackScanIndex(ctx)
	if err != nil {
		return nil, err
	}

	return helpers.BuildScanIndex(rows, func(row database.ListMusicTrackScanIndexRow) (string, int64) {
		return row.FilePath, row.Size
	}), nil
}

// ---------------------------------------------------------------------------
// Scan context
// ---------------------------------------------------------------------------

// scanCache is a two-level map: transaction-local entries over an optional
// read-only base layer. The scan-lifetime context uses just the local layer;
// the per-track overlay created by clone() starts empty and reads through to
// the scan layer. Discarding the overlay after a rolled-back transaction
// discards its entries, which used to require cloning every map per track --
// O(tracks x library) -- and now costs only the new entries.
type scanCache[K comparable, V any] struct {
	local map[K]V
	base  map[K]V // nil on the scan-lifetime context
}

func newScanCache[K comparable, V any]() scanCache[K, V] {
	return scanCache[K, V]{local: make(map[K]V)}
}

// overlay returns an empty transaction-local layer over this cache's entries.
// Only meaningful on the scan-lifetime cache (base == nil); overlays are never
// stacked.
func (c scanCache[K, V]) overlay() scanCache[K, V] {
	return scanCache[K, V]{local: make(map[K]V), base: c.local}
}

func (c scanCache[K, V]) get(k K) (V, bool) {
	if v, ok := c.local[k]; ok {
		return v, true
	}
	if c.base != nil {
		if v, ok := c.base[k]; ok {
			return v, true
		}
	}
	var zero V
	return zero, false
}

func (c scanCache[K, V]) has(k K) bool {
	_, ok := c.get(k)
	return ok
}

func (c scanCache[K, V]) set(k K, v V) {
	c.local[k] = v
}

// mergeFrom publishes another cache's local entries into this cache's local
// layer, after the transaction that wrote them committed.
func (c scanCache[K, V]) mergeFrom(other scanCache[K, V]) {
	maps.Copy(c.local, other.local)
}

type musicScanContext struct {
	// trackIndex maps cleaned file path -> file size for every track already in
	// the DB. It is read to skip unchanged files and is only written after a
	// successful commit, never inside a transaction, so it is shared (not copied)
	// across per-track transactions.
	trackIndex map[string]int64
	// musicianIDs, albumIDs and genreIDs memoize entity name/tag -> id within a
	// scan. They are written inside the per-track transaction, so the clone
	// overlay isolates them until commit to avoid caching ids from a rolled-back
	// transaction.
	musicianIDs scanCache[string, int64]
	albumIDs    scanCache[string, int64]
	genreIDs    scanCache[string, int64]
	// musicianAlbums, musicianGenres, albumGenres and trackGenres remember which
	// join rows this scan already wrote, keyed by musicIDPairKey. Like the id
	// caches they are transaction-scoped until commit.
	musicianAlbums scanCache[string, struct{}]
	musicianGenres scanCache[string, struct{}]
	albumGenres    scanCache[string, struct{}]
	trackGenres    scanCache[string, struct{}]
	// spotifyArtistMisses and spotifyAlbumMisses cache unmatched/failed Spotify
	// lookups so a scan queries Spotify at most once per artist or album.
	spotifyArtistMisses scanCache[string, resolvedSpotifyMatch]
	spotifyAlbumMisses  scanCache[string, resolvedSpotifyMatch]
	// spotifyMusicianGenresHandled and spotifyAlbumGenresHandled record entities
	// whose Spotify genres were fully written, so later tracks skip the work.
	spotifyMusicianGenresHandled scanCache[int64, struct{}]
	spotifyAlbumGenresHandled    scanCache[int64, struct{}]
}

func newMusicScanContext(trackIndex map[string]int64) *musicScanContext {
	if trackIndex == nil {
		trackIndex = make(map[string]int64)
	}

	// Take ownership of trackIndex: loadMusicScanIndex already cleaned its keys
	// and the caller discards its reference, so no defensive copy is needed.
	return &musicScanContext{
		trackIndex:                   trackIndex,
		musicianIDs:                  newScanCache[string, int64](),
		albumIDs:                     newScanCache[string, int64](),
		genreIDs:                     newScanCache[string, int64](),
		musicianAlbums:               newScanCache[string, struct{}](),
		musicianGenres:               newScanCache[string, struct{}](),
		albumGenres:                  newScanCache[string, struct{}](),
		trackGenres:                  newScanCache[string, struct{}](),
		spotifyArtistMisses:          newScanCache[string, resolvedSpotifyMatch](),
		spotifyAlbumMisses:           newScanCache[string, resolvedSpotifyMatch](),
		spotifyMusicianGenresHandled: newScanCache[int64, struct{}](),
		spotifyAlbumGenresHandled:    newScanCache[int64, struct{}](),
	}
}

func (scan *musicScanContext) clone() *musicScanContext {
	return &musicScanContext{
		trackIndex:                   scan.trackIndex, // shared; never written inside the transaction
		musicianIDs:                  scan.musicianIDs.overlay(),
		albumIDs:                     scan.albumIDs.overlay(),
		genreIDs:                     scan.genreIDs.overlay(),
		musicianAlbums:               scan.musicianAlbums.overlay(),
		musicianGenres:               scan.musicianGenres.overlay(),
		albumGenres:                  scan.albumGenres.overlay(),
		trackGenres:                  scan.trackGenres.overlay(),
		spotifyArtistMisses:          scan.spotifyArtistMisses.overlay(),
		spotifyAlbumMisses:           scan.spotifyAlbumMisses.overlay(),
		spotifyMusicianGenresHandled: scan.spotifyMusicianGenresHandled.overlay(),
		spotifyAlbumGenresHandled:    scan.spotifyAlbumGenresHandled.overlay(),
	}
}

func (scan *musicScanContext) mergeFrom(other *musicScanContext) {
	scan.musicianIDs.mergeFrom(other.musicianIDs)
	scan.albumIDs.mergeFrom(other.albumIDs)
	scan.genreIDs.mergeFrom(other.genreIDs)
	scan.musicianAlbums.mergeFrom(other.musicianAlbums)
	scan.musicianGenres.mergeFrom(other.musicianGenres)
	scan.albumGenres.mergeFrom(other.albumGenres)
	scan.trackGenres.mergeFrom(other.trackGenres)
	scan.spotifyArtistMisses.mergeFrom(other.spotifyArtistMisses)
	scan.spotifyAlbumMisses.mergeFrom(other.spotifyAlbumMisses)
	scan.spotifyMusicianGenresHandled.mergeFrom(other.spotifyMusicianGenresHandled)
	scan.spotifyAlbumGenresHandled.mergeFrom(other.spotifyAlbumGenresHandled)
}

func (scan *musicScanContext) trackUnchanged(path string, size int64) bool {
	return helpers.ScanIndexUnchanged(scan.trackIndex, path, size)
}

func musicIDPairKey(left, right int64) string {
	return strings.Join([]string{strconv.FormatInt(left, 10), strconv.FormatInt(right, 10)}, "\x00")
}

// ---------------------------------------------------------------------------
// Track resolution (ffprobe tags -> resolved entities)
// ---------------------------------------------------------------------------

type resolvedTrack struct {
	params    database.UpsertTrackParams
	musicians []resolvedMusician
	album     *resolvedAlbum
	genreTag  string
	filePath  string
	fileSize  int64
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

func (app *Application) resolveTrackFile(ctx context.Context, scan *musicScanContext, file helpers.ScanFile) (*resolvedTrack, error) {
	info, err := app.Ffprobe.GetAudioMetadata(file.Path)
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w", err)
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
		filePath: file.Path,
		fileSize: file.Size,
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

	credits := parseCompoundArtistCredits(artistTag)
	if !shouldSplitCompoundArtistCreditsLocally(credits) {
		musician, err := app.resolveMusician(ctx, scan, artistTag, sortArtist)
		if err != nil {
			return nil, fmt.Errorf("musician failed: %w", err)
		}

		if len(credits.parts) < 2 || !credits.hasDelimiter || !musician.splitCompoundOnNoMatch {
			return []resolvedMusician{*musician}, nil
		}
	}

	musicians := make([]resolvedMusician, 0, len(credits.parts))
	for _, part := range credits.parts {
		musician, err := app.resolveMusician(ctx, scan, part, part)
		if err != nil {
			app.Logger.Warn("failed to resolve compound artist part", "part", part, "error", err)
			return nil, fmt.Errorf("compound musician failed for %q: %w", part, err)
		}
		musicians = append(musicians, *musician)
	}

	return musicians, nil
}

func (app *Application) resolveMusician(ctx context.Context, scan *musicScanContext, name, sortName string) (*resolvedMusician, error) {
	cacheKey := helpers.NormalizedScanCacheKey(name, sortName)
	if musicianID, ok := scan.musicianIDs.get(cacheKey); ok {
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
		resolved.existing = &existing

		persisted, matchErr := app.Queries.GetMusicSpotifyMatch(ctx, database.GetMusicSpotifyMatchParams{
			EntityType: musicSpotifyEntityMusician,
			EntityID:   existing.ID,
		})
		if matchErr == nil {
			if musicSpotifyMatchStatusIsFinal(persisted.Status) {
				resolved.splitCompoundOnNoMatch = musicSpotifyMatchSplitsCompound(persisted.Status, persisted.Reason)
				scan.musicianIDs.set(cacheKey, existing.ID)
				return resolved, nil
			}
		} else if !errors.Is(matchErr, sql.ErrNoRows) {
			return nil, matchErr
		}
	}

	spotifyKey := helpers.NormalizedScanCacheKey(name)
	if cachedMiss, ok := scan.spotifyArtistMisses.get(spotifyKey); ok {
		resolved.spotifyMatch = &cachedMiss
		resolved.splitCompoundOnNoMatch = musicSpotifyMatchSplitsCompound(cachedMiss.status, cachedMiss.reason)
		return resolved, nil
	}

	if app.Spotify == nil {
		if found {
			scan.musicianIDs.set(cacheKey, existing.ID)
		}
		return resolved, nil
	}

	artist, err := app.Spotify.SearchArtistByName(ctx, name)
	if err != nil {
		match := resolvedSpotifyMatchFromError(err)
		scan.spotifyArtistMisses.set(spotifyKey, match)
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
	cacheKey := helpers.NormalizedScanCacheKey(title, albumArtist)
	if albumID, ok := scan.albumIDs.get(cacheKey); ok {
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
		resolved.existing = &existing

		persisted, matchErr := app.Queries.GetMusicSpotifyMatch(ctx, database.GetMusicSpotifyMatchParams{
			EntityType: musicSpotifyEntityAlbum,
			EntityID:   existing.ID,
		})
		if matchErr == nil {
			if musicSpotifyMatchStatusIsFinal(persisted.Status) {
				scan.albumIDs.set(cacheKey, existing.ID)
				return resolved, nil
			}
		} else if !errors.Is(matchErr, sql.ErrNoRows) {
			return nil, matchErr
		}
	}

	spotifyKey := helpers.NormalizedScanCacheKey(title, albumArtist)
	if cachedMiss, ok := scan.spotifyAlbumMisses.get(spotifyKey); ok {
		resolved.spotifyMatch = &cachedMiss
		return resolved, nil
	}

	if app.Spotify == nil {
		if found {
			scan.albumIDs.set(cacheKey, existing.ID)
		}
		return resolved, nil
	}

	albumDetails, err := app.Spotify.SearchAndGetAlbumDetails(ctx, title, albumArtist)
	if err != nil {
		match := resolvedSpotifyMatchFromError(err)
		scan.spotifyAlbumMisses.set(spotifyKey, match)
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

// ---------------------------------------------------------------------------
// Compound artist credits
// ---------------------------------------------------------------------------

type compoundArtistCredits struct {
	parts        []string
	hasDelimiter bool
	hasComma     bool
	hasDuplicate bool
}

func parseCompoundArtistCredits(artistTag string) compoundArtistCredits {
	rawCommaParts := strings.Split(artistTag, ",")
	commaParts := make([]string, 0, len(rawCommaParts))

	for _, rawPart := range rawCommaParts {
		part := strings.TrimSpace(rawPart)
		if part == "" {
			continue
		}

		if isArtistSuffix(part) && len(commaParts) > 0 {
			lastIndex := len(commaParts) - 1
			commaParts[lastIndex] = commaParts[lastIndex] + ", " + part
			continue
		}

		commaParts = append(commaParts, part)
	}

	credits := compoundArtistCredits{
		hasDelimiter: strings.Contains(artistTag, " & ") || strings.Contains(artistTag, ","),
		hasComma:     strings.Contains(artistTag, ","),
	}
	seen := make(map[string]struct{}, len(commaParts))

	for _, commaPart := range commaParts {
		ampersandParts := strings.Split(commaPart, " & ")
		for _, rawPart := range ampersandParts {
			part := strings.TrimSpace(rawPart)
			if part == "" {
				continue
			}

			cacheKey := helpers.NormalizedScanCacheKey(part)
			if _, exists := seen[cacheKey]; exists {
				credits.hasDuplicate = true
				continue
			}

			seen[cacheKey] = struct{}{}
			credits.parts = append(credits.parts, part)
		}
	}

	return credits
}

func shouldSplitCompoundArtistCreditsLocally(credits compoundArtistCredits) bool {
	if len(credits.parts) < 2 || !credits.hasComma {
		return false
	}

	if credits.hasDuplicate {
		return true
	}

	for _, part := range credits.parts {
		if len(strings.Fields(part)) < 2 {
			return false
		}
	}

	return true
}

func shouldSplitCompoundArtistCredits(err error) bool {
	matchErr, ok := spotifyapi.AsMatchError(err)
	if !ok {
		return false
	}

	return musicSpotifyReasonSplitsCompound(matchErr.Info.Reason)
}

func isArtistSuffix(value string) bool {
	suffix := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(value), "."))

	switch suffix {
	case "jr", "sr", "ii", "iii", "iv", "v", "vi":
		return true
	default:
		return false
	}
}

// ---------------------------------------------------------------------------
// Persistence
// ---------------------------------------------------------------------------

func (app *Application) persistResolvedTrack(ctx context.Context, scan *musicScanContext, resolved *resolvedTrack) (int64, error) {
	txScan := scan.clone()

	app.ScannerDBMu.Lock()
	defer app.ScannerDBMu.Unlock()

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

	// A rescan can move the file or change its type, so the cached lookup is
	// dropped here, after the new row is committed.
	app.StreamFileCache.invalidate(trackStreamFileKey(trackID))

	// trackIndex is shared (never written inside the transaction) and is only
	// updated here, after a successful commit, so a track whose transaction
	// failed is never recorded as scanned/unchanged.
	scan.trackIndex[filepath.Clean(resolved.filePath)] = resolved.fileSize
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
	cacheKey := helpers.NormalizedScanCacheKey(input.name, input.sortName)
	if musicianID, ok := scan.musicianIDs.get(cacheKey); ok {
		return musicianID, nil
	}

	var musician database.Musician
	var err error

	if input.spotifyArtist != nil {
		spotifyID := sql.NullString{String: input.spotifyArtist.ID.String(), Valid: true}
		if input.existing != nil && input.existing.SpotifyID == spotifyID {
			// The row fetched during resolution is this Spotify artist; no
			// need to read it again. Only this scan writes musicians.
			musician, err = *input.existing, nil
		} else {
			musician, err = qtx.GetMusicianBySpotifyID(ctx, spotifyID)
		}
		if err == nil {
			musician, err = app.updateMusicianThumbIfChanged(ctx, qtx, musician, firstImageURL(input.spotifyArtist.Images))
			if err != nil {
				return 0, err
			}
			app.processSpotifyGenres(ctx, qtx, scan, musician.ID, input.spotifyArtist.Genres)
			err = app.upsertMusicSpotifyMatchAndCacheID(ctx, qtx, musicSpotifyEntityMusician, musician.ID, input.spotifyMatch, scan.musicianIDs, cacheKey)
			if err != nil {
				return 0, err
			}
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
			Thumb:             helpers.NullString(firstImageURL(input.spotifyArtist.Images)),
		}
		musician, err = qtx.UpsertMusician(ctx, params)
		if err != nil {
			return 0, err
		}
		app.processSpotifyGenres(ctx, qtx, scan, musician.ID, input.spotifyArtist.Genres)
		err = app.upsertMusicSpotifyMatchAndCacheID(ctx, qtx, musicSpotifyEntityMusician, musician.ID, input.spotifyMatch, scan.musicianIDs, cacheKey)
		if err != nil {
			return 0, err
		}
		return musician.ID, nil
	}

	if input.hasExistingID {
		err = app.upsertMusicSpotifyMatchAndCacheID(ctx, qtx, musicSpotifyEntityMusician, input.existingID, input.spotifyMatch, scan.musicianIDs, cacheKey)
		if err != nil {
			return 0, err
		}
		return input.existingID, nil
	}

	musician, err = qtx.UpsertMusician(ctx, database.UpsertMusicianParams{
		Name:     input.name,
		SortName: input.sortName,
	})
	if err != nil {
		return 0, err
	}

	err = app.upsertMusicSpotifyMatchAndCacheID(ctx, qtx, musicSpotifyEntityMusician, musician.ID, input.spotifyMatch, scan.musicianIDs, cacheKey)
	if err != nil {
		return 0, err
	}

	return musician.ID, nil
}

func (app *Application) persistAlbum(ctx context.Context, qtx *database.Queries, scan *musicScanContext, input resolvedAlbum) (int64, error) {
	cacheKey := helpers.NormalizedScanCacheKey(input.title, input.albumArtist)
	if albumID, ok := scan.albumIDs.get(cacheKey); ok {
		return albumID, nil
	}

	var album database.Album
	var err error

	if input.spotifyAlbum != nil {
		spotifyID := sql.NullString{String: input.spotifyAlbum.ID.String(), Valid: true}
		if input.existing != nil && input.existing.SpotifyID == spotifyID {
			// See persistMusician: the resolution-phase row is this album.
			album, err = *input.existing, nil
		} else {
			album, err = qtx.GetAlbumBySpotifyID(ctx, spotifyID)
		}
		if err == nil {
			album, err = app.updateAlbumCoverIfChanged(ctx, qtx, album, firstImageURL(input.spotifyAlbum.Images))
			if err != nil {
				return 0, err
			}
			app.processSpotifyAlbumGenres(ctx, qtx, scan, album.ID, input.spotifyAlbum.Genres)
			err = app.upsertMusicSpotifyMatchAndCacheID(ctx, qtx, musicSpotifyEntityAlbum, album.ID, input.spotifyMatch, scan.albumIDs, cacheKey)
			if err != nil {
				return 0, err
			}
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
			Cover:             helpers.NullString(firstImageURL(input.spotifyAlbum.Images)),
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
		err = app.upsertMusicSpotifyMatchAndCacheID(ctx, qtx, musicSpotifyEntityAlbum, album.ID, input.spotifyMatch, scan.albumIDs, cacheKey)
		if err != nil {
			return 0, err
		}
		return album.ID, nil
	}

	if input.hasExistingID {
		err = app.upsertMusicSpotifyMatchAndCacheID(ctx, qtx, musicSpotifyEntityAlbum, input.existingID, input.spotifyMatch, scan.albumIDs, cacheKey)
		if err != nil {
			return 0, err
		}
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

	err = app.upsertMusicSpotifyMatchAndCacheID(ctx, qtx, musicSpotifyEntityAlbum, album.ID, input.spotifyMatch, scan.albumIDs, cacheKey)
	if err != nil {
		return 0, err
	}

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

// ---------------------------------------------------------------------------
// Genres and relationships
// ---------------------------------------------------------------------------

func (app *Application) processSpotifyGenres(ctx context.Context, qtx *database.Queries, scan *musicScanContext, musicianID int64, spotifyGenres []string) {
	app.processSpotifyEntityGenres(ctx, qtx, scan, musicianID, spotifyGenres, scan.spotifyMusicianGenresHandled, spotifyGenreProcessor{
		getGenreLogMessage:      "failed to get/create Spotify genre",
		relationshipLogMessage:  "failed to create musician-genre relationship for Spotify genre",
		createGenreRelationship: func(genreID int64) error { return app.createMusicianGenreIfNeeded(ctx, qtx, scan, musicianID, genreID) },
		genreRelationshipLogContext: func(genreID int64, genreTag string) []any {
			return []any{"musician_id", musicianID, "genre_id", genreID, "genre", genreTag}
		},
	})
}

func (app *Application) processSpotifyAlbumGenres(ctx context.Context, qtx *database.Queries, scan *musicScanContext, albumID int64, spotifyGenres []string) {
	app.processSpotifyEntityGenres(ctx, qtx, scan, albumID, spotifyGenres, scan.spotifyAlbumGenresHandled, spotifyGenreProcessor{
		getGenreLogMessage:      "failed to get/create Spotify genre for album",
		relationshipLogMessage:  "failed to create album-genre relationship for Spotify genre",
		createGenreRelationship: func(genreID int64) error { return app.createAlbumGenreIfNeeded(ctx, qtx, scan, albumID, genreID) },
		genreRelationshipLogContext: func(genreID int64, genreTag string) []any {
			return []any{"album_id", albumID, "genre_id", genreID, "genre", genreTag}
		},
	})
}

type spotifyGenreProcessor struct {
	getGenreLogMessage          string
	relationshipLogMessage      string
	createGenreRelationship     func(genreID int64) error
	genreRelationshipLogContext func(genreID int64, genreTag string) []any
}

func (app *Application) processSpotifyEntityGenres(
	ctx context.Context,
	qtx *database.Queries,
	scan *musicScanContext,
	entityID int64,
	spotifyGenres []string,
	handled scanCache[int64, struct{}],
	processor spotifyGenreProcessor,
) {
	if len(spotifyGenres) == 0 {
		return
	}
	if handled.has(entityID) {
		return
	}

	hadError := false
	for _, genreTag := range spotifyGenres {
		genreID, err := app.getOrCreateMusicGenreID(ctx, qtx, scan, genreTag)
		if err != nil {
			hadError = true
			app.Logger.Warn(processor.getGenreLogMessage,
				"error", err,
				"genre", genreTag,
			)
			continue
		}

		err = processor.createGenreRelationship(genreID)
		if err != nil {
			hadError = true
			args := []any{"error", err}
			args = append(args, processor.genreRelationshipLogContext(genreID, genreTag)...)
			app.Logger.Warn(processor.relationshipLogMessage, args...)
		}
	}

	if !hadError {
		handled.set(entityID, struct{}{})
	}
}

func (app *Application) getOrCreateMusicGenreID(ctx context.Context, qtx *database.Queries, scan *musicScanContext, tag string) (int64, error) {
	cacheKey := helpers.NormalizedScanCacheKey(tag, "music")
	if genreID, ok := scan.genreIDs.get(cacheKey); ok {
		return genreID, nil
	}

	genre, err := qtx.GetOrCreateGenre(ctx, database.GetOrCreateGenreParams{
		Tag:       tag,
		GenreType: "music",
	})
	if err != nil {
		return 0, err
	}

	scan.genreIDs.set(cacheKey, genre.ID)
	return genre.ID, nil
}

func (app *Application) createMusicianAlbumIfNeeded(ctx context.Context, qtx *database.Queries, scan *musicScanContext, musicianID, albumID int64) error {
	return createCachedMusicRelationshipIfNeeded(scan.musicianAlbums, musicianID, albumID, func() error {
		return qtx.CreateMusicianAlbum(ctx, database.CreateMusicianAlbumParams{
			MusicianID: musicianID,
			AlbumID:    albumID,
		})
	})
}

func (app *Application) createMusicianGenreIfNeeded(ctx context.Context, qtx *database.Queries, scan *musicScanContext, musicianID, genreID int64) error {
	return createCachedMusicRelationshipIfNeeded(scan.musicianGenres, musicianID, genreID, func() error {
		return qtx.UpsertMusicianGenre(ctx, database.UpsertMusicianGenreParams{
			MusicianID: musicianID,
			GenreID:    genreID,
		})
	})
}

func (app *Application) createAlbumGenreIfNeeded(ctx context.Context, qtx *database.Queries, scan *musicScanContext, albumID, genreID int64) error {
	return createCachedMusicRelationshipIfNeeded(scan.albumGenres, albumID, genreID, func() error {
		return qtx.UpsertAlbumGenre(ctx, database.UpsertAlbumGenreParams{
			AlbumID: albumID,
			GenreID: genreID,
		})
	})
}

func (app *Application) createTrackGenreIfNeeded(ctx context.Context, qtx *database.Queries, scan *musicScanContext, trackID, genreID int64) error {
	return createCachedMusicRelationshipIfNeeded(scan.trackGenres, trackID, genreID, func() error {
		return qtx.CreateTrackGenre(ctx, database.CreateTrackGenreParams{
			TrackID: trackID,
			GenreID: genreID,
		})
	})
}

func createCachedMusicRelationshipIfNeeded(cache scanCache[string, struct{}], leftID, rightID int64, create func() error) error {
	cacheKey := musicIDPairKey(leftID, rightID)
	if cache.has(cacheKey) {
		return nil
	}

	err := create()
	if err != nil {
		return err
	}

	cache.set(cacheKey, struct{}{})
	return nil
}

// ---------------------------------------------------------------------------
// Spotify match bookkeeping
// ---------------------------------------------------------------------------

const (
	musicSpotifyEntityAlbum               = "album"
	musicSpotifyEntityMusician            = "musician"
	musicSpotifyStatusMatched             = "matched"
	musicSpotifyStatusFailed              = "failed"
	musicSpotifyStatusUnmatched           = "unmatched"
	musicSpotifyReasonNoResults           = "no_results"
	musicSpotifyReasonScoreBelowThreshold = "score_below_threshold"
	musicSpotifyReasonEmpty               = "empty_query"
)

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

func (app *Application) upsertMusicSpotifyMatchAndCacheID(
	ctx context.Context,
	qtx *database.Queries,
	entityType string,
	entityID int64,
	match *resolvedSpotifyMatch,
	cache scanCache[string, int64],
	cacheKey string,
) error {
	if match != nil {
		err := qtx.UpsertMusicSpotifyMatch(ctx, database.UpsertMusicSpotifyMatchParams{
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
		if err != nil {
			return err
		}
	}

	cache.set(cacheKey, entityID)
	return nil
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
	if musicSpotifyReasonIsUnmatched(info.Reason) {
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

func musicSpotifyMatchSplitsCompound(status string, reason sql.NullString) bool {
	if status != musicSpotifyStatusUnmatched || !reason.Valid {
		return false
	}

	return musicSpotifyReasonSplitsCompound(reason.String)
}

func musicSpotifyMatchStatusIsFinal(status string) bool {
	return status == musicSpotifyStatusMatched || status == musicSpotifyStatusUnmatched
}

func musicSpotifyReasonIsUnmatched(reason string) bool {
	return reason == musicSpotifyReasonNoResults || reason == musicSpotifyReasonScoreBelowThreshold || reason == musicSpotifyReasonEmpty
}

func musicSpotifyReasonSplitsCompound(reason string) bool {
	return reason == musicSpotifyReasonNoResults || reason == musicSpotifyReasonScoreBelowThreshold
}

func generateMusicianSummary(artist *spotifylib.FullArtist) string {
	var parts []string

	parts = append(parts, artist.Name)

	if len(artist.Genres) > 0 {
		maxGenres := min(len(artist.Genres), 3)
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

func firstImageURL(images []spotifylib.Image) string {
	if len(images) == 0 {
		return ""
	}

	return images[0].URL
}
