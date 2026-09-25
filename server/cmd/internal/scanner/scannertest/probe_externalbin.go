//go:build externalbin

package scannertest

import (
	"testing"

	"igloo/cmd/internal/ffprobe"
)

// RealProbe resolves the ffprobe binary the way the externalbin build does
// (IGLOO_FFPROBE_PATH, else PATH) and releases it when the test ends. Tests
// that need real container parsing share it instead of repeating the
// lifecycle.
func RealProbe(t testing.TB) ffprobe.FfprobeInterface {
	t.Helper()
	probe, err := ffprobe.New()
	if err != nil {
		t.Fatalf("ffprobe: %v", err)
	}
	t.Cleanup(func() {
		err := ffprobe.Cleanup()
		if err != nil {
			t.Errorf("ffprobe cleanup: %v", err)
		}
	})
	return probe
}
