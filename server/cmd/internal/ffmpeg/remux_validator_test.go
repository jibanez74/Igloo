package ffmpeg

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"igloo/cmd/internal/ffmpeg/fmp4testutil"
	"igloo/cmd/internal/helpers"
)

func TestValidateRemuxSafety_SafeFragments(t *testing.T) {
	dir := t.TempDir()
	err := fmp4testutil.WriteHLSFixture(dir, fmp4testutil.Fixture{
		SafeVideo: true,
		Segments:  helpers.HLS_REMUX_PREVALIDATE_SEGMENTS,
	})
	if err != nil {
		t.Fatalf("WriteHLSFixture: %v", err)
	}

	summary, err := ValidateRemuxSafety(dir, helpers.HLS_REMUX_PREVALIDATE_SEGMENTS)
	if err != nil {
		t.Fatalf("ValidateRemuxSafety returned error: %v", err)
	}
	if summary.CheckedSegments != helpers.HLS_REMUX_PREVALIDATE_SEGMENTS {
		t.Fatalf("CheckedSegments = %d, want %d", summary.CheckedSegments, helpers.HLS_REMUX_PREVALIDATE_SEGMENTS)
	}
	if summary.CheckedSyncSamples != helpers.HLS_REMUX_PREVALIDATE_SEGMENTS {
		t.Fatalf("CheckedSyncSamples = %d, want %d", summary.CheckedSyncSamples, helpers.HLS_REMUX_PREVALIDATE_SEGMENTS)
	}
}

func TestValidateRemuxSafety_UnsafeSyncSampleMetadata(t *testing.T) {
	dir := t.TempDir()
	err := fmp4testutil.WriteHLSFixture(dir, fmp4testutil.Fixture{
		SafeVideo: false,
		Segments:  helpers.HLS_REMUX_PREVALIDATE_SEGMENTS,
	})
	if err != nil {
		t.Fatalf("WriteHLSFixture: %v", err)
	}

	_, err = ValidateRemuxSafety(dir, helpers.HLS_REMUX_PREVALIDATE_SEGMENTS)
	if err == nil {
		t.Fatal("expected unsafe remux validation error")
	}
}

func TestValidateRemuxSafety_IgnoresAudioTrackNoise(t *testing.T) {
	dir := t.TempDir()
	err := fmp4testutil.WriteHLSFixture(dir, fmp4testutil.Fixture{
		SafeVideo:  true,
		AudioNoise: true,
		Segments:   helpers.HLS_REMUX_PREVALIDATE_SEGMENTS,
	})
	if err != nil {
		t.Fatalf("WriteHLSFixture: %v", err)
	}

	summary, err := ValidateRemuxSafety(dir, helpers.HLS_REMUX_PREVALIDATE_SEGMENTS)
	if err != nil {
		t.Fatalf("ValidateRemuxSafety returned error: %v", err)
	}
	if summary.CheckedSegments != helpers.HLS_REMUX_PREVALIDATE_SEGMENTS {
		t.Fatalf("CheckedSegments = %d, want %d", summary.CheckedSegments, helpers.HLS_REMUX_PREVALIDATE_SEGMENTS)
	}
}

func TestValidateRemuxSafety_MalformedFragmentsAreUnsafe(t *testing.T) {
	dir := t.TempDir()

	initData := fmp4testutil.BuildInitMP4()
	err := os.WriteFile(filepath.Join(dir, helpers.HLS_INIT_FILENAME), initData, 0644)
	if err != nil {
		t.Fatalf("write init.mp4: %v", err)
	}

	for i := 0; i < helpers.HLS_REMUX_PREVALIDATE_SEGMENTS; i++ {
		name := fmt.Sprintf(
			"%s%d%s",
			helpers.HLS_SEGMENT_FILENAME_PREFIX,
			i,
			helpers.HLS_SEGMENT_FILENAME_SUFFIX,
		)

		data := []byte("not-a-valid-fragment")
		if i == 0 {
			data = fmp4testutil.BuildSegment(fmp4testutil.BuildVideoSample(true), false)
		}

		err = os.WriteFile(filepath.Join(dir, name), data, 0644)
		if err != nil {
			t.Fatalf("write segment %d: %v", i, err)
		}
	}

	summary, err := ValidateRemuxSafety(dir, helpers.HLS_REMUX_PREVALIDATE_SEGMENTS)
	if err == nil {
		t.Fatal("expected malformed fragment validation error")
	}
	if summary.CheckedSegments != 1 {
		t.Fatalf("CheckedSegments = %d, want partial summary with 1 checked segment", summary.CheckedSegments)
	}
	if summary.CheckedSyncSamples != 1 {
		t.Fatalf("CheckedSyncSamples = %d, want partial summary with 1 sync sample", summary.CheckedSyncSamples)
	}
}

