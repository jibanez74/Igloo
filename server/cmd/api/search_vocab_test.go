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

func TestVocabCorrectionsRefreshesAfterMovieChanges(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	ctx := context.Background()

	movieID := createSearchMovie(t, app, "Licence to Kill", "/movies/licence-to-kill.mkv")
	corrections, err := testVocabCorrections(app, ctx, "movies_fts_vocab", "license")
	if err != nil {
		t.Fatalf("initial vocabCorrections failed: %v", err)
	}
	if !slices.Contains(corrections, "licence") {
		t.Fatalf("expected initial correction, got %#v", corrections)
	}

	var initialGeneration int64
	err = app.DB.QueryRow(`
		SELECT generation FROM search_vocab_generations
		WHERE vocab_table = 'movies_fts_vocab'
	`).Scan(&initialGeneration)
	if err != nil {
		t.Fatalf("read initial generation: %v", err)
	}
	initialIndex, ok := app.SearchVocab.get("movies_fts_vocab", initialGeneration)
	if !ok {
		t.Fatal("expected initial movie vocabulary index to be cached")
	}

	_, err = app.DB.Exec("UPDATE movies SET title = ? WHERE id = ?", "Arrival", movieID)
	if err != nil {
		t.Fatalf("update movie title: %v", err)
	}
	corrections, err = testVocabCorrections(app, ctx, "movies_fts_vocab", "arival")
	if err != nil {
		t.Fatalf("updated vocabCorrections failed: %v", err)
	}
	if !slices.Contains(corrections, "arrival") {
		t.Fatalf("expected updated correction, got %#v", corrections)
	}

	var updatedGeneration int64
	err = app.DB.QueryRow(`
		SELECT generation FROM search_vocab_generations
		WHERE vocab_table = 'movies_fts_vocab'
	`).Scan(&updatedGeneration)
	if err != nil {
		t.Fatalf("read updated generation: %v", err)
	}
	updatedIndex, ok := app.SearchVocab.get("movies_fts_vocab", updatedGeneration)
	if !ok || updatedIndex == initialIndex {
		t.Fatal("expected movie vocabulary update to replace the cached index")
	}

	_, err = app.DB.Exec("DELETE FROM movies WHERE id = ?", movieID)
	if err != nil {
		t.Fatalf("delete movie: %v", err)
	}
	corrections, err = testVocabCorrections(app, ctx, "movies_fts_vocab", "arival")
	if err != nil {
		t.Fatalf("deleted vocabCorrections failed: %v", err)
	}
	if slices.Contains(corrections, "arrival") {
		t.Fatalf("deleted term remained cached: %#v", corrections)
	}
}

func TestTrackVocabRefreshesAfterMusicianRename(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
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
