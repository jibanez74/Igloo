package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"

	"github.com/go-chi/chi/v5"
	"golang.org/x/sync/singleflight"
)

// subtitleStartProbeTimeout bounds the header read that finds a media file's
// container start before its cached track is converted. A variable so tests
// can shorten it.
var subtitleStartProbeTimeout = 10 * time.Second

const (
	subtitleWebVTTContentType = "text/vtt"
	subtitleExtractTimeout    = 60 * time.Second
	subtitleCacheTTL          = time.Hour
	subtitleCacheCleanup      = 10 * time.Minute
)

// Said by the subtitle endpoint and by the movie and episode detail handlers
// that embed the same list.
const (
	getSubtitlesLogMessage = "failed to get subtitles"
	fetchSubtitlesMessage  = "failed to fetch subtitles"
)

// invalidateSubtitleVTTCache drops every extracted track of one file; the
// cache is keyed on the physical file, so a combined TV file is cleared for
// all of its episodes at once.
func (app *Application) invalidateSubtitleVTTCache(kind mediaKind, fileID int64) {
	prefix := helpers.SubtitleCachePrefix(string(kind), fileID)
	for key := range app.SubtitleVTTCache.Items() {
		if strings.HasPrefix(key, prefix) {
			app.SubtitleVTTCache.Delete(key)
		}
	}
}

// SubtitleWebVTT converts one of a movie's subtitle tracks to WebVTT.
func (app *Application) SubtitleWebVTT(w http.ResponseWriter, r *http.Request) {
	app.serveSubtitleWebVTT(w, r, mediaKindMovie)
}

// EpisodeSubtitleWebVTT converts one of a TV episode's subtitle tracks to
// WebVTT.
func (app *Application) EpisodeSubtitleWebVTT(w http.ResponseWriter, r *http.Request) {
	app.serveSubtitleWebVTT(w, r, mediaKindEpisode)
}

// subtitleTrackRequest is a subtitle route's checked request: the media, its
// source file, the subtitle row trackIndex names, and the session start.
type subtitleTrackRequest struct {
	Media    mediaRef
	Source   playbackSource
	Subtitle database.Subtitle
	StartSec float64
}

// resolveSubtitleTrackRequest reads a subtitle route's id, trackIndex and
// start, answering the request itself when they do not name a track.
// trackIndex refers to the subtitle row order, not the raw ffprobe stream
// index.
func (app *Application) resolveSubtitleTrackRequest(w http.ResponseWriter, r *http.Request, kind mediaKind) (subtitleTrackRequest, bool) {
	media, err := parseMediaID(chi.URLParam(r, "id"), kind)
	if err != nil {
		helpers.ErrorJSON(w, err, http.StatusBadRequest)
		return subtitleTrackRequest{}, false
	}

	trackIndex, err := strconv.Atoi(chi.URLParam(r, "trackIndex"))
	if err != nil || trackIndex < 0 {
		helpers.ErrorJSON(w, errors.New("invalid subtitle track index"), http.StatusBadRequest)
		return subtitleTrackRequest{}, false
	}

	// An HLS session started with -ss has a media timeline that begins at zero,
	// while subtitle tracks carry absolute source timestamps. `start` is that
	// session's offset, so the track can be rebased onto the timeline it will
	// actually be played against.
	startSec, err := parseSubtitleStartSec(r)
	if err != nil {
		helpers.ErrorJSON(w, err, http.StatusBadRequest)
		return subtitleTrackRequest{}, false
	}

	source, err := app.loadPlaybackSource(r.Context(), media)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New(media.notFoundMessage()), http.StatusNotFound)
			return subtitleTrackRequest{}, false
		}
		app.serverError(w, err, "failed to resolve media file", "failed to fetch media", "media", media.String())
		return subtitleTrackRequest{}, false
	}

	subtitles, err := loadPlaybackSubtitles(r.Context(), app.Queries, source)
	if err != nil {
		app.serverError(w, err, getSubtitlesLogMessage, fetchSubtitlesMessage, "media", media.String())
		return subtitleTrackRequest{}, false
	}

	if trackIndex >= len(subtitles) {
		helpers.ErrorJSON(w, fmt.Errorf("subtitle track %d out of range (0–%d)", trackIndex, len(subtitles)-1), http.StatusBadRequest)
		return subtitleTrackRequest{}, false
	}

	return subtitleTrackRequest{
		Media:    media,
		Source:   source,
		Subtitle: subtitles[trackIndex],
		StartSec: startSec,
	}, true
}

