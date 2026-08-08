package main

import (
	"context"
	"database/sql"
	"errors"
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

// movieStreamFingerprintBase captures file identity for one video stream:
// movie, stream, size, and update timestamp. movie.UpdatedAt bumps on scanner
// metadata upserts for re-processed files too, which re-pays one computation
// per file — acceptable, since re-processing implies the file's size or path
// changed. Shared by every per-stream persistence fingerprint.
func movieStreamFingerprintBase(movie *database.Movie, streamIndex int64) string {
	return fmt.Sprintf("%d:%d:%d:%s", movie.ID, streamIndex, movie.Size, movie.UpdatedAt)
}

// remuxVerdictProducerRevision versions everything on our side of the verdict:
// the remux FFmpeg arguments and ValidateRemuxSafety itself. Bump it whenever
// either changes, so verdicts recorded against the old behavior are discarded.
const remuxVerdictProducerRevision = 1

// remuxSafetyFingerprint keys the persisted remux-safety verdict. Beyond file
// identity it includes the stream properties the safety decision reads
// (isBrowserSafeH264RemuxCandidate), so a rescan that changes stream rows
// without touching the file invalidates the verdict.
//
// A verdict validates FFmpeg-generated fMP4 output, not just the source, so the
// producer is part of the key too: ffmpegVersion covers an upgraded embedded
// payload or a swapped PATH binary, and remuxVerdictProducerRevision covers our
// own argument and validator changes. A mismatch fails open into a fresh
// preflight, which costs one re-validation per file.
func remuxSafetyFingerprint(
	movie *database.Movie,
	video *database.VideoStream,
	ffmpegVersion string,
) string {
	return fmt.Sprintf(
		"%s:%s:%s:%d:%s:%s:p%d:%s",
		movieStreamFingerprintBase(movie, video.StreamIndex),
		video.Codec,
		video.CodecProfile.String,
		video.BitDepth.Int64,
		video.PixelFormat.String,
		video.FieldOrder.String,
		remuxVerdictProducerRevision,
		ffmpegVersion,
	)
}

// getRemuxSafetyVerdict reads the persisted verdict for one video stream. Any
// miss — no row, stale fingerprint, or a read error — fails open into a fresh
// preflight, which rewrites the row with a current verdict.
func (app *Application) getRemuxSafetyVerdict(
	ctx context.Context,
	movieID int64,
	streamIndex int64,
	fingerprint string,
) (remuxSafetyVerdict, bool) {
	row, err := app.Queries.GetRemuxSafetyVerdict(ctx, database.GetRemuxSafetyVerdictParams{
		MovieID:     movieID,
		StreamIndex: streamIndex,
	})
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			app.Logger.Warn("failed to read remux safety verdict",
				"movie_id", movieID,
				"stream_index", streamIndex,
				"error", err,
			)
		}
		return remuxSafetyVerdict{}, false
	}

	if row.Fingerprint != fingerprint {
		return remuxSafetyVerdict{}, false
	}

	return remuxSafetyVerdict{
		Safe:   row.Safe,
		Reason: row.Reason,
	}, true
}

// setRemuxSafetyVerdict persists a definitive validation result. It runs on
// context.Background() because the verdict just cost a multi-second FFmpeg
// preflight and must not be lost to a client disconnect (FFmpeg itself already
// runs on a background context for the same reason). A write failure only
// means the verdict is recomputed on the next play.
func (app *Application) setRemuxSafetyVerdict(
	movieID int64,
	streamIndex int64,
	fingerprint string,
	safe bool,
	reason string,
) {
	err := app.Queries.UpsertRemuxSafetyVerdict(context.Background(), database.UpsertRemuxSafetyVerdictParams{
		MovieID:     movieID,
		StreamIndex: streamIndex,
		Fingerprint: fingerprint,
		Safe:        safe,
		Reason:      reason,
	})
	if err != nil {
		app.Logger.Warn("failed to persist remux safety verdict",
			"movie_id", movieID,
			"stream_index", streamIndex,
			"error", err,
		)
	}
}

func waitForRemuxPreflight(session *HLSSession, segmentCount int, timeout time.Duration) error {
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

		time.Sleep(hlsRemuxPreflightPoll)
	}

	return fmt.Errorf(
		"timed out waiting for %d complete remux segments after %s",
		segmentCount,
		timeout,
	)
}
