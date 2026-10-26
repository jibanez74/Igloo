package music

import (
	"os"
	"path/filepath"
	"testing"

	"igloo/cmd/internal/scanner"
	"igloo/cmd/internal/scanner/scannertest"
)

// An album folder renamed within the library, as Picard or Lidarr does, keeps
// its tracks' rows and with them their likes, whether a run of changes or a
// library scan sees the rename.
func TestMusicRenameKeepsTheTrackAndItsLike(t *testing.T) {
	for _, watched := range []bool{false, true} {
		t.Run(map[bool]string{false: "library scan", true: "run of changes"}[watched], func(t *testing.T) {
			s := setupMusicScanner(t)
			s.ffprobe = &scannertest.CountingProbe{Default: testMusicMetadata()}
			root := t.TempDir()
			old := scannertest.WriteFile(t, filepath.Join(root, "Album", "01 Track.flac"), "track")
			s.scan(root)
			db := s.tx.DB
			var id int64
			err := db.QueryRow("SELECT id FROM tracks WHERE file_path = ?", old).Scan(&id)
			if err != nil {
				t.Fatal(err)
			}
			_, err = db.Exec("INSERT INTO users (name, email, password) VALUES ('u', 'u@example.com', 'p')")
			if err != nil {
				t.Fatal(err)
			}
			_, err = db.Exec("INSERT INTO user_liked_tracks (user_id, track_id) VALUES (1, ?)", id)
			if err != nil {
				t.Fatal(err)
			}

			err = os.Rename(filepath.Join(root, "Album"), filepath.Join(root, "Album (2001)"))
			if err != nil {
				t.Fatal(err)
			}
			renamed := filepath.Join(root, "Album (2001)", "01 Track.flac")
			if watched {
				s.scanChanges(root, scanner.Changes{Paths: []string{"Album", "Album (2001)/01 Track.flac"}})
			} else {
				s.scan(root)
			}

			var movedID int64
			err = db.QueryRow("SELECT id FROM tracks WHERE file_path = ?", renamed).Scan(&movedID)
			if err != nil {
				t.Fatal(err)
			}
			if movedID != id || scannertest.CountRows(t, db, "SELECT count(*) FROM tracks") != 1 {
				t.Fatalf("moved row id %d, want %d, with no second track", movedID, id)
			}
			if scannertest.CountRows(t, db, "SELECT count(*) FROM user_liked_tracks") != 1 {
				t.Fatal("the rename lost the like")
			}
			status := s.Status()
			if status.Deleted != 0 || status.Imported != 0 {
				t.Fatalf("a rename counted as a removal and an import: %+v", status.Counts)
			}
		})
	}
}
