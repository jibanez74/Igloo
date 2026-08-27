package main

import (
	"context"
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
	// A session that has not published its first segment yet is seconds away,
	// not minutes, so it gets a shorter retry hint than a capacity refusal.
	hlsPlaylistRetryAfterSec    = 1
	hlsPlaybackSessionIDPattern = `^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`
	hlsSegmentWait              = 120 * time.Second
	// A segment that lands just after a check waits a full interval before it
	// is served, and that wait is on the startup and post-seek path. The check
	// is one or two stats against page-cached directory entries, so polling
	// tightly costs far less than the latency it removes.
	hlsSegmentPoll = 25 * time.Millisecond
	// waitForRemuxPreflight stats the init segment plus every segment it is
	// waiting on, on every pass, so it keeps the relaxed cadence the tight
	// single-segment availability check above can afford to drop.
	hlsRemuxPreflightPoll = 250 * time.Millisecond
	// Copy-video manifests are read back from FFmpeg rather than synthesized,
	// so the first request for one waits for FFmpeg to publish a segment.
	hlsLivePlaylistWait       = 30 * time.Second
	hlsPlaylistContentType    = "application/vnd.apple.mpegurl"
	hlsSegmentHTTPContentType = "video/mp4"
	// The extension FFmpeg's hls muxer appends while writing a file under
	// -hls_flags temp_file, before the rename to the final name.
	hlsTempFileSuffix              = ".tmp"
	hlsEffectiveProfileHeader      = "X-Igloo-Effective-Profile"
	hlsActualStartHeader           = "X-Igloo-Actual-Start"
	hlsEffectiveAudioCodecHeader   = "X-Igloo-Effective-Audio-Codec"
	hlsEffectiveAudioChannelsHdr   = "X-Igloo-Effective-Audio-Channels"
	hlsEffectiveAudioBitrateHeader = "X-Igloo-Effective-Audio-Bitrate"
)

var hlsPlaybackSessionIDRegexp = regexp.MustCompile(hlsPlaybackSessionIDPattern)

var errHLSSessionNotFound = errors.New("session not found")

// errHLSPlaylistNotReady means FFmpeg has not published a usable playlist yet.
// It is retryable: the session is healthy, it just has not produced output.
var errHLSPlaylistNotReady = errors.New("playlist not ready")

// errHLSSessionEmpty means FFmpeg finished without producing playable media,
// which happens when a session starts past the end of a stream. Retrying the
// same offset cannot help, so it is reported as not found rather than busy.
var errHLSSessionEmpty = errors.New("no playable media at this position")

// errHLSSessionFailed means FFmpeg exited with an error before publishing a
// playlist — a server-side failure, not a client error.
var errHLSSessionFailed = errors.New("transcoding stopped before publishing a playlist")

type hlsRequestParams struct {
	MovieID    int64
	Profile    string
	AudioTrack *int
	// AudioProfile is the validated audio_codec/audio_channels pair; nil means
	// legacy audio behavior.
	AudioProfile    *helpers.HLSAudioProfileRequest
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
		params.AudioProfile,
		params.PlaybackSession,
		params.StartSec,
		userID,
	)
	if err != nil {
		// Session creation can park waiting for a transcode permit, so a client
		// that navigates away lands here. That is not a server failure.
		if errors.Is(err, context.Canceled) {
			return
		}
		app.Logger.Error("hls session failed", "error", err, "movie_id", params.MovieID)
		writeHLSSessionError(w, err)
		return
	}

	baseURL := strings.TrimSuffix(r.URL.Path, helpers.HLS_PLAYLIST_FILENAME)
	effectiveStartSec := int(session.StartSec)
	querySuffix := buildHLSAssetQuerySuffix(hlsAssetQueryParams{
		AudioTrack:      params.AudioTrack,
		AudioProfile:    params.AudioProfile,
		StartSec:        &effectiveStartSec,
		PlaybackSession: params.PlaybackSession,
		Reload:          params.Reload,
	})

	playlist, err := buildHLSPlaylistBody(r.Context(), session, sessionPlaylistDurationSec(session), baseURL, querySuffix)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return
		}
		app.Logger.Error("hls playlist unavailable", "error", err, "movie_id", params.MovieID)
		writeHLSSessionError(w, err)
		return
	}

	refreshed := app.RefreshHLSSessionTTL(key, session)
	if !refreshed {
		helpers.ErrorJSON(w, errors.New("session not found; request the manifest again"), http.StatusNotFound)
		return
	}

	writeHLSPlaylistHeaders(w, session)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(playlist))
}

