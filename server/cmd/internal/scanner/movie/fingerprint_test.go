package movie

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
	fixture := setupMovieScanner(t)
	s := fixture.scanner
	defer fixture.db.Close()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "media.mkv")
	err := os.WriteFile(path, []byte("media"), 0600)
	if err != nil {
		t.Fatal(err)
	}
	stub := &stubMovieScannerFfprobe{result: movieScannerMetadataFixture("120")}
	s.ffprobe = stub
	invalidations := 0
	s.invalidateCommittedMovie = func(int64) { invalidations++ }
	scan := newMovieScanContext(nil)
	file := scanner.ScanFile{Path: path, Ext: "mkv", Size: 999} // Walking size is deliberately stale.
	s.now = time.Now
	scanned, skipped, failures, deferred := s.processMoviesBatch(ctx, scan, []scanner.ScanFile{file})
	if scanned != 0 || skipped != 0 || failures != 0 || deferred != 1 || stub.calls != 0 {
		t.Fatalf("recent file: %d/%d/%d deferred=%d probes=%d", scanned, skipped, failures, deferred, stub.calls)
	}
	s.now = func() time.Time { return time.Now().Add(time.Hour) }
	scanned, _, failures, _ = s.processMoviesBatch(ctx, scan, []scanner.ScanFile{file})
	if scanned != 1 || failures != 0 || invalidations != 1 {
		t.Fatalf("initial import: %d/%d invalidations=%d", scanned, failures, invalidations)
	}
	baseline := scan.movieIndex[path]
	if baseline.Size != 5 {
		t.Fatalf("size=%d", baseline.Size)
	}
	var id int64
	err = s.db.QueryRow("SELECT id FROM movies WHERE file_path = ?", path).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	// Persist an old timestamp to make accidental catalog writes observable.
	_, err = s.db.Exec("UPDATE movies SET updated_at = '2000-01-01 00:00:00' WHERE id = ?", id)
	if err != nil {
		t.Fatal(err)
	}
	before, err := s.queries.GetMovieByID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	reloaded, _, err := s.loadMovieScanIndex(ctx)
	if err != nil || !reflect.DeepEqual(reloaded, scan.movieIndex) {
		t.Fatalf("reload: %+v %v", reloaded, err)
	}
	scan = newMovieScanContext(reloaded)
	scanned, skipped, failures, _ = s.processMoviesBatch(ctx, scan, []scanner.ScanFile{file})
	if scanned != 0 || skipped != 1 || failures != 0 || stub.calls != 1 {
		t.Fatalf("unchanged reload: %d/%d/%d probes=%d", scanned, skipped, failures, stub.calls)
	}
	_, err = s.db.Exec("INSERT INTO keyframe_indexes(movie_id,stream_index,fingerprint,duration_sec,keyframes) VALUES (?,0,'old',120,'[0]')", id)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.db.Exec("INSERT INTO remux_safety_verdicts(movie_id,stream_index,fingerprint,safe) VALUES (?,0,'old',1)", id)
	if err != nil {
		t.Fatal(err)
	}
	err = os.Chmod(path, 0640)
	if err != nil {
		t.Fatal(err)
	}
	scanned, skipped, failures, _ = s.processMoviesBatch(ctx, scan, []scanner.ScanFile{file})
	if scanned != 1 || skipped != 0 || failures != 0 || stub.calls != 2 || invalidations != 2 {
		t.Fatalf("identical bytes: %d/%d/%d probes=%d invalidations=%d", scanned, skipped, failures, stub.calls, invalidations)
	}
	after, err := s.queries.GetMovieByID(ctx, id)
	if err != nil || before.ID != after.ID || before.Title != after.Title || after.UpdatedAt == before.UpdatedAt {
		t.Fatalf("metadata change did not retain identity and refresh technical state: %+v %+v %v", before, after, err)
	}
	if scan.movieIndex[path] == baseline {
		t.Fatal("fingerprint did not advance")
	}
	if countScannerRows(t, s.db, "SELECT count(*) FROM keyframe_indexes") != 0 || countScannerRows(t, s.db, "SELECT count(*) FROM remux_safety_verdicts") != 0 {
		t.Fatal("changed filesystem metadata retained playback work")
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
	scanned, _, failures, _ = s.processMoviesBatch(ctx, scan, []scanner.ScanFile{file})
	if scanned != 1 || failures != 0 || stub.calls != 3 || invalidations != 3 {
		t.Fatalf("same size edit: %d/%d probes=%d invalidations=%d", scanned, failures, stub.calls, invalidations)
	}
	var currentID int64
	err = s.db.QueryRow("SELECT id FROM movies WHERE file_path = ?", path).Scan(&currentID)
	if err != nil || currentID != id {
		t.Fatalf("catalog ID changed: %d -> %d, %v", id, currentID, err)
	}
	if countScannerRows(t, s.db, "SELECT count(*) FROM keyframe_indexes") != 0 || countScannerRows(t, s.db, "SELECT count(*) FROM remux_safety_verdicts") != 0 {
		t.Fatal("changed bytes retained playback work")
	}
	// A missing fingerprint always establishes a new full baseline, even when
	// the catalog already contains the same path and size.
	_, err = s.db.Exec("DELETE FROM movie_file_fingerprints WHERE movie_id = ?", id)
	if err != nil {
		t.Fatal(err)
	}
	reloaded, _, err = s.loadMovieScanIndex(ctx)
	if err != nil {
		t.Fatal(err)
	}
	scan = newMovieScanContext(reloaded)
	scanned, _, failures, _ = s.processMoviesBatch(ctx, scan, []scanner.ScanFile{file})
	if scanned != 1 || failures != 0 || stub.calls != 4 {
		t.Fatalf("missing baseline: %d/%d probes=%d", scanned, failures, stub.calls)
	}
	_, err = s.db.Exec("DELETE FROM movies WHERE id = ?", id)
	if err != nil {
		t.Fatal(err)
	}
	if countScannerRows(t, s.db, "SELECT count(*) FROM movie_file_fingerprints") != 0 {
		t.Fatal("fingerprint foreign key did not cascade")
	}
	err = storeMovieFingerprint(ctx, s.queries, path, baseline.FileFingerprint)
	missing := errors.Is(err, sql.ErrNoRows)
	if !missing {
		t.Fatalf("missing catalog accepted a fingerprint: %v", err)
	}
}

