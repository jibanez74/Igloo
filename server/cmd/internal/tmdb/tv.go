package tmdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	cache "github.com/patrickmn/go-cache"
)

var ErrNoShowsFound = errors.New("no shows found with the given query")

type TVPerson struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	ProfilePath string `json:"profile_path"`
}
type TVRole struct {
	Character    string `json:"character"`
	Job          string `json:"job"`
	CreditID     string `json:"credit_id"`
	EpisodeCount int    `json:"episode_count"`
}
type TVAggregateCast struct {
	TVPerson
	Order int      `json:"order"`
	Roles []TVRole `json:"roles"`
}
type TVAggregateCrew struct {
	TVPerson
	Department string   `json:"department"`
	Jobs       []TVRole `json:"jobs"`
}
type TVAggregateCredits struct {
	Cast []TVAggregateCast `json:"cast"`
	Crew []TVAggregateCrew `json:"crew"`
}
type TVCastCredit struct {
	TVPerson
	Character string `json:"character"`
	Order     int    `json:"order"`
	CreditID  string `json:"credit_id"`
}
type TVCrewCredit struct {
	TVPerson
	Department string `json:"department"`
	Job        string `json:"job"`
	CreditID   string `json:"credit_id"`
}
type TVVideos struct {
	Results []TmdbVideoResult `json:"results"`
}
type TVShow struct {
	ID                  int                 `json:"id"`
	Name                string              `json:"name"`
	OriginalName        string              `json:"original_name"`
	Overview            string              `json:"overview"`
	Tagline             string              `json:"tagline"`
	OriginalLanguage    string              `json:"original_language"`
	OriginCountry       []string            `json:"origin_country"`
	FirstAirDate        string              `json:"first_air_date"`
	LastAirDate         string              `json:"last_air_date"`
	Status              string              `json:"status"`
	Type                string              `json:"type"`
	Adult               bool                `json:"adult"`
	PosterPath          string              `json:"poster_path"`
	BackdropPath        string              `json:"backdrop_path"`
	Homepage            string              `json:"homepage"`
	VoteAverage         float64             `json:"vote_average"`
	VoteCount           int                 `json:"vote_count"`
	Popularity          float64             `json:"popularity"`
	NumberOfSeasons     int                 `json:"number_of_seasons"`
	NumberOfEpisodes    int                 `json:"number_of_episodes"`
	Genres              []Genre             `json:"genres"`
	ProductionCompanies []ProductionCompany `json:"production_companies"`
	Networks            []ProductionCompany `json:"networks"`
	CreatedBy           []TVPerson          `json:"created_by"`
	AggregateCredits    TVAggregateCredits  `json:"aggregate_credits"`
	Videos              TVVideos            `json:"videos"`
	ExternalIDs         struct {
		IMDbID string `json:"imdb_id"`
	} `json:"external_ids"`
	ContentRatings struct {
		Results []struct {
			Country string `json:"iso_3166_1"`
			Rating  string `json:"rating"`
		} `json:"results"`
	} `json:"content_ratings"`
}
type TVSeason struct {
	ID               int                `json:"id"`
	SeasonNumber     int                `json:"season_number"`
	Name             string             `json:"name"`
	Overview         string             `json:"overview"`
	AirDate          string             `json:"air_date"`
	PosterPath       string             `json:"poster_path"`
	VoteAverage      float64            `json:"vote_average"`
	Episodes         []TVEpisode        `json:"episodes"`
	AggregateCredits TVAggregateCredits `json:"aggregate_credits"`
	Videos           TVVideos           `json:"videos"`
}
type TVEpisode struct {
	ID             int     `json:"id"`
	SeasonNumber   int     `json:"season_number"`
	EpisodeNumber  int     `json:"episode_number"`
	Name           string  `json:"name"`
	Overview       string  `json:"overview"`
	AirDate        string  `json:"air_date"`
	StillPath      string  `json:"still_path"`
	ProductionCode string  `json:"production_code"`
	Runtime        int     `json:"runtime"`
	VoteAverage    float64 `json:"vote_average"`
	VoteCount      int     `json:"vote_count"`
	// GuestStars and Crew ride along in the season payload, so episode credits
	// never need a request of their own.
	GuestStars []TVCastCredit `json:"guest_stars"`
	Crew       []TVCrewCredit `json:"crew"`
}

func (s *TVShow) Certification() string {
	first := ""
	for _, r := range s.ContentRatings.Results {
		rating := strings.TrimSpace(r.Rating)
		if rating == "" {
			continue
		}
		if r.Country == tmdbCertificationCountry {
			return rating
		}
		if first == "" {
			first = rating
		}
	}
	return first
}

