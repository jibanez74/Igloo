package music

import (
	"context"
	"database/sql"
	"errors"
	"time"

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
// leaves the stored one alone. A failed outcome counts the scans in a row whose
// lookup failed, and from the second holds the entity back from the retry pass
// on the scanner's miss backoff, so a lookup that keeps failing is not paid for
// on every scan. One scan counts once: the same failure is saved again whenever
// a later file, merge or adoption resolves the entity from the scan's cache.
// The count lives in the row, so a rolled-back transaction takes it back too.
func saveMusicMatch(ctx context.Context, qtx *database.Queries, scan *musicScanContext, provider, entityType string, entityID int64, match *resolvedMatch) error {
	if match == nil {
		return nil
	}

	var attempts int64
	var failedAt, retryAfter sql.NullInt64
	if match.status == musicStatusFailed {
		previous, err := qtx.GetMusicProviderMatchFailure(ctx, database.GetMusicProviderMatchFailureParams{
			EntityType: entityType,
			EntityID:   entityID,
		})
		hasPrevious := err == nil
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		scanID := scan.startedAt.UnixMilli()
		countedThisScan := hasPrevious && previous.FailedAt.Valid && previous.FailedAt.Int64 == scanID
		if countedThisScan {
			attempts, failedAt, retryAfter = previous.Attempts, previous.FailedAt, previous.RetryAfter
		} else {
			if hasPrevious {
				attempts = previous.Attempts
			}
			attempts++
			failedAt = sql.NullInt64{Int64: scanID, Valid: true}
			retryAfter = scanner.MissRetryAfter(attempts, time.Now())
		}
	}

	return qtx.UpsertMusicProviderMatch(ctx, database.UpsertMusicProviderMatchParams{
		EntityType: entityType,
		EntityID:   entityID,
		Provider:   provider,
		Status:     match.status,
		Reason:     match.reason,
		Candidate:  match.candidate,
		Score:      match.score,
		Attempts:   attempts,
		FailedAt:   failedAt,
		RetryAfter: retryAfter,
	})
}

// musicianFailureWaiting reports a musician whose failed Spotify lookup is
// still inside its backoff.
func (s *Scanner) musicianFailureWaiting(ctx context.Context, musicianID int64) (bool, error) {
	failure, err := s.queries.GetMusicProviderMatchFailure(ctx, database.GetMusicProviderMatchFailureParams{
		EntityType: musicEntityMusician,
		EntityID:   musicianID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return failure.RetryAfter.Valid && time.Now().Before(time.Unix(failure.RetryAfter.Int64, 0)), nil
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
