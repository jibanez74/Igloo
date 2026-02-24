package main

import (
	"bufio"
	"database/sql"
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

// isAllowedHLSSegmentFilename returns true only for init.mp4 or segment_N.m4s (N any non-negative integer).
func isAllowedHLSSegmentFilename(name string) bool {
	if name == "init.mp4" {
		return true
	}

	if !strings.HasPrefix(name, "segment_") || !strings.HasSuffix(name, ".m4s") {
		return false
	}

	mid := name[len("segment_") : len(name)-len(".m4s")]
	if mid == "" {
		return false
	}

	for _, c := range mid {
		if c < '0' || c > '9' {
			return false
		}
	}

	return true
}

// parseHLSIDAndProfile extracts id and profile from the request path.
// Returns 400 with a clear message on invalid id or profile.
func parseHLSIDAndProfile(w http.ResponseWriter, r *http.Request) (movieID int64, profile string, ok bool) {
	idParam := chi.URLParam(r, "id")
	movieID, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil || movieID < 1 {
		helpers.ErrorJSON(w, errors.New("invalid movie id"), http.StatusBadRequest)
		return 0, "", false
	}
	profile = chi.URLParam(r, "profile")
	if profile == "" {
		helpers.ErrorJSON(w, errors.New("missing profile"), http.StatusBadRequest)
		return 0, "", false
	}
	if !helpers.IsAllowedHLSProfile(profile) {
		helpers.ErrorJSON(w, fmt.Errorf("invalid profile %q", profile), http.StatusBadRequest)
		return 0, "", false
	}
	return movieID, profile, true
}

// parseAudioTrack returns the audio_track query param (default 0). Returns 400 if invalid.
func parseAudioTrack(w http.ResponseWriter, r *http.Request) (audioTrack int, ok bool) {
	q := r.URL.Query().Get("audio_track")
	if q == "" {
		return 0, true
	}
	audioTrack, err := strconv.Atoi(q)
	if err != nil || audioTrack < 0 {
		helpers.ErrorJSON(w, errors.New("invalid audio_track"), http.StatusBadRequest)
		return 0, false
	}
	return audioTrack, true
}

// validateMovieForHLS loads the movie, runs ffprobe, and checks:
// - movie exists and file exists on disk
// - at least one video stream (playable)
// - audio_track is in range for the file's audio streams
// On failure it writes the appropriate response (400/404) and returns false.
func validateMovieForHLS(w http.ResponseWriter, r *http.Request, app *Application, movieID int64, audioTrack int) bool {
	ctx := r.Context()
	movie, err := app.Queries.GetMovieByID(ctx, movieID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("movie not found"), http.StatusNotFound)
			return false
		}
		app.Logger.Error("hls: get movie", "error", err, "id", movieID)
		helpers.ErrorJSON(w, errors.New("failed to fetch movie"), http.StatusInternalServerError)
		return false
	}

	if _, err := os.Stat(movie.FilePath); err != nil {
		if os.IsNotExist(err) {
			helpers.ErrorJSON(w, errors.New("movie file not found"), http.StatusNotFound)
			return false
		}
		app.Logger.Error("hls: stat movie file", "error", err, "path", movie.FilePath)
		helpers.ErrorJSON(w, errors.New("failed to read movie file"), http.StatusInternalServerError)
		return false
	}

	meta, err := app.Ffprobe.GetMetadata(movie.FilePath)
	if err != nil {
		app.Logger.Error("hls: ffprobe", "error", err, "path", movie.FilePath)
		helpers.ErrorJSON(w, errors.New("failed to read movie metadata"), http.StatusBadRequest)
		return false
	}

	var videoCount, audioCount int
	for _, s := range meta.Streams {
		switch s.CodecType {
		case "video":
			videoCount++
		case "audio":
			audioCount++
		}
	}
	if videoCount == 0 {
		helpers.ErrorJSON(w, errors.New("no playable video track"), http.StatusBadRequest)
		return false
	}
	if audioTrack >= audioCount {
		helpers.ErrorJSON(w, fmt.Errorf("invalid audio_track (file has %d audio stream(s))", audioCount), http.StatusBadRequest)
		return false
	}

	return true
}

