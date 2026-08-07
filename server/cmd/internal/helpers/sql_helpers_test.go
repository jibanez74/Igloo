package helpers

import (
	"database/sql"
	"math"
	"testing"
)

func TestNullString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected sql.NullString
	}{
		{"empty", "", sql.NullString{}},
		{"value", "value", sql.NullString{String: "value", Valid: true}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NullString(tt.input)
			if got != tt.expected {
				t.Errorf("NullString(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestNullInt64(t *testing.T) {
	tests := []struct {
		name     string
		input    int64
		expected sql.NullInt64
	}{
		{"zero", 0, sql.NullInt64{}},
		{"value", 42, sql.NullInt64{Int64: 42, Valid: true}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NullInt64(tt.input)
			if got != tt.expected {
				t.Errorf("NullInt64(%d) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestNullFloat64(t *testing.T) {
	tests := []struct {
		name     string
		input    float64
		expected sql.NullFloat64
	}{
		{"zero", 0, sql.NullFloat64{}},
		{"value", 1.25, sql.NullFloat64{Float64: 1.25, Valid: true}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NullFloat64(tt.input)
			if got != tt.expected {
				t.Errorf("NullFloat64(%f) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestNullFloat64FromPtr(t *testing.T) {
	zero := 0.0
	value := 2.5

	tests := []struct {
		name     string
		input    *float64
		expected sql.NullFloat64
	}{
		{"nil", nil, sql.NullFloat64{}},
		{"zero", &zero, sql.NullFloat64{}},
		{"value", &value, sql.NullFloat64{Float64: value, Valid: true}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NullFloat64FromPtr(tt.input)
			if got != tt.expected {
				t.Errorf("NullFloat64FromPtr(%v) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestStringPtrFromNull(t *testing.T) {
	if got := StringPtrFromNull(sql.NullString{}); got != nil {
		t.Fatalf("StringPtrFromNull(invalid) = %v, want nil", got)
	}

	got := StringPtrFromNull(sql.NullString{String: "value", Valid: true})
	if got == nil || *got != "value" {
		t.Fatalf("StringPtrFromNull(valid) = %v, want pointer to value", got)
	}
}

func TestFloat64PtrFromNull(t *testing.T) {
	if got := Float64PtrFromNull(sql.NullFloat64{}); got != nil {
		t.Fatalf("Float64PtrFromNull(invalid) = %v, want nil", got)
	}

	got := Float64PtrFromNull(sql.NullFloat64{Float64: 3.5, Valid: true})
	if got == nil || *got != 3.5 {
		t.Fatalf("Float64PtrFromNull(valid) = %v, want pointer to value", got)
	}
}

func TestClampFloat64(t *testing.T) {
	tests := []struct {
		name     string
		v        float64
		min      float64
		max      float64
		expected float64
	}{
		{"within range", 5.0, 0.0, 10.0, 5.0},
		{"at minimum", 0.0, 0.0, 10.0, 0.0},
		{"at maximum", 10.0, 0.0, 10.0, 10.0},
		{"below minimum", -5.0, 0.0, 10.0, 0.0},
		{"above maximum", 15.0, 0.0, 10.0, 10.0},
		{"negative range", -3.0, -10.0, -1.0, -3.0},
		{"zero range", 5.0, 3.0, 3.0, 3.0},
		{"fractional values", 0.5, 0.0, 1.0, 0.5},
		{"large values", 99999.0, 0.0, 7200.0, 7200.0},
		{"inverted bounds clamp high", 7.0, 10.0, 5.0, 7.0},
		{"inverted bounds clamp low", 3.0, 10.0, 5.0, 5.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClampFloat64(tt.v, tt.min, tt.max)
			if got != tt.expected {
				t.Errorf("ClampFloat64(%f, %f, %f) = %f, want %f",
					tt.v, tt.min, tt.max, got, tt.expected)
			}
		})
	}
}

func TestClampFloat64_NaNPropagates(t *testing.T) {
	nan := math.NaN()
	if !math.IsNaN(ClampFloat64(nan, 0, 1)) {
		t.Error("expected NaN when v is NaN")
	}
	if !math.IsNaN(ClampFloat64(1, nan, 2)) {
		t.Error("expected NaN when min is NaN")
	}
	if !math.IsNaN(ClampFloat64(1, 0, nan)) {
		t.Error("expected NaN when max is NaN")
	}
}

func TestParseBitRate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int64
	}{
		{
			name:     "valid bitrate",
			input:    "5000000",
			expected: 5000000,
		},
		{
			name:     "empty string",
			input:    "",
			expected: 0,
		},
		{
			name:     "invalid format",
			input:    "invalid",
			expected: 0,
		},
		{
			name:     "zero",
			input:    "0",
			expected: 0,
		},
		{
			name:     "very large number",
			input:    "999999999999",
			expected: 999999999999,
		},
		{
			name:     "negative number",
			input:    "-1000",
			expected: -1000,
		},
		{
			name:     "bitrate with decimal",
			input:    "5000.5",
			expected: 0,
		},
		{
			name:     "bitrate with spaces",
			input:    " 5000000 ",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseBitRate(tt.input)
			if result != tt.expected {
				t.Errorf("ParseBitRate(%q) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseDate(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantYear  int
		wantMonth int
		wantDay   int
	}{
		{"iso date", "2004-06-17", 2004, 6, 17},
		{"iso without leading zeros", "2004-6-7", 2004, 6, 7},
		{"iso with naive time", "2004-06-17T12:00:00", 2004, 6, 17},
		{"rfc3339 utc", "2004-06-17T12:00:00Z", 2004, 6, 17},
		{"rfc3339 offset", "2004-06-17T12:00:00-05:00", 2004, 6, 17},
		{"iso with space time", "2004-06-17 12:00:00", 2004, 6, 17},
		{"year and month", "2004-06", 2004, 6, 1},
		{"year only", "2004", 2004, 1, 1},
		{"us format", "06/17/2004", 2004, 6, 17},
		{"european format", "17-06-2004", 2004, 6, 17},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDate(tt.input)
			if err != nil {
				t.Fatalf("ParseDate(%q) error = %v", tt.input, err)
			}
			if got.Year() != tt.wantYear || int(got.Month()) != tt.wantMonth || got.Day() != tt.wantDay {
				t.Errorf("ParseDate(%q) = %v, want %04d-%02d-%02d", tt.input, got, tt.wantYear, tt.wantMonth, tt.wantDay)
			}
		})
	}
}

func TestParseDateRejectsUnparseable(t *testing.T) {
	for _, input := range []string{"", "not a date", "17th of June 2004"} {
		_, err := ParseDate(input)
		if err == nil {
			t.Errorf("ParseDate(%q) = nil error, want failure", input)
		}
	}
}
