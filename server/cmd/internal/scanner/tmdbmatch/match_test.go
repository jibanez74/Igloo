package tmdbmatch

import (
	"testing"

	"igloo/cmd/internal/tmdb"
)

// topMatch is the top-ranked candidate, or nil when there are none. The
// scanners use the same ranking as the manual picker.
func topMatch(results []tmdb.TmdbMovie, targetTitle string, targetYear int) *Match {
	ranked := Rank(results, targetTitle, targetYear)
	if len(ranked) == 0 {
		return nil
	}
	return ranked[0]
}

func TestSelectBestMatch(t *testing.T) {
	t.Run("clean title beats noisy similar candidate", func(t *testing.T) {
		results := []tmdb.TmdbMovie{
			{TmdbID: 1, Title: "Moneyball", ReleaseDate: "2011-09-22", Popularity: 35.0, VoteAverage: 7.6},
			{TmdbID: 2, Title: "Balls of Fury", ReleaseDate: "2007-08-29", Popularity: 50.0, VoteAverage: 7.0},
		}
		result := topMatch(results, "Moneyball", 2011)
		if result == nil || result.Movie.TmdbID != 1 {
			t.Fatalf("Expected TMDB ID 1 (best title match), got %v", result)
		}
	})

	t.Run("missing year still chooses strongest title match", func(t *testing.T) {
		results := []tmdb.TmdbMovie{
			{TmdbID: 1, Title: "Train Dreams", Popularity: 5.0, VoteAverage: 6.0},
			{TmdbID: 2, Title: "Dream Scenario", ReleaseDate: "2023-01-01", Popularity: 20.0, VoteAverage: 7.0},
		}
		result := topMatch(results, "Train Dreams", 2025)
		if result == nil || result.Movie.TmdbID != 1 {
			t.Fatalf("Expected TMDB ID 1 (best title match), got %v", result)
		}
	})
}

func TestRankSortsBestCandidateFirst(t *testing.T) {
	results := []tmdb.TmdbMovie{
		{TmdbID: 1, Title: "Casino Royale", ReleaseDate: "1967-04-13", Popularity: 40.0, VoteAverage: 6.1},
		{TmdbID: 2, Title: "Casino Royale", ReleaseDate: "2006-11-14", Popularity: 35.0, VoteAverage: 7.6},
		{TmdbID: 3, Title: "Quantum of Solace", ReleaseDate: "2008-10-29", Popularity: 50.0, VoteAverage: 6.3},
	}

	ranked := Rank(results, "Casino Royale", 2006)
	if len(ranked) != 3 {
		t.Fatalf("expected 3 ranked results, got %d", len(ranked))
	}
	if ranked[0].Movie.TmdbID != 2 {
		t.Fatalf("expected 2006 Casino Royale first, got TMDB ID %d", ranked[0].Movie.TmdbID)
	}
	if ranked[1].Movie.TmdbID != 1 {
		t.Fatalf("expected 1967 Casino Royale second, got TMDB ID %d", ranked[1].Movie.TmdbID)
	}
}

func TestNormalizeTitleForSearch(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "Moneyball.2011.REMASTERED.2160p.4K.WEB.x265.10bit.AAC5.1-[YTS.MX]", want: "moneyball 2011"},
		{input: "Mary.Queen.of.Scots", want: "mary queen of scots"},
		{input: "If.I.Had.Legs.Id.Kick.You", want: "if i had legs id kick you"},
		{input: "The.Hobbit.An.Unexpected.Journey.2012.1080p.x265.10bit", want: "the hobbit an unexpected journey 2012"},
		{input: "Rabbit.Hole.2010.720p.8bit", want: "rabbit hole 2010"},
		{input: "Orbit.2022.10bit", want: "orbit 2022"},
		{input: "Gambit.2012.8bit", want: "gambit 2012"},
		{input: "The.Isaac.Story.2020.1080p.AAC.x264", want: "the isaac story 2020"},
	}

	for _, tt := range tests {
		got := NormalizeTitleForSearch(tt.input)
		if got != tt.want {
			t.Errorf("NormalizeTitleForSearch(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormalizeComparableTitlePreservesWordsEndingInBit(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "The Hobbit: An Unexpected Journey", want: "the hobbit an unexpected journey"},
		{input: "Rabbit Hole", want: "rabbit hole"},
		{input: "Orbit!", want: "orbit"},
		{input: "Gambit (2012)", want: "gambit 2012"},
	}

	for _, tt := range tests {
		got := normalizeComparableTitle(tt.input)
		if got != tt.want {
			t.Errorf("normalizeComparableTitle(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestRankingTitleDominatesYearAndPopularity(t *testing.T) {
	for _, tc := range []struct {
		name, title string
		year        int
		candidates  []tmdb.TmdbMovie
		want        int
	}{
		{"unrelated same year", "Arrival", 2016, []tmdb.TmdbMovie{{TmdbID: 1, Title: "Arrival", ReleaseDate: "2015-01-01"}, {TmdbID: 2, Title: "Random Film", ReleaseDate: "2016-01-01", Popularity: 1e9, VoteAverage: 1e9}}, 1},
		{"adjacent release", "Arrival", 2016, []tmdb.TmdbMovie{{TmdbID: 1, Title: "Arrival", ReleaseDate: "2015-01-01"}, {TmdbID: 2, Title: "Arrival", ReleaseDate: "2012-01-01", Popularity: 1e9, VoteAverage: 10}}, 1},
		{"sequel", "Rocky II", 1979, []tmdb.TmdbMovie{{TmdbID: 1, Title: "Rocky II", ReleaseDate: "1979-01-01"}, {TmdbID: 2, Title: "Rocky", ReleaseDate: "1979-01-01", Popularity: 1e9, VoteAverage: 10}}, 1},
		{"deterministic ties", "Movie", 0, []tmdb.TmdbMovie{{TmdbID: 2, Title: "Movie"}, {TmdbID: 1, Title: "Movie"}}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ranked := Rank(tc.candidates, tc.title, tc.year)
			if ranked[0].Movie.TmdbID != tc.want {
				t.Fatalf("winner=%+v", ranked[0])
			}
		})
	}
}

// A result returned by two interpretations keeps its stronger score, an
// invalid id is dropped, and ties resolve by title then id.
func TestCandidatesKeepBestPerID(t *testing.T) {
	var candidates Candidates
	if candidates.Best() != nil {
		t.Fatal("empty candidates produced a match")
	}
	weak := []tmdb.TmdbMovie{{TmdbID: 2, Title: "Space 1999", ReleaseDate: "1975-01-01"}, {TmdbID: 0, Title: "Invalid"}}
	strong := []tmdb.TmdbMovie{{TmdbID: 2, Title: "Space 1999", ReleaseDate: "1975-01-01"}, {TmdbID: 1, Title: "Space 1999", ReleaseDate: "1975-01-01"}}
	candidates.Add(Rank(weak, "space", 0))
	first := candidates.Best()
	candidates.Add(Rank(strong, "space 1999", 1975))
	second := candidates.Best()
	if first == nil || first.Movie.TmdbID != 2 || second == nil || second.Movie.TmdbID != 1 || len(candidates.best) != 2 {
		t.Fatalf("first=%+v second=%+v size=%d", first, second, len(candidates.best))
	}
	kept := candidates.best[2]
	if kept.Score <= first.Score {
		t.Fatalf("stronger interpretation did not replace the weaker score: %v <= %v", kept.Score, first.Score)
	}
}
