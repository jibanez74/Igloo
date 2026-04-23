package main

import (
	"fmt"
	"path/filepath"
	"time"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
)

type remuxSafetyVerdict struct {
	Safe   bool
	Reason string
}

func remuxSafetyFingerprint(movie *database.Movie, video *database.VideoStream) string {
	return fmt.Sprintf(
		"%d:%d:%d:%s",
		movie.ID,
		video.StreamIndex,
		movie.Size,
		movie.UpdatedAt,
	)
}

func (app *Application) getRemuxSafetyVerdict(key string) (remuxSafetyVerdict, bool) {
	if app.RemuxSafetyCache == nil {
		return remuxSafetyVerdict{}, false
	}

	raw, ok := app.RemuxSafetyCache.Get(key)
	if !ok || raw == nil {
		return remuxSafetyVerdict{}, false
	}

	verdict, typeOK := raw.(remuxSafetyVerdict)
	if !typeOK {
		app.RemuxSafetyCache.Delete(key)
		return remuxSafetyVerdict{}, false
	}

	return verdict, true
}

func (app *Application) setRemuxSafetyVerdict(key string, safe bool, reason string) {
	if app.RemuxSafetyCache == nil {
		return
	}

	app.RemuxSafetyCache.SetDefault(key, remuxSafetyVerdict{
		Safe:   safe,
		Reason: reason,
	})
}

func waitForRemuxPreflight(session *HLSSession, segmentCount int, timeout time.Duration) error {
	if session == nil {
		return fmt.Errorf("missing HLS session")
	}
	if segmentCount <= 0 {
		return fmt.Errorf("segment count must be positive")
	}

	initPath := filepath.Join(session.TempDir, helpers.HLS_INIT_FILENAME)
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if fileReady(initPath) {
			allReady := true
			for i := 0; i < segmentCount; i++ {
				name := fmt.Sprintf(
					"%s%d%s",
					helpers.HLS_SEGMENT_FILENAME_PREFIX,
					i,
					helpers.HLS_SEGMENT_FILENAME_SUFFIX,
				)
				if !segmentComplete(session, name) {
					allReady = false
					break
				}
			}
			if allReady {
				return nil
			}
		}

		session.ExitMu.Lock()
		exited := session.Exited
		exitErr := session.ExitErr
		session.ExitMu.Unlock()

		if exited {
			if !fileReady(initPath) {
				if exitErr != nil {
					return fmt.Errorf("init segment was not generated before ffmpeg exit: %w", exitErr)
				}
				return fmt.Errorf("init segment was not generated before ffmpeg exit")
			}

			for i := 0; i < segmentCount; i++ {
				name := fmt.Sprintf(
					"%s%d%s",
					helpers.HLS_SEGMENT_FILENAME_PREFIX,
					i,
					helpers.HLS_SEGMENT_FILENAME_SUFFIX,
				)
				if !segmentComplete(session, name) {
					if exitErr != nil {
						return fmt.Errorf("segment %q was not completed before ffmpeg exit: %w", name, exitErr)
					}
					return fmt.Errorf("segment %q was not completed before ffmpeg exit", name)
				}
			}
		}

		time.Sleep(helpers.HLS_SEGMENT_POLL)
	}

	return fmt.Errorf(
		"timed out waiting for %d complete remux segments after %s",
		segmentCount,
		timeout,
	)
}
