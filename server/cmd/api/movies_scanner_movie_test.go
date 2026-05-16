package main

import (
	"math"
	"strconv"
	"testing"

	"igloo/cmd/internal/tmdb"
)

func TestSelectBestTmdbMatch(t *testing.T) {
	t.Run("empty results returns nil", func(t *testing.T) {
		results := []tmdb.TmdbMovie{}
		result := selectBestTmdbMatch(results, "test movie", 2023)
		if result != nil {
			t.Errorf("Expected nil for empty results, got %v", result)
		}
	})

	t.Run("single result returns that result", func(t *testing.T) {
		results := []tmdb.TmdbMovie{
			{
				TmdbID:      1,
				Title:       "Test Movie",
				ReleaseDate: "2023-01-01",
				Popularity:  50.0,
				VoteAverage: 7.5,
			},
		}
		result := selectBestTmdbMatch(results, "Test Movie", 2023)
		if result == nil {
			t.Fatal("Expected non-nil result")
		}
		if result.Movie.TmdbID != 1 {
			t.Errorf("Expected TMDB ID 1, got %d", result.Movie.TmdbID)
		}
	})

	t.Run("exact title and year beats popularity", func(t *testing.T) {
		results := []tmdb.TmdbMovie{
			{
				TmdbID:      1,
				Title:       "Casino Royale",
				ReleaseDate: "2020-01-01",
				Popularity:  90.0,
				VoteAverage: 8.5,
			},
			{
				TmdbID:      2,
				Title:       "Casino Royale",
				ReleaseDate: "2006-11-14",
				Popularity:  30.0,
				VoteAverage: 7.6,
			},
		}
		result := selectBestTmdbMatch(results, "Casino Royale", 2006)
		if result == nil {
			t.Fatal("Expected non-nil result")
		}
		if result.Movie.TmdbID != 2 {
			t.Errorf("Expected TMDB ID 2 (title/year match), got %d", result.Movie.TmdbID)
		}
	})

	t.Run("clean title beats noisy similar candidate", func(t *testing.T) {
		results := []tmdb.TmdbMovie{
			{
				TmdbID:      1,
				Title:       "Moneyball",
				ReleaseDate: "2011-09-22",
				Popularity:  35.0,
				VoteAverage: 7.6,
			},
			{
				TmdbID:      2,
				Title:       "Balls of Fury",
				ReleaseDate: "2007-08-29",
				Popularity:  50.0,
				VoteAverage: 7.0,
			},
		}
		result := selectBestTmdbMatch(results, "Moneyball", 2011)
		if result == nil {
			t.Fatal("Expected non-nil result")
		}
		if result.Movie.TmdbID != 1 {
			t.Errorf("Expected TMDB ID 1 (best title match), got %d", result.Movie.TmdbID)
		}
	})

	t.Run("missing year still chooses strongest title match", func(t *testing.T) {
		results := []tmdb.TmdbMovie{
			{
				TmdbID:      1,
				Title:       "Train Dreams",
				ReleaseDate: "",
				Popularity:  5.0,
				VoteAverage: 6.0,
			},
			{
				TmdbID:      2,
				Title:       "Dream Scenario",
				ReleaseDate: "2023-01-01",
				Popularity:  20.0,
				VoteAverage: 7.0,
			},
		}
		result := selectBestTmdbMatch(results, "Train Dreams", 2025)
		if result == nil {
			t.Fatal("Expected non-nil result")
		}
		if result.Movie.TmdbID != 1 {
			t.Errorf("Expected TMDB ID 1 (best title match), got %d", result.Movie.TmdbID)
		}
	})

	t.Run("confidence stays bounded", func(t *testing.T) {
		results := []tmdb.TmdbMovie{
			{
				TmdbID:      1,
				Title:       "Goldfinger",
				ReleaseDate: "1964-09-20",
				Popularity:  200.0,
				VoteAverage: 10.0,
			},
		}
		result := selectBestTmdbMatch(results, "Goldfinger", 1964)
		if result == nil {
			t.Fatal("Expected non-nil result")
		}
		if result.Confidence < 0 || result.Confidence > 100 {
			t.Errorf("Expected bounded confidence, got %f", result.Confidence)
		}
	})
}

func TestRankTmdbMatches_SortsBestCandidateFirst(t *testing.T) {
	results := []tmdb.TmdbMovie{
		{
			TmdbID:      1,
			Title:       "Casino Royale",
			ReleaseDate: "1967-04-13",
			Popularity:  40.0,
			VoteAverage: 6.1,
		},
		{
			TmdbID:      2,
			Title:       "Casino Royale",
			ReleaseDate: "2006-11-14",
			Popularity:  35.0,
			VoteAverage: 7.6,
		},
		{
			TmdbID:      3,
			Title:       "Quantum of Solace",
			ReleaseDate: "2008-10-29",
			Popularity:  50.0,
			VoteAverage: 6.3,
		},
	}

	ranked := rankTmdbMatches(results, "Casino Royale", 2006)
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

func TestNormalizeMovieTitleForSearch(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{
			input: "Moneyball.2011.REMASTERED.2160p.4K.WEB.x265.10bit.AAC5.1-[YTS.MX]",
			want:  "moneyball 2011",
		},
		{
			input: "Mary.Queen.of.Scots",
			want:  "mary queen of scots",
		},
		{
			input: "If.I.Had.Legs.Id.Kick.You",
			want:  "if i had legs id kick you",
		},
	}

	for _, tt := range tests {
		got := normalizeMovieTitleForSearch(tt.input)
		if got != tt.want {
			t.Errorf("normalizeMovieTitleForSearch(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFfprobeFormatDurationToRunTimeMinutes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		format  string
		wantMin int64
		wantSec float64
	}{
		{"5423.456", 90, 5423.456},
		{"3600", 60, 3600},
		{"59.9", 1, 59.9},
	}
	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			t.Parallel()
			durationSec, err := strconv.ParseFloat(tt.format, 64)
			if err != nil || durationSec <= 0 {
				t.Fatalf("parse %q: %v", tt.format, err)
			}
			runTimeMinutes := int64(math.Round(durationSec / 60))
			if runTimeMinutes != tt.wantMin {
				t.Errorf("rounded minutes = %d, want %d", runTimeMinutes, tt.wantMin)
			}
			if durationSec != tt.wantSec {
				t.Errorf("seconds = %v, want %v", durationSec, tt.wantSec)
			}
		})
	}
}

func TestFfprobeFormatDurationRejectedWhenInvalid(t *testing.T) {
	t.Parallel()
	for _, s := range []string{"", "0", "-1", "not-a-number"} {
		sec, err := strconv.ParseFloat(s, 64)
		ok := err == nil && sec > 0
		if ok {
			t.Errorf("%q should not be accepted as positive duration", s)
		}
	}
}
