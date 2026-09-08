package scanner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"syscall"
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

func TestWalkMediaLibrarySymlinksAndSpecialFiles(t *testing.T) {
	for _, ext := range []string{"m4a", "mkv"} {
		t.Run(ext, func(t *testing.T) {
			root := t.TempDir()
			targets := t.TempDir()
			target := filepath.Join(targets, "target")
			err := os.WriteFile(target, []byte("target content with a different size than its link"), 0600)
			if err != nil {
				t.Fatal(err)
			}
			regular := filepath.Join(root, "regular."+ext)
			err = os.WriteFile(regular, []byte("regular"), 0600)
			if err != nil {
				t.Fatal(err)
			}
			link := filepath.Join(root, "linked."+ext)
			broken := filepath.Join(root, "broken."+ext)
			directory := filepath.Join(root, "directory."+ext)
			fifo := filepath.Join(root, "fifo."+ext)
			err = syscall.Mkfifo(fifo, 0600)
			if err != nil {
				t.Fatal(err)
			}
			for path, dest := range map[string]string{link: target, broken: filepath.Join(targets, "missing"), directory: targets, filepath.Join(root, "fifo-link."+ext): fifo} {
				err = os.Symlink(dest, path)
				if err != nil {
					t.Fatal(err)
				}
			}
			// A directory link must never expose descendants to the walker.
			err = os.WriteFile(filepath.Join(targets, "hidden."+ext), []byte("hidden"), 0600)
			if err != nil {
				t.Fatal(err)
			}
			var failures []error
			var files []ScanFile
			err = WalkMediaLibraryContext(context.Background(), root, map[string]bool{ext: true}, func(err error) { failures = append(failures, err) }, func(file ScanFile) error { files = append(files, file); return nil })
			if err != nil {
				t.Fatal(err)
			}
			expected := []ScanFile{{Path: link, Ext: ext}, {Path: regular, Ext: ext, Size: 7}}
			info, err := os.Stat(target)
			if err != nil {
				t.Fatal(err)
			}
			expected[0].Size = info.Size()
			if !reflect.DeepEqual(files, expected) {
				t.Fatalf("files=%+v, want %+v", files, expected)
			}
			if len(failures) != 1 || !strings.Contains(failures[0].Error(), broken) || !errors.Is(failures[0], os.ErrNotExist) {
				t.Fatalf("walk errors=%v", failures)
			}
		})
	}
}
