package tmdb

import (
	"os"
	"testing"

	"github.com/joho/godotenv"
)

// loadEnv loads the .env file from the project root.
// Returns the TMDB API key or skips the test if not available.
func loadEnv(t *testing.T) string {
	t.Helper()

	// Try to load .env from project root (3 levels up from this test file)
	_ = godotenv.Load("../../../.env")

	apiKey := os.Getenv("TMDB_API_KEY")
	if apiKey == "" {
		t.Skip("TMDB_API_KEY not set, skipping integration test")
	}

	return apiKey
}

func TestNew(t *testing.T) {
	t.Run("returns error when API key is empty", func(t *testing.T) {
		_, err := New("")
		if err == nil {
			t.Error("Expected error when API key is empty")
		}
	})

	t.Run("returns client when API key is provided", func(t *testing.T) {
		client, err := New("test-api-key")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if client == nil {
			t.Error("Expected client to be non-nil")
		}
	})
}

func TestGetTmdbMovieByID(t *testing.T) {
	apiKey := loadEnv(t)

	client, err := New(apiKey)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	t.Run("returns error when TmdbID is zero", func(t *testing.T) {
		movie := &TmdbMovie{TmdbID: 0}
		err := client.GetTmdbMovieByID(movie)
		if err == nil {
			t.Error("Expected error when TmdbID is zero")
		}
	})

	t.Run("fetches movie by valid ID and validates all fields", func(t *testing.T) {
		// The Matrix (1999) - TMDB ID: 603
		// This is a well-known movie with complete data, ideal for testing field mapping
		movie := &TmdbMovie{TmdbID: 603}
		err := client.GetTmdbMovieByID(movie)
		if err != nil {
			t.Fatalf("Failed to get movie: %v", err)
		}

		// Basic info
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

		// Images
		if movie.PosterPath == "" {
			t.Error("Expected poster_path to be populated")
		}

		if movie.BackdropPath == "" {
			t.Error("Expected backdrop_path to be populated")
		}

		// Ratings and popularity
		if movie.Popularity == 0 {
			t.Error("Expected popularity to be non-zero")
		}

		if movie.VoteAverage == 0 {
			t.Error("Expected vote_average to be non-zero")
		}

		if movie.VoteCount == 0 {
			t.Error("Expected vote_count to be non-zero")
		}

		// Movie details
		if movie.Runtime == 0 {
			t.Error("Expected runtime to be non-zero")
		}

		if movie.Status == "" {
			t.Error("Expected status to be populated")
		}

		if movie.OriginalLang == "" {
			t.Error("Expected original_language to be populated")
		}

		// Financial info (The Matrix had a budget)
		if movie.Budget == 0 {
			t.Error("Expected budget to be non-zero for The Matrix")
		}

		if movie.Revenue == 0 {
			t.Error("Expected revenue to be non-zero for The Matrix")
		}

		// External IDs
		if movie.ImdbID == "" {
			t.Error("Expected imdb_id to be populated")
		}

		// Genres (The Matrix has genres)
		if len(movie.Genres) == 0 {
			t.Error("Expected genres to be populated")
		} else {
			// Verify genre structure
			for _, genre := range movie.Genres {
				if genre.ID == 0 {
					t.Error("Expected genre ID to be non-zero")
				}
				if genre.Name == "" {
					t.Error("Expected genre name to be populated")
				}
			}
		}

		// Production companies
		if len(movie.ProductionCompanies) == 0 {
			t.Error("Expected production_companies to be populated")
		} else {
			// Verify production company structure
			if movie.ProductionCompanies[0].Name == "" {
				t.Error("Expected production company name to be populated")
			}
		}

		// Credits (cast and crew) - requested via append_to_response
		if len(movie.Credits.Cast) == 0 {
			t.Error("Expected credits.cast to be populated")
		} else {
			// Verify cast structure
			cast := movie.Credits.Cast[0]
			if cast.ID == 0 {
				t.Error("Expected cast member ID to be non-zero")
			}
			if cast.Name == "" {
				t.Error("Expected cast member name to be populated")
			}
			if cast.Character == "" {
				t.Error("Expected cast member character to be populated")
			}
		}

		if len(movie.Credits.Crew) == 0 {
			t.Error("Expected credits.crew to be populated")
		} else {
			// Verify crew structure
			crew := movie.Credits.Crew[0]
			if crew.ID == 0 {
				t.Error("Expected crew member ID to be non-zero")
			}
			if crew.Name == "" {
				t.Error("Expected crew member name to be populated")
			}
			if crew.Job == "" {
				t.Error("Expected crew member job to be populated")
			}
		}

		// Videos - requested via append_to_response
		if len(movie.Videos.Results) == 0 {
			t.Error("Expected videos.results to be populated")
		} else {
			// Verify video structure
			video := movie.Videos.Results[0]
			if video.Key == "" {
				t.Error("Expected video key to be populated")
			}
			if video.Site == "" {
				t.Error("Expected video site to be populated")
			}
			if video.Type == "" {
				t.Error("Expected video type to be populated")
			}
		}
	})

	t.Run("returns error for non-existent movie ID", func(t *testing.T) {
		movie := &TmdbMovie{TmdbID: 999999999}
		err := client.GetTmdbMovieByID(movie)
		if err == nil {
			t.Error("Expected error for non-existent movie ID")
		}
	})
}

