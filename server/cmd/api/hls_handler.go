package main

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"igloo/cmd/internal/helpers"
)

const (
	hlsSegmentWait      = 120 * time.Second
	hlsSegmentPoll      = 250 * time.Millisecond
	playlistContentType = "application/vnd.apple.mpegurl"
	segmentContentType  = "video/mp4"
)

var segmentFilenameRe = regexp.MustCompile(`^segment_\d+\.m4s$`)

// HLSManifest serves GET /api/movies/:id/hls/:profile/playlist.m3u8
//
// Returns a complete VOD M3U8 immediately (generated from known duration).
// All segments are listed upfront with #EXT-X-ENDLIST, so hls.js treats it
// as on-demand: starts from 0, shows full duration bar, allows seeking.
// FFmpeg produces the actual segment files in the background.
func (app *Application) HLSManifest(w http.ResponseWriter, r *http.Request) {
	movieID, profile, audioTrack, ok := parseHLSParams(w, r)
	if !ok {
		return
	}

	session, err := app.GetOrCreateHLSSession(r.Context(), movieID, profile, audioTrack)
	if err != nil {
		app.Logger.Error("hls session failed", "error", err, "movie_id", movieID)
		helpers.ErrorJSON(w, err, http.StatusBadRequest)
		return
	}

	baseURL := strings.TrimSuffix(r.URL.Path, "playlist.m3u8")
	playlist := generateVODPlaylist(session.DurationSec, baseURL, audioTrack)

	app.RefreshHLSSessionTTL(HLSSessionKey(movieID, profile, audioTrack), session)

	w.Header().Set("Content-Type", playlistContentType)
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(playlist))
}

// HLSSegment serves GET /api/movies/:id/hls/:profile/:filename
//
// Waits for the requested segment file to appear on disk (FFmpeg produces
// them sequentially in the background), then serves it.
func (app *Application) HLSSegment(w http.ResponseWriter, r *http.Request) {
	movieID, profile, audioTrack, ok := parseHLSParams(w, r)
	if !ok {
		return
	}

	filename := chi.URLParam(r, "filename")
	if filename != "init.mp4" && !segmentFilenameRe.MatchString(filename) {
		helpers.ErrorJSON(w, errors.New("invalid segment filename"), http.StatusBadRequest)
		return
	}

	key := HLSSessionKey(movieID, profile, audioTrack)
	raw, ok := app.HLSSessionCache.Get(key)
	if !ok {
		helpers.ErrorJSON(w, errors.New("session not found; request the manifest first"), http.StatusNotFound)
		return
	}
	session := raw.(*HLSSession)
	app.RefreshHLSSessionTTL(key, session)

	filePath := filepath.Join(session.TempDir, filename)

	deadline := time.Now().Add(hlsSegmentWait)
	for time.Now().Before(deadline) {
		if fileReady(filePath) {
			w.Header().Set("Content-Type", segmentContentType)
			http.ServeFile(w, r, filePath)
			return
		}

		session.ExitMu.Lock()
		exitErr := session.ExitErr
		session.ExitMu.Unlock()
		if exitErr != nil {
			helpers.ErrorJSON(w, errors.New("transcoding stopped"), http.StatusInternalServerError)
			return
		}

		time.Sleep(hlsSegmentPoll)
	}

	helpers.ErrorJSON(w, errors.New("segment not ready"), http.StatusServiceUnavailable)
}

func parseHLSParams(w http.ResponseWriter, r *http.Request) (movieID int64, profile string, audioTrack int, ok bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		helpers.ErrorJSON(w, errors.New("invalid movie id"), http.StatusBadRequest)
		return 0, "", 0, false
	}
	profile = chi.URLParam(r, "profile")
	if !helpers.IsAllowedHLSProfile(profile) {
		helpers.ErrorJSON(w, errors.New("invalid quality profile"), http.StatusBadRequest)
		return 0, "", 0, false
	}
	if q := r.URL.Query().Get("audio_track"); q != "" {
		at, err := strconv.Atoi(q)
		if err != nil || at < 0 {
			helpers.ErrorJSON(w, errors.New("invalid audio_track"), http.StatusBadRequest)
			return 0, "", 0, false
		}
		audioTrack = at
	}
	return id, profile, audioTrack, true
}

// fileReady returns true when the file exists and has content.
func fileReady(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Size() > 0
}
