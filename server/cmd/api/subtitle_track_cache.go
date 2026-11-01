package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"igloo/cmd/internal/ffmpeg"
)

// The subtitle track cache keeps a copy of each subtitle track a TV has
// streamed, as a single-track Matroska file, so later streams of it — every
// seek back inside a session reloads the stream from its start — read a few
// megabytes instead of the whole media file again. Copies live in
// <transcode root>/igloo-subtitle-cache/<kind>/<file id>/ and are named
// <stream index>-<source size>-<source mtime ns>.mkv, so a changed source
// simply misses.
const (
	subtitleTrackCacheDirName = "igloo-subtitle-cache"
	// A copy unused for this long is deleted; serving a copy marks it used.
	subtitleTrackCacheRetention     = 30 * 24 * time.Hour
	subtitleTrackCacheSweepInterval = 24 * time.Hour
	// Copying a track reads its whole media file, which takes minutes for a
	// large remux on slow disks.
	subtitleTrackExtractTimeout = 3 * time.Hour
	subtitleTrackCacheExt       = ".mkv"
	subtitleTrackTempExt        = ".tmp"
)

// subtitleTrackCacheMaxBytes caps a copy, far above any real track — a whole
// film of image subtitles runs to tens of megabytes — so a copy cannot eat the
// space the transcode directory keeps free for HLS sessions. A copy that
// reaches it is discarded. A variable so tests can shrink it.
var subtitleTrackCacheMaxBytes int64 = 512 << 20

// subtitleTrackCacheGate is how long a stream must stay open before its
// track is copied: a TV opens and drops a stream on every start at a
// position, and copying a track only glanced at in a menu wastes a full read
// of the file. A variable so tests can shorten it.
var subtitleTrackCacheGate = 30 * time.Second

// subtitleTrackRef names one subtitle track of one media file.
type subtitleTrackRef struct {
	Kind            mediaKind
	FileID          int64
	StreamIndex     int64
	SourcePath      string
	ConvertToSubRip bool
}

func (ref subtitleTrackRef) String() string {
	return fmt.Sprintf("%s:%d:%d", ref.Kind, ref.FileID, ref.StreamIndex)
}

func subtitleTrackCacheRoot(transcodeRoot string) string {
	return filepath.Join(transcodeRoot, subtitleTrackCacheDirName)
}

func subtitleTrackCacheDir(transcodeRoot string, kind mediaKind, fileID int64) string {
	return filepath.Join(subtitleTrackCacheRoot(transcodeRoot), string(kind), fmt.Sprint(fileID))
}

// subtitleTrackCachePath is where the copy of ref made from the source as
// described by info lives.
func subtitleTrackCachePath(transcodeRoot string, ref subtitleTrackRef, info fs.FileInfo) string {
	name := fmt.Sprintf("%d-%d-%d%s", ref.StreamIndex, info.Size(), info.ModTime().UnixNano(), subtitleTrackCacheExt)
	return filepath.Join(subtitleTrackCacheDir(transcodeRoot, ref.Kind, ref.FileID), name)
}

// cachedSubtitleTrack returns the copy of ref made from its source as it is
// now and marks it used, after deleting copies of the same track made from
// an older version of the source. It returns "" when there is none.
func (app *Application) cachedSubtitleTrack(ref subtitleTrackRef) string {
	transcodeRoot := app.hlsTranscodeRoot()
	if strings.TrimSpace(transcodeRoot) == "" {
		return ""
	}
	info, err := os.Stat(ref.SourcePath)
	if err != nil {
		return ""
	}
	cachePath := subtitleTrackCachePath(transcodeRoot, ref, info)
	removeStaleSubtitleTrackCopies(filepath.Dir(cachePath), ref.StreamIndex, filepath.Base(cachePath))

	cached, err := os.Stat(cachePath)
	if err != nil {
		return ""
	}
	if cached.Size() == 0 || !cached.Mode().IsRegular() {
		return ""
	}
	now := time.Now()
	err = os.Chtimes(cachePath, now, now)
	if err != nil {
		app.Logger.Warn("could not mark cached subtitle track used", "track", ref.String(), "error", err)
	}
	return cachePath
}

// removeStaleSubtitleTrackCopies deletes the copies of one stream in dir
// other than keep: they were made from an older version of the source.
func removeStaleSubtitleTrackCopies(dir string, streamIndex int64, keep string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	prefix := fmt.Sprintf("%d-", streamIndex)
	for _, entry := range entries {
		name := entry.Name()
		isCopyOfStream := strings.HasPrefix(name, prefix) && strings.HasSuffix(name, subtitleTrackCacheExt)
		if isCopyOfStream && name != keep {
			_ = os.Remove(filepath.Join(dir, name))
		}
	}
}

