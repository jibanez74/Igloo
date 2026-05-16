package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"igloo/cmd/internal/helpers"

	"github.com/go-chi/chi/v5"
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
			w.Header().Set("Content-Type", helpers.SUBTITLE_WEBVTT_CONTENT_TYPE)
			w.Header().Set("Cache-Control", "no-cache")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(vtt)
			return
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), helpers.SUBTITLE_EXTRACT_TIMEOUT)
	defer cancel()

	out, err := app.FFmpeg.ExtractSubtitleAsWebVTT(ctx, movie.FilePath, sub.StreamIndex)
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

	app.SubtitleVTTCache.Set(cacheKey, out, helpers.SUBTITLE_CACHE_TTL)

	w.Header().Set("Content-Type", helpers.SUBTITLE_WEBVTT_CONTENT_TYPE)
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(out)
}
