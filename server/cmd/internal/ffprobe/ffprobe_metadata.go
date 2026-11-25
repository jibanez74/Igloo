package ffprobe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const metadataTimeout = 60 * time.Second

type FfprobeResult struct {
	Streams  []Stream  `json:"streams"`
	Format   Format    `json:"format"`
	Chapters []Chapter `json:"chapters"`
}

type Stream struct {
	Index         int    `json:"index"`
	CodecName     string `json:"codec_name"`
	CodecType     string `json:"codec_type"`
	Profile       string `json:"profile"`
	BitRate       string `json:"bit_rate"`
	SampleRate    string `json:"sample_rate"`
	Channels      int    `json:"channels"`
	ChannelLayout string `json:"channel_layout"`

	// Video-specific fields
	Width          int              `json:"width"`
	Height         int              `json:"height"`
	CodedWidth     int              `json:"coded_width"`
	CodedHeight    int              `json:"coded_height"`
	AspectRatio    string           `json:"display_aspect_ratio"`
	Level          int              `json:"level"`
	AvgFrameRate   string           `json:"avg_frame_rate"`
	FrameRate      string           `json:"r_frame_rate"`
	BitDepth       string           `json:"bits_per_raw_sample"`
	PixelFormat    string           `json:"pix_fmt"`
	ColorRange     string           `json:"color_range"`
	ColorTransfer  string           `json:"color_transfer"`
	ColorPrimaries string           `json:"color_primaries"`
	ColorSpace     string           `json:"color_space"`
	FieldOrder     string           `json:"field_order"`
	SideDataList   []StreamSideData `json:"side_data_list"`

	Tags        StreamTags        `json:"tags"`
	Disposition StreamDisposition `json:"disposition"`
}

type StreamSideData struct {
	SideDataType string `json:"side_data_type"`
	Rotation     int64  `json:"rotation"`
	// Set on the "DOVI configuration record" entry.
	DVProfile                 int64 `json:"dv_profile"`
	DVBLSignalCompatibilityID int64 `json:"dv_bl_signal_compatibility_id"`
}

// Rotation returns the display-matrix rotation in degrees and whether a
// display matrix was present at all, so callers can distinguish an explicit
// 0-degree matrix from no rotation side data.
func (s Stream) Rotation() (int64, bool) {
	for _, sideData := range s.SideDataList {
		if sideData.SideDataType == "Display Matrix" {
			return sideData.Rotation, true
		}
	}
	return 0, false
}

// DolbyVision returns the stream's Dolby Vision profile and base-layer
// compatibility id, and whether it carries a configuration record at all, so
// callers can tell a compatibility id of 0 from no Dolby Vision.
func (s Stream) DolbyVision() (int64, int64, bool) {
	for _, sideData := range s.SideDataList {
		if sideData.SideDataType == "DOVI configuration record" {
			return sideData.DVProfile, sideData.DVBLSignalCompatibilityID, true
		}
	}
	return 0, 0, false
}

// StreamDisposition carries only the flags Igloo acts on: cover-art detection
// and the default/forced track hints that reach the database and direct-play
// eligibility. ffprobe reports several more; decoding them earns nothing.
type StreamDisposition struct {
	AttachedPic int `json:"attached_pic"`
	Default     int `json:"default"`
	Forced      int `json:"forced"`
}

type StreamTags struct {
	Title    string `json:"title"`
	Language string `json:"language"`
	// Comment names an attached picture's role in MP3 and FLAC files, such as
	// "Cover (front)".
	Comment string `json:"comment"`
	// Music is the stream's full music tag set: Opus and Ogg Vorbis files
	// carry every tag on their audio stream and none on the container.
	Music FormatTags `json:"-"`
}

// UnmarshalJSON normalizes stream tag keys the same way FormatTags does, so
// Matroska muxers that write TITLE/LANGUAGE (or the "lang" alias) still
// produce labelled, preference-matchable streams.
func (t *StreamTags) UnmarshalJSON(data []byte) error {
	values, err := decodeTagValues(data)
	if err != nil {
		return err
	}

	t.Title = firstTagValue(values, "title")
	t.Language = firstTagValue(values, "language", "lang")
	t.Comment = firstTagValue(values, "comment")
	t.Music = formatTagsFrom(values)

	return nil
}

type Format struct {
	Duration string `json:"duration"`
	// StartTime is the container's start, which FFmpeg subtracts from every
	// timestamp it reads unless told to keep them.
	StartTime string     `json:"start_time"`
	BitRate   string     `json:"bit_rate"`
	Tags      FormatTags `json:"tags"`
}

