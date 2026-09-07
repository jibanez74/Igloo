package music

import (
	"context"
	"fmt"
	"testing"

	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/scanner"
)

// Each iteration scans 120 changed files against real SQLite. Setup is excluded;
// database writes, resolution, and transaction cache publication are measured.
func BenchmarkMusicScan(b *testing.B) {
	for _, distinct := range []bool{false, true} {
		name := "RepeatedAlbum"
		if distinct {
			name = "DistinctTracks"
		}
		b.Run(name, func(b *testing.B) {
			s := setupMusicScanner(b)
			defer s.db.Close()
			results := make(map[string]*ffprobe.FfprobeResult)
			files := make([]scanner.ScanFile, 120)
			for i := range files {
				path := fmt.Sprintf("/music/%d.m4a", i)
				tags := ffprobe.FormatTags{Title: fmt.Sprintf("Track %d", i), Artist: "Artist", Album: "Album", Genre: "Rock"}
				if distinct {
					tags.Artist = fmt.Sprintf("Artist %d", i)
					tags.Album = fmt.Sprintf("Album %d", i)
				}
				results[path] = testMusicMetadataWithTags(tags)
				files[i] = scanner.ScanFile{Path: path, Ext: "m4a", Size: 1}
			}
			s.ffprobe = newMusicScannerFfprobeByPath(results)
			s.spotify = &musicScannerSpotifyStub{}
			b.ReportAllocs()
			b.ResetTimer()
			for n := 0; n < b.N; n++ {
				for i := range files {
					files[i].Size = int64(n + 1)
				}
				scanned, _, failures := s.processMusicBatch(context.Background(), newMusicScanContext(nil), files)
				if scanned != len(files) || failures != 0 {
					b.Fatalf("scanned=%d failures=%d", scanned, failures)
				}
			}
		})
	}
}
