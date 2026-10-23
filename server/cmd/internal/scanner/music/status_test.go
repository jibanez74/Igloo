package music

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/scanner"
	"igloo/cmd/internal/scanner/scannertest"
	spotifyapi "igloo/cmd/internal/spotify"
)

// musicStatusFixture probes three files, failing c.flac until the test clears
// the probe's Hook.
func musicStatusFixture(t *testing.T) (*Scanner, string, *scannertest.CountingProbe) {
	t.Helper()
	s := setupMusicScanner(t)
	dir := t.TempDir()
	for _, name := range []string{"a.m4a", "b.mp3", "c.flac"} {
		scannertest.WriteFile(t, filepath.Join(dir, name), name)
	}
	failingPath := filepath.Join(dir, "c.flac")
	probe := &scannertest.CountingProbe{Default: testMusicMetadata()}
	probe.Hook = func(_ context.Context, path string) (*ffprobe.FfprobeResult, error) {
		if path == failingPath {
			return nil, errors.New("ffprobe failed")
		}
		result := *probe.Default
		return &result, nil
	}
	s.ffprobe = probe
	s.currentMusicDirectory = func() sql.NullString { return sql.NullString{String: dir, Valid: true} }
	return s, dir, probe
}

func TestMusicScanStatusReportsLocalOutcomes(t *testing.T) {
	s, dir, probe := musicStatusFixture(t)

	s.scan(dir)
	first := s.Status()
	if first.State != scanner.StateCompletedWithIssues || first.Phase != scanner.PhaseEnrichment || first.RunID == "" || first.StartedAt == nil || first.FinishedAt == nil {
		t.Fatalf("first scan: %+v", first)
	}
	if first.Total != 3 || first.Processed != 3 || first.Imported != 2 || first.Updated != 0 || first.Unchanged != 0 || first.Failed != 1 || first.Deferred != 0 || first.Deleted != 0 {
		t.Fatalf("first scan counts: %+v", first)
	}
	if first.IssueCount != 1 || len(first.Issues) != 1 || first.Issues[0].Filename != "c.flac" || first.Issues[0].Phase != scanner.PhaseLocal || strings.Contains(first.Issues[0].Reason, dir) {
		t.Fatalf("first scan issues: %+v", first.Issues)
	}
	if len(first.ActiveFiles) != 0 || first.EnrichmentTotal != 0 || first.EnrichmentProcessed != 0 {
		t.Fatalf("finished scan left activity: %+v", first)
	}
	first.Issues[0].Reason = "mutated"
	first.ActiveFiles = append(first.ActiveFiles, "mutated")
	again := s.Status()
	if again.Issues[0].Reason == "mutated" || len(again.ActiveFiles) != 0 {
		t.Fatalf("Status shares memory with its caller: %+v", again)
	}

	s.scan(dir)
	second := s.Status()
	if second.RunID == first.RunID || second.Unchanged != 2 || second.Failed != 1 || second.Imported != 0 || second.Updated != 0 {
		t.Fatalf("unchanged rescan: %+v", second)
	}

	scannertest.WriteFile(t, filepath.Join(dir, "a.m4a"), "a.m4a changed")
	err := os.Remove(filepath.Join(dir, "b.mp3"))
	if err != nil {
		t.Fatal(err)
	}
	probe.Hook = nil
	s.scan(dir)
	third := s.Status()
	if third.State != scanner.StateCompleted || third.Total != 2 || third.Processed != 2 || third.Updated != 1 || third.Imported != 1 || third.Unchanged != 0 || third.Failed != 0 || third.Deleted != 1 || third.IssueCount != 0 {
		t.Fatalf("changed rescan: %+v", third)
	}
}

func TestMusicScanStatusDeferredFilesStayIssues(t *testing.T) {
	s, dir, probe := musicStatusFixture(t)
	s.now = time.Now

	s.scan(dir)
	status := s.Status()
	if status.State != scanner.StateCompletedWithIssues || status.Deferred != 3 || status.Processed != 3 || status.IssueCount != 3 || probe.Calls() != 0 {
		t.Fatalf("deferred scan: %+v probes=%d", status, probe.Calls())
	}
	if status.Issues[0].Reason != scanner.ReasonFileDeferredNextScan {
		t.Fatalf("deferred issue reason = %q", status.Issues[0].Reason)
	}
}

func TestMusicScanStatusFailsWhenDirectoryIsUnavailable(t *testing.T) {
	s := setupMusicScanner(t)
	s.ffprobe = &scannertest.CountingProbe{Default: testMusicMetadata()}

	s.scan(filepath.Join(t.TempDir(), "missing"))
	status := s.Status()
	if status.State != scanner.StateFailed || status.FinishedAt == nil || status.Total != 0 {
		t.Fatalf("missing directory: %+v", status)
	}
	if status.IssueCount != 1 || status.Issues[0].Filename != "" || status.Issues[0].Reason == "" {
		t.Fatalf("missing directory issue: %+v", status.Issues)
	}
}

