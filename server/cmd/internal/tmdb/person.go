package tmdb

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

type PersonExternalIDs struct {
	IMDbID      string `json:"imdb_id"`
	FacebookID  string `json:"facebook_id"`
	InstagramID string `json:"instagram_id"`
	TwitterID   string `json:"twitter_id"`
	TikTokID    string `json:"tiktok_id"`
	YouTubeID   string `json:"youtube_id"`
	WikidataID  string `json:"wikidata_id"`
}

type PersonCredit struct {
	ID           int64   `json:"id"`
	MediaType    string  `json:"media_type"`
	Title        string  `json:"title"`
	Name         string  `json:"name"`
	ReleaseDate  string  `json:"release_date"`
	FirstAirDate string  `json:"first_air_date"`
	PosterPath   string  `json:"poster_path"`
	Popularity   float64 `json:"popularity"`
	Character    string  `json:"character"`
	Department   string  `json:"department"`
	Job          string  `json:"job"`
}

type Person struct {
	ID                 int64             `json:"id"`
	Name               string            `json:"name"`
	Biography          string            `json:"biography"`
	KnownForDepartment string            `json:"known_for_department"`
	Birthday           string            `json:"birthday"`
	Deathday           string            `json:"deathday"`
	PlaceOfBirth       string            `json:"place_of_birth"`
	Gender             int               `json:"gender"`
	AlsoKnownAs        []string          `json:"also_known_as"`
	ProfilePath        string            `json:"profile_path"`
	Homepage           string            `json:"homepage"`
	ExternalIDs        PersonExternalIDs `json:"external_ids"`
	CombinedCredits    struct {
		Cast []PersonCredit `json:"cast"`
		Crew []PersonCredit `json:"crew"`
	} `json:"combined_credits"`
	Images struct {
		Profiles []struct {
			FilePath string `json:"file_path"`
		} `json:"profiles"`
	} `json:"images"`
}

func (t *tmdbClient) GetPersonDetails(ctx context.Context, id int) (*Person, error) {
	if id <= 0 {
		return nil, errors.New("person ID is required")
	}
	var result Person
	err := t.getMetadata(ctx, fmt.Sprintf("/person/%d", id), url.Values{
		"append_to_response": {"combined_credits,external_ids,images"},
	}, &result, func() error {
		if result.ID != int64(id) || strings.TrimSpace(result.Name) == "" {
			return errors.New("invalid TMDB person identity or name")
		}
		return nil
	}, "combined_credits", "external_ids", "images")
	if err != nil {
		return nil, err
	}
	return &result, nil
}
