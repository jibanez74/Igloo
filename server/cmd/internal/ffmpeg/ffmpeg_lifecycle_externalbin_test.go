//go:build externalbin

package ffmpeg

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func prepareSingletonTest(t *testing.T) {
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

func versionOnlyFakeFFmpeg(t *testing.T, logPath string) string {
	t.Helper()
	body := ""
	if logPath != "" {
		body += appendInvocationLog(logPath)
	}
	body += `
if [ "$#" -eq 1 ] && [ "$1" = "-version" ]; then
  printf '%s\n' 'ffmpeg version test'
fi
exit 0
`
	return writeFakeFFmpeg(t, "fake ffmpeg", body)
}

func TestResolveBinaryCandidateUsesConfiguredExternalBinary(t *testing.T) {
	prepareSingletonTest(t)
	script := versionOnlyFakeFFmpeg(t, "")
	t.Setenv("IGLOO_FFMPEG_PATH", "  "+script+"  ")

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
	prepareSingletonTest(t)
	t.Setenv("IGLOO_FFMPEG_PATH", "")
	t.Setenv("PATH", t.TempDir())

	_, err := resolveBinaryCandidate()
	if err == nil {
		t.Fatal("expected missing FFmpeg error")
	}
	if !strings.Contains(err.Error(), "IGLOO_FFMPEG_PATH") {
		t.Fatalf("error = %q, want environment-variable guidance", err.Error())
	}
}

func TestNewVerifiesAndReusesSingleton(t *testing.T) {
	prepareSingletonTest(t)
	logPath := filepath.Join(t.TempDir(), "calls.log")
	script := versionOnlyFakeFFmpeg(t, logPath)
	t.Setenv("IGLOO_FFMPEG_PATH", script)

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

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read call log: %v", err)
	}
	versionCalls := 0
	for _, line := range strings.Split(strings.TrimSpace(string(logData)), "\n") {
		if line == "-version" {
			versionCalls++
		}
	}
	if versionCalls != 1 {
		t.Fatalf("version calls = %d, want 1; log: %s", versionCalls, logData)
	}
}

func TestNewConcurrentCallsShareSingleton(t *testing.T) {
	prepareSingletonTest(t)
	script := versionOnlyFakeFFmpeg(t, "")
	t.Setenv("IGLOO_FFMPEG_PATH", script)

	const callers = 16
	results := make([]FFmpegInterface, callers)
	errors := make([]error, callers)
	var wg sync.WaitGroup
	wg.Add(callers)
	for i := 0; i < callers; i++ {
		index := i
		go func() {
			defer wg.Done()
			results[index], errors[index] = New()
		}()
	}
	wg.Wait()

	for i := 0; i < callers; i++ {
		if errors[i] != nil {
			t.Fatalf("New call %d: %v", i, errors[i])
		}
		if results[i] != results[0] {
			t.Fatalf("New call %d returned a different singleton", i)
		}
	}
}

func TestCleanupResetsSingletonAndRemovesOwnedDirectory(t *testing.T) {
	prepareSingletonTest(t)
	dir := t.TempDir()
	ownedDir := filepath.Join(dir, "extracted")
	err := os.Mkdir(ownedDir, 0755)
	if err != nil {
		t.Fatalf("mkdir extracted dir: %v", err)
	}

	instance = &ffmpeg{bin: "/unused"}
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
}

func TestCleanupAllowsFreshInitialization(t *testing.T) {
	prepareSingletonTest(t)
	firstScript := versionOnlyFakeFFmpeg(t, "")
	t.Setenv("IGLOO_FFMPEG_PATH", firstScript)
	first, err := New()
	if err != nil {
		t.Fatalf("first New: %v", err)
	}
	err = Cleanup()
	if err != nil {
		t.Fatalf("Cleanup: %v", err)
	}

	secondScript := versionOnlyFakeFFmpeg(t, "")
	t.Setenv("IGLOO_FFMPEG_PATH", secondScript)
	second, err := New()
	if err != nil {
		t.Fatalf("second New: %v", err)
	}
	if first == second {
		t.Fatal("Cleanup did not permit a fresh singleton")
	}
	secondImpl := second.(*ffmpeg)
	if secondImpl.bin != secondScript {
		t.Fatalf("fresh singleton binary = %q, want %q", secondImpl.bin, secondScript)
	}
}

func TestNewRejectsInvalidExecutableAndCanRetry(t *testing.T) {
	prepareSingletonTest(t)
	badPath := filepath.Join(t.TempDir(), "ffmpeg")
	err := os.WriteFile(badPath, []byte("not executable\n"), 0644)
	if err != nil {
		t.Fatalf("write invalid executable: %v", err)
	}
	t.Setenv("IGLOO_FFMPEG_PATH", badPath)

	_, err = New()
	if err == nil {
		t.Fatal("expected invalid executable error")
	}
	if instance != nil || extractedDir != "" {
		t.Fatalf("failed initialization claimed singleton state: instance=%v extractedDir=%q", instance, extractedDir)
	}

	goodPath := versionOnlyFakeFFmpeg(t, "")
	t.Setenv("IGLOO_FFMPEG_PATH", goodPath)
	retried, err := New()
	if err != nil {
		t.Fatalf("retry New: %v", err)
	}
	if retried.(*ffmpeg).bin != goodPath {
		t.Fatalf("retry binary = %q, want %q", retried.(*ffmpeg).bin, goodPath)
	}
}

func TestInitializeCandidateCleansFailedExtraction(t *testing.T) {
	prepareSingletonTest(t)
	extracted := filepath.Join(t.TempDir(), "igloo-ffmpeg-test")
	err := os.Mkdir(extracted, 0755)
	if err != nil {
		t.Fatalf("mkdir extraction: %v", err)
	}
	badBinary := filepath.Join(extracted, "ffmpeg")
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

func TestCapabilitiesReturnsIndependentSnapshot(t *testing.T) {
	original := Capabilities{
		Probed:         true,
		Encoders:       map[string]bool{"libx264": true},
		Filters:        map[string]bool{"scale": true},
		HWAccels:       map[string]bool{"cuda": true},
		CLIOptions:     map[string]bool{"readrate": true},
		FilterOptions:  map[string]map[string]bool{"scale_cuda": {"format": true}},
		EncoderOptions: map[string]map[string]bool{"h264_qsv": {"preset": true}},
	}
	f := &ffmpeg{capabilities: original}

	first := f.Capabilities()
	delete(first.Encoders, "libx264")
	delete(first.Filters, "scale")
	delete(first.HWAccels, "cuda")
	delete(first.CLIOptions, "readrate")
	delete(first.FilterOptions["scale_cuda"], "format")
	delete(first.EncoderOptions["h264_qsv"], "preset")

	second := f.Capabilities()
	if !second.Encoders["libx264"] || !second.Filters["scale"] || !second.HWAccels["cuda"] {
		t.Fatalf("top-level maps were mutated through snapshot: %#v", second)
	}
	if !second.CLIOptions["readrate"] || !second.FilterOptions["scale_cuda"]["format"] {
		t.Fatalf("filter/CLI maps were mutated through snapshot: %#v", second)
	}
	if !second.EncoderOptions["h264_qsv"]["preset"] {
		t.Fatalf("nested encoder map was mutated through snapshot: %#v", second)
	}

	// An unprobed instance must not gain empty maps through cloning.
	empty := (&ffmpeg{}).Capabilities()
	if empty.Encoders != nil || empty.FilterOptions != nil || empty.EncoderOptions != nil {
		t.Fatalf("nil maps changed while cloning: %#v", empty)
	}
}
