package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/helpers"
)

// HLSSession holds state for one HLS transcode session.
type HLSSession struct {
	TempDir       string
	Cmd           *exec.Cmd
	DurationSec   float64
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

// evictOtherHLSSessions kills FFmpeg and removes temp files for all cached
// sessions whose key differs from keepKey.  This prevents hardware-encoder
// contention (e.g. macOS videotoolbox supports only one concurrent encode).
func (app *Application) evictOtherHLSSessions(keepKey string) {
	for key, item := range app.HLSSessionCache.Items() {
		if key == keepKey {
			continue
		}
		if session, ok := item.Object.(*HLSSession); ok {
			app.Logger.Info("evicting stale hls session", "key", key)
			cleanupHLSSession(session)
		}
		app.HLSSessionCache.Delete(key)
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

		app.evictOtherHLSSessions(key)

		session, createErr := app.createHLSSession(ctx, movieID, profile, audioTrack)
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
			if helpers.IsCoverArtVideoCodec(s.CodecName) {
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

	chosenVideoGlobal := videoStreams[0]
	videoStreamIndex := videoStreamOrdinal(meta.Streams, chosenVideoGlobal)
	if videoStreamIndex < 0 {
		return nil, fmt.Errorf("could not resolve video stream index for HLS mapping")
	}

	audioStreamIndex := audioTrack

	audioCodec := strings.ToLower(meta.Streams[audioStreams[audioTrack]].CodecName)
	copyAudio := audioCodec == "aac"
	copyVideo := strings.EqualFold(meta.Streams[videoStreams[0]].CodecName, "h264")

	// Remux always copies video; the frontend only offers this when the
	// source video codec is browser-compatible (H.264).
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

	session := &HLSSession{TempDir: tempDir, DurationSec: durationSec}

	videoCodec := meta.Streams[videoStreams[0]].CodecName

	app.Logger.Info("hls session starting",
		"movie_id", movieID,
		"profile", profile,
		"audio_track", audioTrack,
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

	cmd, err := app.FFmpeg.RunHLS(context.Background(), movie.FilePath, tempDir, profile, videoStreamIndex, audioStreamIndex, hwDevice, copyVideo, copyAudio, onExit)
	if err != nil {
		cleanupHLSSession(session)
		return nil, fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	session.Cmd = cmd
	return session, nil
}

// videoStreamOrdinal returns N for FFmpeg -map 0:v:N where N is the 0-based
// index among all video streams in the container in file order.
func videoStreamOrdinal(streams []ffprobe.Stream, globalIndex int) int {
	n := 0
	for i, s := range streams {
		if s.CodecType != "video" {
			continue
		}
		if i == globalIndex {
			return n
		}
		n++
	}
	return -1
}
