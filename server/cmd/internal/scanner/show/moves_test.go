package show

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"igloo/cmd/internal/scanner"
	"igloo/cmd/internal/scanner/scannertest"
)

// episodeWatched seeds a user and a watch position for the one episode and
// returns the show's and the episode's ids.
func episodeWatched(t *testing.T, s *Scanner) (int64, int64) {
	t.Helper()
	var showID, episodeID int64
	err := s.DB.QueryRow("SELECT show_id, e.id FROM show_episodes e JOIN show_seasons ss ON ss.id = e.season_id").Scan(&showID, &episodeID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.DB.Exec("INSERT INTO users (name, email, password) VALUES ('u', 'u@example.com', 'p')")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.DB.Exec("INSERT INTO show_episode_watch_progress (user_id, episode_id, progress_sec, duration_sec) VALUES (1, ?, 600, 2400)", episodeID)
	if err != nil {
		t.Fatal(err)
	}
	return showID, episodeID
}

// A show folder renamed to fix its match, as the scanner's unmatched issue
// suggests, keeps its show, episode and watch progress, and takes the new
// folder's name and year, whether a run of changes or a library scan sees it.
func TestShowFolderRenameKeepsTheShowAndItsProgress(t *testing.T) {
	for _, watched := range []bool{false, true} {
		t.Run(map[bool]string{false: "library scan", true: "run of changes"}[watched], func(t *testing.T) {
			s, _, root := setupScanner(t)
			scannertest.WriteFile(t, filepath.Join(root, "Show/Season 1/S01E01.mkv"), "e1")
			scanOK(t, s, root)
			showID, episodeID := episodeWatched(t, s)
			revision := s.Status().CatalogRevision
			_, err := s.DB.Exec("UPDATE show_tmdb_retries SET attempts = 3, last_attempt_at = 1")
			if err != nil {
				t.Fatal(err)
			}

			err = os.Rename(filepath.Join(root, "Show"), filepath.Join(root, "Show (2020)"))
			if err != nil {
				t.Fatal(err)
			}
			if watched {
				s.scanChanges(root, scanner.Changes{Paths: []string{"Show", "Show (2020)/Season 1/S01E01.mkv"}})
			} else {
				scanOK(t, s, root)
			}

			var directory, localName string
			var year int64
			err = s.DB.QueryRow("SELECT directory_path, local_name, premiere_year FROM shows WHERE id = ?", showID).Scan(&directory, &localName, &year)
			if err != nil {
				t.Fatalf("the show did not survive the rename: %v", err)
			}
			if directory != filepath.Join(root, "Show (2020)") || localName != "Show" || year != 2020 {
				t.Fatalf("show after the rename: %q %q %d", directory, localName, year)
			}
			// The web client refreshes its lists when the revision moves.
			if s.Status().CatalogRevision <= revision {
				t.Fatalf("catalog revision %d after the rename, was %d", s.Status().CatalogRevision, revision)
			}
			attempts := scannertest.CountRows(t, s.DB, "SELECT attempts FROM show_tmdb_retries")
			if attempts != 0 {
				t.Fatalf("the unmatched show keeps %d misses of backoff after its rename", attempts)
			}
			if scannertest.CountRows(t, s.DB, "SELECT count(*) FROM shows") != 1 || scannertest.CountRows(t, s.DB, "SELECT count(*) FROM show_files") != 1 {
				t.Fatal("the rename left a second show or file")
			}
			var progress int
			err = s.DB.QueryRow("SELECT count(*) FROM show_episode_watch_progress WHERE episode_id = ?", episodeID).Scan(&progress)
			if err != nil || progress != 1 {
				t.Fatalf("the rename lost the episode's watch progress: %d %v", progress, err)
			}
		})
	}
}

// A file moved into another season's folder is another episode: it is
// imported anew and its old row cleaned up.
func TestShowFileMovedToAnotherSeasonIsImportedAnew(t *testing.T) {
	s, _, root := setupScanner(t)
	scannertest.WriteFile(t, filepath.Join(root, "Show/Season 1/S01E01.mkv"), "e1")
	scanOK(t, s, root)
	err := os.MkdirAll(filepath.Join(root, "Show/Season 2"), 0o755)
	if err != nil {
		t.Fatal(err)
	}
	err = os.Rename(filepath.Join(root, "Show/Season 1/S01E01.mkv"), filepath.Join(root, "Show/Season 2/S02E01.mkv"))
	if err != nil {
		t.Fatal(err)
	}
	scanOK(t, s, root)
	status := s.Status()
	if status.Failed != 0 || scannertest.CountRows(t, s.DB, "SELECT count(*) FROM show_files") != 1 {
		t.Fatalf("after the season move: %+v", status.Counts)
	}
	var season int64
	err = s.DB.QueryRow("SELECT ss.season_number FROM show_files f JOIN show_seasons ss ON ss.id = f.season_id").Scan(&season)
	if err != nil || season != 2 {
		t.Fatalf("the file is in season %d (%v), want 2", season, err)
	}
}

// A show folder only follows a rename that took every one of its files with
// it to one new folder that has no show; otherwise each moved file is
// imported anew, and nothing fails.
func TestShowFolderMovesOnlyWhenTheWholeFolderWasRenamed(t *testing.T) {
	for _, tc := range []struct {
		name   string
		layout []string
		move   func(t *testing.T, root string)
	}{
		{"one episode moved out of a folder that remains", []string{"Show/Season 1/S01E01.mkv", "Show/Season 1/S01E02.mkv"}, func(t *testing.T, root string) {
			rename(t, root, "Show/Season 1/S01E02.mkv", "Spin-off/Season 1/S01E02.mkv")
		}},
		{"a folder split in two", []string{"Show/Season 1/S01E01.mkv", "Show/Season 1/S01E02.mkv"}, func(t *testing.T, root string) {
			rename(t, root, "Show/Season 1/S01E01.mkv", "A/Season 1/S01E01.mkv")
			rename(t, root, "Show/Season 1/S01E02.mkv", "B/Season 1/S01E02.mkv")
			removeAll(t, root, "Show")
		}},
		{"a folder renamed onto another show's", []string{"Show/Season 1/S01E01.mkv", "Other/Season 2/S02E01.mkv"}, func(t *testing.T, root string) {
			rename(t, root, "Show/Season 1/S01E01.mkv", "Other/Season 1/S01E01.mkv")
			removeAll(t, root, "Show")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _, root := setupScanner(t)
			for _, path := range tc.layout {
				scannertest.WriteFile(t, filepath.Join(root, path), path)
			}
			scanOK(t, s, root)
			original := filepath.Join(root, "Show")
			tc.move(t, root)
			scanOK(t, s, root)
			if s.Status().Failed != 0 {
				t.Fatalf("after the move: %+v", s.Status().Counts)
			}
			folderStays := scannertest.CountRows(t, s.DB, "SELECT count(*) FROM shows WHERE directory_path = '"+original+"'")
			_, err := os.Stat(original)
			if err == nil && folderStays != 1 {
				t.Fatal("the show of a folder that remains moved")
			}
			if scannertest.CountRows(t, s.DB, "SELECT count(*) FROM show_files") != len(tc.layout) {
				t.Fatal("a moved file was lost or duplicated")
			}
		})
	}
}

// A matched show keeps its match through a folder rename that still names it,
// as Sonarr's {Series TitleYear} folders add the first air year; any other
// rename of a matched show is how a wrong match is fixed, since shows have no
// Identify, so its files are imported as a new show and searched again.
func TestMatchedShowFolderRename(t *testing.T) {
	for _, tc := range []struct {
		name, folder string
		kept         bool
	}{
		{"its first air year added", "Show (2020)", true},
		{"another year added", "Show (1999)", false},
		{"another title", "Other Show", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _, root := setupScanner(t)
			scannertest.WriteFile(t, filepath.Join(root, "Show/Season 1/S01E01.mkv"), "e1")
			scanOK(t, s, root)
			showID, episodeID := episodeWatched(t, s)
			_, err := s.DB.Exec("UPDATE shows SET tmdb_id = 2316, name = 'Show', first_air_date = '2020-03-24'")
			if err != nil {
				t.Fatal(err)
			}
			rename(t, root, "Show", tc.folder)
			scanOK(t, s, root)
			var id int64
			var tmdbID sql.NullInt64
			err = s.DB.QueryRow("SELECT id, tmdb_id FROM shows WHERE directory_path = ?", filepath.Join(root, tc.folder)).Scan(&id, &tmdbID)
			if err != nil {
				t.Fatal(err)
			}
			kept := id == showID && tmdbID.Valid
			if kept != tc.kept {
				t.Fatalf("show %d (was %d), tmdb %v: kept %v, want %v", id, showID, tmdbID, kept, tc.kept)
			}
			var progress int
			err = s.DB.QueryRow("SELECT count(*) FROM show_episode_watch_progress WHERE episode_id = ?", episodeID).Scan(&progress)
			if err != nil || (progress == 1) != tc.kept {
				t.Fatalf("watch progress rows %d (%v), want kept %v", progress, err, tc.kept)
			}
			if scannertest.CountRows(t, s.DB, "SELECT count(*) FROM shows") != 1 {
				t.Fatal("the old show outlived its files")
			}
		})
	}
}

