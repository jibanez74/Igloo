package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/helpers"
	"maps"
	"path/filepath"
	"strconv"
	"strings"
	"time"
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

	if ctx.Err() != nil {
		app.Logger.Info("music library scan canceled")
		return
	}

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

		resolved, err := app.resolveTrackFile(ctx, file)
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				app.Logger.Error(fmt.Sprintf("failed to process %s: %s", file.Path, err.Error()))
			}
			errCount++
			continue
		}

		_, err = app.persistResolvedTrack(ctx, scan, resolved)
		if err != nil {
			app.Logger.Error("failed to persist music track", "path", file.Path, "error", err)
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
}

func newMusicScanContext(trackIndex map[string]int64) *musicScanContext {
	if trackIndex == nil {
		trackIndex = make(map[string]int64)
	}

	// Take ownership of trackIndex: loadMusicScanIndex already cleaned its keys
	// and the caller discards its reference, so no defensive copy is needed.
	return &musicScanContext{
		trackIndex:     trackIndex,
		musicianIDs:    newScanCache[string, int64](),
		albumIDs:       newScanCache[string, int64](),
		genreIDs:       newScanCache[string, int64](),
		musicianAlbums: newScanCache[string, struct{}](),
		musicianGenres: newScanCache[string, struct{}](),
		albumGenres:    newScanCache[string, struct{}](),
		trackGenres:    newScanCache[string, struct{}](),
	}
}

