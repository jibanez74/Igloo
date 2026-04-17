package ffmpeg

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"igloo/cmd/internal/helpers"
)

func TestValidateRemuxSafety_SafeFragments(t *testing.T) {
	dir := t.TempDir()
	err := writeValidatorFixture(dir, validatorFixture{
		SafeVideo: true,
		Segments:  helpers.HLS_REMUX_PREVALIDATE_SEGMENTS,
	})
	if err != nil {
		t.Fatalf("writeValidatorFixture: %v", err)
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
	err := writeValidatorFixture(dir, validatorFixture{
		SafeVideo: false,
		Segments:  helpers.HLS_REMUX_PREVALIDATE_SEGMENTS,
	})
	if err != nil {
		t.Fatalf("writeValidatorFixture: %v", err)
	}

	_, err = ValidateRemuxSafety(dir, helpers.HLS_REMUX_PREVALIDATE_SEGMENTS)
	if err == nil {
		t.Fatal("expected unsafe remux validation error")
	}
}

func TestValidateRemuxSafety_IgnoresAudioTrackNoise(t *testing.T) {
	dir := t.TempDir()
	err := writeValidatorFixture(dir, validatorFixture{
		SafeVideo:  true,
		AudioNoise: true,
		Segments:   helpers.HLS_REMUX_PREVALIDATE_SEGMENTS,
	})
	if err != nil {
		t.Fatalf("writeValidatorFixture: %v", err)
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

	initData := buildValidatorInitMP4()
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
			data = buildValidatorSegment(buildValidatorVideoSample(true), false)
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

type validatorFixture struct {
	SafeVideo  bool
	AudioNoise bool
	Segments   int
}

func writeValidatorFixture(dir string, fixture validatorFixture) error {
	if fixture.Segments <= 0 {
		return fmt.Errorf("segments must be positive")
	}

	err := os.WriteFile(filepath.Join(dir, helpers.HLS_INIT_FILENAME), buildValidatorInitMP4(), 0644)
	if err != nil {
		return err
	}

	for i := 0; i < fixture.Segments; i++ {
		name := fmt.Sprintf(
			"%s%d%s",
			helpers.HLS_SEGMENT_FILENAME_PREFIX,
			i,
			helpers.HLS_SEGMENT_FILENAME_SUFFIX,
		)
		segmentData := buildValidatorSegment(
			buildValidatorVideoSample(fixture.SafeVideo),
			fixture.AudioNoise,
		)
		err = os.WriteFile(filepath.Join(dir, name), segmentData, 0644)
		if err != nil {
			return err
		}
	}

	return nil
}

func buildValidatorInitMP4() []byte {
	videoTrack := validatorBox(
		"trak",
		validatorTKHD(1),
		validatorBox("mdia", validatorHDLR("vide")),
		validatorBox("avcC", []byte{1, 0x64, 0x00, 0x1f, 0xff}),
	)

	audioTrack := validatorBox(
		"trak",
		validatorTKHD(2),
		validatorBox("mdia", validatorHDLR("soun")),
	)

	return validatorBox("moov", videoTrack, audioTrack)
}

func buildValidatorSegment(videoSample []byte, includeAudioNoise bool) []byte {
	moofSize := 0
	audioSample := []byte{0xaa, 0xbb, 0xcc}

	if includeAudioNoise {
		audioTraf := validatorBox(
			"traf",
			validatorTFHD(2),
			validatorTRUN([]validatorSampleSpec{{
				Data: audioSample,
			}}),
		)
		videoTraf := validatorBox(
			"traf",
			validatorTFHD(1),
			validatorTRUN([]validatorSampleSpec{{
				Data: videoSample,
			}}),
		)
		moofSize = 8 + len(audioTraf) + len(videoTraf)

		audioTraf = validatorBox(
			"traf",
			validatorTFHD(2),
			validatorTRUN([]validatorSampleSpec{{
				Data:       audioSample,
				DataOffset: int32(moofSize + 8),
			}}),
		)
		videoTraf = validatorBox(
			"traf",
			validatorTFHD(1),
			validatorTRUN([]validatorSampleSpec{{
				Data:       videoSample,
				DataOffset: int32(moofSize + 8 + len(audioSample)),
			}}),
		)

		moof := validatorBox("moof", audioTraf, videoTraf)
		mdat := validatorBox("mdat", append(audioSample, videoSample...))
		return append(moof, mdat...)
	}

	videoTraf := validatorBox(
		"traf",
		validatorTFHD(1),
		validatorTRUN([]validatorSampleSpec{{
			Data: videoSample,
		}}),
	)
	moofSize = 8 + len(videoTraf)
	videoTraf = validatorBox(
		"traf",
		validatorTFHD(1),
		validatorTRUN([]validatorSampleSpec{{
			Data:       videoSample,
			DataOffset: int32(moofSize + 8),
		}}),
	)

	moof := validatorBox("moof", videoTraf)
	mdat := validatorBox("mdat", videoSample)
	return append(moof, mdat...)
}

func buildValidatorVideoSample(safe bool) []byte {
	sps := []byte{0x67, 0x64, 0x00, 0x1f}

	vcl := []byte{0x41, 0x9a, 0x22}
	if safe {
		vcl = []byte{0x65, 0x88, 0x84}
	}

	out := make([]byte, 0, 16)
	out = append(out, validatorNALU(sps)...)
	out = append(out, validatorNALU(vcl)...)
	return out
}

type validatorSampleSpec struct {
	Data       []byte
	DataOffset int32
}

func validatorNALU(payload []byte) []byte {
	out := make([]byte, 4+len(payload))
	binary.BigEndian.PutUint32(out[:4], uint32(len(payload)))
	copy(out[4:], payload)
	return out
}

func validatorTKHD(trackID uint32) []byte {
	payload := make([]byte, 20)
	binary.BigEndian.PutUint32(payload[12:16], trackID)
	return validatorBox("tkhd", payload)
}

func validatorHDLR(handlerType string) []byte {
	payload := make([]byte, 12)
	copy(payload[8:12], []byte(handlerType))
	return validatorBox("hdlr", payload)
}

func validatorTFHD(trackID uint32) []byte {
	payload := make([]byte, 8)
	binary.BigEndian.PutUint32(payload[4:8], trackID)
	return validatorBox("tfhd", payload)
}

func validatorTRUN(samples []validatorSampleSpec) []byte {
	flags := uint32(0x000001 | 0x000200 | 0x000400)
	payload := make([]byte, 8)
	payload[1] = byte(flags >> 16)
	payload[2] = byte(flags >> 8)
	payload[3] = byte(flags)
	binary.BigEndian.PutUint32(payload[4:8], uint32(len(samples)))

	dataOffset := int32(0)
	if len(samples) > 0 {
		dataOffset = samples[0].DataOffset
	}
	offsetBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(offsetBytes, uint32(dataOffset))
	payload = append(payload, offsetBytes...)

	for _, sample := range samples {
		sizeBytes := make([]byte, 4)
		binary.BigEndian.PutUint32(sizeBytes, uint32(len(sample.Data)))
		payload = append(payload, sizeBytes...)

		flagBytes := make([]byte, 4)
		binary.BigEndian.PutUint32(flagBytes, 0)
		payload = append(payload, flagBytes...)
	}

	return validatorBox("trun", payload)
}

func validatorBox(typ string, payloadParts ...[]byte) []byte {
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
