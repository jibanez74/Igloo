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
	TempDir          string
	Cmd              *exec.Cmd
	DurationSec      float64
	StartSec         float64
	StartSegment     int64
	Exited           bool
	ExitErr          error
	FinalPlaylist    string
	ExitMu           sync.Mutex
	RequestedProfile string
	EffectiveProfile string
	CopyVideo        bool // true when FFmpeg uses -c:v copy for the effective session profile
}

type hlsSessionStartParams struct {
	Movie            database.Movie
	PrimaryVideo     database.VideoStream
	SelectedAudio    database.AudioStream
	RequestedProfile string
	EffectiveProfile string
	AudioTrack       int
	StartSec         float64
	DurationSec      float64
}

// isHDRStream returns true when the stream's color_transfer indicates HDR content
// (HDR10/PQ or HLG). These sources require tone-mapping when transcoded to SDR profiles.
func isHDRStream(stream database.VideoStream) bool {
	if !stream.ColorTransfer.Valid {
		return false
	}
	ct := strings.ToLower(strings.TrimSpace(stream.ColorTransfer.String))
	return ct == helpers.HDR_TRANSFER_PQ || ct == helpers.HDR_TRANSFER_HLG
}

func HLSSessionKey(movieID int64, profile string, audioTrack int) string {
	return fmt.Sprintf("%d:%s:%d", movieID, profile, audioTrack)
}

// RoomHLSSessionKey returns the HLS session cache key for a watch room.
// The "room:" prefix ensures it never collides with a regular HLSSessionKey,
// which has the form "movieID:profile:audioTrack".
func RoomHLSSessionKey(roomID int64) string {
	return fmt.Sprintf("room:%d", roomID)
}

func (app *Application) RefreshHLSSessionTTL(key string, session *HLSSession) {
	app.HLSSessionCache.Set(key, session, helpers.HLS_SESSION_TTL)
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
	if session.Cmd != nil && session.Cmd.Process != nil {
		session.ExitMu.Lock()
		exited := session.Exited
		session.ExitMu.Unlock()

		if !exited {
			_ = session.Cmd.Process.Kill()

			// Wait for the process to fully exit before removing the temp dir
			// so the OS has released all file handles. Timeout after 2 seconds.
			deadline := time.Now().Add(2 * time.Second)
			for time.Now().Before(deadline) {
				session.ExitMu.Lock()
				exited = session.Exited
				session.ExitMu.Unlock()
				if exited {
					break
				}
				time.Sleep(50 * time.Millisecond)
			}
		}
	}
	if session.TempDir != "" {
		_ = os.RemoveAll(session.TempDir)
	}
}

