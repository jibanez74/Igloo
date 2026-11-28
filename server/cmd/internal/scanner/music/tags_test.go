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
			name: "a blank ARTISTS slot keeps the names but drops the ids beside them",
			tags: ffprobe.FormatTags{Artists: "A;;C", MusicBrainzArtistID: a + ";" + b},
			want: trackTags{artists: []string{"A", "C"}},
		},
		{
			name: "a blank slot beside a single ARTISTS name keeps the ids",
			tags: ffprobe.FormatTags{Artists: "A;", MusicBrainzArtistID: a},
			want: trackTags{soleArtist: "A", artistIDs: []string{a}},
		},
		{
			name: "a trailing separator is an empty slot too",
			tags: ffprobe.FormatTags{MusicBrainzArtistID: a + ";"},
			want: trackTags{},
		},
		{
			name: "a lone ARTISTS value is no credit list, only the lead artist",
			tags: ffprobe.FormatTags{Artists: "Leslie Odom, Jr."},
			want: trackTags{soleArtist: "Leslie Odom, Jr."},
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
			same := slices.Equal(got.artists, tc.want.artists) && got.soleArtist == tc.want.soleArtist && slices.Equal(got.artistIDs, tc.want.artistIDs) &&
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

func TestPairableArtistIDsDistrustsALoneIDFromACutList(t *testing.T) {
	const a = "cc197bad-dc9c-440d-a5b5-d52ba2e14234"
	const b = "1f9df192-a621-4f54-8850-2c5373b7eac9"
	credit := func(names ...string) []resolvedMusician {
		credits := make([]resolvedMusician, 0, len(names))
		for _, name := range names {
			credits = append(credits, resolvedMusician{name: name})
		}
		return credits
	}
	cases := []struct {
		name      string
		tags      trackTags
		container string
		credits   []resolvedMusician
		want      []string
	}{
		{"one id per credit", trackTags{artistIDs: []string{a, b}}, "mp3", credit("A", "B"), []string{a, b}},
		{"a count mismatch pairs nothing", trackTags{artistIDs: []string{a, b}}, "flac", credit("A"), nil},
		{"a lone plain credit keeps its id", trackTags{artistIDs: []string{a}}, "m4a", credit("Adele"), []string{a}},
		{"a cut list beside a compound credit", trackTags{artistIDs: []string{a}}, "mp3", credit("Sonny Rollins & Coleman Hawkins"), nil},
		{"a cut list beside a known act keeps its id", trackTags{artistIDs: []string{a}}, "m4a", credit("Simon & Garfunkel"), []string{a}},
		{"a whole Vorbis list beside a compound credit", trackTags{artistIDs: []string{a}}, "flac", credit("Sonny Rollins & Coleman Hawkins"), []string{a}},
		{"the cut ARTISTS lead names someone else", trackTags{artistIDs: []string{a}, soleArtist: "A"}, "mp3", credit("A x B"), nil},
		{"the cut ARTISTS lead is the credit", trackTags{artistIDs: []string{a}, soleArtist: " adele "}, "m4a", credit("Adele"), []string{a}},
		{"a lead naming someone else withholds the id in a Vorbis list too", trackTags{artistIDs: []string{a}, soleArtist: "A"}, "flac", credit("A x B"), nil},
		{"a Vorbis lead that is the credit keeps the id", trackTags{artistIDs: []string{a}, soleArtist: "Hall & Oates"}, "opus", credit("Hall & Oates"), []string{a}},
		{"a lead other than the first of several credits", trackTags{artistIDs: []string{a, b}, soleArtist: "A"}, "mp3", credit("B", "A"), nil},
		{"a lead that is the first of several credits", trackTags{artistIDs: []string{a, b}, soleArtist: "A"}, "mp3", credit("A", "B"), []string{a, b}},
		{"no credits pair nothing", trackTags{}, "flac", nil, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := pairableArtistIDs(tc.tags, tc.container, tc.credits)
			if !slices.Equal(got, tc.want) {
				t.Errorf("pairableArtistIDs() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestStoredArtistsReadsBackWhatTheTrackKept(t *testing.T) {
	cases := []struct {
		value string
		names []string
		lead  string
	}{
		{"", nil, ""},
		{"Adele", nil, "Adele"},
		{"Leslie Odom, Jr.;Lin-Manuel Miranda", []string{"Leslie Odom, Jr.", "Lin-Manuel Miranda"}, ""},
	}
	for _, tc := range cases {
		names, lead := storedArtists(tc.value)
		if !slices.Equal(names, tc.names) || lead != tc.lead {
			t.Errorf("storedArtists(%q) = %q, %q, want %q, %q", tc.value, names, lead, tc.names, tc.lead)
		}
	}
}
