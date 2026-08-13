package mediabin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractEmbeddedUsesCacheAcrossCalls(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	payload := []byte("fake-binary-payload")

	firstPath, firstDir, err := ExtractEmbedded("fakebin", payload)
	if err != nil {
		t.Fatalf("first extract failed: %v", err)
	}
	if firstDir != "" {
		t.Fatalf("cached extraction should return an empty cleanup dir, got %q", firstDir)
	}

	info, err := os.Stat(firstPath)
	if err != nil {
		t.Fatalf("extracted binary missing: %v", err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("expected mode 0755, got %v", info.Mode().Perm())
	}

	firstModTime := info.ModTime()

	secondPath, secondDir, err := ExtractEmbedded("fakebin", payload)
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

func TestExtractEmbeddedRewritesCorruptedCacheEntry(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	payload := []byte("fake-binary-payload")

	binPath, _, err := ExtractEmbedded("fakebin", payload)
	if err != nil {
		t.Fatalf("extract failed: %v", err)
	}

	err = os.WriteFile(binPath, []byte("tampered"), 0o755)
	if err != nil {
		t.Fatalf("failed to corrupt cache entry: %v", err)
	}

	binPath, _, err = ExtractEmbedded("fakebin", payload)
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

func TestExtractEmbeddedPrunesOldVersions(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	oldPath, _, err := ExtractEmbedded("fakebin", []byte("version-one"))
	if err != nil {
		t.Fatalf("extract of old version failed: %v", err)
	}

	newPath, _, err := ExtractEmbedded("fakebin", []byte("version-two"))
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

func TestExtractEmbeddedRejectsEmptyPayload(t *testing.T) {
	_, _, err := ExtractEmbedded("fakebin", nil)
	if err == nil {
		t.Fatal("expected error for empty payload")
	}
}