// scheduleSubtitleTrackCache copies ref into the cache once the stream that
// asks for it has stayed open for subtitleTrackCacheGate. It must be called
// from the request goroutine, before the handler returns, so app.Wait counts
// the waiter before shutdown can wait on it.
func (app *Application) scheduleSubtitleTrackCache(streamCtx context.Context, ref subtitleTrackRef) {
	if app.hlsShuttingDown() {
		return
	}
	app.Wait.Go(func() {
		timer := time.NewTimer(subtitleTrackCacheGate)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-streamCtx.Done():
			return
		}
		app.cacheSubtitleTrack(ref)
	})
}

// cacheSubtitleTrack copies ref into the cache unless a copy already
// matches its source. Requests for one track share a run, and one run at a
// time reads a media file server-wide, so full-file reads over a network
// share stay sequential.
func (app *Application) cacheSubtitleTrack(ref subtitleTrackRef) {
	_, _, _ = app.SubtitleTrackCacheGroup.Do(ref.String(), func() (any, error) {
		select {
		case app.SubtitleTrackExtractSlot <- struct{}{}:
		case <-app.HLSShutdown.Done():
			return nil, nil
		}
		defer func() { <-app.SubtitleTrackExtractSlot }()

		app.extractSubtitleTrackToCache(ref)
		return nil, nil
	})
}

func (app *Application) extractSubtitleTrackToCache(ref subtitleTrackRef) {
	transcodeRoot := app.hlsTranscodeRoot()
	if strings.TrimSpace(transcodeRoot) == "" {
		return
	}
	cached := app.cachedSubtitleTrack(ref)
	if cached != "" {
		return
	}
	generation := app.subtitleTrackCacheGeneration()
	before, err := os.Stat(ref.SourcePath)
	if err != nil {
		return
	}
	freeBytes, err := freeDiskBytes(transcodeRoot)
	lowOnSpace := err == nil && freeBytes < hlsMinFreeTranscodeBytes
	if lowOnSpace {
		app.Logger.Warn("not caching subtitle track: transcode directory is nearly full",
			"track", ref.String(), "free_bytes", freeBytes)
		return
	}

	cachePath := subtitleTrackCachePath(transcodeRoot, ref, before)
	err = os.MkdirAll(filepath.Dir(cachePath), 0o755)
	if err != nil {
		app.Logger.Warn("could not create subtitle track cache directory", "track", ref.String(), "error", err)
		return
	}
	temp, err := os.CreateTemp(filepath.Dir(cachePath), fmt.Sprintf("%d-*%s", ref.StreamIndex, subtitleTrackTempExt))
	if err != nil {
		app.Logger.Warn("could not create subtitle track cache file", "track", ref.String(), "error", err)
		return
	}
	tempPath := temp.Name()
	_ = temp.Close()
	defer func() { _ = os.Remove(tempPath) }()

	ctx, cancel := context.WithTimeout(app.HLSShutdown, subtitleTrackExtractTimeout)
	defer cancel()
	startedAt := time.Now()
	err = app.FFmpeg.ExtractSubtitleMatroska(ctx, ffmpeg.SubtitleExtractParams{
		SourcePath:      ref.SourcePath,
		StreamIndex:     ref.StreamIndex,
		ConvertToSubRip: ref.ConvertToSubRip,
		OutputPath:      tempPath,
		MaxBytes:        subtitleTrackCacheMaxBytes,
	})
	if err != nil {
		shuttingDown := app.hlsShuttingDown()
		if !shuttingDown {
			app.Logger.Warn("subtitle track cache extraction failed", "track", ref.String(), "error", err)
		}
		return
	}
	copied, err := os.Stat(tempPath)
	if err != nil {
		return
	}
	if copied.Size() >= subtitleTrackCacheMaxBytes {
		app.Logger.Warn("not caching subtitle track: it is larger than the cache allows",
			"track", ref.String(), "max_bytes", subtitleTrackCacheMaxBytes)
		return
	}

	// A source rewritten while it was read, or a transcode directory moved
	// meanwhile, leaves a copy nobody would look for.
	after, err := os.Stat(ref.SourcePath)
	sourceUnchanged := err == nil && after.Size() == before.Size() && after.ModTime().Equal(before.ModTime())
	rootUnchanged := app.hlsTranscodeRoot() == transcodeRoot
	if !sourceUnchanged || !rootUnchanged {
		return
	}
	// A source replaced without changing its size or modification time is
	// only told apart by the scanner, whose invalidation may already have
	// removed this directory. Holding the lock across the rename means an
	// invalidation either comes first and stops it, or comes after and removes
	// what it published.
	app.SubtitleTrackCacheMu.Lock()
	invalidated := app.SubtitleTrackCacheGeneration != generation
	if !invalidated {
		err = os.Rename(tempPath, cachePath)
	}
	app.SubtitleTrackCacheMu.Unlock()
	if invalidated {
		return
	}
	if err != nil {
		app.Logger.Warn("could not publish cached subtitle track", "track", ref.String(), "error", err)
		return
	}

	size := int64(0)
	published, statErr := os.Stat(cachePath)
	if statErr == nil {
		size = published.Size()
	}
	app.Logger.Info("subtitle track cached",
		"track", ref.String(),
		"bytes", size,
		"duration_ms", time.Since(startedAt).Milliseconds(),
	)
}