func (app *Application) serveSubtitleWebVTT(w http.ResponseWriter, r *http.Request, kind mediaKind) {
	track, ok := app.resolveSubtitleTrackRequest(w, r, kind)
	if !ok {
		return
	}
	sub := track.Subtitle

	if helpers.IsBitmapSubtitleCodec(sub.Codec) {
		helpers.ErrorJSON(w, errors.New("image-based subtitles cannot be converted to WebVTT"), http.StatusUnsupportedMediaType)
		return
	}

	cacheKey := helpers.SubtitleCacheKey(string(kind), track.Source.FileID, sub.StreamIndex)

	// Only []byte is ever stored under a subtitle key.
	cached, _ := app.SubtitleVTTCache.Get(cacheKey)
	vtt, isVTT := cached.([]byte)
	if isVTT {
		// A track in use is asked for again on every rebase, so a hit restarts
		// its TTL rather than letting a long film outlive it. Replace leaves
		// alone an entry a rescan invalidated after the read.
		_ = app.SubtitleVTTCache.Replace(cacheKey, vtt, subtitleCacheTTL)
		writeSubtitleWebVTT(w, vtt, track.StartSec)
		return
	}

	ref := subtitleTrackRef{
		Kind:        kind,
		FileID:      track.Source.FileID,
		StreamIndex: sub.StreamIndex,
		SourcePath:  track.Source.FilePath,
	}
	out, err := app.extractSubtitleWebVTT(r.Context(), cacheKey, ref)
	if err != nil {
		clientGone := r.Context().Err() != nil
		if clientGone {
			return
		}
		app.serverError(w, err, "subtitle extraction failed", "failed to extract subtitle track",
			"media", track.Media.String(), "stream_index", sub.StreamIndex, "codec", sub.Codec)
		return
	}

	writeSubtitleWebVTT(w, out, track.StartSec)
}

