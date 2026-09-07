package music

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/logger"
	"igloo/cmd/internal/scanner"
	spotifyapi "igloo/cmd/internal/spotify"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	spotifylib "github.com/zmb3/spotify/v2"
)

// Dependencies are the application services used by Scanner. DB, Queries,
// Logger and Ffprobe are required. Spotify and shutdown tracking are optional.
type Dependencies struct {
	DB                       *sql.DB
	Queries                  *database.Queries
	Logger                   logger.LoggerInterface
	Ffprobe                  ffprobe.FfprobeInterface
	Spotify                  spotifyapi.SpotifyInterface
	ScanContext              context.Context
	Wait                     *sync.WaitGroup
	ScannerDBMu              *sync.Mutex
	CurrentMusicDirectory    func() sql.NullString
	InvalidateCommittedTrack func(trackID int64)
}

// Scanner scans and persists the configured music library.
type Scanner struct {
	db                       *sql.DB
	queries                  *database.Queries
	logger                   logger.LoggerInterface
	ffprobe                  ffprobe.FfprobeInterface
	spotify                  spotifyapi.SpotifyInterface
	shutdownContext          context.Context
	wait                     *sync.WaitGroup
	scannerDBMu              *sync.Mutex
	currentMusicDirectory    func() sql.NullString
	invalidateCommittedTrack func(int64)
}

// StartStatus describes whether a scan goroutine was launched.
type StartStatus int

const (
	StartStarted StartStatus = iota
	StartNotConfigured
	StartAlreadyRunning
)

// StartResult records the observed directory and start outcome.
type StartResult struct {
	Directory string
	Status    StartStatus
}

// New constructs a scanner with optional directory and invalidation callbacks.
func New(deps Dependencies) *Scanner {
	if deps.ScannerDBMu == nil {
		deps.ScannerDBMu = &sync.Mutex{}
	}
	if deps.CurrentMusicDirectory == nil {
		deps.CurrentMusicDirectory = func() sql.NullString { return sql.NullString{} }
	}
	if deps.InvalidateCommittedTrack == nil {
		deps.InvalidateCommittedTrack = func(int64) {}
	}
	return &Scanner{
		db: deps.DB, queries: deps.Queries, logger: deps.Logger, ffprobe: deps.Ffprobe,
		spotify: deps.Spotify, shutdownContext: deps.ScanContext, wait: deps.Wait,
		scannerDBMu: deps.ScannerDBMu, currentMusicDirectory: deps.CurrentMusicDirectory,
		invalidateCommittedTrack: deps.InvalidateCommittedTrack,
	}
}

// The guard remains process-wide, including across Scanner instances.
var musicScanGuard scanner.ScanGuard

// scanContext returns the shutdown-aware context library scans run under, and
// falls back to a background context when the scanner has none configured.
func (s *Scanner) scanContext() context.Context {
	if s.shutdownContext != nil {
		return s.shutdownContext
	}

	return context.Background()
}

// ---------------------------------------------------------------------------
// Scan orchestration
// ---------------------------------------------------------------------------

// Start launches a scan asynchronously when configured and no music scan is running.
func (s *Scanner) Start() StartResult {
	directory := s.currentMusicDirectory()
	if !directory.Valid || directory.String == "" {
		return StartResult{Status: StartNotConfigured}
	}

	started := musicScanGuard.TryBegin()
	if !started {
		return StartResult{Directory: directory.String, Status: StartAlreadyRunning}
	}

	if s.wait != nil {
		s.wait.Add(1)
	}
	go s.runMusicScan()
	return StartResult{Directory: directory.String, Status: StartStarted}
}

