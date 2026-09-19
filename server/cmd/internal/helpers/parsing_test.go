package helpers

import (
	"math"
	"testing"
)

func TestParseFrameRate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected float64
	}{
		{"empty", "", 0},
		{"decimal 23.976", "23.976", 23.976},
		{"decimal 24", "24", 24},
		{"fraction 24000/1001", "24000/1001", 24000.0 / 1001},
		{"fraction with spaces", " 24000 / 1001 ", 24000.0 / 1001},
		{"invalid fraction one part", "24000/", 0},
		{"invalid fraction three parts", "24000/1001/1", 0},
		{"invalid decimal", "abc", 0},
		{"zero denominator", "1/0", 0},
		{"not a number", "NaN", 0},
		{"infinite", "Inf", 0},
		{"negative decimal", "-24", 0},
		{"negative fraction", "-24000/1001", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseFrameRate(tt.input)
			if got != tt.expected {
				t.Errorf("ParseFrameRate(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
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

func TestParseDurationSeconds(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      float64
		wantValid bool
	}{
		{name: "whole seconds", input: "245", want: 245, wantValid: true},
		{name: "fractional seconds", input: "245.123456", want: 245.123456, wantValid: true},
		{name: "surrounding whitespace", input: " 245.5 ", want: 245.5, wantValid: true},
		{name: "zero", input: "0", want: 0, wantValid: true},
		{name: "empty", input: "", wantValid: false},
		{name: "ffprobe N/A", input: "N/A", wantValid: false},
		{name: "negative", input: "-1", wantValid: false},
		{name: "infinity", input: "Inf", wantValid: false},
		{name: "not a number", input: "NaN", wantValid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseDurationSeconds(tt.input)
			if ok != tt.wantValid {
				t.Fatalf("ParseDurationSeconds(%q) ok = %v, want %v", tt.input, ok, tt.wantValid)
			}
			if ok && got != tt.want {
				t.Errorf("ParseDurationSeconds(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseDurationMs(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int64
		wantErr bool
	}{
		{name: "fractional seconds", input: "245.123456", want: 245123},
		{name: "whole seconds", input: "10", want: 10000},
		{name: "zero", input: "0", want: 0},
		{name: "empty", input: "", wantErr: true},
		{name: "ffprobe N/A", input: "N/A", wantErr: true},
		{name: "infinity", input: "Inf", wantErr: true},
		{name: "not a number", input: "NaN", wantErr: true},
		{name: "negative", input: "-3.5", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDurationMs(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseDurationMs(%q) = %d, want an error", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseDurationMs(%q) returned error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("ParseDurationMs(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseSlashNumber(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int64
		wantErr bool
	}{
		{name: "track of total", input: "1/12", want: 1},
		{name: "bare number", input: "7", want: 7},
		{name: "surrounding whitespace", input: " 3 /12", want: 3},
		{name: "empty", input: "", wantErr: true},
		{name: "leading separator", input: "/12", wantErr: true},
		{name: "not a number", input: "A/12", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSlashNumber(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseSlashNumber(%q) = %d, want an error", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseSlashNumber(%q) returned error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("ParseSlashNumber(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseDate(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "ISO 8601", input: "2011-09-23", want: "2011-09-23"},
		{name: "ISO 8601 without leading zeros", input: "2011-9-3", want: "2011-09-03"},
		{name: "ISO 8601 with time", input: "2011-09-23T14:30:00", want: "2011-09-23"},
		{name: "year only", input: "2011", want: "2011-01-01"},
		{name: "US format", input: "09/23/2011", want: "2011-09-23"},
		{name: "European format", input: "23-09-2011", want: "2011-09-23"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDate(tt.input)
			if err != nil {
				t.Fatalf("ParseDate(%q) returned error: %v", tt.input, err)
			}
			if got.Format("2006-01-02") != tt.want {
				t.Errorf("ParseDate(%q) = %s, want %s", tt.input, got.Format("2006-01-02"), tt.want)
			}
		})
	}
}

func TestParseDateRejectsUnknownFormats(t *testing.T) {
	for _, input := range []string{"", "not a date", "23 September 2011", "2011/09/23"} {
		got, err := ParseDate(input)
		if err == nil {
			t.Errorf("ParseDate(%q) = %v, want an error", input, got)
		}
	}
}
