package music

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/itunes"
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

func TestMusicRevisionCountsCommittedPairingRetries(t *testing.T) {
	for _, provider := range []string{musicProviderApple, musicProviderSpotify} {
		for _, fingerprintOnly := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/fingerprintOnly=%v", provider, fingerprintOnly), func(t *testing.T) {
				dir := t.TempDir()
				s := setupMusicScanner(t)
				probe := newMusicScannerFfprobeByPath(map[string]*ffprobe.FfprobeResult{})
				s.ffprobe = probe
				paths := []string{filepath.Join(dir, "first.m4a"), filepath.Join(dir, "second.m4a")}
				for i, path := range paths {
					tags := albumTags(fmt.Sprintf("Song %d", i+1), strconv.Itoa(i+1))
					writePurchase(t, probe, path, itunes.CatalogIDs{Track: int64(i + 1), Album: 11}, tags)
				}
				album := testAlbum(spotifyTrack("t1", "Song 1", 1, 1, 180250, true), spotifyTrack("t2", "Song 2", 1, 2, 180250, true))
				s.spotify = &musicScannerSpotifyStub{artist: testArtist(), album: album, albumsByID: map[spotifyapi.ID]*spotifyapi.FullAlbum{"album": album}}
				if provider == musicProviderApple {
					s.apple = &appleStub{albums: map[int64]itunes.Album{11: cryPretty(map[int64]bool{1: true, 2: true})}}
				}
				s.scan(dir)
				initial := s.Status()
				if initial.Imported != 2 || initial.CatalogRevision != int64(4+initial.Enriched) {
					t.Fatalf("initial imports and pairings: %+v", initial)
				}
				// Keep matched artists and albums, with pending track pairings as
				// left by an interrupted or failed pairing pass.
				_, err := s.tx.DB.Exec(`DELETE FROM music_provider_matches WHERE entity_type = 'track';
					UPDATE tracks SET apple_id = NULL, spotify_id = NULL, explicit = 0;
					CREATE TRIGGER reject_pairing BEFORE INSERT ON music_provider_matches
					WHEN NEW.entity_type = 'track' AND NEW.entity_id = (SELECT max(id) FROM tracks)
					BEGIN SELECT RAISE(ABORT, 'pairing failure'); END;`)
				if err != nil {
					t.Fatal(err)
				}
				s.scan(dir)
				rolledBack := s.Status()
				if rolledBack.State != scanner.StateFailed || rolledBack.CatalogRevision != initial.CatalogRevision || rolledBack.Enriched != 0 {
					t.Fatalf("rolled-back pairings changed revision: %+v", rolledBack)
				}
				linked := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM tracks WHERE apple_id IS NOT NULL OR spotify_id IS NOT NULL OR explicit != 0")
				if linked != 0 {
					t.Fatalf("rollback left %d linked tracks", linked)
				}
				_, err = s.tx.DB.Exec("DROP TRIGGER reject_pairing")
				if err != nil {
					t.Fatal(err)
				}
				baseline, _, err := s.loadMusicScanIndex(context.Background(), nil, nil)
				if err != nil {
					t.Fatal(err)
				}
				if fingerprintOnly {
					for _, path := range paths {
						err = os.Chmod(path, 0600)
						if err != nil {
							t.Fatal(err)
						}
					}
				}
				s.scan(dir)
				retried := s.Status()
				if retried.State != scanner.StateCompleted || retried.Imported != 0 || retried.Updated != 0 || retried.Deleted != 0 || retried.Enriched != 0 || retried.Unchanged != 2 || retried.CatalogRevision != initial.CatalogRevision+2 {
					t.Fatalf("committed pairing-only retry: %+v", retried)
				}
				current, _, err := s.loadMusicScanIndex(context.Background(), nil, nil)
				if err != nil {
					t.Fatal(err)
				}
				for i, path := range paths {
					fingerprintChanged := current[path] != baseline[path]
					if fingerprintChanged != fingerprintOnly {
						t.Fatalf("unexpected fingerprint change for %s", path)
					}
					if provider == musicProviderApple {
						assertApplePaired(t, appleTrackFor(t, s, path), int64(i+1), true)
					} else {
						assertLinked(t, trackLinkFor(t, s, path), fmt.Sprintf("t%d", i+1), true, int64(i+1))
					}
				}
				// Publishing the committed cumulative tally again adds nothing.
				report := newScanReport(retried)
				report.scan = newMusicScanContext(nil)
				report.scan.tracksLinked = 2
				s.publish(report)
				s.publish(report)
				republished := s.Status()
				if republished.CatalogRevision != retried.CatalogRevision {
					t.Fatal("repeated publication counted pairings twice")
				}
				s.scan(dir)
				settled := s.Status()
				probeCalls := probe.totalAudioCalls()
				if settled.CatalogRevision != retried.CatalogRevision || probeCalls != 2 {
					t.Fatal("settled rescan changed revision or re-probed unchanged bytes")
				}
			})
		}
	}
}
