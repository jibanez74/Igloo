package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"time"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/keyframeindex"

	"golang.org/x/sync/singleflight"
)

// hlsStartProbeTimeout is the budget each of the two stages that resolve a
// session's real start second gets: the keyframe extraction, and the ffprobe
// fallback after it. Both are advisory, so exceeding it costs the session
// nothing but an unknown start.
const hlsStartProbeTimeout = 15 * time.Second

// keyframeIndexFingerprint keys the persisted keyframe index. The index
// depends only on the file's bytes, so unlike remuxSafetyFingerprint no
// stream properties are included — file identity alone invalidates it.
func keyframeIndexFingerprint(source playbackSource, video *database.VideoStream) string {
	return sourceStreamFingerprintBase(source, video.StreamIndex)
}

// persistedKeyframeIndex is the stored row shape shared by the movie and show
// tables, so the validation below reads one type whichever table it came from.
type persistedKeyframeIndex struct {
	Fingerprint string
	DurationSec float64
	Keyframes   string
}

// readKeyframeIndexRow reads the row from the table the source's kind keys
// on: movies by movie id, show files by show_files.id.
func (app *Application) readKeyframeIndexRow(ctx context.Context, source playbackSource, streamIndex int64) (persistedKeyframeIndex, error) {
	if source.Ref.Kind == mediaKindEpisode {
		row, err := app.Queries.GetShowKeyframeIndex(ctx, database.GetShowKeyframeIndexParams{
			FileID:      source.FileID,
			StreamIndex: streamIndex,
		})
		if err != nil {
			return persistedKeyframeIndex{}, err
		}
		return persistedKeyframeIndex{Fingerprint: row.Fingerprint, DurationSec: row.DurationSec, Keyframes: row.Keyframes}, nil
	}

	row, err := app.Queries.GetKeyframeIndex(ctx, database.GetKeyframeIndexParams{
		MovieID:     source.FileID,
		StreamIndex: streamIndex,
	})
	if err != nil {
		return persistedKeyframeIndex{}, err
	}
	return persistedKeyframeIndex{Fingerprint: row.Fingerprint, DurationSec: row.DurationSec, Keyframes: row.Keyframes}, nil
}

// writeKeyframeIndexRow is the upsert twin of readKeyframeIndexRow.
func (app *Application) writeKeyframeIndexRow(ctx context.Context, source playbackSource, streamIndex int64, row persistedKeyframeIndex) error {
	if source.Ref.Kind == mediaKindEpisode {
		return app.Queries.UpsertShowKeyframeIndex(ctx, database.UpsertShowKeyframeIndexParams{
			FileID:      source.FileID,
			StreamIndex: streamIndex,
			Fingerprint: row.Fingerprint,
			DurationSec: row.DurationSec,
			Keyframes:   row.Keyframes,
		})
	}

	return app.Queries.UpsertKeyframeIndex(ctx, database.UpsertKeyframeIndexParams{
		MovieID:     source.FileID,
		StreamIndex: streamIndex,
		Fingerprint: row.Fingerprint,
		DurationSec: row.DurationSec,
		Keyframes:   row.Keyframes,
	})
}

// getKeyframeIndex reads the persisted index for one video stream. Any miss —
// no row, stale fingerprint, or an unreadable payload — sends the caller to a
// fresh extraction, which rewrites the row.
func (app *Application) getKeyframeIndex(
	ctx context.Context,
	source playbackSource,
	streamIndex int64,
	fingerprint string,
) (keyframeindex.Index, bool) {
	row, err := app.readKeyframeIndexRow(ctx, source, streamIndex)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			app.Logger.Warn("failed to read keyframe index",
				"media", source.Ref.String(),
				"stream_index", streamIndex,
				"error", err,
			)
		}
		return keyframeindex.Index{}, false
	}

	if row.Fingerprint != fingerprint {
		return keyframeindex.Index{}, false
	}

	var keyframes []float64
	err = json.Unmarshal([]byte(row.Keyframes), &keyframes)
	if err != nil {
		return keyframeindex.Index{}, false
	}

	// Seeks binary-search this slice, so a row that lost the extractor's
	// invariants would answer with the wrong keyframe. Re-establish them here
	// rather than trusting the row: ordering is repaired, values that cannot
	// be a presentation time are rejected, and a rejection just re-extracts.
	idx, err := keyframeindex.Finalize(keyframeindex.Index{
		KeyframeSec: keyframes,
		DurationSec: row.DurationSec,
	})
	if err != nil {
		app.Logger.Warn("discarding invalid persisted keyframe index",
			"media", source.Ref.String(),
			"stream_index", streamIndex,
			"error", err,
		)
		return keyframeindex.Index{}, false
	}

	return idx, true
}

// setKeyframeIndex persists an extracted index on context.Background(), so an
// index that cost a read of the source is not lost to a client disconnect. A
// write failure only means the index is re-extracted on the next play.
func (app *Application) setKeyframeIndex(
	source playbackSource,
	streamIndex int64,
	fingerprint string,
	idx keyframeindex.Index,
) {
	payload, err := json.Marshal(idx.KeyframeSec)
	if err != nil {
		app.Logger.Warn("failed to encode keyframe index",
			"media", source.Ref.String(),
			"stream_index", streamIndex,
			"error", err,
		)
		return
	}

	err = app.writeKeyframeIndexRow(context.Background(), source, streamIndex, persistedKeyframeIndex{
		Fingerprint: fingerprint,
		DurationSec: idx.DurationSec,
		Keyframes:   string(payload),
	})
	if err != nil {
		app.Logger.Warn("failed to persist keyframe index",
			"media", source.Ref.String(),
			"stream_index", streamIndex,
			"error", err,
		)
	}
}