// writeHLSPlaylistHeaders publishes the delivery shape the client cannot infer
// from the request. The requested profile is in the URL, but the profile the
// server actually ran may differ (remux falling back to a transcode), and a
// copy-video session starts at the source keyframe at or before the requested
// offset rather than at the offset itself.
func writeHLSPlaylistHeaders(w http.ResponseWriter, session *HLSSession) {
	w.Header().Set("Content-Type", hlsPlaylistContentType)
	w.Header().Set("Cache-Control", "no-store")

	if session == nil {
		return
	}

	if session.EffectiveProfile != "" {
		w.Header().Set(hlsEffectiveProfileHeader, session.EffectiveProfile)
	}

	actualStart := session.actualStartSec()
	if actualStart >= 0 {
		w.Header().Set(hlsActualStartHeader, strconv.FormatFloat(actualStart, 'f', 3, 64))
	}

	// Diagnostic description of the session's audio output; the media stream
	// stays the playback authority. Video-only sessions omit the headers.
	audio := session.EffectiveAudioProfile
	if audio != nil {
		w.Header().Set(hlsEffectiveAudioCodecHeader, string(audio.Codec))
		w.Header().Set(hlsEffectiveAudioChannelsHdr, strconv.Itoa(audio.Channels))
		if audio.Bitrate != "" {
			w.Header().Set(hlsEffectiveAudioBitrateHeader, audio.Bitrate)
		}
	}
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

	key := HLSSessionKey(params.MovieID, params.Profile, params.AudioTrack, params.AudioProfile, params.PlaybackSession, params.StartSec)
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

// readLiveHLSPlaylist returns the playlist FFmpeg is currently writing. FFmpeg
// renames a temporary file into place, so a successful read is never torn, and
// it appends a segment only once that segment is closed.
//
// A playlist is only usable once it describes real media. A session that seeks
// past the end of a stream still exits cleanly, having written one empty
// segment declared as `#EXTINF:0.000000` under `#EXT-X-TARGETDURATION:0` —
// which is not a valid playlist and is not playable. Serving that would trade
// one bad manifest for another, so it is rejected here and the caller reports
// the session as having produced nothing.
func readLiveHLSPlaylist(tempDir string) (string, error) {
	raw, err := os.ReadFile(filepath.Join(tempDir, helpers.HLS_PLAYLIST_FILENAME))
	if err != nil {
		return "", err
	}

	playlist := string(raw)
	if !hasPlayableSegment(playlist) {
		return "", errHLSPlaylistNotReady
	}

	return playlist, nil
}

// hasPlayableSegment reports whether a playlist declares at least one segment
// with a positive duration.
func hasPlayableSegment(playlist string) bool {
	for _, line := range strings.Split(playlist, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "#EXTINF:") {
			continue
		}

		value := strings.TrimPrefix(trimmed, "#EXTINF:")
		value = strings.TrimSuffix(strings.TrimSpace(value), ",")

		duration, parseErr := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if parseErr == nil && duration > 0 {
			return true
		}
	}

	return false
}

// buildHLSPlaylistBody returns the media playlist for a session.
//
// Transcode sessions pin keyframes to the segment cadence, so a playlist
// synthesized from the movie duration describes the output exactly. Copy-video
// sessions can only split on source keyframes, so their segment durations and
// count are whatever the source encode dictates: synthesizing those advertises
// durations FFmpeg never produces and segments that will never exist. They are
// read back from FFmpeg instead, and the request waits for the first one.
func buildHLSPlaylistBody(
	ctx context.Context,
	session *HLSSession,
	durationSec float64,
	baseURL string,
	querySuffix string,
) (string, error) {
	if session == nil {
		return "", errHLSSessionNotFound
	}

	if !session.CopyVideo {
		finalPlaylist := session.currentFinalPlaylist()
		if finalPlaylist != "" {
			if !hasPlayableSegment(finalPlaylist) {
				return "", errHLSSessionEmpty
			}
			return rewritePlaylistURLs(finalPlaylist, baseURL, querySuffix), nil
		}

		// A synthesized playlist describes output FFmpeg has not produced yet,
		// which is correct while it is still running and a lie once it is not:
		// without this the client is handed a complete playlist for a dead
		// session and only discovers the failure by waiting out every segment.
		exited, exitErr := session.exitStatus()
		if exited {
			if exitErr != nil {
				return "", fmt.Errorf("%w: %v", errHLSSessionFailed, exitErr)
			}
			return "", errHLSSessionEmpty
		}

		return generateVODPlaylist(durationSec, baseURL, querySuffix, session.IndependentSegments), nil
	}

	ticker := time.NewTicker(hlsRemuxPreflightPoll)
	defer ticker.Stop()

	deadline := time.Now().Add(hlsLivePlaylistWait)
	for {
		// onExit publishes FinalPlaylist before it marks the session exited, so
		// checking it first closes the race against a session finishing here.
		finalPlaylist := session.currentFinalPlaylist()
		if finalPlaylist != "" {
			if !hasPlayableSegment(finalPlaylist) {
				return "", errHLSSessionEmpty
			}
			return rewritePlaylistURLs(finalPlaylist, baseURL, querySuffix), nil
		}

		// Exit status is checked before the live file because the live file
		// outlives the process that was appending to it. onExit publishes
		// FinalPlaylist for every exit it can read one from, so reaching here
		// with Exited set means there is nothing publishable — serving the live
		// file instead would hand the client an EVENT playlist with no
		// ENDLIST, which it reloads forever while playback sits stalled.
		exited, exitErr := session.exitStatus()
		if exited {
			if exitErr != nil {
				return "", fmt.Errorf("%w: %v", errHLSSessionFailed, exitErr)
			}
			return "", errHLSSessionEmpty
		}

		livePlaylist, readErr := readLiveHLSPlaylist(session.TempDir)
		if readErr == nil {
			return rewritePlaylistURLs(livePlaylist, baseURL, querySuffix), nil
		}

		if !time.Now().Before(deadline) {
			return "", errHLSPlaylistNotReady
		}

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-ticker.C:
		}
	}
}

