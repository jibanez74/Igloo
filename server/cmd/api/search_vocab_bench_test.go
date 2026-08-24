package main

import (
	"math/rand"
	"testing"
)

// benchVocabHitTerm is one edit away from the query the correction benchmark
// issues, so the lookup always finds a candidate.
const (
	benchVocabHitTerm  = "example"
	benchVocabHitQuery = "exmaple"
	benchVocabMaxDoc   = 1000
)

func benchVocabTerms(n int) []searchVocabTerm {
	rng := rand.New(rand.NewSource(42))
	terms := make([]searchVocabTerm, 0, n)

	// One slot is reserved for the planted hit appended below, so the caller
	// gets exactly n terms.
	for i := 0; i < n-1; i++ {
		runes := make([]rune, 4+rng.Intn(9))
		for j := range runes {
			runes[j] = rune('a' + rng.Intn(26))
		}
		terms = append(terms, searchVocabTerm{
			term:  string(runes),
			runes: runes,
			doc:   int64(rng.Intn(benchVocabMaxDoc)),
		})
	}

	// The rest of the vocabulary is random, so nothing is guaranteed to sit
	// within maxDist of the benchmark's query. Plant one deterministic hit so
	// the correction benchmark measures candidate ranking and result
	// allocation rather than only the miss path. Its doc count sits just above
	// the generated range because the search frontier is a max-heap on doc
	// frequency: a rare term is never reached within the visit limit, and
	// corrections exist to surface frequent terms anyway.
	hit := []rune(benchVocabHitTerm)
	terms = append(terms, searchVocabTerm{
		term:  benchVocabHitTerm,
		runes: hit,
		doc:   benchVocabMaxDoc,
	})

	return terms
}

// Benchmarks the synchronous BK-tree rebuild that currently runs inside a
// search request whenever a searchable-field write bumps the vocab generation.
func BenchmarkSearchVocabBuild(b *testing.B) {
	terms := benchVocabTerms(10_000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// newSearchVocabIndex sorts its input, so hand it a fresh copy — but
		// keep the copy out of the timer, which measures the build alone.
		b.StopTimer()
		copied := make([]searchVocabTerm, len(terms))
		copy(copied, terms)
		b.StartTimer()

		newSearchVocabIndex(copied)
	}
}

// Benchmarks a single typo-correction lookup against a built index.
func BenchmarkSearchVocabCorrections(b *testing.B) {
	index := newSearchVocabIndex(benchVocabTerms(10_000))

	matches, _ := index.corrections(benchVocabHitQuery, 2, searchVocabMaxVisited)
	if len(matches) == 0 {
		b.Fatalf("expected %q to correct to a candidate; the benchmark would only measure the miss path", benchVocabHitQuery)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		index.corrections(benchVocabHitQuery, 2, searchVocabMaxVisited)
	}
}
