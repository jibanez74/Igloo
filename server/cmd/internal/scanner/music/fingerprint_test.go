package music

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	sqlite3 "github.com/mattn/go-sqlite3"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/scanner"
	"igloo/cmd/internal/scanner/scannertest"
)

func TestFileFingerprintLifecycle(t *testing.T) {
	s := setupMusicScanner(t)
	defer s.tx.DB.Close()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "media.m4a")
	err := os.WriteFile(path, []byte("media"), 0600)
	if err != nil {
		t.Fatal(err)
	}
	stub := &countingMusicScannerFfprobe{result: testMusicMetadata()}
	s.ffprobe = stub
	invalidations := 0
	s.invalidateCommittedTrack = func(int64) { invalidations++ }
	scan := newMusicScanContext(nil)
	file := scanner.ScanFile{Path: path, Ext: "m4a", Size: 999} // Walking size is deliberately stale.
	s.now = time.Now
	recent := s.processBatchReport(ctx, scan, []scanner.ScanFile{file}).status
	if recent.Imported != 0 || recent.Unchanged != 0 || recent.Failed != 0 || recent.Deferred != 1 || stub.calls != 0 {
		t.Fatalf("recent file: %+v probes=%d", recent, stub.calls)
	}
	s.now = func() time.Time { return time.Now().Add(time.Hour) }
	scanned, _, failures := s.processBatchCounts(ctx, scan, []scanner.ScanFile{file})
	if scanned != 1 || failures != 0 || invalidations != 1 {
		t.Fatalf("initial import: %d/%d invalidations=%d", scanned, failures, invalidations)
	}
	baseline := scan.trackIndex[path]
	if baseline.Size != 5 {
		t.Fatalf("size=%d", baseline.Size)
	}
	var id int64
	err = s.tx.DB.QueryRow("SELECT id FROM tracks WHERE file_path = ?", path).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	// Persist an old timestamp to make accidental catalog writes observable.
	_, err = s.tx.DB.Exec("UPDATE tracks SET updated_at = '2000-01-01 00:00:00' WHERE id = ?", id)
	if err != nil {
		t.Fatal(err)
	}
	before, err := s.queries.GetTrack(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	reloaded, _, err := s.loadMusicScanIndex(ctx)
	if err != nil || !reflect.DeepEqual(reloaded, scan.trackIndex) {
		t.Fatalf("reload: %+v %v", reloaded, err)
	}
	scan = newMusicScanContext(reloaded)
	scanned, skipped, failures := s.processBatchCounts(ctx, scan, []scanner.ScanFile{file})
	if scanned != 0 || skipped != 1 || failures != 0 || stub.calls != 1 {
		t.Fatalf("unchanged reload: %d/%d/%d probes=%d", scanned, skipped, failures, stub.calls)
	}

	err = os.Chmod(path, 0640)
	if err != nil {
		t.Fatal(err)
	}
	scanned, skipped, failures = s.processBatchCounts(ctx, scan, []scanner.ScanFile{file})
	if scanned != 0 || skipped != 1 || failures != 0 || stub.calls != 1 || invalidations != 1 {
		t.Fatalf("identical bytes: %d/%d/%d probes=%d invalidations=%d", scanned, skipped, failures, stub.calls, invalidations)
	}
	after, err := s.queries.GetTrack(ctx, id)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("fingerprint-only update changed catalog: %+v %+v %v", before, after, err)
	}
	if scan.trackIndex[path] == baseline {
		t.Fatal("fingerprint did not advance")
	}

	err = os.WriteFile(path, []byte("edited"[:5]), 0600)
	if err != nil {
		t.Fatal(err)
	}
	stamp := time.Unix(0, baseline.MtimeNS)
	err = os.Chtimes(path, stamp, stamp)
	if err != nil {
		t.Fatal(err)
	}
	scanned, _, failures = s.processBatchCounts(ctx, scan, []scanner.ScanFile{file})
	if scanned != 1 || failures != 0 || stub.calls != 2 || invalidations != 2 {
		t.Fatalf("same size edit: %d/%d probes=%d invalidations=%d", scanned, failures, stub.calls, invalidations)
	}
	var currentID int64
	err = s.tx.DB.QueryRow("SELECT id FROM tracks WHERE file_path = ?", path).Scan(&currentID)
	if err != nil || currentID != id {
		t.Fatalf("catalog ID changed: %d -> %d, %v", id, currentID, err)
	}

	// A missing fingerprint always establishes a new full baseline, even when
	// the catalog already contains the same path and size.
	_, err = s.tx.DB.Exec("DELETE FROM track_file_fingerprints WHERE track_id = ?", id)
	if err != nil {
		t.Fatal(err)
	}
	reloaded, _, err = s.loadMusicScanIndex(ctx)
	if err != nil {
		t.Fatal(err)
	}
	scan = newMusicScanContext(reloaded)
	scanned, _, failures = s.processBatchCounts(ctx, scan, []scanner.ScanFile{file})
	if scanned != 1 || failures != 0 || stub.calls != 3 {
		t.Fatalf("missing baseline: %d/%d probes=%d", scanned, failures, stub.calls)
	}
	_, err = s.tx.DB.Exec("DELETE FROM tracks WHERE id = ?", id)
	if err != nil {
		t.Fatal(err)
	}
	if scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM track_file_fingerprints") != 0 {
		t.Fatal("fingerprint foreign key did not cascade")
	}
	err = storeTrackFingerprint(ctx, s.queries, path, baseline)
	missing := errors.Is(err, sql.ErrNoRows)
	if !missing {
		t.Fatalf("missing catalog accepted a fingerprint: %v", err)
	}
}

