// Package subtitletestutil writes subtitle fixtures for tests that run the real
// FFmpeg: SubRip text, and a hand-built PGS (Blu-ray) stream, since FFmpeg can
// encode image-based subtitles only from other image-based subtitles.
package subtitletestutil

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Cue is one subtitle shown from Start to End, in seconds.
type Cue struct {
	Start float64
	End   float64
	Text  string
}

// FixtureCues are the cues the fixtures share. The second straddles 3.5 s, a
// session start the tests use, and the gaps leave room for starts between cues.
var FixtureCues = []Cue{
	{Start: 1.0, End: 2.5, Text: "One"},
	{Start: 3.0, End: 4.5, Text: "Straddle"},
	{Start: 6.0, End: 7.0, Text: "Two"},
	{Start: 8.0, End: 9.5, Text: "Three"},
}

// PGS segment types.
const (
	pgsPaletteSegment      = 0x14
	pgsObjectSegment       = 0x15
	pgsCompositionSegment  = 0x16
	pgsWindowSegment       = 0x17
	pgsEndOfDisplaySegment = 0x80
)

const (
	pgsVideoWidth   = 320
	pgsVideoHeight  = 180
	pgsObjectWidth  = 64
	pgsObjectHeight = 16
	pgsObjectX      = 10
	pgsObjectY      = 10
)

// WriteSubRip writes cues as a SubRip file.
func WriteSubRip(path string, cues []Cue) error {
	var out strings.Builder
	for i, cue := range cues {
		fmt.Fprintf(&out, "%d\n%s --> %s\n%s\n\n", i+1, subRipTime(cue.Start), subRipTime(cue.End), cue.Text)
	}
	return os.WriteFile(path, []byte(out.String()), 0o644)
}

func subRipTime(seconds float64) string {
	millis := int64(seconds*1000 + 0.5)
	return fmt.Sprintf("%02d:%02d:%02d,%03d", millis/3_600_000, millis/60_000%60, millis/1000%60, millis%1000)
}

// WritePGS writes cues as a PGS .sup stream for a 320x180 picture: each cue
// is a display set showing a solid 64x16 box, and a later display set clears
// it. The text is not drawn.
func WritePGS(path string, cues []Cue) error {
	var out bytes.Buffer
	composition := uint16(0)
	for _, cue := range cues {
		writePGSShow(&out, pgsTicks(cue.Start), composition)
		composition++
		writePGSClear(&out, pgsTicks(cue.End), composition)
		composition++
	}
	return os.WriteFile(path, out.Bytes(), 0o644)
}

func pgsTicks(seconds float64) uint32 {
	return uint32(seconds*90_000 + 0.5)
}

func writePGSShow(out *bytes.Buffer, pts uint32, composition uint16) {
	composite := pgsComposition(composition, 0x80, 1)
	composite = binary.BigEndian.AppendUint16(composite, 0) // object id
	composite = append(composite, 0, 0)                     // window id, not cropped
	composite = binary.BigEndian.AppendUint16(composite, pgsObjectX)
	composite = binary.BigEndian.AppendUint16(composite, pgsObjectY)
	writePGSSegment(out, pts, pgsCompositionSegment, composite)
	writePGSSegment(out, pts, pgsWindowSegment, pgsWindow())
	// Palette 0, version 0, entry 1: opaque white in Y Cr Cb.
	writePGSSegment(out, pts, pgsPaletteSegment, []byte{0, 0, 1, 235, 128, 128, 255})

	var picture []byte
	picture = binary.BigEndian.AppendUint16(picture, pgsObjectWidth)
	picture = binary.BigEndian.AppendUint16(picture, pgsObjectHeight)
	for range pgsObjectHeight {
		// The long-run code (0x00, 0b11 then a 14-bit length, color) for a
		// line of color 1, then end of line.
		picture = append(picture, 0x00, 0xC0|byte(pgsObjectWidth>>8), byte(pgsObjectWidth), 1, 0x00, 0x00)
	}
	object := []byte{0, 0, 0, 0xC0} // object id, version, first and last fragment
	length := uint32(len(picture))
	object = append(object, byte(length>>16), byte(length>>8), byte(length))
	object = append(object, picture...)
	writePGSSegment(out, pts, pgsObjectSegment, object)
	writePGSSegment(out, pts, pgsEndOfDisplaySegment, nil)
}

func writePGSClear(out *bytes.Buffer, pts uint32, composition uint16) {
	writePGSSegment(out, pts, pgsCompositionSegment, pgsComposition(composition, 0x00, 0))
	writePGSSegment(out, pts, pgsWindowSegment, pgsWindow())
	writePGSSegment(out, pts, pgsEndOfDisplaySegment, nil)
}

// pgsComposition starts a composition segment: picture size, frame rate,
// composition number and state, no palette update, palette 0, and the object
// count.
func pgsComposition(composition uint16, state byte, objects byte) []byte {
	var segment []byte
	segment = binary.BigEndian.AppendUint16(segment, pgsVideoWidth)
	segment = binary.BigEndian.AppendUint16(segment, pgsVideoHeight)
	segment = append(segment, 0x10)
	segment = binary.BigEndian.AppendUint16(segment, composition)
	return append(segment, state, 0, 0, objects)
}