func TestFingerprintRollbackPreservesBaseline(t *testing.T) {
	for _, identical := range []bool{false, true} {
		t.Run(map[bool]string{false: "content", true: "metadata change"}[identical], func(t *testing.T) {
			fixture := setupMovieScanner(t)
			s := fixture.scanner
			defer fixture.db.Close()
			ctx := context.Background()
			path := filepath.Join(t.TempDir(), "media.mkv")
			err := os.WriteFile(path, []byte("media"), 0600)
			if err != nil {
				t.Fatal(err)
			}
			stub := &stubMovieScannerFfprobe{result: movieScannerMetadataFixture("120")}
			s.ffprobe = stub
			invalidations := 0
			s.invalidateCommittedMovie = func(int64) { invalidations++ }
			scan := newMovieScanContext(nil)
			file := scanner.ScanFile{Path: path, Ext: "mkv"}
			outcome, err := s.processFile(ctx, scan, file)
			if err != nil || outcome != scanner.FileNeedsProcessing {
				t.Fatalf("initial: %v %v", outcome, err)
			}
			baseline := scan.movieIndex[path]
			_, err = s.db.Exec("UPDATE movies SET title='retained',updated_at='2000-01-01 00:00:00'")
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
				stored, _, err := s.loadMovieScanIndex(ctx)
				if err != nil || stored[path] != baseline || scan.movieIndex[path] != baseline || invalidations != 1 {
					t.Fatalf("probe failure changed baseline: %v", err)
				}
				s.ffprobe = stub
			}
			_, err = s.db.Exec("CREATE TRIGGER fail_fingerprint BEFORE UPDATE ON movie_file_fingerprints BEGIN SELECT RAISE(ABORT,'fingerprint failure'); END")
			if err != nil {
				t.Fatal(err)
			}
			_, err = s.processFile(ctx, scan, file)
			if err == nil {
				t.Fatal("expected persistence failure")
			}
			stored, _, err := s.loadMovieScanIndex(ctx)
			if err != nil || stored[path] != baseline || scan.movieIndex[path] != baseline || invalidations != 1 {
				t.Fatalf("baseline changed on rollback: %v", err)
			}
			var name, updated string
			err = s.db.QueryRow("SELECT title,updated_at FROM movies").Scan(&name, &updated)
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
			if scan.movieIndex[path] == baseline {
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
			fixture := setupMovieScanner(t)
			s := fixture.scanner
			defer fixture.db.Close()
			ctx := context.Background()
			path := filepath.Join(t.TempDir(), "media.mkv")
			err := os.WriteFile(path, []byte("media"), 0600)
			if err != nil {
				t.Fatal(err)
			}
			s.ffprobe = &stubMovieScannerFfprobe{result: movieScannerMetadataFixture("120")}
			scan := newMovieScanContext(nil)
			file := scanner.ScanFile{Path: path, Ext: "mkv"}
			_, err = s.processFile(ctx, scan, file)
			if err != nil {
				t.Fatal(err)
			}
			baseline := scan.movieIndex[path]
			invalidations := 0
			s.invalidateCommittedMovie = func(int64) { invalidations++ }
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
					return movieScannerMetadataFixture("120"), nil
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
				_, err = s.db.Exec("CREATE TRIGGER mutate_before_commit AFTER UPDATE ON movie_file_fingerprints BEGIN SELECT mutate_file(); END")
				if err != nil {
					t.Fatal(err)
				}
			}
			scanned, skipped, failures, deferred := s.processMoviesBatch(ctx, scan, []scanner.ScanFile{file})
			if scanned != 0 || skipped != 0 || failures != 0 || deferred != 1 || invalidations != 0 {
				t.Fatalf("unstable: %d/%d/%d deferred=%d invalidations=%d", scanned, skipped, failures, deferred, invalidations)
			}
			stored, _, err := s.loadMovieScanIndex(ctx)
			if err != nil || stored[path] != baseline || scan.movieIndex[path] != baseline {
				t.Fatalf("unstable baseline published: %v", err)
			}
		})
	}
}

