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
	if got := minimal.Error(); got != `spotify artist match failed input="Q"` {
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

	if _, ok := AsMatchError(errors.New("plain error")); ok {
		t.Fatal("expected AsMatchError to reject a plain error")
	}
}

func TestScoreArtistName(t *testing.T) {
	if got := scoreArtistName("Beyoncé", "Beyonce"); got != 100 {
		t.Fatalf("diacritic-normalized exact match = %d, want 100", got)
	}

	if got := scoreArtistName("The Beatles", "Beatles"); got < spotifyArtistThreshold {
		t.Fatalf("stop-word-insensitive token match = %d, want >= threshold %d", got, spotifyArtistThreshold)
	}

	if got := scoreArtistName("Hall & Oates", "Daryl Hall & John Oates"); got < spotifyArtistThreshold {
		t.Fatalf("canonical duo name = %d, want >= threshold %d", got, spotifyArtistThreshold)
	}

	if got := scoreArtistName("Oates Hall", "Hall Oates"); got < spotifyArtistThreshold {
		t.Fatalf("out-of-order full token overlap = %d, want >= threshold %d", got, spotifyArtistThreshold)
	}

	// A single-token query matching only the first token of a longer candidate is
	// ambiguous ("Beyonce" vs "Beyonce Smith") and must stay below the threshold.
	if got := scoreArtistName("Beyonce", "Beyonce Smith"); got >= spotifyArtistThreshold {
		t.Fatalf("single-token prefix match = %d, want < threshold %d", got, spotifyArtistThreshold)
	}

	// A candidate covering only part of the query's tokens is a compound-credit
	// mismatch and must stay below the threshold.
	if got := scoreArtistName("Charlie Puth & Coco Jones", "Charlie Puth"); got >= spotifyArtistThreshold {
		t.Fatalf("truncated candidate = %d, want < threshold %d", got, spotifyArtistThreshold)
	}

	if got := scoreArtistName("Red Hot Chili Peppers", "Red Vines"); got >= spotifyArtistThreshold {
		t.Fatalf("partial token overlap = %d, want < threshold %d", got, spotifyArtistThreshold)
	}

	exact := scoreArtistName("Guns N' Roses", "Guns N' Roses")
	superset := scoreArtistName("Guns N' Roses", "Guns N' Roses Tribute")
	if exact <= superset {
		t.Fatalf("exact match (%d) must outrank superset candidate (%d)", exact, superset)
	}

	if got := scoreArtistName("", "Anyone"); got != 0 {
		t.Fatalf("empty query = %d, want 0", got)
	}
}

func TestScoreAlbumTitle(t *testing.T) {
	if got := scoreAlbumTitle("Abbey Road", "abbey road"); got != 100 {
		t.Fatalf("case-insensitive exact match = %d, want 100", got)
	}

	if got := scoreAlbumTitle("Meteora (Deluxe Edition)", "Meteora"); got < spotifyAlbumThreshold {
		t.Fatalf("noise-token edition variant = %d, want >= threshold %d", got, spotifyAlbumThreshold)
	}

	// A candidate missing real (non-noise) query tokens is a different release and
	// must stay below the threshold.
	if got := scoreAlbumTitle("Meteora Live Around the World", "Meteora"); got >= spotifyAlbumThreshold || got == 0 {
		t.Fatalf("truncated candidate = %d, want partial score < threshold %d", got, spotifyAlbumThreshold)
	}

	if got := scoreAlbumTitle("My Album", "Completely Different"); got >= spotifyAlbumThreshold {
		t.Fatalf("unrelated candidate = %d, want < threshold %d", got, spotifyAlbumThreshold)
	}

	if got := scoreAlbumTitle("Road Abbey", "Abbey Road"); got < spotifyAlbumThreshold {
		t.Fatalf("out-of-order full token overlap = %d, want >= threshold %d", got, spotifyAlbumThreshold)
	}

	// A title made only of noise tokens has nothing left to compare against.
	if got := scoreAlbumTitle("Deluxe Edition", "Meteora"); got != 0 {
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
	if got := tokenizeComparisonText("!!!", nil); got != nil {
		t.Fatalf("tokenize(%q) = %v, want nil", "!!!", got)
	}
	if got := tokenizeComparisonText("the and", artistStopWords); len(got) != 0 {
		t.Fatalf("tokenize stop-words-only = %v, want empty", got)
	}
}
