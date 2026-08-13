package main

import (
	"math/rand"
	"testing"
)

func benchVocabTerms(n int) []searchVocabTerm {
	rng := rand.New(rand.NewSource(42))
	terms := make([]searchVocabTerm, 0, n)
	for i := 0; i < n; i++ {
		runes := make([]rune, 4+rng.Intn(9))
		for j := range runes {
			runes[j] = rune('a' + rng.Intn(26))
		}
		terms = append(terms, searchVocabTerm{
			term:  string(runes),
			runes: runes,
			doc:   int64(rng.Intn(1000)),
		})
	}
	return terms
}

// Benchmarks the synchronous BK-tree rebuild that currently runs inside a
// search request whenever a library write bumps the vocab generation.
func BenchmarkSearchVocabBuild(b *testing.B) {
	terms := benchVocabTerms(10_000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// newSearchVocabIndex sorts its input, so hand it a fresh copy.
		copied := make([]searchVocabTerm, len(terms))
		copy(copied, terms)
		newSearchVocabIndex(copied)
	}
}

// Benchmarks a single typo-correction lookup against a built index.
func BenchmarkSearchVocabCorrections(b *testing.B) {
	index := newSearchVocabIndex(benchVocabTerms(10_000))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		index.corrections("exmaple", 2, searchVocabMaxVisited)
	}
}
