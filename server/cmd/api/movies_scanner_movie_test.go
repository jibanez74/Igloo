package main

import (
	"testing"

	"igloo/cmd/internal/tmdb"
)

func TestSelectBestTmdbMatch(t *testing.T) {
	t.Run("empty results returns nil", func(t *testing.T) {
		results := []tmdb.TmdbMovie{}
		result := selectBestTmdbMatch(results, 2023)
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
		result := selectBestTmdbMatch(results, 2023)
		if result == nil {
			t.Fatal("Expected non-nil result")
		}
		if result.TmdbID != 1 {
			t.Errorf("Expected TMDB ID 1, got %d", result.TmdbID)
		}
	})

	t.Run("exact year match preferred", func(t *testing.T) {
		results := []tmdb.TmdbMovie{
			{
				TmdbID:      1,
				Title:       "Movie 2020",
				ReleaseDate: "2020-01-01",
				Popularity:  10.0,
				VoteAverage: 5.0,
			},
			{
				TmdbID:      2,
				Title:       "Movie 2023",
				ReleaseDate: "2023-01-01",
				Popularity:  5.0,
				VoteAverage: 4.0,
			},
		}
		result := selectBestTmdbMatch(results, 2023)
		if result == nil {
			t.Fatal("Expected non-nil result")
		}
		if result.TmdbID != 2 {
			t.Errorf("Expected TMDB ID 2 (year match), got %d", result.TmdbID)
		}
	})

	t.Run("multiple year matches prefers highest popularity", func(t *testing.T) {
		results := []tmdb.TmdbMovie{
			{
				TmdbID:      1,
				Title:       "Movie A 2023",
				ReleaseDate: "2023-01-01",
				Popularity:  20.0,
				VoteAverage: 6.0,
			},
			{
				TmdbID:      2,
				Title:       "Movie B 2023",
				ReleaseDate: "2023-01-01",
				Popularity:  50.0,
				VoteAverage: 7.0,
			},
			{
				TmdbID:      3,
				Title:       "Movie C 2023",
				ReleaseDate: "2023-01-01",
				Popularity:  30.0,
				VoteAverage: 8.0,
			},
		}
		result := selectBestTmdbMatch(results, 2023)
		if result == nil {
			t.Fatal("Expected non-nil result")
		}
		// Movie 2 has highest popularity (50.0) among year matches
		if result.TmdbID != 2 {
			t.Errorf("Expected TMDB ID 2 (highest popularity), got %d", result.TmdbID)
		}
	})

	t.Run("no year match prefers highest popularity and vote average", func(t *testing.T) {
		results := []tmdb.TmdbMovie{
			{
				TmdbID:      1,
				Title:       "Movie 2020",
				ReleaseDate: "2020-01-01",
				Popularity:  10.0,
				VoteAverage: 5.0, // Score: 10 + 50 = 60
			},
			{
				TmdbID:      2,
				Title:       "Movie 2021",
				ReleaseDate: "2021-01-01",
				Popularity:  20.0,
				VoteAverage: 7.0, // Score: 20 + 70 = 90
			},
			{
				TmdbID:      3,
				Title:       "Movie 2022",
				ReleaseDate: "2022-01-01",
				Popularity:  15.0,
				VoteAverage: 8.0, // Score: 15 + 80 = 95
			},
		}
		result := selectBestTmdbMatch(results, 2023) // Target year doesn't match any
		if result == nil {
			t.Fatal("Expected non-nil result")
		}
		// Movie 3 has highest combined score (95)
		if result.TmdbID != 3 {
			t.Errorf("Expected TMDB ID 3 (highest score), got %d", result.TmdbID)
		}
	})

	t.Run("year match always wins over popularity", func(t *testing.T) {
		results := []tmdb.TmdbMovie{
			{
				TmdbID:      1,
				Title:       "Popular Movie 2020",
				ReleaseDate: "2020-01-01",
				Popularity:  100.0, // Very popular
				VoteAverage: 10.0,  // Perfect rating
				// Score: 100 + 100 = 200
			},
			{
				TmdbID:      2,
				Title:       "Less Popular Movie 2023",
				ReleaseDate: "2023-01-01",
				Popularity:  1.0,   // Not popular
				VoteAverage: 1.0,   // Low rating
				// Score: 10000 (year match) + 1 + 10 = 10011
			},
		}
		result := selectBestTmdbMatch(results, 2023)
		if result == nil {
			t.Fatal("Expected non-nil result")
		}
		// Year match should win even with lower popularity
		if result.TmdbID != 2 {
			t.Errorf("Expected TMDB ID 2 (year match), got %d", result.TmdbID)
		}
	})

	t.Run("same popularity prefers higher vote average", func(t *testing.T) {
		results := []tmdb.TmdbMovie{
			{
				TmdbID:      1,
				Title:       "Movie A 2023",
				ReleaseDate: "2023-01-01",
				Popularity:  50.0,
				VoteAverage: 6.0, // Score: 10000 + 50 + 60 = 100110
			},
			{
				TmdbID:      2,
				Title:       "Movie B 2023",
				ReleaseDate: "2023-01-01",
				Popularity:  50.0,
				VoteAverage: 8.0, // Score: 10000 + 50 + 80 = 100130
			},
		}
		result := selectBestTmdbMatch(results, 2023)
		if result == nil {
			t.Fatal("Expected non-nil result")
		}
		// Movie 2 has higher vote average
		if result.TmdbID != 2 {
			t.Errorf("Expected TMDB ID 2 (higher vote average), got %d", result.TmdbID)
		}
	})

	t.Run("missing release date handles gracefully", func(t *testing.T) {
		results := []tmdb.TmdbMovie{
			{
				TmdbID:      1,
				Title:       "Movie No Date",
				ReleaseDate: "",
				Popularity:  50.0,
				VoteAverage: 7.0,
			},
			{
				TmdbID:      2,
				Title:       "Movie With Date",
				ReleaseDate: "2023-01-01",
				Popularity:  10.0,
				VoteAverage: 5.0,
			},
		}
		result := selectBestTmdbMatch(results, 2023)
		if result == nil {
			t.Fatal("Expected non-nil result")
		}
		// Movie 2 should win due to year match
		if result.TmdbID != 2 {
			t.Errorf("Expected TMDB ID 2 (year match), got %d", result.TmdbID)
		}
	})

	t.Run("short release date handles gracefully", func(t *testing.T) {
		results := []tmdb.TmdbMovie{
			{
				TmdbID:      1,
				Title:       "Movie Short Date",
				ReleaseDate: "20",
				Popularity:  50.0,
				VoteAverage: 7.0,
			},
			{
				TmdbID:      2,
				Title:       "Movie Valid Date",
				ReleaseDate: "2023-01-01",
				Popularity:  10.0,
				VoteAverage: 5.0,
			},
		}
		result := selectBestTmdbMatch(results, 2023)
		if result == nil {
			t.Fatal("Expected non-nil result")
		}
		// Movie 2 should win due to year match
		if result.TmdbID != 2 {
			t.Errorf("Expected TMDB ID 2 (year match), got %d", result.TmdbID)
		}
	})

	t.Run("target year zero ignores year matching", func(t *testing.T) {
		results := []tmdb.TmdbMovie{
			{
				TmdbID:      1,
				Title:       "Movie 2020",
				ReleaseDate: "2020-01-01",
				Popularity:  10.0,
				VoteAverage: 5.0,
			},
			{
				TmdbID:      2,
				Title:       "Movie 2023",
				ReleaseDate: "2023-01-01",
				Popularity:  20.0,
				VoteAverage: 7.0,
			},
		}
		result := selectBestTmdbMatch(results, 0) // No target year
		if result == nil {
			t.Fatal("Expected non-nil result")
		}
		// Movie 2 should win due to higher popularity + vote average
		if result.TmdbID != 2 {
			t.Errorf("Expected TMDB ID 2 (higher score), got %d", result.TmdbID)
		}
	})
}
