package movie

import (
	"context"
	"database/sql"
	"errors"
	"igloo/cmd/internal/ffprobe"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	sqlite3 "github.com/mattn/go-sqlite3"
	"igloo/cmd/internal/scanner"
)

func TestMissingMovieCleanupLifecycle(t *testing.T) {
	for _, scenario := range []string{"deleted", "removed subdirectory", "broken symlink", "no fingerprint", "outside directory", "unavailable root"} {
		t.Run(scenario, func(t *testing.T) {
			fixture := setupMovieScanner(t)
			s := fixture.scanner
			defer s.db.Close()
			root := t.TempDir()
			directory := filepath.Join(root, "library")
			err := os.Mkdir(directory, 0700)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(directory, "media.mkv")
			if scenario == "removed subdirectory" {
				err = os.Mkdir(filepath.Join(directory, "album"), 0700)
				if err != nil {
					t.Fatal(err)
				}
				path = filepath.Join(directory, "album", "media.mkv")
			}
			if scenario == "outside directory" {
				err = os.Mkdir(directory+"-other", 0700)
				if err != nil {
					t.Fatal(err)
				}
				path = filepath.Join(directory+"-other", "media.mkv")
			}
			err = os.WriteFile(path, []byte("media"), 0600)
			if err != nil {
				t.Fatal(err)
			}
			probe := &stubMovieScannerFfprobe{result: movieScannerMetadataFixture("120")}
			s.ffprobe = probe
			scan := newMovieScanContext(nil)
			imported, _, failures := s.processMoviesBatch(context.Background(), scan, []scanner.ScanFile{{Path: path, Ext: "mkv"}})
			if imported != 1 || failures != 0 {
				t.Fatalf("import=%d errors=%d", imported, failures)
			}
			err = os.Remove(path)
			if err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "removed subdirectory":
				err = os.Remove(filepath.Dir(path))
			case "broken symlink":
				err = os.Symlink(filepath.Join(root, "missing-target"), path)
			case "no fingerprint":
				_, err = s.db.Exec("DELETE FROM movie_file_fingerprints")
			case "unavailable root":
				err = os.Remove(directory)
			}
			if err != nil {
				t.Fatal(err)
			}
			invalidations := 0
			s.invalidateCommittedMovie = func(id int64) {
				invalidations++
				count := countScannerRows(t, s.db, "SELECT count(*) FROM movies WHERE id = ?", id)
				if count != 0 {
					t.Error("invalidation preceded commit")
				}
			}
			s.runMovieScan(directory)
			s.runMovieScan(directory)
			wantRows, wantInvalidations := 0, 1
			if scenario == "outside directory" || scenario == "unavailable root" {
				wantRows, wantInvalidations = 1, 0
			}
			count := countScannerRows(t, s.db, "SELECT count(*) FROM movies")
			if count != wantRows || invalidations != wantInvalidations || probe.calls != 1 {
				t.Fatalf("rows=%d invalidations=%d probes=%d", count, invalidations, probe.calls)
			}
		})
	}
}