func (s *Scanner) runMusicScan() {
	if s.wait != nil {
		defer s.wait.Done()
	}
	defer musicScanGuard.Finish()

	directory := s.currentMusicDirectory()
	if !directory.Valid || directory.String == "" {
		s.logger.Info("skipping music library scan: music directory is not configured")
		return
	}

	s.logger.Info(fmt.Sprintf("scanning music directory: %s", directory.String))

	ctx := s.scanContext()
	errorCount := 0
	tracksScanned := 0
	tracksSkipped := 0
	startTime := time.Now()
	batch := make([]scanner.ScanFile, 0, scanner.BatchSize)
	scanIndex, err := s.loadMusicScanIndex(ctx)
	if err != nil {
		s.logger.Error(fmt.Sprintf("failed to load music scan index: %s", err.Error()))
		return
	}
	scan := newMusicScanContext(scanIndex)
	flushBatch := func() {
		if len(batch) == 0 {
			return
		}

		scanned, skipped, errors := s.processMusicBatch(ctx, scan, batch)
		tracksScanned += scanned
		tracksSkipped += skipped
		errorCount += errors
		batch = batch[:0]
	}

	err = scanner.WalkMediaLibraryContext(
		ctx,
		directory.String,
		helpers.ValidAudioExtensions,
		func(err error) {
			s.logger.Error(err.Error())
			errorCount++
		},
		func(file scanner.ScanFile) error {
			if scan.trackUnchanged(file.Path, file.Size) {
				tracksSkipped++
				return nil
			}

			batch = append(batch, file)

			if len(batch) >= scanner.BatchSize {
				flushBatch()
			}

			return nil
		},
	)

	if err != nil {
		if errors.Is(err, context.Canceled) {
			s.logger.Info("music library scan canceled")
			return
		}
		s.logger.Error(fmt.Sprintf("unexpected error walking music directory: %s", err.Error()))
		return
	}

	flushBatch()
	contextErr := ctx.Err()
	if contextErr != nil {
		s.logger.Info("music library scan canceled")
		return
	}

	s.logger.Info(fmt.Sprintf("music scanner completed: %d scanned, %d skipped, %d errors in %s",
		tracksScanned, tracksSkipped, errorCount, helpers.FormatDuration(time.Since(startTime))))
}

func (s *Scanner) processMusicBatch(ctx context.Context, scan *musicScanContext, files []scanner.ScanFile) (scanned, skipped, errCount int) {
	for _, file := range files {
		if ctx.Err() != nil {
			return scanned, skipped, errCount
		}

		if scan.trackUnchanged(file.Path, file.Size) {
			skipped++
			continue
		}

		resolved, err := s.resolveTrackFile(ctx, scan, file)
		if err != nil {
			errCount++
			continue
		}

		_, err = s.persistResolvedTrack(ctx, scan, resolved)
		if err != nil {
			s.logger.Warn("failed to persist music track", "path", file.Path, "error", err)
			errCount++
			continue
		}

		scanned++
	}

	return scanned, skipped, errCount
}

func (s *Scanner) loadMusicScanIndex(ctx context.Context) (map[string]int64, error) {
	rows, err := s.queries.ListMusicTrackScanIndex(ctx)
	if err != nil {
		return nil, err
	}

	return scanner.BuildScanIndex(rows, func(row database.ListMusicTrackScanIndexRow) (string, int64) {
		return row.FilePath, row.Size
	}), nil
}

// ---------------------------------------------------------------------------
// Scan context
// ---------------------------------------------------------------------------

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
	musicianIDs scanner.ScanCache[string, int64]
	albumIDs    scanner.ScanCache[string, int64]
	genreIDs    scanner.ScanCache[string, int64]
	// musicianAlbums, musicianGenres, albumGenres and trackGenres remember which
	// join rows this scan already wrote, keyed by musicIDPairKey. Like the id
	// caches they are transaction-scoped until commit.
	musicianAlbums scanner.ScanCache[string, struct{}]
	musicianGenres scanner.ScanCache[string, struct{}]
	albumGenres    scanner.ScanCache[string, struct{}]
	trackGenres    scanner.ScanCache[string, struct{}]
	// spotifyArtistMisses and spotifyAlbumMisses cache unmatched/failed Spotify
	// lookups so a scan queries Spotify at most once per artist or album.
	spotifyArtistMisses scanner.ScanCache[string, resolvedSpotifyMatch]
	spotifyAlbumMisses  scanner.ScanCache[string, resolvedSpotifyMatch]
	// spotifyMusicianGenresHandled and spotifyAlbumGenresHandled record entities
	// whose Spotify genres were fully written, so later tracks skip the work.
	spotifyMusicianGenresHandled scanner.ScanCache[int64, struct{}]
	spotifyAlbumGenresHandled    scanner.ScanCache[int64, struct{}]
}