// An episode file renamed to other episode numbers keeps its row and is
// linked to them, as a Sonarr rename after a renumbering does.
func TestShowFileRenamedToAnotherEpisodeIsRelinked(t *testing.T) {
	s, _, root := setupScanner(t)
	scannertest.WriteFile(t, filepath.Join(root, "Show/Season 1/S01E01.mkv"), "e1")
	scanOK(t, s, root)
	var fileID int64
	err := s.DB.QueryRow("SELECT id FROM show_files").Scan(&fileID)
	if err != nil {
		t.Fatal(err)
	}
	rename(t, root, "Show/Season 1/S01E01.mkv", "Show/Season 1/S01E02.mkv")
	scanOK(t, s, root)
	var movedID, episode int64
	err = s.DB.QueryRow("SELECT f.id, e.episode_number FROM show_files f JOIN show_episode_files l ON l.file_id = f.id JOIN show_episodes e ON e.id = l.episode_id").Scan(&movedID, &episode)
	if err != nil {
		t.Fatal(err)
	}
	if movedID != fileID || episode != 2 {
		t.Fatalf("after the rename: file %d (want %d) linked to episode %d (want 2)", movedID, fileID, episode)
	}
}

func rename(t *testing.T, root, from, to string) {
	t.Helper()
	err := os.MkdirAll(filepath.Dir(filepath.Join(root, to)), 0o755)
	if err != nil {
		t.Fatal(err)
	}
	err = os.Rename(filepath.Join(root, from), filepath.Join(root, to))
	if err != nil {
		t.Fatal(err)
	}
}

