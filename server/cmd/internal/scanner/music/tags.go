package music

import (
	"regexp"
	"strconv"
	"strings"

	"igloo/cmd/internal/ffprobe"
)

// trackTags are the tags a track keeps for its album's and artists' votes and
// for later provider lookups; see music_track_metadata.
type trackTags struct {
	// artists holds the ARTISTS values when the tag has two or more. ffprobe
	// keeps only the first value of an ID3 or MP4 multi-value tag, so a lone
	// value says nothing about the credits and is ignored.
	artists                                []string
	recordingID, releaseID, releaseGroupID string
	// artistIDs and albumArtistIDs are in credit order, or nil when any entry
	// is not a MusicBrainz id, since one bad entry shifts every position.
	artistIDs, albumArtistIDs []string
	isrc, label               string
	compilation               bool
	// trackTotal and discTotal are 0 when untagged.
	trackTotal, discTotal int64
}

var musicBrainzIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// multiValueSeparator is how ffprobe joins the values of a Vorbis comment
// that appears more than once.
const multiValueSeparator = ";"

func readTrackTags(tags ffprobe.FormatTags) trackTags {
	read := trackTags{
		recordingID:    musicBrainzID(tags.MusicBrainzRecordingID),
		releaseID:      musicBrainzID(tags.MusicBrainzReleaseID),
		releaseGroupID: musicBrainzID(tags.MusicBrainzReleaseGroupID),
		artistIDs:      musicBrainzIDs(tags.MusicBrainzArtistID),
		albumArtistIDs: musicBrainzIDs(tags.MusicBrainzAlbumArtistID),
		isrc:           strings.ToUpper(strings.TrimSpace(tags.ISRC)),
		label:          strings.TrimSpace(tags.Label),
		compilation:    tagIsTrue(tags.Compilation),
	}

	artists := splitMultiValue(tags.Artists)
	if len(artists) >= 2 {
		read.artists = artists
	}

	return read
}

// musicBrainzID returns the first of a tag's values when it is a MusicBrainz
// id, lowercased, and "" otherwise.
func musicBrainzID(value string) string {
	ids := musicBrainzIDs(value)
	if len(ids) == 0 {
		return ""
	}
	return ids[0]
}

// musicBrainzIDs splits a tag on ';' (ffprobe's Vorbis join) and '/' (how
// ID3v2.3 taggers join multiple values); neither appears in an id.
func musicBrainzIDs(value string) []string {
	fields := strings.FieldsFunc(value, func(r rune) bool { return r == ';' || r == '/' })
	ids := make([]string, 0, len(fields))
	for _, field := range fields {
		id := strings.ToLower(strings.TrimSpace(field))
		if id == "" {
			continue
		}
		valid := musicBrainzIDPattern.MatchString(id)
		if !valid {
			return nil
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil
	}
	return ids
}

func splitMultiValue(value string) []string {
	values := []string{}
	for raw := range strings.SplitSeq(value, multiValueSeparator) {
		part := strings.TrimSpace(raw)
		if part != "" {
			values = append(values, part)
		}
	}
	return values
}

func tagIsTrue(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes":
		return true
	default:
		return false
	}
}

// tagTotal reads a separate TRACKTOTAL or DISCTOTAL value; a malformed or
// negative one reads as 0, which means untagged.
func tagTotal(value string) int64 {
	total, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0
	}
	if total < 0 {
		return 0
	}
	return total
}

// joinTagList stores a list as its NULL-able ';'-joined column value.
func joinTagList(values []string) string {
	return strings.Join(values, multiValueSeparator)
}
