package helpers

import (
	"testing"
	"time"
)

func TestFormatDate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected time.Time
		wantErr  bool
	}{
		{
			name:     "ISO 8601 with Z timezone",
			input:    "2023-12-25T10:30:45Z",
			expected: time.Date(2023, 12, 25, 10, 30, 45, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "ISO 8601 with microseconds and Z",
			input:    "2023-12-25T10:30:45.123456Z",
			expected: time.Date(2023, 12, 25, 10, 30, 45, 123456000, time.UTC),
			wantErr:  false,
		},
		{
			name:     "ISO 8601 with timezone offset",
			input:    "2023-12-25T10:30:45-05:00",
			expected: time.Date(2023, 12, 25, 10, 30, 45, 0, time.FixedZone("", -5*60*60)),
			wantErr:  false,
		},
		{
			name:     "ISO 8601 with microseconds and timezone offset",
			input:    "2023-12-25T10:30:45.123456-05:00",
			expected: time.Date(2023, 12, 25, 10, 30, 45, 123456000, time.FixedZone("", -5*60*60)),
			wantErr:  false,
		},
		{
			name:     "Date only",
			input:    "2023-12-25",
			expected: time.Date(2023, 12, 25, 0, 0, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "Year only - should set to January 1st",
			input:    "2008",
			expected: time.Date(2008, 1, 1, 0, 0, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "Year only - different year",
			input:    "1995",
			expected: time.Date(1995, 1, 1, 0, 0, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "Year only - future year",
			input:    "2030",
			expected: time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "Invalid date format",
			input:    "not-a-date",
			expected: time.Time{},
			wantErr:  true,
		},
		{
			name:     "Empty string",
			input:    "",
			expected: time.Time{},
			wantErr:  true,
		},
		{
			name:     "Invalid year format",
			input:    "20ab",
			expected: time.Time{},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := FormatDate(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("FormatDate() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("FormatDate() unexpected error: %v", err)
				return
			}

			// Compare the result with expected time
			if !result.Equal(tt.expected) {
				t.Errorf("FormatDate() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestFormatDate_YearOnlyEdgeCases(t *testing.T) {
	t.Run("various year formats", func(t *testing.T) {
		yearTests := []struct {
			input        string
			expectedYear int
		}{
			{"2000", 2000},
			{"1999", 1999},
			{"2024", 2024},
			{"1900", 1900},
			{"2100", 2100},
		}

		for _, yt := range yearTests {
			result, err := FormatDate(yt.input)
			if err != nil {
				t.Errorf("FormatDate(%s) unexpected error: %v", yt.input, err)
				continue
			}

			expected := time.Date(yt.expectedYear, 1, 1, 0, 0, 0, 0, time.UTC)
			if !result.Equal(expected) {
				t.Errorf("FormatDate(%s) = %v, expected %v", yt.input, result, expected)
			}

			// Verify it's specifically January 1st
			if result.Month() != 1 || result.Day() != 1 {
				t.Errorf("FormatDate(%s) should be January 1st, got %s", yt.input, result.Format("January 2"))
			}
		}
	})
}



