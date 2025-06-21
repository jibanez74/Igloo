package jellyfin

import (
	"net/http"
)

type JellyfinClient struct {
	Token         string
	BaseUrl       string
	ClientName    string
	ClientVersion string
	UserID        string
	httpClient    *http.Client
}

type JellyfinItem struct {
	Id           string    `json:"Id"`
	Name         string    `json:"Name"`
	Type         string    `json:"Type"`
	MediaType    string    `json:"MediaType,omitempty"`
	Path         string    `json:"Path,omitempty"`
	ParentId     string    `json:"ParentId,omitempty"`
	IsFolder     bool      `json:"IsFolder"`
	RunTimeTicks int64     `json:"RunTimeTicks,omitempty"`
	UserData     *UserData `json:"UserData,omitempty"`
}

type UserData struct {
	PlayCount             int   `json:"PlayCount"`
	IsFavorite            bool  `json:"IsFavorite"`
	Played                bool  `json:"Played"`
	PlaybackPositionTicks int64 `json:"PlaybackPositionTicks"`
}

type JellyfinUser struct {
	Id       string `json:"Id"`
	Name     string `json:"Name"`
	ServerId string `json:"ServerId"`
}

type JellyfinLibrary struct {
	Id   string `json:"Id"`
	Name string `json:"Name"`
	Type string `json:"CollectionType"`
}

// SearchResult represents search results
type SearchResult struct {
	Items            []JellyfinItem `json:"Items"`
	TotalRecordCount int            `json:"TotalRecordCount"`
	StartIndex       int            `json:"StartIndex"`
}

// Movie represents a movie item from Jellyfin
type Movie struct {
	Id           string    `json:"Id"`
	Name         string    `json:"Name"`
	Type         string    `json:"Type"`
	MediaType    string    `json:"MediaType,omitempty"`
	Path         string    `json:"Path,omitempty"`
	ParentId     string    `json:"ParentId,omitempty"`
	IsFolder     bool      `json:"IsFolder"`
	RunTimeTicks int64     `json:"RunTimeTicks,omitempty"`
	UserData     *UserData `json:"UserData,omitempty"`
	// Movie-specific fields
	ProductionYear  int               `json:"ProductionYear,omitempty"`
	OfficialRating  string            `json:"OfficialRating,omitempty"`
	Overview        string            `json:"Overview,omitempty"`
	Genres          []string          `json:"Genres,omitempty"`
	Studios         []string          `json:"Studios,omitempty"`
	Taglines        []string          `json:"Taglines,omitempty"`
	CommunityRating float64           `json:"CommunityRating,omitempty"`
	CriticRating    float64           `json:"CriticRating,omitempty"`
	PremiereDate    string            `json:"PremiereDate,omitempty"`
	EndDate         string            `json:"EndDate,omitempty"`
	ProviderIds     map[string]string `json:"ProviderIds,omitempty"`
}

// MovieSearchResult represents search results for movies
type MovieSearchResult struct {
	Items            []Movie `json:"Items"`
	TotalRecordCount int     `json:"TotalRecordCount"`
	StartIndex       int     `json:"StartIndex"`
}
