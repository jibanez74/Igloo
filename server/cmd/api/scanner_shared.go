package main

import "strings"

// normalizedScanCacheKey builds a stable, case-insensitive cache key from the
// given parts. It is shared by the movie and music scanners to key their
// in-scan lookup caches (genre tags, musician/album identities, etc.).
func normalizedScanCacheKey(parts ...string) string {
	normalized := make([]string, 0, len(parts))
	for _, part := range parts {
		normalized = append(normalized, strings.ToLower(strings.TrimSpace(part)))
	}

	return strings.Join(normalized, "\x00")
}
