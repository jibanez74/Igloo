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

// sourceStreamFingerprintBase captures file identity for one video stream:
// file, stream, size, and update timestamp. UpdatedAt bumps on scanner
// metadata upserts for re-processed files too, which re-pays one computation
// per file — acceptable, since re-processing implies the file's size or path
// changed. Shared by every per-stream persistence fingerprint. The file id is
// the movie id or show_files.id; the two kinds persist to separate tables, so
// the format needs no kind and stays byte-identical to what movie rows carry.
func sourceStreamFingerprintBase(source playbackSource, streamIndex int64) string {
	return fmt.Sprintf("%d:%d:%d:%s", source.FileID, streamIndex, source.Size, source.UpdatedAt)
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
	source playbackSource,
	video *database.VideoStream,
	ffmpegVersion string,
) string {
	return fmt.Sprintf(
		"%s:%s:%s:%d:%s:%s:p%d:%s",
		sourceStreamFingerprintBase(source, video.StreamIndex),
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
// persistedRemuxVerdict is the stored row shape shared by the movie and show
// tables.
type persistedRemuxVerdict struct {
	Fingerprint string
	Safe        bool
	Reason      string
}

// readRemuxVerdictRow reads the row from the table the source's kind keys on.
func (app *Application) readRemuxVerdictRow(ctx context.Context, source playbackSource, streamIndex int64) (persistedRemuxVerdict, error) {
	if source.Ref.Kind == mediaKindEpisode {
		row, err := app.Queries.GetShowRemuxSafetyVerdict(ctx, database.GetShowRemuxSafetyVerdictParams{
			FileID:      source.FileID,
			StreamIndex: streamIndex,
		})
		if err != nil {
			return persistedRemuxVerdict{}, err
		}
		return persistedRemuxVerdict{Fingerprint: row.Fingerprint, Safe: row.Safe, Reason: row.Reason}, nil
	}

	row, err := app.Queries.GetRemuxSafetyVerdict(ctx, database.GetRemuxSafetyVerdictParams{
		MovieID:     source.FileID,
		StreamIndex: streamIndex,
	})
	if err != nil {
		return persistedRemuxVerdict{}, err
	}
	return persistedRemuxVerdict{Fingerprint: row.Fingerprint, Safe: row.Safe, Reason: row.Reason}, nil
}

// writeRemuxVerdictRow is the upsert twin of readRemuxVerdictRow.
func (app *Application) writeRemuxVerdictRow(ctx context.Context, source playbackSource, streamIndex int64, row persistedRemuxVerdict) error {
	if source.Ref.Kind == mediaKindEpisode {
		return app.Queries.UpsertShowRemuxSafetyVerdict(ctx, database.UpsertShowRemuxSafetyVerdictParams{
			FileID:      source.FileID,
			StreamIndex: streamIndex,
			Fingerprint: row.Fingerprint,
			Safe:        row.Safe,
			Reason:      row.Reason,
		})
	}

	return app.Queries.UpsertRemuxSafetyVerdict(ctx, database.UpsertRemuxSafetyVerdictParams{
		MovieID:     source.FileID,
		StreamIndex: streamIndex,
		Fingerprint: row.Fingerprint,
		Safe:        row.Safe,
		Reason:      row.Reason,
	})
}

func (app *Application) getRemuxSafetyVerdict(
	ctx context.Context,
	source playbackSource,
	streamIndex int64,
	fingerprint string,
) (remuxSafetyVerdict, bool) {
	row, err := app.readRemuxVerdictRow(ctx, source, streamIndex)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			app.Logger.Warn("failed to read remux safety verdict",
				"media", source.Ref.String(),
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
	source playbackSource,
	streamIndex int64,
	fingerprint string,
	safe bool,
	reason string,
) {
	err := app.writeRemuxVerdictRow(context.Background(), source, streamIndex, persistedRemuxVerdict{
		Fingerprint: fingerprint,
		Safe:        safe,
		Reason:      reason,
	})
	if err != nil {
		app.Logger.Warn("failed to persist remux safety verdict",
			"media", source.Ref.String(),
			"stream_index", streamIndex,
			"error", err,
		)
	}
}

// waitForRemuxPreflight blocks until the session has produced init.mp4 and
// the first segmentCount complete segments, or FFmpeg has exited. It returns
// how many leading segments may be validated: segmentCount normally, fewer
// when FFmpeg exited cleanly with at least one (a short file, or a start near
// the end of the media). An exit with an error, or a clean exit with nothing
// complete, is a failure whichever segment it stopped at.
func waitForRemuxPreflight(session *HLSSession, segmentCount int, timeout time.Duration) (int, error) {
	initPath := filepath.Join(session.TempDir, helpers.HLS_INIT_FILENAME)
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if fileReady(initPath) {
			completed := completedLeadingHLSSegments(session, segmentCount)
			if completed == segmentCount {
				return segmentCount, nil
			}
		}

		session.ExitMu.Lock()
		exited := session.Exited
		exitErr := session.ExitErr
		session.ExitMu.Unlock()

		if exited {
			if !fileReady(initPath) {
				if exitErr != nil {
					return 0, fmt.Errorf("init segment was not generated before ffmpeg exit: %w", exitErr)
				}
				return 0, fmt.Errorf("init segment was not generated before ffmpeg exit")
			}

			completed := completedLeadingHLSSegments(session, segmentCount)
			if completed == segmentCount {
				return segmentCount, nil
			}
			cleanExit := exitErr == nil
			if cleanExit && completed > 0 {
				return completed, nil
			}

			name := hlsSegmentFilename(completed)
			if exitErr != nil {
				return 0, fmt.Errorf("segment %q was not completed before ffmpeg exit: %w", name, exitErr)
			}
			return 0, fmt.Errorf("segment %q was not completed before ffmpeg exit", name)
		}

		time.Sleep(hlsRemuxPreflightPoll)
	}

	return 0, fmt.Errorf(
		"timed out waiting for %d complete remux segments after %s",
		segmentCount,
		timeout,
	)
}

// completedLeadingHLSSegments counts complete segments from segment 0 upward,
// stopping at the first that is not, so the result is a prefix the validator
// can read in order.
func completedLeadingHLSSegments(session *HLSSession, limit int) int {
	for i := 0; i < limit; i++ {
		if !segmentReady(session, hlsSegmentFilename(i)) {
			return i
		}
	}
	return limit
}

func hlsSegmentFilename(index int) string {
	return fmt.Sprintf("%s%d%s", helpers.HLS_SEGMENT_FILENAME_PREFIX, index, helpers.HLS_SEGMENT_FILENAME_SUFFIX)
}
