package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"igloo/cmd/internal/helpers"

	"github.com/go-chi/chi/v5"
)

const (
	hlsTranscodeBusyRetryAfterSec = 5
	hlsPlaybackSessionIDPattern   = `^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`
	hlsSegmentWait                = 120 * time.Second
	// A segment that lands just after a check waits a full interval before it
	// is served, and that wait is on the startup and post-seek path. The check
	// is one or two stats against page-cached directory entries, so polling
	// tightly costs far less than the latency it removes.
	hlsSegmentPoll = 25 * time.Millisecond
	// waitForRemuxPreflight stats the init segment plus every segment it is
	// waiting on, on every pass, so it keeps the relaxed cadence the tight
	// single-segment availability check above can afford to drop.
	hlsRemuxPreflightPoll     = 250 * time.Millisecond
	hlsPlaylistContentType    = "application/vnd.apple.mpegurl"
	hlsSegmentHTTPContentType = "video/mp4"
)

var hlsPlaybackSessionIDRegexp = regexp.MustCompile(hlsPlaybackSessionIDPattern)

var errHLSSessionNotFound = errors.New("session not found")

type hlsRequestParams struct {
	MovieID         int64
	Profile         string
	AudioTrack      *int
	PlaybackSession string
	StartSec        int
	Reload          string
}

