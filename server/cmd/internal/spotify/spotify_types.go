package spotify

import (
	"encoding/json"
	"time"
)

// These types mirror the Spotify Web API objects, trimmed to the fields Igloo
// reads. Spotify omits some of them for some catalog entries (popularity,
// followers, genres), which decode as zero values.

type ID string

func (id ID) String() string {
	return string(id)
}

// Numeric decodes a JSON number sent as either an integer or a float, and a
// null as zero.
type Numeric int

func (n *Numeric) UnmarshalJSON(data []byte) error {
	var value float64
	err := json.Unmarshal(data, &value)
	if err != nil {
		return err
	}

	*n = Numeric(int(value))
	return nil
}

type Image struct {
	URL string `json:"url"`
}

type Followers struct {
	Count Numeric `json:"total"`
}

type SimpleArtist struct {
	ID   ID     `json:"id"`
	Name string `json:"name"`
}

type FullArtist struct {
	SimpleArtist
	Popularity Numeric   `json:"popularity"`
	Genres     []string  `json:"genres"`
	Followers  Followers `json:"followers"`
	Images     []Image   `json:"images"`
}

type SimpleAlbum struct {
	ID                   ID                `json:"id"`
	Name                 string            `json:"name"`
	Artists              []SimpleArtist    `json:"artists"`
	AlbumType            string            `json:"album_type"`
	Images               []Image           `json:"images"`
	ExternalURLs         map[string]string `json:"external_urls"`
	ReleaseDate          string            `json:"release_date"`
	ReleaseDatePrecision string            `json:"release_date_precision"`
	TotalTracks          Numeric           `json:"total_tracks"`
}

// ReleaseDateTime parses ReleaseDate at its precision; a "month" or "year"
// date reads as the first day of that period. A missing or unparseable date,
// and Spotify's "0000" placeholder year, return the zero time.
func (a SimpleAlbum) ReleaseDateTime() time.Time {
	layout := "2006"
	switch a.ReleaseDatePrecision {
	case "day":
		layout = time.DateOnly
	case "month":
		layout = "2006-01"
	}

	date, err := time.Parse(layout, a.ReleaseDate)
	if err != nil {
		return time.Time{}
	}

	placeholderYear := date.Year() == 0
	if placeholderYear {
		return time.Time{}
	}

	return date
}

type FullAlbum struct {
	SimpleAlbum
	Genres     []string `json:"genres"`
	Popularity Numeric  `json:"popularity"`
}

type FullTrack struct {
	ID           ID                `json:"id"`
	Name         string            `json:"name"`
	Artists      []SimpleArtist    `json:"artists"`
	Album        SimpleAlbum       `json:"album"`
	Duration     Numeric           `json:"duration_ms"`
	ExternalURLs map[string]string `json:"external_urls"`
}

// searchResult holds the pages of one search. A page Spotify did not return
// stays nil, which callers distinguish from a page with no items.
type searchResult struct {
	Artists *struct {
		Items []FullArtist `json:"items"`
	} `json:"artists"`
	Albums *struct {
		Items []SimpleAlbum `json:"items"`
	} `json:"albums"`
	Tracks *struct {
		Items []FullTrack `json:"items"`
	} `json:"tracks"`
}
