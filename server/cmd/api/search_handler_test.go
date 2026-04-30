package main

import "testing"

func TestBuildFTSQuery(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
		ok   bool
	}{
		{
			name: "simple prefix tokens",
			raw:  "Casino Royale",
			want: "casino* royale*",
			ok:   true,
		},
		{
			name: "keeps unicode letters for unicode61 tokenizer",
			raw:  "Beyoncé año",
			want: "beyoncé* año*",
			ok:   true,
		},
		{
			name: "strips fts syntax",
			raw:  `"casino" OR title:royale`,
			want: "casino* or* title* royale*",
			ok:   true,
		},
		{
			name: "punctuation only",
			raw:  `"'():`,
			ok:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := buildFTSQuery(tt.raw)
			if ok != tt.ok {
				t.Fatalf("buildFTSQuery(%q) ok = %v, want %v", tt.raw, ok, tt.ok)
			}
			if got != tt.want {
				t.Fatalf("buildFTSQuery(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestNormalizeSearchPage(t *testing.T) {
	tests := []struct {
		name      string
		page      int64
		total     int64
		perPage   int64
		wantPage  int64
		wantPages int64
	}{
		{
			name:      "keeps in range page",
			page:      2,
			total:     50,
			perPage:   24,
			wantPage:  2,
			wantPages: 3,
		},
		{
			name:      "clamps overlarge page",
			page:      999,
			total:     50,
			perPage:   24,
			wantPage:  3,
			wantPages: 3,
		},
		{
			name:      "empty result resets to first page",
			page:      999,
			total:     0,
			perPage:   24,
			wantPage:  1,
			wantPages: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPage, gotPages := normalizeSearchPage(tt.page, tt.total, tt.perPage)
			if gotPage != tt.wantPage {
				t.Fatalf("normalizeSearchPage page = %d, want %d", gotPage, tt.wantPage)
			}
			if gotPages != tt.wantPages {
				t.Fatalf("normalizeSearchPage pages = %d, want %d", gotPages, tt.wantPages)
			}
		})
	}
}
