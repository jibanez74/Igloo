package main

import (
	"sync"
	"time"

	"github.com/patrickmn/go-cache"
)

// generationCache is a read-through cache whose fills are ordered against the
// mutations that invalidate them: a reader still in its database query when a
// delete or rescan evicts the key must not publish the row it read before the
// mutation. The generation is global rather than per-key, because invalidations
// are rare and discarding a few unrelated in-flight fills costs nothing. The TTL
// is only a backstop; correctness comes from invalidating at every mutation.
type generationCache[T any] struct {
	entries *cache.Cache

	// mu orders the generation against the entry it guards. go-cache locks its
	// own map, but every step that pairs the counter with a write needs mu.
	mu  sync.Mutex
	gen uint64
}

func newGenerationCache[T any](ttl time.Duration, sweep time.Duration) *generationCache[T] {
	return &generationCache[T]{entries: cache.New(ttl, sweep)}
}

func (c *generationCache[T]) get(key string) (T, bool) {
	cached, _ := c.entries.Get(key)
	resolved, ok := cached.(T)
	return resolved, ok
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

// resolve returns the cached value, or runs fill and publishes its result if
// nothing was invalidated while fill ran. A rejected fill is still returned to
// this caller, since it was true when its query ran; it is only kept out of the
// cache. The generation check and the publish share invalidate's critical
// section, so an invalidation cannot land between them.
func (c *generationCache[T]) resolve(key string, fill func() (T, error)) (T, error) {
	cached, hit := c.get(key)
	if hit {
		return cached, nil
	}

	c.mu.Lock()
	gen := c.gen
	c.mu.Unlock()

	resolved, err := fill()
	if err != nil {
		var zero T
		return zero, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.gen == gen {
		c.entries.SetDefault(key, resolved)
	}

	return resolved, nil
}
