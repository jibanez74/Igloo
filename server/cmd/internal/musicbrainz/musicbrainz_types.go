package musicbrainz

import "time"

// Public API result types
type ArtistResult struct {
	MusicBrainzID  string
	Name           string
	SortName       string
	Country        string
	Type           string
	Disambiguation string
}

type AlbumResult struct {
	MusicBrainzID string
	ReleaseID     string
	Title         string
	ReleaseDate   string
	Year          int
	CoverURL      string
	ArtistName    string
}

// MusicBrainz API JSON response types (internal)
// JSON response structs for MusicBrainz API deserialization.
// Only the fields we need are included; encoding/json ignores unknown fields.

type artistSearchResponse struct {
	Artists []artistJSON `json:"artists"`
}

type artistJSON struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	SortName       string `json:"sort-name"`
	Type           string `json:"type"`
	Country        string `json:"country"`
	Disambiguation string `json:"disambiguation"`
}

type releaseGroupSearchResponse struct {
	ReleaseGroups []releaseGroupJSON `json:"release-groups"`
}

type releaseGroupJSON struct {
	ID               string             `json:"id"`
	Title            string             `json:"title"`
	PrimaryType      string             `json:"primary-type"`
	FirstReleaseDate string             `json:"first-release-date"`
	ArtistCredit     []artistCreditJSON `json:"artist-credit"`
	Releases         []releaseJSON      `json:"releases"`
}

type artistCreditJSON struct {
	Name   string          `json:"name"`
	Artist artistBriefJSON `json:"artist"`
}

type artistBriefJSON struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type releaseJSON struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

// In-memory cache entry types (internal)
// ---------------------------------------------------------------------------

type artistCacheEntry struct {
	result    *ArtistResult
	expiresAt time.Time
}

type albumCacheEntry struct {
	result    *AlbumResult
	expiresAt time.Time
}

type imageCacheEntry struct {
	url       string
	expiresAt time.Time
}

// ---------------------------------------------------------------------------
// TheAudioDB API JSON response types (internal)
// ---------------------------------------------------------------------------

type audioDBResponse struct {
	Artists []audioDBArtist `json:"artists"`
}

type audioDBArtist struct {
	ArtistThumb string `json:"strArtistThumb"`
}

type audioDBAlbumResponse struct {
	Albums []audioDBAlbum `json:"album"`
}

type audioDBAlbum struct {
	AlbumThumb string `json:"strAlbumThumb"`
}
