//go:build externalbin

package ffprobe

import (
	"os"
	"path/filepath"
	"strings"
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

	goodPath := writeFakeFFprobe(t, fakeFFprobeSpec{stdout: "ffprobe version test"})
	t.Setenv("IGLOO_FFPROBE_PATH", goodPath)
	retried, err := New()
	if err != nil {
		t.Fatalf("retry New: %v", err)
	}
	if retried.(*ffprobe).bin != goodPath {
		t.Fatalf("retry binary = %q, want %q", retried.(*ffprobe).bin, goodPath)
	}
}

func TestResolveBinaryCandidateUsesConfiguredExternalBinary(t *testing.T) {
	prepareSingletonLifecycleTest(t)
	script := writeFakeFFprobe(t, fakeFFprobeSpec{stdout: "ffprobe version test"})
	t.Setenv("IGLOO_FFPROBE_PATH", "  "+script+"  ")

	candidate, err := resolveBinaryCandidate()
	if err != nil {
		t.Fatalf("resolveBinaryCandidate: %v", err)
	}
	if candidate.path != script {
		t.Fatalf("candidate path = %q, want %q", candidate.path, script)
	}
	if candidate.extractedDir != "" {
		t.Fatalf("external candidate extractedDir = %q, want empty", candidate.extractedDir)
	}
}

func TestResolveBinaryCandidateReportsMissingExternalBinary(t *testing.T) {
	prepareSingletonLifecycleTest(t)
	t.Setenv("IGLOO_FFPROBE_PATH", "")
	t.Setenv("PATH", t.TempDir())

	_, err := resolveBinaryCandidate()
	if err == nil {
		t.Fatal("expected missing ffprobe error")
	}
	if !strings.Contains(err.Error(), "IGLOO_FFPROBE_PATH") {
		t.Fatalf("error = %q, want environment-variable guidance", err.Error())
	}
}

func countVersionCalls(t *testing.T, callLog string) int {
	t.Helper()
	data, err := os.ReadFile(callLog)
	if err != nil {
		t.Fatalf("read call log: %v", err)
	}
	calls := 0
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "-version" {
			calls++
		}
	}
	return calls
}

func TestNewVerifiesAndReusesSingleton(t *testing.T) {
	prepareSingletonLifecycleTest(t)
	callLog := filepath.Join(t.TempDir(), "calls.log")
	script := writeFakeFFprobe(t, fakeFFprobeSpec{stdout: "ffprobe version test", callLog: callLog})
	t.Setenv("IGLOO_FFPROBE_PATH", script)

	first, err := New()
	if err != nil {
		t.Fatalf("first New: %v", err)
	}
	second, err := New()
	if err != nil {
		t.Fatalf("second New: %v", err)
	}
	if first != second {
		t.Fatal("New did not reuse the singleton instance")
	}
	versionCalls := countVersionCalls(t, callLog)
	if versionCalls != 1 {
		t.Fatalf("version calls = %d, want 1", versionCalls)
	}
}

func TestCleanupResetsSingletonAndRemovesOwnedDirectory(t *testing.T) {
	prepareSingletonLifecycleTest(t)
	ownedDir := filepath.Join(t.TempDir(), "extracted")
	err := os.Mkdir(ownedDir, 0755)
	if err != nil {
		t.Fatalf("mkdir extracted dir: %v", err)
	}

	instance = &ffprobe{bin: "/unused"}
	extractedDir = ownedDir
	err = Cleanup()
	if err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	if instance != nil || extractedDir != "" {
		t.Fatalf("singleton state was not reset: instance=%v extractedDir=%q", instance, extractedDir)
	}
	_, err = os.Stat(ownedDir)
	if !os.IsNotExist(err) {
		t.Fatalf("owned extraction directory still exists: %v", err)
	}

	// With nothing initialized, Cleanup is a no-op.
	err = Cleanup()
	if err != nil {
		t.Fatalf("idle Cleanup: %v", err)
	}
}

func TestCleanupAllowsFreshInitialization(t *testing.T) {
	prepareSingletonLifecycleTest(t)
	firstScript := writeFakeFFprobe(t, fakeFFprobeSpec{stdout: "ffprobe version test"})
	t.Setenv("IGLOO_FFPROBE_PATH", firstScript)
	first, err := New()
	if err != nil {
		t.Fatalf("first New: %v", err)
	}
	err = Cleanup()
	if err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	if instance != nil {
		t.Fatal("Cleanup left the singleton in place")
	}

	callLog := filepath.Join(t.TempDir(), "calls.log")
	secondScript := writeFakeFFprobe(t, fakeFFprobeSpec{stdout: "ffprobe version test", callLog: callLog})
	t.Setenv("IGLOO_FFPROBE_PATH", secondScript)
	second, err := New()
	if err != nil {
		t.Fatalf("second New: %v", err)
	}
	if first == second {
		t.Fatal("Cleanup did not permit a fresh singleton")
	}
	if second.(*ffprobe).bin != secondScript {
		t.Fatalf("fresh singleton binary = %q, want %q", second.(*ffprobe).bin, secondScript)
	}
	versionCalls := countVersionCalls(t, callLog)
	if versionCalls != 1 {
		t.Fatalf("fresh singleton verified the binary %d times, want 1", versionCalls)
	}
}