func TestFingerprintRollbackPreservesBaseline(t *testing.T) {
	for _, identical := range []bool{false, true} {
		t.Run(map[bool]string{false: "content", true: "fingerprint only"}[identical], func(t *testing.T) {
			s := setupMusicScanner(t)
			defer s.tx.DB.Close()
			ctx := context.Background()
			path := filepath.Join(t.TempDir(), "media.m4a")
			err := os.WriteFile(path, []byte("media"), 0600)
			if err != nil {
				t.Fatal(err)
			}
			stub := &countingMusicScannerFfprobe{result: testMusicMetadata()}
			s.ffprobe = stub
			invalidations := 0
			s.invalidateCommittedTrack = func(int64) { invalidations++ }
			scan := newMusicScanContext(nil)
			file := scanner.ScanFile{Path: path, Ext: "m4a"}
			outcome, _, err := s.processFile(ctx, scan, file)
			if err != nil || outcome != scanner.FileNeedsProcessing {
				t.Fatalf("initial: %v %v", outcome, err)
			}
			baseline := scan.trackIndex[path]
			_, err = s.tx.DB.Exec("UPDATE tracks SET title='retained',updated_at='2000-01-01 00:00:00'")
			if err != nil {
				t.Fatal(err)
			}
			if identical {
				err = os.Chmod(path, 0640)
			} else {
				err = os.WriteFile(path, []byte("other"), 0600)
			}
			if err != nil {
				t.Fatal(err)
			}

			if !identical {
				probeFailure := errors.New("probe failed")
				s.ffprobe = &scannertest.Probe{Callback: func(context.Context, string) (*ffprobe.FfprobeResult, error) { return nil, probeFailure }}
				_, _, err = s.processFile(ctx, scan, file)
				failedProbe := errors.Is(err, probeFailure)
				if !failedProbe {
					t.Fatalf("probe error: %v", err)
				}
				stored, _, err := s.loadMusicScanIndex(ctx)
				if err != nil || stored[path] != baseline || scan.trackIndex[path] != baseline || invalidations != 1 {
					t.Fatalf("probe failure changed baseline: %v", err)
				}
				s.ffprobe = stub
			}
			_, err = s.tx.DB.Exec("CREATE TRIGGER fail_fingerprint BEFORE UPDATE ON track_file_fingerprints BEGIN SELECT RAISE(ABORT,'fingerprint failure'); END")
			if err != nil {
				t.Fatal(err)
			}
			_, _, err = s.processFile(ctx, scan, file)
			if err == nil {
				t.Fatal("expected persistence failure")
			}
			stored, _, err := s.loadMusicScanIndex(ctx)
			if err != nil || stored[path] != baseline || scan.trackIndex[path] != baseline || invalidations != 1 {
				t.Fatalf("baseline changed on rollback: %v", err)
			}
			var name, updated string
			err = s.tx.DB.QueryRow("SELECT title,updated_at FROM tracks").Scan(&name, &updated)
			if err != nil || name != "retained" || updated != "2000-01-01 00:00:00" {
				t.Fatalf("catalog changed on rollback: %s %s %v", name, updated, err)
			}
			_, err = s.tx.DB.Exec("DROP TRIGGER fail_fingerprint")
			if err != nil {
				t.Fatal(err)
			}
			_, _, err = s.processFile(ctx, scan, file)
			if err != nil {
				t.Fatal(err)
			}
			if scan.trackIndex[path] == baseline {
				t.Fatal("retry did not advance baseline")
			}
			outcome, _, err = s.processFile(ctx, scan, file)
			if err != nil || outcome != scanner.FileUnchanged {
				t.Fatalf("repeat after commit: %v %v", outcome, err)
			}
		})
	}
}

