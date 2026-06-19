package ffmpeg

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"

	"igloo/cmd/internal/helpers"
)

type RemuxValidationSummary struct {
	CheckedSegments    int
	CheckedSyncSamples int
}

type mp4Box struct {
	Type         string
	Start        int
	End          int
	PayloadStart int
}

type trackFragment struct {
	TrackID        uint32
	BaseDataOffset *int64
	DefaultIsMoof  bool
	DefaultSize    *uint32
	DefaultFlags   *uint32
	Runs           []trackRun
}

type trackRun struct {
	DataOffset *int32
	Samples    []fragmentSample
}

type fragmentSample struct {
	Size  uint32
	Flags uint32
}

const avcSampleEntryHeaderSize = 78

// ValidateRemuxSafety inspects FFmpeg-generated fMP4 HLS fragments and
// verifies that every sync sample in the video track starts with an IDR frame.
func ValidateRemuxSafety(outDir string, segmentCount int) (RemuxValidationSummary, error) {
	if segmentCount <= 0 {
		return RemuxValidationSummary{}, fmt.Errorf("segmentCount must be positive")
	}

	initPath := filepath.Join(outDir, helpers.HLS_INIT_FILENAME)
	initData, err := os.ReadFile(initPath)
	if err != nil {
		return RemuxValidationSummary{}, fmt.Errorf("read init segment: %w", err)
	}

	videoTrackID, nalLengthSize, err := parseVideoTrackConfig(initData)
	if err != nil {
		return RemuxValidationSummary{}, err
	}

	summary := RemuxValidationSummary{}
	for i := 0; i < segmentCount; i++ {
		name := fmt.Sprintf(
			"%s%d%s",
			helpers.HLS_SEGMENT_FILENAME_PREFIX,
			i,
			helpers.HLS_SEGMENT_FILENAME_SUFFIX,
		)
		segmentData, readErr := os.ReadFile(filepath.Join(outDir, name))
		if readErr != nil {
			return summary, fmt.Errorf("read segment %d: %w", i, readErr)
		}

		syncSamples, inspectErr := validateSegmentVideoTrack(
			segmentData,
			videoTrackID,
			nalLengthSize,
		)
		if inspectErr != nil {
			return summary, fmt.Errorf("validate segment %d: %w", i, inspectErr)
		}

		summary.CheckedSegments++
		summary.CheckedSyncSamples += syncSamples
		if syncSamples == 0 {
			return summary, fmt.Errorf("validate segment %d: no sync samples found", i)
		}
	}

	return summary, nil
}

func parseVideoTrackConfig(data []byte) (uint32, int, error) {
	moov, ok, err := findDirectChildBox(data, 0, len(data), "moov")
	if err != nil {
		return 0, 0, err
	}
	if !ok {
		return 0, 0, fmt.Errorf("missing moov box")
	}

	traks, err := listDirectChildBoxes(data, moov.PayloadStart, moov.End)
	if err != nil {
		return 0, 0, err
	}

	for _, trak := range traks {
		if trak.Type != "trak" {
			continue
		}

		trackID, handlerType, parseErr := parseTrackHeader(data, trak)
		if parseErr != nil {
			return 0, 0, parseErr
		}
		if handlerType != "vide" {
			continue
		}

		avcC, found, findErr := findAVCConfigBox(data, trak)
		if findErr != nil {
			return 0, 0, findErr
		}
		if !found {
			return 0, 0, fmt.Errorf("missing avcC box for video track")
		}

		payload := data[avcC.PayloadStart:avcC.End]
		if len(payload) < 5 {
			return 0, 0, fmt.Errorf("invalid avcC payload")
		}

		return trackID, int(payload[4]&0x03) + 1, nil
	}

	return 0, 0, fmt.Errorf("missing video track")
}

