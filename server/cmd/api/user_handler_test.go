package main

import (
	"strings"
	"testing"
)

func TestValidatePasswordCountsRunes(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{
			name:    "eight multibyte characters is too short",
			value:   strings.Repeat("界", 8),
			wantErr: true,
		},
		{
			name:  "nine multibyte characters is accepted",
			value: strings.Repeat("界", 9),
		},
		{
			name:  "one hundred twenty eight multibyte characters is accepted",
			value: strings.Repeat("界", 128),
		},
		{
			name:    "one hundred twenty nine multibyte characters is too long",
			value:   strings.Repeat("界", 129),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePassword(tt.value, "password")
			if tt.wantErr && err == nil {
				t.Fatal("validatePassword returned nil error, want error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("validatePassword returned error: %v", err)
			}
		})
	}
}
