package scanner

import (
	"database/sql"
	"time"
)

const (
	// TMDB no-match backoff: after a second definitive miss, each further miss
	// doubles the wait before the entity is searched again, so an
	// unidentifiable file does not cost a lookup on every scan. Provider
	// failures are not misses.
	TmdbMissBackoff    = 24 * time.Hour
	TmdbMissBackoffMax = 7 * 24 * time.Hour
	// TmdbLookupTimeout bounds one entity's search plus details fetch.
	TmdbLookupTimeout = 30 * time.Second
	// MaxConsecutiveProviderFailures stops new enrichment dispatch for the
	// rest of the scan; pending entities retry on a later scan.
	MaxConsecutiveProviderFailures = 3
	// DiscoveryPublishInterval batches the Total updates published while
	// walking the library. Publishing per file made a large library rebuild
	// the whole status thousands of times before any work started.
	DiscoveryPublishInterval = 100
)

// MissBackoffElapsed grants the first miss a retry on the next scan; from the
// second miss on, the wait doubles from TmdbMissBackoff up to TmdbMissBackoffMax.
func MissBackoffElapsed(attempts int64, lastAttemptAt sql.NullInt64, now time.Time) bool {
	if attempts <= 1 || !lastAttemptAt.Valid {
		return true
	}
	shift := min(attempts-2, 8)
	backoff := min(TmdbMissBackoff<<shift, TmdbMissBackoffMax)
	eligibleAt := time.Unix(lastAttemptAt.Int64, 0).Add(backoff)
	return !now.Before(eligibleAt)
}