// Decode a fresh value even on cache hits, so callers cannot mutate cached slices.
// Required appended responses must be complete before the response is cached.
func (t *tmdbClient) getTV(ctx context.Context, path string, params url.Values, result any, validate func() error, required ...string) error {
	err := ctx.Err()
	if err != nil {
		return err
	}
	params.Set("language", tmdbRequestLanguage)
	key := "tv:" + path + "?" + params.Encode()
	var body []byte
	cached, found := t.movieCache.Get(key)
	if found {
		body, _ = cached.([]byte)
	}
	if body == nil {
		var status int
		requestParams := make(url.Values, len(params)+1)
		for key, values := range params {
			requestParams[key] = values
		}
		body, status, err = t.getJSON(ctx, t.requestURL(path, requestParams))
		if err != nil {
			return err
		}
		if status != http.StatusOK {
			return tmdbStatusError(status, "unable to get TV metadata from tmdb")
		}
	}
	var object map[string]json.RawMessage
	err = json.Unmarshal(body, &object)
	if err != nil {
		return err
	}
	for _, field := range required {
		raw, exists := object[field]
		if !exists || string(raw) == "null" {
			return fmt.Errorf("missing TMDB TV field %s", field)
		}
		if len(raw) > 0 && raw[0] == '{' {
			var status struct {
				Success    *bool `json:"success"`
				StatusCode int   `json:"status_code"`
			}
			err = json.Unmarshal(raw, &status)
			if err != nil {
				return err
			}
			failed := status.Success != nil && !*status.Success
			if failed || status.StatusCode != 0 {
				return fmt.Errorf("TMDB TV %s failed", field)
			}
		}
	}
	err = json.Unmarshal(body, result)
	if err != nil {
		return err
	}
	err = validate()
	if err != nil {
		return err
	}
	t.movieCache.Set(key, body, cache.DefaultExpiration)
	return ctx.Err()
}

func (t *tmdbClient) SearchShowsByTitleAndYear(ctx context.Context, title string, year ...int) ([]TVShow, error) {
	emptyTitle := strings.TrimSpace(title) == ""
	if emptyTitle {
		return nil, errors.New("show title is required")
	}
	params := url.Values{"query": {title}, "include_adult": {"false"}}
	if len(year) > 0 && year[0] > 0 {
		params.Set("first_air_date_year", strconv.Itoa(year[0]))
	}
	var response struct {
		Results []TVShow `json:"results"`
	}
	search := func() error {
		return t.getTV(ctx, "/search/tv", params, &response, func() error {
			if len(response.Results) == 0 {
				return ErrNoShowsFound
			}
			for _, s := range response.Results {
				if s.ID <= 0 {
					return errors.New("invalid TMDB show identity")
				}
			}
			return nil
		}, "results")
	}
	err := search()
	retryUnfiltered := errors.Is(err, ErrNoShowsFound) && params.Get("first_air_date_year") != ""
	if retryUnfiltered {
		params.Del("first_air_date_year")
		err = search()
	}
	return response.Results, err
}

func (t *tmdbClient) GetShowDetails(ctx context.Context, id int) (*TVShow, error) {
	if id <= 0 {
		return nil, errors.New("show ID is required")
	}
	var result TVShow
	err := t.getTV(ctx, fmt.Sprintf("/tv/%d", id), url.Values{"append_to_response": {"aggregate_credits,content_ratings,external_ids,videos"}}, &result, func() error {
		validIdentity := result.ID == id && strings.TrimSpace(result.Name) != ""
		if !validIdentity {
			return errors.New("invalid TMDB show identity or name")
		}
		return nil
	}, "aggregate_credits", "content_ratings", "external_ids", "videos")
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (t *tmdbClient) GetSeasonDetails(ctx context.Context, id, season int) (*TVSeason, error) {
	if id <= 0 || season < 0 {
		return nil, errors.New("invalid show ID or season")
	}
	var result TVSeason
	err := t.getTV(ctx, fmt.Sprintf("/tv/%d/season/%d", id, season), url.Values{"append_to_response": {"aggregate_credits,videos"}}, &result, func() error {
		if result.ID <= 0 || result.SeasonNumber != season {
			return errors.New("invalid TMDB season identity or numbering")
		}
		seen := make(map[int]bool)
		for _, ep := range result.Episodes {
			if ep.ID <= 0 || ep.SeasonNumber != season || ep.EpisodeNumber <= 0 || seen[ep.EpisodeNumber] {
				return errors.New("invalid TMDB episode identity or numbering")
			}
			seen[ep.EpisodeNumber] = true
		}
		return nil
	}, "season_number", "episodes", "aggregate_credits", "videos")
	if err != nil {
		return nil, err
	}
	return &result, nil
}
