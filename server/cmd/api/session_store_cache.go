package main

import (
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/patrickmn/go-cache"
)

const (
	// sessionStoreCacheTTL bounds how long a session read may be served from
	// memory. Igloo is a single process and every mutation goes through this
	// decorator, so the only staleness this window can expose is a session
	// reaching its absolute lifetime; logout and device revocation evict
	// immediately.
	sessionStoreCacheTTL   = 30 * time.Second
	sessionStoreCacheSweep = time.Minute
)

// cachedSessionStore keeps session reads off SQLite. Every authenticated
// request loads the session, including each byte-range request for a movie and
// each HLS segment, and the database runs on a single shared connection
// (InitDB), so an uncached read puts media delivery behind the scanner and
// every other query.
type cachedSessionStore struct {
	store scs.Store
	cache *cache.Cache
}

func newCachedSessionStore(store scs.Store) *cachedSessionStore {
	return &cachedSessionStore{
		store: store,
		cache: cache.New(sessionStoreCacheTTL, sessionStoreCacheSweep),
	}
}

func (s *cachedSessionStore) Find(token string) ([]byte, bool, error) {
	cached, hit := s.cache.Get(token)
	if hit {
		data, ok := cached.([]byte)
		if ok {
			return data, true, nil
		}
	}

	data, found, err := s.store.Find(token)
	if err != nil {
		return nil, false, err
	}
	if !found {
		return nil, false, nil
	}

	s.cache.SetDefault(token, data)

	return data, true, nil
}

func (s *cachedSessionStore) Commit(token string, b []byte, expiry time.Time) error {
	err := s.store.Commit(token, b, expiry)
	if err != nil {
		return err
	}

	// scs treats the store as the sole authority on expiry, so a cached entry
	// must never outlive the session it belongs to.
	ttl := time.Until(expiry)
	if ttl <= 0 {
		s.cache.Delete(token)
		return nil
	}
	if ttl > sessionStoreCacheTTL {
		ttl = sessionStoreCacheTTL
	}

	s.cache.Set(token, b, ttl)

	return nil
}

func (s *cachedSessionStore) Delete(token string) error {
	s.cache.Delete(token)

	return s.store.Delete(token)
}
