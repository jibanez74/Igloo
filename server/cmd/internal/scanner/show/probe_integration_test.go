//go:build externalbin

package show

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"igloo/cmd/internal/ffprobe"
)

func realProbe(t *testing.T) ffprobe.FfprobeInterface {
	t.Helper()
	p, err := ffprobe.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		err := ffprobe.Cleanup()
		if err != nil {
			t.Error(err)
		}
	})
	return p
}
func TestRealProbeTechnicalMetadataAndArtwork(t *testing.T) {
	probe := realProbe(t)
	for _, tc := range []struct {
		fixture, ext, codec string
		index               int
	}{
		{"moving", "avi", "mjpeg", 1}, {"artwork-with-video", "mp4", "h264", 0}, {"artwork-only", "mp4", "", 0},
	} {
		t.Run(tc.fixture, func(t *testing.T) {
			s, _, root := setupScanner(t)
			s.Ffprobe = probe
			data, err := os.ReadFile(filepath.Join("../movie/testdata", tc.fixture+"."+tc.ext))
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, root, "Show/Season 1/S01E01E02."+tc.ext, string(data))
			scanOK(t, s, root)
			if tc.codec == "" {
				if countRows(t, s.DB, "shows") != 0 {
					t.Fatal("artwork-only import")
				}
				return
			}
			var codec string
			var index, width, height int
			var duration float64
			err = s.DB.QueryRow("SELECT v.codec,v.stream_index,v.width,v.height,f.duration FROM show_video_streams v JOIN show_files f ON f.id=v.file_id").Scan(&codec, &index, &width, &height, &duration)
			if err != nil {
				t.Fatal(err)
			}
			if codec != tc.codec || index != tc.index || width != 64 || height != 48 || duration <= 0 || countRows(t, s.DB, "show_episodes") != 2 {
				t.Fatal("invalid persisted technical metadata", codec, index, width, height, duration)
			}
		})
	}
}

// Explicit opt-in: media files are opened read-only and catalog writes go only
// to the isolated in-memory test database. No TMDB credentials are used.
func TestSampleLibraryReadOnly(t *testing.T) {
	root := os.Getenv("IGLOO_TV_SAMPLE_DIR")
	if root == "" {
		t.Skip("IGLOO_TV_SAMPLE_DIR is not configured")
	}
	s, _, _ := setupScanner(t)
	s.Ffprobe = realProbe(t)
	scanOK(t, s, root)
	for _, table := range []string{"shows", "show_seasons", "show_episodes", "show_files", "show_episode_files"} {
		t.Logf("%s: %d", table, countRows(t, s.DB, table))
	}
	rows, err := s.Queries.GetShowScanIndex(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 {
		t.Fatal("sample scan imported no files")
	}
	s.Ffprobe = &testProbe{hook: func(context.Context, string) (*ffprobe.FfprobeResult, error) {
		t.Error("unchanged sample scan probed")
		return nil, nil
	}}
	scanOK(t, s, root)
}
