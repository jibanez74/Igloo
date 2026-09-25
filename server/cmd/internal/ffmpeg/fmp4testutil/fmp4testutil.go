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
	boxHeaderSize = 8

	// nonIDRSampleFlags marks a sample whose first VCL NAL is not an IDR.
	// It deliberately leaves the sample_is_non_sync bit (0x00010000, see
	// isSyncSample in remux_validator.go) clear, so the validator reads these
	// samples as sync samples and rejects the fixture for starting a sync sample
	// on a non-IDR NAL. Setting the real non-sync bit would make the unsafe
	// fixture produce no sync samples at all and test a different path.
	nonIDRSampleFlags = 0x00000001
)

type Fixture struct {
	SafeVideo  bool
	AudioNoise bool
	Segments   int
}

type sampleSpec struct {
	data       []byte
	flags      uint32
	dataOffset int32
}

type fragmentTrack struct {
	trackID uint32
	sample  sampleSpec
}

func WriteHLSFixture(outDir string, fixture Fixture) error {
	if fixture.Segments <= 0 {
		return fmt.Errorf("segments must be positive")
	}

	videoSample := BuildVideoSample(fixture.SafeVideo)
	segments := make([][]byte, 0, fixture.Segments)
	for i := 0; i < fixture.Segments; i++ {
		segments = append(segments, BuildSegment(videoSample, fixture.AudioNoise))
	}
	err := WriteFragmentFiles(outDir, BuildInitMP4(), segments...)
	if err != nil {
		return err
	}

	playlist := buildEventPlaylist(fixture.Segments)
	return os.WriteFile(filepath.Join(outDir, helpers.HLS_PLAYLIST_FILENAME), []byte(playlist), 0o644)
}

// WriteFragmentFiles writes an init segment plus the supplied media segments
// under the names ValidateRemuxSafety reads, so tests can hand it segments
// they have corrupted on purpose.
func WriteFragmentFiles(outDir string, initData []byte, segments ...[]byte) error {
	err := os.WriteFile(filepath.Join(outDir, helpers.HLS_INIT_FILENAME), initData, 0o644)
	if err != nil {
		return err
	}
	for i, segment := range segments {
		name := fmt.Sprintf(
			"%s%d%s",
			helpers.HLS_SEGMENT_FILENAME_PREFIX,
			i,
			helpers.HLS_SEGMENT_FILENAME_SUFFIX,
		)
		err = os.WriteFile(filepath.Join(outDir, name), segment, 0o644)
		if err != nil {
			return err
		}
	}
	return nil
}

func BuildInitMP4() []byte {
	videoTrack := Box(
		"trak",
		tkhd(1),
		Box(
			"mdia",
			hdlr("vide"),
			Box(
				"minf",
				Box(
					"stbl",
					stsd(
						avcSampleEntry(
							"avc1",
							Box("avcC", []byte{1, 0x64, 0x00, 0x1f, 0xff}),
						),
					),
				),
			),
		),
	)

	audioTrack := Box(
		"trak",
		tkhd(2),
		Box("mdia", hdlr("soun")),
	)

	moov := Box("moov", mvhd(), videoTrack, audioTrack)
	return append(ftyp(), moov...)
}

func BuildSegment(videoSample []byte, includeAudioNoise bool) []byte {
	tracks := make([]fragmentTrack, 0, 2)
	mdatPayload := make([]byte, 0, len(videoSample)+3)

	if includeAudioNoise {
		audioSample := []byte{0xaa, 0xbb, 0xcc}
		tracks = append(tracks, fragmentTrack{
			trackID: 2,
			sample: sampleSpec{
				data:  audioSample,
				flags: 0,
			},
		})
		mdatPayload = append(mdatPayload, audioSample...)
	}

	tracks = append(tracks, fragmentTrack{
		trackID: 1,
		sample: sampleSpec{
			data:  videoSample,
			flags: videoSampleFlags(videoSample),
		},
	})
	mdatPayload = append(mdatPayload, videoSample...)

	mfhdBox := mfhd()
	moofSize := boxHeaderSize + len(mfhdBox) + len(tracks)*trafSize()

	trafsWithOffsets := make([][]byte, 0, len(tracks))
	currentOffset := moofSize + boxHeaderSize
	for _, track := range tracks {
		sample := track.sample
		sample.dataOffset = int32(currentOffset)
		trafsWithOffsets = append(trafsWithOffsets, Box(
			"traf",
			tfhd(track.trackID),
			tfdt(),
			trun(sample),
		))
		currentOffset += len(sample.data)
	}

	moof := Box("moof", append([][]byte{mfhdBox}, trafsWithOffsets...)...)
	mdat := Box("mdat", mdatPayload)

	return append(moof, mdat...)
}

