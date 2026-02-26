package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"igloo/cmd/internal/helpers"
)

const hlsSessionTTL = 30 * time.Minute

// HLSSession holds state for one HLS transcode session.
type HLSSession struct {
	TempDir    string
	Cmd        *exec.Cmd
	DurationSec float64
	ExitErr    error
	ExitMu     sync.Mutex
}

func HLSSessionKey(movieID int64, profile string, audioTrack int) string {
	return fmt.Sprintf("%d:%s:%d", movieID, profile, audioTrack)
}

func (app *Application) RefreshHLSSessionTTL(key string, session *HLSSession) {
	app.HLSSessionCache.Set(key, session, hlsSessionTTL)
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
// Uses singleflight to deduplicate concurrent creation for the same key.
func (app *Application) GetOrCreateHLSSession(
	ctx context.Context,
	movieID int64,
	profile string,
	audioTrack int,
) (*HLSSession, error) {
	key := HLSSessionKey(movieID, profile, audioTrack)

	if raw, ok := app.HLSSessionCache.Get(key); ok {
		session := raw.(*HLSSession)
		app.RefreshHLSSessionTTL(key, session)
		return session, nil
	}

	v, err, _ := app.HLSSessionGroup.Do(key, func() (interface{}, error) {
		if raw, ok := app.HLSSessionCache.Get(key); ok {
			return raw.(*HLSSession), nil
		}

		session, createErr := app.createHLSSession(ctx, movieID, profile, audioTrack)
		if createErr != nil {
			return nil, createErr
		}

		app.HLSSessionCache.Set(key, session, hlsSessionTTL)
		return session, nil
	})

	if err != nil {
		return nil, err
	}
	return v.(*HLSSession), nil
}

// createHLSSession probes the movie, creates a temp dir, and starts FFmpeg.
//
// FFmpeg runs on context.Background() so the process outlives the originating
// HTTP request. The session cache (with TTL + eviction) owns the lifecycle.
func (app *Application) createHLSSession(
	ctx context.Context,
	movieID int64,
	profile string,
	audioTrack int,
) (*HLSSession, error) {
	movie, err := app.Queries.GetMovieByID(ctx, movieID)
	if err != nil {
		return nil, fmt.Errorf("movie not found: %w", err)
	}

	meta, err := app.Ffprobe.GetMetadata(movie.FilePath)
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w", err)
	}

	durationSec, _ := strconv.ParseFloat(meta.Format.Duration, 64)
	if durationSec <= 0 {
		return nil, fmt.Errorf("could not determine file duration")
	}

	var videoStreams []int
	var audioStreams []int
	for i, s := range meta.Streams {
		switch s.CodecType {
		case "video":
			if s.Disposition.AttachedPic == 1 {
				continue
			}
			videoStreams = append(videoStreams, i)
		case "audio":
			audioStreams = append(audioStreams, i)
		}
	}

	if len(videoStreams) == 0 {
		return nil, fmt.Errorf("no playable video track found")
	}
	if audioTrack < 0 || audioTrack >= len(audioStreams) {
		return nil, fmt.Errorf("audio track %d out of range (0–%d)", audioTrack, len(audioStreams)-1)
	}

	videoStreamIndex := 0
	audioStreamIndex := audioTrack

	copyVideo := strings.EqualFold(meta.Streams[videoStreams[0]].CodecName, "h264")
	copyAudio := strings.EqualFold(meta.Streams[audioStreams[audioTrack]].CodecName, "aac")

	hwDevice := helpers.HARDWARE_ACCELERATION_DEVICE_CPU
	if app.Settings.HardwareAccelerationDevice.Valid && app.Settings.HardwareAccelerationDevice.String != "" {
		hwDevice = app.Settings.HardwareAccelerationDevice.String
	}

	tempDir, err := os.MkdirTemp("", "igloo-hls-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}

	session := &HLSSession{TempDir: tempDir, DurationSec: durationSec}

	onExit := func(exitErr error) {
		session.ExitMu.Lock()
		session.ExitErr = exitErr
		session.ExitMu.Unlock()
	}

	logStderr := func(line string) {
		app.Logger.Debug("hls ffmpeg", "movie_id", movieID, "line", line)
	}

	cmd, err := app.FFmpeg.RunHLS(context.Background(), movie.FilePath, tempDir, profile, videoStreamIndex, audioStreamIndex, hwDevice, copyVideo, copyAudio, onExit, logStderr)
	if err != nil {
		cleanupHLSSession(session)
		return nil, fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	session.Cmd = cmd
	return session, nil
}
