package main

import (
	"maps"
	"path/filepath"
	"strings"
)

type movieScanContext struct {
	movieIndex           map[string]int64
	artistIDs            map[int]int64
	productionCompanyIDs map[int]int64
	genreIDs             map[string]int64
	extraVideoIDs        map[string]int64
}

func newMovieScanContext(movieIndex map[string]int64) *movieScanContext {
	if movieIndex == nil {
		movieIndex = make(map[string]int64)
	}

	return &movieScanContext{
		movieIndex:           copyCleanPathInt64Map(movieIndex),
		artistIDs:            make(map[int]int64),
		productionCompanyIDs: make(map[int]int64),
		genreIDs:             make(map[string]int64),
		extraVideoIDs:        make(map[string]int64),
	}
}

func (scan *movieScanContext) clone() *movieScanContext {
	return &movieScanContext{
		movieIndex:           copyCleanPathInt64Map(scan.movieIndex),
		artistIDs:            maps.Clone(scan.artistIDs),
		productionCompanyIDs: maps.Clone(scan.productionCompanyIDs),
		genreIDs:             maps.Clone(scan.genreIDs),
		extraVideoIDs:        maps.Clone(scan.extraVideoIDs),
	}
}

func (scan *movieScanContext) mergeFrom(other *movieScanContext) {
	mergeCleanPathInt64Map(scan.movieIndex, other.movieIndex)
	maps.Copy(scan.artistIDs, other.artistIDs)
	maps.Copy(scan.productionCompanyIDs, other.productionCompanyIDs)
	maps.Copy(scan.genreIDs, other.genreIDs)
	maps.Copy(scan.extraVideoIDs, other.extraVideoIDs)
}

func (scan *movieScanContext) movieUnchanged(path string, size int64) bool {
	existingSize, ok := scan.movieIndex[filepath.Clean(path)]
	return ok && existingSize == size
}

func normalizedMovieCacheKey(parts ...string) string {
	normalized := make([]string, 0, len(parts))
	for _, part := range parts {
		normalized = append(normalized, strings.ToLower(strings.TrimSpace(part)))
	}

	return strings.Join(normalized, "\x00")
}
