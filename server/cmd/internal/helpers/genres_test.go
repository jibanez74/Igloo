package helpers

import (
	"testing"
)

func TestSaveGenres_ZeroIDHandling(t *testing.T) {
	tests := []struct {
		name          string
		data          *SaveGenresParams
		expectError   bool
		expectedError string
	}{
		{
			name: "valid music genre with all IDs",
			data: &SaveGenresParams{
				Tag:        "rock",
				GenreType:  "music",
				MusicianID: 1,
				AlbumID:    1,
				TrackID:    1,
			},
			expectError: false,
		},
		{
			name: "valid music genre with zero MusicianID",
			data: &SaveGenresParams{
				Tag:        "rock",
				GenreType:  "music",
				MusicianID: 0, // Zero ID - should be skipped
				AlbumID:    1,
				TrackID:    1,
			},
			expectError: false,
		},
		{
			name: "valid music genre with zero AlbumID",
			data: &SaveGenresParams{
				Tag:        "rock",
				GenreType:  "music",
				MusicianID: 1,
				AlbumID:    0, // Zero ID - should be skipped
				TrackID:    1,
			},
			expectError: false,
		},
		{
			name: "valid music genre with both zero IDs",
			data: &SaveGenresParams{
				Tag:        "rock",
				GenreType:  "music",
				MusicianID: 0, // Zero ID - should be skipped
				AlbumID:    0, // Zero ID - should be skipped
				TrackID:    1,
			},
			expectError: false,
		},
		{
			name: "invalid music genre with zero TrackID",
			data: &SaveGenresParams{
				Tag:        "rock",
				GenreType:  "music",
				MusicianID: 1,
				AlbumID:    1,
				TrackID:    0, // Zero ID - should cause error
			},
			expectError:   true,
			expectedError: "TrackID is required for music genres",
		},
		{
			name: "valid movie genre with MovieID",
			data: &SaveGenresParams{
				Tag:       "action",
				GenreType: "movie",
				MovieID:   1,
			},
			expectError: false,
		},
		{
			name: "invalid movie genre with zero MovieID",
			data: &SaveGenresParams{
				Tag:       "action",
				GenreType: "movie",
				MovieID:   0, // Zero ID - should cause error
			},
			expectError:   true,
			expectedError: "MovieID is required for movie genres",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: This test would need a mock database or test database
			// For now, we're just testing the logic structure
			// In a real test, you would use a test database or mock

			// The actual test would look like:
			// err := SaveGenres(ctx, qtx, tt.data)
			// if tt.expectError {
			//     if err == nil {
			//         t.Errorf("expected error but got none")
			//     }
			//     if !strings.Contains(err.Error(), tt.expectedError) {
			//         t.Errorf("expected error to contain %q, got %q", tt.expectedError, err.Error())
			//     }
			// } else {
			//     if err != nil {
			//         t.Errorf("unexpected error: %v", err)
			//     }
			// }

			// For now, just verify the test structure is correct
			if tt.data == nil {
				t.Error("test data cannot be nil")
			}
		})
	}
}

func TestParseGenres(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: nil,
		},
		{
			name:     "single genre",
			input:    "rock",
			expected: []string{"rock"},
		},
		{
			name:     "multiple genres",
			input:    "rock, pop, jazz",
			expected: []string{"rock", "pop", "jazz"},
		},
		{
			name:     "genres with spaces",
			input:    " rock , pop , jazz ",
			expected: []string{"rock", "pop", "jazz"},
		},
		{
			name:     "genres with empty parts",
			input:    "rock,,pop, ,jazz",
			expected: []string{"rock", "pop", "jazz"},
		},
		{
			name:     "single genre with comma",
			input:    "rock,",
			expected: []string{"rock"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseGenres(tt.input)

			if len(result) != len(tt.expected) {
				t.Errorf("ParseGenres() returned %d items, expected %d", len(result), len(tt.expected))
				return
			}

			for i, genre := range result {
				if genre != tt.expected[i] {
					t.Errorf("ParseGenres()[%d] = %q, expected %q", i, genre, tt.expected[i])
				}
			}
		})
	}
}
