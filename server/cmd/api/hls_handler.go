package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"igloo/cmd/internal/helpers"

	"github.com/go-chi/chi/v5"
)

// Rebased HLS sessions are exposed as session-local VOD playlists that start
// at segment_0 on disk. The web player keeps absolute movie time in the UI and
// converts seeks to session-relative media time client-side.
func (app *Application) HLSManifest(w http.ResponseWriter, r *http.Request) {
	movieID, profile, audioTrack, startSec, ok := parseHLSParams(w, r)
	if !ok {
		return
	}

	session, err := app.GetOrCreateHLSSession(r.Context(), movieID, profile, audioTrack, startSec)
	if err != nil {
		app.Logger.Error("hls session failed", "error", err, "movie_id", movieID)
		helpers.ErrorJSON(w, err, http.StatusBadRequest)
		return
	}

	baseURL := strings.TrimSuffix(r.URL.Path, "playlist.m3u8")
	querySuffix := buildHLSAssetQuerySuffix(audioTrack, r.URL.Query())

	session.ExitMu.Lock()
	finalPlaylist := session.FinalPlaylist
	session.ExitMu.Unlock()

	var playlist string
	if finalPlaylist != "" {
		playlist = rewritePlaylistURLs(finalPlaylist, baseURL, querySuffix)
	} else {
		playlist = generateVODPlaylist(
			sessionPlaylistDurationSec(session),
			baseURL,
			querySuffix,
			session.CopyVideo,
		)
	}

	app.RefreshHLSSessionTTL(HLSSessionKey(movieID, profile, audioTrack), session)

	w.Header().Set("Content-Type", helpers.HLS_PLAYLIST_CONTENT_TYPE)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(playlist))
}

// FFmpeg writes segments asynchronously; serve only once complete.
func (app *Application) HLSSegment(w http.ResponseWriter, r *http.Request) {
	movieID, profile, audioTrack, startSec, ok := parseHLSParams(w, r)
	if !ok {
		return
	}

	filename := chi.URLParam(r, "filename")
	if !isAllowedHLSFilename(filename) {
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

	if session.StartSec != startSec {
		helpers.ErrorJSON(w, errors.New("session not found; request the manifest first"), http.StatusNotFound)
		return
	}

	err := validateHLSFilename(filename)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid segment filename"), http.StatusBadRequest)
		return
	}

	filePath := filepath.Join(session.TempDir, filename)

	deadline := time.Now().Add(helpers.HLS_SEGMENT_WAIT)
	pollCount := 0
	for time.Now().Before(deadline) {
		if segmentComplete(session, filename) {
			w.Header().Set("Content-Type", helpers.HLS_SEGMENT_HTTP_CONTENT_TYPE)
			w.Header().Set("Cache-Control", "no-store")
			http.ServeFile(w, r, filePath)
			return
		}

		session.ExitMu.Lock()
		exited := session.Exited
		exitErr := session.ExitErr
		session.ExitMu.Unlock()

		if exited && !fileReady(filePath) {
			if exitErr != nil {
				helpers.ErrorJSON(w, errors.New("transcoding stopped"), http.StatusInternalServerError)
			} else {
				helpers.ErrorJSON(w, errors.New("segment does not exist"), http.StatusNotFound)
			}
			return
		}

		pollCount++
		time.Sleep(helpers.HLS_SEGMENT_POLL)
	}

	helpers.ErrorJSON(w, errors.New("segment not ready"), http.StatusServiceUnavailable)
}

func sessionPlaylistDurationSec(session *HLSSession) float64 {
	if session == nil {
		return 0
	}

	if session.StartSec <= 0 {
		return session.DurationSec
	}

	remaining := session.DurationSec - session.StartSec
	if remaining > 0 {
		return remaining
	}

	return 0
}

func validateHLSFilename(filename string) error {
	if filename == helpers.HLS_INIT_FILENAME {
		return nil
	}

	if _, err := parseSegmentIndex(filename); err != nil {
		return err
	}

	return nil
}

func parseHLSParams(w http.ResponseWriter, r *http.Request) (movieID int64, profile string, audioTrack int, startSec float64, ok bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		helpers.ErrorJSON(w, errors.New("invalid movie id"), http.StatusBadRequest)
		return 0, "", 0, 0, false
	}
	profile = chi.URLParam(r, "profile")
	if !helpers.IsAllowedHLSProfile(profile) {
		helpers.ErrorJSON(w, errors.New("invalid quality profile"), http.StatusBadRequest)
		return 0, "", 0, 0, false
	}
	if q := r.URL.Query().Get("audio_track"); q != "" {
		at, err := strconv.Atoi(q)
		if err != nil || at < 0 {
			helpers.ErrorJSON(w, errors.New("invalid audio_track"), http.StatusBadRequest)
			return 0, "", 0, 0, false
		}
		audioTrack = at
	}
	if q := r.URL.Query().Get("start"); q != "" {
		s, err := strconv.ParseFloat(q, 64)
		if err != nil || s < 0 {
			helpers.ErrorJSON(w, errors.New("invalid start parameter"), http.StatusBadRequest)
			return 0, "", 0, 0, false
		}
		startSec = s
	}
	return id, profile, audioTrack, startSec, true
}

func parseSegmentIndex(filename string) (int64, error) {
	rest := strings.TrimSuffix(strings.TrimPrefix(filename, helpers.HLS_SEGMENT_FILENAME_PREFIX), helpers.HLS_SEGMENT_FILENAME_SUFFIX)
	return strconv.ParseInt(rest, 10, 64)
}

func fileReady(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Size() > 0
}

// segmentComplete returns true when FFmpeg has finished writing the segment.
//
// A segment is complete when the next segment file exists (meaning FFmpeg
// has moved on) or when FFmpeg has exited (all remaining files are final).
// This prevents serving partially-written files that would cause hls.js
// decode errors and infinite retries.
func segmentComplete(session *HLSSession, filename string) bool {
	dir := session.TempDir
	prefix := helpers.HLS_SEGMENT_FILENAME_PREFIX
	suffix := helpers.HLS_SEGMENT_FILENAME_SUFFIX

	if filename == helpers.HLS_INIT_FILENAME {
		firstSeg := fmt.Sprintf("%s%d%s", prefix, 0, suffix)
		return fileReady(filepath.Join(dir, firstSeg))
	}

	rest := strings.TrimSuffix(strings.TrimPrefix(filename, prefix), suffix)
	n, err := strconv.ParseInt(rest, 10, 64)
	if err != nil || n < 0 {
		return false
	}

	nextSeg := fmt.Sprintf("%s%d%s", prefix, n+1, suffix)
	if fileReady(filepath.Join(dir, nextSeg)) {
		return true
	}

	session.ExitMu.Lock()
	exited := session.Exited
	session.ExitMu.Unlock()

	return exited && fileReady(filepath.Join(dir, filename))
}

func isAllowedHLSFilename(name string) bool {
	if name == helpers.HLS_INIT_FILENAME {
		return true
	}

	p := helpers.HLS_SEGMENT_FILENAME_PREFIX
	s := helpers.HLS_SEGMENT_FILENAME_SUFFIX
	if !strings.HasPrefix(name, p) || !strings.HasSuffix(name, s) {
		return false
	}

	n := name[len(p) : len(name)-len(s)]
	if n == "" {
		return false
	}

	_, err := strconv.ParseUint(n, 10, 64)
	return err == nil
}
