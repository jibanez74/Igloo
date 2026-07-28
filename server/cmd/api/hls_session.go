package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffmpeg"
	"igloo/cmd/internal/helpers"
)

const (
	hlsRemuxPrevalidateTimeout = 30 * time.Second
	// Personal sessions are refreshed by every manifest/segment fetch plus a
	// periodic client keepalive, so a short TTL doubles as an idle timeout that
	// reclaims transcode capacity soon after a browser closes without a stop.
	hlsPersonalSessionTTL = 5 * time.Minute
	// Room sessions have no per-client keepalive and always warm from start 0,
	// so evicting an idle room would restart playback for every participant.
	hlsRoomSessionTTL    = 30 * time.Minute
	hlsSessionCacheSweep = 1 * time.Minute
	// hlsStartClampTailSec is how far before the end a resume start offset at or
	// past the movie duration is clamped, so stale progress plays out the tail
	// instead of failing.
	hlsStartClampTailSec = 5
	hdrTransferPQ        = "smpte2084"
	hdrTransferHLG       = "arib-std-b67"
	// hlsMaxPersonalSessionsPerUserDefault caps concurrent personal sessions per
	// user so abandoned clients cannot pile up ffmpeg processes and temp dirs.
	hlsMaxPersonalSessionsPerUserDefault = 3
	// hlsIdlePermitReclaimThreshold is the minimum idle time before a session may
	// be reclaimed to free a transcode permit. Active clients refresh the TTL on
	// every segment fetch, so a session this idle is abandoned or backgrounded.
	hlsIdlePermitReclaimThreshold = 30 * time.Second
)

// HLSSession holds state for one HLS transcode session.
type HLSSession struct {
	MovieID          int64
	OwnerUserID      int64
	PlaybackSession  string
	TempDir          string
	Cmd              *exec.Cmd
	Cancel           context.CancelFunc
	CleanupOnce      sync.Once
	DurationSec      float64
	StartSec         float64
	Exited           bool
	ExitErr          error
	ExpectedStop     bool
	FinalPlaylist    string
	ExitMu           sync.Mutex
	IsRoom           bool
	CopyVideo        bool // true when FFmpeg uses -c:v copy for the effective session profile
}

type hlsSessionStartParams struct {
	Movie            *database.Movie
	PrimaryVideo     *database.VideoStream
	SelectedAudio    *database.AudioStream
	RequestedProfile string
	EffectiveProfile string
	AudioTrack       *int
	PlaybackSession  string
	StartSec         int
	DurationSec      float64
	IsRoom           bool
}

// isHDRStream returns true when the stream's color_transfer indicates HDR content
// (HDR10/PQ or HLG). These sources require tone-mapping when transcoded to SDR profiles.
func isHDRStream(stream *database.VideoStream) bool {
	if !stream.ColorTransfer.Valid {
		return false
	}
	ct := strings.ToLower(strings.TrimSpace(stream.ColorTransfer.String))
	return ct == hdrTransferPQ || ct == hdrTransferHLG
}

func isBrowserSafeH264RemuxCandidate(stream *database.VideoStream) (bool, string) {
	if !helpers.IsBrowserCompatibleH264(stream.Codec) {
		return false, fmt.Sprintf("requested remux is not supported for codec %q", stream.Codec)
	}

	if stream.BitDepth.Valid && stream.BitDepth.Int64 > 8 {
		return false, fmt.Sprintf("requested remux is not supported for %d-bit H.264", stream.BitDepth.Int64)
	}

	if stream.PixelFormat.Valid && isNonBrowserH264PixelFormat(stream.PixelFormat.String) {
		return false, fmt.Sprintf("requested remux is not supported for pixel format %q", stream.PixelFormat.String)
	}

	if stream.CodecProfile.Valid && isNonBrowserH264Profile(stream.CodecProfile.String) {
		return false, fmt.Sprintf("requested remux is not supported for H.264 profile %q", stream.CodecProfile.String)
	}

	return true, ""
}

func isNonBrowserH264Profile(profile string) bool {
	profile = strings.ToLower(strings.TrimSpace(profile))
	if profile == "" {
		return false
	}

	unsupportedMarkers := []string{
		"10",
		"4:2:2",
		"422",
		"4:4:4",
		"444",
	}
	for _, marker := range unsupportedMarkers {
		if strings.Contains(profile, marker) {
			return true
		}
	}
	return false
}

// browserSafeH264PixelFormats lists the only pixel formats browsers decode for
// H.264: 8-bit 4:2:0. An allowlist rather than a marker denylist because
// substring matching misreads 8-bit names — "nv12" contains "12" and "yuv410p"
// contains "10". Keep in sync with BROWSER_SAFE_H264_PIXEL_FORMATS in the web
// client (web/src/lib/playback.ts).
var browserSafeH264PixelFormats = map[string]bool{
	"yuv420p":  true,
	"yuvj420p": true,
	"nv12":     true,
	"nv21":     true,
}

