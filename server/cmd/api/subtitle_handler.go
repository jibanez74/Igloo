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

	"igloo/cmd/internal/helpers"

	"github.com/go-chi/chi/v5"
	"golang.org/x/sync/singleflight"
)

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

// trackIndex refers to the subtitle row order, not the raw ffprobe stream index.
func (app *Application) serveSubtitleWebVTT(w http.ResponseWriter, r *http.Request, kind mediaKind) {
	media, err := parseMediaID(chi.URLParam(r, "id"), kind)
	if err != nil {
		helpers.ErrorJSON(w, err, http.StatusBadRequest)
		return
	}

	trackIndex, err := strconv.Atoi(chi.URLParam(r, "trackIndex"))
	if err != nil || trackIndex < 0 {
		helpers.ErrorJSON(w, errors.New("invalid subtitle track index"), http.StatusBadRequest)
		return
	}

	// An HLS session started with -ss has a media timeline that begins at zero,
	// while these cues carry absolute source timestamps. `start` is that
	// session's offset, so the cues can be rebased onto the timeline they will
	// actually be played against.
	startSec, err := parseSubtitleStartSec(r)
	if err != nil {
		helpers.ErrorJSON(w, err, http.StatusBadRequest)
		return
	}

	source, err := app.loadPlaybackSource(r.Context(), media)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New(media.notFoundMessage()), http.StatusNotFound)
			return
		}
		app.serverError(w, err, "failed to resolve media file", "failed to fetch media", "media", media.String())
		return
	}

	subtitles, err := loadPlaybackSubtitles(r.Context(), app.Queries, source)
	if err != nil {
		app.serverError(w, err, getSubtitlesLogMessage, fetchSubtitlesMessage, "media", media.String())
		return
	}

	if trackIndex >= len(subtitles) {
		helpers.ErrorJSON(w, fmt.Errorf("subtitle track %d out of range (0–%d)", trackIndex, len(subtitles)-1), http.StatusBadRequest)
		return
	}

	sub := subtitles[trackIndex]

	if helpers.IsBitmapSubtitleCodec(sub.Codec) {
		helpers.ErrorJSON(w, errors.New("image-based subtitles cannot be converted to WebVTT"), http.StatusUnsupportedMediaType)
		return
	}

	cacheKey := helpers.SubtitleCacheKey(string(kind), source.FileID, sub.StreamIndex)

	// Only []byte is ever stored under a subtitle key.
	cached, _ := app.SubtitleVTTCache.Get(cacheKey)
	vtt, isVTT := cached.([]byte)
	if isVTT {
		// A track in use is asked for again on every rebase, so a hit restarts
		// its TTL rather than letting a long film outlive it. Replace leaves
		// alone an entry a rescan invalidated after the read.
		_ = app.SubtitleVTTCache.Replace(cacheKey, vtt, subtitleCacheTTL)
		writeSubtitleWebVTT(w, vtt, startSec)
		return
	}

	out, err := app.extractSubtitleWebVTT(r.Context(), cacheKey, source.FilePath, sub.StreamIndex)
	if err != nil {
		clientGone := r.Context().Err() != nil
		if clientGone {
			return
		}
		app.serverError(w, err, "subtitle extraction failed", "failed to extract subtitle track",
			"media", media.String(), "stream_index", sub.StreamIndex, "codec", sub.Codec)
		return
	}

	writeSubtitleWebVTT(w, out, startSec)
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
func (app *Application) extractSubtitleWebVTT(ctx context.Context, cacheKey string, filePath string, streamIndex int64) ([]byte, error) {
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

			out, err := app.FFmpeg.ExtractSubtitleAsWebVTT(extractCtx, filePath, streamIndex)
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
