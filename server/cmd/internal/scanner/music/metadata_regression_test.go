package music

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	spotifylib "github.com/zmb3/spotify/v2"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/scanner"
	spotifyapi "igloo/cmd/internal/spotify"
)

func TestArtistSortCreditPositions(t *testing.T) {
	cases := []struct {
		value string
		count int
		want  []string
	}{
		{"Ramos, Anthony & Odom, Leslie, Jr.", 2, []string{"Ramos, Anthony", "Odom, Leslie, Jr."}},
		{"Same & Same", 2, []string{"Same", "Same"}},
		{" & Middle & ", 3, []string{"", "Middle", ""}},
		{"& Middle &", 3, []string{"", "Middle", ""}},
		{"First, , Third", 3, []string{"First", "", "Third"}},
		{"First, Jr., Second", 2, []string{"First, Jr.", "Second"}},
		{"First, Jr. & Second, III", 2, []string{"First, Jr.", "Second, III"}},
		{"Last, First, Other, Second", 2, nil},
		{"One & Two & Three", 2, nil},
	}
	for _, tc := range cases {
		got := parseArtistSortCredits(tc.value, tc.count)
		if !slices.Equal(got, tc.want) {
			t.Fatalf("%q: %q, want %q", tc.value, got, tc.want)
		}
	}
	credits := parseCompoundArtistCredits("Artist One, Artist Two, Artist One")
	if !slices.Equal(credits.occurrences, []string{"Artist One", "Artist Two", "Artist One"}) || len(credits.parts) != 2 {
		t.Fatalf("occurrences lost: %+v", credits)
	}
}

func TestArtistSortPersistenceAndSpotifyReconciliation(t *testing.T) {
	cases := []struct {
		artist, sorts string
		want          map[string]string
	}{
		{"Artist One, Artist Two, Artist One", "Z & Middle & A", map[string]string{"Artist One": "A", "Artist Two": "Middle"}},
		{"Artist One, Artist Two", "Same & Same", map[string]string{"Artist One": "Same", "Artist Two": "Same"}},
		{"Artist One, Artist Two, Artist Three", " & Middle & ", map[string]string{"Artist One": "Artist One", "Artist Two": "Middle", "Artist Three": "Artist Three"}},
		{"Artist One, Artist Two", "One, Artist & Two, Artist", map[string]string{"Artist One": "One, Artist", "Artist Two": "Two, Artist"}},
		{"Artist One, Artist Two", "One, Artist, Two, Artist", map[string]string{"Artist One": "Artist One", "Artist Two": "Artist Two"}},
	}
	for i, tc := range cases {
		for _, retry := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/retry=%v", i, retry), func(t *testing.T) {
				s := setupMusicScanner(t)
				defer s.db.Close()
				artist := tc.artist
				if retry {
					// Ampersand-only credits remain combined offline.
					artist = strings.ReplaceAll(artist, ", ", " & ")
				}
				scanTaggedTrack(t, s, newMusicScanContext(nil), filepath.Join(t.TempDir(), "track"), 1, ffprobe.FormatTags{Title: "Track", Artist: artist, SortArtist: tc.sorts})
				if retry {
					s.spotify = &musicScannerSpotifyStub{artistErr: &spotifyapi.MatchError{Info: spotifyapi.MatchDebugInfo{Reason: musicSpotifyReasonNoResults}}}
					err := s.retrySpotify(context.Background(), newMusicScanContext(nil))
					if err != nil {
						t.Fatal(err)
					}
				}
				for name, want := range tc.want {
					var got string
					err := s.db.QueryRow("SELECT sort_name FROM musicians WHERE name=?", name).Scan(&got)
					if err != nil || got != want {
						t.Fatalf("%s: %q want %q: %v", name, got, want, err)
					}
				}
				var raw string
				err := s.db.QueryRow("SELECT artist_sort FROM music_track_metadata").Scan(&raw)
				if err != nil || raw != tc.sorts {
					t.Fatalf("raw sort lost: %q: %v", raw, err)
				}
			})
		}
	}
}

