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
)

const (
	subtitleWebVTTContentType = "text/vtt"
	subtitleExtractTimeout    = 60 * time.Second
	subtitleCacheTTL          = time.Hour
	subtitleCacheCleanup      = 10 * time.Minute
)

func (app *Application) invalidateSubtitleVTTCache(movieID int64) {
	prefix := helpers.SubtitleCachePrefix(movieID)
	for key := range app.SubtitleVTTCache.Items() {
		if strings.HasPrefix(key, prefix) {
			app.SubtitleVTTCache.Delete(key)
		}
	}
}

// trackIndex refers to the subtitle row order, not the raw ffprobe stream index.
func (app *Application) SubtitleWebVTT(w http.ResponseWriter, r *http.Request) {
	movieID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || movieID <= 0 {
		helpers.ErrorJSON(w, errors.New("invalid movie id"), http.StatusBadRequest)
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

	movie, err := app.Queries.GetMovieByID(r.Context(), movieID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("movie not found"), http.StatusNotFound)
			return
		}
		app.Logger.Error("failed to get movie", "error", err, "id", movieID)
		helpers.ErrorJSON(w, errors.New("failed to fetch movie"))
		return
	}

	subtitles, err := app.Queries.GetSubtitlesByMovieID(r.Context(), movieID)
	if err != nil {
		app.Logger.Error("failed to get subtitles", "error", err, "movie_id", movieID)
		helpers.ErrorJSON(w, errors.New("failed to fetch subtitles"))
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

	cacheKey := helpers.SubtitleCacheKey(movieID, sub.StreamIndex)

	if cached, found := app.SubtitleVTTCache.Get(cacheKey); found {
		vtt, ok := cached.([]byte)
		if !ok {
			app.Logger.Warn("subtitle VTT cache type mismatch",
				"key", cacheKey,
				"got_type", fmt.Sprintf("%T", cached),
			)
			app.SubtitleVTTCache.Delete(cacheKey)
		} else {
			writeSubtitleWebVTT(w, vtt, startSec)
			return
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), subtitleExtractTimeout)
	defer cancel()

	// Collapse concurrent extractions of the same track (e.g. every watch-room
	// participant requesting the same .vtt at once) into a single ffmpeg run.
	v, err, _ := app.SubtitleExtractGroup.Do(cacheKey, func() (interface{}, error) {
		if cached, found := app.SubtitleVTTCache.Get(cacheKey); found {
			if vtt, ok := cached.([]byte); ok {
				return vtt, nil
			}
		}

		out, extractErr := app.FFmpeg.ExtractSubtitleAsWebVTT(ctx, movie.FilePath, sub.StreamIndex)
		if extractErr != nil {
			return nil, extractErr
		}

		app.SubtitleVTTCache.Set(cacheKey, out, subtitleCacheTTL)
		return out, nil
	})
	if err != nil {
		app.Logger.Error("subtitle extraction failed",
			"error", err,
			"movie_id", movieID,
			"stream_index", sub.StreamIndex,
			"codec", sub.Codec,
		)
		helpers.ErrorJSON(w, errors.New("failed to extract subtitle track"))
		return
	}

	out, ok := v.([]byte)
	if !ok {
		app.Logger.Error("subtitle extraction returned unexpected type",
			"got_type", fmt.Sprintf("%T", v),
			"movie_id", movieID,
			"stream_index", sub.StreamIndex,
		)
		helpers.ErrorJSON(w, errors.New("failed to extract subtitle track"))
		return
	}

	writeSubtitleWebVTT(w, out, startSec)
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
// the requesting session's timeline. The cache stays keyed on movie and stream
// alone, so shifting here costs no extra extraction and no extra cache entries.
func writeSubtitleWebVTT(w http.ResponseWriter, vtt []byte, startSec float64) {
	w.Header().Set("Content-Type", subtitleWebVTTContentType)
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(helpers.ShiftWebVTT(vtt, startSec))
}