func TestValidateRemuxSafety_MissingLaterSegmentReturnsPartialSummary(t *testing.T) {
	dir := t.TempDir()

	initData := fmp4testutil.BuildInitMP4()
	err := os.WriteFile(filepath.Join(dir, helpers.HLS_INIT_FILENAME), initData, 0644)
	if err != nil {
		t.Fatalf("write init.mp4: %v", err)
	}

	name := fmt.Sprintf(
		"%s%d%s",
		helpers.HLS_SEGMENT_FILENAME_PREFIX,
		0,
		helpers.HLS_SEGMENT_FILENAME_SUFFIX,
	)
	segment := fmp4testutil.BuildSegment(fmp4testutil.BuildVideoSample(true), false)
	err = os.WriteFile(filepath.Join(dir, name), segment, 0644)
	if err != nil {
		t.Fatalf("write segment 0: %v", err)
	}

	summary, err := ValidateRemuxSafety(dir, 2)
	if err == nil {
		t.Fatal("expected missing segment validation error")
	}
	if !strings.Contains(err.Error(), "read segment 1") {
		t.Fatalf("error = %q, want read segment 1 failure", err.Error())
	}
	if summary.CheckedSegments != 1 {
		t.Fatalf("CheckedSegments = %d, want partial summary with 1 checked segment", summary.CheckedSegments)
	}
	if summary.CheckedSyncSamples != 1 {
		t.Fatalf("CheckedSyncSamples = %d, want partial summary with 1 sync sample", summary.CheckedSyncSamples)
	}
}

