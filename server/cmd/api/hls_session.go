package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"time"

	"igloo/cmd/internal/ffmpeg"
	"igloo/cmd/internal/helpers"
)

// HLSSession holds the state for one HLS transcoding session.
// Session key is (movie_id, profile, audio_track); see HLSSessionKey.
// Segment URL design (§12.1): we use (movie_id, profile, audio_track) as the cache key.
// Segment and manifest requests include profile and audio_track (e.g. query ?audio_track=0)
// so the handler can look up the session with the same key. No opaque session id.
type HLSSession struct {
	// OutDir is the temp directory containing init.mp4, playlist.m3u8, segment_*.m4s.
	OutDir string
	// Cmd is the running (or exited) FFmpeg process. Kill on cleanup.
	Cmd *exec.Cmd
	// mu guards ExitErr.
	mu sync.Mutex
	// ExitErr is set by the FFmpeg onExit callback when the process exits.
	// The manifest handler should return 400 with this message if set (per §12.4).
	ExitErr error
}

// HLSSessionKey returns the cache key for the given (movieID, profile, audioTrack).
// Used consistently for cache Get/Set and singleflight.
func HLSSessionKey(movieID int64, profile string, audioTrack int) string {
	return fmt.Sprintf("%d:%s:%d", movieID, profile, audioTrack)
}

// GetOrCreateHLSSession returns an existing session from the cache (refreshing TTL)
// or creates one via singleflight. Only one creation runs per key; others wait and
// receive the same result. On creation failure the session is not stored.
func (app *Application) GetOrCreateHLSSession(ctx context.Context, movieID int64, profile string, audioTrack int) (*HLSSession, error) {
	if !helpers.IsAllowedHLSProfile(profile) {
		return nil, fmt.Errorf("invalid HLS profile %q", profile)
	}

	key := HLSSessionKey(movieID, profile, audioTrack)

	if v, ok := app.HLSSessionCache.Get(key); ok {
		session := v.(*HLSSession)
		app.HLSSessionCache.Set(key, session, 30*time.Minute)
		return session, nil
	}

	v, err, _ := app.HLSSessionGroup.Do(key, func() (any, error) {
		return app.createHLSSession(ctx, key, movieID, profile, audioTrack)
	})
	if err != nil {
		return nil, err
	}
	return v.(*HLSSession), nil
}

// createHLSSession allocates a temp dir, starts FFmpeg HLS, and returns the session.
// Called inside singleflight so only one creation runs per key.
func (app *Application) createHLSSession(ctx context.Context, key string, movieID int64, profile string, audioTrack int) (*HLSSession, error) {
	// Double-check cache after singleflight (another goroutine may have created it).
	if v, ok := app.HLSSessionCache.Get(key); ok {
		return v.(*HLSSession), nil
	}

	movie, err := app.Queries.GetMovieByID(ctx, movieID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("movie not found")
		}
		return nil, fmt.Errorf("get movie: %w", err)
	}

	outDir, err := os.MkdirTemp("", "igloo-hls-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}

	session := &HLSSession{OutDir: outDir}
	runCfg := &ffmpeg.RunHLSConfig{
		Ctx:           ctx,
		Log:           app.Logger,
		OnExit:        session.setExitErr,
		SourcePath:    movie.FilePath,
		OutDir:        outDir,
		Profile:       profile,
		AudioTrackIdx: audioTrack,
		VideoTrackIdx: 0,
		HWDevice:      app.hlsHWDevice(),
		UseFastPath:   false,
	}

	cmd, err := app.FFmpeg.RunHLS(runCfg)
	if err != nil {
		os.RemoveAll(outDir)
		return nil, fmt.Errorf("start ffmpeg: %w", err)
	}
	session.Cmd = cmd

	app.HLSSessionCache.Set(key, session, 30*time.Minute)
	return session, nil
}

func (s *HLSSession) setExitErr(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ExitErr = err
}

// ExitError returns the FFmpeg exit error if the process has exited with an error.
func (s *HLSSession) ExitError() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ExitErr
}

// CleanupHLSSession kills the FFmpeg process (if running) and deletes the session temp dir.
// Safe to call multiple times; idempotent after the first call.
func CleanupHLSSession(session *HLSSession, log *slog.Logger) {
	if session == nil {
		return
	}
	if session.Cmd != nil && session.Cmd.Process != nil {
		if err := session.Cmd.Process.Kill(); err != nil {
			log.Debug("hls session cleanup: kill ffmpeg", "error", err)
		}
		_ = session.Cmd.Wait()
	}
	if session.OutDir != "" {
		if err := os.RemoveAll(session.OutDir); err != nil {
			log.Debug("hls session cleanup: remove temp dir", "dir", session.OutDir, "error", err)
		}
	}
}

// RefreshHLSSessionTTL refreshes the cache TTL for the given key and session.
// Call after each manifest or segment request so the session lives 30 minutes from last activity.
func (app *Application) RefreshHLSSessionTTL(key string, session *HLSSession) {
	if key != "" && session != nil {
		app.HLSSessionCache.Set(key, session, 30*time.Minute)
	}
}

// hlsHWDevice returns the hardware acceleration device from settings (e.g. cpu, apple).
func (app *Application) hlsHWDevice() string {
	if app.Settings != nil && app.Settings.HardwareAccelerationDevice.Valid && app.Settings.HardwareAccelerationDevice.String != "" {
		return app.Settings.HardwareAccelerationDevice.String
	}
	return helpers.HARDWARE_ACCELERATION_DEVICE_CPU
}
