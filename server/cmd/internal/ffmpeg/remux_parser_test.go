package ffmpeg

import (
	"encoding/binary"
	"math"
	"strconv"
	"strings"
	"testing"

	"igloo/cmd/internal/ffmpeg/fmp4testutil"
)

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

func TestReadBoxSupportsStandardExtendedAndParentSizedBoxes(t *testing.T) {
	standardData := mp4TestBoxForTest("free", []byte{1, 2, 3})
	standard, next, err := readBox(standardData, 0, len(standardData))
	if err != nil {
		t.Fatalf("standard readBox: %v", err)
	}
	if standard.Type != "free" || standard.PayloadStart != 8 || next != len(standardData) {
		t.Fatalf("standard box = %#v next=%d", standard, next)
	}

	extendedData := make([]byte, 19)
	binary.BigEndian.PutUint32(extendedData[0:4], 1)
	copy(extendedData[4:8], []byte("uuid"))
	binary.BigEndian.PutUint64(extendedData[8:16], uint64(len(extendedData)))
	copy(extendedData[16:], []byte{4, 5, 6})
	extended, next, err := readBox(extendedData, 0, len(extendedData))
	if err != nil {
		t.Fatalf("extended readBox: %v", err)
	}
	if extended.PayloadStart != 16 || extended.End != len(extendedData) || next != len(extendedData) {
		t.Fatalf("extended box = %#v next=%d", extended, next)
	}

	parentData := mp4TestBoxForTest("mdat", []byte{7, 8})
	binary.BigEndian.PutUint32(parentData[0:4], 0)
	parentSized, next, err := readBox(parentData, 0, len(parentData))
	if err != nil {
		t.Fatalf("parent-sized readBox: %v", err)
	}
	if parentSized.End != len(parentData) || next != len(parentData) {
		t.Fatalf("parent-sized box = %#v next=%d", parentSized, next)
	}
}

func TestReadBoxRejectsMalformedSizesAndBounds(t *testing.T) {
	extendedTooLarge := make([]byte, 16)
	binary.BigEndian.PutUint32(extendedTooLarge[0:4], 1)
	copy(extendedTooLarge[4:8], []byte("free"))
	binary.BigEndian.PutUint64(extendedTooLarge[8:16], math.MaxUint64)
	undersized := mp4TestBoxForTest("free")
	binary.BigEndian.PutUint32(undersized[0:4], 4)
	oversized := mp4TestBoxForTest("free")
	binary.BigEndian.PutUint32(oversized[0:4], uint32(len(oversized)+1))
	truncatedExtended := append([]byte(nil), extendedTooLarge[:12]...)

	tests := []struct {
		name  string
		data  []byte
		start int
		end   int
	}{
		{name: "truncated header", data: []byte{0, 0, 0, 8}, end: 4},
		{name: "undersized", data: undersized, end: len(undersized)},
		{name: "oversized", data: oversized, end: len(oversized)},
		{name: "truncated extended", data: truncatedExtended, end: len(truncatedExtended)},
		{name: "extended integer overflow", data: extendedTooLarge, end: len(extendedTooLarge)},
		{name: "negative start", data: oversized, start: -1, end: len(oversized)},
		{name: "end before start", data: oversized, start: 7, end: 6},
		{name: "end beyond data", data: oversized, end: len(oversized) + 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := readBox(tt.data, tt.start, tt.end)
			if err == nil {
				t.Fatal("expected malformed MP4 box error")
			}
		})
	}
}

func TestParseTFHDResolvesBaseOffsetsAndDefaults(t *testing.T) {
	flags := uint32(0x000001 | 0x000002 | 0x000008 | 0x000010 | 0x000020)
	payload := fullBoxPayloadForTest(
		flags,
		uint32BytesForTest(9),
		uint64BytesForTest(1234),
		uint32BytesForTest(2),
		uint32BytesForTest(1000),
		uint32BytesForTest(55),
		uint32BytesForTest(0x00010000),
	)
	data := mp4TestBoxForTest("tfhd", payload)
	tfhd, _, err := readBox(data, 0, len(data))
	if err != nil {
		t.Fatalf("read tfhd: %v", err)
	}
	fragment, err := parseTFHD(data, mp4Box{Start: 22}, tfhd)
	if err != nil {
		t.Fatalf("parseTFHD: %v", err)
	}
	if fragment.TrackID != 9 || fragment.BaseDataOffset == nil || *fragment.BaseDataOffset != 1234 {
		t.Fatalf("parsed base fragment = %#v", fragment)
	}
	if fragment.DefaultSize == nil || *fragment.DefaultSize != 55 {
		t.Fatalf("default size = %v, want 55", fragment.DefaultSize)
	}
	if fragment.DefaultFlags == nil || *fragment.DefaultFlags != 0x00010000 {
		t.Fatalf("default flags = %v, want non-sync", fragment.DefaultFlags)
	}

	defaultMoofFlags := uint32(0x020000)
	defaultMoofData := mp4TestBoxForTest("tfhd", fullBoxPayloadForTest(defaultMoofFlags, uint32BytesForTest(3)))
	defaultMoofBox, _, err := readBox(defaultMoofData, 0, len(defaultMoofData))
	if err != nil {
		t.Fatalf("read default-moof tfhd: %v", err)
	}
	defaultMoof, err := parseTFHD(defaultMoofData, mp4Box{Start: 44}, defaultMoofBox)
	if err != nil {
		t.Fatalf("parse default-moof tfhd: %v", err)
	}
	if defaultMoof.BaseDataOffset == nil || *defaultMoof.BaseDataOffset != 44 {
		t.Fatalf("default-base-is-moof offset = %v, want 44", defaultMoof.BaseDataOffset)
	}
}

