package music

import (
	"slices"
	"testing"

	"igloo/cmd/internal/ffprobe"
)

func TestReadTrackTags(t *testing.T) {
	const a = "cc197bad-dc9c-440d-a5b5-d52ba2e14234"
	const b = "1f9df192-a621-4f54-8850-2c5373b7eac9"
	cases := []struct {
		name string
		tags ffprobe.FormatTags
		want trackTags
	}{
		{
			name: "ids are lowercased and kept only as uuids",
			tags: ffprobe.FormatTags{MusicBrainzReleaseID: "  03271BB1-A396-4038-A068-B7D5945C2C37 ", MusicBrainzReleaseGroupID: "not-an-id", MusicBrainzRecordingID: "12345"},
			want: trackTags{releaseID: "03271bb1-a396-4038-a068-b7d5945c2c37"},
		},
		{
			name: "multi-value ids split on either separator in credit order",
			tags: ffprobe.FormatTags{MusicBrainzArtistID: a + ";" + b, MusicBrainzAlbumArtistID: b + "/" + a},
			want: trackTags{artistIDs: []string{a, b}, albumArtistIDs: []string{b, a}},
		},
		{
			name: "one bad entry drops the whole list",
			tags: ffprobe.FormatTags{MusicBrainzArtistID: a + "; oops; " + b},
			want: trackTags{},
		},
		{
			name: "an empty slot drops the whole list, as dropping it would shift the ids",
			tags: ffprobe.FormatTags{MusicBrainzArtistID: ";" + a + ";" + b, MusicBrainzAlbumArtistID: a + ";;" + b},
			want: trackTags{},
		},
		{
			name: "a trailing separator is an empty slot too",
			tags: ffprobe.FormatTags{MusicBrainzArtistID: a + ";"},
			want: trackTags{},
		},
		{
			name: "a lone ARTISTS value is ignored",
			tags: ffprobe.FormatTags{Artists: "Leslie Odom, Jr."},
			want: trackTags{},
		},
		{
			name: "two or more ARTISTS values name the credits",
			tags: ffprobe.FormatTags{Artists: "Leslie Odom, Jr.; Lin-Manuel Miranda;;Ensemble "},
			want: trackTags{artists: []string{"Leslie Odom, Jr.", "Lin-Manuel Miranda", "Ensemble"}},
		},
		{
			name: "compilation, isrc and label",
			tags: ffprobe.FormatTags{Compilation: " TRUE ", ISRC: "gbaye0000351", Label: " Parlophone "},
			want: trackTags{compilation: true, isrc: "GBAYE0000351", label: "Parlophone"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := readTrackTags(tc.tags)
			same := slices.Equal(got.artists, tc.want.artists) && slices.Equal(got.artistIDs, tc.want.artistIDs) &&
				slices.Equal(got.albumArtistIDs, tc.want.albumArtistIDs) && got.recordingID == tc.want.recordingID &&
				got.releaseID == tc.want.releaseID && got.releaseGroupID == tc.want.releaseGroupID &&
				got.isrc == tc.want.isrc && got.label == tc.want.label && got.compilation == tc.want.compilation
			if !same {
				t.Errorf("readTrackTags() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestTagIsTrueAcceptsOnlyAffirmativeValues(t *testing.T) {
	for value, want := range map[string]bool{"1": true, "yes": true, "True": true, "0": false, "": false, "no": false, "2": false} {
		got := tagIsTrue(value)
		if got != want {
			t.Errorf("tagIsTrue(%q) = %v, want %v", value, got, want)
		}
	}
}

func TestTagTotalReadsOnlyNonnegativeNumbers(t *testing.T) {
	for value, want := range map[string]int64{"12": 12, " 3 ": 3, "": 0, "x": 0, "-1": 0} {
		got := tagTotal(value)
		if got != want {
			t.Errorf("tagTotal(%q) = %d, want %d", value, got, want)
		}
	}
}
