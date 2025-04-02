package tmdb

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func TestGetTmdbMovieByID(t *testing.T) {
	// Create a test API key
	apiKey := "test-api-key"

	// Test cases
	tests := []struct {
		name           string
		tmdbID         *string
		serverResponse *tmdbMovie
		serverStatus   int
		expectedError  error
	}{
		{
			name:   "successful API call",
			tmdbID: stringPtr("12345"),
			serverResponse: &tmdbMovie{
				Title:          "Test Movie",
				Adult:          false,
				TagLine:        "A test movie",
				Summary:        "This is a test movie summary",
				Budget:         1000000,
				Revenue:        5000000,
				RunTime:        120,
				AudienceRating: 7.5,
				ImdbID:         "tt1234567",
				ReleaseDate:    "2023-01-01",
				Thumb:          "/poster.jpg",
				Art:            "/backdrop.jpg",
				SpokenLanguages: []tmdbLanguage{
					{ISO639_1: "en", Name: "English"},
				},
				Genres: []tmdbGenre{
					{ID: 28, Tag: "Action"},
				},
				Studios: []tmdbStudio{
					{ID: 1, Name: "Test Studio", Logo: "/logo.png", Country: "US"},
				},
				Credits: tmdbCredits{
					Cast: []struct {
						ID           int32  `json:"id"`
						Name         string `json:"name"`
						OriginalName string `json:"original_name"`
						Thumb        string `json:"profile_path"`
						Character    string `json:"character"`
						SortOrder    int32  `json:"order"`
					}{
						{ID: 1, Name: "Actor 1", Character: "Character 1", SortOrder: 1},
					},
					Crew: []struct {
						ID           int32  `json:"id"`
						Name         string `json:"name"`
						OriginalName string `json:"original_name"`
						Thumb        string `json:"profile_path"`
						Job          string `json:"job"`
						Department   string `json:"department"`
					}{
						{ID: 2, Name: "Director 1", Job: "Director", Department: "Directing"},
					},
				},
				Videos: tmdbVideos{
					Results: []tmdbVideo{
						{ID: "1", Site: "YouTube", Key: "abc123", Name: "Trailer", Type: "Trailer", Official: true},
					},
				},
				ReleaseDates: tmdbReleaseDates{
					Results: []struct {
						Country      string            `json:"iso_3166_1"`
						ReleaseDates []tmdbReleaseDate `json:"release_dates"`
					}{
						{
							Country: "US",
							ReleaseDates: []tmdbReleaseDate{
								{Country: "US", Certification: "PG-13", ReleaseDate: "2023-01-01"},
							},
						},
					},
				},
			},
			serverStatus:  fiber.StatusOK,
			expectedError: nil,
		},
		{
			name:           "nil tmdbID",
			tmdbID:         nil,
			serverResponse: nil,
			serverStatus:   fiber.StatusOK,
			expectedError:  errors.New("tmdb id is required"),
		},
		{
			name:           "rate limit exceeded",
			tmdbID:         stringPtr("12345"),
			serverResponse: nil,
			serverStatus:   fiber.StatusTooManyRequests,
			expectedError:  errors.New("rate limit exceeded for tmdb"),
		},
		{
			name:           "server error",
			tmdbID:         stringPtr("12345"),
			serverResponse: nil,
			serverStatus:   fiber.StatusInternalServerError,
			expectedError:  errors.New("unable to get movie from tmdb"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Skip request verification for nil tmdbID case
				if tt.tmdbID != nil {
					// Verify request
					assert.Equal(t, "GET", r.Method)
					assert.Equal(t, "/movie/"+*tt.tmdbID, r.URL.Path)
					assert.Equal(t, "test-api-key", r.URL.Query().Get("api_key"))
					assert.Equal(t, "credits,videos,release_dates", r.URL.Query().Get("append_to_response"))
					assert.Equal(t, "application/json", r.Header.Get("Accept"))
					assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
				}

				// Set response status
				w.WriteHeader(tt.serverStatus)

				// Return mock response if provided
				if tt.serverResponse != nil {
					json.NewEncoder(w).Encode(tt.serverResponse)
				}
			}))
			defer server.Close()

			// Create TMDB client with test server URL
			client, err := New(&apiKey)
			assert.NoError(t, err)
			assert.NotNil(t, client)

			// Override the base URL to point to our test server
			tmdbClient := client.(*tmdb)
			tmdbClient.baseUrl = server.URL

			// Call the function
			result, err := tmdbClient.GetTmdbMovieByID(tt.tmdbID)

			// Check error
			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError.Error(), err.Error())
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.serverResponse.Title, result.Title)
				assert.Equal(t, tt.serverResponse.Adult, result.Adult)
				assert.Equal(t, tt.serverResponse.TagLine, result.TagLine)
				assert.Equal(t, tt.serverResponse.Summary, result.Summary)
				assert.Equal(t, tt.serverResponse.Budget, result.Budget)
				assert.Equal(t, tt.serverResponse.Revenue, result.Revenue)
				assert.Equal(t, tt.serverResponse.RunTime, result.RunTime)
				assert.Equal(t, tt.serverResponse.AudienceRating, result.AudienceRating)
				assert.Equal(t, tt.serverResponse.ImdbID, result.ImdbID)
				assert.Equal(t, tt.serverResponse.ReleaseDate, result.ReleaseDate)
				assert.Equal(t, tt.serverResponse.Thumb, result.Thumb)
				assert.Equal(t, tt.serverResponse.Art, result.Art)
				assert.Equal(t, len(tt.serverResponse.SpokenLanguages), len(result.SpokenLanguages))
				assert.Equal(t, len(tt.serverResponse.Genres), len(result.Genres))
				assert.Equal(t, len(tt.serverResponse.Studios), len(result.Studios))
				assert.Equal(t, len(tt.serverResponse.Credits.Cast), len(result.Credits.Cast))
				assert.Equal(t, len(tt.serverResponse.Credits.Crew), len(result.Credits.Crew))
				assert.Equal(t, len(tt.serverResponse.Videos.Results), len(result.Videos.Results))
				assert.Equal(t, len(tt.serverResponse.ReleaseDates.Results), len(result.ReleaseDates.Results))
			}
		})
	}
}

// Helper function to create string pointer
func stringPtr(s string) *string {
	return &s
}
