package spotify

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	spotifylib "github.com/zmb3/spotify/v2"
)

func TestMatchErrorFormatsDebugInfo(t *testing.T) {
	err := newMatchError(MatchDebugInfo{
		Lookup:          "album",
		Input:           "Abbey Road",
		SearchQuery:     "abbey road query",
		Strategy:        "album_field_search",
		CandidateName:   "Abbey Load",
		CandidateArtist: "The Beetles",
		Score:           42,
		Threshold:       76,
		Reason:          "score_below_threshold",
	}, errors.New("boom"))

	msg := err.Error()
	for _, want := range []string{
		"spotify album match failed",
		`input="Abbey Road"`,
		`search="abbey road query"`,
		`candidate="Abbey Load"`,
		`candidate_artist="The Beetles"`,
		"score=42",
		"threshold=76",
		"strategy=album_field_search",
		"reason=score_below_threshold",
		"error=boom",
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("Error() = %q, missing %q", msg, want)
		}
	}

	minimal := newMatchError(MatchDebugInfo{Lookup: "artist", Input: "Q"}, nil)
	got := minimal.Error()
	if got != `spotify artist match failed input="Q"` {
		t.Fatalf("minimal Error() = %q, want only lookup and input", got)
	}
}

func TestMatchErrorUnwrapsToCause(t *testing.T) {
	cause := errors.New("api down")
	err := newMatchError(MatchDebugInfo{Lookup: "artist", Input: "Q"}, cause)
	if !errors.Is(err, cause) {
		t.Fatalf("expected errors.Is to reach the wrapped cause through %v", err)
	}
}

func TestAsMatchError(t *testing.T) {
	base := newMatchError(MatchDebugInfo{Lookup: "artist", Input: "Q", Reason: "no_results"}, nil)
	wrapped := fmt.Errorf("scan artist failed: %w", base)

	matchErr, ok := AsMatchError(wrapped)
	if !ok {
		t.Fatalf("expected AsMatchError to find MatchError in %v", wrapped)
	}
	if matchErr.Info.Reason != "no_results" {
		t.Fatalf("reason = %q, want no_results", matchErr.Info.Reason)
	}

	_, ok = AsMatchError(errors.New("plain error"))
	if ok {
		t.Fatal("expected AsMatchError to reject a plain error")
	}
}

func TestScoreArtistName(t *testing.T) {
	got := scoreArtistName("Beyoncé", "Beyonce")
	if got != 100 {
		t.Fatalf("diacritic-normalized exact match = %d, want 100", got)
	}

	got = scoreArtistName("The Beatles", "Beatles")
	if got < spotifyArtistThreshold {
		t.Fatalf("stop-word-insensitive token match = %d, want >= threshold %d", got, spotifyArtistThreshold)
	}

	got = scoreArtistName("Hall & Oates", "Daryl Hall & John Oates")
	if got < spotifyArtistThreshold {
		t.Fatalf("canonical duo name = %d, want >= threshold %d", got, spotifyArtistThreshold)
	}

	got = scoreArtistName("Oates Hall", "Hall Oates")
	if got < spotifyArtistThreshold {
		t.Fatalf("out-of-order full token overlap = %d, want >= threshold %d", got, spotifyArtistThreshold)
	}

	// A single-token query matching only the first token of a longer candidate is
	// ambiguous ("Beyonce" vs "Beyonce Smith") and must stay below the threshold.
	got = scoreArtistName("Beyonce", "Beyonce Smith")
	if got >= spotifyArtistThreshold {
		t.Fatalf("single-token prefix match = %d, want < threshold %d", got, spotifyArtistThreshold)
	}

	// A candidate covering only part of the query's tokens is a compound-credit
	// mismatch and must stay below the threshold.
	got = scoreArtistName("Charlie Puth & Coco Jones", "Charlie Puth")
	if got >= spotifyArtistThreshold {
		t.Fatalf("truncated candidate = %d, want < threshold %d", got, spotifyArtistThreshold)
	}

	got = scoreArtistName("Red Hot Chili Peppers", "Red Vines")
	if got >= spotifyArtistThreshold {
		t.Fatalf("partial token overlap = %d, want < threshold %d", got, spotifyArtistThreshold)
	}

	exact := scoreArtistName("Guns N' Roses", "Guns N' Roses")
	superset := scoreArtistName("Guns N' Roses", "Guns N' Roses Tribute")
	if exact <= superset {
		t.Fatalf("exact match (%d) must outrank superset candidate (%d)", exact, superset)
	}

	got = scoreArtistName("", "Anyone")
	if got != 0 {
		t.Fatalf("empty query = %d, want 0", got)
	}
}

