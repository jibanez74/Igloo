package main

import (
	"context"
	"errors"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"

	"igloo/cmd/internal/ffprobe"
)

// stubKeyframeFfprobe answers the keyframe lookup that measures where a
// copy-video session's media really begins.
type stubKeyframeFfprobe struct {
	keyframeSec float64
	err         error

	mu     sync.Mutex
	target float64
	calls  int
}

func (s *stubKeyframeFfprobe) GetMetadata(string) (*ffprobe.FfprobeResult, error) {
	return nil, errors.New("not used")
}

func (s *stubKeyframeFfprobe) GetAudioMetadata(string) (*ffprobe.FfprobeResult, error) {
	return nil, errors.New("not used")
}

func (s *stubKeyframeFfprobe) KeyframeAtOrBefore(
	_ context.Context,
	_ string,
	_ int64,
	targetSec float64,
) (float64, error) {
	s.mu.Lock()
	s.target = targetSec
	s.calls++
	s.mu.Unlock()

	if s.err != nil {
		return 0, s.err
	}
	return s.keyframeSec, nil
}

func TestMeasureHLSSessionStart_RecordsKeyframeBeforeRequestedStart(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	app.Wait = &sync.WaitGroup{}
	prober := &stubKeyframeFfprobe{keyframeSec: 591.174}
	app.Ffprobe = prober

	session := &HLSSession{MovieID: 1, CopyVideo: true, StartSec: 600}
	session.setActualStartSec(hlsUnknownActualStart)

	app.Wait.Add(1)
	app.measureHLSSessionStart(context.Background(), session, "/movies/example.mp4", 0, 600)

	if prober.calls != 1 {
		t.Fatalf("keyframe probe calls = %d, want 1", prober.calls)
	}
	if prober.target != 600 {
		t.Fatalf("probe target = %v, want 600", prober.target)
	}
	if got := session.actualStartSec(); got != 591.174 {
		t.Fatalf("actual start = %v, want 591.174", got)
	}
}

// The probe is advisory: a failure must leave the start unknown rather than
// publish a wrong one, so the client keeps its existing fallback.
func TestMeasureHLSSessionStart_LeavesStartUnknownOnFailure(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	app.Wait = &sync.WaitGroup{}
	app.Ffprobe = &stubKeyframeFfprobe{err: errors.New("probe failed")}

	session := &HLSSession{MovieID: 1, CopyVideo: true, StartSec: 600}
	session.setActualStartSec(hlsUnknownActualStart)

	app.Wait.Add(1)
	app.measureHLSSessionStart(context.Background(), session, "/movies/example.mp4", 0, 600)

	if got := session.actualStartSec(); got >= 0 {
		t.Fatalf("actual start = %v, want it to stay unknown", got)
	}
}

func TestWriteHLSPlaylistHeaders_PublishesEffectiveProfileAndStart(t *testing.T) {
	session := &HLSSession{
		EffectiveProfile: "1080p_8mbps",
		ActualStartSec:   591.174,
	}

	recorder := httptest.NewRecorder()
	writeHLSPlaylistHeaders(recorder, session)

	if got := recorder.Header().Get(hlsEffectiveProfileHeader); got != "1080p_8mbps" {
		t.Fatalf("effective profile header = %q, want 1080p_8mbps", got)
	}

	start, err := strconv.ParseFloat(recorder.Header().Get(hlsActualStartHeader), 64)
	if err != nil {
		t.Fatalf("actual start header did not parse: %v", err)
	}
	if start != 591.174 {
		t.Fatalf("actual start header = %v, want 591.174", start)
	}
}

// A remux request that falls back still serves from the /hls/remux/ path, so
// the header is the only thing that can tell the client what it is getting.
func TestWriteHLSPlaylistHeaders_ReportsFallbackProfile(t *testing.T) {
	session := &HLSSession{EffectiveProfile: "2160p_16mbps", ActualStartSec: 0}

	recorder := httptest.NewRecorder()
	writeHLSPlaylistHeaders(recorder, session)

	if got := recorder.Header().Get(hlsEffectiveProfileHeader); got != "2160p_16mbps" {
		t.Fatalf("effective profile header = %q, want the profile that actually ran", got)
	}
}

func TestWriteHLSPlaylistHeaders_OmitsUnknownStart(t *testing.T) {
	session := &HLSSession{EffectiveProfile: "remux", ActualStartSec: hlsUnknownActualStart}

	recorder := httptest.NewRecorder()
	writeHLSPlaylistHeaders(recorder, session)

	if got := recorder.Header().Get(hlsActualStartHeader); got != "" {
		t.Fatalf("unknown start must not be published, got %q", got)
	}
}
