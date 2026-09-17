package scanner

import (
	"errors"
	"testing"

	"igloo/cmd/internal/ffprobe"
)

func TestClassifyStreamsSkipsArtworkAndConvertsFields(t *testing.T) {
	streams := []ffprobe.Stream{
		{Index: 0, CodecType: "video", CodecName: "mjpeg", Disposition: ffprobe.StreamDisposition{AttachedPic: 1}},
		{Index: 1, CodecType: "video", CodecName: "png"},
		{Index: 2, CodecType: "video", CodecName: "hevc", Profile: "Main 10", Level: 153, BitDepth: "10", Width: 1920, Height: 1080, CodedWidth: 1920, CodedHeight: 1088, FrameRate: "24000/1001", SideDataList: []ffprobe.StreamSideData{{SideDataType: "Display Matrix", Rotation: 0}}},
		{Index: 3, CodecType: "video", CodecName: "h264", BitDepth: "not a number"},
		{Index: 4, CodecType: "audio", CodecName: "aac", SampleRate: "48000", Channels: 6, Disposition: ffprobe.StreamDisposition{Default: 1}},
		{Index: 5, CodecType: "subtitle", CodecName: "subrip", Disposition: ffprobe.StreamDisposition{Forced: 1}},
		{Index: 6, CodecType: "data", CodecName: "bin_data"},
	}
	var videos []VideoStreamFields
	var audios []AudioStreamFields
	var subtitles []SubtitleFields
	count, err := ClassifyStreams(streams,
		func(v VideoStreamFields) error { videos = append(videos, v); return nil },
		func(a AudioStreamFields) error { audios = append(audios, a); return nil },
		func(s SubtitleFields) error { subtitles = append(subtitles, s); return nil })
	if err != nil || count != 2 || len(videos) != 2 || len(audios) != 1 || len(subtitles) != 1 {
		t.Fatalf("count=%d videos=%d audios=%d subtitles=%d err=%v", count, len(videos), len(audios), len(subtitles), err)
	}
	hevc := videos[0]
	if hevc.StreamIndex != 2 || hevc.CodecLevel.Int64 != 153 || hevc.BitDepth.Int64 != 10 || hevc.CodedHeight.Int64 != 1088 || !hevc.Rotation.Valid || hevc.Rotation.Int64 != 0 || hevc.FrameRate < 23.97 || hevc.FrameRate > 23.98 {
		t.Fatalf("hevc fields: %+v", hevc)
	}
	h264 := videos[1]
	if h264.CodecLevel.Valid || h264.BitDepth.Valid || h264.CodedWidth.Valid || h264.Rotation.Valid {
		t.Fatalf("absent metadata must persist as NULL: %+v", h264)
	}
	if audios[0].SampleRate.Int64 != 48000 || audios[0].Channels != 6 || !audios[0].IsDefault {
		t.Fatalf("audio fields: %+v", audios[0])
	}
	if !subtitles[0].IsForced || subtitles[0].IsDefault {
		t.Fatalf("subtitle fields: %+v", subtitles[0])
	}
}

func TestClassifyStreamsStopsOnFirstError(t *testing.T) {
	failure := errors.New("insert failed")
	calls := 0
	count, err := ClassifyStreams([]ffprobe.Stream{{CodecType: "video", CodecName: "h264"}, {CodecType: "video", CodecName: "h264"}},
		func(VideoStreamFields) error { calls++; return failure },
		func(AudioStreamFields) error { return nil },
		func(SubtitleFields) error { return nil })
	if !errors.Is(err, failure) || count != 0 || calls != 1 {
		t.Fatalf("count=%d calls=%d err=%v", count, calls, err)
	}
}
