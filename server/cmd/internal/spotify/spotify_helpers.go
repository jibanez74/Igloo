package spotify

import "strings"

func normalizeCacheKey(key string) string {
	return strings.ToLower(strings.TrimSpace(key))
}
