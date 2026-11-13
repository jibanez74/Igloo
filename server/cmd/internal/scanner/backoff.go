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
	eligibleAt := time.Unix(lastAttemptAt.Int64, 0).Add(missBackoff(attempts))
	return !now.Before(eligibleAt)
}

// MissRetryAfter is the unix time before which an entity whose attempts-th
// lookup in a row just failed at lastAttempt is not looked up again, on the
// schedule MissBackoffElapsed applies; invalid (no wait) after the first.
func MissRetryAfter(attempts int64, lastAttempt time.Time) sql.NullInt64 {
	if attempts <= 1 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: lastAttempt.Add(missBackoff(attempts)).Unix(), Valid: true}
}

func missBackoff(attempts int64) time.Duration {
	shift := min(attempts-2, 8)
	return min(TmdbMissBackoff<<shift, TmdbMissBackoffMax)
}

// ProviderBreaker stops enrichment dispatch after a fatal provider failure
// (rejected credentials, an exhausted quota) or a run of transient ones;
// pending entities retry on a later scan. Classify is the provider's own
// failure classification, so only errors returned by that provider's client
// should be observed.
type ProviderBreaker struct {
	Classify    func(err error) (fatal, transient bool)
	consecutive int
	Stopped     bool
}

func (b *ProviderBreaker) Observe(err error) {
	fatal, transient := b.Classify(err)
	if transient {
		b.consecutive++
	} else {
		b.consecutive = 0
	}
	if fatal || b.consecutive >= MaxConsecutiveProviderFailures {
		b.Stopped = true
	}
}
