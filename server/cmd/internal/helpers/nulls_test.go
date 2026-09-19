package helpers

import (
	"database/sql"
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