func TestFileChangesDuringResolutionAndCommit(t *testing.T) {
	for _, stage := range []string{"resolution", "commit"} {
		t.Run(stage, func(t *testing.T) {
			s := setupMusicScanner(t)
			defer s.tx.DB.Close()
			ctx := context.Background()
			path := filepath.Join(t.TempDir(), "media.m4a")
			err := os.WriteFile(path, []byte("media"), 0600)
			if err != nil {
				t.Fatal(err)
			}
			s.ffprobe = &countingMusicScannerFfprobe{result: testMusicMetadata()}
			scan := newMusicScanContext(nil)
			file := scanner.ScanFile{Path: path, Ext: "m4a"}
			_, _, err = s.processFile(ctx, scan, file)
			if err != nil {
				t.Fatal(err)
			}
			baseline := scan.trackIndex[path]
			invalidations := 0
			s.invalidateCommittedTrack = func(int64) { invalidations++ }
			err = os.WriteFile(path, []byte("other"), 0600)
			if err != nil {
				t.Fatal(err)
			}
			mutate := func() {
				err := os.WriteFile(path+".new", []byte("replacement"), 0600)
				if err != nil {
					t.Fatal(err)
				}
				err = os.Rename(path+".new", path)
				if err != nil {
					t.Fatal(err)
				}
			}
			if stage == "resolution" {
				s.ffprobe = &scannertest.Probe{Callback: func(context.Context, string) (*ffprobe.FfprobeResult, error) {
					unlocked := s.tx.Mu.TryLock()
					if !unlocked {
						t.Error("probe held database mutex")
					} else {
						s.tx.Mu.Unlock()
					}
					mutate()
					return testMusicMetadata(), nil
				}}
			} else {
				conn, err := s.tx.DB.Conn(ctx)
				if err != nil {
					t.Fatal(err)
				}
				err = conn.Raw(func(raw any) error {
					return raw.(*sqlite3.SQLiteConn).RegisterFunc("mutate_file", func() int { mutate(); return 0 }, false)
				})
				conn.Close()
				if err != nil {
					t.Fatal(err)
				}
				_, err = s.tx.DB.Exec("CREATE TRIGGER mutate_before_commit AFTER UPDATE ON track_file_fingerprints BEGIN SELECT mutate_file(); END")
				if err != nil {
					t.Fatal(err)
				}
			}
			unstable := s.processBatchReport(ctx, scan, []scanner.ScanFile{file}).status
			if unstable.Imported != 0 || unstable.Unchanged != 0 || unstable.Failed != 0 || unstable.Deferred != 1 || invalidations != 0 {
				t.Fatalf("unstable: %+v invalidations=%d", unstable, invalidations)
			}
			stored, _, err := s.loadMusicScanIndex(ctx)
			if err != nil || stored[path] != baseline || scan.trackIndex[path] != baseline {
				t.Fatalf("unstable baseline published: %v", err)
			}
		})
	}
}

func TestCanceledFinalBatchDoesNotComplete(t *testing.T) {
	s := setupMusicScanner(t)
	defer s.tx.DB.Close()
	dir := t.TempDir()
	path := filepath.Join(dir, "media.m4a")
	err := os.WriteFile(path, []byte("media"), 0600)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.scanContext = ctx
	s.ffprobe = &scannertest.Probe{Callback: func(context.Context, string) (*ffprobe.FfprobeResult, error) {
		cancel()
		return testMusicMetadata(), nil
	}}
	s.scan(dir)
	log := s.logger.(*scannertest.Logger)
	for _, entry := range log.InfoEntries {
		if strings.Contains(entry.Msg, "scanner completed") {
			t.Fatalf("canceled scan reported completion: %s", entry.Msg)
		}
	}
	if scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM tracks") != 0 {
		t.Fatal("canceled scan persisted item")
	}
}

func TestStoreFingerprintRejectsMissingCatalogRow(t *testing.T) {
	s := setupMusicScanner(t)
	defer s.tx.DB.Close()
	err := storeTrackFingerprint(context.Background(), s.queries, "/missing/media", scanner.FileFingerprint{})
	missing := errors.Is(err, sql.ErrNoRows)
	if !missing {
		t.Fatalf("missing catalog fingerprint error = %v, want sql.ErrNoRows", err)
	}
}
