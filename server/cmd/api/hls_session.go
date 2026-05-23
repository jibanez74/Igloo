package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffmpeg"
	"igloo/cmd/internal/helpers"
)

// HLSSession holds state for one HLS transcode session.
type HLSSession struct {
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
	RequestedProfile string
	EffectiveProfile string
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
	return ct == helpers.HDR_TRANSFER_PQ || ct == helpers.HDR_TRANSFER_HLG
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

func isNonBrowserH264PixelFormat(pixelFormat string) bool {
	pixelFormat = strings.ToLower(strings.TrimSpace(pixelFormat))
	if pixelFormat == "" {
		return false
	}

	unsupportedMarkers := []string{
		"10",
		"12",
		"14",
		"16",
		"422",
		"444",
	}
	for _, marker := range unsupportedMarkers {
		if strings.Contains(pixelFormat, marker) {
			return true
		}
	}
	return false
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

func (app *Application) RefreshHLSSessionTTL(key string, session *HLSSession) {
	app.HLSSessionCache.Set(key, session, helpers.HLS_SESSION_TTL)
}

func (app *Application) removeHLSSession(key string) {
	raw, ok := app.HLSSessionCache.Get(key)
	app.HLSSessionCache.Delete(key)
	if !ok {
		return
	}
	if session, ok := raw.(*HLSSession); ok {
		cleanupHLSSession(session)
	}
}

func (app *Application) cleanupPersonalHLSSessions(playbackSession string, keepKey string) {
	app.PersonalHLSMu.Lock()
	defer app.PersonalHLSMu.Unlock()

	for key, item := range app.HLSSessionCache.Items() {
		if key == keepKey {
			continue
		}
		session, ok := item.Object.(*HLSSession)
		if !ok || session == nil || session.IsRoom {
			continue
		}
		if session.PlaybackSession == playbackSession {
			app.removeHLSSession(key)
		}
	}
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
		app.HLSSessionCache.Set(key, session, helpers.HLS_SESSION_TTL)
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

func (app *Application) startHLSSession(params *hlsSessionStartParams) (*HLSSession, error) {
	videoCodec := strings.ToLower(params.PrimaryVideo.Codec)
	audioCodec := ""
	copyAudio := false
	audioStreamIndex := -1
	if params.SelectedAudio != nil {
		audioCodec = strings.ToLower(params.SelectedAudio.Codec)
		copyAudio = audioCodec == "aac"
		audioStreamIndex = int(params.SelectedAudio.StreamIndex)
	}
	sourceIsHDR := isHDRStream(params.PrimaryVideo)
	copyVideo := params.EffectiveProfile == helpers.HLS_PROFILE_REMUX
	tonemapHDR := sourceIsHDR && params.EffectiveProfile != helpers.HLS_PROFILE_REMUX

	hwDevice := helpers.HARDWARE_ACCELERATION_DEVICE_CPU
	if app.Settings != nil &&
		app.Settings.HardwareAccelerationDevice.Valid &&
		app.Settings.HardwareAccelerationDevice.String != "" {
		hwDevice = app.Settings.HardwareAccelerationDevice.String
	}

	tempDir, err := os.MkdirTemp("", "igloo-hls-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}

	releaseTranscode, err := app.acquireHLSTranscodeSlot()
	if err != nil {
		_ = os.RemoveAll(tempDir)
		return nil, err
	}

	startSec := float64(params.StartSec)
	startSegment := int64(startSec / float64(helpers.HLS_SEGMENT_TIME_SEC))
	runCtx, cancel := context.WithCancel(context.Background())
	session := &HLSSession{
		PlaybackSession:  params.PlaybackSession,
		TempDir:          tempDir,
		Cancel:           cancel,
		DurationSec:      params.DurationSec,
		StartSec:         startSec,
		RequestedProfile: params.RequestedProfile,
		EffectiveProfile: params.EffectiveProfile,
		IsRoom:           params.IsRoom,
		CopyVideo:        copyVideo,
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
	)

	startTime := time.Now()
	onExit := func(exitErr error, stderrTail []string) {
		if exitErr == nil {
			raw, readErr := os.ReadFile(filepath.Join(tempDir, "playlist.m3u8"))
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
		HWDevice:         hwDevice,
		CopyVideo:        copyVideo,
		CopyAudio:        copyAudio,
		StartSec:         startSec,
		TonemapHDR:       tonemapHDR,
		SourceFrameRate:  params.PrimaryVideo.FrameRate,
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
) (*HLSSession, string, error) {
	key := HLSSessionKey(movieID, profile, audioTrack, playbackSession, startSec)

	if raw, ok := app.HLSSessionCache.Get(key); ok {
		session, typeOK := raw.(*HLSSession)
		if !typeOK || session == nil {
			app.removeHLSSession(key)
		} else {
			app.RefreshHLSSessionTTL(key, session)
			return session, key, nil
		}
	}

	v, err, _ := app.HLSSessionGroup.Do(key, func() (interface{}, error) {
		if raw, ok := app.HLSSessionCache.Get(key); ok {
			existing, typeOK := raw.(*HLSSession)
			if !typeOK || existing == nil {
				app.removeHLSSession(key)
			} else {
				return existing, nil
			}
		}

		session, createErr := app.createHLSSession(ctx, movieID, profile, audioTrack, playbackSession, startSec, false)
		if createErr != nil {
			return nil, createErr
		}

		app.HLSSessionCache.Set(key, session, helpers.HLS_SESSION_TTL)
		app.cleanupPersonalHLSSessions(playbackSession, key)
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

		audioTrackCopy := audioTrack
		session, createErr := app.createHLSSession(ctx, movieID, profile, &audioTrackCopy, "", 0, true)
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

// createHLSSession loads stream metadata from the database, creates a temp dir,
// and starts FFmpeg. No runtime ffprobe call is made.
//
// FFmpeg runs on context.Background() so the process outlives the originating
// HTTP request. The session cache (with TTL + eviction) owns the lifecycle.
func (app *Application) createHLSSession(
	ctx context.Context,
	movieID int64,
	profile string,
	audioTrack *int,
	playbackSession string,
	startSec int,
	isRoom bool,
) (*HLSSession, error) {
	movie, err := app.Queries.GetMovieByID(ctx, movieID)
	if err != nil {
		return nil, fmt.Errorf("movie not found: %w", err)
	}

	if !movie.Duration.Valid || movie.Duration.Float64 <= 0 {
		return nil, fmt.Errorf("movie %d has no valid duration in the database", movieID)
	}
	durationSec := movie.Duration.Float64
	if startSec < 0 || float64(startSec) >= durationSec {
		return nil, fmt.Errorf("start %d is outside movie duration %.3f", startSec, durationSec)
	}

	videoStreams, err := app.Queries.GetVideoStreamsByMovieID(ctx, movieID)
	if err != nil {
		return nil, fmt.Errorf("failed to load video streams: %w", err)
	}
	if len(videoStreams) == 0 {
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
		selectedAudio = &audioStreams[*audioTrack]
	}

	primaryVideo := videoStreams[0]
	requestedProfile := profile
	effectiveProfile := profile
	fallbackProfile := helpers.BestFitHLSFallbackProfile(primaryVideo.Height)
	safetyCacheKey := remuxSafetyFingerprint(&movie, &primaryVideo)
	needsRemuxPreflight := false

	if requestedProfile == helpers.HLS_PROFILE_REMUX {
		if ok, fallbackReason := isBrowserSafeH264RemuxCandidate(&primaryVideo); !ok {
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
		Movie:            &movie,
		PrimaryVideo:     &primaryVideo,
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
		helpers.HLS_REMUX_PREVALIDATE_TIMEOUT,
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
