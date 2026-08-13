package mediabin

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func ResolveExternal(binaryName, envVar string) (string, error) {
	if path := strings.TrimSpace(os.Getenv(envVar)); path != "" {
		return path, nil
	}

	path, err := exec.LookPath(binaryName)
	if err != nil {
		return "", fmt.Errorf("%s binary not found on PATH; set %s or build without the externalbin tag to use embedded release payloads: %w", binaryName, envVar, err)
	}
	return path, nil
}

// ExtractEmbedded materializes an embedded binary on disk and returns its
// path plus a cleanup directory for CleanupExtracted. It reuses a per-version
// cache so the payload is written once per release instead of on every boot;
// cached binaries return an empty cleanup directory so shutdown leaves them
// in place. Falls back to a fresh temp-dir extraction when no cache
// directory is available.
func ExtractEmbedded(binaryName string, payload []byte) (string, string, error) {
	if len(payload) == 0 {
		return "", "", fmt.Errorf("%s binary is missing: embedded payload is empty (binary was not included at compile time)", binaryName)
	}

	binPath, err := extractToCache(binaryName, payload)
	if err == nil {
		return binPath, "", nil
	}

	tempDir, err := os.MkdirTemp("", "igloo-"+binaryName+"-*")
	if err != nil {
		return "", "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	binPath = filepath.Join(tempDir, binaryName)
	if err := os.WriteFile(binPath, payload, 0755); err != nil {
		os.RemoveAll(tempDir)
		return "", "", fmt.Errorf("failed to write %s binary: %w", binaryName, err)
	}

	return binPath, tempDir, nil
}

// extractToCache writes the payload to a hash-keyed cache path, reusing an
// existing file when its content hash matches. The hash check means a
// corrupted or tampered cache entry is silently rewritten rather than
// executed, and the write-to-temp-then-rename keeps the final path atomic.
func extractToCache(binaryName string, payload []byte) (string, error) {
	cacheRoot, err := os.UserCacheDir()
	if err != nil {
		cacheRoot = os.TempDir()
	}

	sum := sha256.Sum256(payload)
	digest := hex.EncodeToString(sum[:])
	dir := filepath.Join(cacheRoot, "igloo", "bin", binaryName+"-"+digest[:16])
	binPath := filepath.Join(dir, binaryName)

	if cachedFileMatches(binPath, sum) {
		return binPath, nil
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("failed to create cache directory: %w", err)
	}

	tmp, err := os.CreateTemp(dir, binaryName+"-*")
	if err != nil {
		return "", fmt.Errorf("failed to create cache temp file: %w", err)
	}
	tmpPath := tmp.Name()

	_, writeErr := tmp.Write(payload)
	chmodErr := tmp.Chmod(0o755)
	closeErr := tmp.Close()
	for _, err := range []error{writeErr, chmodErr, closeErr} {
		if err != nil {
			os.Remove(tmpPath)
			return "", fmt.Errorf("failed to write cached %s binary: %w", binaryName, err)
		}
	}

	if err := os.Rename(tmpPath, binPath); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("failed to move cached %s binary in place: %w", binaryName, err)
	}

	pruneStaleCacheEntries(filepath.Dir(dir), binaryName, filepath.Base(dir))

	return binPath, nil
}

// cachedFileMatches streams the cached file through sha256 and compares it
// to the embedded payload's hash.
func cachedFileMatches(binPath string, sum [sha256.Size]byte) bool {
	f, err := os.Open(binPath)
	if err != nil {
		return false
	}
	defer f.Close()

	h := sha256.New()
	_, err = io.Copy(h, f)
	if err != nil {
		return false
	}
	return [sha256.Size]byte(h.Sum(nil)) == sum
}

// pruneStaleCacheEntries removes cached extractions of older releases of the
// same binary so upgrades do not accumulate 100MB+ payloads.
func pruneStaleCacheEntries(binRoot, binaryName, keep string) {
	entries, err := os.ReadDir(binRoot)
	if err != nil {
		return
	}
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() || name == keep || !strings.HasPrefix(name, binaryName+"-") {
			continue
		}
		os.RemoveAll(filepath.Join(binRoot, name))
	}
}

func CleanupExtracted(binaryName, dir string) error {
	if dir == "" {
		return nil
	}
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("failed to cleanup %s: %w", binaryName, err)
	}
	return nil
}
