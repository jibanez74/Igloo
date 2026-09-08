package music

import (
	"context"
	"database/sql"
	"errors"
	"igloo/cmd/internal/ffprobe"
	"os"
	"path/filepath"
	"testing"
	"time"

	sqlite3 "github.com/mattn/go-sqlite3"
	"igloo/cmd/internal/scanner"
)

func TestMissingMusicCleanupLifecycle(t *testing.T) {
	for _, scenario := range []string{"deleted", "removed subdirectory", "broken symlink", "no fingerprint", "outside directory", "unavailable root"} {
		t.Run(scenario, func(t *testing.T) {
			s := setupMusicScanner(t)
			defer s.db.Close()
			root := t.TempDir()
			directory := filepath.Join(root, "library")
			err := os.Mkdir(directory, 0700)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(directory, "media.m4a")
			if scenario == "removed subdirectory" {
				err = os.Mkdir(filepath.Join(directory, "album"), 0700)
				if err != nil {
					t.Fatal(err)
				}
				path = filepath.Join(directory, "album", "media.m4a")
			}
			if scenario == "outside directory" {
				err = os.Mkdir(directory+"-other", 0700)
				if err != nil {
					t.Fatal(err)
				}
				path = filepath.Join(directory+"-other", "media.m4a")
			}
			err = os.WriteFile(path, []byte("media"), 0600)
			if err != nil {
				t.Fatal(err)
			}
			probe := &countingMusicScannerFfprobe{result: testMusicMetadata()}
			s.ffprobe = probe
			scan := newMusicScanContext(nil)
			imported, _, failures := s.processMusicBatch(context.Background(), scan, []scanner.ScanFile{{Path: path, Ext: "m4a"}})
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
				_, err = s.db.Exec("DELETE FROM track_file_fingerprints")
			case "unavailable root":
				err = os.Remove(directory)
			}
			if err != nil {
				t.Fatal(err)
			}
			invalidations := 0
			s.invalidateCommittedTrack = func(id int64) {
				invalidations++
				count := countScannerRows(t, s.db, "SELECT count(*) FROM tracks WHERE id = ?", id)
				if count != 0 {
					t.Error("invalidation preceded commit")
				}
			}
			s.runMusicScan(directory)
			s.runMusicScan(directory)
			wantRows, wantInvalidations := 0, 1
			if scenario == "outside directory" || scenario == "unavailable root" {
				wantRows, wantInvalidations = 1, 0
			}
			count := countScannerRows(t, s.db, "SELECT count(*) FROM tracks")
			if count != wantRows || invalidations != wantInvalidations || probe.calls != 1 {
				t.Fatalf("rows=%d invalidations=%d probes=%d", count, invalidations, probe.calls)
			}
		})
	}
}

func TestMissingMusicDeletionTransaction(t *testing.T) {
	for _, scenario := range []string{"commit", "rollback", "reappeared", "root replaced", "path changed", "id changed"} {
		t.Run(scenario, func(t *testing.T) {
			s := setupMusicScanner(t)
			defer s.db.Close()
			ctx := context.Background()
			root := filepath.Join(t.TempDir(), "library")
			err := os.Mkdir(root, 0700)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, "media.m4a")
			err = os.WriteFile(path, []byte("media"), 0600)
			if err != nil {
				t.Fatal(err)
			}
			s.ffprobe = &countingMusicScannerFfprobe{result: testMusicMetadata()}
			scan := newMusicScanContext(nil)
			imported, _, failures := s.processMusicBatch(ctx, scan, []scanner.ScanFile{{Path: path, Ext: "m4a"}})
			if imported != 1 || failures != 0 {
				t.Fatalf("import=%d errors=%d", imported, failures)
			}
			_, files, err := s.loadMusicScanIndex(ctx)
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
			case "rollback":
				_, err = s.db.Exec("CREATE TRIGGER reject_delete AFTER DELETE ON tracks BEGIN SELECT RAISE(ABORT,'delete failed'); END")
			case "path changed":
				_, err = s.db.Exec("UPDATE tracks SET file_path = ? WHERE id = ?", path+".moved", files[0].ID)
			case "id changed":
				// This fixture has cascading dependents; changing the identity requires a fresh row.
				_, err = s.db.Exec("DELETE FROM tracks WHERE id = ?", files[0].ID)
				if err == nil {
					err = os.WriteFile(path, []byte("replacement"), 0600)
					if err != nil {
						t.Fatal(err)
					}
					imported, _, failures = s.processMusicBatch(ctx, newMusicScanContext(nil), []scanner.ScanFile{{Path: path, Ext: "m4a"}})
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
					_, err = s.db.Exec("CREATE TRIGGER change_cleanup AFTER DELETE ON tracks BEGIN SELECT change_cleanup_file(); END")
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			invalidations := 0
			s.invalidateCommittedTrack = func(id int64) {
				invalidations++
				count := countScannerRows(t, s.db, "SELECT count(*) FROM tracks WHERE id = ?", id)
				if count != 0 {
					t.Error("invalidation preceded commit")
				}
				unlocked := s.scannerDBMu.TryLock()
				if unlocked {
					s.scannerDBMu.Unlock()
					t.Error("deletion callback did not hold database mutex")
				}
			}
			deleted, err := s.cleanupMissingMusic(ctx, scan, reconciliation)
			wantDeleted := 0
			if scenario == "commit" {
				wantDeleted = 1
			}
			if scenario == "rollback" && err == nil {
				t.Fatal("expected transaction failure")
			}
			if deleted != wantDeleted || invalidations != wantDeleted {
				t.Fatalf("deleted=%d invalidations=%d err=%v", deleted, invalidations, err)
			}
			_, indexed := scan.trackIndex[path]
			if indexed != (wantDeleted == 0) {
				t.Fatalf("fingerprint index published incorrectly: %v", indexed)
			}
			count := countScannerRows(t, s.db, "SELECT count(*) FROM tracks")
			if count != 1-wantDeleted {
				t.Fatalf("rows=%d", count)
			}
			count = countScannerRows(t, s.db, "SELECT count(*) FROM track_file_fingerprints")
			if count != 1-wantDeleted {
				t.Fatalf("fingerprints=%d", count)
			}
		})
	}
}

