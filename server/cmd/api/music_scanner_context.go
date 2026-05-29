package main

import (
	"path/filepath"
	"strconv"
	"strings"
)

type musicScanContext struct {
	trackIndex                   map[string]int64
	musicianIDs                  map[string]int64
	albumIDs                     map[string]int64
	genreIDs                     map[string]int64
	musicianAlbums               map[string]struct{}
	musicianGenres               map[string]struct{}
	albumGenres                  map[string]struct{}
	trackGenres                  map[string]struct{}
	spotifyArtistMisses          map[string]resolvedSpotifyMatch
	spotifyAlbumMisses           map[string]resolvedSpotifyMatch
	spotifyMusicianGenresHandled map[int64]struct{}
	spotifyAlbumGenresHandled    map[int64]struct{}
}

func newMusicScanContext(trackIndex map[string]int64) *musicScanContext {
	if trackIndex == nil {
		trackIndex = make(map[string]int64)
	}

	return &musicScanContext{
		trackIndex:                   trackIndex,
		musicianIDs:                  make(map[string]int64),
		albumIDs:                     make(map[string]int64),
		genreIDs:                     make(map[string]int64),
		musicianAlbums:               make(map[string]struct{}),
		musicianGenres:               make(map[string]struct{}),
		albumGenres:                  make(map[string]struct{}),
		trackGenres:                  make(map[string]struct{}),
		spotifyArtistMisses:          make(map[string]resolvedSpotifyMatch),
		spotifyAlbumMisses:           make(map[string]resolvedSpotifyMatch),
		spotifyMusicianGenresHandled: make(map[int64]struct{}),
		spotifyAlbumGenresHandled:    make(map[int64]struct{}),
	}
}

func (scan *musicScanContext) clone() *musicScanContext {
	return &musicScanContext{
		trackIndex:                   copyStringInt64Map(scan.trackIndex),
		musicianIDs:                  copyStringInt64Map(scan.musicianIDs),
		albumIDs:                     copyStringInt64Map(scan.albumIDs),
		genreIDs:                     copyStringInt64Map(scan.genreIDs),
		musicianAlbums:               copyStringSet(scan.musicianAlbums),
		musicianGenres:               copyStringSet(scan.musicianGenres),
		albumGenres:                  copyStringSet(scan.albumGenres),
		trackGenres:                  copyStringSet(scan.trackGenres),
		spotifyArtistMisses:          copySpotifyMatchMap(scan.spotifyArtistMisses),
		spotifyAlbumMisses:           copySpotifyMatchMap(scan.spotifyAlbumMisses),
		spotifyMusicianGenresHandled: copyInt64Set(scan.spotifyMusicianGenresHandled),
		spotifyAlbumGenresHandled:    copyInt64Set(scan.spotifyAlbumGenresHandled),
	}
}

func (scan *musicScanContext) mergeFrom(other *musicScanContext) {
	mergeStringInt64Map(scan.trackIndex, other.trackIndex)
	mergeStringInt64Map(scan.musicianIDs, other.musicianIDs)
	mergeStringInt64Map(scan.albumIDs, other.albumIDs)
	mergeStringInt64Map(scan.genreIDs, other.genreIDs)
	mergeStringSet(scan.musicianAlbums, other.musicianAlbums)
	mergeStringSet(scan.musicianGenres, other.musicianGenres)
	mergeStringSet(scan.albumGenres, other.albumGenres)
	mergeStringSet(scan.trackGenres, other.trackGenres)
	mergeSpotifyMatchMap(scan.spotifyArtistMisses, other.spotifyArtistMisses)
	mergeSpotifyMatchMap(scan.spotifyAlbumMisses, other.spotifyAlbumMisses)
	mergeInt64Set(scan.spotifyMusicianGenresHandled, other.spotifyMusicianGenresHandled)
	mergeInt64Set(scan.spotifyAlbumGenresHandled, other.spotifyAlbumGenresHandled)
}

func (scan *musicScanContext) trackUnchanged(path string, size int64) bool {
	existingSize, ok := scan.trackIndex[filepath.Clean(path)]
	return ok && existingSize == size
}

func normalizedMusicCacheKey(parts ...string) string {
	normalized := make([]string, 0, len(parts))
	for _, part := range parts {
		normalized = append(normalized, strings.ToLower(strings.TrimSpace(part)))
	}

	return strings.Join(normalized, "\x00")
}

func musicIDPairKey(left, right int64) string {
	return strings.Join([]string{strconv.FormatInt(left, 10), strconv.FormatInt(right, 10)}, "\x00")
}

func copyStringInt64Map(input map[string]int64) map[string]int64 {
	output := make(map[string]int64, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func mergeStringInt64Map(target, source map[string]int64) {
	for key, value := range source {
		target[key] = value
	}
}

func copyStringSet(input map[string]struct{}) map[string]struct{} {
	output := make(map[string]struct{}, len(input))
	for key := range input {
		output[key] = struct{}{}
	}
	return output
}

func mergeStringSet(target, source map[string]struct{}) {
	for key := range source {
		target[key] = struct{}{}
	}
}

func copySpotifyMatchMap(input map[string]resolvedSpotifyMatch) map[string]resolvedSpotifyMatch {
	output := make(map[string]resolvedSpotifyMatch, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func mergeSpotifyMatchMap(target, source map[string]resolvedSpotifyMatch) {
	for key, value := range source {
		target[key] = value
	}
}

func copyInt64Set(input map[int64]struct{}) map[int64]struct{} {
	output := make(map[int64]struct{}, len(input))
	for key := range input {
		output[key] = struct{}{}
	}
	return output
}

func mergeInt64Set(target, source map[int64]struct{}) {
	for key := range source {
		target[key] = struct{}{}
	}
}
