package music

import (
	"context"
	"database/sql"

	"igloo/cmd/internal/database"
)

// Each entity keeps one provider outcome in music_provider_matches: the
// provider that settled it, and how.
const (
	musicEntityAlbum     = "album"
	musicEntityMusician  = "musician"
	musicEntityTrack     = "track"
	musicProviderSpotify = "spotify"
	musicStatusMatched   = "matched"
	musicStatusFailed    = "failed"
	musicStatusUnmatched = "unmatched"
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

func musicMatchIsFinal(status string) bool {
	return status == musicStatusMatched || status == musicStatusUnmatched
}
