package main

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/keyframeindex"
	"igloo/cmd/internal/keyframeindex/kftestutil"
)

func TestKeyframeIndexFingerprint(t *testing.T) {
	movie := database.Movie{ID: 7, Size: 1_000_000, UpdatedAt: "2026-08-07"}
	video := database.VideoStream{StreamIndex: 2}

	got := keyframeIndexFingerprint(&movie, &video)
	want := "7:2:1000000:2026-08-07"
	if got != want {
		t.Fatalf("fingerprint = %q, want %q", got, want)
	}
}

func TestKeyframeIndexStore(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	ctx := context.Background()
	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	const streamIndex = int64(0)
	const fingerprint = "fingerprint-a"

	stored := keyframeindex.Index{KeyframeSec: []float64{0, 4.2, 9.96}, DurationSec: 14}

	t.Run("round trips an index", func(t *testing.T) {
		app.setKeyframeIndex(movieID, streamIndex, fingerprint, stored)

		idx, ok := app.getKeyframeIndex(ctx, movieID, streamIndex, fingerprint)
		if !ok {
			t.Fatal("persisted index was not returned")
		}
		if len(idx.KeyframeSec) != 3 || idx.KeyframeSec[1] != 4.2 {
			t.Fatalf("KeyframeSec = %v, want the stored keyframes", idx.KeyframeSec)
		}
		if idx.DurationSec != 14 {
			t.Fatalf("DurationSec = %v, want 14", idx.DurationSec)
		}
	})

	t.Run("treats a changed fingerprint as a miss", func(t *testing.T) {
		_, ok := app.getKeyframeIndex(ctx, movieID, streamIndex, "fingerprint-b")
		if ok {
			t.Fatal("a stale fingerprint was reported as a hit")
		}
	})

	t.Run("reports an absent row as a miss", func(t *testing.T) {
		_, ok := app.getKeyframeIndex(ctx, movieID, streamIndex+1, fingerprint)
		if ok {
			t.Fatal("an absent index was reported as a hit")
		}
	})

	t.Run("treats a corrupt payload as a miss", func(t *testing.T) {
		_, err := app.DB.Exec(`
			UPDATE keyframe_indexes SET keyframes = 'not json' WHERE movie_id = ? AND stream_index = ?
		`, movieID, streamIndex)
		if err != nil {
			t.Fatalf("corrupt payload: %v", err)
		}

		_, ok := app.getKeyframeIndex(ctx, movieID, streamIndex, fingerprint)
		if ok {
			t.Fatal("a corrupt payload was reported as a hit")
		}
	})

	t.Run("upserts over the previous row", func(t *testing.T) {
		app.setKeyframeIndex(movieID, streamIndex, "fingerprint-b", stored)

		idx, ok := app.getKeyframeIndex(ctx, movieID, streamIndex, "fingerprint-b")
		if !ok {
			t.Fatal("upserted index was not returned")
		}
		if len(idx.KeyframeSec) != 3 {
			t.Fatalf("KeyframeSec = %v, want the stored keyframes", idx.KeyframeSec)
		}

		var count int
		err := app.DB.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM keyframe_indexes WHERE movie_id = ? AND stream_index = ?
		`, movieID, streamIndex).Scan(&count)
		if err != nil {
			t.Fatalf("count index rows: %v", err)
		}
		if count != 1 {
			t.Fatalf("index row count = %d, want the upsert to keep a single row", count)
		}
	})
}

func TestKeyframeAtOrBefore(t *testing.T) {
	keyframes := []float64{0, 4, 8.5, 12}

	tests := []struct {
		name    string
		target  float64
		want    float64
		wantHit bool
	}{
		{name: "exact hit", target: 8.5, want: 8.5, wantHit: true},
		{name: "between keyframes", target: 10, want: 8.5, wantHit: true},
		{name: "after last", target: 100, want: 12, wantHit: true},
		{name: "at first", target: 0, want: 0, wantHit: true},
		{name: "before first", target: -1, wantHit: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, hit := keyframeAtOrBefore(keyframes, tt.target)
			if hit != tt.wantHit {
				t.Fatalf("hit = %v, want %v", hit, tt.wantHit)
			}
			if hit && got != tt.want {
				t.Fatalf("keyframe = %v, want %v", got, tt.want)
			}
		})
	}
}

// writeTestMKVFixture puts a real cued Matroska file on disk and registers a
// movie row pointing at it.
func writeTestMKVFixture(t *testing.T, app *Application, cueTimesSec []float64) int64 {
	t.Helper()

	path := filepath.Join(t.TempDir(), "fixture.mkv")
	data := kftestutil.BuildMKV(kftestutil.MKVOptions{
		CueTimesSec: cueTimesSec,
		DurationSec: 7200,
	})
	err := os.WriteFile(path, data, 0o600)
	if err != nil {
		t.Fatalf("write mkv fixture: %v", err)
	}

	return insertTestHLSMovieFixtureAt(t, app, "h264", 1080, path, "mkv")
}

// A persisted index must answer a seek synchronously with no ffprobe spawn —
// the header lands on the first manifest response.
func TestStartHLSSession_KeyframeIndexHitNeedsNoProbe(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.FFmpeg = &fakeFFmpeg{plans: []fakeFFmpegRunPlan{hlsRunPlan(safeRemuxFixture)}}
	app.Wait = &sync.WaitGroup{}
	prober := &stubKeyframeFfprobe{err: os.ErrInvalid}
	app.Ffprobe = prober

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)

	var movie database.Movie
	err := app.DB.QueryRow(`SELECT id, size, updated_at FROM movies WHERE id = ?`, movieID).
		Scan(&movie.ID, &movie.Size, &movie.UpdatedAt)
	if err != nil {
		t.Fatalf("read movie row: %v", err)
	}
	fingerprint := keyframeIndexFingerprint(&movie, &database.VideoStream{StreamIndex: 0})
	app.setKeyframeIndex(movieID, 0, fingerprint, keyframeindex.Index{
		KeyframeSec: []float64{0, 47.5, 95, 142.5},
		DurationSec: 7200,
	})

	session, err := createTestHLSSession(app, context.Background(), movieID, helpers.HLS_PROFILE_REMUX, testIntPtr(0), testPlaybackSessionID, 100, false)
	if err != nil {
		t.Fatalf("createHLSSession returned error: %v", err)
	}
	defer cleanupHLSSession(session)

	if got := session.actualStartSec(); got != 95 {
		t.Fatalf("actual start = %v, want 95 from the index without waiting", got)
	}

	app.Wait.Wait()
	prober.mu.Lock()
	calls := prober.calls
	prober.mu.Unlock()
	if calls != 0 {
		t.Fatalf("keyframe probe calls = %d, want 0 on an index hit", calls)
	}
}

// An index miss extracts from the real container in the background, persists
// the result, and answers the seek — still without ffprobe.
func TestStartHLSSession_KeyframeIndexMissExtractsFromContainer(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.FFmpeg = &fakeFFmpeg{plans: []fakeFFmpegRunPlan{hlsRunPlan(safeRemuxFixture)}}
	app.Wait = &sync.WaitGroup{}
	prober := &stubKeyframeFfprobe{err: os.ErrInvalid}
	app.Ffprobe = prober

	movieID := writeTestMKVFixture(t, app, []float64{0, 42, 84, 126})

	session, err := createTestHLSSession(app, context.Background(), movieID, helpers.HLS_PROFILE_REMUX, testIntPtr(0), testPlaybackSessionID, 100, false)
	if err != nil {
		t.Fatalf("createHLSSession returned error: %v", err)
	}
	defer cleanupHLSSession(session)

	app.Wait.Wait()

	if got := session.actualStartSec(); got != 84 {
		t.Fatalf("actual start = %v, want 84 from the extracted cues", got)
	}
	prober.mu.Lock()
	calls := prober.calls
	prober.mu.Unlock()
	if calls != 0 {
		t.Fatalf("keyframe probe calls = %d, want 0 when the container has an index", calls)
	}

	row, err := app.Queries.GetKeyframeIndex(context.Background(), database.GetKeyframeIndexParams{
		MovieID:     movieID,
		StreamIndex: 0,
	})
	if err != nil {
		t.Fatalf("GetKeyframeIndex after extraction: %v", err)
	}
	var movie database.Movie
	err = app.DB.QueryRow(`SELECT id, size, updated_at FROM movies WHERE id = ?`, movieID).
		Scan(&movie.ID, &movie.Size, &movie.UpdatedAt)
	if err != nil {
		t.Fatalf("read movie row: %v", err)
	}
	if want := keyframeIndexFingerprint(&movie, &database.VideoStream{StreamIndex: 0}); row.Fingerprint != want {
		t.Fatalf("persisted fingerprint = %q, want %q", row.Fingerprint, want)
	}
}

// An avi source has no supported container index: the bounded probe answers
// the seek and nothing is persisted.
func TestStartHLSSession_AviFallsBackToProbeWithoutPersisting(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.FFmpeg = &fakeFFmpeg{plans: []fakeFFmpegRunPlan{hlsRunPlan(safeRemuxFixture)}}
	app.Wait = &sync.WaitGroup{}
	prober := &stubKeyframeFfprobe{keyframeSec: 96}
	app.Ffprobe = prober

	path := filepath.Join(t.TempDir(), "fixture.avi")
	err := os.WriteFile(path, []byte("RIFFxxxxAVI LIST"), 0o600)
	if err != nil {
		t.Fatalf("write avi fixture: %v", err)
	}
	movieID := insertTestHLSMovieFixtureAt(t, app, "h264", 1080, path, "avi")

	session, err := createTestHLSSession(app, context.Background(), movieID, helpers.HLS_PROFILE_REMUX, testIntPtr(0), testPlaybackSessionID, 100, false)
	if err != nil {
		t.Fatalf("createHLSSession returned error: %v", err)
	}
	defer cleanupHLSSession(session)

	app.Wait.Wait()

	if got := session.actualStartSec(); got != 96 {
		t.Fatalf("actual start = %v, want the probed 96", got)
	}
	prober.mu.Lock()
	calls := prober.calls
	prober.mu.Unlock()
	if calls != 1 {
		t.Fatalf("keyframe probe calls = %d, want 1 for an unsupported container", calls)
	}

	_, err = app.Queries.GetKeyframeIndex(context.Background(), database.GetKeyframeIndexParams{
		MovieID:     movieID,
		StreamIndex: 0,
	})
	if err != sql.ErrNoRows {
		t.Fatalf("GetKeyframeIndex error = %v, want sql.ErrNoRows for an unsupported container", err)
	}
}

// A session starting at 0 needs no measurement but still prefetches the index
// so the first real seek answers synchronously.
func TestStartHLSSession_PrefetchesIndexAtStartZero(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.FFmpeg = &fakeFFmpeg{plans: []fakeFFmpegRunPlan{hlsRunPlan(safeRemuxFixture)}}
	app.Wait = &sync.WaitGroup{}
	prober := &stubKeyframeFfprobe{err: os.ErrInvalid}
	app.Ffprobe = prober

	movieID := writeTestMKVFixture(t, app, []float64{0, 42, 84})

	session, err := createTestHLSSession(app, context.Background(), movieID, helpers.HLS_PROFILE_REMUX, testIntPtr(0), testPlaybackSessionID, 0, false)
	if err != nil {
		t.Fatalf("createHLSSession returned error: %v", err)
	}
	defer cleanupHLSSession(session)

	app.Wait.Wait()

	if got := session.actualStartSec(); got != 0 {
		t.Fatalf("actual start = %v, want 0 for a session from the beginning", got)
	}

	_, err = app.Queries.GetKeyframeIndex(context.Background(), database.GetKeyframeIndexParams{
		MovieID:     movieID,
		StreamIndex: 0,
	})
	if err != nil {
		t.Fatalf("GetKeyframeIndex after prefetch: %v", err)
	}
	prober.mu.Lock()
	calls := prober.calls
	prober.mu.Unlock()
	if calls != 0 {
		t.Fatalf("keyframe probe calls = %d, want 0 during prefetch", calls)
	}
}
