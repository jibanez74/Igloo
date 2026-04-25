//go:build integration

package tmdb

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/joho/godotenv"
)

func loadIntegrationEnv(t *testing.T) string {
	t.Helper()

	_, currentFile, _, _ := runtime.Caller(0)
	serverDir := filepath.Join(filepath.Dir(currentFile), "..", "..", "..", "..")
	envPath := filepath.Join(serverDir, ".env")
	err := godotenv.Load(envPath)
	if err != nil {
		_ = godotenv.Load()
	}

	apiKey := os.Getenv("TMDB_API_KEY")
	if apiKey == "" {
		t.Skip("TMDB_API_KEY not set")
	}

	return apiKey
}

func TestGetTmdbMovieByIDIntegration(t *testing.T) {
	apiKey := loadIntegrationEnv(t)

	client, err := New(apiKey)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	t.Run("returns error when TmdbID is zero", func(t *testing.T) {
		movie := &TmdbMovie{TmdbID: 0}
		err := client.GetTmdbMovieByID(context.Background(), movie)
		if err == nil {
			t.Error("Expected error when TmdbID is zero")
		}
	})

	t.Run("fetches movie by valid ID and validates all fields", func(t *testing.T) {
		movie := &TmdbMovie{TmdbID: 603}
		err := client.GetTmdbMovieByID(context.Background(), movie)
		if err != nil {
			t.Fatalf("Failed to get movie: %v", err)
		}

		if movie.Title != "The Matrix" {
			t.Errorf("Expected title 'The Matrix', got '%s'", movie.Title)
		}
		if movie.OriginalTitle == "" {
			t.Error("Expected original_title to be populated")
		}
		if movie.Overview == "" {
			t.Error("Expected overview to be populated")
		}
		if movie.ReleaseDate == "" {
			t.Error("Expected release_date to be populated")
		}
		if movie.PosterPath == "" {
			t.Error("Expected poster_path to be populated")
		}
		if movie.BackdropPath == "" {
			t.Error("Expected backdrop_path to be populated")
		}
		if movie.Popularity == 0 {
			t.Error("Expected popularity to be non-zero")
		}
		if movie.VoteAverage == 0 {
			t.Error("Expected vote_average to be non-zero")
		}
		if movie.VoteCount == 0 {
			t.Error("Expected vote_count to be non-zero")
		}
		if movie.Runtime == 0 {
			t.Error("Expected runtime to be non-zero")
		}
		if movie.Status == "" {
			t.Error("Expected status to be populated")
		}
		if movie.OriginalLang == "" {
			t.Error("Expected original_language to be populated")
		}
		if movie.Budget == 0 {
			t.Error("Expected budget to be non-zero for The Matrix")
		}
		if movie.Revenue == 0 {
			t.Error("Expected revenue to be non-zero for The Matrix")
		}
		if movie.ImdbID == "" {
			t.Error("Expected imdb_id to be populated")
		}
		if len(movie.Genres) == 0 {
			t.Error("Expected genres to be populated")
		}
		if len(movie.ProductionCompanies) == 0 {
			t.Error("Expected production_companies to be populated")
		}
		if len(movie.Credits.Cast) == 0 {
			t.Error("Expected credits.cast to be populated")
		}
		if len(movie.Credits.Crew) == 0 {
			t.Error("Expected credits.crew to be populated")
		}
		if len(movie.Videos.Results) == 0 {
			t.Error("Expected videos.results to be populated")
		}
	})

	t.Run("returns error for non-existent movie ID", func(t *testing.T) {
		movie := &TmdbMovie{TmdbID: 999999999}
		err := client.GetTmdbMovieByID(context.Background(), movie)
		if err == nil {
			t.Error("Expected error for non-existent movie ID")
		}
	})
}

