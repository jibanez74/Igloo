// Package preroll builds the ordered trailer queue played before a movie:
// per-user preferences, the two trailer pools, and the selection rules the
// web client and the TV client share through GET /api/movies/{id}/preroll.
package preroll

import (
	"errors"
	"math/rand/v2"
	"slices"
)

const (
	SourceLibrary  = "library"
	SourceTheaters = "theaters"
	SourceBoth     = "both"

	MinCount     = 1
	MaxCount     = 5
	DefaultCount = 2
)

// Sources lists the accepted preference values; the OpenAPI enum mirrors it.
var Sources = []string{SourceLibrary, SourceTheaters, SourceBoth}

var (
	ErrInvalidCount  = errors.New("trailer count must be between 1 and 5")
	ErrInvalidSource = errors.New("trailer source must be library, theaters or both")
)

// Preferences are the only pre-roll values stored on an account.
type Preferences struct {
	Enabled bool
	Count   int
	Source  string
}

func DefaultPreferences() Preferences {
	return Preferences{Enabled: false, Count: DefaultCount, Source: SourceBoth}
}

func ValidatePreferences(prefs Preferences) error {
	countInRange := prefs.Count >= MinCount && prefs.Count <= MaxCount
	if !countInRange {
		return ErrInvalidCount
	}
	if !IsSource(prefs.Source) {
		return ErrInvalidSource
	}
	return nil
}

func IsSource(value string) bool {
	for _, source := range Sources {
		if source == value {
			return true
		}
	}
	return false
}

// Trailer is one queue entry. Source is SourceLibrary or SourceTheaters;
// MovieID is set for library entries and TmdbID whenever it is known.
type Trailer struct {
	Title      string
	YouTubeKey string
	Source     string
	MovieID    int64
	TmdbID     int64
}

// Select builds the queue: theaters trailers first, then library trailers,
// like a real cinema. The library pool is consumed in the order given (the
// query already randomizes it); the theaters pool is shuffled with rng, so
// tests inject a seeded generator. Entries are deduplicated by TMDB id, an
// entry from either pool matching excludeTmdbID (the movie about to play, which
// another library row may share as a second file or edition) is dropped, and a
// share one pool cannot fill is topped up from the other.
func Select(prefs Preferences, library, theaters []Trailer, excludeTmdbID int64, rng *rand.Rand) []Trailer {
	queue := make([]Trailer, 0, prefs.Count)
	if !prefs.Enabled || prefs.Count <= 0 {
		return queue
	}

	theatersPool := slices.Clone(theaters)
	rng.Shuffle(len(theatersPool), func(i, j int) {
		theatersPool[i], theatersPool[j] = theatersPool[j], theatersPool[i]
	})

	theatersShare, libraryShare := shares(prefs)
	seen := make(map[int64]struct{}, prefs.Count+1)
	if excludeTmdbID != 0 {
		seen[excludeTmdbID] = struct{}{}
	}
	theatersPool, chosenTheaters := take(theatersPool, theatersShare, seen)
	libraryPool, chosenLibrary := take(library, libraryShare, seen)

	missing := prefs.Count - len(chosenTheaters) - len(chosenLibrary)
	if missing > 0 {
		var extra []Trailer
		theatersPool, extra = take(theatersPool, missing, seen)
		chosenTheaters = append(chosenTheaters, extra...)
		missing -= len(extra)
	}
	if missing > 0 {
		var extra []Trailer
		_, extra = take(libraryPool, missing, seen)
		chosenLibrary = append(chosenLibrary, extra...)
	}

	queue = append(queue, chosenTheaters...)
	queue = append(queue, chosenLibrary...)
	return queue
}

func shares(prefs Preferences) (theaters, library int) {
	switch prefs.Source {
	case SourceLibrary:
		return 0, prefs.Count
	case SourceTheaters:
		return prefs.Count, 0
	default:
		theaters = (prefs.Count + 1) / 2
		return theaters, prefs.Count - theaters
	}
}

// take removes up to want entries from the head of pool, skipping TMDB ids
// already chosen, and returns the remaining pool and the chosen entries.
func take(pool []Trailer, want int, seen map[int64]struct{}) (remaining, chosen []Trailer) {
	chosen = make([]Trailer, 0, want)
	index := 0
	for index < len(pool) && len(chosen) < want {
		trailer := pool[index]
		index++
		if trailer.TmdbID != 0 {
			_, duplicate := seen[trailer.TmdbID]
			if duplicate {
				continue
			}
			seen[trailer.TmdbID] = struct{}{}
		}
		chosen = append(chosen, trailer)
	}
	return pool[index:], chosen
}
