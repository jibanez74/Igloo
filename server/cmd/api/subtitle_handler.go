package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"strconv"

	"igloo/cmd/internal/helpers"

	"github.com/go-chi/chi/v5"
)

// SubtitleWebVTT serves GET /api/movies/:id/subtitles/:trackIndex/web.vtt
//
// Extracts the requested subtitle stream from the movie file and returns
// it as WebVTT. trackIndex is the 0-based index into the movie's subtitle
// rows (ordered by stream_index), not the raw ffprobe stream index.
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

	streamIndex := sub.StreamIndex

	ctx, cancel := context.WithTimeout(r.Context(), helpers.SUBTITLE_EXTRACT_TIMEOUT)
	defer cancel()

	args := []string{
		"-y",
		"-i", movie.FilePath,
		"-map", fmt.Sprintf("0:%d", streamIndex),
		"-c:s", "webvtt",
		"-f", "webvtt",
		"pipe:1",
	}

	cmd := exec.CommandContext(ctx, app.FFmpeg.BinPath(), args...)
	out, err := cmd.Output()
	if err != nil {
		app.Logger.Error("subtitle extraction failed",
			"error", err,
			"movie_id", movieID,
			"stream_index", streamIndex,
			"codec", sub.Codec,
		)
		helpers.ErrorJSON(w, errors.New("failed to extract subtitle track"))
		return
	}

	w.Header().Set("Content-Type", helpers.SUBTITLE_WEBVTT_CONTENT_TYPE)
	w.Header().Set("Cache-Control", "private, max-age=3600")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(out)
}
