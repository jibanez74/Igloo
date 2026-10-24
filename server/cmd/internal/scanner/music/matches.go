package music

import (
	"context"
	"database/sql"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/scanner"
)

// Each entity keeps one provider outcome in music_provider_matches: the
// provider that settled it, and how.
const (
	musicEntityAlbum     = "album"
	musicEntityMusician  = "musician"
	musicEntityTrack     = "track"
	musicProviderSpotify = "spotify"
	musicProviderApple   = "apple"
	musicStatusMatched   = "matched"
	musicStatusFailed    = "failed"
	musicStatusUnmatched = "unmatched"
	// musicAppleReasonNotListed records an album, or a track of a matched
	// album, whose Apple id the catalog no longer lists. An album with this
	// outcome is handed to Spotify.
	musicAppleReasonNotListed = "not_listed"
)

// resolvedMatch is one lookup's outcome. A miss keeps the best candidate's
// name and, when one was scored, its score, so it can be explained later.
type resolvedMatch struct {
	status    string
	reason    sql.NullString
	candidate sql.NullString
	score     sql.NullInt64
}

// saveMusicMatch records the provider's outcome for an entity; a nil match
// leaves the stored one alone.
func saveMusicMatch(ctx context.Context, qtx *database.Queries, provider, entityType string, entityID int64, match *resolvedMatch) error {
	if match == nil {
		return nil
	}
	return qtx.UpsertMusicProviderMatch(ctx, database.UpsertMusicProviderMatchParams{
		EntityType: entityType,
		EntityID:   entityID,
		Provider:   provider,
		Status:     match.status,
		Reason:     match.reason,
		Candidate:  match.candidate,
		Score:      match.score,
	})
}

// musicMatchIsFinal reports a settled outcome that no provider looks up
// again. Apple's not_listed is not one: Spotify searches that album next.
func musicMatchIsFinal(match database.GetMusicProviderMatchRow) bool {
	handedOver := match.Provider == musicProviderApple && match.Reason.String == musicAppleReasonNotListed
	settled := match.Status == musicStatusMatched || match.Status == musicStatusUnmatched
	return settled && !handedOver
}

// albumTypeValue keeps a provider's album type inside the albums.album_type
// CHECK; any other value is stored as unknown rather than failing the write.
func albumTypeValue(albumType string) sql.NullString {
	switch albumType {
	case "album", "single", "compilation":
		return sql.NullString{String: albumType, Valid: true}
	}
	return sql.NullString{}
}

// noteProviderStop reports a provider's breaker tripping once per scan and
// returns whether it has tripped.
func (s *Scanner) noteProviderStop(report *scanReport, breaker *scanner.ProviderBreaker, reported *bool, provider, reason string) bool {
	if !breaker.Stopped {
		return false
	}
	if !*reported {
		*reported = true
		s.logger.Warn("provider lookups stopped for this scan", "run", report.status.RunID, "provider", provider)
		report.Issue("", scanner.PhaseEnrichment, reason)
		s.publish(report)
	}
	return true
}
