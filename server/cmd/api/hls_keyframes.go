package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"sort"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/keyframeindex"
)

// keyframeIndexFingerprint keys the persisted keyframe index. The index
// depends only on the file's bytes, so unlike remuxSafetyFingerprint no
// stream properties are included — file identity alone invalidates it.
func keyframeIndexFingerprint(movie *database.Movie, video *database.VideoStream) string {
	return movieStreamFingerprintBase(movie, video.StreamIndex)
}

// getKeyframeIndex reads the persisted index for one video stream. Any miss —
// no row, stale fingerprint, or an unreadable payload — sends the caller to a
// fresh extraction, which rewrites the row.
func (app *Application) getKeyframeIndex(
	ctx context.Context,
	movieID int64,
	streamIndex int64,
	fingerprint string,
) (keyframeindex.Index, bool) {
	row, err := app.Queries.GetKeyframeIndex(ctx, database.GetKeyframeIndexParams{
		MovieID:     movieID,
		StreamIndex: streamIndex,
	})
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			app.Logger.Warn("failed to read keyframe index",
				"movie_id", movieID,
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
			"movie_id", movieID,
			"stream_index", streamIndex,
			"error", err,
		)
		return keyframeindex.Index{}, false
	}

	return idx, true
}

// setKeyframeIndex persists an extracted index. It runs on
// context.Background() because the index just cost a read of the source file
// and must not be lost to a client disconnect (FFmpeg itself already runs on
// a background context for the same reason). A write failure only means the
// index is re-extracted on the next play.
func (app *Application) setKeyframeIndex(
	movieID int64,
	streamIndex int64,
	fingerprint string,
	idx keyframeindex.Index,
) {
	payload, err := json.Marshal(idx.KeyframeSec)
	if err != nil {
		app.Logger.Warn("failed to encode keyframe index",
			"movie_id", movieID,
			"stream_index", streamIndex,
			"error", err,
		)
		return
	}

	err = app.Queries.UpsertKeyframeIndex(context.Background(), database.UpsertKeyframeIndexParams{
		MovieID:     movieID,
		StreamIndex: streamIndex,
		Fingerprint: fingerprint,
		DurationSec: idx.DurationSec,
		Keyframes:   string(payload),
	})
	if err != nil {
		app.Logger.Warn("failed to persist keyframe index",
			"movie_id", movieID,
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
	FilePath    string
	Container   string
	MovieID     int64
	StreamIndex int64
	Fingerprint string
	// RequestedStartSec of 0 means prefetch only: populate the index for
	// later seeks, measure nothing (a start of 0 is already exact).
	RequestedStartSec float64
}

// resolveHLSActualStart records where a copy-video session's media actually
// begins. Stream copy cannot discard frames, so FFmpeg's input seek lands on
// the source keyframe at or before the requested offset and the session
// starts early by up to one GOP. Without this the client maps session time to
// absolute movie time using the requested offset, so the clock and every
// watch-progress write run ahead of the picture.
//
// The primary source is the container's own seek index, extracted once and
// persisted; FFmpeg's -ss consults the same structures, so the answer matches
// where it actually lands. Files without a usable index (avi, missing Cues)
// fall back to the bounded ffprobe packet probe. It runs alongside FFmpeg
// rather than before it so it adds no startup latency, and it is advisory: on
// failure the start stays unknown and the session reports nothing, leaving
// the client on its previous fallback.
//
// parentCtx is deliberately not the session's context — see the call site in
// startHLSSession. The index outlives the session that extracted it, so the
// two stages here are bounded by their own timeouts instead.
func (app *Application) resolveHLSActualStart(parentCtx context.Context, p hlsActualStartParams) {
	defer app.Wait.Done()

	extractCtx, cancelExtract := context.WithTimeout(parentCtx, hlsStartProbeTimeout)
	defer cancelExtract()

	idx, err := app.extractKeyframeIndexFromFile(extractCtx, p.FilePath, p.Container)
	if err == nil {
		app.setKeyframeIndex(p.MovieID, p.StreamIndex, p.Fingerprint, idx)
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
			"movie_id", p.MovieID,
			"container", p.Container,
		)
	} else {
		app.Logger.Warn("keyframe index extraction failed",
			"movie_id", p.MovieID,
			"container", p.Container,
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

	actualStartSec, probeErr := app.Ffprobe.KeyframeAtOrBefore(probeCtx, p.FilePath, p.StreamIndex, p.RequestedStartSec)
	if probeErr != nil {
		app.Logger.Warn("hls actual start probe failed",
			"movie_id", p.MovieID,
			"requested_start_sec", p.RequestedStartSec,
			"error", probeErr.Error(),
		)
		return
	}

	p.Session.setActualStartSec(actualStartSec)
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