type FormatTags struct {
	Title       string `json:"title"`
	Artist      string `json:"artist"`
	AlbumArtist string `json:"album_artist"`
	Composer    string `json:"composer"`
	Album       string `json:"album"`
	Genre       string `json:"genre"`
	Track       string `json:"track"`
	Disc        string `json:"disc"`
	Date        string `json:"date"`
	Copyright   string `json:"copyright"`
	SortName    string `json:"sort_name"`
	SortAlbum   string `json:"sort_album"`
	SortArtist  string `json:"sort_artist"`
	Language    string `json:"language"`
	// The fields below are music-only. Key spellings differ by container
	// (Vorbis MUSICBRAINZ_ALBUMID, ID3 TXXX "MusicBrainz Album Id", MP4
	// freeform atoms), and normalizeTagKey makes each one key.
	Compilation string `json:"compilation"`
	TrackTotal  string `json:"track_total"`
	DiscTotal   string `json:"disc_total"`
	ISRC        string `json:"isrc"`
	Label       string `json:"label"`
	// Artists is the multi-value ARTISTS tag, which ffprobe joins with ";"
	// for Vorbis comments and cuts to its first value for ID3 and MP4.
	Artists                   string `json:"artists"`
	MusicBrainzReleaseID      string `json:"musicbrainz_release_id"`
	MusicBrainzReleaseGroupID string `json:"musicbrainz_release_group_id"`
	MusicBrainzArtistID       string `json:"musicbrainz_artist_id"`
	MusicBrainzAlbumArtistID  string `json:"musicbrainz_album_artist_id"`
	MusicBrainzRecordingID    string `json:"musicbrainz_recording_id"`
}

func (t *FormatTags) UnmarshalJSON(data []byte) error {
	values, err := decodeTagValues(data)
	if err != nil {
		return err
	}

	*t = formatTagsFrom(values)

	return nil
}

func formatTagsFrom(values map[string]string) FormatTags {
	return FormatTags{
		Title:       firstTagValue(values, "title"),
		Artist:      firstTagValue(values, "artist"),
		AlbumArtist: firstTagValue(values, "album_artist"),
		Composer:    firstTagValue(values, "composer"),
		Album:       firstTagValue(values, "album"),
		Genre:       firstTagValue(values, "genre"),
		Track:       firstTagValue(values, "track", "tracknumber"),
		Disc:        firstTagValue(values, "disc", "discnumber"),
		Date:        firstTagValue(values, "date", "year"),
		Copyright:   firstTagValue(values, "copyright"),
		SortName:    firstTagValue(values, "sort_name", "titlesort"),
		SortAlbum:   firstTagValue(values, "sort_album", "albumsort"),
		SortArtist:  firstTagValue(values, "sort_artist", "artistsort"),
		Language:    firstTagValue(values, "language", "lang"),
		Compilation: firstTagValue(values, "compilation"),
		TrackTotal:  firstTagValue(values, "tracktotal", "totaltracks"),
		DiscTotal:   firstTagValue(values, "disctotal", "totaldiscs"),
		// ffprobe leaves the ID3 ISRC frame under its raw id, TSRC, and maps the
		// ID3 label frame (TPUB) to publisher; Vorbis also has ORGANIZATION.
		ISRC:                      firstTagValue(values, "isrc", "tsrc"),
		Label:                     firstTagValue(values, "label", "publisher", "organization"),
		Artists:                   firstTagValue(values, "artists"),
		MusicBrainzReleaseID:      firstTagValue(values, "musicbrainz_albumid"),
		MusicBrainzReleaseGroupID: firstTagValue(values, "musicbrainz_releasegroupid"),
		MusicBrainzArtistID:       firstTagValue(values, "musicbrainz_artistid"),
		MusicBrainzAlbumArtistID:  firstTagValue(values, "musicbrainz_albumartistid"),
		// Picard writes the recording id as MUSICBRAINZ_TRACKID in Vorbis and
		// MP4; in ID3 it goes into a UFID frame, which ffprobe does not expose.
		MusicBrainzRecordingID: firstTagValue(values, "musicbrainz_trackid"),
	}
}

// AudioTags returns the container's tags with each blank field filled from
// the first audio stream's tags, where Opus and Ogg Vorbis keep them.
func (r *FfprobeResult) AudioTags() FormatTags {
	tags := r.Format.Tags
	for _, stream := range r.Streams {
		if stream.CodecType != "audio" {
			continue
		}
		from := stream.Tags.Music
		fields := []struct {
			value    *string
			fallback string
		}{
			{&tags.Title, from.Title}, {&tags.Artist, from.Artist}, {&tags.AlbumArtist, from.AlbumArtist},
			{&tags.Composer, from.Composer}, {&tags.Album, from.Album}, {&tags.Genre, from.Genre},
			{&tags.Track, from.Track}, {&tags.Disc, from.Disc}, {&tags.Date, from.Date},
			{&tags.Copyright, from.Copyright}, {&tags.SortName, from.SortName}, {&tags.SortAlbum, from.SortAlbum},
			{&tags.SortArtist, from.SortArtist}, {&tags.Language, from.Language}, {&tags.Compilation, from.Compilation},
			{&tags.TrackTotal, from.TrackTotal}, {&tags.DiscTotal, from.DiscTotal}, {&tags.ISRC, from.ISRC},
			{&tags.Label, from.Label}, {&tags.Artists, from.Artists},
			{&tags.MusicBrainzReleaseID, from.MusicBrainzReleaseID}, {&tags.MusicBrainzReleaseGroupID, from.MusicBrainzReleaseGroupID},
			{&tags.MusicBrainzArtistID, from.MusicBrainzArtistID}, {&tags.MusicBrainzAlbumArtistID, from.MusicBrainzAlbumArtistID},
			{&tags.MusicBrainzRecordingID, from.MusicBrainzRecordingID},
		}
		for _, field := range fields {
			if *field.value == "" {
				*field.value = field.fallback
			}
		}
		break
	}
	return tags
}

