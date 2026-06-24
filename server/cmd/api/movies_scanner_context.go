package main

import (
	"igloo/cmd/internal/helpers"
	"maps"
)

type movieScanContext struct {
	// movieIndex maps cleaned file path -> file size for every movie already in
	// the DB. It is read to skip unchanged files and is only written after a
	// successful commit, never inside a transaction, so it is shared (not copied)
	// across per-movie transactions.
	movieIndex map[string]int64
	// genreIDs memoizes genre tag -> id within a scan. It is written inside the
	// per-movie transaction (getOrCreateMovieGenreID), so clone/mergeFrom isolate
	// it until commit to avoid caching ids from a rolled-back transaction.
	genreIDs map[string]int64
}

func newMovieScanContext(movieIndex map[string]int64) *movieScanContext {
	if movieIndex == nil {
		movieIndex = make(map[string]int64)
	}

	// Take ownership of movieIndex: loadMovieScanIndex already cleaned its keys
	// and the caller discards its reference, so no defensive copy is needed.
	return &movieScanContext{
		movieIndex: movieIndex,
		genreIDs:   make(map[string]int64),
	}
}

func (scan *movieScanContext) clone() *movieScanContext {
	return &movieScanContext{
		movieIndex: scan.movieIndex, // shared; never written inside the transaction
		genreIDs:   maps.Clone(scan.genreIDs),
	}
}

func (scan *movieScanContext) mergeFrom(other *movieScanContext) {
	maps.Copy(scan.genreIDs, other.genreIDs)
}

func (scan *movieScanContext) movieUnchanged(path string, size int64) bool {
	return helpers.ScanIndexUnchanged(scan.movieIndex, path, size)
}