func isNonBrowserH264PixelFormat(pixelFormat string) bool {
	pixelFormat = strings.ToLower(strings.TrimSpace(pixelFormat))
	if pixelFormat == "" {
		return false
	}

	return !browserSafeH264PixelFormats[pixelFormat]
}

func audioTrackCacheKey(audioTrack *int) string {
	if audioTrack == nil {
		return "audio:none"
	}
	return fmt.Sprintf("audio:%d", *audioTrack)
}

func HLSSessionKey(movieID int64, profile string, audioTrack *int, playbackSession string, startSec int) string {
	return fmt.Sprintf("movie:%d:%s:%s:session:%s:start:%d", movieID, profile, audioTrackCacheKey(audioTrack), playbackSession, startSec)
}

// RoomHLSSessionKey returns the HLS session cache key for a watch room.
// The "room:" prefix ensures it never collides with a personal HLSSessionKey.
func RoomHLSSessionKey(roomID int64) string {
	return fmt.Sprintf("room:%d", roomID)
}

func (app *Application) RefreshHLSSessionTTL(key string, session *HLSSession) bool {
	if session != nil && session.IsRoom {
		app.HLSSessionCache.Set(key, session, hlsRoomSessionTTL)
		return true
	}

	app.PersonalHLSMu.Lock()
	defer app.PersonalHLSMu.Unlock()

	raw, ok := app.HLSSessionCache.Get(key)
	if !ok || raw != session {
		return false
	}
	app.HLSSessionCache.Set(key, session, hlsPersonalSessionTTL)
	return true
}

// deleteHLSSession removes a cache entry without waiting for teardown. Personal
// session callers clean up the returned session after releasing PersonalHLSMu.
func (app *Application) deleteHLSSession(key string) *HLSSession {
	raw, ok := app.HLSSessionCache.Get(key)
	app.HLSSessionCache.Delete(key)
	if !ok {
		return nil
	}
	session, ok := raw.(*HLSSession)
	if !ok {
		return nil
	}
	return session
}

func (app *Application) removeHLSSession(key string) {
	session := app.deleteHLSSession(key)
	cleanupHLSSession(session)
}

func (app *Application) removePersonalHLSSession(key string) {
	app.PersonalHLSMu.Lock()
	session := app.deleteHLSSession(key)
	app.PersonalHLSMu.Unlock()

	cleanupHLSSession(session)
}

func (app *Application) cleanupPersonalHLSSessionsForOwner(movieID int64, ownerUserID int64, playbackSession string, keepKey string) int {
	app.PersonalHLSMu.Lock()
	sessions := app.cleanupPersonalHLSSessionsForOwnerLocked(movieID, ownerUserID, playbackSession, keepKey)
	app.PersonalHLSMu.Unlock()

	cleanupRemovedHLSSessions(sessions)
	return len(sessions)
}

// cleanupPersonalHLSSessionsForOwnerLocked requires PersonalHLSMu to be held.
// It removes only the owner's other sessions for this movie and this
// playback_session UUID — superseded windows from the same client (seeks,
// profile or audio-track switches). Sessions from the owner's other clients
// (different UUIDs, e.g. a TV playing the same movie) are never touched.
func (app *Application) cleanupPersonalHLSSessionsForOwnerLocked(movieID int64, ownerUserID int64, playbackSession string, keepKey string) []*HLSSession {
	var removed []*HLSSession
	for key, item := range app.HLSSessionCache.Items() {
		if key == keepKey {
			continue
		}
		session, ok := item.Object.(*HLSSession)
		if !ok || session == nil || !canAccessPersonalHLSSession(session, movieID, ownerUserID) {
			continue
		}
		if session.PlaybackSession == playbackSession {
			removed = append(removed, app.deleteHLSSession(key))
		}
	}
	return removed
}

func cleanupRemovedHLSSessions(sessions []*HLSSession) {
	for _, session := range sessions {
		cleanupHLSSession(session)
	}
}

// personalHLSSessionsForOwnerLocked returns the owner's personal (non-room)
// session cache entries across all movies, sorted least-recently-used first.
// Every access re-sets the entry with the full personal TTL, so ascending
// Item.Expiration is LRU order. Requires PersonalHLSMu to be held.
func (app *Application) personalHLSSessionsForOwnerLocked(ownerUserID int64) []hlsOwnedSessionEntry {
	var entries []hlsOwnedSessionEntry
	for key, item := range app.HLSSessionCache.Items() {
		session, ok := item.Object.(*HLSSession)
		if !ok || session == nil || session.IsRoom || session.OwnerUserID != ownerUserID {
			continue
		}
		entries = append(entries, hlsOwnedSessionEntry{key: key, session: session, expiration: item.Expiration})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].expiration != entries[j].expiration {
			return entries[i].expiration < entries[j].expiration
		}
		return entries[i].key < entries[j].key
	})
	return entries
}