func parseTrackHeader(data []byte, trak mp4Box) (uint32, string, error) {
	children, err := listDirectChildBoxes(data, trak.PayloadStart, trak.End)
	if err != nil {
		return 0, "", err
	}

	var trackID uint32
	var haveTrackID bool
	var handlerType string

	for _, child := range children {
		switch child.Type {
		case "tkhd":
			parsedTrackID, parseErr := parseTrackID(data, child)
			if parseErr != nil {
				return 0, "", parseErr
			}
			trackID = parsedTrackID
			haveTrackID = true
		case "mdia":
			parsedHandlerType, parseErr := parseHandlerType(data, child)
			if parseErr != nil {
				return 0, "", parseErr
			}
			handlerType = parsedHandlerType
		}
	}

	if !haveTrackID {
		return 0, "", fmt.Errorf("missing tkhd box")
	}
	if handlerType == "" {
		return 0, "", fmt.Errorf("missing hdlr box")
	}

	return trackID, handlerType, nil
}

func parseTrackID(data []byte, tkhd mp4Box) (uint32, error) {
	payload := data[tkhd.PayloadStart:tkhd.End]
	if len(payload) < 20 {
		return 0, fmt.Errorf("invalid tkhd payload")
	}

	version := payload[0]
	var trackIDOffset int
	if version == 1 {
		trackIDOffset = 20
	} else {
		trackIDOffset = 12
	}
	if len(payload) < trackIDOffset+4 {
		return 0, fmt.Errorf("invalid tkhd payload")
	}

	return binary.BigEndian.Uint32(payload[trackIDOffset : trackIDOffset+4]), nil
}

func parseHandlerType(data []byte, mdia mp4Box) (string, error) {
	hdlr, ok, err := findDirectChildBox(data, mdia.PayloadStart, mdia.End, "hdlr")
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("missing hdlr box")
	}

	payload := data[hdlr.PayloadStart:hdlr.End]
	if len(payload) < 12 {
		return "", fmt.Errorf("invalid hdlr payload")
	}

	return string(payload[8:12]), nil
}

func findAVCConfigBox(data []byte, trak mp4Box) (mp4Box, bool, error) {
	mdia, ok, err := findDirectChildBox(data, trak.PayloadStart, trak.End, "mdia")
	if err != nil {
		return mp4Box{}, false, err
	}
	if !ok {
		return mp4Box{}, false, fmt.Errorf("missing mdia box")
	}

	minf, ok, err := findDirectChildBox(data, mdia.PayloadStart, mdia.End, "minf")
	if err != nil {
		return mp4Box{}, false, err
	}
	if !ok {
		return mp4Box{}, false, fmt.Errorf("missing minf box")
	}

	stbl, ok, err := findDirectChildBox(data, minf.PayloadStart, minf.End, "stbl")
	if err != nil {
		return mp4Box{}, false, err
	}
	if !ok {
		return mp4Box{}, false, fmt.Errorf("missing stbl box")
	}

	stsd, ok, err := findDirectChildBox(data, stbl.PayloadStart, stbl.End, "stsd")
	if err != nil {
		return mp4Box{}, false, err
	}
	if !ok {
		return mp4Box{}, false, fmt.Errorf("missing stsd box")
	}

	return findAVCConfigInSampleDescriptions(data, stsd)
}

func findAVCConfigInSampleDescriptions(data []byte, stsd mp4Box) (mp4Box, bool, error) {
	payload := data[stsd.PayloadStart:stsd.End]
	if len(payload) < 8 {
		return mp4Box{}, false, fmt.Errorf("invalid stsd payload")
	}

	entryCount := int(binary.BigEndian.Uint32(payload[4:8]))
	offset := stsd.PayloadStart + 8

	for i := 0; i < entryCount; i++ {
		entry, nextOffset, err := readBox(data, offset, stsd.End)
		if err != nil {
			return mp4Box{}, false, err
		}

		if entry.Type == "avc1" || entry.Type == "avc3" {
			childStart := entry.PayloadStart + avcSampleEntryHeaderSize
			if childStart > entry.End {
				return mp4Box{}, false, fmt.Errorf("invalid %s sample entry", entry.Type)
			}

			avcC, ok, findErr := findDirectChildBox(data, childStart, entry.End, "avcC")
			if findErr != nil {
				return mp4Box{}, false, findErr
			}
			if ok {
				return avcC, true, nil
			}
		}

		offset = nextOffset
	}

	return mp4Box{}, false, nil
}