func TestCanceledFinalBatchDoesNotComplete(t *testing.T) {
	fixture := setupMovieScanner(t)
	s := fixture.scanner
	defer fixture.db.Close()
	dir := t.TempDir()
	path := filepath.Join(dir, "media.mkv")
	err := os.WriteFile(path, []byte("media"), 0600)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.scanContext = ctx
	s.ffprobe = &fingerprintProbe{callback: func(context.Context, string) (*ffprobe.FfprobeResult, error) {
		cancel()
		return movieScannerMetadataFixture("120"), nil
	}}
	fixture.moviesDir.String, fixture.moviesDir.Valid = dir, true
	s.scan(s.currentMoviesDirectory().String)
	log := s.logger.(*capturedLogger)
	for _, entry := range log.infoEntries {
		if strings.Contains(entry.msg, "scanner completed") {
			t.Fatalf("canceled scan reported completion: %s", entry.msg)
		}
	}
	if countScannerRows(t, s.db, "SELECT count(*) FROM movies") != 0 {
		t.Fatal("canceled scan persisted item")
	}
}

func TestStoreFingerprintRejectsMissingCatalogRow(t *testing.T) {
	s := setupMovieScanner(t)
	defer s.db.Close()
	err := storeMovieFingerprint(context.Background(), s.queries, "/missing/media", scanner.FileFingerprint{})
	missing := errors.Is(err, sql.ErrNoRows)
	if !missing {
		t.Fatalf("missing catalog fingerprint error = %v, want sql.ErrNoRows", err)
	}
}