func TestGetTmdbMovieByTitle(t *testing.T) {
	apiKey := loadEnv(t)

	client, err := New(apiKey)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	t.Run("returns error when title is empty", func(t *testing.T) {
		movie := &TmdbMovie{Title: ""}
		err := client.GetTmdbMovieByTitle(movie)
		if err == nil {
			t.Error("Expected error when title is empty")
		}
	})

	t.Run("fetches movie by title", func(t *testing.T) {
		movie := &TmdbMovie{Title: "Inception"}
		err := client.GetTmdbMovieByTitle(movie)
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
		err := client.GetTmdbMovieByTitle(movie)
		if err == nil {
			t.Error("Expected error for non-existent movie title")
		}
	})
}

func TestSearchMoviesByTitleAndYear(t *testing.T) {
	apiKey := loadEnv(t)

	client, err := New(apiKey)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	t.Run("returns error when title is empty", func(t *testing.T) {
		_, err := client.SearchMoviesByTitleAndYear("")
		if err == nil {
			t.Error("Expected error when title is empty")
		}
	})

	t.Run("searches movies by title only", func(t *testing.T) {
		movies, err := client.SearchMoviesByTitleAndYear("Inception")
		if err != nil {
			t.Fatalf("Failed to search movies: %v", err)
		}

		if len(movies) == 0 {
			t.Error("Expected at least one movie result")
		}
	})

	t.Run("searches movies by title and year", func(t *testing.T) {
		movies, err := client.SearchMoviesByTitleAndYear("The Matrix", 1999)
		if err != nil {
			t.Fatalf("Failed to search movies: %v", err)
		}

		if len(movies) == 0 {
			t.Error("Expected at least one movie result")
		}

		// Verify at least one result is from 1999
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

	t.Run("returns error when no movies match year filter", func(t *testing.T) {
		// Search for a movie with an impossible year
		_, err := client.SearchMoviesByTitleAndYear("The Matrix", 1850)
		if err == nil {
			t.Error("Expected error when no movies match year filter")
		}
	})
}

func TestGetMoviesInTheaters(t *testing.T) {
	apiKey := loadEnv(t)

	client, err := New(apiKey)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	t.Run("fetches movies currently in theaters", func(t *testing.T) {
		movies, err := client.GetMoviesInTheaters()
		if err != nil {
			t.Fatalf("Failed to get movies in theaters: %v", err)
		}

		if len(movies) == 0 {
			t.Error("Expected at least one movie in theaters")
		}

		// Verify first movie has basic data
		if movies[0].TmdbID == 0 {
			t.Error("Expected TmdbID to be populated")
		}

		if movies[0].Title == "" {
			t.Error("Expected title to be populated")
		}
	})
}

func TestGetTmdbPopularMovies(t *testing.T) {
	apiKey := loadEnv(t)

	client, err := New(apiKey)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	t.Run("fetches popular movies with default region", func(t *testing.T) {
		movies, err := client.GetTmdbPopularMovies()
		if err != nil {
			t.Fatalf("Failed to get popular movies: %v", err)
		}

		if len(movies) == 0 {
			t.Error("Expected at least one popular movie")
		}

		// Verify first movie has basic data
		if movies[0].TmdbID == 0 {
			t.Error("Expected TmdbID to be populated")
		}

		if movies[0].Title == "" {
			t.Error("Expected title to be populated")
		}
	})

	t.Run("fetches popular movies with custom region", func(t *testing.T) {
		movies, err := client.GetTmdbPopularMovies("GB")
		if err != nil {
			t.Fatalf("Failed to get popular movies for GB: %v", err)
		}

		if len(movies) == 0 {
			t.Error("Expected at least one popular movie for GB region")
		}
	})
}
