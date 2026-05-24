package mediabin

import (
	"fmt"
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

func ExtractEmbedded(binaryName string, payload []byte) (string, string, error) {
	if len(payload) == 0 {
		return "", "", fmt.Errorf("%s binary is missing: embedded payload is empty (binary was not included at compile time)", binaryName)
	}

	tempDir, err := os.MkdirTemp("", "igloo-"+binaryName+"-*")
	if err != nil {
		return "", "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	binPath := filepath.Join(tempDir, binaryName)
	if err := os.WriteFile(binPath, payload, 0755); err != nil {
		os.RemoveAll(tempDir)
		return "", "", fmt.Errorf("failed to write %s binary: %w", binaryName, err)
	}

	return binPath, tempDir, nil
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
