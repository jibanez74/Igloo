package mediabin

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/klauspost/compress/zstd"
)

func zstdCompress(t *testing.T, payload []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w, err := zstd.NewWriter(&buf)
	if err != nil {
		t.Fatalf("failed to create zstd writer: %v", err)
	}
	_, err = w.Write(payload)
	if err != nil {
		t.Fatalf("failed to compress payload: %v", err)
	}
	err = w.Close()
	if err != nil {
		t.Fatalf("failed to close zstd writer: %v", err)
	}
	return buf.Bytes()
}

func TestResolveExternal(t *testing.T) {
	const envVar = "IGLOO_TEST_MEDIABIN_PATH"

	t.Run("prefers the trimmed override", func(t *testing.T) {
		t.Setenv(envVar, "  /opt/media/fakebin  ")
		t.Setenv("PATH", t.TempDir())

		path, err := ResolveExternal("fakebin", envVar)
		if err != nil || path != "/opt/media/fakebin" {
			t.Fatalf("ResolveExternal = %q, %v; want the trimmed override", path, err)
		}
	})

	t.Run("falls back to PATH when the override is blank", func(t *testing.T) {
		dir := t.TempDir()
		binary := filepath.Join(dir, "fakebin")
		err := os.WriteFile(binary, []byte("#!/bin/sh\n"), 0o755)
		if err != nil {
			t.Fatalf("write fake binary: %v", err)
		}
		t.Setenv(envVar, "   ")
		t.Setenv("PATH", dir)

		path, err := ResolveExternal("fakebin", envVar)
		if err != nil || path != binary {
			t.Fatalf("ResolveExternal = %q, %v; want %q from PATH", path, err, binary)
		}
	})

	t.Run("names the override when the binary is missing", func(t *testing.T) {
		t.Setenv(envVar, "")
		t.Setenv("PATH", t.TempDir())

		_, err := ResolveExternal("fakebin", envVar)
		if err == nil || !strings.Contains(err.Error(), "fakebin binary not found") || !strings.Contains(err.Error(), envVar) {
			t.Fatalf("error = %v, want a not-found error naming %s", err, envVar)
		}
	})
}