func BuildVideoSample(safe bool) []byte {
	sps := []byte{0x67, 0x64, 0x00, 0x1f}

	vcl := []byte{0x41, 0x9a, 0x22}
	if safe {
		vcl = []byte{0x65, 0x88, 0x84}
	}

	out := make([]byte, 0, 16)
	out = append(out, nalu(sps)...)
	out = append(out, nalu(vcl)...)
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

func nalu(payload []byte) []byte {
	out := make([]byte, 4+len(payload))
	binary.BigEndian.PutUint32(out[:4], uint32(len(payload)))
	copy(out[4:], payload)
	return out
}

func tkhd(trackID uint32) []byte {
	payload := make([]byte, 20)
	binary.BigEndian.PutUint32(payload[12:16], trackID)
	return Box("tkhd", payload)
}

func hdlr(handlerType string) []byte {
	payload := make([]byte, 12)
	copy(payload[8:12], []byte(handlerType))
	return Box("hdlr", payload)
}

func stsd(entry []byte) []byte {
	return Box("stsd", FullBoxPayload(0, U32(1)), entry)
}

func avcSampleEntry(typ string, childBoxes ...[]byte) []byte {
	header := make([]byte, 78)
	binary.BigEndian.PutUint16(header[6:8], 1)
	return Box(typ, append([][]byte{header}, childBoxes...)...)
}

func ftyp() []byte {
	payload := make([]byte, 16)
	copy(payload[0:4], []byte("isom"))
	binary.BigEndian.PutUint32(payload[4:8], 512)
	copy(payload[8:12], []byte("iso2"))
	copy(payload[12:16], []byte("avc1"))
	return Box("ftyp", payload)
}

func mvhd() []byte {
	payload := make([]byte, 84)
	binary.BigEndian.PutUint32(payload[4:8], 1000)
	binary.BigEndian.PutUint32(payload[12:16], 0x00010000)
	binary.BigEndian.PutUint16(payload[16:18], 0x0100)
	binary.BigEndian.PutUint32(payload[28:32], 0x00010000)
	binary.BigEndian.PutUint32(payload[36:40], 0x00010000)
	binary.BigEndian.PutUint32(payload[44:48], 0x40000000)
	binary.BigEndian.PutUint32(payload[80:84], 3)
	return Box("mvhd", payload)
}

// mfhd and tfdt write the single-fragment values the fixtures
// need: sequence number 0 and base media decode time 0.
func mfhd() []byte {
	return Box("mfhd", FullBoxPayload(0, U32(0)))
}

func tfdt() []byte {
	return Box("tfdt", FullBoxPayload(0, U32(0)))
}

func tfhd(trackID uint32) []byte {
	return Box("tfhd", FullBoxPayload(0, U32(trackID)))
}

func videoSampleFlags(sample []byte) uint32 {
	offset := 0
	for offset < len(sample) {
		if offset+4 > len(sample) {
			return nonIDRSampleFlags
		}

		naluLen := int(binary.BigEndian.Uint32(sample[offset : offset+4]))
		offset += 4

		if naluLen == 0 || offset+naluLen > len(sample) {
			return nonIDRSampleFlags
		}

		nalType := sample[offset] & 0x1F
		if nalType == 5 {
			return 0
		}

		offset += naluLen
	}

	return nonIDRSampleFlags
}

// trun writes a single-sample run with an explicit data offset, sample size
// and sample flags; trafSize is the size of the traf such a run produces.
func trun(sample sampleSpec) []byte {
	flags := uint32(0x000001 | 0x000200 | 0x000400)
	return Box("trun", FullBoxPayload(
		flags,
		U32(1),
		U32(uint32(sample.dataOffset)),
		U32(uint32(len(sample.data))),
		U32(sample.flags),
	))
}

func trafSize() int {
	trunSize := boxHeaderSize + 12 + 8
	return boxHeaderSize + len(tfhd(0)) + len(tfdt()) + trunSize
}

// FullBoxPayload builds an ISO-BMFF full-box payload: a zero version byte, the
// 24-bit flags field, then the supplied fields.
func FullBoxPayload(flags uint32, payloadParts ...[]byte) []byte {
	out := []byte{0, byte(flags >> 16), byte(flags >> 8), byte(flags)}
	for _, part := range payloadParts {
		out = append(out, part...)
	}
	return out
}

// U32 encodes value as a big-endian 32-bit field.
func U32(value uint32) []byte {
	out := make([]byte, 4)
	binary.BigEndian.PutUint32(out, value)
	return out
}

// U64 encodes value as a big-endian 64-bit field.
func U64(value uint64) []byte {
	out := make([]byte, 8)
	binary.BigEndian.PutUint64(out, value)
	return out
}

// Box builds an ISO-BMFF box with a 32-bit size header of the given type.
func Box(typ string, payloadParts ...[]byte) []byte {
	payloadLen := 0
	for _, part := range payloadParts {
		payloadLen += len(part)
	}

	out := make([]byte, boxHeaderSize+payloadLen)
	binary.BigEndian.PutUint32(out[:4], uint32(len(out)))
	copy(out[4:8], []byte(typ))

	offset := boxHeaderSize
	for _, part := range payloadParts {
		copy(out[offset:], part)
		offset += len(part)
	}

	return out
}