func pgsWindow() []byte {
	window := []byte{1, 0} // one window, id 0
	window = binary.BigEndian.AppendUint16(window, pgsObjectX)
	window = binary.BigEndian.AppendUint16(window, pgsObjectY)
	window = binary.BigEndian.AppendUint16(window, pgsObjectWidth)
	return binary.BigEndian.AppendUint16(window, pgsObjectHeight)
}

// writePGSSegment writes one .sup segment: magic, presentation and decoding
// timestamps in 90 kHz ticks, type and length.
func writePGSSegment(out *bytes.Buffer, pts uint32, segmentType byte, payload []byte) {
	header := []byte{'P', 'G'}
	header = binary.BigEndian.AppendUint32(header, pts)
	header = binary.BigEndian.AppendUint32(header, 0)
	header = append(header, segmentType)
	header = binary.BigEndian.AppendUint16(header, uint16(len(payload)))
	out.Write(header)
	out.Write(payload)
}

// Fixtures are short media files carrying FixtureCues, made by BuildFixtures.
type Fixtures struct {
	// Matroska holds a 12 s video (stream 0), the cues as PGS (1), as SubRip
	// (2), and as DVD subtitles converted from the PGS (3).
	Matroska string
	// MP4 holds a 12 s video (stream 0) and the cues as mov_text (1).
	MP4 string
	// ShiftedMatroska is Matroska with every timestamp 2 s later, so its
	// start time is not zero.
	ShiftedMatroska string
}

// Stream indexes inside Fixtures.Matroska.
const (
	MatroskaPGSStream    = 1
	MatroskaSubRipStream = 2
	MatroskaDVDStream    = 3
	MP4MovTextStream     = 1
)

// BuildFixtures writes the fixture media into dir with the FFmpeg binary at
// ffmpegPath. -copyts keeps the cues at their FixtureCues times.
func BuildFixtures(ffmpegPath string, dir string) (Fixtures, error) {
	supPath := filepath.Join(dir, "cues.sup")
	srtPath := filepath.Join(dir, "cues.srt")
	fixtures := Fixtures{
		Matroska:        filepath.Join(dir, "fixture movie.mkv"),
		MP4:             filepath.Join(dir, "fixture movie.mp4"),
		ShiftedMatroska: filepath.Join(dir, "shifted movie.mkv"),
	}

	err := WritePGS(supPath, FixtureCues)
	if err != nil {
		return Fixtures{}, err
	}
	err = WriteSubRip(srtPath, FixtureCues)
	if err != nil {
		return Fixtures{}, err
	}

	video := []string{"-f", "lavfi", "-i", "testsrc2=size=320x180:rate=24:duration=12"}
	h264 := []string{"-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p", "-g", "48"}
	commands := [][]string{
		concat(video, []string{"-i", supPath, "-i", srtPath,
			"-map", "0:v", "-map", "1:s", "-map", "2:s", "-map", "1:s"}, h264, []string{
			"-c:s:0", "copy", "-c:s:1", "copy", "-c:s:2", "dvdsub",
			"-metadata:s:s:0", "language=eng", "-metadata:s:s:1", "language=spa", "-metadata:s:s:2", "language=fre",
			"-copyts", fixtures.Matroska}),
		concat(video, []string{"-i", srtPath, "-map", "0:v", "-map", "1:s"}, h264, []string{
			"-c:s", "mov_text", "-metadata:s:s:0", "language=ger", "-copyts", fixtures.MP4}),
		{"-i", fixtures.Matroska, "-map", "0", "-c", "copy", "-output_ts_offset", "2", fixtures.ShiftedMatroska},
	}
	for _, args := range commands {
		output, runErr := exec.Command(ffmpegPath, concat([]string{"-y", "-v", "error", "-nostdin"}, args)...).CombinedOutput()
		if runErr != nil {
			return Fixtures{}, fmt.Errorf("ffmpeg %q: %w: %s", args, runErr, strings.TrimSpace(string(output)))
		}
	}
	return fixtures, nil
}

func concat(parts ...[]string) []string {
	var out []string
	for _, part := range parts {
		out = append(out, part...)
	}
	return out
}

// PacketTimes lists the presentation times of one stream's packets in seconds,
// read with the ffprobe binary at ffprobePath.
func PacketTimes(ffprobePath string, path string, streamIndex int) ([]float64, error) {
	output, err := exec.Command(ffprobePath, "-v", "error",
		"-select_streams", strconv.Itoa(streamIndex),
		"-show_entries", "packet=pts_time", "-of", "csv=p=0", path).Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe packets of %s: %w", path, err)
	}
	var times []float64
	for _, line := range strings.Fields(string(output)) {
		value, parseErr := strconv.ParseFloat(strings.TrimSuffix(line, ","), 64)
		if parseErr != nil {
			return nil, fmt.Errorf("ffprobe packet time %q: %w", line, parseErr)
		}
		times = append(times, value)
	}
	return times, nil
}

// StreamLanguage reads one stream's language tag with the ffprobe binary at
// ffprobePath; "" when it has none.
func StreamLanguage(ffprobePath string, path string, streamIndex int) (string, error) {
	output, err := exec.Command(ffprobePath, "-v", "error",
		"-select_streams", strconv.Itoa(streamIndex),
		"-show_entries", "stream_tags=language", "-of", "csv=p=0", path).Output()
	if err != nil {
		return "", fmt.Errorf("ffprobe language of %s: %w", path, err)
	}
	return strings.TrimSpace(string(output)), nil
}
