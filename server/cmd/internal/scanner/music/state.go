package music

import (
	"igloo/cmd/internal/scanner"
	spotifyapi "igloo/cmd/internal/spotify"
)

// Database caches use transaction overlays; lookup outcomes and splitting
// decisions live for the scan and do not depend on transaction success.
type musicScanContext struct {
	trackIndex        map[string]scanner.FileFingerprint
	merged            bool
	invalidatedTracks map[int64]bool
	artistAttempts    map[string]*resolvedMusician
	albumAttempts     map[string]*resolvedAlbum
	enrichmentCounts  map[string]int
	// enrichmentOutcome keys the entities already tallied into enrichmentCounts
	// by their status, so a name resolved for a hundred tracks contributes one
	// outcome, and an adoption can move it.
	enrichmentOutcome   map[string]string
	artistAttemptsByID  map[int64]*resolvedMusician
	albumAttemptsByID   map[int64]*resolvedAlbum
	musicianIDs         scanner.ScanCache[string, int64]
	albumIDs            scanner.ScanCache[string, int64]
	genreIDs            scanner.ScanCache[string, int64]
	spotifyArtistMisses map[string]resolvedSpotifyMatch
	spotifyAlbumMisses  map[string]resolvedSpotifyMatch
	compoundSplits      map[string]bool
	// albumTracks holds the Spotify track list of every album matched or
	// fetched this scan, keyed by Spotify ID, so its tracks pair without
	// another request.
	albumTracks        map[string][]spotifyapi.SimpleTrack
	albumTrackFailures map[string]bool
	artistIDAttempts   map[spotifyapi.ID]artistIDResult
	// tracksLinked counts the tracks paired with a Spotify track; a clone
	// counts its own transaction and mergeFrom adds it.
	tracksLinked int
	// breaker stops Spotify lookups for the rest of the scan once the provider
	// is failing; stopReported keeps the scan-wide issue to one.
	breaker      *scanner.ProviderBreaker
	stopReported bool
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
		enrichmentOutcome:   make(map[string]string),
		compoundSplits:      make(map[string]bool),
		musicianIDs:         scanner.NewScanCache[string, int64](),
		albumIDs:            scanner.NewScanCache[string, int64](),
		genreIDs:            scanner.NewScanCache[string, int64](),
		spotifyArtistMisses: make(map[string]resolvedSpotifyMatch),
		spotifyAlbumMisses:  make(map[string]resolvedSpotifyMatch),
		albumTracks:         make(map[string][]spotifyapi.SimpleTrack),
		albumTrackFailures:  make(map[string]bool),
		artistIDAttempts:    make(map[spotifyapi.ID]artistIDResult),
		breaker:             &scanner.ProviderBreaker{Classify: spotifyapi.ProviderFailure},
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
		enrichmentOutcome:   scan.enrichmentOutcome,
		trackIndex:          scan.trackIndex, // shared; never written inside the transaction
		musicianIDs:         scan.musicianIDs.Overlay(),
		albumIDs:            scan.albumIDs.Overlay(),
		genreIDs:            scan.genreIDs.Overlay(),
		spotifyArtistMisses: scan.spotifyArtistMisses,
		spotifyAlbumMisses:  scan.spotifyAlbumMisses,
		albumTracks:         scan.albumTracks,
		albumTrackFailures:  scan.albumTrackFailures,
		artistIDAttempts:    scan.artistIDAttempts,
		breaker:             scan.breaker,
	}
}

func (scan *musicScanContext) mergeFrom(other *musicScanContext) {
	scan.genreIDs.MergeFrom(other.genreIDs)
	scan.tracksLinked += other.tracksLinked
	if other.merged {
		scan.musicianIDs = scanner.NewScanCache[string, int64]()
		scan.albumIDs = scanner.NewScanCache[string, int64]()
		return
	}
	scan.musicianIDs.MergeFrom(other.musicianIDs)
	scan.albumIDs.MergeFrom(other.albumIDs)
}

// countEnrichment tallies one provider outcome per unique entity per scan.
// Every resolution path funnels through it, including the cached ones, because
// counting only where the network call happens reported requests rather than
// artists and albums.
func (scan *musicScanContext) countEnrichment(key string, match *resolvedSpotifyMatch) {
	if match == nil {
		return
	}
	_, counted := scan.enrichmentOutcome[key]
	if counted {
		return
	}
	scan.enrichmentOutcome[key] = match.status
	scan.enrichmentCounts[match.status]++
}

// recountEnrichment moves an entity's tally to status, for an artist adopted
// from its album after its own search had been tallied as a miss.
func (scan *musicScanContext) recountEnrichment(key, status string) {
	previous, counted := scan.enrichmentOutcome[key]
	if counted {
		if previous == status {
			return
		}
		scan.enrichmentCounts[previous]--
	}
	scan.enrichmentOutcome[key] = status
	scan.enrichmentCounts[status]++
}