func TestMissingMovieDeletionTransaction(t *testing.T) {
	for _, scenario := range []string{"commit", "rollback", "query failure", "canceled", "reappeared", "root replaced", "path changed", "id changed"} {
		t.Run(scenario, func(t *testing.T) {
			fixture := setupMovieScanner(t)
			s := fixture.scanner
			defer s.db.Close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			root := filepath.Join(t.TempDir(), "library")
			err := os.Mkdir(root, 0700)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, "media.mkv")
			err = os.WriteFile(path, []byte("media"), 0600)
			if err != nil {
				t.Fatal(err)
			}
			s.ffprobe = &stubMovieScannerFfprobe{result: movieScannerMetadataFixture("120")}
			scan := newMovieScanContext(nil)
			imported, _, failures := s.processMoviesBatch(ctx, scan, []scanner.ScanFile{{Path: path, Ext: "mkv"}})
			if imported != 1 || failures != 0 {
				t.Fatalf("import=%d errors=%d", imported, failures)
			}
			_, files, err := s.loadMovieScanIndex(ctx)
			if err != nil {
				t.Fatal(err)
			}
			_, err = s.db.Exec(`INSERT INTO users(id,name,email,password) VALUES(1,'Viewer','viewer@test','unused');
 INSERT INTO watch_rooms(id,owner_user_id,movie_id,playback_mode) VALUES(11,1,?,'direct'),(12,1,?,'direct');`, files[0].ID, files[0].ID)
			if err != nil {
				t.Fatal(err)
			}
			reconciliation, err := scanner.NewReconciliation(root, files)
			if err != nil {
				t.Fatal(err)
			}
			err = os.Remove(path)
			if err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "query failure":
				_, err = s.db.Exec("ALTER TABLE watch_rooms RENAME TO unavailable_watch_rooms")
			case "canceled":
				cancel()
			case "rollback":
				_, err = s.db.Exec("CREATE TRIGGER reject_delete AFTER DELETE ON movies BEGIN SELECT RAISE(ABORT,'delete failed'); END")
			case "path changed":
				_, err = s.db.Exec("UPDATE movies SET file_path = ? WHERE id = ?", path+".moved", files[0].ID)
			case "id changed":
				// This fixture has cascading dependents; changing the identity requires a fresh row.
				_, err = s.db.Exec("DELETE FROM movies WHERE id = ?", files[0].ID)
				if err == nil {
					err = os.WriteFile(path, []byte("replacement"), 0600)
					if err != nil {
						t.Fatal(err)
					}
					imported, _, failures = s.processMoviesBatch(ctx, newMovieScanContext(nil), []scanner.ScanFile{{Path: path, Ext: "mkv"}})
					if imported != 1 || failures != 0 {
						t.Fatal("replacement import failed")
					}
					err = os.Remove(path)
				}
			case "reappeared", "root replaced":
				conn, connErr := s.db.Conn(ctx)
				if connErr != nil {
					t.Fatal(connErr)
				}
				err = conn.Raw(func(raw any) error {
					return raw.(*sqlite3.SQLiteConn).RegisterFunc("change_cleanup_file", func() int {
						var changeErr error
						if scenario == "reappeared" {
							changeErr = os.WriteFile(path, []byte("back"), 0600)
						} else {
							changeErr = os.Rename(root, root+"-old")
							if changeErr == nil {
								changeErr = os.Mkdir(root, 0700)
							}
						}
						if changeErr != nil {
							t.Error(changeErr)
						}
						return 0
					}, false)
				})
				conn.Close()
				if err == nil {
					_, err = s.db.Exec("CREATE TRIGGER change_cleanup AFTER DELETE ON movies BEGIN SELECT change_cleanup_file(); END")
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			roomInvalidations := 0
			s.invalidateDeletedWatchRooms = func(ids []int64) {
				roomInvalidations++
				if !slices.Equal(ids, []int64{11, 12}) {
					t.Errorf("room IDs = %v", ids)
				}
				count := countScannerRows(t, s.db, "SELECT count(*) FROM watch_rooms")
				if count != 0 {
					t.Error("room callback preceded commit")
				}
			}
			invalidations := 0
			s.invalidateCommittedMovie = func(id int64) {
				invalidations++
				if roomInvalidations != 1 {
					t.Error("movie invalidation preceded room invalidation")
				}
				count := countScannerRows(t, s.db, "SELECT count(*) FROM movies WHERE id = ?", id)
				if count != 0 {
					t.Error("invalidation preceded commit")
				}
				unlocked := s.scannerDBMu.TryLock()
				if unlocked {
					s.scannerDBMu.Unlock()
					t.Error("deletion callback did not hold database mutex")
				}
			}
			deleted, err := s.cleanupMissingMovie(ctx, scan, reconciliation)
			wantDeleted := 0
			if scenario == "commit" {
				wantDeleted = 1
			}
			if (scenario == "rollback" || scenario == "query failure" || scenario == "canceled") && err == nil {
				t.Fatal("expected transaction failure")
			}
			if deleted != wantDeleted || invalidations != wantDeleted || roomInvalidations != wantDeleted {
				t.Fatalf("deleted=%d invalidations=%d err=%v", deleted, invalidations, err)
			}
			_, indexed := scan.movieIndex[path]
			if indexed != (wantDeleted == 0) {
				t.Fatalf("fingerprint index published incorrectly: %v", indexed)
			}
			count := countScannerRows(t, s.db, "SELECT count(*) FROM movies")
			if count != 1-wantDeleted {
				t.Fatalf("rows=%d", count)
			}
			count = countScannerRows(t, s.db, "SELECT count(*) FROM movie_file_fingerprints")
			if count != 1-wantDeleted {
				t.Fatalf("fingerprints=%d", count)
			}
		})
	}
}

