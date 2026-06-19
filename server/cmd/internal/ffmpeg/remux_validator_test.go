package ffmpeg

import (
	"encoding/binary"
	"fmt"
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

func TestParseTFHDRejectsTruncatedSkippedOptionalFields(t *testing.T) {
	tests := []struct {
		name  string
		flags uint32
		want  string
	}{
		{
			name:  "sample_description_index",
			flags: 0x000002,
			want:  "invalid tfhd sample description index",
		},
		{
			name:  "default_sample_duration",
			flags: 0x000008,
			want:  "invalid tfhd default sample duration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := mp4TestBoxForTest("tfhd", fullBoxPayloadForTest(tt.flags, uint32BytesForTest(1)))
			tfhd, _, err := readBox(data, 0, len(data))
			if err != nil {
				t.Fatalf("readBox: %v", err)
			}

			_, err = parseTFHD(data, mp4Box{Start: 0}, tfhd)
			if err == nil {
				t.Fatal("expected truncated tfhd optional field error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %q, want %q", err.Error(), tt.want)
			}
		})
	}
}

func TestParseTRUNRejectsTruncatedSkippedOptionalFields(t *testing.T) {
	defaultSize := uint32(1)
	defaultFlags := uint32(0)
	tests := []struct {
		name  string
		flags uint32
		want  string
	}{
		{
			name:  "sample_duration",
			flags: 0x000100,
			want:  "invalid trun sample duration",
		},
		{
			name:  "sample_composition_time_offset",
			flags: 0x000800,
			want:  "invalid trun sample composition time offset",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := mp4TestBoxForTest("trun", fullBoxPayloadForTest(tt.flags, uint32BytesForTest(1)))
			trun, _, err := readBox(data, 0, len(data))
			if err != nil {
				t.Fatalf("readBox: %v", err)
			}

			_, err = parseTRUN(data, trun, &defaultSize, &defaultFlags)
			if err == nil {
				t.Fatal("expected truncated trun optional field error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %q, want %q", err.Error(), tt.want)
			}
		})
	}
}

func TestParseVideoTrackConfig_IgnoresFakeAvcCBytePatternOutsideSampleEntry(t *testing.T) {
	initData := fmp4testutil.BuildInitMP4()

	moov, found, err := findDirectChildBox(initData, 0, len(initData), "moov")
	if err != nil {
		t.Fatalf("find moov: %v", err)
	}
	if !found {
		t.Fatal("missing moov box")
	}

	traks, err := listDirectChildBoxes(initData, moov.PayloadStart, moov.End)
	if err != nil {
		t.Fatalf("list traks: %v", err)
	}

	videoTrak := mp4Box{}
	foundVideoTrak := false
	for _, trak := range traks {
		if trak.Type != "trak" {
			continue
		}

		trackID, handlerType, parseErr := parseTrackHeader(initData, trak)
		if parseErr != nil {
			t.Fatalf("parseTrackHeader: %v", parseErr)
		}
		if trackID == 1 && handlerType == "vide" {
			videoTrak = trak
			foundVideoTrak = true
			break
		}
	}
	if !foundVideoTrak {
		t.Fatal("missing video track")
	}

	tkhd, found, err := findDirectChildBox(initData, videoTrak.PayloadStart, videoTrak.End, "tkhd")
	if err != nil {
		t.Fatalf("find tkhd: %v", err)
	}
	if !found {
		t.Fatal("missing tkhd box")
	}

	fakePayload := make([]byte, 13)
	binary.BigEndian.PutUint32(fakePayload[0:4], 13)
	copy(fakePayload[4:8], []byte("avcC"))
	fakePayload[12] = 0xfc
	fakeBox := mp4TestBoxForTest("free", fakePayload)

	mutated := make([]byte, 0, len(initData)+len(fakeBox))
	mutated = append(mutated, initData[:tkhd.End]...)
	mutated = append(mutated, fakeBox...)
	mutated = append(mutated, initData[tkhd.End:]...)

	updateTestBoxSize(mutated, moov.Start, moov.End-moov.Start+len(fakeBox))
	updateTestBoxSize(mutated, videoTrak.Start, videoTrak.End-videoTrak.Start+len(fakeBox))

	trackID, nalLengthSize, err := parseVideoTrackConfig(mutated)
	if err != nil {
		t.Fatalf("parseVideoTrackConfig returned error: %v", err)
	}
	if trackID != 1 {
		t.Fatalf("trackID = %d, want 1", trackID)
	}
	if nalLengthSize != 4 {
		t.Fatalf("nalLengthSize = %d, want 4", nalLengthSize)
	}
}

func mp4TestBoxForTest(typ string, payloadParts ...[]byte) []byte {
	payloadLen := 0
	for _, part := range payloadParts {
		payloadLen += len(part)
	}

	out := make([]byte, 8+payloadLen)
	binary.BigEndian.PutUint32(out[:4], uint32(len(out)))
	copy(out[4:8], []byte(typ))

	offset := 8
	for _, part := range payloadParts {
		copy(out[offset:], part)
		offset += len(part)
	}

	return out
}

func fullBoxPayloadForTest(flags uint32, payloadParts ...[]byte) []byte {
	out := []byte{0, byte(flags >> 16), byte(flags >> 8), byte(flags)}
	for _, part := range payloadParts {
		out = append(out, part...)
	}
	return out
}

func uint32BytesForTest(value uint32) []byte {
	out := make([]byte, 4)
	binary.BigEndian.PutUint32(out, value)
	return out
}

func setFirstTRUNSampleFlagsForTest(t *testing.T, segment []byte, flags uint32) {
	t.Helper()

	moof, found, err := findDirectChildBox(segment, 0, len(segment), "moof")
	if err != nil {
		t.Fatalf("find moof: %v", err)
	}
	if !found {
		t.Fatal("missing moof box")
	}

	traf, found, err := findDirectChildBox(segment, moof.PayloadStart, moof.End, "traf")
	if err != nil {
		t.Fatalf("find traf: %v", err)
	}
	if !found {
		t.Fatal("missing traf box")
	}

	trun, found, err := findDirectChildBox(segment, traf.PayloadStart, traf.End, "trun")
	if err != nil {
		t.Fatalf("find trun: %v", err)
	}
	if !found {
		t.Fatal("missing trun box")
	}

	sampleFlagsOffset := trun.PayloadStart + 16
	if sampleFlagsOffset+4 > trun.End {
		t.Fatalf("trun sample flags field exceeds box bounds")
	}

	binary.BigEndian.PutUint32(segment[sampleFlagsOffset:sampleFlagsOffset+4], flags)
}

func updateTestBoxSize(data []byte, start int, size int) {
	binary.BigEndian.PutUint32(data[start:start+4], uint32(size))
}
