package ffmpeg

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"igloo/cmd/internal/ffmpeg/fmp4testutil"
	"igloo/cmd/internal/helpers"
)

func TestValidateRemuxSafetyAcceptsSafeFixtures(t *testing.T) {
	tests := []struct {
		name    string
		fixture fmp4testutil.Fixture
		wantErr bool
	}{
		{
			name:    "safe fragments",
			fixture: fmp4testutil.Fixture{SafeVideo: true, Segments: helpers.HLS_REMUX_PREVALIDATE_SEGMENTS},
		},
		{
			name:    "audio track noise is ignored",
			fixture: fmp4testutil.Fixture{SafeVideo: true, AudioNoise: true, Segments: helpers.HLS_REMUX_PREVALIDATE_SEGMENTS},
		},
		{
			name:    "unsafe sync sample metadata",
			fixture: fmp4testutil.Fixture{SafeVideo: false, Segments: helpers.HLS_REMUX_PREVALIDATE_SEGMENTS},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			err := fmp4testutil.WriteHLSFixture(dir, tt.fixture)
			if err != nil {
				t.Fatalf("WriteHLSFixture: %v", err)
			}

			summary, err := ValidateRemuxSafety(dir, helpers.HLS_REMUX_PREVALIDATE_SEGMENTS)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected unsafe remux validation error")
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateRemuxSafety returned error: %v", err)
			}
			if summary.CheckedSegments != helpers.HLS_REMUX_PREVALIDATE_SEGMENTS {
				t.Fatalf("CheckedSegments = %d, want %d", summary.CheckedSegments, helpers.HLS_REMUX_PREVALIDATE_SEGMENTS)
			}
			if summary.CheckedSyncSamples != helpers.HLS_REMUX_PREVALIDATE_SEGMENTS {
				t.Fatalf("CheckedSyncSamples = %d, want %d", summary.CheckedSyncSamples, helpers.HLS_REMUX_PREVALIDATE_SEGMENTS)
			}
		})
	}
}

func TestValidateRemuxSafetyRejectsInvalidSegmentCounts(t *testing.T) {
	for _, segmentCount := range []int{0, -1} {
		_, err := ValidateRemuxSafety(t.TempDir(), segmentCount)
		if err == nil {
			t.Fatalf("segment count %d did not return an error", segmentCount)
		}
		if !strings.Contains(err.Error(), "segmentCount must be positive") {
			t.Fatalf("error = %q, want segment count validation", err.Error())
		}
	}
}

func TestValidateRemuxSafetyRejectsMissingAndInvalidInitSegments(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		_, err := ValidateRemuxSafety(t.TempDir(), 1)
		if err == nil || !strings.Contains(err.Error(), "read init segment") {
			t.Fatalf("error = %v, want missing init failure", err)
		}
	})

	t.Run("invalid", func(t *testing.T) {
		dir := t.TempDir()
		err := os.WriteFile(filepath.Join(dir, helpers.HLS_INIT_FILENAME), []byte("invalid"), 0644)
		if err != nil {
			t.Fatalf("write invalid init: %v", err)
		}
		_, err = ValidateRemuxSafety(dir, 1)
		if err == nil {
			t.Fatal("expected invalid init failure")
		}
	})
}

func TestValidateRemuxSafetyRejectsMissingVideoConfiguration(t *testing.T) {
	dir := t.TempDir()
	initData := fmp4testutil.BuildInitMP4()
	moov, found, err := findDirectChildBox(initData, 0, len(initData), "moov")
	if err != nil || !found {
		t.Fatalf("find moov: found=%v err=%v", found, err)
	}
	traks, err := listDirectChildBoxes(initData, moov.PayloadStart, moov.End)
	if err != nil {
		t.Fatalf("list tracks: %v", err)
	}
	mutated := append([]byte(nil), initData...)
	foundConfig := false
	for _, trak := range traks {
		if trak.Type != "trak" {
			continue
		}
		avcC, configFound, findErr := findAVCConfigBox(initData, trak)
		if findErr == nil && configFound {
			copy(mutated[avcC.Start+4:avcC.Start+8], []byte("free"))
			foundConfig = true
			break
		}
	}
	if !foundConfig {
		t.Fatal("fixture did not contain avcC")
	}
	err = os.WriteFile(filepath.Join(dir, helpers.HLS_INIT_FILENAME), mutated, 0644)
	if err != nil {
		t.Fatalf("write init: %v", err)
	}

	_, err = ValidateRemuxSafety(dir, 1)
	if err == nil || !strings.Contains(err.Error(), "missing avcC") {
		t.Fatalf("error = %v, want missing avcC", err)
	}
}