func TestGetTmdbMovieByTitleIntegration(t *testing.T) {
	apiKey := loadIntegrationEnv(t)

	client, err := New(apiKey)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	t.Run("returns error when title is empty", func(t *testing.T) {
		movie := &TmdbMovie{Title: ""}
		err := client.GetTmdbMovieByTitle(context.Background(), movie)
		if err == nil {
			t.Error("Expected error when title is empty")
		}
	})

	t.Run("fetches movie by title", func(t *testing.T) {
		movie := &TmdbMovie{Title: "Inception"}
		err := client.GetTmdbMovieByTitle(context.Background(), movie)
		if err != nil {
			t.Fatalf("Failed to get movie by title: %v", err)
		}

		if movie.TmdbID == 0 {
			t.Error("Expected TmdbID to be populated")
		}
		if movie.Title == "" {
			t.Error("Expected title to be populated")
		}
	})

	t.Run("returns error for non-existent movie title", func(t *testing.T) {
		movie := &TmdbMovie{Title: "xyznonexistentmovietitle12345"}
		err := client.GetTmdbMovieByTitle(context.Background(), movie)
		if err == nil {
			t.Error("Expected error for non-existent movie title")
		}
	})
}

func TestSearchMoviesByTitleAndYearIntegration(t *testing.T) {
	apiKey := loadIntegrationEnv(t)

	client, err := New(apiKey)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	t.Run("returns error when title is empty", func(t *testing.T) {
		_, err := client.SearchMoviesByTitleAndYear(context.Background(), "")
		if err == nil {
			t.Error("Expected error when title is empty")
		}
	})

	t.Run("searches movies by title only", func(t *testing.T) {
		movies, err := client.SearchMoviesByTitleAndYear(context.Background(), "Inception")
		if err != nil {
			t.Fatalf("Failed to search movies: %v", err)
		}
		if len(movies) == 0 {
			t.Error("Expected at least one movie result")
		}
	})

	t.Run("searches movies by title and year", func(t *testing.T) {
		movies, err := client.SearchMoviesByTitleAndYear(context.Background(), "The Matrix", 1999)
		if err != nil {
			t.Fatalf("Failed to search movies: %v", err)
		}
		if len(movies) == 0 {
			t.Error("Expected at least one movie result")
		}

		found := false
		for _, movie := range movies {
			if len(movie.ReleaseDate) >= 4 && movie.ReleaseDate[:4] == "1999" {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected at least one movie from 1999")
		}
	})

	t.Run("returns broad results when no movies match year exactly", func(t *testing.T) {
		movies, err := client.SearchMoviesByTitleAndYear(context.Background(), "The Matrix", 1850)
		if err != nil {
			t.Fatalf("Expected broad results, got error: %v", err)
		}
		if len(movies) == 0 {
			t.Error("Expected broad candidate results even when year does not match exactly")
		}
	})
}

func TestGetMoviesInTheatersIntegration(t *testing.T) {
	apiKey := loadIntegrationEnv(t)

	client, err := New(apiKey)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	movies, err := client.GetMoviesInTheaters(context.Background())
	if err != nil {
		t.Fatalf("Failed to get movies in theaters: %v", err)
	}
	if len(movies) == 0 {
		t.Error("Expected at least one movie in theaters")
	}
	if movies[0].TmdbID == 0 {
		t.Error("Expected TmdbID to be populated")
	}
	if movies[0].Title == "" {
		t.Error("Expected title to be populated")
	}
}

func TestGetTmdbPopularMoviesIntegration(t *testing.T) {
	apiKey := loadIntegrationEnv(t)

	client, err := New(apiKey)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	t.Run("fetches popular movies with default region", func(t *testing.T) {
		movies, err := client.GetTmdbPopularMovies(context.Background())
		if err != nil {
			t.Fatalf("Failed to get popular movies: %v", err)
		}
		if len(movies) == 0 {
			t.Error("Expected at least one popular movie")
		}
		if movies[0].TmdbID == 0 {
			t.Error("Expected TmdbID to be populated")
		}
		if movies[0].Title == "" {
			t.Error("Expected title to be populated")
		}
	})

	t.Run("fetches popular movies with custom region", func(t *testing.T) {
		movies, err := client.GetTmdbPopularMovies(context.Background(), "GB")
		if err != nil {
			t.Fatalf("Failed to get popular movies for GB: %v", err)
		}
		if len(movies) == 0 {
			t.Error("Expected at least one popular movie for GB region")
		}
	})
}
