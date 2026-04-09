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

// HLSManifest serves GET /api/movies/:id/hls/:profile/playlist.m3u8
// Returns a complete VOD M3U8 immediately (generated from known duration).
// All segments are listed upfront with #EXT-X-ENDLIST, so hls.js treats it
// as on-demand: starts from 0, shows full duration bar, allows seeking.
// FFmpeg produces the actual segment files in the background.
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

	session.ExitMu.Lock()
	finalPlaylist := session.FinalPlaylist
	session.ExitMu.Unlock()

	var playlist string
	if finalPlaylist != "" {
		if session.StartSegment > 0 {
			playlist = buildResumePlaylist(finalPlaylist, session.DurationSec, baseURL, audioTrack, session.StartSegment)
		} else {
			playlist = rewritePlaylistURLs(finalPlaylist, baseURL, audioTrack)
		}
	} else {
		playlist = generateVODPlaylist(session.DurationSec, baseURL, audioTrack, session.CopyVideo)
	}

	app.RefreshHLSSessionTTL(HLSSessionKey(movieID, profile, audioTrack), session)

	w.Header().Set("Content-Type", helpers.HLS_PLAYLIST_CONTENT_TYPE)
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(playlist))
}

// HLSSegment serves GET /api/movies/:id/hls/:profile/:filename
//
// Waits for the requested segment file to appear on disk (FFmpeg produces
// them sequentially in the background), then serves it.
func (app *Application) HLSSegment(w http.ResponseWriter, r *http.Request) {
	movieID, profile, audioTrack, _, ok := parseHLSParams(w, r)
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

	diskFilename, err := resolveHLSDiskFilename(session, filename)
	if err != nil {
		if errors.Is(err, errSegmentBeforeStart) {
			helpers.ErrorJSON(w, errors.New("segment not available"), http.StatusNotFound)
		} else {
			helpers.ErrorJSON(w, errors.New("invalid segment filename"), http.StatusBadRequest)
		}
		return
	}

	filePath := filepath.Join(session.TempDir, diskFilename)

	deadline := time.Now().Add(helpers.HLS_SEGMENT_WAIT)
	pollCount := 0
	for time.Now().Before(deadline) {
		if segmentComplete(session, diskFilename) {
			w.Header().Set("Content-Type", helpers.HLS_SEGMENT_HTTP_CONTENT_TYPE)
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

// errSegmentBeforeStart is returned by resolveHLSDiskFilename when the
// requested segment index is before the session's start segment. The caller
// should respond with 404 so hls.js triggers its session-lost recovery path.
var errSegmentBeforeStart = errors.New("segment before session start")

func resolveHLSDiskFilename(session *HLSSession, filename string) (string, error) {
	if filename == helpers.HLS_INIT_FILENAME || session.StartSegment <= 0 {
		return filename, nil
	}

	reqIdx, err := parseSegmentIndex(filename)
	if err != nil {
		return "", err
	}

	mappedIdx := reqIdx - session.StartSegment
	if mappedIdx < 0 {
		return "", errSegmentBeforeStart
	}

	return fmt.Sprintf(
		"%s%d%s",
		helpers.HLS_SEGMENT_FILENAME_PREFIX,
		mappedIdx,
		helpers.HLS_SEGMENT_FILENAME_SUFFIX,
	), nil
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

// fileReady returns true when the file exists and has content.
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
