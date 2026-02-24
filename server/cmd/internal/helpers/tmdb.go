package helpers

import "fmt"

// TmdbImageURL returns the full TMDB image URL for the given relative path and size.
// Size is typically "w500" (poster), "w185" (profile), "w92" (logo), or "original".
// Returns empty string if path is empty.
func TmdbImageURL(path, size string) string {
	if path == "" {
		return ""
	}
	return fmt.Sprintf("%s/%s/%s", TMDB_IMAGE_BASE_URL, size, path)
}