func validateSegmentVideoTrack(data []byte, videoTrackID uint32, nalLengthSize int) (int, error) {
	moof, ok, err := findDirectChildBox(data, 0, len(data), "moof")
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, fmt.Errorf("missing moof box")
	}

	fragments, err := parseTrackFragments(data, moof)
	if err != nil {
		return 0, err
	}

	for _, fragment := range fragments {
		if fragment.TrackID != videoTrackID {
			continue
		}

		baseOffset := moof.Start
		if fragment.BaseDataOffset != nil {
			baseOffset = int(*fragment.BaseDataOffset)
		}

		cursor := 0
		cursorInitialized := false
		syncSamples := 0

		for _, run := range fragment.Runs {
			if run.DataOffset != nil {
				cursor = baseOffset + int(*run.DataOffset)
				cursorInitialized = true
			} else if !cursorInitialized {
				return 0, fmt.Errorf("missing initial trun data offset")
			}

			for _, sample := range run.Samples {
				sampleSize := int(sample.Size)
				if sampleSize <= 0 {
					return 0, fmt.Errorf("invalid sample size %d", sample.Size)
				}
				if cursor < 0 || cursor+sampleSize > len(data) {
					return 0, fmt.Errorf("sample exceeds segment bounds")
				}

				if isSyncSample(sample.Flags) {
					syncSamples++
					validateErr := validateSyncSample(
						data[cursor:cursor+sampleSize],
						nalLengthSize,
					)
					if validateErr != nil {
						return 0, validateErr
					}
				}

				cursor += sampleSize
			}
		}

		return syncSamples, nil
	}

	return 0, fmt.Errorf("missing video traf for track %d", videoTrackID)
}

func parseTrackFragments(data []byte, moof mp4Box) ([]trackFragment, error) {
	children, err := listDirectChildBoxes(data, moof.PayloadStart, moof.End)
	if err != nil {
		return nil, err
	}

	fragments := make([]trackFragment, 0, len(children))
	for _, child := range children {
		if child.Type != "traf" {
			continue
		}

		fragment, parseErr := parseTrackFragment(data, moof, child)
		if parseErr != nil {
			return nil, parseErr
		}
		fragments = append(fragments, fragment)
	}

	if len(fragments) == 0 {
		return nil, fmt.Errorf("missing traf boxes")
	}

	return fragments, nil
}

func parseTrackFragment(data []byte, moof mp4Box, traf mp4Box) (trackFragment, error) {
	children, err := listDirectChildBoxes(data, traf.PayloadStart, traf.End)
	if err != nil {
		return trackFragment{}, err
	}

	fragment := trackFragment{}
	var trackIDSet bool

	for _, child := range children {
		switch child.Type {
		case "tfhd":
			parsed, parseErr := parseTFHD(data, moof, child)
			if parseErr != nil {
				return trackFragment{}, parseErr
			}
			fragment = parsed
			trackIDSet = true
		case "trun":
			run, parseErr := parseTRUN(
				data,
				child,
				fragment.DefaultSize,
				fragment.DefaultFlags,
			)
			if parseErr != nil {
				return trackFragment{}, parseErr
			}
			fragment.Runs = append(fragment.Runs, run)
		}
	}

	if !trackIDSet {
		return trackFragment{}, fmt.Errorf("missing tfhd box")
	}
	if len(fragment.Runs) == 0 {
		return trackFragment{}, fmt.Errorf("missing trun box")
	}

	return fragment, nil
}

