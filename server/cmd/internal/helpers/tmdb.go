package helpers

import "fmt"

// TmdbImageURL returns the full TMDB image URL for the given relative path and size
// (e.g. TMDB_POSTER_SIZE, TMDB_PROFILE_SIZE, TMDB_LOGO_SIZE, TMDB_IMAGE_SIZE).
// Returns empty string if path is empty.
func TmdbImageURL(path, size string) string {
	if path == "" {
		return ""
	}
	return fmt.Sprintf("%s/%s/%s", TMDB_IMAGE_BASE_URL, size, path)
}
