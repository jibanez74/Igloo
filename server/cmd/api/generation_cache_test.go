package main

import (
	"errors"
	"testing"
	"time"
)

func newTestGenerationCache() *generationCache[string] {
	return newGenerationCache[string](time.Minute, 2*time.Minute)
}

func TestGenerationCachePublishesAFillFromTheCurrentGeneration(t *testing.T) {
	c := newTestGenerationCache()

	resolved, err := c.resolve("movie:1", func() (string, error) {
		return "path", nil
	})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolved != "path" {
		t.Errorf("resolve = %q, want %q", resolved, "path")
	}

	cached, hit := c.get("movie:1")
	if !hit {
		t.Fatal("a fill from the current generation was not published")
	}
	if cached != "path" {
		t.Errorf("cached = %q, want %q", cached, "path")
	}
}

// The guard's whole job: a fill that started before a mutation must not land in
// the cache after it, or the superseded row stays live for the rest of the TTL.
// Invalidating from inside fill is how the test pins that interleaving down.
func TestGenerationCacheDropsAFillSupersededByInvalidate(t *testing.T) {
	c := newTestGenerationCache()
	c.entries.SetDefault("movie:1", "before")

	resolved, err := c.resolve("movie:2", func() (string, error) {
		c.invalidate("movie:1")
		return "stale", nil
	})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	// The caller still gets what its query read; only publication is refused.
	if resolved != "stale" {
		t.Errorf("resolve = %q, want the value fill returned", resolved)
	}
	if _, hit := c.get("movie:2"); hit {
		t.Error("a fill superseded by an invalidation was published")
	}
	if _, hit := c.get("movie:1"); hit {
		t.Error("invalidate did not evict its own key")
	}
}

func TestGenerationCacheDropsAFillSupersededByInvalidateAll(t *testing.T) {
	c := newTestGenerationCache()
	c.entries.SetDefault("track:1", "before")

	_, err := c.resolve("track:2", func() (string, error) {
		c.invalidateAll()
		return "stale", nil
	})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	if _, hit := c.get("track:2"); hit {
		t.Error("a fill superseded by invalidateAll was published")
	}
	if _, hit := c.get("track:1"); hit {
		t.Error("invalidateAll did not flush the cache")
	}
}

func TestGenerationCacheDoesNotPublishAFailedFill(t *testing.T) {
	c := newTestGenerationCache()

	_, err := c.resolve("movie:1", func() (string, error) {
		return "", errors.New("query failed")
	})
	if err == nil {
		t.Fatal("resolve swallowed the fill error")
	}
	if _, hit := c.get("movie:1"); hit {
		t.Error("a failed fill was published")
	}
}