func decodeTagValues(data []byte) (map[string]string, error) {
	var raw map[string]interface{}
	err := json.Unmarshal(data, &raw)
	if err != nil {
		return nil, err
	}

	values := make(map[string]string, len(raw))
	for key, value := range raw {
		text, ok := value.(string)
		if !ok {
			continue
		}

		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}

		values[key] = text
	}

	return values, nil
}

func normalizeTagKey(key string) string {
	key = strings.ToLower(strings.TrimSpace(key))
	key = strings.ReplaceAll(key, "_", "")
	key = strings.ReplaceAll(key, "-", "")
	key = strings.ReplaceAll(key, " ", "")
	return key
}

func firstTagValue(values map[string]string, keys ...string) string {
	for _, canonical := range keys {
		normalizedCanonical := normalizeTagKey(canonical)
		bestKey, bestValue, bestRank := "", "", 3
		for key, value := range values {
			normalizedKey := normalizeTagKey(key)
			if normalizedKey != normalizedCanonical {
				continue
			}
			rank := 2
			caseVariant := strings.EqualFold(key, canonical)
			if key == canonical {
				rank = 0
			} else if caseVariant {
				rank = 1
			}
			preferred := rank < bestRank || (rank == bestRank && key < bestKey)
			if preferred {
				bestKey, bestValue, bestRank = key, value, rank
			}
		}
		if bestValue != "" {
			return bestValue
		}
	}
	return ""
}

type ChapterTags struct {
	Title string `json:"title"`
}

type Chapter struct {
	StartTime string      `json:"start_time"`
	Tags      ChapterTags `json:"tags"`
}

func (f *ffprobe) GetMetadata(ctx context.Context, filePath string) (*FfprobeResult, error) {
	return f.runMetadata(ctx, filePath,
		"-v", "quiet",
		"-print_format", "json",
		"-show_streams",
		"-show_format",
		"-show_chapters",
		filePath,
	)
}

func (f *ffprobe) GetAudioMetadata(ctx context.Context, filePath string) (*FfprobeResult, error) {
	return f.runMetadata(ctx, filePath,
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		"-show_entries", "format=duration,bit_rate:format_tags:stream=index,codec_name,codec_type,profile,channels,channel_layout:stream_disposition=attached_pic:stream_tags",
		filePath,
	)
}

// runMetadata caps every probe at metadataTimeout while still honoring the
// caller's context, so a canceled library scan kills its in-flight ffprobe
// instead of leaving shutdown to wait out the timeout.
func (f *ffprobe) runMetadata(ctx context.Context, filePath string, args ...string) (*FfprobeResult, error) {
	if strings.TrimSpace(filePath) == "" {
		return nil, fmt.Errorf("file path is required")
	}

	probeCtx, cancel := context.WithTimeout(ctx, metadataTimeout)
	defer cancel()

	cmd := exec.CommandContext(probeCtx, f.bin, args...)

	output, err := cmd.Output()
	if err != nil {
		// A caller that went away and a probe that outlasted its own deadline are
		// different failures: only the latter says the file is slow to read.
		if ctx.Err() != nil {
			return nil, fmt.Errorf("ffprobe canceled for %s: %w", filePath, ctx.Err())
		}
		if probeCtx.Err() != nil {
			return nil, fmt.Errorf("ffprobe timed out for %s after %s: %w", filePath, metadataTimeout, probeCtx.Err())
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
			stderr := strings.TrimSpace(string(exitErr.Stderr))
			if stderr != "" {
				return nil, fmt.Errorf("ffprobe failed for %s: %w: %s", filePath, err, stderr)
			}
		}
		return nil, fmt.Errorf("ffprobe failed for %s: %w", filePath, err)
	}

	var result FfprobeResult

	err = json.Unmarshal(output, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ffprobe output for %s: %w", filePath, err)
	}

	if len(result.Streams) == 0 {
		return nil, fmt.Errorf("no streams found in %s", filePath)
	}

	return &result, nil
}
