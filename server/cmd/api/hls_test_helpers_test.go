package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"igloo/cmd/internal/ffmpeg"
	"igloo/cmd/internal/helpers"
)

type fakeFFmpegRunPlan struct {
	StartErr   error
	ExitErr    error
	WriteFiles func(outDir string) error
}

type fakeFFmpeg struct {
	mu    sync.Mutex
	plans []fakeFFmpegRunPlan
	calls []ffmpeg.HLSParams
}

func (f *fakeFFmpeg) RunHLS(
	_ context.Context,
	params ffmpeg.HLSParams,
	onExit func(exitErr error, stderrTail []string),
) (*exec.Cmd, error) {
	f.mu.Lock()
	callIndex := len(f.calls)
	f.calls = append(f.calls, params)

	if callIndex >= len(f.plans) {
		f.mu.Unlock()
		return nil, fmt.Errorf("unexpected RunHLS call %d", callIndex)
	}

	plan := f.plans[callIndex]
	f.mu.Unlock()

	if plan.StartErr != nil {
		return nil, plan.StartErr
	}

	if plan.WriteFiles != nil {
		err := plan.WriteFiles(params.OutDir)
		if err != nil {
			return nil, err
		}
	}

	if onExit != nil {
		onExit(plan.ExitErr, nil)
	}

	return &exec.Cmd{}, nil
}

func (f *fakeFFmpeg) ExtractSubtitleAsWebVTT(_ context.Context, _ string, _ int64) ([]byte, error) {
	return []byte("WEBVTT\n"), nil
}

func (f *fakeFFmpeg) CallCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func (f *fakeFFmpeg) Calls() []ffmpeg.HLSParams {
	f.mu.Lock()
	defer f.mu.Unlock()

	out := make([]ffmpeg.HLSParams, len(f.calls))
	copy(out, f.calls)
	return out
}

type testFMP4Fixture struct {
	SafeVideo  bool
	AudioNoise bool
	Segments   int
}

func writeTestHLSFixture(outDir string, fixture testFMP4Fixture) error {
	if fixture.Segments <= 0 {
		return fmt.Errorf("segments must be positive")
	}

	initData := buildTestInitMP4()
	err := os.WriteFile(filepath.Join(outDir, helpers.HLS_INIT_FILENAME), initData, 0644)
	if err != nil {
		return err
	}

	for i := 0; i < fixture.Segments; i++ {
		segmentData := buildTestSegment(buildTestVideoSample(fixture.SafeVideo), fixture.AudioNoise)
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

	playlist := buildTestEventPlaylist(fixture.Segments)
	return os.WriteFile(filepath.Join(outDir, "playlist.m3u8"), []byte(playlist), 0644)
}

func buildTestEventPlaylist(segments int) string {
	var builder strings.Builder
	builder.WriteString("#EXTM3U\n")
	builder.WriteString("#EXT-X-VERSION:7\n")
	builder.WriteString("#EXT-X-TARGETDURATION:4\n")
	builder.WriteString("#EXT-X-MEDIA-SEQUENCE:0\n")
	builder.WriteString("#EXT-X-PLAYLIST-TYPE:EVENT\n")
	builder.WriteString("#EXT-X-MAP:URI=\"init.mp4\"\n")
	for i := 0; i < segments; i++ {
		builder.WriteString("#EXTINF:4.000000,\n")
		builder.WriteString(fmt.Sprintf("segment_%d.m4s\n", i))
	}
	return builder.String()
}

func buildTestInitMP4() []byte {
	videoTrack := mp4TestBox(
		"trak",
		mp4TestTKHD(1),
		mp4TestBox("mdia", mp4TestHDLR("vide")),
		mp4TestBox("avcC", []byte{1, 0x64, 0x00, 0x1f, 0xff}),
	)

	audioTrack := mp4TestBox(
		"trak",
		mp4TestTKHD(2),
		mp4TestBox("mdia", mp4TestHDLR("soun")),
	)

	return mp4TestBox("moov", videoTrack, audioTrack)
}

func buildTestSegment(videoSample []byte, includeAudioNoise bool) []byte {
	videoRun := mp4TestTRUN([]testSampleSpec{{
		Data:  videoSample,
		Flags: 0,
	}})
	videoTraf := mp4TestBox(
		"traf",
		mp4TestTFHD(1),
		videoRun,
	)

	trafs := [][]byte{videoTraf}
	mdatPayload := append([]byte{}, videoSample...)

	if includeAudioNoise {
		audioSample := []byte{0xaa, 0xbb, 0xcc}
		audioRun := mp4TestTRUN([]testSampleSpec{{
			Data:  audioSample,
			Flags: 0,
		}})
		audioTraf := mp4TestBox(
			"traf",
			mp4TestTFHD(2),
			audioRun,
		)
		trafs = append([][]byte{audioTraf}, trafs...)
		mdatPayload = append(audioSample, mdatPayload...)
	}

	moofPayload := make([]byte, 0, 256)
	for _, traf := range trafs {
		moofPayload = append(moofPayload, traf...)
	}
	moofSize := 8 + len(moofPayload)

	trafsWithOffsets := make([][]byte, 0, len(trafs))
	currentOffset := moofSize + 8
	if includeAudioNoise {
		audioSample := []byte{0xaa, 0xbb, 0xcc}
		audioRun := mp4TestTRUN([]testSampleSpec{{
			Data:       audioSample,
			Flags:      0,
			DataOffset: int32(currentOffset),
		}})
		audioTraf := mp4TestBox(
			"traf",
			mp4TestTFHD(2),
			audioRun,
		)
		trafsWithOffsets = append(trafsWithOffsets, audioTraf)
		currentOffset += len(audioSample)
	}

	videoRun = mp4TestTRUN([]testSampleSpec{{
		Data:       videoSample,
		Flags:      0,
		DataOffset: int32(currentOffset),
	}})
	videoTraf = mp4TestBox(
		"traf",
		mp4TestTFHD(1),
		videoRun,
	)
	trafsWithOffsets = append(trafsWithOffsets, videoTraf)

	moofPayload = make([]byte, 0, 256)
	for _, traf := range trafsWithOffsets {
		moofPayload = append(moofPayload, traf...)
	}

	moof := mp4TestBox("moof", moofPayload)
	mdat := mp4TestBox("mdat", mdatPayload)

	return append(moof, mdat...)
}

func buildTestVideoSample(safe bool) []byte {
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

type testSampleSpec struct {
	Data       []byte
	Flags      uint32
	DataOffset int32
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

func mp4TestTFHD(trackID uint32) []byte {
	payload := make([]byte, 8)
	binary.BigEndian.PutUint32(payload[4:8], trackID)
	return mp4TestBox("tfhd", payload)
}

func mp4TestTRUN(samples []testSampleSpec) []byte {
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

func mp4TestBox(typ string, payloadParts ...[]byte) []byte {
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
