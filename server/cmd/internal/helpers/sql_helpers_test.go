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