type hlsOwnedSessionEntry struct {
	key        string
	session    *HLSSession
	expiration int64
}

type hlsPersonalSessionReservation struct {
	app         *Application
	ownerUserID int64
	once        sync.Once
}

func (reservation *hlsPersonalSessionReservation) release() {
	if reservation == nil || reservation.app == nil {
		return
	}

	reservation.once.Do(func() {
		app := reservation.app
		app.PersonalHLSMu.Lock()
		remaining := app.PersonalHLSReservations[reservation.ownerUserID] - 1
		if remaining > 0 {
			app.PersonalHLSReservations[reservation.ownerUserID] = remaining
		} else {
			delete(app.PersonalHLSReservations, reservation.ownerUserID)
		}
		app.PersonalHLSMu.Unlock()
	})
}

func (reservation *hlsPersonalSessionReservation) commit(
	movieID int64,
	key string,
	session *HLSSession,
) {
	reservation.once.Do(func() {
		app := reservation.app
		app.PersonalHLSMu.Lock()
		removed := app.cleanupPersonalHLSSessionsForOwnerLocked(
			movieID,
			reservation.ownerUserID,
			session.PlaybackSession,
			key,
		)
		app.HLSSessionCache.Set(key, session, hlsPersonalSessionTTL)

		remaining := app.PersonalHLSReservations[reservation.ownerUserID] - 1
		if remaining > 0 {
			app.PersonalHLSReservations[reservation.ownerUserID] = remaining
		} else {
			delete(app.PersonalHLSReservations, reservation.ownerUserID)
		}
		app.PersonalHLSMu.Unlock()

		cleanupRemovedHLSSessions(removed)
	})
}

// reservePersonalHLSSession admits one personal session before FFmpeg starts.
// Cached sessions and concurrent reservations share the same per-user cap.
func (app *Application) reservePersonalHLSSession(
	movieID int64,
	ownerUserID int64,
	playbackSession string,
) (*hlsPersonalSessionReservation, error) {
	app.PersonalHLSMu.Lock()

	app.HLSSessionCache.DeleteExpired()
	removed := app.cleanupPersonalHLSSessionsForOwnerLocked(movieID, ownerUserID, playbackSession, "")

	limit := app.hlsMaxPersonalSessionsPerUser()
	entries := app.personalHLSSessionsForOwnerLocked(ownerUserID)
	reserved := app.PersonalHLSReservations[ownerUserID]
	for len(entries)+reserved >= limit && len(entries) > 0 {
		victim := entries[0]
		entries = entries[1:]
		removed = append(removed, app.deleteHLSSession(victim.key))
		app.Logger.Info(
			"hls session evicted by per-user cap",
			"owner_user_id", ownerUserID,
			"victim_key", victim.key,
			"limit", limit,
		)
	}

	if len(entries)+reserved >= limit {
		app.PersonalHLSMu.Unlock()
		cleanupRemovedHLSSessions(removed)
		return nil, &hlsPersonalSessionCapacityError{MaxActive: limit}
	}

	if app.PersonalHLSReservations == nil {
		app.PersonalHLSReservations = make(map[int64]int)
	}
	app.PersonalHLSReservations[ownerUserID] = reserved + 1
	reservation := &hlsPersonalSessionReservation{
		app:         app,
		ownerUserID: ownerUserID,
	}
	app.PersonalHLSMu.Unlock()

	cleanupRemovedHLSSessions(removed)
	return reservation, nil
}

// reclaimIdlePersonalHLSSessionForOwner evicts the owner's least-recently-used
// transcoding (non-copy-video) session that has been idle for at least
// hlsIdlePermitReclaimThreshold, freeing its transcode permit. Active clients
// refresh the TTL on every segment fetch, so a genuinely-playing device is
// never reclaimed. Returns whether a session was evicted.
func (app *Application) reclaimIdlePersonalHLSSessionForOwner(ownerUserID int64) bool {
	app.PersonalHLSMu.Lock()

	maxExpiration := time.Now().Add(hlsPersonalSessionTTL - hlsIdlePermitReclaimThreshold).UnixNano()
	for _, entry := range app.personalHLSSessionsForOwnerLocked(ownerUserID) {
		if entry.session.CopyVideo {
			continue
		}
		if entry.expiration > maxExpiration {
			continue
		}

		entry.session.ExitMu.Lock()
		exited := entry.session.Exited
		entry.session.ExitMu.Unlock()
		if exited {
			continue
		}

		session := app.deleteHLSSession(entry.key)
		app.PersonalHLSMu.Unlock()
		cleanupHLSSession(session)
		app.Logger.Info("idle hls session reclaimed for transcode capacity", "owner_user_id", ownerUserID, "victim_key", entry.key)
		return true
	}
	app.PersonalHLSMu.Unlock()
	return false
}

