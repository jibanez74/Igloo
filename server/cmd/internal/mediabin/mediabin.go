// Package mediabin resolves the ffmpeg/ffprobe binaries for both build
// configurations. Exactly one half is live in any given build — ResolveExternal
// under the externalbin tag, ExtractEmbeddedZstd and its helpers without it —
// so a reachability analysis always reports the other half as unreachable. Run
// it under both tag sets before concluding anything here is dead.
package mediabin

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/klauspost/compress/zstd"
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

// ExtractEmbeddedZstd materializes a zstd-compressed embedded binary on disk
// and returns its path plus a cleanup directory for CleanupExtracted. It
// reuses a per-version cache so the payload is decompressed once per release
// instead of on every boot; cached binaries return an empty cleanup directory
// so shutdown leaves them in place. Falls back to a fresh temp-dir extraction
// when no cache directory is available.
func ExtractEmbeddedZstd(binaryName string, compressed []byte) (string, string, error) {
	if len(compressed) == 0 {
		return "", "", fmt.Errorf("%s binary is missing: embedded payload is empty (binary was not included at compile time)", binaryName)
	}

	binPath, err := extractToCache(binaryName, compressed)
	if err == nil {
		return binPath, "", nil
	}

	tempDir, err := os.MkdirTemp("", "igloo-"+binaryName+"-*")
	if err != nil {
		return "", "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	binPath = filepath.Join(tempDir, binaryName)
	err = decompressToFile(binPath, compressed, io.Discard)
	if err != nil {
		os.RemoveAll(tempDir)
		return "", "", fmt.Errorf("failed to write %s binary: %w", binaryName, err)
	}

	return binPath, tempDir, nil
}

// extractToCache decompresses the payload to a cache path keyed by the
// compressed payload's hash. A sidecar .sha256 marker records the hash of
// the decompressed binary; reuse requires the on-disk file to match it, so a
// corrupted or tampered cache entry is silently rewritten rather than
// executed. A truncated marker or an unreadable binary is likewise a miss —
// neither may satisfy the digest check by comparing empty strings.
// Write-to-temp-then-rename keeps the final paths atomic.
//
// A missing user cache directory is an error, never a fall back to
// os.TempDir(): the cache holds executables at a path derived only from the
// payload hash, so on a shared temp dir a local attacker could pre-create the
// binary and a matching marker and have it pass the digest check and run. The
// caller falls back to a randomized temp dir instead.
func extractToCache(binaryName string, compressed []byte) (string, error) {
	cacheRoot, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("no user cache directory available: %w", err)
	}

	key := sha256.Sum256(compressed)
	dir := filepath.Join(cacheRoot, "igloo", "bin", binaryName+"-"+hex.EncodeToString(key[:])[:16])
	binPath := filepath.Join(dir, binaryName)
	markerPath := binPath + ".sha256"

	marker, err := os.ReadFile(markerPath)
	if err == nil {
		want := strings.TrimSpace(string(marker))

		// A truncated marker holds no digest to check the binary against, so
		// treat it as a miss instead of comparing two empty strings.
		if want != "" {
			got, err := fileSHA256(binPath)
			if err == nil && got == want {
				return binPath, nil
			}
		}
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("failed to create cache directory: %w", err)
	}

	tmp, err := os.CreateTemp(dir, binaryName+"-*")
	if err != nil {
		return "", fmt.Errorf("failed to create cache temp file: %w", err)
	}
	tmpPath := tmp.Name()
	tmp.Close()

	digest := sha256.New()
	err = decompressToFile(tmpPath, compressed, digest)
	if err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("failed to write cached %s binary: %w", binaryName, err)
	}

	if err := os.Rename(tmpPath, binPath); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("failed to move cached %s binary in place: %w", binaryName, err)
	}

	err = writeFileAtomic(markerPath, []byte(hex.EncodeToString(digest.Sum(nil))))
	if err != nil {
		return "", fmt.Errorf("failed to write cache marker for %s: %w", binaryName, err)
	}

	pruneStaleCacheEntries(filepath.Dir(dir), binaryName, filepath.Base(dir))

	return binPath, nil
}

// decompressToFile streams the zstd payload into path (mode 0755), teeing
// the decompressed bytes into digest.
func decompressToFile(path string, compressed []byte, digest io.Writer) error {
	reader, err := zstd.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return fmt.Errorf("failed to open zstd payload: %w", err)
	}
	defer reader.Close()

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}

	_, copyErr := reader.WriteTo(io.MultiWriter(f, digest))
	// O_CREATE's mode is ignored when the file pre-exists (the cache path
	// hands us an os.CreateTemp file), so set it explicitly.
	chmodErr := f.Chmod(0o755)
	closeErr := f.Close()
	for _, err := range []error{copyErr, chmodErr, closeErr} {
		if err != nil {
			return err
		}
	}
	return nil
}

func writeFileAtomic(path string, content []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+"-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()

	_, writeErr := tmp.Write(content)
	closeErr := tmp.Close()
	for _, err := range []error{writeErr, closeErr} {
		if err != nil {
			os.Remove(tmpPath)
			return err
		}
	}

	err = os.Rename(tmpPath, path)
	if err != nil {
		os.Remove(tmpPath)
		return err
	}
	return nil
}

// fileSHA256 hashes the file at path. It reports the read error rather than an
// empty digest: an empty string would compare equal to an empty .sha256 marker
// and validate a binary that could not be read at all.
func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	_, err = io.Copy(h, f)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
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
