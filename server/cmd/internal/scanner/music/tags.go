package music

import (
	"regexp"
	"strconv"
	"strings"

	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/scanner"
)

// trackTags are the tags a track keeps for its album's and artists' votes and
// for later provider lookups; see music_track_metadata.
type trackTags struct {
	// artists holds the ARTISTS values when the tag has two or more. ffprobe
	// keeps only the first value of an ID3 or MP4 multi-value tag, so a lone
	// value says nothing about the credits and is ignored as a credit; it is
	// kept as soleArtist, the artist the first id of such a list belongs to.
	artists                                []string
	soleArtist                             string
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

	artists, gap := splitMultiValue(tags.Artists)
	read.artists, read.soleArtist = artistsList(artists)
	// The usable names still credit the track, but an id list no longer
	// lines up with credits that skipped a blank slot.
	listed := len(read.artists) > 0
	if listed && gap {
		read.artistIDs = nil
	}

	return read
}

// artistsList reads an ARTISTS tag's names: two or more are the track's
// credits, and a single one is only its lead artist.
func artistsList(names []string) ([]string, string) {
	switch len(names) {
	case 0:
		return nil, ""
	case 1:
		return nil, names[0]
	default:
		return names, ""
	}
}

// storedArtists reads back the artists_tag column, the ARTISTS names joined.
func storedArtists(value string) ([]string, string) {
	names, _ := splitMultiValue(value)
	return artistsList(names)
}

// pairableArtistIDs returns the tags' artist ids when they line up with the
// track's credits by position, one each, and nil otherwise. A single ARTISTS
// value naming an artist other than the first credit says the ids follow
// another order, in any container. And ffprobe cuts an ID3 or MP4 list to its
// first id (a Vorbis comment list stays whole), so in those containers one id
// beside one credit that reads as several names ("A & B") may be only the
// first artist's. container is the track's.
func pairableArtistIDs(tags trackTags, container string, credits []resolvedMusician) []string {
	aligned := len(tags.artistIDs) == len(credits) && len(credits) > 0
	if !aligned {
		return nil
	}
	// The first id is the lead's, so a lead the ARTISTS tag names must be the
	// first credit.
	name := credits[0].name
	other := tags.soleArtist != "" && scanner.NormalizedScanCacheKey(tags.soleArtist) != scanner.NormalizedScanCacheKey(name)
	if other {
		return nil
	}
	lone := len(credits) == 1
	if !lone {
		return tags.artistIDs
	}
	cut := !vorbisCommentContainers[container]
	compound := len(compoundCredits(name).parts) >= 2
	if cut && compound {
		return nil
	}
	return tags.artistIDs
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
// ID3v2.3 taggers join multiple values); neither appears in an id. An empty
// slot is as bad as an invalid id: dropping it would shift every later id
// onto the wrong credit.
func musicBrainzIDs(value string) []string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	slots := strings.Split(strings.ReplaceAll(trimmed, "/", multiValueSeparator), multiValueSeparator)
	ids := make([]string, 0, len(slots))
	for _, slot := range slots {
		id := strings.ToLower(strings.TrimSpace(slot))
		valid := musicBrainzIDPattern.MatchString(id)
		if !valid {
			return nil
		}
		ids = append(ids, id)
	}
	return ids
}

// splitMultiValue returns a ';'-joined tag's nonblank values, and whether a
// value between separators was blank.
func splitMultiValue(value string) ([]string, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, false
	}
	values := []string{}
	gap := false
	for raw := range strings.SplitSeq(trimmed, multiValueSeparator) {
		part := strings.TrimSpace(raw)
		if part == "" {
			gap = true
			continue
		}
		values = append(values, part)
	}
	return values, gap
}

// vorbisCommentContainers keep their tags as Vorbis comments, whose repeated
// values ffprobe joins in full; it cuts an ID3 or MP4 list to its first value.
var vorbisCommentContainers = map[string]bool{"flac": true, "opus": true, "ogg": true}

// streamTaggedContainers keep every tag on the audio stream: the Ogg muxer
// writes Opus and Vorbis comments there and none on the container.
var streamTaggedContainers = map[string]bool{"opus": true, "ogg": true}

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