func (app *Application) hlsMaxPersonalSessionsPerUser() int {
	if app.HLSMaxPersonalSessionsPerUser > 0 {
		return app.HLSMaxPersonalSessionsPerUser
	}
	return hlsMaxPersonalSessionsPerUserDefault
}

func canAccessPersonalHLSSession(session *HLSSession, movieID int64, ownerUserID int64) bool {
	return session != nil &&
		!session.IsRoom &&
		session.MovieID == movieID &&
		session.OwnerUserID == ownerUserID
}

func (app *Application) markRoomHLSSessionDeleted(roomID int64) {
	if app.RoomHLSTombstone == nil {
		return
	}
	app.RoomHLSTombstone.SetDefault(RoomHLSSessionKey(roomID), struct{}{})
}

func (app *Application) isRoomHLSSessionDeleted(roomID int64) bool {
	if app.RoomHLSTombstone == nil {
		return false
	}
	_, deleted := app.RoomHLSTombstone.Get(RoomHLSSessionKey(roomID))
	return deleted
}

func (app *Application) storeRoomHLSSessionIfActive(roomID int64, key string, session *HLSSession) error {
	app.RoomHLSMu.Lock()
	deleted := app.isRoomHLSSessionDeleted(roomID)
	if !deleted {
		app.HLSSessionCache.Set(key, session, hlsRoomSessionTTL)
	}
	app.RoomHLSMu.Unlock()

	if deleted {
		cleanupHLSSession(session)
		return fmt.Errorf("watch room %d was deleted during hls session creation", roomID)
	}

	return nil
}

func (app *Application) getActiveRoomHLSSession(roomID int64, key string) (*HLSSession, bool, error) {
	app.RoomHLSMu.Lock()
	defer app.RoomHLSMu.Unlock()

	if app.isRoomHLSSessionDeleted(roomID) {
		return nil, false, fmt.Errorf("watch room %d was deleted", roomID)
	}

	raw, ok := app.HLSSessionCache.Get(key)
	if !ok {
		return nil, false, nil
	}
	if raw == nil {
		return nil, false, fmt.Errorf("cached HLS session %q is nil", key)
	}

	session, typeOK := raw.(*HLSSession)
	if !typeOK {
		return nil, false, fmt.Errorf("cached HLS session %q has unexpected type %T", key, raw)
	}

	app.RefreshHLSSessionTTL(key, session)
	return session, true, nil
}

func cleanupHLSSession(session *HLSSession) {
	if session == nil {
		return
	}

	session.CleanupOnce.Do(func() {
		session.ExitMu.Lock()
		session.ExpectedStop = true
		exited := session.Exited
		cancel := session.Cancel
		session.ExitMu.Unlock()

		if cancel != nil {
			cancel()
		}

		if session.Cmd != nil && session.Cmd.Process != nil && !exited {
			exited = waitForHLSSessionExit(session, 2*time.Second)
			if !exited {
				_ = session.Cmd.Process.Kill()
				_ = waitForHLSSessionExit(session, 2*time.Second)
			}
		}

		if session.TempDir != "" {
			_ = os.RemoveAll(session.TempDir)
		}
	})
}

func (app *Application) hlsTranscodeRoot() string {
	if app == nil {
		return RuntimeConfig{}.effectiveTranscodeDir()
	}
	if app.Settings != nil && strings.TrimSpace(app.Settings.TranscodeDir) != "" {
		return app.Settings.TranscodeDir
	}
	return app.Config.effectiveTranscodeDir()
}

func waitForHLSSessionExit(session *HLSSession, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		session.ExitMu.Lock()
		exited := session.Exited
		session.ExitMu.Unlock()
		if exited {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}

	session.ExitMu.Lock()
	exited := session.Exited
	session.ExitMu.Unlock()
	return exited
}

func normalizedHLSStartSec(startSec int, durationSec float64) int {
	if float64(startSec) < durationSec {
		return startSec
	}

	clampedStart := int(durationSec) - hlsStartClampTailSec
	if clampedStart < 0 {
		return 0
	}
	return clampedStart
}

