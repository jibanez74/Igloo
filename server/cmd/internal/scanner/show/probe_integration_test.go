//go:build externalbin

package show

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/scanner"
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
	s, probe, _ := setupScanner(t)
	s.Now = time.Now
	s.Logger = &sampleLogger{t: t}
	real := realProbe(t)
	probe.Hook = func(ctx context.Context, path string) (*ffprobe.FfprobeResult, error) {
		t.Logf("real probe %d: %q", probe.Calls(), path)
		return real.GetMetadata(ctx, path)
	}
	scanOK(t, s, root)
	before := sampleCatalogSnapshot(t, s.DB)
	firstProbes := probe.Calls()
	t.Logf("first scan real probes: %d", firstProbes)
	rows, err := s.Queries.GetShowScanIndex(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 {
		t.Fatal("sample scan imported no files")
	}
	if firstProbes < len(rows) {
		t.Fatalf("imported %d files with only %d real probes", len(rows), firstProbes)
	}
	assertEveryDiscoveredFileReachedTheCatalog(t, s.Status(), len(rows))
	logShowDirectoryCounts(t, root, rows)
	for _, row := range rows {
		episodes := fileEpisodes(t, s.DB, row.ID)
		numbers := make([]int64, 0, len(episodes))
		for _, episode := range episodes {
			numbers = append(numbers, episode.episodeNumber)
		}
		t.Logf("catalog file: %q episodes: %v", row.FilePath, numbers)
	}
	scanOK(t, s, root)
	t.Logf("second scan additional real probes: %d", probe.Calls()-firstProbes)
	if probe.Calls() != firstProbes {
		t.Errorf("unchanged sample scan made %d additional probes", probe.Calls()-firstProbes)
	}
	after := sampleCatalogSnapshot(t, s.DB)
	for table, original := range before {
		unchanged := reflect.DeepEqual(original, after[table])
		if !unchanged {
			t.Errorf("unchanged scan changed catalog table %s", table)
		}
	}
}

// assertEveryDiscoveredFileReachedTheCatalog is the check the counters alone
// never make. Total is stamped during discovery and is never reduced, so a run
// that rejects, defers, or simply never dispatches part of its job list still
// finishes and still reports a run -- and a show whose every file went missing
// that way vanishes from the catalog with nothing to show for it. A walk error
// that skipped a whole subtree lands here too, as completed-with-issues.
func assertEveryDiscoveredFileReachedTheCatalog(t *testing.T, status Status, catalogFiles int) {
	t.Helper()
	if status.State != scanner.StateCompleted {
		t.Fatalf("scan state = %q, want %q (issues: %+v)", status.State, scanner.StateCompleted, status.Issues)
	}
	if status.Failed != 0 || status.Deferred != 0 {
		t.Fatalf("scan reported failed=%d deferred=%d, want 0 of each (issues: %+v)", status.Failed, status.Deferred, status.Issues)
	}
	if status.Total != catalogFiles {
		t.Fatalf("discovered %d files but only %d reached the catalog", status.Total, catalogFiles)
	}
}

// logShowDirectoryCounts prints one line per show directory so a short or
// missing show is legible without counting 300 catalog lines by hand.
func logShowDirectoryCounts(t *testing.T, root string, rows []database.GetShowScanIndexRow) {
	t.Helper()
	absolute, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	counts := make(map[string]int)
	for _, row := range rows {
		relative, err := filepath.Rel(absolute, row.FilePath)
		if err != nil {
			t.Fatal(err)
		}
		counts[strings.Split(relative, string(filepath.Separator))[0]]++
	}
	directories := make([]string, 0, len(counts))
	for directory := range counts {
		directories = append(directories, directory)
	}
	sort.Strings(directories)
	for _, directory := range directories {
		t.Logf("show directory %q: %d files", directory, counts[directory])
	}
}

type sampleLogger struct{ t *testing.T }

func (l *sampleLogger) Debug(msg string, args ...any) { l.t.Log("DEBUG", msg, args) }
func (l *sampleLogger) Info(msg string, args ...any)  { l.t.Log("INFO", msg, args) }
func (l *sampleLogger) Warn(msg string, args ...any)  { l.t.Log("WARN", msg, args) }
func (l *sampleLogger) Error(msg string, args ...any) { l.t.Log("ERROR", msg, args) }

// Capture every TV column, including row identities, ordered links, streams,
// chapters, fingerprints, timestamps, and pending enrichment. Sorting encoded
// rows avoids relying on SQLite's unspecified row order or a particular key.
func sampleCatalogSnapshot(t *testing.T, db *sql.DB) map[string][]string {
	t.Helper()
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type = 'table' AND (name = 'shows' OR name GLOB 'show_*') ORDER BY name")
	if err != nil {
		t.Fatal(err)
	}
	var tables []string
	for rows.Next() {
		var table string
		err = rows.Scan(&table)
		if err != nil {
			t.Fatal(err)
		}
		tables = append(tables, table)
	}
	err = rows.Err()
	if err != nil {
		t.Fatal(err)
	}
	err = rows.Close()
	if err != nil {
		t.Fatal(err)
	}
	snapshot := make(map[string][]string, len(tables))
	for _, table := range tables {
		rows, err := db.Query(`SELECT * FROM "` + strings.ReplaceAll(table, `"`, `""`) + `"`)
		if err != nil {
			t.Fatal(err)
		}
		columns, err := rows.Columns()
		if err != nil {
			t.Fatal(err)
		}
		var records []string
		for rows.Next() {
			values := make([]any, len(columns))
			pointers := make([]any, len(columns))
			for i := range values {
				pointers[i] = &values[i]
			}
			err = rows.Scan(pointers...)
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(values)
			if err != nil {
				t.Fatal(err)
			}
			records = append(records, string(encoded))
		}
		err = rows.Err()
		if err != nil {
			t.Fatal(err)
		}
		err = rows.Close()
		if err != nil {
			t.Fatal(err)
		}
		sort.Strings(records)
		snapshot[table] = records
		t.Logf("%s: %d rows", table, len(records))
	}
	return snapshot
}
