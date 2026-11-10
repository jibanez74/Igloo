package main

import (
	"maps"
	"sync"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/patrickmn/go-cache"
)

const (
	// sessionCodecMemoMax bounds the decoded sessions kept. A household has
	// far fewer live sessions, so reaching it means replaced ones piled up.
	sessionCodecMemoMax = 1024

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
	ttl := min(time.Until(expiry), sessionStoreCacheTTL)
	if ttl <= 0 {
		s.cache.Delete(token)
		return nil
	}

	s.cache.Set(token, b, ttl)

	return nil
}

func (s *cachedSessionStore) Delete(token string) error {
	s.cache.Delete(token)

	return s.store.Delete(token)
}

type decodedSession struct {
	deadline time.Time
	values   map[string]any
}

// memoSessionCodec remembers what a session's bytes decode to. scs decodes
// them on every request, a cached read included, and gob compiles a decoder
// for each one. The stored format stays gob. The values are copied one level
// deep, which is enough for the integers Igloo keeps in a session.
type memoSessionCodec struct {
	scs.GobCodec
	mu      sync.RWMutex
	decoded map[string]decodedSession
}

func newMemoSessionCodec() *memoSessionCodec {
	return &memoSessionCodec{decoded: make(map[string]decodedSession)}
}

func (c *memoSessionCodec) Decode(b []byte) (time.Time, map[string]any, error) {
	c.mu.RLock()
	cached, hit := c.decoded[string(b)]
	c.mu.RUnlock()
	if hit {
		return cached.deadline, maps.Clone(cached.values), nil
	}

	deadline, values, err := c.GobCodec.Decode(b)
	if err != nil {
		return time.Time{}, nil, err
	}

	c.mu.Lock()
	isFull := len(c.decoded) >= sessionCodecMemoMax
	if isFull {
		clear(c.decoded)
	}
	c.decoded[string(b)] = decodedSession{deadline: deadline, values: maps.Clone(values)}
	c.mu.Unlock()

	return deadline, values, nil
}
