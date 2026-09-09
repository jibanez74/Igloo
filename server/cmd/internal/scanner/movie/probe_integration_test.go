//go:build externalbin

package movie

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/scanner"
)

func TestMovieRealProbeVideoAndArtwork(t *testing.T) {
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
	for _, tc := range []struct {
		name, ext, codec string
		index            int64
		artwork          bool
	}{
		{"moving", "avi", "mjpeg", 1, false},
		{"artwork-with-video", "mp4", "h264", 0, true},
		{"artwork-only", "mp4", "", 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := setupMovieScanner(t)
			defer fixture.db.Close()
			s := fixture.scanner
			s.ffprobe = probe
			filename := tc.name + "." + tc.ext
			data, err := os.ReadFile(filepath.Join("testdata", filename))
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), filename)
			err = os.WriteFile(path, data, 0600)
			if err != nil {
				t.Fatal(err)
			}
			metadata, err := probe.GetMetadata(context.Background(), path)
			if err != nil {
				t.Fatal(err)
			}
			attached := false
			for _, stream := range metadata.Streams {
				if stream.Disposition.AttachedPic == 1 {
					attached = true
				}
			}
			if attached != tc.artwork {
				t.Fatalf("fixture artwork disposition=%v want=%v", attached, tc.artwork)
			}
			_, err = s.processFile(context.Background(), nextMovieScan(t, s), scanner.ScanFile{Path: path, Ext: tc.ext})
			if tc.codec == "" {
				if err == nil {
					t.Fatal("artwork-only movie imported")
				}
				for _, table := range []string{"movies", "movie_file_fingerprints", "movie_tmdb_retries", "video_streams"} {
					if countScannerRows(t, s.db, "SELECT count(*) FROM "+table) != 0 {
						t.Fatalf("artwork-only import leaked %s", table)
					}
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			movie, err := readTestMovieByPath(context.Background(), s.queries, path)
			if err != nil {
				t.Fatal(err)
			}
			streams, err := s.queries.GetVideoStreamsByMovieID(context.Background(), movie.ID)
			if err != nil {
				t.Fatal(err)
			}
			if len(streams) != 1 || streams[0].Codec != tc.codec || streams[0].StreamIndex != tc.index {
				t.Fatalf("persisted video streams=%+v", streams)
			}
			if streams[0].Width != 64 || streams[0].Height != 48 || movie.Duration.Float64 <= 0 {
				t.Fatalf("invalid technical metadata: %+v %+v", movie, streams)
			}
		})
	}
}
