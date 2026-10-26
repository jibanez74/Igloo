package movie

import (
	"os"
	"path/filepath"
	"testing"

	"igloo/cmd/internal/scanner"
	"igloo/cmd/internal/scanner/scannertest"
)

// movieWatchedAt seeds a user and a watch position for the movie at path and
// returns the movie's id.
func movieWatchedAt(t *testing.T, fixture *movieScannerTestContext, path string) int64 {
	t.Helper()
	var id int64
	err := fixture.db.QueryRow("SELECT id FROM movies WHERE file_path = ?", path).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	_, err = fixture.db.Exec("INSERT INTO users (name, email, password) VALUES ('u', 'u@example.com', 'p')")
	if err != nil {
		t.Fatal(err)
	}
	_, err = fixture.db.Exec("INSERT INTO movie_watch_progress (user_id, movie_id, progress_sec, duration_sec) VALUES (1, ?, 1800, 6000)", id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// A movie renamed or moved within the library, as Radarr does on an upgrade
// or a folder rename, keeps its row and with it its watch progress, whether a
// run of changes or a library scan sees the rename.
func TestMovieRenameKeepsTheMovieAndItsProgress(t *testing.T) {
	for _, watched := range []bool{false, true} {
		t.Run(map[bool]string{false: "library scan", true: "run of changes"}[watched], func(t *testing.T) {
			fixture := setupMovieScanner(t)
			s := fixture.scanner
			s.ffprobe = &scannertest.CountingProbe{Default: movieScannerMetadataFixture("120")}
			root := t.TempDir()
			old := scannertest.WriteFile(t, filepath.Join(root, "Heat (1995)", "Heat 720p.mkv"), "heat")
			s.scan(root)
			id := movieWatchedAt(t, fixture, old)

			renamed := filepath.Join(root, "Heat (1995) [Remux]", "Heat 1080p.mkv")
			err := os.MkdirAll(filepath.Dir(renamed), 0o755)
			if err != nil {
				t.Fatal(err)
			}
			err = os.Rename(old, renamed)
			if err != nil {
				t.Fatal(err)
			}
			err = os.Remove(filepath.Dir(old))
			if err != nil {
				t.Fatal(err)
			}
			if watched {
				s.scanChanges(root, scanner.Changes{Paths: []string{"Heat (1995)", "Heat (1995) [Remux]/Heat 1080p.mkv"}})
			} else {
				s.scan(root)
			}

			var movedID int64
			var fileName string
			err = fixture.db.QueryRow("SELECT id, file_name FROM movies WHERE file_path = ?", renamed).Scan(&movedID, &fileName)
			if err != nil {
				t.Fatal(err)
			}
			if movedID != id || fileName != "Heat 1080p.mkv" {
				t.Fatalf("moved row id %d (want %d), file name %q", movedID, id, fileName)
			}
			if scannertest.CountRows(t, fixture.db, "SELECT count(*) FROM movies") != 1 || scannertest.CountRows(t, fixture.db, "SELECT count(*) FROM movie_watch_progress") != 1 {
				t.Fatal("the rename left a second movie or lost the watch progress")
			}
			status := s.Status()
			if status.Deleted != 0 || status.Imported != 0 {
				t.Fatalf("a rename counted as a removal and an import: %+v", status.Counts)
			}
		})
	}
}

// A copy leaves the original in place, so the copy is a movie of its own.
func TestMovieCopyIsNotAMove(t *testing.T) {
	fixture := setupMovieScanner(t)
	s := fixture.scanner
	s.ffprobe = &scannertest.CountingProbe{Default: movieScannerMetadataFixture("120")}
	root := t.TempDir()
	original := scannertest.WriteFile(t, filepath.Join(root, "Heat (1995)", "Heat.mkv"), "heat")
	s.scan(root)
	err := os.Link(original, filepath.Join(root, "Heat (1995)", "Heat copy.mkv"))
	if err != nil {
		t.Fatal(err)
	}
	s.scan(root)
	if got := scannertest.CountRows(t, fixture.db, "SELECT count(*) FROM movies"); got != 2 {
		t.Fatalf("%d movies, want the original and its link", got)
	}
}

// An unmatched movie renamed to fix its match is searched again at once,
// rather than after the backoff its misses earned.
func TestMovieRenameClearsTheMissBackoffOfAnUnmatchedMovie(t *testing.T) {
	fixture := setupMovieScanner(t)
	s := fixture.scanner
	s.ffprobe = &scannertest.CountingProbe{Default: movieScannerMetadataFixture("120")}
	root := t.TempDir()
	old := scannertest.WriteFile(t, filepath.Join(root, "Hete (1995)", "Hete.mkv"), "heat")
	s.scan(root)
	_, err := fixture.db.Exec("UPDATE movie_tmdb_retries SET attempts = 3, last_attempt_at = 1")
	if err != nil {
		t.Fatal(err)
	}
	renamed := filepath.Join(root, "Hete (1995)", "Heat (1995).mkv")
	err = os.Rename(old, renamed)
	if err != nil {
		t.Fatal(err)
	}
	s.scan(root)
	var attempts int
	err = fixture.db.QueryRow("SELECT attempts FROM movie_tmdb_retries").Scan(&attempts)
	if err != nil || attempts != 0 {
		t.Fatalf("attempts after the rename = %d (%v), want 0", attempts, err)
	}
}
