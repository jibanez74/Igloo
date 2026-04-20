package fmp4testutil

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"igloo/cmd/internal/helpers"
)

const (
	mp4TestBoxHeaderSize      = 8
	mp4TestNonSyncSampleFlag  = 0x00000001
)

type Fixture struct {
	SafeVideo  bool
	AudioNoise bool
	Segments   int
}

type sampleSpec struct {
	Data       []byte
	Flags      uint32
	DataOffset int32
}

type fragmentTrack struct {
	trackID uint32
	sample  sampleSpec
}

func WriteHLSFixture(outDir string, fixture Fixture) error {
	if fixture.Segments <= 0 {
		return fmt.Errorf("segments must be positive")
	}

	initData := BuildInitMP4()
	err := os.WriteFile(filepath.Join(outDir, helpers.HLS_INIT_FILENAME), initData, 0644)
	if err != nil {
		return err
	}

	videoSample := BuildVideoSample(fixture.SafeVideo)
	for i := 0; i < fixture.Segments; i++ {
		segmentData := BuildSegment(videoSample, fixture.AudioNoise)
		name := fmt.Sprintf(
			"%s%d%s",
			helpers.HLS_SEGMENT_FILENAME_PREFIX,
			i,
			helpers.HLS_SEGMENT_FILENAME_SUFFIX,
		)
		err = os.WriteFile(filepath.Join(outDir, name), segmentData, 0644)
		if err != nil {
			return err
		}
	}

	playlist := buildEventPlaylist(fixture.Segments)
	return os.WriteFile(filepath.Join(outDir, "playlist.m3u8"), []byte(playlist), 0644)
}

func BuildInitMP4() []byte {
	videoTrack := mp4TestBox(
		"trak",
		mp4TestTKHD(1),
		mp4TestBox(
			"mdia",
			mp4TestHDLR("vide"),
			mp4TestBox(
				"minf",
				mp4TestBox(
					"stbl",
					mp4TestSTSD(
						mp4TestAVCSampleEntry(
							"avc1",
							mp4TestBox("avcC", []byte{1, 0x64, 0x00, 0x1f, 0xff}),
						),
					),
				),
			),
		),
	)

	audioTrack := mp4TestBox(
		"trak",
		mp4TestTKHD(2),
		mp4TestBox("mdia", mp4TestHDLR("soun")),
	)

	moov := mp4TestBox("moov", mp4TestMVHD(), videoTrack, audioTrack)
	return append(mp4TestFTYP(), moov...)
}

func BuildSegment(videoSample []byte, includeAudioNoise bool) []byte {
	tracks := make([]fragmentTrack, 0, 2)
	mdatPayload := make([]byte, 0, len(videoSample)+3)

	if includeAudioNoise {
		audioSample := []byte{0xaa, 0xbb, 0xcc}
		tracks = append(tracks, fragmentTrack{
			trackID: 2,
			sample: sampleSpec{
				Data:  audioSample,
				Flags: 0,
			},
		})
		mdatPayload = append(mdatPayload, audioSample...)
	}

	tracks = append(tracks, fragmentTrack{
		trackID: 1,
		sample: sampleSpec{
			Data:  videoSample,
			Flags: mp4TestVideoSampleFlags(videoSample),
		},
	})
	mdatPayload = append(mdatPayload, videoSample...)

	mfhd := mp4TestMFHD(0)
	moofSize := mp4TestBoxHeaderSize + len(mfhd)
	for range tracks {
		moofSize += mp4TestTrafSize(1)
	}

	trafsWithOffsets := make([][]byte, 0, len(tracks))
	currentOffset := moofSize + mp4TestBoxHeaderSize
	for _, track := range tracks {
		sample := track.sample
		sample.DataOffset = int32(currentOffset)
		trafsWithOffsets = append(trafsWithOffsets, mp4TestBox(
			"traf",
			mp4TestTFHD(track.trackID),
			mp4TestTFDT(0),
			mp4TestTRUN([]sampleSpec{sample}),
		))
		currentOffset += len(sample.Data)
	}

	moof := mp4TestBox("moof", append([][]byte{mfhd}, trafsWithOffsets...)...)
	mdat := mp4TestBox("mdat", mdatPayload)

	return append(moof, mdat...)
}

func BuildVideoSample(safe bool) []byte {
	sps := []byte{0x67, 0x64, 0x00, 0x1f}

	vcl := []byte{0x41, 0x9a, 0x22}
	if safe {
		vcl = []byte{0x65, 0x88, 0x84}
	}

	out := make([]byte, 0, 16)
	out = append(out, mp4TestNALU(sps)...)
	out = append(out, mp4TestNALU(vcl)...)
	return out
}

func buildEventPlaylist(segments int) string {
	var builder strings.Builder
	builder.WriteString("#EXTM3U\n")
	builder.WriteString("#EXT-X-VERSION:7\n")
	builder.WriteString("#EXT-X-TARGETDURATION:4\n")
	builder.WriteString("#EXT-X-MEDIA-SEQUENCE:0\n")
	builder.WriteString("#EXT-X-PLAYLIST-TYPE:EVENT\n")
	builder.WriteString(fmt.Sprintf("#EXT-X-MAP:URI=\"%s\"\n", helpers.HLS_INIT_FILENAME))
	for i := 0; i < segments; i++ {
		builder.WriteString("#EXTINF:4.000000,\n")
		builder.WriteString(fmt.Sprintf("%s%d%s\n", helpers.HLS_SEGMENT_FILENAME_PREFIX, i, helpers.HLS_SEGMENT_FILENAME_SUFFIX))
	}
	return builder.String()
}