func TestValidateRemuxSafety_ZeroSyncSamplesAreUnsafe(t *testing.T) {
	dir := t.TempDir()

	initData := fmp4testutil.BuildInitMP4()
	err := os.WriteFile(filepath.Join(dir, helpers.HLS_INIT_FILENAME), initData, 0644)
	if err != nil {
		t.Fatalf("write init.mp4: %v", err)
	}

	segment := fmp4testutil.BuildSegment(fmp4testutil.BuildVideoSample(true), false)
	setFirstTRUNSampleFlagsForTest(t, segment, 0x00010000)

	name := fmt.Sprintf(
		"%s%d%s",
		helpers.HLS_SEGMENT_FILENAME_PREFIX,
		0,
		helpers.HLS_SEGMENT_FILENAME_SUFFIX,
	)
	err = os.WriteFile(filepath.Join(dir, name), segment, 0644)
	if err != nil {
		t.Fatalf("write segment 0: %v", err)
	}

	summary, err := ValidateRemuxSafety(dir, 1)
	if err == nil {
		t.Fatal("expected zero-sync-sample validation error")
	}
	if !strings.Contains(err.Error(), "no sync samples") {
		t.Fatalf("error = %q, want no sync samples failure", err.Error())
	}
	if summary.CheckedSegments != 1 {
		t.Fatalf("CheckedSegments = %d, want checked zero-sync segment counted", summary.CheckedSegments)
	}
	if summary.CheckedSyncSamples != 0 {
		t.Fatalf("CheckedSyncSamples = %d, want 0", summary.CheckedSyncSamples)
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

func TestValidateRemuxSafetyRejectsAbsentVideoFragment(t *testing.T) {
	dir := t.TempDir()
	initData := fmp4testutil.BuildInitMP4()
	err := os.WriteFile(filepath.Join(dir, helpers.HLS_INIT_FILENAME), initData, 0644)
	if err != nil {
		t.Fatalf("write init: %v", err)
	}
	segment := fmp4testutil.BuildSegment(fmp4testutil.BuildVideoSample(true), false)
	setFirstTFHDTrackIDForTest(t, segment, 99)
	err = os.WriteFile(filepath.Join(dir, helpers.HLS_SEGMENT_FILENAME_PREFIX+"0"+helpers.HLS_SEGMENT_FILENAME_SUFFIX), segment, 0644)
	if err != nil {
		t.Fatalf("write segment: %v", err)
	}

	summary, err := ValidateRemuxSafety(dir, 1)
	if err == nil || !strings.Contains(err.Error(), "missing video traf") {
		t.Fatalf("error = %v, want missing video fragment", err)
	}
	if summary.CheckedSegments != 0 || summary.CheckedSyncSamples != 0 {
		t.Fatalf("summary = %#v, want no completed segments", summary)
	}
}

func TestValidateRemuxSafetyRejectsSampleBoundsFailure(t *testing.T) {
	dir := t.TempDir()
	initData := fmp4testutil.BuildInitMP4()
	err := os.WriteFile(filepath.Join(dir, helpers.HLS_INIT_FILENAME), initData, 0644)
	if err != nil {
		t.Fatalf("write init: %v", err)
	}
	segment := fmp4testutil.BuildSegment(fmp4testutil.BuildVideoSample(true), false)
	setFirstTRUNSampleSizeForTest(t, segment, math.MaxUint32)
	err = os.WriteFile(filepath.Join(dir, helpers.HLS_SEGMENT_FILENAME_PREFIX+"0"+helpers.HLS_SEGMENT_FILENAME_SUFFIX), segment, 0644)
	if err != nil {
		t.Fatalf("write segment: %v", err)
	}

	summary, err := ValidateRemuxSafety(dir, 1)
	if err == nil || !strings.Contains(err.Error(), "sample exceeds segment bounds") {
		t.Fatalf("error = %v, want sample bounds failure", err)
	}
	if summary.CheckedSegments != 0 {
		t.Fatalf("CheckedSegments = %d, want 0", summary.CheckedSegments)
	}
}

func TestValidateRemuxSafetyRejectsSampleOutsideMdat(t *testing.T) {
	dir := t.TempDir()
	initData := fmp4testutil.BuildInitMP4()
	err := os.WriteFile(filepath.Join(dir, helpers.HLS_INIT_FILENAME), initData, 0644)
	if err != nil {
		t.Fatalf("write init: %v", err)
	}
	segment := fmp4testutil.BuildSegment(fmp4testutil.BuildVideoSample(true), false)
	// Point the run at the moof interior; the bytes stay inside the segment but
	// outside any mdat payload.
	setFirstTRUNDataOffsetForTest(t, segment, 8)
	err = os.WriteFile(filepath.Join(dir, helpers.HLS_SEGMENT_FILENAME_PREFIX+"0"+helpers.HLS_SEGMENT_FILENAME_SUFFIX), segment, 0644)
	if err != nil {
		t.Fatalf("write segment: %v", err)
	}

	summary, err := ValidateRemuxSafety(dir, 1)
	if err == nil || !strings.Contains(err.Error(), "sample outside mdat payload") {
		t.Fatalf("error = %v, want sample outside mdat payload", err)
	}
	if summary.CheckedSegments != 0 {
		t.Fatalf("CheckedSegments = %d, want 0", summary.CheckedSegments)
	}
}

func firstTRUNForTest(t *testing.T, segment []byte) mp4Box {
	t.Helper()
	moof, found, err := findDirectChildBox(segment, 0, len(segment), "moof")
	if err != nil || !found {
		t.Fatalf("find moof: found=%v err=%v", found, err)
	}
	traf, found, err := findDirectChildBox(segment, moof.PayloadStart, moof.End, "traf")
	if err != nil || !found {
		t.Fatalf("find traf: found=%v err=%v", found, err)
	}
	trun, found, err := findDirectChildBox(segment, traf.PayloadStart, traf.End, "trun")
	if err != nil || !found {
		t.Fatalf("find trun: found=%v err=%v", found, err)
	}
	return trun
}

func setFirstTRUNSampleSizeForTest(t *testing.T, segment []byte, size uint32) {
	t.Helper()
	trun := firstTRUNForTest(t, segment)
	sampleSizeOffset := trun.PayloadStart + 12
	if sampleSizeOffset+4 > trun.End {
		t.Fatal("trun sample size exceeds box bounds")
	}
	binary.BigEndian.PutUint32(segment[sampleSizeOffset:sampleSizeOffset+4], size)
}

func setFirstTRUNDataOffsetForTest(t *testing.T, segment []byte, dataOffset int32) {
	t.Helper()
	trun := firstTRUNForTest(t, segment)
	dataOffsetStart := trun.PayloadStart + 8
	if dataOffsetStart+4 > trun.End {
		t.Fatal("trun data offset exceeds box bounds")
	}
	binary.BigEndian.PutUint32(segment[dataOffsetStart:dataOffsetStart+4], uint32(dataOffset))
}

func setFirstTFHDTrackIDForTest(t *testing.T, segment []byte, trackID uint32) {
	t.Helper()
	moof, found, err := findDirectChildBox(segment, 0, len(segment), "moof")
	if err != nil || !found {
		t.Fatalf("find moof: found=%v err=%v", found, err)
	}
	traf, found, err := findDirectChildBox(segment, moof.PayloadStart, moof.End, "traf")
	if err != nil || !found {
		t.Fatalf("find traf: found=%v err=%v", found, err)
	}
	tfhd, found, err := findDirectChildBox(segment, traf.PayloadStart, traf.End, "tfhd")
	if err != nil || !found {
		t.Fatalf("find tfhd: found=%v err=%v", found, err)
	}
	binary.BigEndian.PutUint32(segment[tfhd.PayloadStart+4:tfhd.PayloadStart+8], trackID)
}
