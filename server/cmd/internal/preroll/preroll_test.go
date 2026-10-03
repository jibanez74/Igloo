package preroll

import (
	"math/rand/v2"
	"testing"
)

func seeded() *rand.Rand {
	return rand.New(rand.NewPCG(1, 2))
}

func library(ids ...int64) []Trailer {
	trailers := make([]Trailer, 0, len(ids))
	for _, id := range ids {
		trailers = append(trailers, Trailer{
			Title:      "Library " + string(rune('A'+id)),
			YouTubeKey: "lib-" + string(rune('a'+id)),
			Source:     SourceLibrary,
			MovieID:    id,
			TmdbID:     1000 + id,
		})
	}
	return trailers
}

func theaters(tmdbIDs ...int64) []Trailer {
	trailers := make([]Trailer, 0, len(tmdbIDs))
	for _, id := range tmdbIDs {
		trailers = append(trailers, Trailer{
			Title:      "Theaters",
			YouTubeKey: "th",
			Source:     SourceTheaters,
			TmdbID:     id,
		})
	}
	return trailers
}

func sourcesOf(queue []Trailer) []string {
	sources := make([]string, 0, len(queue))
	for _, trailer := range queue {
		sources = append(sources, trailer.Source)
	}
	return sources
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestSelect(t *testing.T) {
	cases := []struct {
		name        string
		prefs       Preferences
		library     []Trailer
		theaters    []Trailer
		exclude     int64
		wantSources []string
	}{
		{
			name:        "disabled yields nothing",
			prefs:       Preferences{Enabled: false, Count: 3, Source: SourceBoth},
			library:     library(1, 2, 3),
			theaters:    theaters(1, 2, 3),
			wantSources: []string{},
		},
		{
			name:        "library only",
			prefs:       Preferences{Enabled: true, Count: 2, Source: SourceLibrary},
			library:     library(1, 2, 3),
			theaters:    theaters(1, 2, 3),
			wantSources: []string{SourceLibrary, SourceLibrary},
		},
		{
			name:        "theaters only",
			prefs:       Preferences{Enabled: true, Count: 2, Source: SourceTheaters},
			library:     library(1, 2, 3),
			theaters:    theaters(1, 2, 3),
			wantSources: []string{SourceTheaters, SourceTheaters},
		},
		{
			name:        "both splits ceil(N/2) theaters first",
			prefs:       Preferences{Enabled: true, Count: 3, Source: SourceBoth},
			library:     library(1, 2, 3),
			theaters:    theaters(1, 2, 3),
			wantSources: []string{SourceTheaters, SourceTheaters, SourceLibrary},
		},
		{
			name:        "both with an even count",
			prefs:       Preferences{Enabled: true, Count: 4, Source: SourceBoth},
			library:     library(1, 2, 3),
			theaters:    theaters(1, 2, 3),
			wantSources: []string{SourceTheaters, SourceTheaters, SourceLibrary, SourceLibrary},
		},
		{
			name:        "both with a count of one plays a theaters trailer",
			prefs:       Preferences{Enabled: true, Count: 1, Source: SourceBoth},
			library:     library(1, 2, 3),
			theaters:    theaters(1, 2, 3),
			wantSources: []string{SourceTheaters},
		},
		{
			name:        "empty theaters pool tops up from the library",
			prefs:       Preferences{Enabled: true, Count: 3, Source: SourceBoth},
			library:     library(1, 2, 3, 4),
			theaters:    nil,
			wantSources: []string{SourceLibrary, SourceLibrary, SourceLibrary},
		},
		{
			name:        "theaters preference falls back to the library when TMDB is unavailable",
			prefs:       Preferences{Enabled: true, Count: 2, Source: SourceTheaters},
			library:     library(1, 2, 3),
			theaters:    nil,
			wantSources: []string{SourceLibrary, SourceLibrary},
		},
		{
			name:        "short library tops up from theaters",
			prefs:       Preferences{Enabled: true, Count: 4, Source: SourceBoth},
			library:     library(1),
			theaters:    theaters(1, 2, 3, 4, 5),
			wantSources: []string{SourceTheaters, SourceTheaters, SourceTheaters, SourceLibrary},
		},
		{
			name:        "library preference tops up from theaters",
			prefs:       Preferences{Enabled: true, Count: 3, Source: SourceLibrary},
			library:     library(1),
			theaters:    theaters(1, 2),
			wantSources: []string{SourceTheaters, SourceTheaters, SourceLibrary},
		},
		{
			name:        "both pools empty",
			prefs:       Preferences{Enabled: true, Count: 5, Source: SourceBoth},
			wantSources: []string{},
		},
		{
			name:        "never more than the count",
			prefs:       Preferences{Enabled: true, Count: 5, Source: SourceBoth},
			library:     library(1, 2, 3, 4, 5, 6, 7),
			theaters:    theaters(1, 2, 3, 4, 5, 6, 7),
			wantSources: []string{SourceTheaters, SourceTheaters, SourceTheaters, SourceLibrary, SourceLibrary},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			queue := Select(tc.prefs, tc.library, tc.theaters, tc.exclude, seeded())
			if queue == nil {
				t.Fatal("Select returned nil; want an empty, non-nil queue")
			}
			got := sourcesOf(queue)
			if !equalStrings(got, tc.wantSources) {
				t.Fatalf("sources = %v, want %v", got, tc.wantSources)
			}
		})
	}
}

