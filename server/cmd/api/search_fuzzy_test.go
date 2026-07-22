package main

import (
	"context"
	"slices"
	"testing"
)

func testVocabCorrections(app *Application, ctx context.Context, vocabTable, token string) ([]string, error) {
	tokenLen := len([]rune(token))
	maxDist := searchTypoMaxDist(tokenLen)
	if maxDist == 0 {
		return nil, nil
	}

	index, err := app.searchVocabIndex(ctx, vocabTable)
	if err != nil {
		return nil, err
	}

	corrections, _ := index.corrections(token, maxDist, searchVocabMaxVisited)
	return corrections, nil
}

func TestSearchTokens(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{
			name: "simple tokens",
			raw:  "Casino Royale",
			want: []string{"casino", "royale"},
		},
		{
			name: "keeps unicode letters for unicode61 tokenizer",
			raw:  "Beyoncé año",
			want: []string{"beyoncé", "año"},
		},
		{
			name: "strips fts syntax",
			raw:  `"casino" OR title:royale`,
			want: []string{"casino", "or", "title", "royale"},
		},
		{
			name: "punctuation only",
			raw:  `"'():`,
			want: nil,
		},
		{
			name: "blank",
			raw:  "   ",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := searchTokens(tt.raw)
			if !slices.Equal(got, tt.want) {
				t.Fatalf("searchTokens(%q) = %#v, want %#v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestPrefixQueries(t *testing.T) {
	plain := [][]string{{"casino"}, {"royale"}}
	if got := andPrefixQuery(plain); got != "casino* AND royale*" {
		t.Fatalf("andPrefixQuery(plain) = %q", got)
	}
	if got := orPrefixQuery(plain); got != "casino* OR royale*" {
		t.Fatalf("orPrefixQuery(plain) = %q", got)
	}

	corrected := [][]string{{"license", "licence"}, {"kill"}}
	wantAnd := `(license* OR "licence"*) AND kill*`
	if got := andPrefixQuery(corrected); got != wantAnd {
		t.Fatalf("andPrefixQuery(corrected) = %q, want %q", got, wantAnd)
	}
	wantOr := `license* OR "licence"* OR kill*`
	if got := orPrefixQuery(corrected); got != wantOr {
		t.Fatalf("orPrefixQuery(corrected) = %q, want %q", got, wantOr)
	}
}

func TestDamerauLevenshtein(t *testing.T) {
	tests := []struct {
		name    string
		a, b    string
		maxDist int
		want    int
	}{
		{name: "identical", a: "kill", b: "kill", maxDist: 2, want: 0},
		{name: "substitution", a: "license", b: "licence", maxDist: 2, want: 1},
		{name: "transposition", a: "teh", b: "the", maxDist: 2, want: 1},
		{name: "two edits", a: "lisense", b: "licence", maxDist: 2, want: 2},
		{name: "insertion", a: "adelle", b: "adele", maxDist: 2, want: 1},
		{name: "unicode runes", a: "beyonce", b: "beyoncé", maxDist: 2, want: 1},
		{name: "exceeds cap", a: "casino", b: "royale", maxDist: 2, want: 3},
		{name: "length gap exceeds cap", a: "abc", b: "abcdefgh", maxDist: 2, want: 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := damerauLevenshtein(tt.a, tt.b, tt.maxDist)
			if got != tt.want {
				t.Fatalf("damerauLevenshtein(%q, %q, %d) = %d, want %d", tt.a, tt.b, tt.maxDist, got, tt.want)
			}
		})
	}
}

func TestVocabCorrections(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	createSearchMovie(t, app, "Licence to Kill", "/movies/licence-to-kill.mkv")

	corrections, err := testVocabCorrections(app, context.Background(), "movies_fts_vocab", "license")
	if err != nil {
		t.Fatalf("vocabCorrections failed: %v", err)
	}
	if !slices.Contains(corrections, "licence") {
		t.Fatalf("expected licence correction, got %#v", corrections)
	}

	corrections, err = testVocabCorrections(app, context.Background(), "movies_fts_vocab", "to")
	if err != nil {
		t.Fatalf("vocabCorrections short token failed: %v", err)
	}
	if corrections != nil {
		t.Fatalf("expected no corrections for short token, got %#v", corrections)
	}
}