func (app *Application) subtitleTrackCacheGeneration() uint64 {
	app.SubtitleTrackCacheMu.Lock()
	defer app.SubtitleTrackCacheMu.Unlock()
	return app.SubtitleTrackCacheGeneration
}

// advanceSubtitleTrackCacheGeneration stops every copy being made from being
// published. It must run before the removal it announces is scheduled.
func (app *Application) advanceSubtitleTrackCacheGeneration() {
	app.SubtitleTrackCacheMu.Lock()
	defer app.SubtitleTrackCacheMu.Unlock()
	app.SubtitleTrackCacheGeneration++
}

// invalidateSubtitleTrackCache deletes every cached track of one media file,
// and keeps a copy being made of any track from being published, since it may
// have been read from the old file. The deletion runs off the caller's
// goroutine because the scanner calls it holding its lock.
func (app *Application) invalidateSubtitleTrackCache(kind mediaKind, fileID int64) {
	transcodeRoot := app.hlsTranscodeRoot()
	if strings.TrimSpace(transcodeRoot) == "" {
		return
	}
	app.advanceSubtitleTrackCacheGeneration()
	dir := subtitleTrackCacheDir(transcodeRoot, kind, fileID)
	app.Wait.Go(func() {
		err := os.RemoveAll(dir)
		if err != nil {
			app.Logger.Warn("could not remove cached subtitle tracks", "path", dir, "error", err)
		}
	})
}

// removeSubtitleTrackCacheRoot deletes the cache a previous transcode root
// held; nothing reads it once the root has moved.
func (app *Application) removeSubtitleTrackCacheRoot(transcodeRoot string) {
	if strings.TrimSpace(transcodeRoot) == "" {
		return
	}
	app.advanceSubtitleTrackCacheGeneration()
	root := subtitleTrackCacheRoot(transcodeRoot)
	app.Wait.Go(func() {
		err := os.RemoveAll(root)
		if err != nil {
			app.Logger.Warn("could not remove the old subtitle track cache", "path", root, "error", err)
		}
	})
}

// runSubtitleTrackCacheSweeper sweeps the cache now and then once a day
// until ctx ends.
func (app *Application) runSubtitleTrackCacheSweeper(ctx context.Context) {
	ticker := time.NewTicker(subtitleTrackCacheSweepInterval)
	defer ticker.Stop()

	for {
		app.sweepSubtitleTrackCache(time.Now())
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// sweepSubtitleTrackCache deletes copies unused for
// subtitleTrackCacheRetention, temporary files older than any extraction can
// run, and the directories left empty.
func (app *Application) sweepSubtitleTrackCache(now time.Time) {
	transcodeRoot := app.hlsTranscodeRoot()
	if strings.TrimSpace(transcodeRoot) == "" {
		return
	}
	root := subtitleTrackCacheRoot(transcodeRoot)
	var dirs []string
	removed := 0
	walkErr := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			missingRoot := errors.Is(err, fs.ErrNotExist) && path == root
			if missingRoot {
				return fs.SkipAll
			}
			return nil
		}
		if entry.IsDir() {
			dirs = append(dirs, path)
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		age := now.Sub(info.ModTime())
		isCopy := strings.HasSuffix(path, subtitleTrackCacheExt)
		isTemp := strings.HasSuffix(path, subtitleTrackTempExt)
		expired := (isCopy && age > subtitleTrackCacheRetention) || (isTemp && age > subtitleTrackExtractTimeout)
		if expired {
			removeErr := os.Remove(path)
			if removeErr == nil {
				removed++
			}
		}
		return nil
	})
	if walkErr != nil {
		app.Logger.Warn("could not sweep the subtitle track cache", "path", root, "error", walkErr)
	}

	// Deepest first, so a file directory emptied above lets its kind
	// directory go too; a directory still holding copies stays.
	slices.Reverse(dirs)
	for _, dir := range dirs {
		_ = os.Remove(dir)
	}
	if removed > 0 {
		app.Logger.Info("swept the subtitle track cache", "removed", removed)
	}
}