func TestParseTFHDRejectsInvalidOptionalValues(t *testing.T) {
	tests := []struct {
		name    string
		flags   uint32
		payload []byte
		want    string
	}{
		{name: "base offset truncated", flags: 0x000001, want: "base data offset"},
		{name: "base offset overflow", flags: 0x000001, payload: uint64BytesForTest(math.MaxUint64), want: "base data offset"},
		{name: "default size truncated", flags: 0x000010, want: "default sample size"},
		{name: "default flags truncated", flags: 0x000020, want: "default sample flags"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parts := [][]byte{uint32BytesForTest(1)}
			if tt.payload != nil {
				parts = append(parts, tt.payload)
			}
			data := mp4TestBoxForTest("tfhd", fullBoxPayloadForTest(tt.flags, parts...))
			tfhd, _, err := readBox(data, 0, len(data))
			if err != nil {
				t.Fatalf("read tfhd: %v", err)
			}
			_, err = parseTFHD(data, mp4Box{}, tfhd)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestParseTRUNResolvesOffsetsFlagsAndSampleOverrides(t *testing.T) {
	defaultSize := uint32(11)
	defaultFlags := uint32(0x00010000)

	t.Run("data offset and first sample flags", func(t *testing.T) {
		flags := uint32(0x000001 | 0x000004)
		payload := fullBoxPayloadForTest(
			flags,
			uint32BytesForTest(1),
			uint32BytesForTest(math.MaxUint32-7),
			uint32BytesForTest(0),
		)
		data := mp4TestBoxForTest("trun", payload)
		trun, _, err := readBox(data, 0, len(data))
		if err != nil {
			t.Fatalf("read trun: %v", err)
		}
		runResult, err := parseTRUN(data, trun, &defaultSize, &defaultFlags)
		if err != nil {
			t.Fatalf("parseTRUN: %v", err)
		}
		if runResult.DataOffset == nil || *runResult.DataOffset != -8 {
			t.Fatalf("data offset = %v, want -8", runResult.DataOffset)
		}
		if len(runResult.Samples) != 1 || runResult.Samples[0].Size != defaultSize || runResult.Samples[0].Flags != 0 {
			t.Fatalf("first-sample resolution = %#v", runResult.Samples)
		}
	})

	t.Run("per sample overrides", func(t *testing.T) {
		flags := uint32(0x000200 | 0x000400)
		payload := fullBoxPayloadForTest(
			flags,
			uint32BytesForTest(2),
			uint32BytesForTest(5), uint32BytesForTest(0),
			uint32BytesForTest(7), uint32BytesForTest(0x00010000),
		)
		data := mp4TestBoxForTest("trun", payload)
		trun, _, err := readBox(data, 0, len(data))
		if err != nil {
			t.Fatalf("read trun: %v", err)
		}
		runResult, err := parseTRUN(data, trun, &defaultSize, &defaultFlags)
		if err != nil {
			t.Fatalf("parseTRUN: %v", err)
		}
		if len(runResult.Samples) != 2 || runResult.Samples[0].Size != 5 || runResult.Samples[1].Size != 7 {
			t.Fatalf("sample size overrides = %#v", runResult.Samples)
		}
		if runResult.Samples[0].Flags != 0 || runResult.Samples[1].Flags != 0x00010000 {
			t.Fatalf("sample flag overrides = %#v", runResult.Samples)
		}
	})

	t.Run("tfhd defaults", func(t *testing.T) {
		data := mp4TestBoxForTest("trun", fullBoxPayloadForTest(0, uint32BytesForTest(1)))
		trun, _, err := readBox(data, 0, len(data))
		if err != nil {
			t.Fatalf("read trun: %v", err)
		}
		runResult, err := parseTRUN(data, trun, &defaultSize, &defaultFlags)
		if err != nil {
			t.Fatalf("parseTRUN: %v", err)
		}
		if len(runResult.Samples) != 1 || runResult.Samples[0].Size != defaultSize || runResult.Samples[0].Flags != defaultFlags {
			t.Fatalf("default sample resolution = %#v", runResult.Samples)
		}
	})
}

func TestParseTRUNRejectsMissingDefaultsAndInvalidCounts(t *testing.T) {
	defaultSize := uint32(1)
	defaultFlags := uint32(0)
	tests := []struct {
		name         string
		count        uint32
		defaultSize  *uint32
		defaultFlags *uint32
		want         string
	}{
		{name: "missing size", count: 1, defaultFlags: &defaultFlags, want: "missing sample size"},
		{name: "missing flags", count: 1, defaultSize: &defaultSize, want: "missing sample flags"},
		{name: "disproportionate count", count: math.MaxUint32, defaultSize: &defaultSize, defaultFlags: &defaultFlags, want: "invalid trun sample count"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := mp4TestBoxForTest("trun", fullBoxPayloadForTest(0, uint32BytesForTest(tt.count)))
			trun, _, err := readBox(data, 0, len(data))
			if err != nil {
				t.Fatalf("read trun: %v", err)
			}
			_, err = parseTRUN(data, trun, tt.defaultSize, tt.defaultFlags)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestParseTRUNRejectsTruncatedOptionalFields(t *testing.T) {
	defaultSize := uint32(1)
	defaultFlags := uint32(0)
	tests := []struct {
		name  string
		flags uint32
		want  string
	}{
		{name: "data offset", flags: 0x000001, want: "data offset"},
		{name: "first sample flags", flags: 0x000004, want: "first sample flags"},
		{name: "sample size", flags: 0x000200, want: "sample size"},
		{name: "sample flags", flags: 0x000400, want: "sample flags"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := mp4TestBoxForTest("trun", fullBoxPayloadForTest(tt.flags, uint32BytesForTest(1)))
			trun, _, err := readBox(data, 0, len(data))
			if err != nil {
				t.Fatalf("read trun: %v", err)
			}
			_, err = parseTRUN(data, trun, &defaultSize, &defaultFlags)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestValidateSyncSampleSupportsNALLengthWidthsOneThroughFour(t *testing.T) {
	for width := 1; width <= 4; width++ {
		t.Run(strconv.Itoa(width), func(t *testing.T) {
			sample := appendNALForTest(nil, width, []byte{0x67, 0x64})
			sample = appendNALForTest(sample, width, []byte{0x65, 0x88})
			err := validateSyncSample(sample, width)
			if err != nil {
				t.Fatalf("validateSyncSample width %d: %v", width, err)
			}
		})
	}
}

func TestValidateSyncSampleRejectsMalformedOrNonIDRSamples(t *testing.T) {
	tests := []struct {
		name   string
		sample []byte
		width  int
		want   string
	}{
		{name: "invalid width zero", sample: []byte{1, 0x65}, width: 0, want: "invalid NAL length size"},
		{name: "invalid width five", sample: []byte{1, 0x65}, width: 5, want: "invalid NAL length size"},
		{name: "non IDR", sample: appendNALForTest(nil, 2, []byte{0x41, 0x01}), width: 2, want: "non-IDR"},
		{name: "missing VCL", sample: appendNALForTest(nil, 3, []byte{0x67, 0x01}), width: 3, want: "does not contain a VCL"},
		{name: "invalid length", sample: []byte{0, 0, 0, 8, 0x65}, width: 4, want: "invalid NAL unit size"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSyncSample(tt.sample, tt.width)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func appendNALForTest(destination []byte, width int, payload []byte) []byte {
	size := uint32(len(payload))
	length := make([]byte, width)
	switch width {
	case 1:
		length[0] = byte(size)
	case 2:
		binary.BigEndian.PutUint16(length, uint16(size))
	case 3:
		length[0] = byte(size >> 16)
		length[1] = byte(size >> 8)
		length[2] = byte(size)
	case 4:
		binary.BigEndian.PutUint32(length, size)
	}
	destination = append(destination, length...)
	return append(destination, payload...)
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

func uint64BytesForTest(value uint64) []byte {
	out := make([]byte, 8)
	binary.BigEndian.PutUint64(out, value)
	return out
}

func setFirstTRUNSampleFlagsForTest(t *testing.T, segment []byte, flags uint32) {
	t.Helper()
	trun := firstTRUNForTest(t, segment)

	sampleFlagsOffset := trun.PayloadStart + 16
	if sampleFlagsOffset+4 > trun.End {
		t.Fatalf("trun sample flags field exceeds box bounds")
	}

	binary.BigEndian.PutUint32(segment[sampleFlagsOffset:sampleFlagsOffset+4], flags)
}

func updateTestBoxSize(data []byte, start int, size int) {
	binary.BigEndian.PutUint32(data[start:start+4], uint32(size))
}