// keyframeAtOrBefore answers a seek from a sorted keyframe list. It reports
// false when targetSec precedes the first keyframe, matching the probing
// path's "no keyframe found" error so the header is simply omitted.
func keyframeAtOrBefore(keyframes []float64, targetSec float64) (float64, bool) {
	idx := sort.SearchFloat64s(keyframes, targetSec)
	if idx < len(keyframes) && keyframes[idx] == targetSec {
		return keyframes[idx], true
	}
	if idx == 0 {
		return 0, false
	}
	return keyframes[idx-1], true
}

type hlsActualStartParams struct {
	Session     *HLSSession
	Source      playbackSource
	StreamIndex int64
	Fingerprint string
	// RequestedStartSec of 0 means prefetch only: populate the index for
	// later seeks, measure nothing (a start of 0 is already exact).
	RequestedStartSec float64
}

// resolveHLSActualStart records where a copy-video session's media actually
// begins: stream copy lands FFmpeg's input seek on the keyframe at or before
// the requested offset, and without this the client's clock and progress
// writes would run up to one GOP ahead of the picture. It reads the container's
// persisted seek index (which -ss consults too) and falls back to a bounded
// ffprobe probe. It runs alongside FFmpeg, is advisory (a failure leaves the
// start unknown), and its parentCtx is deliberately not the session's; see the
// call site in startHLSSession.
func (app *Application) resolveHLSActualStart(parentCtx context.Context, p hlsActualStartParams) {
	defer app.Wait.Done()

	extractCtx, cancelExtract := context.WithTimeout(parentCtx, hlsStartProbeTimeout)
	defer cancelExtract()

	idx, err := app.extractAndPersistKeyframeIndex(extractCtx, p)
	if err == nil {
		if p.RequestedStartSec > 0 {
			keyframe, ok := keyframeAtOrBefore(idx.KeyframeSec, p.RequestedStartSec)
			if ok {
				p.Session.setActualStartSec(keyframe)
			}
		}
		return
	}

	if errors.Is(err, keyframeindex.ErrUnsupportedContainer) {
		app.Logger.Debug("keyframe index unsupported for container",
			"media", p.Source.Ref.String(),
			"container", p.Source.Container,
		)
	} else {
		app.Logger.Warn("keyframe index extraction failed",
			"media", p.Source.Ref.String(),
			"container", p.Source.Container,
			"error", err,
		)
	}

	// No usable index: keep the previous behavior, a bounded packet probe
	// around the requested start. Its single-point answer is never persisted.
	if p.RequestedStartSec <= 0 || app.Ffprobe == nil {
		return
	}

	// The probe gets its own budget rather than what the extraction left of
	// the shared one. Sharing meant a slow moov read that burned most of the
	// timeout handed the fallback a fraction of a second and it always failed.
	probeCtx, cancelProbe := context.WithTimeout(parentCtx, hlsStartProbeTimeout)
	defer cancelProbe()

	actualStartSec, probeErr := app.Ffprobe.KeyframeAtOrBefore(probeCtx, p.Source.FilePath, p.StreamIndex, p.RequestedStartSec)
	if probeErr != nil {
		app.Logger.Warn("hls actual start probe failed",
			"media", p.Source.Ref.String(),
			"requested_start_sec", p.RequestedStartSec,
			"error", probeErr.Error(),
		)
		return
	}

	p.Session.setActualStartSec(actualStartSec)
}

// extractAndPersistKeyframeIndex extracts and stores a file's index once for
// every session waiting on it. A seek during a file's first copy-video play
// starts a second session before the first one's extraction has persisted
// anything, and each would otherwise read the container's seek tables again.
// Every caller still stops waiting at its own deadline: a persist stalled on
// the database must not hold the others back from their ffprobe fallback. The
// shared work keeps running after that, tracked by app.Wait for shutdown; the
// count it adds to is never zero then, because its caller holds one.
func (app *Application) extractAndPersistKeyframeIndex(ctx context.Context, p hlsActualStartParams) (keyframeindex.Index, error) {
	key := fmt.Sprintf("%s:%d:%d:%s", p.Source.Ref.Kind, p.Source.FileID, p.StreamIndex, p.Fingerprint)
	results := app.KeyframeExtractGroup.DoChan(key, func() (any, error) {
		app.Wait.Add(1)
		defer app.Wait.Done()

		idx, extractErr := app.extractKeyframeIndexFromFile(ctx, p.Source.FilePath, p.Source.Container)
		if extractErr != nil {
			return keyframeindex.Index{}, extractErr
		}
		app.setKeyframeIndex(p.Source, p.StreamIndex, p.Fingerprint, idx)
		return idx, nil
	})

	var result singleflight.Result
	select {
	case result = <-results:
	case <-ctx.Done():
		return keyframeindex.Index{}, ctx.Err()
	}
	if result.Err != nil {
		return keyframeindex.Index{}, result.Err
	}
	idx, ok := result.Val.(keyframeindex.Index)
	if !ok {
		return keyframeindex.Index{}, fmt.Errorf("singleflight returned unexpected keyframe index type %T for %q", result.Val, key)
	}
	return idx, nil
}

func (app *Application) extractKeyframeIndexFromFile(
	ctx context.Context,
	filePath string,
	container string,
) (keyframeindex.Index, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return keyframeindex.Index{}, err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return keyframeindex.Index{}, err
	}

	return keyframeindex.Extract(ctx, file, stat.Size(), container)
}
