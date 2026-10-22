package music

import (
	"context"
	"database/sql"
	"errors"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/scanner"
	spotifyapi "igloo/cmd/internal/spotify"
)

const (
	musicSpotifyEntityAlbum     = "album"
	musicSpotifyEntityMusician  = "musician"
	musicSpotifyEntityTrack     = "track"
	musicSpotifyStatusMatched   = "matched"
	musicSpotifyStatusFailed    = "failed"
	musicSpotifyStatusUnmatched = "unmatched"
	// musicSpotifyReasonTracksDisagree records an album match the link pass
	// dropped because the album's tracks contradicted it.
	musicSpotifyReasonTracksDisagree = "tracks_disagree"
	// albumCheckMinTracks is the fewest local tracks an album needs before its
	// tracks may contradict its match; one or two files say too little.
	albumCheckMinTracks = 3
)

// resolvedSpotifyMatch is one lookup's outcome. A miss keeps the best
// candidate's name and, when one was scored, its score, so it can be
// explained later.
type resolvedSpotifyMatch struct {
	status    string
	reason    sql.NullString
	candidate sql.NullString
	score     sql.NullInt64
}

// saveMusicSpotifyMatch records an outcome; a nil match leaves the stored one
// alone.
func saveMusicSpotifyMatch(ctx context.Context, qtx *database.Queries, entityType string, entityID int64, match *resolvedSpotifyMatch) error {
	if match == nil {
		return nil
	}
	return qtx.UpsertMusicSpotifyMatch(ctx, database.UpsertMusicSpotifyMatchParams{
		EntityType: entityType,
		EntityID:   entityID,
		Status:     match.status,
		Reason:     match.reason,
		Candidate:  match.candidate,
		Score:      match.score,
	})
}

func resolvedSpotifyMatchFromError(err error) resolvedSpotifyMatch {
	matchErr, ok := errors.AsType[*spotifyapi.MatchError](err)
	if !ok {
		return resolvedSpotifyMatch{status: musicSpotifyStatusFailed}
	}
	return resolvedSpotifyMatchFromInfo(matchErr.Info)
}

// resolvedSpotifyMatchFromInfo reads a miss: a definitive reason is unmatched
// for good, any other is failed and retried.
func resolvedSpotifyMatchFromInfo(info spotifyapi.MatchDebugInfo) resolvedSpotifyMatch {
	match := resolvedSpotifyMatch{
		status:    musicSpotifyStatusFailed,
		reason:    helpers.NullString(info.Reason),
		candidate: helpers.NullString(info.CandidateName),
	}
	scored := info.Reason == spotifyapi.MatchReasonScoreBelowThreshold || info.Reason == spotifyapi.MatchReasonTrackClaimed
	if scored {
		match.score = sql.NullInt64{Int64: int64(info.Score), Valid: true}
	}
	unmatched := musicSpotifyReasonIsUnmatched(info.Reason)
	if unmatched {
		match.status = musicSpotifyStatusUnmatched
	}
	return match
}

func musicSpotifyMatchSplitsCompound(status string, reason sql.NullString) bool {
	if status != musicSpotifyStatusUnmatched || !reason.Valid {
		return false
	}

	return musicSpotifyReasonSplitsCompound(reason.String)
}

func musicSpotifyMatchStatusIsFinal(status string) bool {
	return status == musicSpotifyStatusMatched || status == musicSpotifyStatusUnmatched
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

// spotifyReady reports whether lookups may still go to Spotify this scan.
func (s *Scanner) spotifyReady(scan *musicScanContext) bool {
	return s.spotify != nil && !scan.breaker.Stopped
}

// noteSpotifyStop reports the breaker tripping once per scan and returns
// whether it has tripped.
func (s *Scanner) noteSpotifyStop(scan *musicScanContext, report *scanReport) bool {
	if !scan.breaker.Stopped {
		return false
	}
	if !scan.stopReported {
		scan.stopReported = true
		s.logger.Warn("spotify lookups stopped for this scan", "run", report.status.RunID)
		report.Issue("", scanner.PhaseEnrichment, reasonSpotifyStopped)
		s.publish(report)
	}
	return true
}

// albumTypeValue keeps Spotify's album type inside the albums.album_type
// CHECK; any other value is stored as unknown rather than failing the write.
func albumTypeValue(albumType string) sql.NullString {
	switch albumType {
	case "album", "single", "compilation":
		return sql.NullString{String: albumType, Valid: true}
	}
	return sql.NullString{}
}

func firstImageURL(images []spotifyapi.Image) string {
	if len(images) == 0 {
		return ""
	}

	return images[0].URL
}
