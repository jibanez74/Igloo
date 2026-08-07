package helpers

import "testing"

func TestNormalizeComparisonText(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{"diacritics stripped and lowercased", "Beyoncé", "beyonce"},
		{"slash collapses to space", "AC/DC", "ac dc"},
		{"comma dropped and ampersand spelled out", "Earth, Wind & Fire", "earth wind and fire"},
		{"plus spelled out", "Mike + The Mechanics", "mike and the mechanics"},
		{"punctuation collapses to single spaces", "Sgt. Pepper's!!", "sgt pepper s"},
		{"whitespace trimmed and collapsed", "  The   Beatles  ", "the beatles"},
		{"punctuation only normalizes to empty", "!!!", ""},
		{"empty input", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeComparisonText(tt.value)
			if got != tt.want {
				t.Fatalf("NormalizeComparisonText(%q) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func TestNormalizeMBID(t *testing.T) {
	tests := []struct {
		name   string
		value  string
		want   string
		wantOK bool
	}{
		{
			"lowercase uuid accepted",
			"859d0860-d480-4efd-970c-c05d5f1882b9",
			"859d0860-d480-4efd-970c-c05d5f1882b9",
			true,
		},
		{
			"uppercase uuid lowercased",
			"859D0860-D480-4EFD-970C-C05D5F1882B9",
			"859d0860-d480-4efd-970c-c05d5f1882b9",
			true,
		},
		{
			"surrounding whitespace trimmed",
			"  859d0860-d480-4efd-970c-c05d5f1882b9  ",
			"859d0860-d480-4efd-970c-c05d5f1882b9",
			true,
		},
		{"empty rejected", "", "", false},
		{"too short rejected", "859d0860-d480-4efd-970c", "", false},
		{"too long rejected", "859d0860-d480-4efd-970c-c05d5f1882b9ff", "", false},
		{
			"misplaced hyphens rejected",
			"859d0860d-480-4efd-970c-c05d5f1882b9",
			"",
			false,
		},
		{
			"non-hex characters rejected",
			"859d0860-d480-4efd-970c-c05d5f1882bz",
			"",
			false,
		},
		{
			"multi-value tag rejected",
			"859d0860-d480-4efd-970c-c05d5f1882b9/6f70b8b9-84fe-4ef8-b3ff-1ac5a3a0f0e1",
			"",
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := NormalizeMBID(tt.value)
			if got != tt.want || ok != tt.wantOK {
				t.Fatalf("NormalizeMBID(%q) = (%q, %t), want (%q, %t)", tt.value, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}
