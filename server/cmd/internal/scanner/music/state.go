package music

import (
	"igloo/cmd/internal/scanner"
)

// Database caches use transaction overlays; lookup outcomes and splitting
// decisions live for the scan and do not depend on transaction success.
type musicScanContext struct {
	deferred            int
	trackIndex          map[string]scanner.FileFingerprint
	merged              bool
	invalidatedTracks   map[int64]bool
	artistAttempts      map[string]*resolvedMusician
	albumAttempts       map[string]*resolvedAlbum
	enrichmentCounts    map[string]int
	artistAttemptsByID  map[int64]*resolvedMusician
	albumAttemptsByID   map[int64]*resolvedAlbum
	musicianIDs         scanner.ScanCache[string, int64]
	albumIDs            scanner.ScanCache[string, int64]
	genreIDs            scanner.ScanCache[string, int64]
	spotifyArtistMisses map[string]resolvedSpotifyMatch
	spotifyAlbumMisses  map[string]resolvedSpotifyMatch
	compoundSplits      map[string]bool
}

func newMusicScanContext(trackIndex map[string]scanner.FileFingerprint) *musicScanContext {
	if trackIndex == nil {
		trackIndex = make(map[string]scanner.FileFingerprint)
	}

	// Take ownership of trackIndex: loadMusicScanIndex already cleaned its keys
	// and the caller discards its reference, so no defensive copy is needed.
	return &musicScanContext{
		trackIndex:          trackIndex,
		artistAttemptsByID:  make(map[int64]*resolvedMusician),
		albumAttemptsByID:   make(map[int64]*resolvedAlbum),
		invalidatedTracks:   make(map[int64]bool),
		artistAttempts:      make(map[string]*resolvedMusician),
		albumAttempts:       make(map[string]*resolvedAlbum),
		enrichmentCounts:    make(map[string]int),
		compoundSplits:      make(map[string]bool),
		musicianIDs:         scanner.NewScanCache[string, int64](),
		albumIDs:            scanner.NewScanCache[string, int64](),
		genreIDs:            scanner.NewScanCache[string, int64](),
		spotifyArtistMisses: make(map[string]resolvedSpotifyMatch),
		spotifyAlbumMisses:  make(map[string]resolvedSpotifyMatch),
	}
}

func (scan *musicScanContext) clone() *musicScanContext {
	return &musicScanContext{
		compoundSplits:      scan.compoundSplits,
		artistAttemptsByID:  scan.artistAttemptsByID,
		albumAttemptsByID:   scan.albumAttemptsByID,
		invalidatedTracks:   make(map[int64]bool),
		artistAttempts:      scan.artistAttempts,
		albumAttempts:       scan.albumAttempts,
		enrichmentCounts:    scan.enrichmentCounts,
		trackIndex:          scan.trackIndex, // shared; never written inside the transaction
		musicianIDs:         scan.musicianIDs.Overlay(),
		albumIDs:            scan.albumIDs.Overlay(),
		genreIDs:            scan.genreIDs.Overlay(),
		spotifyArtistMisses: scan.spotifyArtistMisses,
		spotifyAlbumMisses:  scan.spotifyAlbumMisses,
	}
}

func (scan *musicScanContext) mergeFrom(other *musicScanContext) {
	scan.genreIDs.MergeFrom(other.genreIDs)
	if other.merged {
		scan.musicianIDs = scanner.NewScanCache[string, int64]()
		scan.albumIDs = scanner.NewScanCache[string, int64]()
		return
	}
	scan.musicianIDs.MergeFrom(other.musicianIDs)
	scan.albumIDs.MergeFrom(other.albumIDs)
}
