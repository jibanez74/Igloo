package main

import (
	"errors"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"igloo/cmd/internal/helpers"

	"github.com/go-chi/chi/v5"
)

func (app *Application) WatchRoomHLSManifest(w http.ResponseWriter, r *http.Request) {
	room, _, ok := app.loadAuthorizedWatchRoomForRequest(w, r)
	if !ok {
		return
	}

	if room.PlaybackMode == watchRoomPlaybackModeDirect {
		helpers.ErrorJSON(w, errors.New("this room uses direct playback"), http.StatusBadRequest)
		return
	}

	session, err := app.GetOrCreateRoomHLSSession(r.Context(), room.ID, room.MovieID, room.PlaybackMode, int(room.AudioTrack))
	if err != nil {
		app.Logger.Error("watch room hls session failed", "error", err, "room_id", room.ID)
		writeHLSSessionError(w, err)
		return
	}

	baseURL := strings.TrimSuffix(r.URL.Path, helpers.HLS_PLAYLIST_FILENAME)
	audioTrack := int(room.AudioTrack)
	querySuffix := buildHLSAssetQuerySuffix(hlsAssetQueryParams{AudioTrack: &audioTrack})

	playlist := buildHLSPlaylistBody(session, session.DurationSec, baseURL, querySuffix)

	w.Header().Set("Content-Type", hlsPlaylistContentType)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(playlist))
}

func (app *Application) WatchRoomHLSSegment(w http.ResponseWriter, r *http.Request) {
	room, _, ok := app.loadAuthorizedWatchRoomForRequest(w, r)
	if !ok {
		return
	}

	filename := chi.URLParam(r, "filename")
	if !isAllowedHLSFilename(filename) {
		helpers.ErrorJSON(w, errors.New("invalid segment filename"), http.StatusBadRequest)
		return
	}

	key := RoomHLSSessionKey(room.ID)
	session, found, err := app.getActiveRoomHLSSession(room.ID, key)
	if err != nil {
		app.Logger.Error("watch room hls session fetch failed", "error", err, "room_id", room.ID)
		helpers.ErrorJSON(w, errors.New(internalServerErrorMessage), http.StatusInternalServerError)
		return
	}
	if !found {
		helpers.ErrorJSON(w, errors.New("session not found; request the manifest first"), http.StatusNotFound)
		return
	}

	serveReadyHLSSegment(w, r, session, filename)
}

func (app *Application) StreamWatchRoomMovie(w http.ResponseWriter, r *http.Request) {
	room, _, ok := app.loadAuthorizedWatchRoomForRequest(w, r)
	if !ok {
		return
	}

	if room.PlaybackMode != watchRoomPlaybackModeDirect {
		helpers.ErrorJSON(w, errors.New("this room uses HLS playback"), http.StatusBadRequest)
		return
	}

	movie, err := app.Queries.GetMovieForDirectStream(r.Context(), room.MovieID)
	if err != nil {
		app.Logger.Error("failed to get watch room movie for streaming", "error", err, "room_id", room.ID, "movie_id", room.MovieID)
		helpers.ErrorJSON(w, errors.New("failed to fetch movie from server"))
		return
	}

	file, err := os.Open(movie.FilePath)
	if err != nil {
		if os.IsNotExist(err) {
			app.Logger.Error("watch room movie file not found on disk", "path", movie.FilePath, "room_id", room.ID, "movie_id", room.MovieID)
			helpers.ErrorJSON(w, errors.New("movie file not found"), http.StatusNotFound)
			return
		}

		app.Logger.Error("failed to open watch room movie file", "error", err, "path", movie.FilePath, "room_id", room.ID)
		helpers.ErrorJSON(w, errors.New("failed to open movie file"))
		return
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		app.Logger.Error("failed to stat watch room movie file", "error", err, "path", movie.FilePath, "room_id", room.ID)
		helpers.ErrorJSON(w, errors.New("failed to read movie file"))
		return
	}

	contentType := movie.MimeType
	if contentType == "" {
		ext := filepath.Ext(movie.FileName)
		contentType = mime.TypeByExtension(ext)
		if contentType == "" {
			contentType = "application/octet-stream"
		}
	}

	w.Header().Set("Content-Type", contentType)
	http.ServeContent(w, r, movie.FileName, stat.ModTime(), file)
}