// extractSubtitleWebVTT converts a track once for every request waiting on it
// (every watch-room member asks for the same .vtt at once) and caches the
// result. The run is detached from the request that starts it: a player drops
// its subtitle request whenever it rebuilds the track, which every rebase does,
// and canceling the shared run would fail each request joined to it and throw
// away work that, for Matroska, reads the whole file. It ends at its own
// timeout or when shutdown begins. Each caller waits in a goroutine counted in
// app.Wait before the caller can return, so shutdown never passes
// app.Wait.Wait() with a run still to start; the caller stops waiting when its
// own request ends.
func (app *Application) extractSubtitleWebVTT(ctx context.Context, cacheKey string, ref subtitleTrackRef) ([]byte, error) {
	// Buffered, so the goroutine never blocks on a caller that stopped waiting.
	results := make(chan singleflight.Result, 1)
	app.Wait.Add(1)
	go func() {
		defer app.Wait.Done()

		val, err, shared := app.SubtitleExtractGroup.Do(cacheKey, func() (interface{}, error) {
			cached, found := app.SubtitleVTTCache.Get(cacheKey)
			if found {
				vtt, ok := cached.([]byte)
				if ok {
					return vtt, nil
				}
			}

			extractCtx, cancel := context.WithTimeout(app.HLSShutdown, subtitleExtractTimeout)
			defer cancel()

			out, err := app.convertSubtitleToWebVTT(extractCtx, ref)
			if err != nil {
				return nil, err
			}

			app.SubtitleVTTCache.Set(cacheKey, out, subtitleCacheTTL)
			return out, nil
		})
		results <- singleflight.Result{Val: val, Err: err, Shared: shared}
	}()

	var result singleflight.Result
	select {
	case result = <-results:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if result.Err != nil {
		return nil, result.Err
	}

	// The flight returns []byte whenever it succeeds.
	out, _ := result.Val.([]byte)
	return out, nil
}

// convertSubtitleToWebVTT converts the track's cached copy when there is one,
// a read of a few kilobytes, and otherwise the media file, which for Matroska
// means reading all of it. It never starts a copy into the cache, nor waits
// for one.
func (app *Application) convertSubtitleToWebVTT(ctx context.Context, ref subtitleTrackRef) ([]byte, error) {
	cachePath := app.cachedSubtitleTrack(ref)
	if cachePath != "" {
		out, err := app.convertCachedSubtitleToWebVTT(ctx, ref, cachePath)
		if err == nil {
			return out, nil
		}
		if ctx.Err() != nil {
			return nil, err
		}
		app.Logger.Warn("cached subtitle track could not be converted; reading the media file",
			"track", ref.String(), "error", err)
	}
	return app.FFmpeg.ExtractSubtitleAsWebVTT(ctx, ref.SourcePath, ref.StreamIndex)
}

// convertCachedSubtitleToWebVTT converts the cached copy on the timeline the
// media file's conversion uses, which starts at the container's start time.
// Reading that takes ffprobe a look at the file's header, not the full read
// the copy spares.
func (app *Application) convertCachedSubtitleToWebVTT(ctx context.Context, ref subtitleTrackRef, cachePath string) ([]byte, error) {
	// A probe of its own budget, so one stuck on a slow share leaves the
	// conversion time to fall back to the media file.
	probeCtx, cancel := context.WithTimeout(ctx, subtitleStartProbeTimeout)
	defer cancel()
	probe, err := app.Ffprobe.GetMetadata(probeCtx, ref.SourcePath)
	if err != nil {
		return nil, fmt.Errorf("read the media file's start time: %w", err)
	}
	startSec := 0.0
	rawStart := strings.TrimSpace(probe.Format.StartTime)
	if rawStart != "" && rawStart != "N/A" {
		startSec, err = strconv.ParseFloat(rawStart, 64)
		if err != nil || math.IsNaN(startSec) || math.IsInf(startSec, 0) {
			return nil, fmt.Errorf("invalid media file start time %q", rawStart)
		}
	}
	return app.FFmpeg.ExtractCachedSubtitleAsWebVTT(ctx, cachePath, startSec)
}

// parseSubtitleStartSec reads the optional `start` query parameter naming the
// HLS session offset these cues will be played against.
func parseSubtitleStartSec(r *http.Request) (float64, error) {
	raw := strings.TrimSpace(r.URL.Query().Get("start"))
	if raw == "" {
		return 0, nil
	}

	startSec, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, errors.New("invalid start")
	}

	// ParseFloat accepts "NaN" and "Inf", which would corrupt the cue shifting
	// math downstream.
	if math.IsNaN(startSec) || math.IsInf(startSec, 0) {
		return 0, errors.New("invalid start")
	}

	if startSec < 0 {
		return 0, errors.New("start must not be negative")
	}

	return startSec, nil
}

// writeSubtitleWebVTT serves the cached absolute-timestamp WebVTT, rebased onto
// the requesting session's timeline. The cache stays keyed on file and stream
// alone, so shifting here costs no extra extraction and no extra cache entries.
func writeSubtitleWebVTT(w http.ResponseWriter, vtt []byte, startSec float64) {
	w.Header().Set("Content-Type", subtitleWebVTTContentType)
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(helpers.ShiftWebVTT(vtt, startSec))
}
