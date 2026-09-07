package music

import (
	"igloo/cmd/internal/scanner"
)

// Database caches use transaction overlays; lookup outcomes and splitting
// decisions live for the scan and do not depend on transaction success.
type musicScanContext struct {
	trackIndex                   map[string]int64
	musicianIDs                  scanner.ScanCache[string, int64]
	albumIDs                     scanner.ScanCache[string, int64]
	genreIDs                     scanner.ScanCache[string, int64]
	musicianAlbums               scanner.ScanCache[musicIDPair, struct{}]
	musicianGenres               scanner.ScanCache[musicIDPair, struct{}]
	albumGenres                  scanner.ScanCache[musicIDPair, struct{}]
	spotifyArtistMisses          map[string]resolvedSpotifyMatch
	spotifyAlbumMisses           map[string]resolvedSpotifyMatch
	compoundSplits               map[string]bool
	spotifyMusicianGenresHandled scanner.ScanCache[int64, struct{}]
	spotifyAlbumGenresHandled    scanner.ScanCache[int64, struct{}]
}

type musicIDPair struct{ left, right int64 }

func newMusicScanContext(trackIndex map[string]int64) *musicScanContext {
	if trackIndex == nil {
		trackIndex = make(map[string]int64)
	}

	// Take ownership of trackIndex: loadMusicScanIndex already cleaned its keys
	// and the caller discards its reference, so no defensive copy is needed.
	return &musicScanContext{
		trackIndex:                   trackIndex,
		compoundSplits:               make(map[string]bool),
		musicianIDs:                  scanner.NewScanCache[string, int64](),
		albumIDs:                     scanner.NewScanCache[string, int64](),
		genreIDs:                     scanner.NewScanCache[string, int64](),
		musicianAlbums:               scanner.NewScanCache[musicIDPair, struct{}](),
		musicianGenres:               scanner.NewScanCache[musicIDPair, struct{}](),
		albumGenres:                  scanner.NewScanCache[musicIDPair, struct{}](),
		spotifyArtistMisses:          make(map[string]resolvedSpotifyMatch),
		spotifyAlbumMisses:           make(map[string]resolvedSpotifyMatch),
		spotifyMusicianGenresHandled: scanner.NewScanCache[int64, struct{}](),
		spotifyAlbumGenresHandled:    scanner.NewScanCache[int64, struct{}](),
	}
}

func (scan *musicScanContext) clone() *musicScanContext {
	return &musicScanContext{
		compoundSplits:               scan.compoundSplits,
		trackIndex:                   scan.trackIndex, // shared; never written inside the transaction
		musicianIDs:                  scan.musicianIDs.Overlay(),
		albumIDs:                     scan.albumIDs.Overlay(),
		genreIDs:                     scan.genreIDs.Overlay(),
		musicianAlbums:               scan.musicianAlbums.Overlay(),
		musicianGenres:               scan.musicianGenres.Overlay(),
		albumGenres:                  scan.albumGenres.Overlay(),
		spotifyArtistMisses:          scan.spotifyArtistMisses,
		spotifyAlbumMisses:           scan.spotifyAlbumMisses,
		spotifyMusicianGenresHandled: scan.spotifyMusicianGenresHandled.Overlay(),
		spotifyAlbumGenresHandled:    scan.spotifyAlbumGenresHandled.Overlay(),
	}
}

func (scan *musicScanContext) mergeFrom(other *musicScanContext) {
	scan.musicianIDs.MergeFrom(other.musicianIDs)
	scan.albumIDs.MergeFrom(other.albumIDs)
	scan.genreIDs.MergeFrom(other.genreIDs)
	scan.musicianAlbums.MergeFrom(other.musicianAlbums)
	scan.musicianGenres.MergeFrom(other.musicianGenres)
	scan.albumGenres.MergeFrom(other.albumGenres)
	scan.spotifyMusicianGenresHandled.MergeFrom(other.spotifyMusicianGenresHandled)
	scan.spotifyAlbumGenresHandled.MergeFrom(other.spotifyAlbumGenresHandled)
}

func (scan *musicScanContext) trackUnchanged(path string, size int64) bool {
	return scanner.ScanIndexUnchanged(scan.trackIndex, path, size)
}
