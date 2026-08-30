package main

import (
	"sync"
	"time"

	"github.com/patrickmn/go-cache"
)

// generationCache is a read-through cache whose fills are ordered against the
// mutations that invalidate them. A reader that missed the cache may still be in
// its database query when a delete or a rescan evicts the key; without the
// generation guard that reader would publish the row it read before the mutation
// and keep stale data live until the TTL expired.
//
// The generation is global rather than per-key: fills take microseconds and
// invalidations happen only on delete and rescan, so discarding a handful of
// unrelated in-flight fills costs nothing and keeps the counter cheap.
//
// The TTL is only a backstop. Correctness comes from the explicit invalidation
// at every mutation that can change what is cached.
type generationCache[T any] struct {
	entries *cache.Cache

	// mu orders the generation against the entry it guards. Reads of the
	// entries map do not need it — go-cache locks its own map — but every
	// step that pairs the counter with a write to that map does.
	mu  sync.Mutex
	gen uint64
}

func newGenerationCache[T any](ttl time.Duration, sweep time.Duration) *generationCache[T] {
	return &generationCache[T]{entries: cache.New(ttl, sweep)}
}

// generation must be read before the database query whose result will be
// published with setIfCurrent.
func (c *generationCache[T]) generation() uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.gen
}

func (c *generationCache[T]) get(key string) (T, bool) {
	var zero T

	cached, hit := c.entries.Get(key)
	if !hit {
		return zero, false
	}

	resolved, ok := cached.(T)
	if !ok {
		return zero, false
	}

	return resolved, true
}

// setIfCurrent publishes a fill only when nothing was invalidated since gen was
// read. A stale fill is dropped rather than cached.
//
// The check and the publish are one critical section, shared with invalidate:
// an invalidation that landed between them would delete the key and then watch
// this call put the superseded row straight back, which is the exact eviction
// the guard exists to perform.
func (c *generationCache[T]) setIfCurrent(key string, gen uint64, resolved T) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.gen != gen {
		return
	}

	c.entries.SetDefault(key, resolved)
}

// invalidate must be called after the mutation commits, so a racing fill either
// reads the new row or is discarded by the generation bump.
func (c *generationCache[T]) invalidate(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.gen++
	c.entries.Delete(key)
}

// invalidateAll is for mutations that remove an unknown set of keys, such as an
// album delete cascading to its tracks.
func (c *generationCache[T]) invalidateAll() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.gen++
	c.entries.Flush()
}

// resolve is the read-through body: return the cached value, or run fill and
// publish it if nothing invalidated the key while fill was running.
//
// A fill the guard rejects is still returned to this one caller. It was true
// when its query ran, so serving it is no different from a request that
// finished a moment before the mutation; what must not happen is publishing it
// for the rest of the TTL, and setIfCurrent is what prevents that.
func (c *generationCache[T]) resolve(key string, fill func() (T, error)) (T, error) {
	cached, hit := c.get(key)
	if hit {
		return cached, nil
	}

	gen := c.generation()

	resolved, err := fill()
	if err != nil {
		var zero T
		return zero, err
	}

	c.setIfCurrent(key, gen, resolved)

	return resolved, nil
}