// Each case corrupts one field of an otherwise valid single-segment fixture and
// checks both the reported error and how much of the summary survived.
func TestValidateRemuxSafetyRejectsCorruptFragments(t *testing.T) {
	tests := []struct {
		name            string
		mutate          func(t *testing.T, segment []byte)
		wantErr         string
		wantSegments    int
		wantSyncSamples int
	}{
		{
			name: "video track absent from the fragment",
			mutate: func(t *testing.T, segment []byte) {
				patchTrafFieldForTest(t, segment, "tfhd", tfhdTrackIDOffset, 99)
			},
			wantErr: "missing video traf",
		},
		{
			name: "sample size exceeds the segment",
			mutate: func(t *testing.T, segment []byte) {
				patchTrafFieldForTest(t, segment, "trun", trunFirstSampleSizeOffset, math.MaxUint32)
			},
			wantErr: "sample exceeds segment bounds",
		},
		{
			// Point the run at the moof interior: the bytes stay inside the
			// segment but outside any mdat payload.
			name: "sample lands outside mdat",
			mutate: func(t *testing.T, segment []byte) {
				patchTrafFieldForTest(t, segment, "trun", trunDataOffsetOffset, 8)
			},
			wantErr: "sample outside mdat payload",
		},
		{
			name: "no sync samples in the segment",
			mutate: func(t *testing.T, segment []byte) {
				patchTrafFieldForTest(t, segment, "trun", trunFirstSampleFlagsOffset, 0x00010000)
			},
			wantErr:      "no sync samples",
			wantSegments: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			segment := fmp4testutil.BuildSegment(fmp4testutil.BuildVideoSample(true), false)
			tt.mutate(t, segment)
			writeRemuxFixtureFiles(t, dir, fmp4testutil.BuildInitMP4(), segment)

			summary, err := ValidateRemuxSafety(dir, 1)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want %q", err, tt.wantErr)
			}
			if summary.CheckedSegments != tt.wantSegments {
				t.Fatalf("CheckedSegments = %d, want %d", summary.CheckedSegments, tt.wantSegments)
			}
			if summary.CheckedSyncSamples != tt.wantSyncSamples {
				t.Fatalf("CheckedSyncSamples = %d, want %d", summary.CheckedSyncSamples, tt.wantSyncSamples)
			}
		})
	}
}

// A failure partway through must still report the segments already validated.
func TestValidateRemuxSafetyReturnsPartialSummaryOnLaterSegmentFailure(t *testing.T) {
	validSegment := fmp4testutil.BuildSegment(fmp4testutil.BuildVideoSample(true), false)

	tests := []struct {
		name         string
		segments     [][]byte
		requestCount int
		wantErr      string
	}{
		{
			name:         "later segment is malformed",
			segments:     [][]byte{validSegment, []byte("not-a-valid-fragment")},
			requestCount: 2,
			wantErr:      "validate segment 1",
		},
		{
			name:         "later segment is missing",
			segments:     [][]byte{validSegment},
			requestCount: 2,
			wantErr:      "read segment 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeRemuxFixtureFiles(t, dir, fmp4testutil.BuildInitMP4(), tt.segments...)

			summary, err := ValidateRemuxSafety(dir, tt.requestCount)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want %q", err, tt.wantErr)
			}
			if summary.CheckedSegments != 1 || summary.CheckedSyncSamples != 1 {
				t.Fatalf("summary = %#v, want 1 checked segment with 1 sync sample", summary)
			}
		})
	}
}
