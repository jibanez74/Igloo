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
)

func TestFileFingerprintLifecycle(t *testing.T) {
	s := setupMusicScanner(t)
	defer s.db.Close()
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
	scanned, skipped, failures := s.processMusicBatch(ctx, scan, []scanner.ScanFile{file})
	if scanned != 0 || skipped != 0 || failures != 0 || scan.deferred != 1 || stub.calls != 0 {
		t.Fatalf("recent file: %d/%d/%d deferred=%d probes=%d", scanned, skipped, failures, scan.deferred, stub.calls)
	}
	s.now = func() time.Time { return time.Now().Add(time.Hour) }
	scanned, _, failures = s.processMusicBatch(ctx, scan, []scanner.ScanFile{file})
	if scanned != 1 || failures != 0 || invalidations != 1 {
		t.Fatalf("initial import: %d/%d invalidations=%d", scanned, failures, invalidations)
	}
	baseline := scan.trackIndex[path]
	if baseline.Size != 5 {
		t.Fatalf("size=%d", baseline.Size)
	}
	var id int64
	err = s.db.QueryRow("SELECT id FROM tracks WHERE file_path = ?", path).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	// Persist an old timestamp to make accidental catalog writes observable.
	_, err = s.db.Exec("UPDATE tracks SET updated_at = '2000-01-01 00:00:00' WHERE id = ?", id)
	if err != nil {
		t.Fatal(err)
	}
	before, err := s.queries.GetTrack(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	reloaded, err := s.loadMusicScanIndex(ctx)
	if err != nil || !reflect.DeepEqual(reloaded, scan.trackIndex) {
		t.Fatalf("reload: %+v %v", reloaded, err)
	}
	scan = newMusicScanContext(reloaded)
	scanned, skipped, failures = s.processMusicBatch(ctx, scan, []scanner.ScanFile{file})
	if scanned != 0 || skipped != 1 || failures != 0 || stub.calls != 1 {
		t.Fatalf("unchanged reload: %d/%d/%d probes=%d", scanned, skipped, failures, stub.calls)
	}

	err = os.Chmod(path, 0640)
	if err != nil {
		t.Fatal(err)
	}
	scanned, skipped, failures = s.processMusicBatch(ctx, scan, []scanner.ScanFile{file})
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
	scanned, _, failures = s.processMusicBatch(ctx, scan, []scanner.ScanFile{file})
	if scanned != 1 || failures != 0 || stub.calls != 2 || invalidations != 2 {
		t.Fatalf("same size edit: %d/%d probes=%d invalidations=%d", scanned, failures, stub.calls, invalidations)
	}
	var currentID int64
	err = s.db.QueryRow("SELECT id FROM tracks WHERE file_path = ?", path).Scan(&currentID)
	if err != nil || currentID != id {
		t.Fatalf("catalog ID changed: %d -> %d, %v", id, currentID, err)
	}

	// A missing fingerprint always establishes a new full baseline, even when
	// the catalog already contains the same path and size.
	_, err = s.db.Exec("DELETE FROM track_file_fingerprints WHERE track_id = ?", id)
	if err != nil {
		t.Fatal(err)
	}
	reloaded, err = s.loadMusicScanIndex(ctx)
	if err != nil {
		t.Fatal(err)
	}
	scan = newMusicScanContext(reloaded)
	scanned, _, failures = s.processMusicBatch(ctx, scan, []scanner.ScanFile{file})
	if scanned != 1 || failures != 0 || stub.calls != 3 {
		t.Fatalf("missing baseline: %d/%d probes=%d", scanned, failures, stub.calls)
	}
	_, err = s.db.Exec("DELETE FROM tracks WHERE id = ?", id)
	if err != nil {
		t.Fatal(err)
	}
	if countScannerRows(t, s.db, "SELECT count(*) FROM track_file_fingerprints") != 0 {
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
			defer s.db.Close()
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
			outcome, err := s.processFile(ctx, scan, file)
			if err != nil || outcome != scanner.FileNeedsProcessing {
				t.Fatalf("initial: %v %v", outcome, err)
			}
			baseline := scan.trackIndex[path]
			_, err = s.db.Exec("UPDATE tracks SET title='retained',updated_at='2000-01-01 00:00:00'")
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
				s.ffprobe = &fingerprintProbe{callback: func(context.Context, string) (*ffprobe.FfprobeResult, error) { return nil, probeFailure }}
				_, err = s.processFile(ctx, scan, file)
				failedProbe := errors.Is(err, probeFailure)
				if !failedProbe {
					t.Fatalf("probe error: %v", err)
				}
				stored, err := s.loadMusicScanIndex(ctx)
				if err != nil || stored[path] != baseline || scan.trackIndex[path] != baseline || invalidations != 1 {
					t.Fatalf("probe failure changed baseline: %v", err)
				}
				s.ffprobe = stub
			}
			_, err = s.db.Exec("CREATE TRIGGER fail_fingerprint BEFORE UPDATE ON track_file_fingerprints BEGIN SELECT RAISE(ABORT,'fingerprint failure'); END")
			if err != nil {
				t.Fatal(err)
			}
			_, err = s.processFile(ctx, scan, file)
			if err == nil {
				t.Fatal("expected persistence failure")
			}
			stored, err := s.loadMusicScanIndex(ctx)
			if err != nil || stored[path] != baseline || scan.trackIndex[path] != baseline || invalidations != 1 {
				t.Fatalf("baseline changed on rollback: %v", err)
			}
			var name, updated string
			err = s.db.QueryRow("SELECT title,updated_at FROM tracks").Scan(&name, &updated)
			if err != nil || name != "retained" || updated != "2000-01-01 00:00:00" {
				t.Fatalf("catalog changed on rollback: %s %s %v", name, updated, err)
			}
			_, err = s.db.Exec("DROP TRIGGER fail_fingerprint")
			if err != nil {
				t.Fatal(err)
			}
			_, err = s.processFile(ctx, scan, file)
			if err != nil {
				t.Fatal(err)
			}
			if scan.trackIndex[path] == baseline {
				t.Fatal("retry did not advance baseline")
			}
			outcome, err = s.processFile(ctx, scan, file)
			if err != nil || outcome != scanner.FileUnchanged {
				t.Fatalf("repeat after commit: %v %v", outcome, err)
			}
		})
	}
}

