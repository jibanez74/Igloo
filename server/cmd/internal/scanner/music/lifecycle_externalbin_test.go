//go:build externalbin

package music

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"igloo/cmd/internal/scanner"
	"igloo/cmd/internal/scanner/scannertest"
)

func TestMusicMetadataRealProbePersistence(t *testing.T) {
	probe := scannertest.RealProbe(t)
	for _, ext := range []string{"mp3", "flac", "m4a"} {
		t.Run(ext, func(t *testing.T) {
			s := setupMusicScanner(t)
			s.ffprobe = probe
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
				n, skipped, failures := s.processBatchCounts(context.Background(), scan, []scanner.ScanFile{{Path: path, Ext: ext}})
				if n != 1 || skipped != 0 || failures != 0 {
					t.Fatalf("%s scan: %d/%d/%d", state, n, skipped, failures)
				}
				var id, trackIndex int64
				var title, language string
				err = s.tx.DB.QueryRow("SELECT id,title,language,track_index FROM tracks WHERE file_path=?", path).Scan(&id, &title, &language, &trackIndex)
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
				count := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM musicians WHERE (name='Artist One' AND sort_name='A, One') OR (name='Artist Two' AND sort_name='Two, Artist')")
				if count != 2 {
					t.Fatal("inverted duplicate sort credits not persisted")
				}
				count = scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM music_credit_metadata")
				if count != 3 {
					t.Fatalf("sort contributions=%d, want 3", count)
				}
				count = scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM track_musicians")
				if count != 2 {
					t.Fatalf("artist links=%d, want 2", count)
				}
				count = scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM pragma_foreign_key_check")
				if count != 0 {
					t.Fatal("foreign key violations")
				}
			}
		})
	}
}

// The formats Igloo's own FFmpeg cannot write, probed for real: Opus and Ogg
// keep their tags on the audio stream, AIFF in ID3 frames, WAV in RIFF INFO.
func TestMusicRealProbeReadsEachNewFormat(t *testing.T) {
	probe := scannertest.RealProbe(t)
	cases := []struct {
		ext, mime string
		// WAV's INFO chunk has no field for the rest of the tags.
		fullTags bool
	}{
		{"opus", "audio/ogg", true},
		{"ogg", "audio/ogg", true},
		{"aiff", "audio/aiff", true},
		{"wav", "audio/wav", false},
	}
	for _, tc := range cases {
		t.Run(tc.ext, func(t *testing.T) {
			s := setupMusicScanner(t)
			s.ffprobe = probe
			data, err := os.ReadFile(filepath.Join("testdata", "tagged."+tc.ext))
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "track."+tc.ext)
			err = os.WriteFile(path, data, 0600)
			if err != nil {
				t.Fatal(err)
			}
			n, _, failures := s.processBatchCounts(context.Background(), newMusicScanContext(nil), []scanner.ScanFile{{Path: path, Ext: tc.ext}})
			if n != 1 || failures != 0 {
				t.Fatalf("scan: %d imported, %d failed", n, failures)
			}

			var title, mime, album string
			var trackIndex int64
			err = s.tx.DB.QueryRow("SELECT t.title, t.mime_type, t.track_index, a.title FROM tracks t JOIN albums a ON a.id = t.album_id WHERE t.file_path = ?", path).Scan(&title, &mime, &trackIndex, &album)
			if err != nil {
				t.Fatal(err)
			}
			if title != "Tagged" || mime != tc.mime || trackIndex != 2 || album != "Format Test" {
				t.Fatalf("track = %q, %q, %d, album %q", title, mime, trackIndex, album)
			}
			if !tc.fullTags {
				return
			}
			var release string
			var compilation bool
			var trackTotal int64
			err = s.tx.DB.QueryRow("SELECT a.musicbrainz_release_id, m.compilation, m.track_total FROM albums a JOIN tracks t ON t.album_id = a.id JOIN music_track_metadata m ON m.track_id = t.id").Scan(&release, &compilation, &trackTotal)
			if err != nil {
				t.Fatal(err)
			}
			if release != "03271bb1-a396-4038-a068-b7d5945c2c37" || !compilation || trackTotal != 3 {
				t.Fatalf("release, compilation, track total = %q, %v, %d", release, compilation, trackTotal)
			}
			credits := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM track_musicians tm JOIN musicians m ON m.id = tm.musician_id WHERE m.name IN ('Format Band', 'Guest')")
			if credits != 2 {
				t.Fatalf("ARTISTS credits = %d, want Format Band and Guest", credits)
			}
		})
	}
}
