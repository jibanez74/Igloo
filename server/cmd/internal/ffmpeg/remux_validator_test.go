package ffmpeg

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
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

	_, err = ValidateRemuxSafety(dir, helpers.HLS_REMUX_PREVALIDATE_SEGMENTS)
	if err == nil {
		t.Fatal("expected malformed fragment validation error")
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

func updateTestBoxSize(data []byte, start int, size int) {
	binary.BigEndian.PutUint32(data[start:start+4], uint32(size))
}