func removeAll(t *testing.T, root, path string) {
	t.Helper()
	err := os.RemoveAll(filepath.Join(root, path))
	if err != nil {
		t.Fatal(err)
	}
}

// On a filesystem whose rename leaves the file's ctime alone, a file renamed
// to other episode numbers would read as unchanged; it is processed again
// regardless, which links it to them.
func TestShowFileRenamedToAnotherEpisodeIsRelinkedWithoutANewCtime(t *testing.T) {
	s, _, root := setupScanner(t)
	scannertest.WriteFile(t, filepath.Join(root, "Show/Season 1/S01E01.mkv"), "e1")
	scanOK(t, s, root)
	rename(t, root, "Show/Season 1/S01E01.mkv", "Show/Season 1/S01E02.mkv")
	info, err := os.Stat(filepath.Join(root, "Show/Season 1/S01E02.mkv"))
	if err != nil {
		t.Fatal(err)
	}
	// Record the ctime the rename gave, as if the rename had kept it.
	_, err = s.DB.Exec("UPDATE show_file_fingerprints SET ctime_ns = ?", scanner.FingerprintOf(info).CtimeNS)
	if err != nil {
		t.Fatal(err)
	}
	scanOK(t, s, root)
	var episode int64
	err = s.DB.QueryRow("SELECT e.episode_number FROM show_episode_files l JOIN show_episodes e ON e.id = l.episode_id").Scan(&episode)
	if err != nil || episode != 2 {
		t.Fatalf("linked to episode %d (%v), want 2", episode, err)
	}
}