func newMusicScanContext(trackIndex map[string]int64) *musicScanContext {
	if trackIndex == nil {
		trackIndex = make(map[string]int64)
	}

	// Take ownership of trackIndex: loadMusicScanIndex already cleaned its keys
	// and the caller discards its reference, so no defensive copy is needed.
	return &musicScanContext{
		trackIndex:                   trackIndex,
		musicianIDs:                  scanner.NewScanCache[string, int64](),
		albumIDs:                     scanner.NewScanCache[string, int64](),
		genreIDs:                     scanner.NewScanCache[string, int64](),
		musicianAlbums:               scanner.NewScanCache[string, struct{}](),
		musicianGenres:               scanner.NewScanCache[string, struct{}](),
		albumGenres:                  scanner.NewScanCache[string, struct{}](),
		trackGenres:                  scanner.NewScanCache[string, struct{}](),
		spotifyArtistMisses:          scanner.NewScanCache[string, resolvedSpotifyMatch](),
		spotifyAlbumMisses:           scanner.NewScanCache[string, resolvedSpotifyMatch](),
		spotifyMusicianGenresHandled: scanner.NewScanCache[int64, struct{}](),
		spotifyAlbumGenresHandled:    scanner.NewScanCache[int64, struct{}](),
	}
}

func (scan *musicScanContext) clone() *musicScanContext {
	return &musicScanContext{
		trackIndex:                   scan.trackIndex, // shared; never written inside the transaction
		musicianIDs:                  scan.musicianIDs.Overlay(),
		albumIDs:                     scan.albumIDs.Overlay(),
		genreIDs:                     scan.genreIDs.Overlay(),
		musicianAlbums:               scan.musicianAlbums.Overlay(),
		musicianGenres:               scan.musicianGenres.Overlay(),
		albumGenres:                  scan.albumGenres.Overlay(),
		trackGenres:                  scan.trackGenres.Overlay(),
		spotifyArtistMisses:          scan.spotifyArtistMisses.Overlay(),
		spotifyAlbumMisses:           scan.spotifyAlbumMisses.Overlay(),
		spotifyMusicianGenresHandled: scan.spotifyMusicianGenresHandled.Overlay(),
		spotifyAlbumGenresHandled:    scan.spotifyAlbumGenresHandled.Overlay(),
	}
}

func (scan *musicScanContext) mergeFrom(other *musicScanContext) {
	scan.musicianIDs.MergeFrom(other.musicianIDs)
	scan.albumIDs.MergeFrom(other.albumIDs)
	scan.genreIDs.MergeFrom(other.genreIDs)
	scan.musicianAlbums.MergeFrom(other.musicianAlbums)
	scan.musicianGenres.MergeFrom(other.musicianGenres)
	scan.albumGenres.MergeFrom(other.albumGenres)
	scan.trackGenres.MergeFrom(other.trackGenres)
	scan.spotifyArtistMisses.MergeFrom(other.spotifyArtistMisses)
	scan.spotifyAlbumMisses.MergeFrom(other.spotifyAlbumMisses)
	scan.spotifyMusicianGenresHandled.MergeFrom(other.spotifyMusicianGenresHandled)
	scan.spotifyAlbumGenresHandled.MergeFrom(other.spotifyAlbumGenresHandled)
}

func (scan *musicScanContext) trackUnchanged(path string, size int64) bool {
	return scanner.ScanIndexUnchanged(scan.trackIndex, path, size)
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

func (s *Scanner) resolveTrackFile(ctx context.Context, scan *musicScanContext, file scanner.ScanFile) (*resolvedTrack, error) {
	info, err := s.ffprobe.GetAudioMetadata(ctx, file.Path)
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
	if !shouldSplitCompoundArtistCreditsLocally(credits) {
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
			s.logger.Warn("failed to resolve compound artist part", "part", part, "error", err)
			return nil, fmt.Errorf("compound musician failed for %q: %w", part, err)
		}
		musicians = append(musicians, *musician)
	}

	return musicians, nil
}

