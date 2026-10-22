package spotify

import (
	"errors"
	"fmt"
	"strings"
	"testing"
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

	matchErr, ok := errors.AsType[*MatchError](wrapped)
	if !ok {
		t.Fatalf("expected errors.AsType to find MatchError in %v", wrapped)
	}
	if matchErr.Info.Reason != "no_results" {
		t.Fatalf("reason = %q, want no_results", matchErr.Info.Reason)
	}

	_, ok = errors.AsType[*MatchError](errors.New("plain error"))
	if ok {
		t.Fatal("expected errors.AsType to reject a plain error")
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

func TestScoreTitle(t *testing.T) {
	got := scoreTitle("Abbey Road", "abbey road")
	if got != 100 {
		t.Fatalf("case-insensitive exact match = %d, want 100", got)
	}

	got = scoreTitle("Meteora (Deluxe Edition)", "Meteora")
	if got < spotifyAlbumThreshold {
		t.Fatalf("noise-token edition variant = %d, want >= threshold %d", got, spotifyAlbumThreshold)
	}

	// A candidate missing real (non-noise) query tokens is a different release and
	// must stay below the threshold.
	got = scoreTitle("Meteora Live Around the World", "Meteora")
	if got >= spotifyAlbumThreshold || got == 0 {
		t.Fatalf("truncated candidate = %d, want partial score < threshold %d", got, spotifyAlbumThreshold)
	}

	got = scoreTitle("My Album", "Completely Different")
	if got >= spotifyAlbumThreshold {
		t.Fatalf("unrelated candidate = %d, want < threshold %d", got, spotifyAlbumThreshold)
	}

	got = scoreTitle("Road Abbey", "Abbey Road")
	if got < spotifyAlbumThreshold {
		t.Fatalf("out-of-order full token overlap = %d, want >= threshold %d", got, spotifyAlbumThreshold)
	}

	// A title made only of noise tokens has nothing left to compare against.
	got = scoreTitle("Deluxe Edition", "Meteora")
	if got != 0 {
		t.Fatalf("noise-only query vs unrelated candidate = %d, want 0", got)
	}
}

func TestScoreAlbumArtist(t *testing.T) {
	// No artist in the query: neutral perfect score so title alone decides.
	score, name := scoreAlbumArtist(AlbumQuery{}, []SimpleArtist{{Name: "Someone"}})
	if score != 100 || name != "" {
		t.Fatalf("empty query artist = (%d, %q), want (100, \"\")", score, name)
	}

	// Candidate without artist credits: lenient score that keeps the match viable.
	score, name = scoreAlbumArtist(AlbumQuery{Artist: "The Beatles"}, nil)
	if score != 70 || name != "" {
		t.Fatalf("candidate without artists = (%d, %q), want (70, \"\")", score, name)
	}

	score, name = scoreAlbumArtist(AlbumQuery{Artist: "The Beatles"}, []SimpleArtist{
		{Name: "Wrong Artist"},
		{Name: "The Beatles"},
	})
	if score != 100 || name != "The Beatles" {
		t.Fatalf("best candidate artist = (%d, %q), want (100, \"The Beatles\")", score, name)
	}

	// A credited artist carrying the known Spotify id is certain whatever it is called.
	score, name = scoreAlbumArtist(AlbumQuery{Artist: "The Beatles", ArtistID: "beatles1"}, []SimpleArtist{
		{ID: "other1", Name: "The Beatles"},
		{ID: "beatles1", Name: "Beatles, The"},
	})
	if score != 100 || name != "Beatles, The" {
		t.Fatalf("known artist id = (%d, %q), want (100, \"Beatles, The\")", score, name)
	}

	// A same-named credit without the known id is held to the uncredited score.
	score, name = scoreAlbumArtist(AlbumQuery{Artist: "The Beatles", ArtistID: "beatles1"}, []SimpleArtist{{ID: "other1", Name: "The Beatles"}})
	if score != uncreditedAlbumArtistScore || name != "The Beatles" {
		t.Fatalf("same-named credit without the known id = (%d, %q), want (%d, \"The Beatles\")", score, name, uncreditedAlbumArtistScore)
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
			artists := make([]FullArtist, 0, len(tc.candidates))
			for _, name := range tc.candidates {
				var artist FullArtist
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
	album := func(id, name string, artists ...string) SimpleAlbum {
		candidate := SimpleAlbum{ID: ID(id), Name: name}
		for _, artist := range artists {
			candidate.Artists = append(candidate.Artists, SimpleArtist{Name: artist})
		}
		return candidate
	}

	edition := func(id, name, releaseDate string, totalTracks int) SimpleAlbum {
		return SimpleAlbum{ID: ID(id), Name: name, ReleaseDate: releaseDate, ReleaseDatePrecision: "day", TotalTracks: Numeric(totalTracks)}
	}

	tests := []struct {
		name       string
		query      AlbumQuery
		albums     []SimpleAlbum
		wantID     string
		wantReason string
	}{
		{"no candidates", AlbumQuery{Title: "Greatest Hits", Artist: "Artist B"}, nil, "", MatchReasonNoResults},
		{
			"later candidate with the matching artist wins",
			AlbumQuery{Title: "Greatest Hits", Artist: "Artist B"},
			[]SimpleAlbum{album("wrong123", "Greatest Hits", "Artist A"), album("right123", "Greatest Hits", "Artist B")},
			"right123", MatchReasonAccepted,
		},
		{
			"unicode-bearing title matches its ascii query",
			AlbumQuery{Title: "Love Yourself Answer", Artist: "BTS"},
			[]SimpleAlbum{album("answer123", "Love Yourself 結 'Answer'", "BTS")},
			"answer123", MatchReasonAccepted,
		},
		{
			"all-negative scores keep the first candidate as the best",
			AlbumQuery{Title: "Target Title", Artist: "Expected Artist"},
			[]SimpleAlbum{album("bad123", "Completely Different", "Wrong Artist"), album("worse123", "Nothing Related", "Another Artist")},
			"bad123", MatchReasonScoreBelowThreshold,
		},
		{
			"the known artist id outranks an earlier same-named credit",
			AlbumQuery{Title: "Greatest Hits", Artist: "Nirvana", ArtistID: "nirvana1"},
			[]SimpleAlbum{
				{ID: "cover123", Name: "Greatest Hits", Artists: []SimpleArtist{{ID: "cover1", Name: "Nirvana"}}},
				{ID: "real123", Name: "Greatest Hits", Artists: []SimpleArtist{{ID: "nirvana1", Name: "Nirvana"}}},
			},
			"real123", MatchReasonAccepted,
		},
		{
			"the release year breaks a tie between editions",
			AlbumQuery{Title: "Meteora", Year: 2003},
			[]SimpleAlbum{edition("remaster123", "Meteora", "2023-04-07", 34), edition("original123", "Meteora", "2003-03-25", 13)},
			"original123", MatchReasonAccepted,
		},
		{
			"the track total breaks a tie between editions",
			AlbumQuery{Title: "Meteora", TotalTracks: 13},
			[]SimpleAlbum{edition("remaster123", "Meteora", "2023-04-07", 34), edition("original123", "Meteora", "2003-03-25", 13)},
			"original123", MatchReasonAccepted,
		},
		{
			"without tag evidence the earlier edition keeps Spotify's order",
			AlbumQuery{Title: "Meteora"},
			[]SimpleAlbum{edition("remaster123", "Meteora", "2023-04-07", 34), edition("original123", "Meteora", "2003-03-25", 13)},
			"remaster123", MatchReasonAccepted,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			matched, info := selectBestAlbumMatch(tc.query, tc.albums, "query", strategyAlbumFieldSearch)
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

func TestArtistNamesAgree(t *testing.T) {
	tests := []struct {
		local, candidate string
		want             bool
	}{
		{"Beatles", "The Beatles", true},
		{"Red Hot Chili Peppers", "Red Hot Chili Peppers", true},
		{"A & B", "A", false},
		{"Someone", "Various Artists", false},
		{"", "Anyone", false},
	}
	for _, tc := range tests {
		got := ArtistNamesAgree(tc.local, tc.candidate)
		if got != tc.want {
			t.Errorf("ArtistNamesAgree(%q, %q) = %v, want %v", tc.local, tc.candidate, got, tc.want)
		}
	}
}

func TestMatchAlbumTrack(t *testing.T) {
	track := func(id, name string, disc, number, durationMs int) SimpleTrack {
		return SimpleTrack{ID: ID(id), Name: name, DiscNumber: Numeric(disc), TrackNumber: Numeric(number), Duration: Numeric(durationMs)}
	}
	okComputer := []SimpleTrack{
		track("t1", "Airbag", 1, 1, 287880),
		track("t2", "Paranoid Android", 1, 2, 387213),
		track("t3", "Subterranean Homesick Alien", 1, 3, 267480),
	}
	twoDiscs := []SimpleTrack{
		track("d1t1", "Opening", 1, 1, 200000),
		track("d1t2", "Second Song", 1, 2, 210000),
		track("d2t1", "Disc Two Opener", 2, 1, 220000),
	}

	tests := []struct {
		name         string
		query        TrackQuery
		tracks       []SimpleTrack
		wantID       string
		wantReason   string
		wantStrategy string
	}{
		{"empty album list", TrackQuery{Title: "Airbag", Number: 1}, nil, "", MatchReasonNoResults, strategyTrackPosition},
		{"position with an agreeing title", TrackQuery{Title: "Paranoid Android", Disc: 1, Number: 2, DurationMs: 300000}, okComputer, "t2", MatchReasonAccepted, strategyTrackPosition},
		{"position with an agreeing length but a file-name title", TrackQuery{Title: "02 - Track 2", Disc: 1, Number: 2, DurationMs: 386500}, okComputer, "t2", MatchReasonAccepted, strategyTrackPosition},
		{"position without disc on a single-disc album", TrackQuery{Title: "Airbag", Number: 1, DurationMs: 287000}, okComputer, "t1", MatchReasonAccepted, strategyTrackPosition},
		{"position disagreeing on title and length falls back to the title", TrackQuery{Title: "Subterranean Homesick Alien", Disc: 1, Number: 1, DurationMs: 100000}, okComputer, "t3", MatchReasonAccepted, strategyTrackTitle},
		{"no position falls back to the title", TrackQuery{Title: "Paranoid Android (Remastered)", DurationMs: 387000}, okComputer, "t2", MatchReasonAccepted, strategyTrackTitle},
		{"untagged disc on a multi-disc album is ambiguous and uses the title", TrackQuery{Title: "Disc Two Opener", Number: 1, DurationMs: 220000}, twoDiscs, "d2t1", MatchReasonAccepted, strategyTrackTitle},
		{"tagged disc on a multi-disc album uses the position", TrackQuery{Title: "untitled", Disc: 2, Number: 1, DurationMs: 220500}, twoDiscs, "d2t1", MatchReasonAccepted, strategyTrackPosition},
		{"a bonus track missing from Spotify is rejected with the nearest title", TrackQuery{Title: "Hidden Bonus Jam", Disc: 1, Number: 13, DurationMs: 500000}, okComputer, "", MatchReasonScoreBelowThreshold, strategyTrackTitle},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			matched, info := MatchAlbumTrack(tc.query, tc.tracks)
			if info.Reason != tc.wantReason || info.Strategy != tc.wantStrategy || info.Lookup != lookupTrack {
				t.Fatalf("info = %+v, want reason %s strategy %s", info, tc.wantReason, tc.wantStrategy)
			}
			accepted := matched != nil
			if accepted != (tc.wantReason == MatchReasonAccepted) {
				t.Fatalf("track = %v for reason %s", matched, info.Reason)
			}
			if accepted && string(matched.ID) != tc.wantID {
				t.Fatalf("track.ID = %q, want %q", matched.ID, tc.wantID)
			}
			if tc.wantReason != MatchReasonNoResults && info.CandidateName == "" {
				t.Fatalf("info = %+v, want a candidate name", info)
			}
		})
	}

	t.Run("a length-decided position still records the title score", func(t *testing.T) {
		_, info := MatchAlbumTrack(TrackQuery{Title: "02 - Track 2", Disc: 1, Number: 2, DurationMs: 386500}, okComputer)
		if info.CandidateName != "Paranoid Android" || info.Score >= spotifyTrackThreshold {
			t.Fatalf("info = %+v, want the candidate with its real (low) title score", info)
		}
	})
}
