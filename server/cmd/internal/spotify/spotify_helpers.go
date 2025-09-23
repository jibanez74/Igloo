package spotify

import "strings"

// normalizeCacheKey normalizes a string for use as a cache key
func normalizeCacheKey(key string) string {
	return strings.ToLower(strings.TrimSpace(key))
}