func (s *Scanner) resolveMusician(ctx context.Context, scan *musicScanContext, name, sortName string) (*resolvedMusician, error) {
	cacheKey := scanner.NormalizedScanCacheKey(name, sortName)
	if musicianID, ok := scan.musicianIDs.Get(cacheKey); ok {
		return &resolvedMusician{
			name:          name,
			sortName:      sortName,
			existingID:    musicianID,
			hasExistingID: true,
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
			if musicSpotifyMatchStatusIsFinal(persisted.Status) {
				resolved.splitCompoundOnNoMatch = musicSpotifyMatchSplitsCompound(persisted.Status, persisted.Reason)
				scan.musicianIDs.Set(cacheKey, existing.ID)
				return resolved, nil
			}
		} else if !errors.Is(matchErr, sql.ErrNoRows) {
			return nil, matchErr
		}
	}

	spotifyKey := scanner.NormalizedScanCacheKey(name)
	if cachedMiss, ok := scan.spotifyArtistMisses.Get(spotifyKey); ok {
		resolved.spotifyMatch = &cachedMiss
		resolved.splitCompoundOnNoMatch = musicSpotifyMatchSplitsCompound(cachedMiss.status, cachedMiss.reason)
		return resolved, nil
	}

	if s.spotify == nil {
		if found {
			scan.musicianIDs.Set(cacheKey, existing.ID)
		}
		return resolved, nil
	}

	artist, err := s.spotify.SearchArtistByName(ctx, name)
	if err != nil {
		match := resolvedSpotifyMatchFromError(err)
		scan.spotifyArtistMisses.Set(spotifyKey, match)
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

func (s *Scanner) resolveAlbum(ctx context.Context, scan *musicScanContext, title, sortTitle, albumArtist string) (*resolvedAlbum, error) {
	cacheKey := scanner.NormalizedScanCacheKey(title, albumArtist)
	if albumID, ok := scan.albumIDs.Get(cacheKey); ok {
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
			if musicSpotifyMatchStatusIsFinal(persisted.Status) {
				scan.albumIDs.Set(cacheKey, existing.ID)
				return resolved, nil
			}
		} else if !errors.Is(matchErr, sql.ErrNoRows) {
			return nil, matchErr
		}
	}

	spotifyKey := scanner.NormalizedScanCacheKey(title, albumArtist)
	if cachedMiss, ok := scan.spotifyAlbumMisses.Get(spotifyKey); ok {
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
	if err != nil {
		match := resolvedSpotifyMatchFromError(err)
		scan.spotifyAlbumMisses.Set(spotifyKey, match)
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
	if errors.Is(err, sql.ErrNoRows) {
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

			cacheKey := scanner.NormalizedScanCacheKey(part)
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
	scan.trackIndex[filepath.Clean(resolved.filePath)] = resolved.fileSize
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
		if _, exists := seenMusicianIDs[musicianID]; exists {
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
				s.logger.Warn("failed to create musician-album relationship",
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

		err = s.createTrackGenreIfNeeded(ctx, qtx, scan, track.ID, genreID)
		if err != nil {
			return 0, fmt.Errorf("track-genre relationship failed: %w", err)
		}

		for _, musicianID := range musicianIDs {
			err = s.createMusicianGenreIfNeeded(ctx, qtx, scan, musicianID, genreID)
			if err != nil {
				s.logger.Warn("failed to create musician-genre relationship",
					"error", err,
					"musician_id", musicianID,
					"genre_id", genreID,
				)
			}
		}

		if albumID.Valid {
			err = s.createAlbumGenreIfNeeded(ctx, qtx, scan, albumID.Int64, genreID)
			if err != nil {
				s.logger.Warn("failed to create album-genre relationship",
					"error", err,
					"album_id", albumID.Int64,
					"genre_id", genreID,
				)
			}
		}
	}

	return track.ID, nil
}

func (s *Scanner) persistMusician(ctx context.Context, qtx *database.Queries, scan *musicScanContext, input resolvedMusician) (int64, error) {
	cacheKey := scanner.NormalizedScanCacheKey(input.name, input.sortName)
	if musicianID, ok := scan.musicianIDs.Get(cacheKey); ok {
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
			musician, err = s.updateMusicianThumbIfChanged(ctx, qtx, musician, firstImageURL(input.spotifyArtist.Images))
			if err != nil {
				return 0, err
			}
			s.processSpotifyGenres(ctx, qtx, scan, musician.ID, input.spotifyArtist.Genres)
			err = s.upsertMusicSpotifyMatchAndCacheID(ctx, qtx, musicSpotifyEntityMusician, musician.ID, input.spotifyMatch, scan.musicianIDs, cacheKey)
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
		s.processSpotifyGenres(ctx, qtx, scan, musician.ID, input.spotifyArtist.Genres)
		err = s.upsertMusicSpotifyMatchAndCacheID(ctx, qtx, musicSpotifyEntityMusician, musician.ID, input.spotifyMatch, scan.musicianIDs, cacheKey)
		if err != nil {
			return 0, err
		}
		return musician.ID, nil
	}

	if input.hasExistingID {
		err = s.upsertMusicSpotifyMatchAndCacheID(ctx, qtx, musicSpotifyEntityMusician, input.existingID, input.spotifyMatch, scan.musicianIDs, cacheKey)
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

	err = s.upsertMusicSpotifyMatchAndCacheID(ctx, qtx, musicSpotifyEntityMusician, musician.ID, input.spotifyMatch, scan.musicianIDs, cacheKey)
	if err != nil {
		return 0, err
	}

	return musician.ID, nil
}

func (s *Scanner) persistAlbum(ctx context.Context, qtx *database.Queries, scan *musicScanContext, input resolvedAlbum) (int64, error) {
	cacheKey := scanner.NormalizedScanCacheKey(input.title, input.albumArtist)
	if albumID, ok := scan.albumIDs.Get(cacheKey); ok {
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
			album, err = s.updateAlbumCoverIfChanged(ctx, qtx, album, firstImageURL(input.spotifyAlbum.Images))
			if err != nil {
				return 0, err
			}
			s.processSpotifyAlbumGenres(ctx, qtx, scan, album.ID, input.spotifyAlbum.Genres)
			err = s.upsertMusicSpotifyMatchAndCacheID(ctx, qtx, musicSpotifyEntityAlbum, album.ID, input.spotifyMatch, scan.albumIDs, cacheKey)
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
		s.processSpotifyAlbumGenres(ctx, qtx, scan, album.ID, input.spotifyAlbum.Genres)
		err = s.upsertMusicSpotifyMatchAndCacheID(ctx, qtx, musicSpotifyEntityAlbum, album.ID, input.spotifyMatch, scan.albumIDs, cacheKey)
		if err != nil {
			return 0, err
		}
		return album.ID, nil
	}

	if input.hasExistingID {
		err = s.upsertMusicSpotifyMatchAndCacheID(ctx, qtx, musicSpotifyEntityAlbum, input.existingID, input.spotifyMatch, scan.albumIDs, cacheKey)
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

// ---------------------------------------------------------------------------
// Genres and relationships
// ---------------------------------------------------------------------------

func (s *Scanner) processSpotifyGenres(ctx context.Context, qtx *database.Queries, scan *musicScanContext, musicianID int64, spotifyGenres []string) {
	s.processSpotifyEntityGenres(ctx, qtx, scan, musicianID, spotifyGenres, scan.spotifyMusicianGenresHandled, spotifyGenreProcessor{
		getGenreLogMessage:      "failed to get/create Spotify genre",
		relationshipLogMessage:  "failed to create musician-genre relationship for Spotify genre",
		createGenreRelationship: func(genreID int64) error { return s.createMusicianGenreIfNeeded(ctx, qtx, scan, musicianID, genreID) },
		genreRelationshipLogContext: func(genreID int64, genreTag string) []any {
			return []any{"musician_id", musicianID, "genre_id", genreID, "genre", genreTag}
		},
	})
}

func (s *Scanner) processSpotifyAlbumGenres(ctx context.Context, qtx *database.Queries, scan *musicScanContext, albumID int64, spotifyGenres []string) {
	s.processSpotifyEntityGenres(ctx, qtx, scan, albumID, spotifyGenres, scan.spotifyAlbumGenresHandled, spotifyGenreProcessor{
		getGenreLogMessage:      "failed to get/create Spotify genre for album",
		relationshipLogMessage:  "failed to create album-genre relationship for Spotify genre",
		createGenreRelationship: func(genreID int64) error { return s.createAlbumGenreIfNeeded(ctx, qtx, scan, albumID, genreID) },
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

func (s *Scanner) processSpotifyEntityGenres(
	ctx context.Context,
	qtx *database.Queries,
	scan *musicScanContext,
	entityID int64,
	spotifyGenres []string,
	handled scanner.ScanCache[int64, struct{}],
	processor spotifyGenreProcessor,
) {
	if len(spotifyGenres) == 0 {
		return
	}
	if handled.Has(entityID) {
		return
	}

	hadError := false
	for _, genreTag := range spotifyGenres {
		genreID, err := s.getOrCreateMusicGenreID(ctx, qtx, scan, genreTag)
		if err != nil {
			hadError = true
			s.logger.Warn(processor.getGenreLogMessage,
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
			s.logger.Warn(processor.relationshipLogMessage, args...)
		}
	}

	if !hadError {
		handled.Set(entityID, struct{}{})
	}
}

func (s *Scanner) getOrCreateMusicGenreID(ctx context.Context, qtx *database.Queries, scan *musicScanContext, tag string) (int64, error) {
	cacheKey := scanner.NormalizedScanCacheKey(tag, "music")
	if genreID, ok := scan.genreIDs.Get(cacheKey); ok {
		return genreID, nil
	}

	genre, err := qtx.GetOrCreateGenre(ctx, database.GetOrCreateGenreParams{
		Tag:       tag,
		GenreType: "music",
	})
	if err != nil {
		return 0, err
	}

	scan.genreIDs.Set(cacheKey, genre.ID)
	return genre.ID, nil
}

func (s *Scanner) createMusicianAlbumIfNeeded(ctx context.Context, qtx *database.Queries, scan *musicScanContext, musicianID, albumID int64) error {
	return createCachedMusicRelationshipIfNeeded(scan.musicianAlbums, musicianID, albumID, func() error {
		return qtx.CreateMusicianAlbum(ctx, database.CreateMusicianAlbumParams{
			MusicianID: musicianID,
			AlbumID:    albumID,
		})
	})
}

func (s *Scanner) createMusicianGenreIfNeeded(ctx context.Context, qtx *database.Queries, scan *musicScanContext, musicianID, genreID int64) error {
	return createCachedMusicRelationshipIfNeeded(scan.musicianGenres, musicianID, genreID, func() error {
		return qtx.UpsertMusicianGenre(ctx, database.UpsertMusicianGenreParams{
			MusicianID: musicianID,
			GenreID:    genreID,
		})
	})
}

func (s *Scanner) createAlbumGenreIfNeeded(ctx context.Context, qtx *database.Queries, scan *musicScanContext, albumID, genreID int64) error {
	return createCachedMusicRelationshipIfNeeded(scan.albumGenres, albumID, genreID, func() error {
		return qtx.UpsertAlbumGenre(ctx, database.UpsertAlbumGenreParams{
			AlbumID: albumID,
			GenreID: genreID,
		})
	})
}

func (s *Scanner) createTrackGenreIfNeeded(ctx context.Context, qtx *database.Queries, scan *musicScanContext, trackID, genreID int64) error {
	return createCachedMusicRelationshipIfNeeded(scan.trackGenres, trackID, genreID, func() error {
		return qtx.CreateTrackGenre(ctx, database.CreateTrackGenreParams{
			TrackID: trackID,
			GenreID: genreID,
		})
	})
}

func createCachedMusicRelationshipIfNeeded(cache scanner.ScanCache[string, struct{}], leftID, rightID int64, create func() error) error {
	cacheKey := musicIDPairKey(leftID, rightID)
	if cache.Has(cacheKey) {
		return nil
	}

	err := create()
	if err != nil {
		return err
	}

	cache.Set(cacheKey, struct{}{})
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

func (s *Scanner) upsertMusicSpotifyMatchAndCacheID(
	ctx context.Context,
	qtx *database.Queries,
	entityType string,
	entityID int64,
	match *resolvedSpotifyMatch,
	cache scanner.ScanCache[string, int64],
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

	cache.Set(cacheKey, entityID)
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