func TestTrackLanguageImportAndChange(t *testing.T) {
	cases := []struct{ stream, format, want string }{
		{"eng", "spa", "eng"}, {"", "spa", "spa"}, {"UnD", "spa", "spa"}, {"zxx", "spa", "zxx"}, {"und", "zxx", "zxx"}, {"", "UND", ""}, {"und", "", ""}, {"und", "und", ""},
	}
	for _, tc := range cases {
		t.Run(tc.stream+"/"+tc.format, func(t *testing.T) {
			s := setupMusicScanner(t)
			defer s.db.Close()
			path := filepath.Join(t.TempDir(), "track.m4a")
			scan := newMusicScanContext(nil)
			var id int64
			for pass := 0; pass < 2; pass++ {
				metadata := testMusicMetadataWithTags(ffprobe.FormatTags{Title: "Track", Language: tc.format})
				metadata.Streams[0].Tags.Language = tc.stream
				metadata.Streams = append([]ffprobe.Stream{{CodecType: "video", Tags: ffprobe.StreamTags{Language: "ita"}}}, metadata.Streams...)
				metadata.Streams = append(metadata.Streams, ffprobe.Stream{CodecType: "audio", Tags: ffprobe.StreamTags{Language: "deu"}})
				s.ffprobe = &countingMusicScannerFfprobe{result: metadata}
				n, _, failures := s.processMusicFixtureBatch(t, context.Background(), scan, []scanner.ScanFile{{Path: path, Ext: "m4a", Size: int64(pass + 1)}})
				if n != 1 || failures != 0 {
					t.Fatalf("scan %d: %d/%d", pass, n, failures)
				}
				var got sql.NullString
				var currentID int64
				err := s.db.QueryRow("SELECT id,language FROM tracks WHERE file_path=?", path).Scan(&currentID, &got)
				if err != nil || got.String != tc.want || got.Valid != (tc.want != "") {
					t.Fatalf("language %+v want %q: %v", got, tc.want, err)
				}
				if pass == 1 && currentID != id {
					t.Fatal("changed track ID")
				}
				id = currentID
				// Ensure changed-file processing replaces an existing explicit language.
				_, err = s.db.Exec("UPDATE tracks SET language='fra' WHERE id=?", id)
				if err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestMergedArtistSortVotesOncePerTrack(t *testing.T) {
	s := setupMusicScanner(t)
	defer s.db.Close()
	s.spotify = &musicScannerSpotifyStub{artist: &spotifylib.FullArtist{SimpleArtist: spotifylib.SimpleArtist{ID: "shared"}}}
	dir := t.TempDir()
	scanTaggedTrack(t, s, newMusicScanContext(nil), filepath.Join(dir, "one"), 1,
		ffprobe.FormatTags{Title: "One", Artist: "Artist One, Artist Two, Artist One", SortArtist: "Z & A & Z"})
	scanTaggedTrack(t, s, newMusicScanContext(nil), filepath.Join(dir, "two"), 1,
		ffprobe.FormatTags{Title: "Two", Artist: "Artist Two", SortArtist: "Z"})
	var sort string
	err := s.db.QueryRow("SELECT sort_name FROM musicians").Scan(&sort)
	if err != nil || sort != "A" {
		t.Fatalf("one vote per track tie: %q: %v", sort, err)
	}
	count := countScannerRows(t, s.db, "SELECT count(*) FROM musicians")
	if count != 1 {
		t.Fatal("artists did not merge")
	}
	scanTaggedTrack(t, s, newMusicScanContext(nil), filepath.Join(dir, "three"), 1,
		ffprobe.FormatTags{Title: "Three", Artist: "Artist One", SortArtist: "Z"})
	err = s.db.QueryRow("SELECT sort_name FROM musicians").Scan(&sort)
	if err != nil || sort != "Z" {
		t.Fatalf("majority: %q: %v", sort, err)
	}
}
