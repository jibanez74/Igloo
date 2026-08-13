package mediabin

import (
	"bytes"
	"os"
	"path/filepath"
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

	firstModTime := info.ModTime()

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
	if !info.ModTime().Equal(firstModTime) {
		t.Fatal("cached binary was rewritten on reuse")
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
}

func TestExtractEmbeddedZstdRejectsEmptyPayload(t *testing.T) {
	_, _, err := ExtractEmbeddedZstd("fakebin", nil)
	if err == nil {
		t.Fatal("expected error for empty payload")
	}
}

func TestExtractEmbeddedZstdRejectsGarbagePayload(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	_, _, err := ExtractEmbeddedZstd("fakebin", []byte("not-zstd-data"))
	if err == nil {
		t.Fatal("expected error for non-zstd payload")
	}
}