func TestScoreAlbumTitle(t *testing.T) {
	got := scoreAlbumTitle("Abbey Road", "abbey road")
	if got != 100 {
		t.Fatalf("case-insensitive exact match = %d, want 100", got)
	}

	got = scoreAlbumTitle("Meteora (Deluxe Edition)", "Meteora")
	if got < spotifyAlbumThreshold {
		t.Fatalf("noise-token edition variant = %d, want >= threshold %d", got, spotifyAlbumThreshold)
	}

	// A candidate missing real (non-noise) query tokens is a different release and
	// must stay below the threshold.
	got = scoreAlbumTitle("Meteora Live Around the World", "Meteora")
	if got >= spotifyAlbumThreshold || got == 0 {
		t.Fatalf("truncated candidate = %d, want partial score < threshold %d", got, spotifyAlbumThreshold)
	}

	got = scoreAlbumTitle("My Album", "Completely Different")
	if got >= spotifyAlbumThreshold {
		t.Fatalf("unrelated candidate = %d, want < threshold %d", got, spotifyAlbumThreshold)
	}

	got = scoreAlbumTitle("Road Abbey", "Abbey Road")
	if got < spotifyAlbumThreshold {
		t.Fatalf("out-of-order full token overlap = %d, want >= threshold %d", got, spotifyAlbumThreshold)
	}

	// A title made only of noise tokens has nothing left to compare against.
	got = scoreAlbumTitle("Deluxe Edition", "Meteora")
	if got != 0 {
		t.Fatalf("noise-only query vs unrelated candidate = %d, want 0", got)
	}
}

func TestScoreAlbumArtist(t *testing.T) {
	// No artist in the query: neutral perfect score so title alone decides.
	score, name := scoreAlbumArtist("", []spotifylib.SimpleArtist{{Name: "Someone"}})
	if score != 100 || name != "" {
		t.Fatalf("empty query artist = (%d, %q), want (100, \"\")", score, name)
	}

	// Candidate without artist credits: lenient score that keeps the match viable.
	score, name = scoreAlbumArtist("The Beatles", nil)
	if score != 70 || name != "" {
		t.Fatalf("candidate without artists = (%d, %q), want (70, \"\")", score, name)
	}

	score, name = scoreAlbumArtist("The Beatles", []spotifylib.SimpleArtist{
		{Name: "Wrong Artist"},
		{Name: "The Beatles"},
	})
	if score != 100 || name != "The Beatles" {
		t.Fatalf("best candidate artist = (%d, %q), want (100, \"The Beatles\")", score, name)
	}
}

func TestTokenizeComparisonTextDropsEmptyInput(t *testing.T) {
	got := tokenizeComparisonText("!!!", nil)
	if got != nil {
		t.Fatalf("tokenize(%q) = %v, want nil", "!!!", got)
	}
	got = tokenizeComparisonText("the and", artistStopWords)
	if len(got) != 0 {
		t.Fatalf("tokenize stop-words-only = %v, want empty", got)
	}
}

// The index penalty in selectBestArtistMatch must not stop a later, better
// candidate from beating an earlier partial match.
func TestSelectBestArtistMatch(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		candidates []string
		wantName   string
		wantReason string
	}{
		{"no candidates", "Beyonce", nil, "", MatchReasonNoResults},
		{"diacritic variant at a later index", "Beyonce", []string{"Beyonce Smith", "Beyoncé"}, "Beyoncé", MatchReasonAccepted},
		{"punctuation variant at a later index", "Guns N Roses", []string{"Guns Tribute", "Guns N' Roses"}, "Guns N' Roses", MatchReasonAccepted},
		{"best candidate below threshold", "Red Hot Chili Peppers", []string{"Red Vines", "Chili Sauce"}, "Red Vines", MatchReasonScoreBelowThreshold},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			artists := make([]spotifylib.FullArtist, 0, len(tc.candidates))
			for _, name := range tc.candidates {
				var artist spotifylib.FullArtist
				artist.Name = name
				artists = append(artists, artist)
			}

			artist, info := selectBestArtistMatch(tc.query, artists, strategyArtistSearch)
			if info.Reason != tc.wantReason || info.CandidateName != tc.wantName {
				t.Fatalf("info = %+v, want reason %s candidate %q", info, tc.wantReason, tc.wantName)
			}
			accepted := artist != nil
			if accepted != (tc.wantReason == MatchReasonAccepted) {
				t.Fatalf("artist = %v for reason %s", artist, info.Reason)
			}
			if accepted && artist.Name != tc.wantName {
				t.Fatalf("artist.Name = %q, want %q", artist.Name, tc.wantName)
			}
		})
	}
}