func TestMusicScanStatusObservesActiveFileAndCancellation(t *testing.T) {
	s, dir, _ := musicStatusFixture(t)

	var observed Status
	s.ffprobe = &scannertest.Probe{Callback: func(context.Context, string) (*ffprobe.FfprobeResult, error) {
		if observed.RunID == "" {
			observed = s.Status()
		}
		return testMusicMetadata(), nil
	}}
	s.scan(dir)
	if observed.State != scanner.StateRunning || observed.Phase != scanner.PhaseLocal || len(observed.ActiveFiles) != 1 || observed.ActiveFiles[0] != "a.m4a" || strings.Contains(observed.ActiveFiles[0], dir) {
		t.Fatalf("mid-probe status: %+v", observed)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.scanContext = ctx
	// The probe holds the scan until the test cancels it, so the running
	// report is read before the background run can finish.
	probe := scannertest.NewGateProbe(nil)
	s.ffprobe = probe
	scannertest.WriteFile(t, filepath.Join(dir, "a.m4a"), "a.m4a changed")
	result := s.Start()
	if result.Status != scanner.StartStarted {
		t.Fatal(result)
	}
	probe.WaitEntered(t, 5*time.Second, "music scan reaching probing")
	started := s.Status()
	if started.State != scanner.StateRunning || started.RunID == observed.RunID {
		t.Fatalf("Start did not publish a new running report: %+v", started)
	}
	cancel()
	scannertest.WaitForGroup(t, s.launcher.Wait, 5*time.Second, "music scan")
	status := s.Status()
	if status.State != scanner.StateCanceled || status.FinishedAt == nil || len(status.ActiveFiles) != 0 || status.Failed != 0 {
		t.Fatalf("canceled scan: %+v", status)
	}
}

func TestMusicScanStatusSpotifyTallies(t *testing.T) {
	s, dir, probe := musicStatusFixture(t)
	probe.Hook = nil
	s.spotify = &musicScannerSpotifyStub{
		artistErr: &spotifyapi.MatchError{Info: spotifyapi.MatchDebugInfo{Reason: spotifyapi.MatchReasonNoResults}},
		albumErr:  errors.New("temporary"),
	}

	s.scan(dir)
	first := s.Status()
	// The unmatched artist is final; only the failed album is a retry candidate.
	if first.Enriched != 0 || first.EnrichmentUnmatched != 1 || first.EnrichmentFailed != 1 || first.EnrichmentTotal != 1 || first.EnrichmentProcessed != 1 {
		t.Fatalf("first scan tallies: %+v", first)
	}

	// The fixture's files are one track three times over, so all three pair
	// with the album's one Spotify track.
	tracks := []spotifyapi.SimpleTrack{spotifyTrack("t1", "Test Track", 1, 1, 180000, false)}
	s.spotify = &musicScannerSpotifyStub{
		artist: &spotifyapi.FullArtist{SimpleArtist: spotifyapi.SimpleArtist{ID: "artist"}},
		album:  &spotifyapi.FullAlbum{SimpleAlbum: spotifyapi.SimpleAlbum{ID: "album"}, Tracks: tracks},
	}
	s.scan(dir)
	second := s.Status()
	// The retried album counts once, and once more when its tracks are paired
	// with the track list; the track outcomes themselves are not tallied.
	if second.Unchanged != 3 || second.Enriched != 1 || second.EnrichmentFailed != 0 || second.EnrichmentUnmatched != 0 || second.EnrichmentTotal != 2 || second.EnrichmentProcessed != 2 {
		t.Fatalf("retry scan tallies: %+v", second)
	}

	stub := &musicScannerSpotifyStub{
		artistErr:  errors.New("temporary"),
		albumErr:   errors.New("temporary"),
		albumsByID: map[spotifyapi.ID]*spotifyapi.FullAlbum{"album": {SimpleAlbum: spotifyapi.SimpleAlbum{ID: "album"}, Tracks: tracks}},
	}
	s.spotify = stub
	scannertest.WriteFile(t, filepath.Join(dir, "a.m4a"), "a.m4a changed")
	s.scan(dir)
	third := s.Status()
	// The matched artist and album are not searched again. The changed file
	// dropped its track pairing, so its album's track list is fetched once to
	// pair it afresh; that album is the one enrichment candidate.
	if third.Updated != 1 || third.State != scanner.StateCompleted || third.EnrichmentTotal != 1 || third.EnrichmentProcessed != 1 || third.Enriched != 0 {
		t.Fatalf("matched entities are not retried: %+v", third)
	}
	if stub.artistCalls != 0 || stub.albumCalls != 0 || stub.albumFetches != 1 {
		t.Fatalf("third scan requests: artist=%d album=%d fetches=%d, want only one track-list fetch", stub.artistCalls, stub.albumCalls, stub.albumFetches)
	}
}