func parseTFHD(data []byte, moof mp4Box, tfhd mp4Box) (trackFragment, error) {
	payload := data[tfhd.PayloadStart:tfhd.End]
	if len(payload) < 8 {
		return trackFragment{}, fmt.Errorf("invalid tfhd payload")
	}

	flags := readFullBoxFlags(payload)
	offset := 4
	if len(payload) < offset+4 {
		return trackFragment{}, fmt.Errorf("invalid tfhd payload")
	}

	fragment := trackFragment{
		TrackID:       binary.BigEndian.Uint32(payload[offset : offset+4]),
		DefaultIsMoof: flags&0x020000 != 0,
	}
	offset += 4

	if flags&0x000001 != 0 {
		if len(payload) < offset+8 {
			return trackFragment{}, fmt.Errorf("invalid tfhd base data offset")
		}
		baseDataOffset := int64(binary.BigEndian.Uint64(payload[offset : offset+8]))
		fragment.BaseDataOffset = &baseDataOffset
		offset += 8
	} else if fragment.DefaultIsMoof {
		moofStart := int64(moof.Start)
		fragment.BaseDataOffset = &moofStart
	}

	if flags&0x000002 != 0 {
		if len(payload) < offset+4 {
			return trackFragment{}, fmt.Errorf("invalid tfhd sample description index")
		}
		offset += 4
	}
	if flags&0x000008 != 0 {
		if len(payload) < offset+4 {
			return trackFragment{}, fmt.Errorf("invalid tfhd default sample duration")
		}
		offset += 4
	}
	if flags&0x000010 != 0 {
		if len(payload) < offset+4 {
			return trackFragment{}, fmt.Errorf("invalid tfhd default sample size")
		}
		defaultSize := binary.BigEndian.Uint32(payload[offset : offset+4])
		fragment.DefaultSize = &defaultSize
		offset += 4
	}
	if flags&0x000020 != 0 {
		if len(payload) < offset+4 {
			return trackFragment{}, fmt.Errorf("invalid tfhd default sample flags")
		}
		defaultFlags := binary.BigEndian.Uint32(payload[offset : offset+4])
		fragment.DefaultFlags = &defaultFlags
	}

	return fragment, nil
}

func parseTRUN(
	data []byte,
	trun mp4Box,
	defaultSampleSize *uint32,
	defaultSampleFlags *uint32,
) (trackRun, error) {
	payload := data[trun.PayloadStart:trun.End]
	if len(payload) < 8 {
		return trackRun{}, fmt.Errorf("invalid trun payload")
	}

	flags := readFullBoxFlags(payload)
	offset := 4

	if len(payload) < offset+4 {
		return trackRun{}, fmt.Errorf("invalid trun payload")
	}
	sampleCount := int(binary.BigEndian.Uint32(payload[offset : offset+4]))
	offset += 4

	run := trackRun{
		Samples: make([]fragmentSample, 0, sampleCount),
	}

	if flags&0x000001 != 0 {
		if len(payload) < offset+4 {
			return trackRun{}, fmt.Errorf("invalid trun data offset")
		}
		dataOffset := int32(binary.BigEndian.Uint32(payload[offset : offset+4]))
		run.DataOffset = &dataOffset
		offset += 4
	}

	var firstSampleFlags *uint32
	if flags&0x000004 != 0 {
		if len(payload) < offset+4 {
			return trackRun{}, fmt.Errorf("invalid trun first sample flags")
		}
		value := binary.BigEndian.Uint32(payload[offset : offset+4])
		firstSampleFlags = &value
		offset += 4
	}

	for i := 0; i < sampleCount; i++ {
		var sampleSize *uint32
		var sampleFlags *uint32

		if flags&0x000100 != 0 {
			if len(payload) < offset+4 {
				return trackRun{}, fmt.Errorf("invalid trun sample duration")
			}
			offset += 4
		}
		if flags&0x000200 != 0 {
			if len(payload) < offset+4 {
				return trackRun{}, fmt.Errorf("invalid trun sample size")
			}
			value := binary.BigEndian.Uint32(payload[offset : offset+4])
			sampleSize = &value
			offset += 4
		}
		if flags&0x000400 != 0 {
			if len(payload) < offset+4 {
				return trackRun{}, fmt.Errorf("invalid trun sample flags")
			}
			value := binary.BigEndian.Uint32(payload[offset : offset+4])
			sampleFlags = &value
			offset += 4
		}
		if flags&0x000800 != 0 {
			if len(payload) < offset+4 {
				return trackRun{}, fmt.Errorf("invalid trun sample composition time offset")
			}
			offset += 4
		}

		resolvedSize := sampleSize
		if resolvedSize == nil {
			resolvedSize = defaultSampleSize
		}
		if resolvedSize == nil {
			return trackRun{}, fmt.Errorf("missing sample size")
		}

		resolvedFlags := sampleFlags
		if resolvedFlags == nil {
			if i == 0 && firstSampleFlags != nil {
				resolvedFlags = firstSampleFlags
			} else {
				resolvedFlags = defaultSampleFlags
			}
		}
		if resolvedFlags == nil {
			return trackRun{}, fmt.Errorf("missing sample flags")
		}

		run.Samples = append(run.Samples, fragmentSample{
			Size:  *resolvedSize,
			Flags: *resolvedFlags,
		})
	}

	return run, nil
}