func (app *Application) startHLSSession(params hlsSessionStartParams) (*HLSSession, error) {
	videoCodec := strings.ToLower(params.PrimaryVideo.Codec)
	audioCodec := strings.ToLower(params.SelectedAudio.Codec)
	sourceIsHDR := isHDRStream(params.PrimaryVideo)
	copyVideo := params.EffectiveProfile == helpers.HLS_PROFILE_REMUX
	copyAudio := audioCodec == "aac"
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

	startSegment := int64(params.StartSec / float64(helpers.HLS_SEGMENT_TIME_SEC))
	session := &HLSSession{
		TempDir:          tempDir,
		DurationSec:      params.DurationSec,
		StartSec:         params.StartSec,
		StartSegment:     startSegment,
		RequestedProfile: params.RequestedProfile,
		EffectiveProfile: params.EffectiveProfile,
		CopyVideo:        copyVideo,
	}

	videoStreamIndex := int(params.PrimaryVideo.StreamIndex)
	audioStreamIndex := int(params.SelectedAudio.StreamIndex)

	app.Logger.Info("hls session starting",
		"movie_id", params.Movie.ID,
		"requested_profile", params.RequestedProfile,
		"effective_profile", params.EffectiveProfile,
		"audio_track", params.AudioTrack,
		"start_sec", params.StartSec,
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
		session.Exited = true
		session.ExitErr = exitErr
		session.ExitMu.Unlock()

		elapsed := time.Since(startTime).Round(time.Second)

		if exitErr != nil {
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

	cmd, err := app.FFmpeg.RunHLS(context.Background(), ffmpeg.HLSParams{
		SourcePath:       params.Movie.FilePath,
		OutDir:           tempDir,
		Profile:          params.EffectiveProfile,
		VideoStreamIndex: videoStreamIndex,
		AudioStreamIndex: audioStreamIndex,
		HWDevice:         hwDevice,
		CopyVideo:        copyVideo,
		CopyAudio:        copyAudio,
		StartSec:         params.StartSec,
		TonemapHDR:       tonemapHDR,
	}, onExit)
	if err != nil {
		cleanupHLSSession(session)
		return nil, fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	session.Cmd = cmd
	return session, nil
}

// GetOrCreateHLSSession returns a cached session or creates a new one.
// If a cached session exists but its StartSec differs from the requested
// startSec, the old session is evicted and a new one is created so FFmpeg
// begins encoding from the requested offset.
// Uses singleflight to deduplicate concurrent creation for the same key.
func (app *Application) GetOrCreateHLSSession(
	ctx context.Context,
	movieID int64,
	profile string,
	audioTrack int,
	startSec float64,
) (*HLSSession, error) {
	key := HLSSessionKey(movieID, profile, audioTrack)

	if raw, ok := app.HLSSessionCache.Get(key); ok {
		session := raw.(*HLSSession)
		if session.StartSec == startSec {
			app.RefreshHLSSessionTTL(key, session)
			return session, nil
		}
		app.HLSSessionCache.Delete(key)
	}

	v, err, _ := app.HLSSessionGroup.Do(key, func() (interface{}, error) {
		if raw, ok := app.HLSSessionCache.Get(key); ok {
			existing := raw.(*HLSSession)
			if existing.StartSec == startSec {
				return existing, nil
			}
			app.HLSSessionCache.Delete(key)
		}

		session, createErr := app.createHLSSession(ctx, movieID, profile, audioTrack, startSec)
		if createErr != nil {
			return nil, createErr
		}

		app.HLSSessionCache.Set(key, session, helpers.HLS_SESSION_TTL)
		return session, nil
	})

	if err != nil {
		return nil, err
	}
	return v.(*HLSSession), nil
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

		session, createErr := app.createHLSSession(ctx, movieID, profile, audioTrack, 0)
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
	raw, ok := app.HLSSessionCache.Get(key)
	if ok {
		app.HLSSessionCache.Delete(key)
	}
	app.RoomHLSMu.Unlock()

	if !ok {
		return
	}
	if session, ok := raw.(*HLSSession); ok {
		cleanupHLSSession(session)
	}
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
	audioTrack int,
	startSec float64,
) (*HLSSession, error) {
	movie, err := app.Queries.GetMovieByID(ctx, movieID)
	if err != nil {
		return nil, fmt.Errorf("movie not found: %w", err)
	}

	if !movie.Duration.Valid || movie.Duration.Float64 <= 0 {
		return nil, fmt.Errorf("movie %d has no valid duration in the database", movieID)
	}
	durationSec := movie.Duration.Float64

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
	if audioTrack < 0 || audioTrack >= len(audioStreams) {
		return nil, fmt.Errorf("audio track %d out of range (0–%d)", audioTrack, len(audioStreams)-1)
	}

	primaryVideo := videoStreams[0]
	selectedAudio := audioStreams[audioTrack]
	requestedProfile := profile
	effectiveProfile := profile
	fallbackProfile := helpers.BestFitHLSFallbackProfile(primaryVideo.Height)
	videoCodec := strings.ToLower(strings.TrimSpace(primaryVideo.Codec))
	safetyCacheKey := remuxSafetyFingerprint(movie, primaryVideo)
	needsRemuxPreflight := false

	if requestedProfile == helpers.HLS_PROFILE_REMUX {
		if !helpers.IsBrowserCompatibleH264(videoCodec) {
			fallbackReason := fmt.Sprintf(
				"requested remux is not supported for codec %q",
				primaryVideo.Codec,
			)
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

	session, err := app.startHLSSession(hlsSessionStartParams{
		Movie:            movie,
		PrimaryVideo:     primaryVideo,
		SelectedAudio:    selectedAudio,
		RequestedProfile: requestedProfile,
		EffectiveProfile: effectiveProfile,
		AudioTrack:       audioTrack,
		StartSec:         startSec,
		DurationSec:      durationSec,
	})
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
		app.setRemuxSafetyVerdict(safetyCacheKey, false, fallbackReason)
		app.Logger.Warn("remux safety fallback engaged",
			"movie_id", movieID,
			"requested_profile", requestedProfile,
			"effective_profile", fallbackProfile,
			"validation_result", "unsafe",
			"fallback_reason", fallbackReason,
		)
		cleanupHLSSession(session)
		return app.startHLSSession(hlsSessionStartParams{
			Movie:            movie,
			PrimaryVideo:     primaryVideo,
			SelectedAudio:    selectedAudio,
			RequestedProfile: requestedProfile,
			EffectiveProfile: fallbackProfile,
			AudioTrack:       audioTrack,
			StartSec:         startSec,
			DurationSec:      durationSec,
		})
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
		return app.startHLSSession(hlsSessionStartParams{
			Movie:            movie,
			PrimaryVideo:     primaryVideo,
			SelectedAudio:    selectedAudio,
			RequestedProfile: requestedProfile,
			EffectiveProfile: fallbackProfile,
			AudioTrack:       audioTrack,
			StartSec:         startSec,
			DurationSec:      durationSec,
		})
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
