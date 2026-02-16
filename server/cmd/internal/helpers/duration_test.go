package helpers

import (
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