type fingerprintProbe struct {
	callback func(context.Context, string) (*ffprobe.FfprobeResult, error)
}

func (p *fingerprintProbe) GetMetadata(ctx context.Context, path string) (*ffprobe.FfprobeResult, error) {
	return p.callback(ctx, path)
}
func (p *fingerprintProbe) GetAudioMetadata(ctx context.Context, path string) (*ffprobe.FfprobeResult, error) {
	return p.callback(ctx, path)
}
func (p *fingerprintProbe) KeyframeAtOrBefore(context.Context, string, int64, float64) (float64, error) {
	return 0, errors.New("unused")
}

func TestFileChangesDuringResolutionAndCommit(t *testing.T) {
	for _, stage := range []string{"resolution", "commit"} {
		t.Run(stage, func(t *testing.T) {
			s := setupMusicScanner(t)
			defer s.db.Close()
			ctx := context.Background()
			path := filepath.Join(t.TempDir(), "media.m4a")
			err := os.WriteFile(path, []byte("media"), 0600)
			if err != nil {
				t.Fatal(err)
			}
			s.ffprobe = &countingMusicScannerFfprobe{result: testMusicMetadata()}
			scan := newMusicScanContext(nil)
			file := scanner.ScanFile{Path: path, Ext: "m4a"}
			_, err = s.processFile(ctx, scan, file)
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
				s.ffprobe = &fingerprintProbe{callback: func(context.Context, string) (*ffprobe.FfprobeResult, error) {
					unlocked := s.scannerDBMu.TryLock()
					if !unlocked {
						t.Error("probe held database mutex")
					} else {
						s.scannerDBMu.Unlock()
					}
					mutate()
					return testMusicMetadata(), nil
				}}
			} else {
				conn, err := s.db.Conn(ctx)
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
				_, err = s.db.Exec("CREATE TRIGGER mutate_before_commit AFTER UPDATE ON track_file_fingerprints BEGIN SELECT mutate_file(); END")
				if err != nil {
					t.Fatal(err)
				}
			}
			scanned, skipped, failures := s.processMusicBatch(ctx, scan, []scanner.ScanFile{file})
			if scanned != 0 || skipped != 0 || failures != 0 || scan.deferred != 1 || invalidations != 0 {
				t.Fatalf("unstable: %d/%d/%d deferred=%d invalidations=%d", scanned, skipped, failures, scan.deferred, invalidations)
			}
			stored, err := s.loadMusicScanIndex(ctx)
			if err != nil || stored[path] != baseline || scan.trackIndex[path] != baseline {
				t.Fatalf("unstable baseline published: %v", err)
			}
		})
	}
}

func TestCanceledFinalBatchDoesNotComplete(t *testing.T) {
	s := setupMusicScanner(t)
	defer s.db.Close()
	dir := t.TempDir()
	path := filepath.Join(dir, "media.m4a")
	err := os.WriteFile(path, []byte("media"), 0600)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.scanContext = ctx
	s.ffprobe = &fingerprintProbe{callback: func(context.Context, string) (*ffprobe.FfprobeResult, error) {
		cancel()
		return testMusicMetadata(), nil
	}}
	s.runMusicScan(dir)
	log := s.logger.(*capturedLogger)
	for _, entry := range log.infoEntries {
		if strings.Contains(entry.msg, "scanner completed") {
			t.Fatalf("canceled scan reported completion: %s", entry.msg)
		}
	}
	if countScannerRows(t, s.db, "SELECT count(*) FROM tracks") != 0 {
		t.Fatal("canceled scan persisted item")
	}
}
