package helpers

import (
	"math"
	"testing"
)

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
