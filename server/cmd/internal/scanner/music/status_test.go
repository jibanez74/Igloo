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
	spotifyapi "igloo/cmd/internal/spotify"

	spotifylib "github.com/zmb3/spotify/v2"
)

func musicStatusFixture(t *testing.T) (*Scanner, string, *failingPathMusicScannerFfprobe) {
	t.Helper()
	s := setupMusicScanner(t)
	dir := t.TempDir()
	for _, name := range []string{"a.m4a", "b.mp3", "c.flac"} {
		writeMusicScannerTestFile(t, filepath.Join(dir, name), name)
	}
	probe := &failingPathMusicScannerFfprobe{result: testMusicMetadata(), failingPath: filepath.Join(dir, "c.flac")}
	s.ffprobe = probe
	s.currentMusicDirectory = func() sql.NullString { return sql.NullString{String: dir, Valid: true} }
	return s, dir, probe
}

func TestMusicScanStatusReportsLocalOutcomes(t *testing.T) {
	s, dir, probe := musicStatusFixture(t)
	defer s.tx.DB.Close()

	if idle := s.Status(); idle.State != scanner.StateIdle || idle.Phase != scanner.PhaseIdle || idle.ActiveFiles == nil || idle.Issues == nil {
		t.Fatalf("never-run status is not idle: %+v", idle)
	}

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
	if again := s.Status(); again.Issues[0].Reason == "mutated" || len(again.ActiveFiles) != 0 {
		t.Fatalf("Status shares memory with its caller: %+v", again)
	}

	s.scan(dir)
	second := s.Status()
	if second.RunID == first.RunID || second.Unchanged != 2 || second.Failed != 1 || second.Imported != 0 || second.Updated != 0 {
		t.Fatalf("unchanged rescan: %+v", second)
	}

	writeMusicScannerTestFile(t, filepath.Join(dir, "a.m4a"), "a.m4a changed")
	err := os.Remove(filepath.Join(dir, "b.mp3"))
	if err != nil {
		t.Fatal(err)
	}
	probe.failingPath = ""
	s.scan(dir)
	third := s.Status()
	if third.State != scanner.StateCompleted || third.Total != 2 || third.Processed != 2 || third.Updated != 1 || third.Imported != 1 || third.Unchanged != 0 || third.Failed != 0 || third.Deleted != 1 || third.IssueCount != 0 {
		t.Fatalf("changed rescan: %+v", third)
	}
}

func TestMusicScanStatusDeferredFilesStayIssues(t *testing.T) {
	s, dir, probe := musicStatusFixture(t)
	defer s.tx.DB.Close()
	s.now = time.Now

	s.scan(dir)
	status := s.Status()
	if status.State != scanner.StateCompletedWithIssues || status.Deferred != 3 || status.Processed != 3 || status.IssueCount != 3 || probe.calls != 0 {
		t.Fatalf("deferred scan: %+v probes=%d", status, probe.calls)
	}
	if status.Issues[0].Reason != scanner.ReasonFileDeferredNextScan {
		t.Fatalf("deferred issue reason = %q", status.Issues[0].Reason)
	}
}

func TestMusicScanStatusFailsWhenDirectoryIsUnavailable(t *testing.T) {
	s := setupMusicScanner(t)
	defer s.tx.DB.Close()
	s.ffprobe = &countingMusicScannerFfprobe{result: testMusicMetadata()}

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
	defer s.tx.DB.Close()

	var observed Status
	s.ffprobe = &callbackMusicProbe{audio: func(context.Context, string) (*ffprobe.FfprobeResult, error) {
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
	s.ffprobe = &callbackMusicProbe{audio: func(ctx context.Context, _ string) (*ffprobe.FfprobeResult, error) {
		cancel()
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	writeMusicScannerTestFile(t, filepath.Join(dir, "a.m4a"), "a.m4a changed")
	result := s.Start()
	if result.Status != scanner.StartStarted {
		t.Fatal(result)
	}
	if started := s.Status(); started.State != scanner.StateRunning || started.RunID == observed.RunID {
		t.Fatalf("Start did not publish a new running report: %+v", started)
	}
	waitForMusicScan(t, s)
	status := s.Status()
	if status.State != scanner.StateCanceled || status.FinishedAt == nil || len(status.ActiveFiles) != 0 || status.Failed != 0 {
		t.Fatalf("canceled scan: %+v", status)
	}
}

func TestMusicScanStatusSpotifyTallies(t *testing.T) {
	s, dir, probe := musicStatusFixture(t)
	defer s.tx.DB.Close()
	probe.failingPath = ""
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

	s.spotify = &musicScannerSpotifyStub{
		artist: &spotifylib.FullArtist{SimpleArtist: spotifylib.SimpleArtist{ID: "artist"}},
		album:  &spotifylib.FullAlbum{SimpleAlbum: spotifylib.SimpleAlbum{ID: "album"}},
	}
	s.scan(dir)
	second := s.Status()
	if second.Unchanged != 3 || second.Enriched != 1 || second.EnrichmentFailed != 0 || second.EnrichmentUnmatched != 0 || second.EnrichmentTotal != 1 || second.EnrichmentProcessed != 1 {
		t.Fatalf("retry scan tallies: %+v", second)
	}

	s.spotify = &musicScannerSpotifyStub{artistErr: errors.New("temporary"), albumErr: errors.New("temporary")}
	writeMusicScannerTestFile(t, filepath.Join(dir, "a.m4a"), "a.m4a changed")
	s.scan(dir)
	third := s.Status()
	if third.Updated != 1 || third.State != scanner.StateCompleted || third.EnrichmentTotal != 0 {
		t.Fatalf("matched entities are not retried: %+v", third)
	}
}