func serveReadyHLSSegment(w http.ResponseWriter, r *http.Request, session *HLSSession, filename string) {
	filePath := filepath.Join(session.TempDir, filename)
	requestStart := time.Now()

	ticker := time.NewTicker(hlsSegmentPoll)
	defer ticker.Stop()

	deadline := time.Now().Add(hlsSegmentWait)
	for time.Now().Before(deadline) {
		if segmentReady(session, filename) {
			logFirstHLSSegmentServed(session, filename, requestStart)
			w.Header().Set("Content-Type", hlsSegmentHTTPContentType)
			w.Header().Set("Cache-Control", "no-store")
			http.ServeFile(w, r, filePath)
			return
		}

		exited, exitErr := session.exitStatus()
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

	// The session is healthy, the encoder just has not reached this segment,
	// so the client is told to come back rather than left to guess.
	w.Header().Set("Retry-After", strconv.Itoa(hlsPlaylistRetryAfterSec))
	helpers.ErrorJSON(w, errors.New("segment not ready"), http.StatusServiceUnavailable)
}

// logFirstHLSSegmentServed emits the session's one-time cold-start metric the
// moment its first file goes out. ttfs_ms measures from session start,
// request_wait_ms only this request's readiness poll, so a slow encoder and a
// late-arriving client are distinguishable. Bare test sessions (nil logger or
// zero StartedAt) are skipped.
func logFirstHLSSegmentServed(session *HLSSession, filename string, requestStart time.Time) {
	if session.Logger == nil || session.StartedAt.IsZero() {
		return
	}

	session.FirstServeOnce.Do(func() {
		session.Logger.Info("hls first segment served",
			"session_dir", filepath.Base(session.TempDir),
			"movie_id", session.MovieID,
			"filename", filename,
			"ttfs_ms", time.Since(session.StartedAt).Milliseconds(),
			"request_wait_ms", time.Since(requestStart).Milliseconds(),
			"copy_video", session.CopyVideo,
			"effective_profile", session.EffectiveProfile,
		)
	})
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
	if errors.Is(err, errHLSSessionNotFound) || errors.Is(err, errHLSSessionEmpty) {
		helpers.ErrorJSON(w, err, http.StatusNotFound)
		return
	}

	if errors.Is(err, errHLSSessionFailed) {
		helpers.ErrorJSON(w, err, http.StatusInternalServerError)
		return
	}

	// A syntactically valid explicit request against a stream without enough
	// stored channel metadata is a media-profile problem, not a bad query.
	var audioMetadataErr *hlsAudioMetadataError
	if errors.As(err, &audioMetadataErr) {
		helpers.ErrorJSON(w, err, http.StatusUnprocessableEntity)
		return
	}

	// A missing AC-3/E-AC-3 encoder is a server installation problem. Unlike
	// the capacity errors below it carries no Retry-After: retrying cannot
	// install the encoder.
	var encoderErr *hlsAudioEncoderUnavailableError
	if errors.As(err, &encoderErr) {
		helpers.ErrorJSON(w, err, http.StatusServiceUnavailable)
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

	var storageErr *hlsStorageCapacityError
	if errors.As(err, &storageErr) {
		w.Header().Set("Retry-After", strconv.Itoa(hlsTranscodeBusyRetryAfterSec))
		helpers.ErrorJSON(w, err, http.StatusServiceUnavailable)
		return
	}

	// The session is healthy, FFmpeg just has not published output yet, so the
	// client should come back rather than treat this as a failed session.
	if errors.Is(err, errHLSPlaylistNotReady) {
		w.Header().Set("Retry-After", strconv.Itoa(hlsPlaylistRetryAfterSec))
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

	// audio_codec and audio_channels form one request: both absent is legacy
	// mode, both present is an explicit profile, and one alone is an error.
	// An invalid pair must never silently fall back to legacy behavior.
	hasAudioCodec := query.Has("audio_codec")
	hasAudioChannels := query.Has("audio_channels")
	if hasAudioCodec != hasAudioChannels {
		helpers.ErrorJSON(w, errors.New("audio_codec and audio_channels must be provided together"), http.StatusBadRequest)
		return params, false
	}
	if hasAudioCodec {
		audioCodec, ok := helpers.ParseHLSAudioCodec(query.Get("audio_codec"))
		if !ok {
			helpers.ErrorJSON(w, errors.New("invalid audio_codec"), http.StatusBadRequest)
			return params, false
		}
		audioChannels, err := strconv.Atoi(strings.TrimSpace(query.Get("audio_channels")))
		if err != nil || !helpers.IsAllowedHLSAudioMaxChannels(audioChannels) {
			helpers.ErrorJSON(w, errors.New("invalid audio_channels"), http.StatusBadRequest)
			return params, false
		}
		params.AudioProfile = &helpers.HLSAudioProfileRequest{
			Codec:       audioCodec,
			MaxChannels: audioChannels,
		}
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

// segmentReady returns true when FFmpeg has finished writing the file, which
// is what prevents serving partially-written files that would cause hls.js
// decode errors and infinite retries.
//
// With -hls_flags temp_file (every session on the embedded FFmpeg), segments
// are written to a .tmp name and renamed on close, so a segment's final name
// existing non-empty means it is complete. init.mp4 is the one file the hls
// muxer opens under its final name directly (verified by strace against the
// embedded build), so it stays gated on evidence FFmpeg moved past it: the
// muxer opens segment_0's .tmp only after closing the init file, making
// either segment_0 name — temp or final — proof that init.mp4 is final.
//
// Sessions whose muxer lacks temp_file fall back to segmentComplete's
// successor-file heuristic, which costs one extra encoded segment of latency.
func segmentReady(session *HLSSession, filename string) bool {
	if !session.TempFileSegments {
		return segmentComplete(session, filename)
	}

	dir := session.TempDir

	if filename == helpers.HLS_INIT_FILENAME {
		if !fileReady(filepath.Join(dir, filename)) {
			return false
		}
		segmentZero := filepath.Join(
			dir,
			helpers.HLS_SEGMENT_FILENAME_PREFIX+"0"+helpers.HLS_SEGMENT_FILENAME_SUFFIX,
		)
		if fileExists(segmentZero) || fileExists(segmentZero+hlsTempFileSuffix) {
			return true
		}
		// A session that died right after writing init.mp4 never opens
		// segment_0; its init file is final because nothing can be appended
		// to a dead session's output.
		session.ExitMu.Lock()
		exited := session.Exited
		session.ExitMu.Unlock()
		return exited
	}

	return fileReady(filepath.Join(dir, filename))
}

// fileExists reports mere presence, any size: the segment_0 evidence in
// segmentReady must count a .tmp file the muxer has opened but not yet
// written to, which fileReady's size check would miss.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// segmentComplete returns true when FFmpeg has finished writing the file. It
// is the readiness fallback for muxers without temp_file; segmentReady is
// the production path.
//
// A file is complete when the file FFmpeg writes after it exists (meaning
// FFmpeg has moved on) or when FFmpeg has exited, since nothing can be
// appended to a dead session's output. This prevents serving partially-written
// files that would cause hls.js decode errors and infinite retries.
//
// Both branches need the exit tail. Without it on init.mp4, a session that
// dies after writing the init segment but before closing segment_0 leaves the
// request polling until the segment deadline and answering 503, even though
// init.mp4 is on disk and final.
func segmentComplete(session *HLSSession, filename string) bool {
	dir := session.TempDir
	prefix := helpers.HLS_SEGMENT_FILENAME_PREFIX
	suffix := helpers.HLS_SEGMENT_FILENAME_SUFFIX

	successor := ""
	if filename == helpers.HLS_INIT_FILENAME {
		successor = fmt.Sprintf("%s%d%s", prefix, 0, suffix)
	} else {
		rest := strings.TrimSuffix(strings.TrimPrefix(filename, prefix), suffix)
		n, err := strconv.ParseInt(rest, 10, 64)
		if err != nil || n < 0 {
			return false
		}
		successor = fmt.Sprintf("%s%d%s", prefix, n+1, suffix)
	}

	if fileReady(filepath.Join(dir, successor)) {
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