// ServeHLSPlaylist handles GET /api/movies/:id/hls/:profile/playlist.m3u8?audio_track=0
// Auth is enforced by IsAuth middleware on the HLS route group.
func (app *Application) ServeHLSPlaylist(w http.ResponseWriter, r *http.Request) {
	movieID, profile, ok := parseHLSIDAndProfile(w, r)
	if !ok {
		return
	}
	audioTrack, ok := parseAudioTrack(w, r)
	if !ok {
		return
	}
	if !validateMovieForHLS(w, r, app, movieID, audioTrack) {
		return
	}

	ctx := r.Context()
	session, err := app.GetOrCreateHLSSession(ctx, movieID, profile, audioTrack)
	if err != nil {
		app.Logger.Error("hls: get-or-create session", "error", err, "movie_id", movieID, "profile", profile)
		helpers.ErrorJSON(w, err, http.StatusBadRequest)
		return
	}

	playlistPath := filepath.Join(session.OutDir, "playlist.m3u8")
	initPath := filepath.Join(session.OutDir, "init.mp4")

	deadline := time.Now().Add(helpers.HLS_MANIFEST_POLL_TIMEOUT)
	for time.Now().Before(deadline) {
		if exitErr := session.ExitError(); exitErr != nil {
			helpers.ErrorJSON(w, fmt.Errorf("transcoding failed: %w", exitErr), http.StatusBadRequest)
			return
		}
		if _, err1 := os.Stat(initPath); err1 == nil {
			if _, err2 := os.Stat(playlistPath); err2 == nil {
				break
			}
		}
		time.Sleep(helpers.HLS_MANIFEST_POLL_INTERVAL)
	}

	if exitErr := session.ExitError(); exitErr != nil {
		helpers.ErrorJSON(w, fmt.Errorf("transcoding failed: %w", exitErr), http.StatusBadRequest)
		return
	}
	body, err := os.ReadFile(playlistPath)
	if err != nil {
		app.Logger.Error("hls: read playlist", "error", err, "path", playlistPath)
		helpers.ErrorJSON(w, errors.New("playlist not ready"), http.StatusServiceUnavailable)
		return
	}

	baseURL := app.hlsSegmentBaseURL(r, movieID, profile)
	rewritten := rewritePlaylistSegmentURLs(string(body), baseURL, audioTrack)

	key := HLSSessionKey(movieID, profile, audioTrack)
	app.RefreshHLSSessionTTL(key, session)

	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(rewritten))
}

// hlsSegmentBaseURL returns the base URL for segment requests (scheme + host + path prefix, no query).
func (app *Application) hlsSegmentBaseURL(r *http.Request, movieID int64, profile string) string {
	scheme := "https"
	if r.TLS == nil {
		scheme = "http"
	}
	if v := r.Header.Get("X-Forwarded-Proto"); v != "" {
		scheme = v
	}
	return fmt.Sprintf("%s://%s/api/movies/%d/hls/%s/", scheme, r.Host, movieID, profile)
}

// rewritePlaylistSegmentURLs rewrites relative segment/init filenames and EXT-X-MAP URI to full URLs with audio_track query.
func rewritePlaylistSegmentURLs(playlist string, baseURL string, audioTrack int) string {
	suffix := fmt.Sprintf("?audio_track=%d", audioTrack)
	var out strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(playlist))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") {
			// Rewrite EXT-X-MAP URI so the client requests init.mp4 with audio_track.
			if strings.Contains(line, "EXT-X-MAP") && strings.Contains(line, "init.mp4") {
				line = strings.ReplaceAll(line, "URI=\"init.mp4\"", "URI=\""+baseURL+"init.mp4"+suffix+"\"")
				line = strings.ReplaceAll(line, "URI='init.mp4'", "URI='"+baseURL+"init.mp4"+suffix+"'")
			}
			out.WriteString(line)
			out.WriteByte('\n')
			continue
		}
		if line == "" {
			out.WriteString(line)
			out.WriteByte('\n')
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && (trimmed == "init.mp4" || (strings.HasPrefix(trimmed, "segment_") && strings.HasSuffix(trimmed, ".m4s"))) {
			out.WriteString(baseURL + trimmed + suffix)
		} else {
			out.WriteString(line)
		}
		out.WriteByte('\n')
	}
	return out.String()
}

// ServeHLSSegment handles GET /api/movies/:id/hls/:profile/:filename?audio_track=0
// Filename must be init.mp4 or segment_N.m4s. Auth is enforced by IsAuth middleware on the HLS route group.
func (app *Application) ServeHLSSegment(w http.ResponseWriter, r *http.Request) {
	movieID, profile, ok := parseHLSIDAndProfile(w, r)
	if !ok {
		return
	}
	audioTrack, ok := parseAudioTrack(w, r)
	if !ok {
		return
	}
	filename := chi.URLParam(r, "filename")
	if filename == "" || !isAllowedHLSSegmentFilename(filename) {
		helpers.ErrorJSON(w, errors.New("invalid segment or file name"), http.StatusBadRequest)
		return
	}
	if filepath.Base(filename) != filename {
		helpers.ErrorJSON(w, errors.New("invalid segment or file name"), http.StatusBadRequest)
		return
	}

	key := HLSSessionKey(movieID, profile, audioTrack)
	v, ok := app.HLSSessionCache.Get(key)
	if !ok {
		helpers.ErrorJSON(w, errors.New("session not found or expired"), http.StatusServiceUnavailable)
		return
	}
	session := v.(*HLSSession)
	segmentPath := filepath.Join(session.OutDir, filename)

	deadline := time.Now().Add(helpers.HLS_SEGMENT_WAIT_TIMEOUT)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(segmentPath); err == nil {
			break
		}
		time.Sleep(helpers.HLS_MANIFEST_POLL_INTERVAL)
	}

	if _, err := os.Stat(segmentPath); err != nil {
		if os.IsNotExist(err) {
			helpers.ErrorJSON(w, errors.New("segment not ready"), http.StatusServiceUnavailable)
			return
		}
		app.Logger.Error("hls: stat segment", "error", err, "path", segmentPath)
		helpers.ErrorJSON(w, errors.New("failed to read segment"), http.StatusInternalServerError)
		return
	}

	app.RefreshHLSSessionTTL(key, session)

	w.Header().Set("Content-Type", "video/mp4")
	http.ServeFile(w, r, segmentPath)
}