func TestExtractEmbeddedZstdUsesCacheAcrossCalls(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	payload := []byte("fake-binary-payload")
	compressed := zstdCompress(t, payload)

	firstPath, firstDir, err := ExtractEmbeddedZstd("fakebin", compressed)
	if err != nil {
		t.Fatalf("first extract failed: %v", err)
	}
	if firstDir != "" {
		t.Fatalf("cached extraction should return an empty cleanup dir, got %q", firstDir)
	}

	content, err := os.ReadFile(firstPath)
	if err != nil {
		t.Fatalf("extracted binary missing: %v", err)
	}
	if string(content) != string(payload) {
		t.Fatal("extracted binary does not match the decompressed payload")
	}

	info, err := os.Stat(firstPath)
	if err != nil {
		t.Fatalf("failed to stat extracted binary: %v", err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("expected mode 0755, got %v", info.Mode().Perm())
	}

	firstInfo := info

	secondPath, secondDir, err := ExtractEmbeddedZstd("fakebin", compressed)
	if err != nil {
		t.Fatalf("second extract failed: %v", err)
	}
	if secondPath != firstPath {
		t.Fatalf("expected cache reuse of %q, got %q", firstPath, secondPath)
	}
	if secondDir != "" {
		t.Fatalf("cached extraction should return an empty cleanup dir, got %q", secondDir)
	}

	info, err = os.Stat(secondPath)
	if err != nil {
		t.Fatalf("cached binary missing after reuse: %v", err)
	}
	// A rewrite goes through a temp file and a rename, so it shows up as a
	// different inode even when the content and timestamps agree.
	if !os.SameFile(firstInfo, info) {
		t.Fatal("cached binary was rewritten on reuse")
	}

	// Shutdown hands the empty cleanup dir back; the cached binary must
	// survive it for the next boot.
	err = CleanupExtracted("fakebin", secondDir)
	if err != nil {
		t.Fatalf("cleanup of a cached extraction failed: %v", err)
	}
	_, err = os.Stat(secondPath)
	if err != nil {
		t.Fatalf("cached binary did not survive cleanup: %v", err)
	}
}

func TestExtractEmbeddedZstdRewritesCorruptedCacheEntry(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	payload := []byte("fake-binary-payload")
	compressed := zstdCompress(t, payload)

	binPath, _, err := ExtractEmbeddedZstd("fakebin", compressed)
	if err != nil {
		t.Fatalf("extract failed: %v", err)
	}

	err = os.WriteFile(binPath, []byte("tampered"), 0o755)
	if err != nil {
		t.Fatalf("failed to corrupt cache entry: %v", err)
	}

	binPath, _, err = ExtractEmbeddedZstd("fakebin", compressed)
	if err != nil {
		t.Fatalf("re-extract failed: %v", err)
	}

	restored, err := os.ReadFile(binPath)
	if err != nil {
		t.Fatalf("failed to read restored binary: %v", err)
	}
	if string(restored) != string(payload) {
		t.Fatal("corrupted cache entry was not rewritten from the payload")
	}
}

// An empty marker holds no digest, and hashing a missing binary used to yield
// an empty string too — so the cache check compared "" to "" and handed back a
// path with no binary at it.
func TestExtractEmbeddedZstdRewritesEntryWithEmptyMarker(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	payload := []byte("fake-binary-payload")
	compressed := zstdCompress(t, payload)

	binPath, _, err := ExtractEmbeddedZstd("fakebin", compressed)
	if err != nil {
		t.Fatalf("extract failed: %v", err)
	}

	err = os.WriteFile(binPath+".sha256", []byte("  \n"), 0o644)
	if err != nil {
		t.Fatalf("failed to truncate cache marker: %v", err)
	}
	err = os.Remove(binPath)
	if err != nil {
		t.Fatalf("failed to remove cached binary: %v", err)
	}

	binPath, _, err = ExtractEmbeddedZstd("fakebin", compressed)
	if err != nil {
		t.Fatalf("re-extract failed: %v", err)
	}

	restored, err := os.ReadFile(binPath)
	if err != nil {
		t.Fatalf("failed to read restored binary: %v", err)
	}
	if string(restored) != string(payload) {
		t.Fatal("empty marker was accepted as a cache hit instead of re-extracting")
	}
}

func TestExtractEmbeddedZstdPrunesOldVersions(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	oldPath, _, err := ExtractEmbeddedZstd("fakebin", zstdCompress(t, []byte("version-one")))
	if err != nil {
		t.Fatalf("extract of old version failed: %v", err)
	}

	newPath, _, err := ExtractEmbeddedZstd("fakebin", zstdCompress(t, []byte("version-two")))
	if err != nil {
		t.Fatalf("extract of new version failed: %v", err)
	}

	_, err = os.Stat(filepath.Dir(oldPath))
	if !os.IsNotExist(err) {
		t.Fatalf("expected old version dir to be pruned, stat err: %v", err)
	}
	_, err = os.Stat(newPath)
	if err != nil {
		t.Fatalf("new version missing after prune: %v", err)
	}
}

// ffmpeg and ffprobe share one cache root, so pruning one binary's old
// releases must leave the other binary's entries alone.
func TestExtractEmbeddedZstdPruneKeepsOtherBinaries(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	otherPath, _, err := ExtractEmbeddedZstd("otherbin", zstdCompress(t, []byte("other-version-one")))
	if err != nil {
		t.Fatalf("extract of the other binary failed: %v", err)
	}

	oldPath, _, err := ExtractEmbeddedZstd("fakebin", zstdCompress(t, []byte("version-one")))
	if err != nil {
		t.Fatalf("extract of old version failed: %v", err)
	}

	_, _, err = ExtractEmbeddedZstd("fakebin", zstdCompress(t, []byte("version-two")))
	if err != nil {
		t.Fatalf("extract of new version failed: %v", err)
	}

	_, err = os.Stat(filepath.Dir(oldPath))
	if !os.IsNotExist(err) {
		t.Fatalf("expected the old fakebin dir to be pruned, stat err: %v", err)
	}
	_, err = os.Stat(otherPath)
	if err != nil {
		t.Fatalf("the other binary's cache entry was pruned: %v", err)
	}
}

// Without a user cache directory the extraction must land in a randomized
// temp dir, never in a predictable path under the shared os.TempDir(): the
// cache path is derived only from the payload hash, so a shared-temp cache
// could be pre-seeded by a local attacker with a matching .sha256 marker and
// would then pass the digest check and be executed.
func TestExtractEmbeddedZstdFallsBackToTempDirWithoutCacheDir(t *testing.T) {
	tempRoot := t.TempDir()
	t.Setenv("TMPDIR", tempRoot)
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("HOME", "")

	_, err := os.UserCacheDir()
	if err == nil {
		t.Skip("this platform still resolves a user cache directory without HOME")
	}

	payload := []byte("fake-binary-payload")

	binPath, cleanupDir, err := ExtractEmbeddedZstd("fakebin", zstdCompress(t, payload))
	if err != nil {
		t.Fatalf("extract failed: %v", err)
	}
	if cleanupDir == "" {
		t.Fatal("temp-dir extraction must return a cleanup dir")
	}
	if filepath.Dir(binPath) != cleanupDir {
		t.Fatalf("expected %q inside the cleanup dir %q", binPath, cleanupDir)
	}

	content, err := os.ReadFile(binPath)
	if err != nil {
		t.Fatalf("extracted binary missing: %v", err)
	}
	if string(content) != string(payload) {
		t.Fatal("extracted binary does not match the decompressed payload")
	}

	_, err = os.Stat(filepath.Join(tempRoot, "igloo", "bin"))
	if !os.IsNotExist(err) {
		t.Fatalf("expected no shared-temp cache directory, stat err: %v", err)
	}

	err = CleanupExtracted("fakebin", cleanupDir)
	if err != nil {
		t.Fatalf("cleanup failed: %v", err)
	}
	_, err = os.Stat(cleanupDir)
	if !os.IsNotExist(err) {
		t.Fatalf("expected the temp extraction dir to be removed, stat err: %v", err)
	}
}

// A cache root that exists but cannot hold a directory (here a regular file)
// is the other way the cache path fails; it must fall back the same way.
func TestExtractEmbeddedZstdFallsBackToTempDirWhenCacheRootIsUnusable(t *testing.T) {
	tempRoot := t.TempDir()
	t.Setenv("TMPDIR", tempRoot)

	cacheRoot := filepath.Join(t.TempDir(), "notadir")
	err := os.WriteFile(cacheRoot, []byte("file"), 0o644)
	if err != nil {
		t.Fatalf("failed to create the blocking file: %v", err)
	}
	t.Setenv("XDG_CACHE_HOME", cacheRoot)

	payload := []byte("fake-binary-payload")

	binPath, cleanupDir, err := ExtractEmbeddedZstd("fakebin", zstdCompress(t, payload))
	if err != nil {
		t.Fatalf("extract failed: %v", err)
	}
	if cleanupDir == "" {
		t.Fatal("temp-dir extraction must return a cleanup dir")
	}
	if filepath.Dir(binPath) != cleanupDir || filepath.Dir(cleanupDir) != tempRoot {
		t.Fatalf("expected %q inside a cleanup dir under %q, got cleanup dir %q", binPath, tempRoot, cleanupDir)
	}

	content, err := os.ReadFile(binPath)
	if err != nil {
		t.Fatalf("extracted binary missing: %v", err)
	}
	if string(content) != string(payload) {
		t.Fatal("extracted binary does not match the decompressed payload")
	}

	err = CleanupExtracted("fakebin", cleanupDir)
	if err != nil {
		t.Fatalf("cleanup failed: %v", err)
	}
	_, err = os.Stat(cleanupDir)
	if !os.IsNotExist(err) {
		t.Fatalf("expected the temp extraction dir to be removed, stat err: %v", err)
	}
}

func TestExtractEmbeddedZstdRejectsEmptyPayload(t *testing.T) {
	_, _, err := ExtractEmbeddedZstd("fakebin", nil)
	if err == nil {
		t.Fatal("expected error for empty payload")
	}
}

// A payload that cannot be decompressed fails on the cache path and then on
// the temp-dir fallback; neither attempt may leave files behind.
func TestExtractEmbeddedZstdRejectsGarbagePayload(t *testing.T) {
	cacheRoot := t.TempDir()
	tempRoot := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheRoot)
	t.Setenv("TMPDIR", tempRoot)

	_, _, err := ExtractEmbeddedZstd("fakebin", []byte("not-zstd-data"))
	if err == nil {
		t.Fatal("expected error for non-zstd payload")
	}

	assertNoFilesUnder(t, cacheRoot)
	assertNoFilesUnder(t, tempRoot)
}

func TestWriteFileAtomicLeavesNothingBehindOnFailure(t *testing.T) {
	t.Run("missing parent directory", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "missing")

		err := writeFileAtomic(filepath.Join(dir, "marker"), []byte("digest"))
		if err == nil {
			t.Fatal("expected an error writing under a missing directory")
		}
		_, err = os.Stat(dir)
		if !os.IsNotExist(err) {
			t.Fatalf("expected no directory to be created, stat err: %v", err)
		}
	})

	t.Run("target path is a directory", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "marker")
		err := os.Mkdir(target, 0o755)
		if err != nil {
			t.Fatalf("failed to create the blocking directory: %v", err)
		}

		err = writeFileAtomic(target, []byte("digest"))
		if err == nil {
			t.Fatal("expected an error renaming onto a directory")
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read dir: %v", err)
		}
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), "marker-") {
				t.Fatalf("temp file %q was left behind after the failed rename", entry.Name())
			}
		}
	})
}

// assertNoFilesUnder fails when any regular file exists below root; empty
// directories are fine because the cache layout is created before the payload
// is decompressed.
func assertNoFilesUnder(t *testing.T, root string) {
	t.Helper()

	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			t.Errorf("file %q was left behind under %q", path, root)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
}