func TestSelectExcludesTheCurrentMovieFromTheaters(t *testing.T) {
	prefs := Preferences{Enabled: true, Count: 3, Source: SourceTheaters}
	queue := Select(prefs, nil, theaters(10, 20, 30), 20, seeded())
	if len(queue) != 2 {
		t.Fatalf("len = %d, want 2 after excluding the current movie", len(queue))
	}
	for _, trailer := range queue {
		if trailer.TmdbID == 20 {
			t.Fatal("the movie about to play was selected")
		}
	}
}

func TestSelectDedupesLibraryAgainstTheatersByTmdbID(t *testing.T) {
	prefs := Preferences{Enabled: true, Count: 2, Source: SourceBoth}
	// The only theaters movie is also in the library under movie id 1.
	lib := library(1, 2)
	th := theaters(lib[0].TmdbID)
	queue := Select(prefs, lib, th, 0, seeded())
	if len(queue) != 2 {
		t.Fatalf("len = %d, want 2", len(queue))
	}
	if queue[0].Source != SourceTheaters || queue[1].MovieID != 2 {
		t.Fatalf("queue = %+v, want the theaters trailer then library movie 2", queue)
	}
}

func TestSelectKeepsLibraryItemsWithoutTmdbID(t *testing.T) {
	prefs := Preferences{Enabled: true, Count: 2, Source: SourceLibrary}
	lib := []Trailer{
		{Title: "A", YouTubeKey: "a", Source: SourceLibrary, MovieID: 1},
		{Title: "B", YouTubeKey: "b", Source: SourceLibrary, MovieID: 2},
	}
	queue := Select(prefs, lib, nil, 0, seeded())
	if len(queue) != 2 {
		t.Fatalf("len = %d, want 2: unknown TMDB ids must never collide", len(queue))
	}
}

func TestSelectIsDeterministicForASeed(t *testing.T) {
	prefs := Preferences{Enabled: true, Count: 3, Source: SourceTheaters}
	th := theaters(1, 2, 3, 4, 5, 6)
	first := Select(prefs, nil, th, 0, seeded())
	second := Select(prefs, nil, th, 0, seeded())
	for i := range first {
		if first[i].TmdbID != second[i].TmdbID {
			t.Fatalf("seeded selections differ: %+v vs %+v", first, second)
		}
	}
}

func TestValidatePreferences(t *testing.T) {
	cases := []struct {
		name  string
		prefs Preferences
		want  error
	}{
		{"defaults", DefaultPreferences(), nil},
		{"count below range", Preferences{Count: 0, Source: SourceBoth}, ErrInvalidCount},
		{"count above range", Preferences{Count: 6, Source: SourceBoth}, ErrInvalidCount},
		{"unknown source", Preferences{Count: 2, Source: "cinema"}, ErrInvalidSource},
		{"empty source", Preferences{Count: 2}, ErrInvalidSource},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidatePreferences(tc.prefs)
			if err != tc.want {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}
