package main

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"testing"
)

func testSearchVocabTerm(term string, doc int64) searchVocabTerm {
	return searchVocabTerm{term: term, runes: []rune(term), doc: doc}
}

func TestExactLevenshtein(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{a: "", b: "abc", want: 3},
		{a: "same", b: "same", want: 0},
		{a: "license", b: "licence", want: 1},
		{a: "teh", b: "the", want: 2},
		{a: "beyonce", b: "beyoncé", want: 1},
	}

	for _, tt := range tests {
		if got := exactLevenshtein([]rune(tt.a), []rune(tt.b)); got != tt.want {
			t.Errorf("exactLevenshtein(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestSearchVocabIndexCorrections(t *testing.T) {
	index := newSearchVocabIndex([]searchVocabTerm{
		testSearchVocabTerm("bat", 2),
		testSearchVocabTerm("cot", 10),
		testSearchVocabTerm("cut", 10),
		testSearchVocabTerm("the", 5),
		testSearchVocabTerm("beyoncé", 4),
	})

	corrections, _ := index.corrections("cat", 1, searchVocabMaxVisited)
	if want := []string{"cot", "cut", "bat"}; !slices.Equal(corrections, want) {
		t.Fatalf("corrections(cat) = %#v, want %#v", corrections, want)
	}

	corrections, _ = index.corrections("teh", 1, searchVocabMaxVisited)
	if !slices.Contains(corrections, "the") {
		t.Fatalf("expected transposition correction, got %#v", corrections)
	}

	corrections, _ = index.corrections("beyonce", 1, searchVocabMaxVisited)
	if !slices.Contains(corrections, "beyoncé") {
		t.Fatalf("expected Unicode correction, got %#v", corrections)
	}
}

func TestSearchVocabIndexVisitLimit(t *testing.T) {
	terms := make([]searchVocabTerm, 0, 2_000)
	for i := 0; i < 2_000; i++ {
		term := fmt.Sprintf("word%04d", i)
		terms = append(terms, testSearchVocabTerm(term, int64(i+1)))
	}
	index := newSearchVocabIndex(terms)

	const visitLimit = 17
	_, visited := index.corrections("wordzzzz", 2, visitLimit)
	if visited == 0 || visited > visitLimit {
		t.Fatalf("visited = %d, want 1..%d", visited, visitLimit)
	}
}

func TestSearchVocabIndexConcurrentLookups(t *testing.T) {
	index := newSearchVocabIndex([]searchVocabTerm{
		testSearchVocabTerm("licence", 8),
		testSearchVocabTerm("license", 3),
		testSearchVocabTerm("silence", 2),
	})

	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 100 {
				corrections, visited := index.corrections("lisence", 2, searchVocabMaxVisited)
				if !slices.Contains(corrections, "licence") {
					t.Errorf("expected licence correction, got %#v", corrections)
				}
				if visited > searchVocabMaxVisited {
					t.Errorf("visited = %d, limit = %d", visited, searchVocabMaxVisited)
				}
			}
		}()
	}
	wg.Wait()
}

func TestTrackVocabRefreshesAfterMusicianRename(t *testing.T) {
	app := setupTestApp(t)
	ctx := context.Background()

	musicianID := createSearchMusician(t, app, "Adele")
	albumID := createSearchAlbum(t, app, "Twenty Five", "Adele")
	createSearchTrack(t, app, "Hello", "/music/hello.flac", albumID, musicianID)

	corrections, err := testVocabCorrections(app, ctx, "tracks_search_fts_vocab", "adelle")
	if err != nil {
		t.Fatalf("initial track vocabCorrections failed: %v", err)
	}
	if !slices.Contains(corrections, "adele") {
		t.Fatalf("expected initial musician correction, got %#v", corrections)
	}

	_, err = app.DB.Exec("UPDATE musicians SET name = ?, sort_name = ? WHERE id = ?", "Sia", "sia", musicianID)
	if err != nil {
		t.Fatalf("rename musician: %v", err)
	}
	corrections, err = testVocabCorrections(app, ctx, "tracks_search_fts_vocab", "siia")
	if err != nil {
		t.Fatalf("renamed track vocabCorrections failed: %v", err)
	}
	if !slices.Contains(corrections, "sia") {
		t.Fatalf("expected renamed musician correction, got %#v", corrections)
	}
}

// A library change bumps the vocabulary generation, which replaces the cached
// correction index; a stale index would keep suggesting renamed or deleted
// titles.
func TestVocabCorrectionsRefreshAfterLibraryChanges(t *testing.T) {
	libraries := []struct {
		name, vocabTable          string
		seed                      func(t *testing.T, app *Application) int64
		renameSQL, deleteSQL      string
		initialQuery, initialTerm string
		renamedQuery, renamedTerm string
	}{
		{
			name: "movies", vocabTable: "movies_fts_vocab",
			seed: func(t *testing.T, app *Application) int64 {
				return createTestMovie(t, app, "Licence to Kill", "/movies/licence-to-kill.mkv")
			},
			renameSQL: "UPDATE movies SET title = 'Arrival' WHERE id = ?", deleteSQL: "DELETE FROM movies WHERE id = ?",
			initialQuery: "license", initialTerm: "licence", renamedQuery: "arival", renamedTerm: "arrival",
		},
		{
			name: "shows", vocabTable: "shows_fts_vocab",
			seed: func(t *testing.T, app *Application) int64 {
				return createSearchShow(t, app, "Severance", "/shows/Severance", "", "")
			},
			renameSQL: "UPDATE shows SET name = 'Andor' WHERE id = ?", deleteSQL: "DELETE FROM shows WHERE id = ?",
			initialQuery: "severence", initialTerm: "severance", renamedQuery: "andorr", renamedTerm: "andor",
		},
	}
	for _, lib := range libraries {
		t.Run(lib.name, func(t *testing.T) {
			app := setupTestApp(t)
			ctx := context.Background()
			id := lib.seed(t, app)

			generation := func(t *testing.T) int64 {
				t.Helper()
				var current int64
				err := app.DB.QueryRow(`SELECT generation FROM search_vocab_generations WHERE vocab_table = ?`, lib.vocabTable).Scan(&current)
				if err != nil {
					t.Fatalf("read %s generation: %v", lib.vocabTable, err)
				}
				return current
			}
			expectCorrection := func(t *testing.T, query, term string, present bool) {
				t.Helper()
				corrections, err := testVocabCorrections(app, ctx, lib.vocabTable, query)
				if err != nil {
					t.Fatalf("vocabCorrections(%q): %v", query, err)
				}
				if slices.Contains(corrections, term) != present {
					t.Fatalf("corrections for %q = %#v, want %q present = %v", query, corrections, term, present)
				}
			}

			expectCorrection(t, lib.initialQuery, lib.initialTerm, true)
			initialIndex, ok := app.SearchVocab.get(lib.vocabTable, generation(t))
			if !ok {
				t.Fatal("expected the initial vocabulary index to be cached")
			}

			_, err := app.DB.Exec(lib.renameSQL, id)
			if err != nil {
				t.Fatalf("rename: %v", err)
			}
			expectCorrection(t, lib.renamedQuery, lib.renamedTerm, true)
			updatedIndex, ok := app.SearchVocab.get(lib.vocabTable, generation(t))
			if !ok || updatedIndex == initialIndex {
				t.Fatal("expected the rename to replace the cached index")
			}

			_, err = app.DB.Exec(lib.deleteSQL, id)
			if err != nil {
				t.Fatalf("delete: %v", err)
			}
			expectCorrection(t, lib.renamedQuery, lib.renamedTerm, false)
		})
	}
}
