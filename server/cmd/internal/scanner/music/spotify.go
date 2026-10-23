package music

import (
	"database/sql"
	"errors"
	"net/http"

	"igloo/cmd/internal/helpers"
	spotifyapi "igloo/cmd/internal/spotify"
)

const (
	// musicSpotifyReasonTracksDisagree records an album match the link pass
	// dropped because the album's tracks contradicted it.
	musicSpotifyReasonTracksDisagree = "tracks_disagree"
	// albumCheckMinTracks is the fewest local tracks an album needs before its
	// tracks may contradict its match; one or two files say too little.
	albumCheckMinTracks = 3
)

func resolvedSpotifyMatchFromError(err error) resolvedMatch {
	matchErr, ok := errors.AsType[*spotifyapi.MatchError](err)
	if !ok {
		return resolvedMatch{status: musicStatusFailed}
	}
	return resolvedSpotifyMatchFromInfo(matchErr.Info)
}

// resolvedSpotifyMatchFromInfo reads a miss: a definitive reason is unmatched
// for good, any other is failed and retried.
func resolvedSpotifyMatchFromInfo(info spotifyapi.MatchDebugInfo) resolvedMatch {
	match := resolvedMatch{
		status:    musicStatusFailed,
		reason:    helpers.NullString(info.Reason),
		candidate: helpers.NullString(info.CandidateName),
	}
	scored := info.Reason == spotifyapi.MatchReasonScoreBelowThreshold || info.Reason == spotifyapi.MatchReasonTrackClaimed
	if scored {
		match.score = sql.NullInt64{Int64: int64(info.Score), Valid: true}
	}
	unmatched := musicSpotifyReasonIsUnmatched(info.Reason)
	if unmatched {
		match.status = musicStatusUnmatched
	}
	return match
}

func musicSpotifyMatchSplitsCompound(status string, reason sql.NullString) bool {
	if status != musicStatusUnmatched || !reason.Valid {
		return false
	}

	return musicSpotifyReasonSplitsCompound(reason.String)
}

func musicSpotifyReasonIsUnmatched(reason string) bool {
	return reason == spotifyapi.MatchReasonNoResults ||
		reason == spotifyapi.MatchReasonScoreBelowThreshold ||
		reason == spotifyapi.MatchReasonEmptyQuery ||
		reason == spotifyapi.MatchReasonTrackClaimed
}

func musicSpotifyReasonSplitsCompound(reason string) bool {
	return reason == spotifyapi.MatchReasonNoResults || reason == spotifyapi.MatchReasonScoreBelowThreshold
}

// spotifyNotFound reports a 404: the requested catalog entry is gone, which
// settles the lookup rather than leaving it to retry.
func spotifyNotFound(err error) bool {
	status, ok := errors.AsType[*spotifyapi.StatusError](err)
	return ok && status.StatusCode == http.StatusNotFound
}

// spotifyReady reports whether lookups may still go to Spotify this scan.
func (s *Scanner) spotifyReady(scan *musicScanContext) bool {
	return s.spotify != nil && !scan.breaker.Stopped
}

// noteSpotifyStop reports the breaker tripping once per scan and returns
// whether it has tripped.
func (s *Scanner) noteSpotifyStop(scan *musicScanContext, report *scanReport) bool {
	return s.noteProviderStop(report, scan.breaker, &scan.stopReported, musicProviderSpotify, reasonSpotifyStopped)
}

func firstImageURL(images []spotifyapi.Image) string {
	if len(images) == 0 {
		return ""
	}

	return images[0].URL
}