func mp4TestNALU(payload []byte) []byte {
	out := make([]byte, 4+len(payload))
	binary.BigEndian.PutUint32(out[:4], uint32(len(payload)))
	copy(out[4:], payload)
	return out
}

func mp4TestTKHD(trackID uint32) []byte {
	payload := make([]byte, 20)
	binary.BigEndian.PutUint32(payload[12:16], trackID)
	return mp4TestBox("tkhd", payload)
}

func mp4TestHDLR(handlerType string) []byte {
	payload := make([]byte, 12)
	copy(payload[8:12], []byte(handlerType))
	return mp4TestBox("hdlr", payload)
}

func mp4TestSTSD(entries ...[]byte) []byte {
	payload := make([]byte, 8)
	binary.BigEndian.PutUint32(payload[4:8], uint32(len(entries)))
	return mp4TestBox("stsd", append([][]byte{payload}, entries...)...)
}

func mp4TestAVCSampleEntry(typ string, childBoxes ...[]byte) []byte {
	header := make([]byte, 78)
	binary.BigEndian.PutUint16(header[6:8], 1)
	return mp4TestBox(typ, append([][]byte{header}, childBoxes...)...)
}

func mp4TestFTYP() []byte {
	payload := make([]byte, 16)
	copy(payload[0:4], []byte("isom"))
	binary.BigEndian.PutUint32(payload[4:8], 512)
	copy(payload[8:12], []byte("iso2"))
	copy(payload[12:16], []byte("avc1"))
	return mp4TestBox("ftyp", payload)
}

func mp4TestMVHD() []byte {
	payload := make([]byte, 84)
	payload[0] = 0
	binary.BigEndian.PutUint32(payload[4:8], 1000)
	binary.BigEndian.PutUint32(payload[12:16], 0x00010000)
	binary.BigEndian.PutUint16(payload[16:18], 0x0100)
	binary.BigEndian.PutUint32(payload[28:32], 0x00010000)
	binary.BigEndian.PutUint32(payload[36:40], 0x00010000)
	binary.BigEndian.PutUint32(payload[44:48], 0x40000000)
	binary.BigEndian.PutUint32(payload[80:84], 3)
	return mp4TestBox("mvhd", payload)
}

func mp4TestMFHD(sequenceNumber uint32) []byte {
	payload := make([]byte, 8)
	payload[0] = 0
	binary.BigEndian.PutUint32(payload[4:8], sequenceNumber)
	return mp4TestBox("mfhd", payload)
}

func mp4TestTFDT(baseMediaDecodeTime uint32) []byte {
	payload := make([]byte, 8)
	payload[0] = 0
	binary.BigEndian.PutUint32(payload[4:8], baseMediaDecodeTime)
	return mp4TestBox("tfdt", payload)
}

func mp4TestTFHD(trackID uint32) []byte {
	payload := make([]byte, 8)
	binary.BigEndian.PutUint32(payload[4:8], trackID)
	return mp4TestBox("tfhd", payload)
}

func mp4TestVideoSampleFlags(sample []byte) uint32 {
	offset := 0
	for offset < len(sample) {
		if offset+4 > len(sample) {
			return mp4TestNonSyncSampleFlag
		}

		naluLen := int(binary.BigEndian.Uint32(sample[offset : offset+4]))
		offset += 4

		if naluLen == 0 || offset+naluLen > len(sample) {
			return mp4TestNonSyncSampleFlag
		}

		nalType := sample[offset] & 0x1F
		if nalType == 5 {
			return 0
		}

		offset += naluLen
	}

	return mp4TestNonSyncSampleFlag
}

func mp4TestTRUN(samples []sampleSpec) []byte {
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
		binary.BigEndian.PutUint32(flagBytes, sample.Flags)
		payload = append(payload, flagBytes...)
	}

	return mp4TestBox("trun", payload)
}

func mp4TestTRUNSize(sampleCount int) int {
	return mp4TestBoxHeaderSize + 12 + sampleCount*8
}

func mp4TestTrafSize(sampleCount int) int {
	return mp4TestBoxHeaderSize + len(mp4TestTFHD(0)) + len(mp4TestTFDT(0)) + mp4TestTRUNSize(sampleCount)
}

func mp4TestBox(typ string, payloadParts ...[]byte) []byte {
	payloadLen := 0
	for _, part := range payloadParts {
		payloadLen += len(part)
	}

	out := make([]byte, mp4TestBoxHeaderSize+payloadLen)
	binary.BigEndian.PutUint32(out[:4], uint32(len(out)))
	copy(out[4:8], []byte(typ))

	offset := mp4TestBoxHeaderSize
	for _, part := range payloadParts {
		copy(out[offset:], part)
		offset += len(part)
	}

	return out
}
