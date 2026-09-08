//go:build externalbin

package music

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/scanner"
)

func TestMusicMetadataRealProbePersistence(t *testing.T) {
	probe, err := ffprobe.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		err := ffprobe.Cleanup()
		if err != nil {
			t.Error(err)
		}
	})
	for _, ext := range []string{"mp3", "flac", "m4a"} {
		t.Run(ext, func(t *testing.T) {
			s := setupMusicScanner(t)
			defer s.db.Close()
			s.ffprobe = probe
			s.now = func() time.Time { return time.Now().Add(2 * time.Minute) }
			path := filepath.Join(t.TempDir(), "track."+ext)
			scan := newMusicScanContext(nil)
			var initialID int64
			for _, state := range []string{"initial", "changed"} {
				data, err := os.ReadFile(filepath.Join("testdata", state+"."+ext))
				if err != nil {
					t.Fatal(err)
				}
				err = os.WriteFile(path, data, 0600)
				if err != nil {
					t.Fatal(err)
				}
				n, skipped, failures := s.processMusicBatch(context.Background(), scan, []scanner.ScanFile{{Path: path, Ext: ext}})
				if n != 1 || skipped != 0 || failures != 0 {
					t.Fatalf("%s scan: %d/%d/%d", state, n, skipped, failures)
				}
				var id, trackIndex int64
				var title, language string
				err = s.db.QueryRow("SELECT id,title,language,track_index FROM tracks WHERE file_path=?", path).Scan(&id, &title, &language, &trackIndex)
				if err != nil {
					t.Fatal(err)
				}
				wantLanguage := "eng"
				if state == "changed" {
					wantLanguage = "spa"
					if ext == "m4a" {
						wantLanguage = "zxx"
					}
					if id != initialID {
						t.Fatal("changed file lost track identity")
					}
				} else {
					initialID = id
				}
				if title != state || language != wantLanguage || trackIndex != 2 {
					t.Fatalf("%s: title=%q language=%q track=%d", state, title, language, trackIndex)
				}
				count := countScannerRows(t, s.db, "SELECT count(*) FROM musicians WHERE (name='Artist One' AND sort_name='A, One') OR (name='Artist Two' AND sort_name='Two, Artist')")
				if count != 2 {
					t.Fatal("inverted duplicate sort credits not persisted")
				}
				count = countScannerRows(t, s.db, "SELECT count(*) FROM music_credit_metadata")
				if count != 3 {
					t.Fatalf("sort contributions=%d, want 3", count)
				}
				count = countScannerRows(t, s.db, "SELECT count(*) FROM track_musicians")
				if count != 2 {
					t.Fatalf("artist links=%d, want 2", count)
				}
				count = countScannerRows(t, s.db, "SELECT count(*) FROM pragma_foreign_key_check")
				if count != 0 {
					t.Fatal("foreign key violations")
				}
			}
		})
	}
}
