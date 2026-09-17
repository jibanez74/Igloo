package ffmpeg

import (
	"encoding/binary"
	"math"
	"strconv"
	"strings"
	"testing"

	"igloo/cmd/internal/ffmpeg/fmp4testutil"
)

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
	fakeBox := fmp4testutil.Box("free", fakePayload)

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
	standardData := fmp4testutil.Box("free", []byte{1, 2, 3})
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

	parentData := fmp4testutil.Box("mdat", []byte{7, 8})
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
	undersized := fmp4testutil.Box("free")
	binary.BigEndian.PutUint32(undersized[0:4], 4)
	oversized := fmp4testutil.Box("free")
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
	payload := fmp4testutil.FullBoxPayload(
		flags,
		fmp4testutil.U32(9),
		fmp4testutil.U64(1234),
		fmp4testutil.U32(2),
		fmp4testutil.U32(1000),
		fmp4testutil.U32(55),
		fmp4testutil.U32(0x00010000),
	)
	data, tfhd := readTestBox(t, "tfhd", payload)
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

	defaultMoofPayload := fmp4testutil.FullBoxPayload(0x020000, fmp4testutil.U32(3))
	defaultMoofData, defaultMoofBox := readTestBox(t, "tfhd", defaultMoofPayload)
	defaultMoof, err := parseTFHD(defaultMoofData, mp4Box{Start: 44}, defaultMoofBox)
	if err != nil {
		t.Fatalf("parse default-moof tfhd: %v", err)
	}
	if defaultMoof.BaseDataOffset == nil || *defaultMoof.BaseDataOffset != 44 {
		t.Fatalf("default-base-is-moof offset = %v, want 44", defaultMoof.BaseDataOffset)
	}
}

// Every optional tfhd field must be bounds-checked, both the ones the parser
// only skips over and the ones it reads.
func TestParseTFHDRejectsInvalidOptionalFields(t *testing.T) {
	tests := []struct {
		name  string
		flags uint32
		extra []byte
		want  string
	}{
		{name: "sample description index truncated", flags: 0x000002, want: "invalid tfhd sample description index"},
		{name: "default sample duration truncated", flags: 0x000008, want: "invalid tfhd default sample duration"},
		{name: "base offset truncated", flags: 0x000001, want: "base data offset"},
		{name: "base offset overflow", flags: 0x000001, extra: fmp4testutil.U64(math.MaxUint64), want: "base data offset"},
		{name: "default size truncated", flags: 0x000010, want: "default sample size"},
		{name: "default flags truncated", flags: 0x000020, want: "default sample flags"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parts := [][]byte{fmp4testutil.U32(1)}
			if tt.extra != nil {
				parts = append(parts, tt.extra)
			}
			data, tfhd := readTestBox(t, "tfhd", fmp4testutil.FullBoxPayload(tt.flags, parts...))

			_, err := parseTFHD(data, mp4Box{}, tfhd)
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
		payload := fmp4testutil.FullBoxPayload(
			0x000001|0x000004,
			fmp4testutil.U32(1),
			fmp4testutil.U32(math.MaxUint32-7),
			fmp4testutil.U32(0),
		)
		data, trun := readTestBox(t, "trun", payload)
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
		payload := fmp4testutil.FullBoxPayload(
			0x000200|0x000400,
			fmp4testutil.U32(2),
			fmp4testutil.U32(5), fmp4testutil.U32(0),
			fmp4testutil.U32(7), fmp4testutil.U32(0x00010000),
		)
		data, trun := readTestBox(t, "trun", payload)
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
		payload := fmp4testutil.FullBoxPayload(0, fmp4testutil.U32(1))
		data, trun := readTestBox(t, "trun", payload)
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
			payload := fmp4testutil.FullBoxPayload(0, fmp4testutil.U32(tt.count))
			data, trun := readTestBox(t, "trun", payload)
			_, err := parseTRUN(data, trun, tt.defaultSize, tt.defaultFlags)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

// Every optional trun field must be bounds-checked, both the ones the parser
// only skips over and the ones it reads.
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
		{name: "sample duration", flags: 0x000100, want: "invalid trun sample duration"},
		{name: "sample size", flags: 0x000200, want: "sample size"},
		{name: "sample flags", flags: 0x000400, want: "sample flags"},
		{name: "sample composition time offset", flags: 0x000800, want: "invalid trun sample composition time offset"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := fmp4testutil.FullBoxPayload(tt.flags, fmp4testutil.U32(1))
			data, trun := readTestBox(t, "trun", payload)
			_, err := parseTRUN(data, trun, &defaultSize, &defaultFlags)
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
