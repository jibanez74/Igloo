//go:build externalbin

package ffprobe

import (
	"os"
	"path/filepath"
	"testing"
)

func prepareSingletonLifecycleTest(t *testing.T) {
	t.Helper()
	err := Cleanup()
	if err != nil {
		t.Fatalf("initial Cleanup: %v", err)
	}
	t.Cleanup(func() {
		cleanupErr := Cleanup()
		if cleanupErr != nil {
			t.Errorf("final Cleanup: %v", cleanupErr)
		}
	})
}

func versionOnlyFakeFFprobe(t *testing.T) string {
	t.Helper()

	binPath := filepath.Join(t.TempDir(), "ffprobe")
	script := `#!/bin/sh
if [ "$#" -eq 1 ] && [ "$1" = "-version" ]; then
		printf '%s\n' 'ffprobe version test'
	fi
exit 0
`
	err := os.WriteFile(binPath, []byte(script), 0755)
	if err != nil {
		t.Fatalf("write fake ffprobe: %v", err)
	}

	return binPath
}

func TestInitializeCandidateCleansFailedExtraction(t *testing.T) {
	prepareSingletonLifecycleTest(t)
	extracted := filepath.Join(t.TempDir(), "igloo-ffprobe-test")
	err := os.Mkdir(extracted, 0755)
	if err != nil {
		t.Fatalf("mkdir extraction: %v", err)
	}
	badBinary := filepath.Join(extracted, "ffprobe")
	err = os.WriteFile(badBinary, []byte("#!/bin/sh\nexit 9\n"), 0755)
	if err != nil {
		t.Fatalf("write bad binary: %v", err)
	}

	_, err = initializeCandidate(binaryCandidate{path: badBinary, extractedDir: extracted})
	if err == nil {
		t.Fatal("expected candidate verification failure")
	}
	_, statErr := os.Stat(extracted)
	if !os.IsNotExist(statErr) {
		t.Fatalf("failed candidate extraction was not removed: %v", statErr)
	}
	if instance != nil || extractedDir != "" {
		t.Fatalf("failed candidate changed singleton ownership: instance=%v extractedDir=%q", instance, extractedDir)
	}
}

func TestNewRejectsInvalidExecutableAndCanRetry(t *testing.T) {
	prepareSingletonLifecycleTest(t)
	badPath := filepath.Join(t.TempDir(), "ffprobe")
	err := os.WriteFile(badPath, []byte("not executable\n"), 0644)
	if err != nil {
		t.Fatalf("write invalid executable: %v", err)
	}
	t.Setenv("IGLOO_FFPROBE_PATH", badPath)

	_, err = New()
	if err == nil {
		t.Fatal("expected invalid executable error")
	}
	if instance != nil || extractedDir != "" {
		t.Fatalf("failed initialization claimed singleton state: instance=%v extractedDir=%q", instance, extractedDir)
	}

	goodPath := versionOnlyFakeFFprobe(t)
	t.Setenv("IGLOO_FFPROBE_PATH", goodPath)
	retried, err := New()
	if err != nil {
		t.Fatalf("retry New: %v", err)
	}
	if retried.(*ffprobe).bin != goodPath {
		t.Fatalf("retry binary = %q, want %q", retried.(*ffprobe).bin, goodPath)
	}
}
