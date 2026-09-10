//go:build externalbin

package movie

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

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

func TestMovieScanCancelsRunningSubprocess(t *testing.T) {
	fixture := setupMovieScanner(t)
	defer fixture.db.Close()
	s := fixture.scanner
	root := createMovieLibrary(t, 1)
	control := t.TempDir()
	marker := filepath.Join(control, "started")
	binary := filepath.Join(control, "ffprobe")
	script := "#!/bin/sh\nif [ \"$1\" = -version ]; then echo test-probe; exit 0; fi\ntouch '" + marker + "'\nexec sleep 60\n"
	err := os.WriteFile(binary, []byte(script), 0700)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("IGLOO_FFPROBE_PATH", binary)
	probe, err := ffprobe.New()
	if err != nil {
		t.Fatal(err)
	}
	defer ffprobe.Cleanup()
	s.ffprobe = probe
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.scanContext = ctx
	done := make(chan struct{})
	go func() { s.runMovieScan(root); close(done) }()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	deadline := time.After(5 * time.Second)
	for {
		_, err := os.Stat(marker)
		if err == nil {
			break
		}
		select {
		case <-ticker.C:
		case <-deadline:
			cancel()
			awaitScanSignal(t, done)
			t.Fatal("probe subprocess never started")
		}
	}
	cancel()
	awaitScanSignal(t, done)
	if s.Status().State != "canceled" || s.Status().Imported != 0 {
		t.Fatalf("subprocess cancellation: %+v", s.Status())
	}
}