func TestSelectBestAlbumMatch(t *testing.T) {
	album := func(id, name string, artists ...string) spotifylib.SimpleAlbum {
		candidate := spotifylib.SimpleAlbum{ID: spotifylib.ID(id), Name: name}
		for _, artist := range artists {
			candidate.Artists = append(candidate.Artists, spotifylib.SimpleArtist{Name: artist})
		}
		return candidate
	}

	tests := []struct {
		name       string
		title      string
		artist     string
		albums     []spotifylib.SimpleAlbum
		wantID     string
		wantReason string
	}{
		{"no candidates", "Greatest Hits", "Artist B", nil, "", MatchReasonNoResults},
		{
			"later candidate with the matching artist wins",
			"Greatest Hits", "Artist B",
			[]spotifylib.SimpleAlbum{album("wrong123", "Greatest Hits", "Artist A"), album("right123", "Greatest Hits", "Artist B")},
			"right123", MatchReasonAccepted,
		},
		{
			"unicode-bearing title matches its ascii query",
			"Love Yourself Answer", "BTS",
			[]spotifylib.SimpleAlbum{album("answer123", "Love Yourself 結 'Answer'", "BTS")},
			"answer123", MatchReasonAccepted,
		},
		{
			"all-negative scores keep the first candidate as the best",
			"Target Title", "Expected Artist",
			[]spotifylib.SimpleAlbum{album("bad123", "Completely Different", "Wrong Artist"), album("worse123", "Nothing Related", "Another Artist")},
			"bad123", MatchReasonScoreBelowThreshold,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			matched, info := selectBestAlbumMatch(tc.title, tc.artist, tc.albums, "query", strategyAlbumFieldSearch)
			if info.Reason != tc.wantReason {
				t.Fatalf("reason = %q, want %q (%+v)", info.Reason, tc.wantReason, info)
			}
			accepted := matched != nil
			if accepted != (tc.wantReason == MatchReasonAccepted) {
				t.Fatalf("album = %v for reason %s", matched, info.Reason)
			}
			if accepted && string(matched.ID) != tc.wantID {
				t.Fatalf("album.ID = %q, want %q", matched.ID, tc.wantID)
			}
			if tc.wantReason == MatchReasonScoreBelowThreshold {
				if info.CandidateName != tc.albums[0].Name || info.Score >= 0 {
					t.Fatalf("info = %+v, want the first candidate with a negative score", info)
				}
			}
		})
	}
}

func TestChooseBetterMatchInfo(t *testing.T) {
	noResults := MatchDebugInfo{Reason: MatchReasonNoResults, Strategy: strategyAlbumFieldSearch}
	lowScore := MatchDebugInfo{Reason: MatchReasonScoreBelowThreshold, Strategy: strategyAlbumFieldSearch, Score: 40}
	higherScore := MatchDebugInfo{Reason: MatchReasonScoreBelowThreshold, Strategy: strategyAlbumFallback, Score: 55}
	tiedFallback := MatchDebugInfo{Reason: MatchReasonScoreBelowThreshold, Strategy: strategyAlbumFallback, Score: 40}

	tests := []struct {
		name      string
		current   MatchDebugInfo
		candidate MatchDebugInfo
		want      MatchDebugInfo
	}{
		{"higher score replaces current", lowScore, higherScore, higherScore},
		{"lower score keeps current", higherScore, lowScore, higherScore},
		{"tie replaces a no_results placeholder", noResults, tiedFallback, tiedFallback},
		{"tie keeps a scored current", lowScore, tiedFallback, lowScore},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := chooseBetterMatchInfo(tc.current, tc.candidate)
			if got != tc.want {
				t.Fatalf("chooseBetterMatchInfo = %+v, want %+v", got, tc.want)
			}
		})
	}
}
