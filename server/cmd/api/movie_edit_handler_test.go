package main

import (
	"database/sql"
	"testing"

	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/tmdb"
)

func TestBuildUpdateParamsFromTmdb(t *testing.T) {
	t.Run("maps basic fields correctly", func(t *testing.T) {
		m := &tmdb.TmdbMovie{
			TmdbID:       42,
			Title:        "Test Movie",
			Overview:     "A test overview",
			PosterPath:   "/poster.jpg",
			BackdropPath: "/backdrop.jpg",
			Adult:        false,
			OriginalLang: "en",
			ImdbID:       "tt1234567",
			Tagline:      "A great tagline",
			Runtime:      120,
			VoteAverage:  7.5,
			Revenue:      1000000,
			Budget:       500000,
		}

		params := buildUpdateParamsFromTmdb(99, m)

		if params.ID != 99 {
			t.Errorf("Expected ID=99, got %d", params.ID)
		}
		if params.Title != "Test Movie" {
			t.Errorf("Expected Title=%q, got %q", "Test Movie", params.Title)
		}
		if !params.TmdbID.Valid || params.TmdbID.Int64 != 42 {
			t.Errorf("Expected TmdbID=42 (valid), got %v", params.TmdbID)
		}
		if !params.ImdbID.Valid || params.ImdbID.String != "tt1234567" {
			t.Errorf("Expected ImdbID=tt1234567 (valid), got %v", params.ImdbID)
		}
		if !params.PosterPath.Valid || params.PosterPath.String != "/poster.jpg" {
			t.Errorf("Expected PosterPath=/poster.jpg (valid), got %v", params.PosterPath)
		}
		if !params.BackdropPath.Valid || params.BackdropPath.String != "/backdrop.jpg" {
			t.Errorf("Expected BackdropPath=/backdrop.jpg (valid), got %v", params.BackdropPath)
		}
		if params.Adult != false {
			t.Error("Expected Adult=false")
		}
		if !params.Language.Valid || params.Language.String != "en" {
			t.Errorf("Expected Language=en (valid), got %v", params.Language)
		}
		if !params.Overview.Valid || params.Overview.String != "A test overview" {
			t.Errorf("Expected Overview set (valid), got %v", params.Overview)
		}
		if !params.TagLine.Valid || params.TagLine.String != "A great tagline" {
			t.Errorf("Expected TagLine set (valid), got %v", params.TagLine)
		}
		if !params.CriticRating.Valid || params.CriticRating.Float64 != 7.5 {
			t.Errorf("Expected CriticRating=7.5 (valid), got %v", params.CriticRating)
		}
		if !params.Revenue.Valid || params.Revenue.Float64 != 1000000.0 {
			t.Errorf("Expected Revenue=1000000 (valid), got %v", params.Revenue)
		}
		if !params.Budget.Valid || params.Budget.Float64 != 500000.0 {
			t.Errorf("Expected Budget=500000 (valid), got %v", params.Budget)
		}
		if !params.RunTime.Valid || params.RunTime.Int64 != 120 {
			t.Errorf("Expected RunTime=120 (valid), got %v", params.RunTime)
		}
	})

	t.Run("extracts year from release date", func(t *testing.T) {
		m := &tmdb.TmdbMovie{
			TmdbID:      1,
			Title:       "Dated Movie",
			ReleaseDate: "2019-07-26",
		}

		params := buildUpdateParamsFromTmdb(1, m)

		if !params.ReleaseDate.Valid || params.ReleaseDate.String != "2019-07-26" {
			t.Errorf("Expected ReleaseDate=2019-07-26, got %v", params.ReleaseDate)
		}
		if !params.Year.Valid || params.Year.Int64 != 2019 {
			t.Errorf("Expected Year=2019 (valid), got %v", params.Year)
		}
	})

	t.Run("empty release date leaves year and release_date invalid", func(t *testing.T) {
		m := &tmdb.TmdbMovie{
			TmdbID: 1,
			Title:  "No Date Movie",
		}

		params := buildUpdateParamsFromTmdb(1, m)

		if params.ReleaseDate.Valid {
			t.Errorf("Expected ReleaseDate to be invalid for empty string, got %v", params.ReleaseDate)
		}
		if params.Year.Valid {
			t.Errorf("Expected Year to be invalid when release date is empty, got %v", params.Year)
		}
	})

	t.Run("zero TmdbID produces invalid NullInt64", func(t *testing.T) {
		m := &tmdb.TmdbMovie{
			TmdbID: 0,
			Title:  "No TMDB ID",
		}

		params := buildUpdateParamsFromTmdb(1, m)

		if params.TmdbID.Valid {
			t.Errorf("Expected TmdbID to be invalid for 0, got %v", params.TmdbID)
		}
	})

	t.Run("zero budget and revenue produce invalid NullFloat64", func(t *testing.T) {
		m := &tmdb.TmdbMovie{
			TmdbID:  1,
			Title:   "Zero Budget",
			Budget:  0,
			Revenue: 0,
		}

		params := buildUpdateParamsFromTmdb(1, m)

		if params.Budget.Valid {
			t.Errorf("Expected Budget to be invalid for 0, got %v", params.Budget)
		}
		if params.Revenue.Valid {
			t.Errorf("Expected Revenue to be invalid for 0, got %v", params.Revenue)
		}
	})

	t.Run("zero runtime produces invalid NullInt64", func(t *testing.T) {
		m := &tmdb.TmdbMovie{
			TmdbID:  1,
			Title:   "No Runtime",
			Runtime: 0,
		}

		params := buildUpdateParamsFromTmdb(1, m)

		if params.RunTime.Valid {
			t.Errorf("Expected RunTime to be invalid for 0, got %v", params.RunTime)
		}
	})

	t.Run("empty string fields produce invalid NullString", func(t *testing.T) {
		m := &tmdb.TmdbMovie{
			TmdbID:       1,
			Title:        "Minimal Movie",
			Overview:     "",
			PosterPath:   "",
			BackdropPath: "",
			ImdbID:       "",
			Tagline:      "",
			OriginalLang: "",
		}

		params := buildUpdateParamsFromTmdb(1, m)

		for name, ns := range map[string]sql.NullString{
			"Overview":     params.Overview,
			"PosterPath":   params.PosterPath,
			"BackdropPath": params.BackdropPath,
			"ImdbID":       params.ImdbID,
			"TagLine":      params.TagLine,
			"Language":     params.Language,
		} {
			if ns.Valid {
				t.Errorf("Expected %s to be invalid for empty string, got %v", name, ns)
			}
		}
	})

	t.Run("adult flag is mapped correctly when true", func(t *testing.T) {
		m := &tmdb.TmdbMovie{
			TmdbID: 1,
			Title:  "Adult Movie",
			Adult:  true,
		}

		params := buildUpdateParamsFromTmdb(5, m)

		if !params.Adult {
			t.Error("Expected Adult=true")
		}
	})

	t.Run("certification is extracted from US release dates", func(t *testing.T) {
		m := &tmdb.TmdbMovie{
			TmdbID: 1,
			Title:  "Rated Movie",
		}
		m.ReleaseDates.Results = []struct {
			ISO3166_1    string `json:"iso_3166_1"`
			ReleaseDates []struct {
				Certification string `json:"certification"`
			} `json:"release_dates"`
		}{
			{
				ISO3166_1: "US",
				ReleaseDates: []struct {
					Certification string `json:"certification"`
				}{{Certification: "PG-13"}},
			},
		}

		params := buildUpdateParamsFromTmdb(1, m)

		if !params.Certification.Valid || params.Certification.String != "PG-13" {
			t.Errorf("Expected Certification=PG-13, got %v", params.Certification)
		}
	})

	t.Run("no certification leaves Certification invalid", func(t *testing.T) {
		m := &tmdb.TmdbMovie{
			TmdbID: 1,
			Title:  "Unrated Movie",
		}

		params := buildUpdateParamsFromTmdb(1, m)

		if params.Certification.Valid {
			t.Errorf("Expected Certification to be invalid when not set, got %v", params.Certification)
		}
	})

	t.Run("zero vote average produces invalid NullFloat64", func(t *testing.T) {
		m := &tmdb.TmdbMovie{
			TmdbID:      1,
			Title:       "No Rating",
			VoteAverage: 0,
		}

		params := buildUpdateParamsFromTmdb(1, m)

		if params.CriticRating.Valid {
			t.Errorf("Expected CriticRating to be invalid for 0 vote average, got %v", params.CriticRating)
		}
	})

	t.Run("helpers NullInt64 and NullString semantics are consistent", func(t *testing.T) {
		if helpers.NullString("").Valid {
			t.Error("Sanity: NullString('') should be invalid")
		}
		if !helpers.NullString("x").Valid {
			t.Error("Sanity: NullString('x') should be valid")
		}
		if helpers.NullInt64(0).Valid {
			t.Error("Sanity: NullInt64(0) should be invalid")
		}
		if !helpers.NullInt64(1).Valid {
			t.Error("Sanity: NullInt64(1) should be valid")
		}
	})

	t.Run("movie ID is preserved in params", func(t *testing.T) {
		m := &tmdb.TmdbMovie{TmdbID: 1, Title: "X"}
		for _, movieID := range []int64{1, 42, 9999} {
			params := buildUpdateParamsFromTmdb(movieID, m)
			if params.ID != movieID {
				t.Errorf("Expected ID=%d, got %d", movieID, params.ID)
			}
		}
	})

	t.Run("non-US first certification is used as fallback", func(t *testing.T) {
		m := &tmdb.TmdbMovie{
			TmdbID: 1,
			Title:  "Foreign Movie",
		}
		m.ReleaseDates.Results = []struct {
			ISO3166_1    string `json:"iso_3166_1"`
			ReleaseDates []struct {
				Certification string `json:"certification"`
			} `json:"release_dates"`
		}{
			{
				ISO3166_1: "GB",
				ReleaseDates: []struct {
					Certification string `json:"certification"`
				}{{Certification: "15"}},
			},
		}

		params := buildUpdateParamsFromTmdb(1, m)

		if !params.Certification.Valid || params.Certification.String != "15" {
			t.Errorf("Expected fallback Certification=15 (GB), got %v", params.Certification)
		}
	})
}