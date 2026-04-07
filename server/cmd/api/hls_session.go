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

	"igloo/cmd/internal/ffmpeg"
	"igloo/cmd/internal/helpers"
)

// HLSSession holds state for one HLS transcode session.
type HLSSession struct {
	TempDir       string
	Cmd           *exec.Cmd
	DurationSec   float64
	StartSec      float64
	StartSegment  int64
	Exited        bool
	ExitErr       error
	FinalPlaylist string
	ExitMu        sync.Mutex
}

func HLSSessionKey(movieID int64, profile string, audioTrack int) string {
	return fmt.Sprintf("%d:%s:%d", movieID, profile, audioTrack)
}

func (app *Application) RefreshHLSSessionTTL(key string, session *HLSSession) {
	app.HLSSessionCache.Set(key, session, helpers.HLS_SESSION_TTL)
}

func cleanupHLSSession(session *HLSSession) {
	if session == nil {
		return
	}
	if session.Cmd != nil && session.Cmd.Process != nil {
		_ = session.Cmd.Process.Kill()
	}
	if session.TempDir != "" {
		_ = os.RemoveAll(session.TempDir)
	}
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

	videoCodec := strings.ToLower(primaryVideo.Codec)
	audioCodec := strings.ToLower(selectedAudio.Codec)
	copyVideo := videoCodec == "h264"
	copyAudio := audioCodec == "aac"

	if profile == helpers.HLS_PROFILE_REMUX {
		copyVideo = true
	}

	hwDevice := helpers.HARDWARE_ACCELERATION_DEVICE_CPU
	if app.Settings.HardwareAccelerationDevice.Valid && app.Settings.HardwareAccelerationDevice.String != "" {
		hwDevice = app.Settings.HardwareAccelerationDevice.String
	}

	tempDir, err := os.MkdirTemp("", "igloo-hls-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}

	startSegment := int64(startSec / float64(helpers.HLS_SEGMENT_TIME_SEC))
	session := &HLSSession{
		TempDir:      tempDir,
		DurationSec:  durationSec,
		StartSec:     startSec,
		StartSegment: startSegment,
	}

	videoStreamIndex := int(primaryVideo.StreamIndex)
	audioStreamIndex := int(selectedAudio.StreamIndex)

	app.Logger.Info("hls session starting",
		"movie_id", movieID,
		"profile", profile,
		"audio_track", audioTrack,
		"start_sec", startSec,
		"start_segment", startSegment,
		"video_stream_index", videoStreamIndex,
		"audio_stream_index", audioStreamIndex,
		"video_codec", videoCodec,
		"audio_codec", audioCodec,
		"copy_video", copyVideo,
		"copy_audio", copyAudio,
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
				"movie_id", movieID,
				"profile", profile,
				"elapsed", elapsed.String(),
				"error", exitErr.Error(),
				"ffmpeg_tail", strings.Join(stderrTail, "\n"),
			)
		} else {
			app.Logger.Info("hls session finished",
				"movie_id", movieID,
				"profile", profile,
				"elapsed", elapsed.String(),
			)
		}
	}

	cmd, err := app.FFmpeg.RunHLS(context.Background(), ffmpeg.HLSParams{
		SourcePath:       movie.FilePath,
		OutDir:           tempDir,
		Profile:          profile,
		VideoStreamIndex: videoStreamIndex,
		AudioStreamIndex: audioStreamIndex,
		HWDevice:         hwDevice,
		CopyVideo:        copyVideo,
		CopyAudio:        copyAudio,
		StartSec:         startSec,
	}, onExit)
	if err != nil {
		cleanupHLSSession(session)
		return nil, fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	session.Cmd = cmd
	return session, nil
}
