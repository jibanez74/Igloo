package scanner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"sync"
	"testing"
)

func TestNormalizedScanCacheKey(t *testing.T) {
	tests := []struct {
		name     string
		parts    []string
		expected string
	}{
		{"single lowercases and trims", []string{"  Foo "}, "foo"},
		{"multiple joined with NUL", []string{"Foo", "Bar"}, "foo\x00bar"},
		{"order matters", []string{"Bar", "Foo"}, "bar\x00foo"},
		{"empty parts", []string{"", ""}, "\x00"},
		{"no parts", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizedScanCacheKey(tt.parts...); got != tt.expected {
				t.Errorf("NormalizedScanCacheKey(%q) = %q, want %q", tt.parts, got, tt.expected)
			}
		})
	}
}

func TestScanIndexUnchanged(t *testing.T) {
	index := map[string]int64{
		filepath.Clean("/movies/a.mkv"): 100,
	}

	tests := []struct {
		name string
		path string
		size int64
		want bool
	}{
		{"present and same size", "/movies/a.mkv", 100, true},
		{"present, unclean path still matches", "/movies/./a.mkv", 100, true},
		{"present but different size", "/movies/a.mkv", 200, false},
		{"absent", "/movies/b.mkv", 100, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ScanIndexUnchanged(index, tt.path, tt.size); got != tt.want {
				t.Errorf("ScanIndexUnchanged(%q, %d) = %v, want %v", tt.path, tt.size, got, tt.want)
			}
		})
	}
}

func TestBuildScanIndex(t *testing.T) {
	type row struct {
		path string
		size int64
	}
	rows := []row{
		{"/music/./a.mp3", 1},
		{"/music/b.mp3", 2},
	}

	index := BuildScanIndex(rows, func(r row) (string, int64) {
		return r.path, r.size
	})

	if len(index) != 2 {
		t.Fatalf("index len = %d, want 2", len(index))
	}
	// Keys are cleaned.
	if got, ok := index[filepath.Clean("/music/a.mp3")]; !ok || got != 1 {
		t.Errorf("cleaned key /music/a.mp3 = (%d, %v), want (1, true)", got, ok)
	}
	if got := index["/music/b.mp3"]; got != 2 {
		t.Errorf("index[/music/b.mp3] = %d, want 2", got)
	}
}

func TestScanGuard(t *testing.T) {
	var g ScanGuard

	if !g.TryBegin() {
		t.Fatal("first TryBegin should succeed")
	}
	if g.TryBegin() {
		t.Fatal("second TryBegin should fail while running")
	}

	g.Finish()
	if !g.TryBegin() {
		t.Fatal("TryBegin should succeed after Finish")
	}
	g.Finish()
}

func TestScanGuardSingleFlight(t *testing.T) {
	var g ScanGuard
	var wins int64
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if g.TryBegin() {
				mu.Lock()
				wins++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if wins != 1 {
		t.Fatalf("expected exactly one TryBegin winner, got %d", wins)
	}
}

func TestWalkMediaLibrary(t *testing.T) {
	root := t.TempDir()

	// valid files
	mustWrite(t, filepath.Join(root, "movie.mkv"), "a")
	mustWrite(t, filepath.Join(root, "nested", "clip.mp4"), "bb")
	// filtered out by extension
	mustWrite(t, filepath.Join(root, "notes.txt"), "ccc")

	validExts := map[string]bool{"mkv": true, "mp4": true}

	var got []ScanFile
	var walkErrors int
	err := WalkMediaLibraryContext(context.Background(), root, validExts,
		func(error) { walkErrors++ },
		func(f ScanFile) error {
			got = append(got, f)
			return nil
		},
	)
	if err != nil {
		t.Fatalf("WalkMediaLibraryContext returned error: %v", err)
	}
	if walkErrors != 0 {
		t.Fatalf("unexpected walk errors: %d", walkErrors)
	}

	sort.Slice(got, func(i, j int) bool { return got[i].Path < got[j].Path })
	want := []ScanFile{
		{Path: filepath.Join(root, "movie.mkv"), Ext: "mkv", Size: 1},
		{Path: filepath.Join(root, "nested", "clip.mp4"), Ext: "mp4", Size: 2},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("files = %+v, want %+v", got, want)
	}
}

func TestWalkMediaLibraryContextStopsWhenCanceled(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "movie.mkv")
	if err := os.WriteFile(path, []byte("movie"), 0o600); err != nil {
		t.Fatalf("write media file: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	called := false
	err := WalkMediaLibraryContext(ctx, root, map[string]bool{"mkv": true}, func(error) {}, func(ScanFile) error {
		called = true
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("WalkMediaLibraryContext error = %v, want context canceled", err)
	}
	if called {
		t.Fatal("expected no files to be processed after cancellation")
	}
}

func TestWalkMediaLibraryMissingRoot(t *testing.T) {
	err := WalkMediaLibraryContext(context.Background(), filepath.Join(t.TempDir(), "does-not-exist"),
		map[string]bool{"mkv": true},
		func(error) {},
		func(ScanFile) error { return nil },
	)
	if err == nil {
		t.Fatal("expected an error walking a missing root")
	}
}

func TestWalkMediaLibraryPropagatesOnFileError(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "a.mkv"), "x")
	mustWrite(t, filepath.Join(root, "b.mkv"), "y")

	sentinel := errors.New("stop")
	count := 0
	err := WalkMediaLibraryContext(context.Background(), root, map[string]bool{"mkv": true},
		func(error) {},
		func(ScanFile) error {
			count++
			return sentinel
		},
	)
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel error, got %v", err)
	}
	if count != 1 {
		t.Fatalf("expected walk to stop after first onFile error, got %d calls", count)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