func TestMovieCleanupProtectsSeenFilesAndInterruptedScans(t *testing.T) {
	for _, scenario := range []string{"failed", "deferred", "unchanged", "canceled", "root replaced"} {
		t.Run(scenario, func(t *testing.T) {
			fixture := setupMovieScanner(t)
			s := fixture.scanner
			defer s.db.Close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			root := filepath.Join(t.TempDir(), "library")
			err := os.Mkdir(root, 0700)
			if err != nil {
				t.Fatal(err)
			}
			seenPath := filepath.Join(root, "a-seen.mkv")
			triggerPath := filepath.Join(root, "b-trigger.mkv")
			missingPath := filepath.Join(root, "c-missing.mkv")
			s.ffprobe = &fingerprintProbe{callback: func(context.Context, string) (*ffprobe.FfprobeResult, error) {
				return movieScannerMetadataFixture("120"), nil
			}}
			for _, path := range []string{seenPath, triggerPath, missingPath} {
				err = os.WriteFile(path, []byte("media"), 0600)
				if err != nil {
					t.Fatal(err)
				}
				imported, _, failures := s.processMoviesBatch(ctx, newMovieScanContext(nil), []scanner.ScanFile{{Path: path, Ext: "mkv"}})
				if imported != 1 || failures != 0 {
					t.Fatalf("import=%d errors=%d", imported, failures)
				}
			}
			err = os.Remove(missingPath)
			if err != nil {
				t.Fatal(err)
			}
			err = os.WriteFile(triggerPath, []byte("changed trigger"), 0600)
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "failed" {
				err = os.WriteFile(seenPath, []byte("changed seen"), 0600)
				if err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "deferred" {
				future := time.Now().Add(24 * time.Hour)
				err = os.Chtimes(seenPath, future, future)
				if err != nil {
					t.Fatal(err)
				}
			}
			s.scanContext = ctx
			s.ffprobe = &fingerprintProbe{callback: func(_ context.Context, path string) (*ffprobe.FfprobeResult, error) {
				if path == triggerPath {
					err := os.Remove(seenPath)
					if err != nil {
						t.Error(err)
					}
					if scenario == "canceled" {
						cancel()
					}
					if scenario == "root replaced" {
						err = os.Rename(root, root+"-old")
						if err == nil {
							err = os.Mkdir(root, 0700)
						}
						if err != nil {
							t.Error(err)
						}
					}
				}
				return nil, errors.New("processing failed")
			}}
			invalidations := 0
			s.invalidateCommittedMovie = func(int64) { invalidations++ }
			s.runMovieScan(root)
			wantRows, wantInvalidations := 2, 1
			if scenario == "canceled" || scenario == "root replaced" {
				wantRows, wantInvalidations = 3, 0
			}
			count := countScannerRows(t, s.db, "SELECT count(*) FROM movies")
			if count != wantRows || invalidations != wantInvalidations {
				t.Fatalf("rows=%d invalidations=%d", count, invalidations)
			}
			count = countScannerRows(t, s.db, "SELECT count(*) FROM movies WHERE file_path = ?", seenPath)
			if count != 1 {
				t.Fatal("seen file deleted during processing was removed")
			}
		})
	}
}

func TestMovieCleanupCapturesConfiguredDirectory(t *testing.T) {
	fixture := setupMovieScanner(t)
	s := fixture.scanner
	defer s.db.Close()
	first, second := t.TempDir(), t.TempDir()
	s.ffprobe = &fingerprintProbe{callback: func(context.Context, string) (*ffprobe.FfprobeResult, error) {
		return movieScannerMetadataFixture("120"), nil
	}}
	for _, root := range []string{first, second} {
		path := filepath.Join(root, "missing.mkv")
		err := os.WriteFile(path, []byte("media"), 0600)
		if err != nil {
			t.Fatal(err)
		}
		imported, _, failures := s.processMoviesBatch(context.Background(), newMovieScanContext(nil), []scanner.ScanFile{{Path: path, Ext: "mkv"}})
		if imported != 1 || failures != 0 {
			t.Fatal("import failed")
		}
		err = os.Remove(path)
		if err != nil {
			t.Fatal(err)
		}
	}
	calls := 0
	s.currentMoviesDirectory = func() sql.NullString {
		calls++
		directory := first
		if calls > 1 {
			directory = second
		}
		return sql.NullString{String: directory, Valid: true}
	}
	result := s.Start()
	if result.Status != StartStarted {
		t.Fatal(result)
	}
	s.wait.Wait()
	count := countScannerRows(t, s.db, "SELECT count(*) FROM movies WHERE file_path = ?", filepath.Join(second, "missing.mkv"))
	total := countScannerRows(t, s.db, "SELECT count(*) FROM movies")
	if calls != 1 || count != 1 || total != 1 {
		t.Fatalf("directory reads=%d second library=%d total=%d", calls, count, total)
	}
}

func TestMovieCleanupCascadesDependentRows(t *testing.T) {
	fixture := setupMovieScanner(t)
	s := fixture.scanner
	defer s.db.Close()
	root := t.TempDir()
	path := filepath.Join(root, "UniqueMovie.mkv")
	err := os.WriteFile(path, []byte("media"), 0600)
	if err != nil {
		t.Fatal(err)
	}
	s.ffprobe = &stubMovieScannerFfprobe{result: movieScannerMetadataFixture("120")}
	imported, _, failures := s.processMoviesBatch(context.Background(), newMovieScanContext(nil), []scanner.ScanFile{{Path: path, Ext: "mkv"}})
	if imported != 1 || failures != 0 {
		t.Fatal("import failed")
	}
	_, err = s.db.Exec(`
 INSERT INTO users(id,name,email,password) VALUES(1,'Viewer','viewer@example.test','unused');
 INSERT INTO playlists(id,user_id,name,content_type) VALUES(1,1,'Favorites','movie');
 INSERT INTO playlist_movies(playlist_id,movie_id,position) SELECT 1,id,1 FROM movies;
 INSERT INTO movie_watch_progress(user_id,movie_id) SELECT 1,id FROM movies;
 INSERT INTO user_liked_movies(user_id,movie_id) SELECT 1,id FROM movies;
 INSERT INTO watch_rooms(owner_user_id,movie_id,playback_mode) SELECT 1,id,'direct' FROM movies;
 INSERT INTO keyframe_indexes(movie_id,stream_index,fingerprint,duration_sec,keyframes) SELECT id,0,'old',120,'[0]' FROM movies;
 INSERT INTO remux_safety_verdicts(movie_id,stream_index,fingerprint,safe) SELECT id,0,'old',1 FROM movies;
 INSERT INTO genres(tag,genre_type) VALUES('Drama','movie');
 INSERT INTO movie_genres(movie_id,genre_id) SELECT m.id,g.id FROM movies m CROSS JOIN genres g;
 `)
	if err != nil {
		t.Fatal(err)
	}
	err = os.Remove(path)
	if err != nil {
		t.Fatal(err)
	}
	s.runMovieScan(root)
	for _, table := range []string{"movies", "movie_file_fingerprints", "video_streams", "audio_streams", "subtitles", "chapters", "movie_genres", "movie_watch_progress", "user_liked_movies", "playlist_movies", "watch_rooms", "keyframe_indexes", "remux_safety_verdicts"} {
		count := countScannerRows(t, s.db, "SELECT count(*) FROM "+table)
		if count != 0 {
			t.Fatalf("cleanup left %d rows in %s", count, table)
		}
	}
	count := countScannerRows(t, s.db, "SELECT count(*) FROM movies_fts WHERE movies_fts MATCH 'UniqueMovie'")
	if count != 0 {
		t.Fatal("search index retained deleted movie")
	}
	for _, table := range []string{"genres", "playlists"} {
		count := countScannerRows(t, s.db, "SELECT count(*) FROM "+table)
		if count != 1 {
			t.Fatalf("shared %s rows=%d", table, count)
		}
	}
}