func (app *Application) startHLSSession(params *hlsSessionStartParams) (*HLSSession, error) {
	videoCodec := strings.ToLower(params.PrimaryVideo.Codec)
	audioCodec := ""
	copyAudio := false
	audioStreamIndex := -1
	if params.SelectedAudio != nil {
		audioCodec = strings.ToLower(params.SelectedAudio.Codec)
		copyAudio = audioCodec == "aac"
		// ffmpeg's -map 0:N addresses the container's global stream numbering,
		// so hand it the absolute ffprobe index rather than the ordinal.
		audioStreamIndex = int(params.SelectedAudio.StreamIndex)
	}
	sourceIsHDR := isHDRStream(params.PrimaryVideo)
	copyVideo := params.EffectiveProfile == helpers.HLS_PROFILE_REMUX
	tonemapHDR := sourceIsHDR && params.EffectiveProfile != helpers.HLS_PROFILE_REMUX

	hwDevice := hardwareAccelerationDeviceOrDefault(app.Settings)
	ffmpegCaps := app.FFmpeg.Capabilities()
	deviceDecision := ffmpeg.ResolveHLSDevice(hwDevice, ffmpegCaps)

	transcodeRoot := app.hlsTranscodeRoot()
	if err := os.MkdirAll(transcodeRoot, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create transcode directory %s: %w", transcodeRoot, err)
	}

	tempDir, err := os.MkdirTemp(transcodeRoot, "igloo-hls-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}

	// Copy-video sessions (-c:v copy) use near-zero CPU, so they bypass the
	// CPU transcode limiter instead of blocking real transcodes.
	releaseTranscode := func() {}
	if !copyVideo {
		releaseTranscode, err = app.acquireHLSTranscodeSlot()
		if err != nil {
			_ = os.RemoveAll(tempDir)
			return nil, err
		}
	}

	startSec := float64(params.StartSec)
	startSegment := int64(startSec / float64(helpers.HLS_SEGMENT_TIME_SEC))
	runCtx, cancel := context.WithCancel(context.Background())
	session := &HLSSession{
		MovieID:         params.Movie.ID,
		PlaybackSession: params.PlaybackSession,
		TempDir:         tempDir,
		Cancel:          cancel,
		DurationSec:     params.DurationSec,
		StartSec:        startSec,
		IsRoom:          params.IsRoom,
		CopyVideo:       copyVideo,
	}

	videoStreamIndex := int(params.PrimaryVideo.StreamIndex)

	app.Logger.Info("hls session starting",
		"movie_id", params.Movie.ID,
		"requested_profile", params.RequestedProfile,
		"effective_profile", params.EffectiveProfile,
		"audio_track", params.AudioTrack,
		"start_sec", startSec,
		"start_segment", startSegment,
		"video_stream_index", videoStreamIndex,
		"audio_stream_index", audioStreamIndex,
		"video_codec", videoCodec,
		"audio_codec", audioCodec,
		"copy_video", copyVideo,
		"copy_audio", copyAudio,
		"source_is_hdr", sourceIsHDR,
		"tonemap_hdr", tonemapHDR,
		"configured_hw_device", deviceDecision.Configured,
		"effective_hw_device", deviceDecision.Effective,
		"hw_fallback_reason", deviceDecision.Reason,
	)

	startTime := time.Now()
	onExit := func(exitErr error, stderrTail []string) {
		if exitErr == nil {
			raw, readErr := os.ReadFile(filepath.Join(tempDir, helpers.HLS_PLAYLIST_FILENAME))
			if readErr == nil {
				session.ExitMu.Lock()
				session.FinalPlaylist = finalizeEventPlaylist(string(raw))
				session.ExitMu.Unlock()
			}
		}

		session.ExitMu.Lock()
		expectedStop := session.ExpectedStop
		session.Exited = true
		session.ExitErr = exitErr
		session.ExitMu.Unlock()

		elapsed := time.Since(startTime).Round(time.Second)

		releaseTranscode()

		if exitErr != nil {
			if expectedStop {
				app.Logger.Info("hls session stopped",
					"movie_id", params.Movie.ID,
					"requested_profile", params.RequestedProfile,
					"effective_profile", params.EffectiveProfile,
					"elapsed", elapsed.String(),
				)
				return
			}

			app.Logger.Error("hls session failed",
				"movie_id", params.Movie.ID,
				"requested_profile", params.RequestedProfile,
				"effective_profile", params.EffectiveProfile,
				"elapsed", elapsed.String(),
				"error", exitErr.Error(),
				"ffmpeg_tail", strings.Join(stderrTail, "\n"),
			)
			return
		}

		app.Logger.Info("hls session finished",
			"movie_id", params.Movie.ID,
			"requested_profile", params.RequestedProfile,
			"effective_profile", params.EffectiveProfile,
			"elapsed", elapsed.String(),
		)
	}

	cmd, err := app.FFmpeg.RunHLS(runCtx, ffmpeg.HLSParams{
		SourcePath:       params.Movie.FilePath,
		OutDir:           tempDir,
		Profile:          params.EffectiveProfile,
		VideoStreamIndex: videoStreamIndex,
		AudioStreamIndex: audioStreamIndex,
		HWDevice:         deviceDecision.Effective,
		CopyVideo:        copyVideo,
		CopyAudio:        copyAudio,
		StartSec:         startSec,
		TonemapHDR:       tonemapHDR,
		SourceFrameRate:  params.PrimaryVideo.FrameRate,
		Capabilities:     ffmpegCaps,
	}, onExit)
	if err != nil {
		releaseTranscode()
		cleanupHLSSession(session)
		return nil, fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	session.Cmd = cmd
	return session, nil
}

// GetOrCreateHLSSession returns a cached personal session or creates a new one.
// Personal sessions are isolated by playback_session and normalized start time.
func (app *Application) GetOrCreateHLSSession(
	ctx context.Context,
	movieID int64,
	profile string,
	audioTrack *int,
	playbackSession string,
	startSec int,
	ownerUserID int64,
) (*HLSSession, string, error) {
	requestedKey := HLSSessionKey(movieID, profile, audioTrack, playbackSession, startSec)
	movie, effectiveStartSec, err := app.loadHLSMovieForSession(ctx, movieID, startSec)
	if err != nil {
		return nil, requestedKey, err
	}
	key := HLSSessionKey(movieID, profile, audioTrack, playbackSession, effectiveStartSec)

	if raw, ok := app.HLSSessionCache.Get(key); ok {
		session, typeOK := raw.(*HLSSession)
		if !typeOK || session == nil {
			app.removePersonalHLSSession(key)
		} else if !canAccessPersonalHLSSession(session, movieID, ownerUserID) {
			return nil, key, errHLSSessionNotFound
		} else {
			refreshed := app.RefreshHLSSessionTTL(key, session)
			if refreshed {
				return session, key, nil
			}
		}
	}

	v, err, _ := app.HLSSessionGroup.Do(key, func() (interface{}, error) {
		if raw, ok := app.HLSSessionCache.Get(key); ok {
			existing, typeOK := raw.(*HLSSession)
			if !typeOK || existing == nil {
				app.removePersonalHLSSession(key)
			} else if !canAccessPersonalHLSSession(existing, movieID, ownerUserID) {
				return nil, errHLSSessionNotFound
			} else {
				refreshed := app.RefreshHLSSessionTTL(key, existing)
				if refreshed {
					return existing, nil
				}
			}
		}

		reservation, reserveErr := app.reservePersonalHLSSession(
			movieID,
			ownerUserID,
			playbackSession,
		)
		if reserveErr != nil {
			return nil, reserveErr
		}
		defer reservation.release()

		session, createErr := app.createHLSSession(
			ctx,
			&movie,
			profile,
			audioTrack,
			playbackSession,
			effectiveStartSec,
			false,
		)
		if createErr != nil {
			// On a full transcode pool, an abandoned client (closed browser that
			// never sent a stop) may be holding a permit. Reclaim the owner's
			// least-recently-used idle transcode session and retry once; if none
			// qualifies, the 503 + Retry-After path stands.
			var capErr *hlsTranscodeCapacityError
			if errors.As(createErr, &capErr) && app.reclaimIdlePersonalHLSSessionForOwner(ownerUserID) {
				session, createErr = app.createHLSSession(
					ctx,
					&movie,
					profile,
					audioTrack,
					playbackSession,
					effectiveStartSec,
					false,
				)
			}
		}
		if createErr != nil {
			return nil, createErr
		}
		session.OwnerUserID = ownerUserID

		reservation.commit(movieID, key, session)
		return session, nil
	})

	if err != nil {
		return nil, key, err
	}
	session, ok := v.(*HLSSession)
	if !ok || session == nil {
		return nil, key, fmt.Errorf("singleflight returned unexpected HLS session type %T for %q", v, key)
	}
	return session, key, nil
}

// WarmUpRoomHLSSession starts an HLS session for a watch room immediately after creation.
// It uses RoomHLSSessionKey so the session is isolated from personal playback sessions.
// If a session for this room already exists in the cache, it is a no-op.
// Always warms up from startSec=0 so participants start from the beginning.
func (app *Application) WarmUpRoomHLSSession(
	ctx context.Context,
	roomID int64,
	movieID int64,
	profile string,
	audioTrack int,
) error {
	_, err := app.GetOrCreateRoomHLSSession(ctx, roomID, movieID, profile, audioTrack)
	return err
}

// GetOrCreateRoomHLSSession returns a cached room-scoped HLS session or
// creates a new one using the room-specific cache key.
func (app *Application) GetOrCreateRoomHLSSession(
	ctx context.Context,
	roomID int64,
	movieID int64,
	profile string,
	audioTrack int,
) (*HLSSession, error) {
	key := RoomHLSSessionKey(roomID)

	session, ok, err := app.getActiveRoomHLSSession(roomID, key)
	if err != nil {
		return nil, err
	}
	if ok {
		return session, nil
	}

	v, err, _ := app.HLSSessionGroup.Do(key, func() (interface{}, error) {
		existing, found, getErr := app.getActiveRoomHLSSession(roomID, key)
		if getErr != nil {
			return nil, getErr
		}
		if found {
			return existing, nil
		}

		if app.isRoomHLSSessionDeleted(roomID) {
			return nil, fmt.Errorf("watch room %d was deleted", roomID)
		}

		movie, _, loadErr := app.loadHLSMovieForSession(ctx, movieID, 0)
		if loadErr != nil {
			return nil, loadErr
		}

		audioTrackCopy := audioTrack
		session, createErr := app.createHLSSession(ctx, &movie, profile, &audioTrackCopy, "", 0, true)
		if createErr != nil {
			return nil, createErr
		}

		storeErr := app.storeRoomHLSSessionIfActive(roomID, key, session)
		if storeErr != nil {
			return nil, storeErr
		}
		return session, nil
	})

	if err != nil {
		return nil, err
	}
	session, ok = v.(*HLSSession)
	if !ok || session == nil {
		return nil, fmt.Errorf("singleflight returned unexpected HLS session type %T for %q", v, key)
	}
	return session, nil
}

// CleanupRoomHLSSession stops and removes the HLS session for a watch room.
// It is a no-op if no session exists for the room.
func (app *Application) CleanupRoomHLSSession(roomID int64) {
	key := RoomHLSSessionKey(roomID)
	app.RoomHLSMu.Lock()
	app.markRoomHLSSessionDeleted(roomID)
	_, ok := app.HLSSessionCache.Get(key)
	if ok {
		app.removeHLSSession(key)
	}
	app.RoomHLSMu.Unlock()
}

// loadHLSMovieForSession loads the movie, validates that it has a usable
// duration, and normalizes the requested start into the duration tail.
func (app *Application) loadHLSMovieForSession(
	ctx context.Context,
	movieID int64,
	startSec int,
) (database.Movie, int, error) {
	movie, err := app.Queries.GetMovieByID(ctx, movieID)
	if err != nil {
		return database.Movie{}, 0, fmt.Errorf("movie not found: %w", err)
	}
	if !movie.Duration.Valid || movie.Duration.Float64 <= 0 {
		return database.Movie{}, 0, fmt.Errorf("movie %d has no valid duration in the database", movieID)
	}
	if startSec < 0 {
		return database.Movie{}, 0, fmt.Errorf("start %d is outside movie duration %.3f", startSec, movie.Duration.Float64)
	}

	effectiveStartSec := normalizedHLSStartSec(startSec, movie.Duration.Float64)
	if effectiveStartSec != startSec {
		app.Logger.Warn("hls start clamped to duration tail",
			"movie_id", movieID,
			"requested_start", startSec,
			"clamped_start", effectiveStartSec,
			"duration", movie.Duration.Float64,
		)
	}
	return movie, effectiveStartSec, nil
}

// createHLSSession loads stream metadata from the database, creates a temp dir,
// and starts FFmpeg. No runtime ffprobe call is made. The movie must come from
// loadHLSMovieForSession, and startSec must already be normalized by it.
//
// FFmpeg runs on context.Background() so the process outlives the originating
// HTTP request. The session cache (with TTL + eviction) owns the lifecycle.
func (app *Application) createHLSSession(
	ctx context.Context,
	movie *database.Movie,
	profile string,
	audioTrack *int,
	playbackSession string,
	startSec int,
	isRoom bool,
) (*HLSSession, error) {
	movieID := movie.ID
	durationSec := movie.Duration.Float64

	videoStreams, err := app.Queries.GetVideoStreamsByMovieID(ctx, movieID)
	if err != nil {
		return nil, fmt.Errorf("failed to load video streams: %w", err)
	}
	primaryVideo := primaryVideoStream(videoStreams)
	if primaryVideo == nil {
		return nil, fmt.Errorf("no playable video track found for movie %d", movieID)
	}

	audioStreams, err := app.Queries.GetAudioStreamsByMovieID(ctx, movieID)
	if err != nil {
		return nil, fmt.Errorf("failed to load audio streams: %w", err)
	}
	var selectedAudio *database.AudioStream
	if len(audioStreams) == 0 {
		if audioTrack != nil {
			return nil, fmt.Errorf("audio_track is not valid for video-only movie %d", movieID)
		}
	} else {
		if audioTrack == nil {
			return nil, fmt.Errorf("audio_track is required for movie %d", movieID)
		}
		if *audioTrack < 0 || *audioTrack >= len(audioStreams) {
			return nil, fmt.Errorf("audio track %d out of range (0-%d)", *audioTrack, len(audioStreams)-1)
		}
		// audioTrack is an ordinal into the stream_index-ordered audio rows, the
		// same ordering the client's audio picker renders, not an ffprobe index.
		selectedAudio = &audioStreams[*audioTrack]
	}

	requestedProfile := profile
	effectiveProfile := profile
	fallbackProfile := helpers.BestFitHLSFallbackProfile(primaryVideo.Height)
	safetyCacheKey := remuxSafetyFingerprint(movie, primaryVideo)
	needsRemuxPreflight := false

	if requestedProfile == helpers.HLS_PROFILE_REMUX {
		if ok, fallbackReason := isBrowserSafeH264RemuxCandidate(primaryVideo); !ok {
			effectiveProfile = fallbackProfile
			app.setRemuxSafetyVerdict(safetyCacheKey, false, fallbackReason)
			app.Logger.Warn("remux safety fallback engaged",
				"movie_id", movieID,
				"requested_profile", requestedProfile,
				"effective_profile", effectiveProfile,
				"validation_result", "unsafe",
				"fallback_reason", fallbackReason,
			)
		} else {
			verdict, ok := app.getRemuxSafetyVerdict(safetyCacheKey)
			if ok {
				if verdict.Safe {
					app.Logger.Info("remux safety cache hit",
						"movie_id", movieID,
						"requested_profile", requestedProfile,
						"effective_profile", requestedProfile,
						"validation_result", "safe",
					)
				} else {
					effectiveProfile = fallbackProfile
					fallbackReason := verdict.Reason
					if fallbackReason == "" {
						fallbackReason = "cached unsafe remux"
					}
					app.Logger.Warn("remux safety fallback engaged",
						"movie_id", movieID,
						"requested_profile", requestedProfile,
						"effective_profile", effectiveProfile,
						"validation_result", "unsafe",
						"fallback_reason", fallbackReason,
					)
				}
			} else {
				needsRemuxPreflight = true
			}
		}
	}

	hlsParams := hlsSessionStartParams{
		Movie:            movie,
		PrimaryVideo:     primaryVideo,
		SelectedAudio:    selectedAudio,
		RequestedProfile: requestedProfile,
		EffectiveProfile: effectiveProfile,
		AudioTrack:       audioTrack,
		PlaybackSession:  playbackSession,
		StartSec:         startSec,
		DurationSec:      durationSec,
		IsRoom:           isRoom,
	}

	session, err := app.startHLSSession(&hlsParams)
	if err != nil {
		return nil, err
	}

	if !needsRemuxPreflight {
		return session, nil
	}

	waitErr := waitForRemuxPreflight(
		session,
		helpers.HLS_REMUX_PREVALIDATE_SEGMENTS,
		hlsRemuxPrevalidateTimeout,
	)
	if waitErr != nil {
		fallbackReason := waitErr.Error()
		// Preflight wait failures can be transient (timeout, early exit, partial output),
		// so fall back without persisting an unsafe remux verdict.
		app.Logger.Warn("remux safety fallback engaged",
			"movie_id", movieID,
			"requested_profile", requestedProfile,
			"effective_profile", fallbackProfile,
			"validation_result", "preflight_failed",
			"fallback_reason", fallbackReason,
		)
		cleanupHLSSession(session)
		fp := hlsParams
		fp.EffectiveProfile = fallbackProfile
		return app.startHLSSession(&fp)
	}

	validationSummary, err := ffmpeg.ValidateRemuxSafety(
		session.TempDir,
		helpers.HLS_REMUX_PREVALIDATE_SEGMENTS,
	)
	if err != nil {
		fallbackReason := err.Error()
		app.setRemuxSafetyVerdict(safetyCacheKey, false, fallbackReason)
		app.Logger.Warn("remux safety fallback engaged",
			"movie_id", movieID,
			"requested_profile", requestedProfile,
			"effective_profile", fallbackProfile,
			"validation_result", "unsafe",
			"checked_segments", validationSummary.CheckedSegments,
			"checked_sync_samples", validationSummary.CheckedSyncSamples,
			"fallback_reason", fallbackReason,
		)
		cleanupHLSSession(session)
		fp := hlsParams
		fp.EffectiveProfile = fallbackProfile
		return app.startHLSSession(&fp)
	}

	app.setRemuxSafetyVerdict(safetyCacheKey, true, "validated safe remux")
	app.Logger.Info("remux safety validated",
		"movie_id", movieID,
		"requested_profile", requestedProfile,
		"effective_profile", requestedProfile,
		"validation_result", "safe",
		"checked_segments", validationSummary.CheckedSegments,
		"checked_sync_samples", validationSummary.CheckedSyncSamples,
	)

	return session, nil
}