func validateSyncSample(sample []byte, nalLengthSize int) error {
	if nalLengthSize < 1 || nalLengthSize > 4 {
		return fmt.Errorf("invalid NAL length size %d", nalLengthSize)
	}

	offset := 0
	for offset+nalLengthSize <= len(sample) {
		nalSize := readNALUnitSize(sample[offset:offset+nalLengthSize], nalLengthSize)
		offset += nalLengthSize
		if nalSize <= 0 || offset+nalSize > len(sample) {
			return fmt.Errorf("invalid NAL unit size")
		}

		nalType := sample[offset] & 0x1f
		if nalType >= 1 && nalType <= 5 {
			if nalType != 5 {
				return fmt.Errorf("sync sample starts with non-IDR VCL NAL type %d", nalType)
			}
			return nil
		}

		offset += nalSize
	}

	return fmt.Errorf("sync sample does not contain a VCL NAL")
}

func readNALUnitSize(data []byte, nalLengthSize int) int {
	switch nalLengthSize {
	case 1:
		return int(data[0])
	case 2:
		return int(binary.BigEndian.Uint16(data))
	case 3:
		return int(uint32(data[0])<<16 | uint32(data[1])<<8 | uint32(data[2]))
	default:
		return int(binary.BigEndian.Uint32(data))
	}
}

func isSyncSample(flags uint32) bool {
	return flags&0x00010000 == 0
}

func readFullBoxFlags(payload []byte) uint32 {
	return uint32(payload[1])<<16 | uint32(payload[2])<<8 | uint32(payload[3])
}

func findDirectChildBox(data []byte, start int, end int, typ string) (mp4Box, bool, error) {
	boxes, err := listDirectChildBoxes(data, start, end)
	if err != nil {
		return mp4Box{}, false, err
	}
	for _, box := range boxes {
		if box.Type == typ {
			return box, true, nil
		}
	}
	return mp4Box{}, false, nil
}

func listDirectChildBoxes(data []byte, start int, end int) ([]mp4Box, error) {
	boxes := make([]mp4Box, 0, 8)
	offset := start
	for offset < end {
		box, nextOffset, err := readBox(data, offset, end)
		if err != nil {
			return nil, err
		}
		boxes = append(boxes, box)
		offset = nextOffset
	}
	return boxes, nil
}

func readBox(data []byte, start int, end int) (mp4Box, int, error) {
	if start < 0 || end > len(data) || start+8 > end {
		return mp4Box{}, 0, fmt.Errorf("invalid MP4 box bounds")
	}

	size32 := binary.BigEndian.Uint32(data[start : start+4])
	box := mp4Box{
		Type:  string(data[start+4 : start+8]),
		Start: start,
	}

	headerSize := 8
	boxSize := int64(size32)
	switch size32 {
	case 0:
		boxSize = int64(end - start)
	case 1:
		if start+16 > end {
			return mp4Box{}, 0, fmt.Errorf("invalid extended MP4 box")
		}
		boxSize = int64(binary.BigEndian.Uint64(data[start+8 : start+16]))
		headerSize = 16
	}

	if boxSize < int64(headerSize) {
		return mp4Box{}, 0, fmt.Errorf("invalid MP4 box size")
	}

	boxEnd := start + int(boxSize)
	if boxEnd > end {
		return mp4Box{}, 0, fmt.Errorf("MP4 box exceeds parent bounds")
	}

	box.End = boxEnd
	box.PayloadStart = start + headerSize
	return box, boxEnd, nil
}
