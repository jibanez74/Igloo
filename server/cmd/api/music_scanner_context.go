package main

import (
	"maps"
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
		trackIndex:                   copyCleanPathInt64Map(trackIndex),
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
		trackIndex:                   copyCleanPathInt64Map(scan.trackIndex),
		musicianIDs:                  maps.Clone(scan.musicianIDs),
		albumIDs:                     maps.Clone(scan.albumIDs),
		genreIDs:                     maps.Clone(scan.genreIDs),
		musicianAlbums:               maps.Clone(scan.musicianAlbums),
		musicianGenres:               maps.Clone(scan.musicianGenres),
		albumGenres:                  maps.Clone(scan.albumGenres),
		trackGenres:                  maps.Clone(scan.trackGenres),
		spotifyArtistMisses:          maps.Clone(scan.spotifyArtistMisses),
		spotifyAlbumMisses:           maps.Clone(scan.spotifyAlbumMisses),
		spotifyMusicianGenresHandled: maps.Clone(scan.spotifyMusicianGenresHandled),
		spotifyAlbumGenresHandled:    maps.Clone(scan.spotifyAlbumGenresHandled),
	}
}

func (scan *musicScanContext) mergeFrom(other *musicScanContext) {
	mergeCleanPathInt64Map(scan.trackIndex, other.trackIndex)
	maps.Copy(scan.musicianIDs, other.musicianIDs)
	maps.Copy(scan.albumIDs, other.albumIDs)
	maps.Copy(scan.genreIDs, other.genreIDs)
	maps.Copy(scan.musicianAlbums, other.musicianAlbums)
	maps.Copy(scan.musicianGenres, other.musicianGenres)
	maps.Copy(scan.albumGenres, other.albumGenres)
	maps.Copy(scan.trackGenres, other.trackGenres)
	maps.Copy(scan.spotifyArtistMisses, other.spotifyArtistMisses)
	maps.Copy(scan.spotifyAlbumMisses, other.spotifyAlbumMisses)
	maps.Copy(scan.spotifyMusicianGenresHandled, other.spotifyMusicianGenresHandled)
	maps.Copy(scan.spotifyAlbumGenresHandled, other.spotifyAlbumGenresHandled)
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

func copyCleanPathInt64Map(input map[string]int64) map[string]int64 {
	output := make(map[string]int64, len(input))
	for key, value := range input {
		output[filepath.Clean(key)] = value
	}
	return output
}

func mergeCleanPathInt64Map(target, source map[string]int64) {
	for key, value := range source {
		target[filepath.Clean(key)] = value
	}
}