// Rebased HLS sessions are exposed as session-local VOD playlists that start
// at segment_0 on disk. The web player keeps absolute movie time in the UI and
// converts seeks to session-relative media time client-side.
func (app *Application) HLSManifest(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.currentUserID(w, r)
	if !ok {
		return
	}

	params, ok := parseHLSParams(w, r)
	if !ok {
		return
	}

	session, key, err := app.GetOrCreateHLSSession(
		r.Context(),
		params.MovieID,
		params.Profile,
		params.AudioTrack,
		params.PlaybackSession,
		params.StartSec,
		userID,
	)
	if err != nil {
		app.Logger.Error("hls session failed", "error", err, "movie_id", params.MovieID)
		writeHLSSessionError(w, err)
		return
	}

	baseURL := strings.TrimSuffix(r.URL.Path, helpers.HLS_PLAYLIST_FILENAME)
	effectiveStartSec := int(session.StartSec)
	querySuffix := buildHLSAssetQuerySuffix(hlsAssetQueryParams{
		AudioTrack:      params.AudioTrack,
		StartSec:        &effectiveStartSec,
		PlaybackSession: params.PlaybackSession,
		Reload:          params.Reload,
	})

	playlist := buildHLSPlaylistBody(session, sessionPlaylistDurationSec(session), baseURL, querySuffix)

	refreshed := app.RefreshHLSSessionTTL(key, session)
	if !refreshed {
		helpers.ErrorJSON(w, errors.New("session not found; request the manifest again"), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", hlsPlaylistContentType)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(playlist))
}

// FFmpeg writes segments asynchronously; serve only once complete.
func (app *Application) HLSSegment(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.currentUserID(w, r)
	if !ok {
		return
	}

	params, ok := parseHLSParams(w, r)
	if !ok {
		return
	}

	filename := chi.URLParam(r, "filename")
	if !isAllowedHLSFilename(filename) {
		helpers.ErrorJSON(w, errors.New("invalid segment filename"), http.StatusBadRequest)
		return
	}

	key := HLSSessionKey(params.MovieID, params.Profile, params.AudioTrack, params.PlaybackSession, params.StartSec)
	raw, ok := app.HLSSessionCache.Get(key)
	if !ok {
		helpers.ErrorJSON(w, errors.New("session not found; request the manifest first"), http.StatusNotFound)
		return
	}
	session, ok := raw.(*HLSSession)
	if !ok || session == nil {
		app.removePersonalHLSSession(key)
		helpers.ErrorJSON(w, errors.New("session not found; request the manifest first"), http.StatusNotFound)
		return
	}
	if !canAccessPersonalHLSSession(session, params.MovieID, userID) {
		helpers.ErrorJSON(w, errors.New("session not found; request the manifest first"), http.StatusNotFound)
		return
	}
	refreshed := app.RefreshHLSSessionTTL(key, session)
	if !refreshed {
		helpers.ErrorJSON(w, errors.New("session not found; request the manifest first"), http.StatusNotFound)
		return
	}

	serveReadyHLSSegment(w, r, session, filename)
}

func buildHLSPlaylistBody(session *HLSSession, durationSec float64, baseURL, querySuffix string) string {
	session.ExitMu.Lock()
	finalPlaylist := session.FinalPlaylist
	session.ExitMu.Unlock()

	if finalPlaylist != "" {
		return rewritePlaylistURLs(finalPlaylist, baseURL, querySuffix)
	}

	return generateVODPlaylist(durationSec, baseURL, querySuffix, session.CopyVideo)
}

func serveReadyHLSSegment(w http.ResponseWriter, r *http.Request, session *HLSSession, filename string) {
	filePath := filepath.Join(session.TempDir, filename)

	ticker := time.NewTicker(hlsSegmentPoll)
	defer ticker.Stop()

	deadline := time.Now().Add(hlsSegmentWait)
	for time.Now().Before(deadline) {
		if segmentComplete(session, filename) {
			w.Header().Set("Content-Type", hlsSegmentHTTPContentType)
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

		select {
		case <-r.Context().Done():
			// The player abandoned the request, almost always because of a
			// seek. Returning now keeps scrubbing from leaving goroutines
			// polling until the deadline.
			return
		case <-ticker.C:
		}
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

func writeHLSSessionError(w http.ResponseWriter, err error) {
	if errors.Is(err, errHLSSessionNotFound) {
		helpers.ErrorJSON(w, err, http.StatusNotFound)
		return
	}

	var capacityErr *hlsTranscodeCapacityError
	if errors.As(err, &capacityErr) {
		w.Header().Set("Retry-After", strconv.Itoa(hlsTranscodeBusyRetryAfterSec))
		helpers.ErrorJSON(w, err, http.StatusServiceUnavailable)
		return
	}

	var personalCapacityErr *hlsPersonalSessionCapacityError
	if errors.As(err, &personalCapacityErr) {
		w.Header().Set("Retry-After", strconv.Itoa(hlsTranscodeBusyRetryAfterSec))
		helpers.ErrorJSON(w, err, http.StatusServiceUnavailable)
		return
	}
	helpers.ErrorJSON(w, err, http.StatusBadRequest)
}
func (app *Application) StopPersonalHLSSession(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.currentUserID(w, r)
	if !ok {
		return
	}

	movieID, err := parseMovieID(r)
	if err != nil {
		helpers.ErrorJSON(w, err, http.StatusBadRequest)
		return
	}

	playbackSession := strings.TrimSpace(r.URL.Query().Get("playback_session"))
	if !hlsPlaybackSessionIDRegexp.MatchString(playbackSession) {
		helpers.ErrorJSON(w, errors.New("invalid playback_session"), http.StatusBadRequest)
		return
	}

	app.cleanupPersonalHLSSessionsForOwner(movieID, userID, playbackSession, "")
	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{Error: false})
}

func parseHLSParams(w http.ResponseWriter, r *http.Request) (hlsRequestParams, bool) {
	var params hlsRequestParams
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		helpers.ErrorJSON(w, errors.New("invalid movie id"), http.StatusBadRequest)
		return params, false
	}
	params.MovieID = id

	profile := chi.URLParam(r, "profile")
	if !helpers.IsAllowedHLSProfile(profile) {
		helpers.ErrorJSON(w, errors.New("invalid quality profile"), http.StatusBadRequest)
		return params, false
	}
	params.Profile = profile

	query := r.URL.Query()
	playbackSession := strings.TrimSpace(query.Get("playback_session"))
	if !hlsPlaybackSessionIDRegexp.MatchString(playbackSession) {
		helpers.ErrorJSON(w, errors.New("invalid playback_session"), http.StatusBadRequest)
		return params, false
	}
	params.PlaybackSession = playbackSession

	startRaw := strings.TrimSpace(query.Get("start"))
	if startRaw == "" {
		helpers.ErrorJSON(w, errors.New("start parameter is required"), http.StatusBadRequest)
		return params, false
	}
	startSec, err := strconv.Atoi(startRaw)
	if err != nil || startSec < 0 {
		helpers.ErrorJSON(w, errors.New("invalid start parameter"), http.StatusBadRequest)
		return params, false
	}
	params.StartSec = startSec

	q := strings.TrimSpace(query.Get("audio_track"))
	if q != "" {
		audioTrack, err := strconv.Atoi(q)
		if err != nil || audioTrack < 0 {
			helpers.ErrorJSON(w, errors.New("invalid audio_track"), http.StatusBadRequest)
			return params, false
		}
		params.AudioTrack = &audioTrack
	}
	params.Reload = strings.TrimSpace(query.Get("reload"))
	return params, true
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

	// bitSize 63 keeps the index representable as a non-negative int64.
	_, err := strconv.ParseUint(n, 10, 63)
	return err == nil
}