func TestMusicCleanupProtectsSeenFilesAndInterruptedScans(t *testing.T) {
	for _, scenario := range []string{"failed", "deferred", "unchanged", "canceled", "root replaced"} {
		t.Run(scenario, func(t *testing.T) {
			s := setupMusicScanner(t)
			defer s.db.Close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			root := filepath.Join(t.TempDir(), "library")
			err := os.Mkdir(root, 0700)
			if err != nil {
				t.Fatal(err)
			}
			seenPath := filepath.Join(root, "a-seen.m4a")
			triggerPath := filepath.Join(root, "b-trigger.m4a")
			missingPath := filepath.Join(root, "c-missing.m4a")
			s.ffprobe = &fingerprintProbe{callback: func(context.Context, string) (*ffprobe.FfprobeResult, error) { return testMusicMetadata(), nil }}
			for _, path := range []string{seenPath, triggerPath, missingPath} {
				err = os.WriteFile(path, []byte("media"), 0600)
				if err != nil {
					t.Fatal(err)
				}
				imported, _, failures := s.processMusicBatch(ctx, newMusicScanContext(nil), []scanner.ScanFile{{Path: path, Ext: "m4a"}})
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
			s.invalidateCommittedTrack = func(int64) { invalidations++ }
			s.runMusicScan(root)
			wantRows, wantInvalidations := 2, 1
			if scenario == "canceled" || scenario == "root replaced" {
				wantRows, wantInvalidations = 3, 0
			}
			count := countScannerRows(t, s.db, "SELECT count(*) FROM tracks")
			if count != wantRows || invalidations != wantInvalidations {
				t.Fatalf("rows=%d invalidations=%d", count, invalidations)
			}
			count = countScannerRows(t, s.db, "SELECT count(*) FROM tracks WHERE file_path = ?", seenPath)
			if count != 1 {
				t.Fatal("seen file deleted during processing was removed")
			}
		})
	}
}

func TestMusicCleanupCapturesConfiguredDirectory(t *testing.T) {
	s := setupMusicScanner(t)
	defer s.db.Close()
	first, second := t.TempDir(), t.TempDir()
	s.ffprobe = &fingerprintProbe{callback: func(context.Context, string) (*ffprobe.FfprobeResult, error) { return testMusicMetadata(), nil }}
	for _, root := range []string{first, second} {
		path := filepath.Join(root, "missing.m4a")
		err := os.WriteFile(path, []byte("media"), 0600)
		if err != nil {
			t.Fatal(err)
		}
		imported, _, failures := s.processMusicBatch(context.Background(), newMusicScanContext(nil), []scanner.ScanFile{{Path: path, Ext: "m4a"}})
		if imported != 1 || failures != 0 {
			t.Fatal("import failed")
		}
		err = os.Remove(path)
		if err != nil {
			t.Fatal(err)
		}
	}
	calls := 0
	s.currentMusicDirectory = func() sql.NullString {
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
	count := countScannerRows(t, s.db, "SELECT count(*) FROM tracks WHERE file_path = ?", filepath.Join(second, "missing.m4a"))
	total := countScannerRows(t, s.db, "SELECT count(*) FROM tracks")
	if calls != 1 || count != 1 || total != 1 {
		t.Fatalf("directory reads=%d second library=%d total=%d", calls, count, total)
	}
}

func TestMusicCleanupReconcilesMetadataAndCascades(t *testing.T) {
	s := setupMusicScanner(t)
	defer s.db.Close()
	root := t.TempDir()
	first, second := filepath.Join(root, "first.m4a"), filepath.Join(root, "second.m4a")
	scan := newMusicScanContext(nil)
	tags := ffprobe.FormatTags{Title: "First", Artist: "Artist", Album: "Album", Genre: "Rock", SortArtist: "A", SortAlbum: "A", Date: "2010"}
	scanTaggedTrack(t, s, scan, first, 1, tags)
	tags.Title, tags.SortArtist, tags.SortAlbum, tags.Date = "Second", "B", "B", "2020"
	scanTaggedTrack(t, s, scan, second, 1, tags)
	_, err := s.db.Exec(`
 INSERT INTO users(id,name,email,password) VALUES(1,'Listener','listener@example.test','unused');
 INSERT INTO playlists(id,user_id,name) VALUES(1,1,'Favorites');
 INSERT INTO playlist_tracks(playlist_id,track_id,position) SELECT 1,id,id FROM tracks;
 INSERT INTO user_play_history(user_id,track_id) SELECT 1,id FROM tracks;
 INSERT INTO user_track_stats(user_id,track_id) SELECT 1,id FROM tracks;
 INSERT INTO user_liked_tracks(user_id,track_id) SELECT 1,id FROM tracks;
 INSERT INTO music_album_metadata(album_id,spotify_date) SELECT id,'2000-01-01' FROM albums;
 CREATE TRIGGER reject_derived_update BEFORE UPDATE OF sort_title ON albums BEGIN SELECT RAISE(ABORT,'derived failure'); END;
 `)
	if err != nil {
		t.Fatal(err)
	}
	err = os.Remove(first)
	if err != nil {
		t.Fatal(err)
	}
	_, files, err := s.loadMusicScanIndex(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	reconciliation, err := scanner.NewReconciliation(root, files)
	if err != nil {
		t.Fatal(err)
	}
	invalidations := 0
	s.invalidateCommittedTrack = func(int64) { invalidations++ }
	deleted, err := s.cleanupMissingMusic(context.Background(), scan, reconciliation)
	if err == nil || deleted != 0 || invalidations != 0 {
		t.Fatalf("rollback: %d %d %v", deleted, invalidations, err)
	}
	for _, table := range []string{"tracks", "track_file_fingerprints", "track_musicians", "track_genres", "music_track_metadata", "music_credit_metadata", "tracks_search_fts", "playlist_tracks", "user_play_history", "user_track_stats", "user_liked_tracks"} {
		count := countScannerRows(t, s.db, "SELECT count(*) FROM "+table)
		if count != 2 {
			t.Fatalf("rollback left %d rows in %s", count, table)
		}
	}
	count := countScannerRows(t, s.db, "SELECT count(*) FROM musicians WHERE sort_name='A'")
	if count != 1 {
		t.Fatal("artist sort update did not roll back")
	}
	_, err = s.db.Exec("DROP TRIGGER reject_derived_update")
	if err != nil {
		t.Fatal(err)
	}
	deleted, err = s.cleanupMissingMusic(context.Background(), scan, reconciliation)
	if err != nil || deleted != 1 || invalidations != 1 {
		t.Fatalf("commit: %d %d %v", deleted, invalidations, err)
	}
	var artistSort, albumSort, date string
	var year int
	err = s.db.QueryRow("SELECT m.sort_name,a.sort_title,a.release_date,a.year FROM musicians m CROSS JOIN albums a").Scan(&artistSort, &albumSort, &date, &year)
	if err != nil {
		t.Fatal(err)
	}
	if artistSort != "B" || albumSort != "B" || date != "2020-01-01" || year != 2020 {
		t.Fatalf("remaining contributions: %s/%s/%s/%d", artistSort, albumSort, date, year)
	}
	err = os.Remove(second)
	if err != nil {
		t.Fatal(err)
	}
	// No surviving references should be sent to Spotify after cleanup.
	spotify := &musicScannerSpotifyStub{}
	s.spotify = spotify
	s.runMusicScan(root)
	if spotify.artistCalls != 0 || spotify.albumCalls != 0 {
		t.Fatal("deleted tracks caused Spotify retries")
	}
	for _, table := range []string{"tracks", "track_file_fingerprints", "track_musicians", "track_genres", "music_track_metadata", "music_credit_metadata", "tracks_search_fts", "playlist_tracks", "user_play_history", "user_track_stats", "user_liked_tracks", "musician_albums", "musician_genres", "album_genres"} {
		count := countScannerRows(t, s.db, "SELECT count(*) FROM "+table)
		if count != 0 {
			t.Fatalf("cleanup left %d rows in %s", count, table)
		}
	}
	for _, table := range []string{"musicians", "albums", "genres", "playlists"} {
		count := countScannerRows(t, s.db, "SELECT count(*) FROM "+table)
		if count != 1 {
			t.Fatalf("shared %s rows=%d", table, count)
		}
	}
	err = s.db.QueryRow("SELECT m.sort_name,a.sort_title,a.release_date,a.year FROM musicians m CROSS JOIN albums a").Scan(&artistSort, &albumSort, &date, &year)
	if err != nil {
		t.Fatal(err)
	}
	if artistSort != "Artist" || albumSort != "Album" || date != "2000-01-01" || year != 2000 {
		t.Fatalf("fallback: %s/%s/%s/%d", artistSort, albumSort, date, year)
	}
}