func (scan *musicScanContext) clone() *musicScanContext {
	return &musicScanContext{
		trackIndex:     scan.trackIndex, // shared; never written inside the transaction
		musicianIDs:    scan.musicianIDs.overlay(),
		albumIDs:       scan.albumIDs.overlay(),
		genreIDs:       scan.genreIDs.overlay(),
		musicianAlbums: scan.musicianAlbums.overlay(),
		musicianGenres: scan.musicianGenres.overlay(),
		albumGenres:    scan.albumGenres.overlay(),
		trackGenres:    scan.trackGenres.overlay(),
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
	name     string
	sortName string
	// nameKey is the normalized identity key (musicians.name_key); persistence
	// and the scan cache key on it, so spelling variants collapse to one row.
	nameKey string
	// mbArtistID is the tag-provided MusicBrainz artist id, set only when the
	// credit resolved to a single artist so a compound credit cannot claim one
	// member's id.
	mbArtistID string
}

type resolvedAlbum struct {
	title       string
	sortTitle   string
	albumArtist string // display string; part of no key
	// albumKey is the tag-derived identity key (albums.album_key).
	albumKey      string
	isCompilation bool
	// albumArtistKey is the normalized first credit of the album artist, used
	// to link albums.album_artist_id when it matches a resolved track musician.
	albumArtistKey   string
	mbReleaseGroupID string
	mbReleaseID      string
	totalTracks      int64
	releaseDate      sql.NullString
	year             sql.NullInt64
}

func (app *Application) resolveTrackFile(ctx context.Context, file helpers.ScanFile) (*resolvedTrack, error) {
	info, err := app.Ffprobe.GetAudioMetadata(ctx, file.Path)
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
		params.Title = strings.TrimSuffix(fileName, filepath.Ext(fileName))
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

		params.Channels = strconv.Itoa(stream.Channels)
		if stream.ChannelLayout != "" {
			params.ChannelLayout = stream.ChannelLayout
		} else {
			params.ChannelLayout = strconv.Itoa(stream.Channels)
		}

		if stream.SampleRate != "" {
			sampleRate, parseErr := strconv.ParseInt(stream.SampleRate, 10, 64)
			if parseErr == nil {
				params.SampleRate = helpers.NullInt64(sampleRate)
			}
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
		resolved.musicians = resolveTrackMusicians(tags.Artist, tags.SortArtist, tags.MbArtistID)
	}

	if tags.Album != "" {
		album, resolveErr := resolveAlbumFromTags(tags, params.ReleaseDate, params.Year)
		if resolveErr != nil {
			return nil, fmt.Errorf("album failed: %w", resolveErr)
		}
		resolved.album = album
	}

	return resolved, nil
}

// resolveTrackMusicians turns an artist credit into entity inputs. Splitting is
// deliberately conservative (see artistCreditDelimiters): the tag MBID is only
// claimed by a single-artist credit, since a compound string's MBID list cannot
// be attributed to one member.
func resolveTrackMusicians(artistTag, sortArtist, artistMBID string) []resolvedMusician {
	credits := splitArtistCredits(artistTag)

	if len(credits) == 1 {
		name := credits[0]
		if sortArtist == "" {
			sortArtist = name
		}
		mbid, _ := helpers.NormalizeMBID(artistMBID)
		return []resolvedMusician{{
			name:       name,
			sortName:   sortArtist,
			nameKey:    normalizedKeyPart(name),
			mbArtistID: mbid,
		}}
	}

	musicians := make([]resolvedMusician, 0, len(credits))
	for _, part := range credits {
		musicians = append(musicians, resolvedMusician{
			name:     part,
			sortName: part,
			nameKey:  normalizedKeyPart(part),
		})
	}

	return musicians
}

func resolveAlbumFromTags(tags ffprobe.FormatTags, releaseDate sql.NullString, year sql.NullInt64) (*resolvedAlbum, error) {
	sortAlbum := tags.SortAlbum
	if sortAlbum == "" {
		sortAlbum = tags.Album
	}

	albumArtist := strings.TrimSpace(tags.AlbumArtist)
	isCompilation := false
	switch {
	case albumArtist != "" && !isVariousArtistsName(albumArtist):
		// A concrete album_artist tag always wins, even over compilation=1:
		// single-artist greatest-hits records are routinely filed under
		// Compilations by iTunes but must stay grouped under their artist.
	case tags.Compilation == "1" || albumArtist != "":
		isCompilation = true
		albumArtist = variousArtistsDisplay
	default:
		albumArtist = strings.TrimSpace(tags.Artist)
	}

	if normalizedKeyPart(tags.Album) == "" {
		return nil, fmt.Errorf("album title %q normalizes to an empty key", tags.Album)
	}

	resolved := &resolvedAlbum{
		title:         tags.Album,
		sortTitle:     sortAlbum,
		albumArtist:   albumArtist,
		albumKey:      albumIdentityKey(tags.Album, albumArtist, isCompilation),
		isCompilation: isCompilation,
		totalTracks:   parseTrackTotal(tags.TotalTracks, tags.Track),
		releaseDate:   releaseDate,
		year:          year,
	}

	if !isCompilation && albumArtist != "" {
		// Keyed on the first credit, mirroring albumIdentityKey, so the FK can
		// match the lead artist when the tag carries a full credit list.
		resolved.albumArtistKey = normalizedKeyPart(firstArtistCredit(albumArtist))
	}

	if mbid, ok := helpers.NormalizeMBID(tags.MbReleaseGroupID); ok {
		resolved.mbReleaseGroupID = mbid
	}
	if mbid, ok := helpers.NormalizeMBID(tags.MbReleaseID); ok {
		resolved.mbReleaseID = mbid
	}

	return resolved, nil
}

// ---------------------------------------------------------------------------
// Identity keys and artist credits
// ---------------------------------------------------------------------------

const (
	albumKeySeparator     = "\x1f"
	variousArtistsDisplay = "Various Artists"
	variousArtistsKeyPart = "various artists"
)

// albumIdentityKey builds the stable identity key for an album row. It is
// derived exclusively from local tags so that a metadata provider can never
// change which tracks belong to which album. The artist part uses only the
// first credit of a compound album-artist string, because taggers write the
// full credit list on some tracks and the lead name on others (cast
// recordings, soundtracks) and both spellings must land on one album.
func albumIdentityKey(title, albumArtist string, isCompilation bool) string {
	artistPart := variousArtistsKeyPart
	if !isCompilation {
		artistPart = normalizedKeyPart(firstArtistCredit(albumArtist))
	}

	return normalizedKeyPart(title) + albumKeySeparator + artistPart
}

// normalizedKeyPart returns the comparison key for a display string, falling
// back to the lowercased raw string for values that normalize to nothing (the
// band "!!!"), so distinct punctuation-only names cannot collapse into one
// empty key.
func normalizedKeyPart(value string) string {
	key := helpers.NormalizeComparisonText(value)
	if key == "" {
		key = strings.ToLower(strings.TrimSpace(value))
	}

	return key
}

func isVariousArtistsName(name string) bool {
	switch helpers.NormalizeComparisonText(name) {
	case "various artists", "various", "va":
		return true
	default:
		return false
	}
}

// artistCreditDelimiters are the unambiguous collaboration markers used to
// split a credit into artist entities. "," and " & " are deliberately absent:
// they appear inside single-act names (Earth, Wind & Fire; Brooks & Dunn), and
// a wrongly-split artist is structurally destructive while an unsplit
// collaboration is only cosmetic. MusicBrainz artist-credit data can refine
// this later; a background scan must not guess.
var artistCreditDelimiters = []string{
	" feat. ", " feat ", " ft. ", " ft ", " featuring ", " with ", " vs. ", " vs ", ";", " / ",
	// Parenthesized collaboration markers ("Beyoncé (feat. JAY-Z)") — the
	// spaced forms above cannot match them because "(" sits where the leading
	// space would be. cleanArtistCreditPart trims the orphaned ")".
	"(feat. ", "(feat ", "(ft. ", "(ft ", "(featuring ", "(with ",
}

// firstArtistCreditDelimiters additionally cuts at "," and " & ". It is used
// only for album identity (albumIdentityKey), never to create artist entities.
var firstArtistCreditDelimiters = append([]string{",", " & "}, artistCreditDelimiters...)

func splitArtistCredits(artistTag string) []string {
	parts := []string{artistTag}
	for _, delimiter := range artistCreditDelimiters {
		split := make([]string, 0, len(parts))
		for _, part := range parts {
			split = append(split, splitASCIIFold(part, delimiter)...)
		}
		parts = split
	}

	seen := make(map[string]struct{}, len(parts))
	credits := make([]string, 0, len(parts))
	for _, part := range parts {
		part = cleanArtistCreditPart(part)
		if part == "" {
			continue
		}
		key := normalizedKeyPart(part)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		credits = append(credits, part)
	}

	return credits
}

// firstArtistCredit returns the leading credit of a compound artist string.
func firstArtistCredit(credit string) string {
	cut := len(credit)
	for _, delimiter := range firstArtistCreditDelimiters {
		if idx := indexASCIIFold(credit, delimiter); idx >= 0 && idx < cut {
			cut = idx
		}
	}

	return cleanArtistCreditPart(credit[:cut])
}

// cleanArtistCreditPart trims whitespace and the stray parenthesis a split
// leaves behind when the delimiter sat inside one ("A (feat. B)"), without
// touching balanced parens that are part of a name.
func cleanArtistCreditPart(part string) string {
	part = strings.TrimSpace(part)
	part = strings.TrimSuffix(part, "(")
	if !strings.Contains(part, "(") {
		part = strings.TrimSuffix(part, ")")
	}

	return strings.TrimSpace(part)
}

// splitASCIIFold splits value on an ASCII delimiter, ignoring the delimiter's
// case. Byte-indexed on purpose: strings.ToLower can change byte offsets for
// some Unicode input, and artist names are arbitrary Unicode.
func splitASCIIFold(value, delimiter string) []string {
	var parts []string
	for {
		idx := indexASCIIFold(value, delimiter)
		if idx < 0 {
			return append(parts, value)
		}
		parts = append(parts, value[:idx])
		value = value[idx+len(delimiter):]
	}
}

// indexASCIIFold reports the first ASCII-case-insensitive occurrence of
// delimiter in value, or -1. delimiter must be ASCII.
func indexASCIIFold(value, delimiter string) int {
	if delimiter == "" || len(value) < len(delimiter) {
		return -1
	}

	for i := 0; i+len(delimiter) <= len(value); i++ {
		match := true
		for j := 0; j < len(delimiter); j++ {
			c := value[i+j]
			if 'A' <= c && c <= 'Z' {
				c += 'a' - 'A'
			}
			if c != delimiter[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}

	return -1
}

// parseTrackTotal extracts an album's track count from the totaltracks tag,
// falling back to the "/total" half of a "1/10"-style track tag.
func parseTrackTotal(totalTag, trackTag string) int64 {
	if totalTag != "" {
		total, err := helpers.ParseSlashNumber(totalTag)
		if err == nil {
			return total
		}
	}

	parts := strings.Split(trackTag, "/")
	if len(parts) == 2 {
		total, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
		if err == nil {
			return total
		}
	}

	return 0
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
	musicianIDsByKey := make(map[string]int64, len(resolved.musicians))

	for _, musicianInput := range resolved.musicians {
		musicianID, err := app.persistMusician(ctx, qtx, scan, musicianInput)
		if err != nil {
			return 0, fmt.Errorf("musician failed: %w", err)
		}
		if !params.MusicianID.Valid {
			params.MusicianID = sql.NullInt64{Int64: musicianID, Valid: true}
		}
		musicianIDsByKey[musicianInput.nameKey] = musicianID
		if _, exists := seenMusicianIDs[musicianID]; exists {
			continue
		}
		seenMusicianIDs[musicianID] = struct{}{}
		musicianIDs = append(musicianIDs, musicianID)
	}

	var albumID sql.NullInt64
	if resolved.album != nil {
		// The album-artist FK is only linked when the album artist is one of
		// the track's resolved musicians; a compound credit list that matches
		// no single artist stays unlinked rather than creating a junk row.
		var albumArtistID sql.NullInt64
		if resolved.album.albumArtistKey != "" {
			if id, ok := musicianIDsByKey[resolved.album.albumArtistKey]; ok {
				albumArtistID = sql.NullInt64{Int64: id, Valid: true}
			}
		}

		id, err := app.persistAlbum(ctx, qtx, scan, *resolved.album, albumArtistID)
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

// persistMusician upserts an artist by identity key. The upsert always runs
// (outside the per-scan cache), so a retagged sort name or a newly tagged MBID
// refreshes on rescan; name stays first-writer-wins in the query itself.
func (app *Application) persistMusician(ctx context.Context, qtx *database.Queries, scan *musicScanContext, input resolvedMusician) (int64, error) {
	if input.nameKey == "" {
		return 0, fmt.Errorf("artist %q resolved to an empty identity key", input.name)
	}

	if musicianID, ok := scan.musicianIDs.get(input.nameKey); ok {
		return musicianID, nil
	}

	musician, err := qtx.UpsertMusician(ctx, database.UpsertMusicianParams{
		Name:       input.name,
		NameKey:    input.nameKey,
		SortName:   input.sortName,
		MbArtistID: helpers.NullString(input.mbArtistID),
	})
	if err != nil {
		return 0, err
	}

	scan.musicianIDs.set(input.nameKey, musician.ID)
	return musician.ID, nil
}

// persistAlbum upserts an album by identity key; see persistMusician for the
// refresh semantics. Display strings (title, musician) are first-writer-wins
// in the query itself.
func (app *Application) persistAlbum(ctx context.Context, qtx *database.Queries, scan *musicScanContext, input resolvedAlbum, albumArtistID sql.NullInt64) (int64, error) {
	if input.albumKey == "" {
		return 0, fmt.Errorf("album %q resolved to an empty identity key", input.title)
	}

	if albumID, ok := scan.albumIDs.get(input.albumKey); ok {
		return albumID, nil
	}

	album, err := qtx.UpsertAlbum(ctx, database.UpsertAlbumParams{
		Title:            input.title,
		SortTitle:        input.sortTitle,
		AlbumKey:         input.albumKey,
		AlbumArtistID:    albumArtistID,
		Musician:         helpers.NullString(input.albumArtist),
		IsCompilation:    input.isCompilation,
		MbReleaseGroupID: helpers.NullString(input.mbReleaseGroupID),
		MbReleaseID:      helpers.NullString(input.mbReleaseID),
		ReleaseDate:      input.releaseDate,
		Year:             input.year,
		TotalTracks:      helpers.NullInt64(input.totalTracks),
	})
	if err != nil {
		return 0, err
	}

	scan.albumIDs.set(input.albumKey, album.ID)
	return album.ID, nil
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
