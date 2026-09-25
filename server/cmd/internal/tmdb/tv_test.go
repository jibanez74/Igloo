package tmdb

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"

	cache "github.com/patrickmn/go-cache"
)

func TestTVRequestsCachingAndFallback(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Query().Get("api_key") != "test-api-key" || r.URL.Query().Get("language") != "en-US" {
			t.Errorf("missing shared request parameters: %s", r.URL)
		}
		switch r.URL.Path {
		case "/search/tv":
			if r.URL.Query().Get("query") != "Example" || r.URL.Query().Has("year") {
				t.Errorf("bad search: %s", r.URL)
			}
			if r.URL.Query().Get("first_air_date_year") != "" {
				fmt.Fprint(w, `{"results":[]}`)
				return
			}
			fmt.Fprint(w, `{"results":[{"id":7,"name":"Example"}]}`)
		case "/tv/7":
			if r.URL.Query().Get("append_to_response") != "aggregate_credits,content_ratings,external_ids,videos" {
				t.Error("missing appended show endpoints")
			}
			fmt.Fprint(w, `{"id":7,"name":"Example","aggregate_credits":{"cast":[{"id":8,"name":"Person","roles":[{"character":"A"},{"character":"B"}]}]},"external_ids":{"imdb_id":"tt7"},"content_ratings":{"results":[{"iso_3166_1":"GB","rating":"15"},{"iso_3166_1":"US","rating":"TV-14"}]},"videos":{"results":[]}}`)
		case "/tv/7/season/0":
			if r.URL.Query().Get("append_to_response") != "aggregate_credits,videos" {
				t.Error("missing appended season endpoints")
			}
			fmt.Fprint(w, `{"id":70,"season_number":0,"episodes":[{"id":71,"season_number":0,"episode_number":1,"guest_stars":[{"id":9,"character":"Guest"}],"crew":[{"id":10,"job":"Director"}]}],"aggregate_credits":{},"videos":{"results":[]}}`)
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()
	client := newTestClient(server.URL)
	ctx := context.Background()
	found, err := client.SearchShowsByTitleAndYear(ctx, "Example", 2020)
	if err != nil || len(found) != 1 {
		t.Fatalf("search: %v %v", found, err)
	}
	beforeSearch := calls.Load()
	_, err = client.SearchShowsByTitleAndYear(ctx, "Example")
	if err != nil || calls.Load() != beforeSearch {
		t.Fatal("unfiltered search did not reuse fallback cache", err)
	}
	show, err := client.GetShowDetails(ctx, 7)
	if err != nil {
		t.Fatal(err)
	}
	if show.Certification() != "TV-14" || len(show.AggregateCredits.Cast[0].Roles) != 2 || show.ExternalIDs.IMDbID != "tt7" {
		t.Fatalf("mapping: %+v", show)
	}
	show.AggregateCredits.Cast[0].Roles[0].Character = "changed"
	again, err := client.GetShowDetails(ctx, 7)
	if err != nil || again.AggregateCredits.Cast[0].Roles[0].Character != "A" {
		t.Fatal("caller mutated cached response", err)
	}
	season, err := client.GetSeasonDetails(ctx, 7, 0)
	if err != nil || season.Episodes[0].ID != 71 || len(season.Episodes[0].GuestStars) != 1 || len(season.Episodes[0].Crew) != 1 {
		t.Fatal("season", err)
	}
	before := calls.Load()
	_, err = client.GetSeasonDetails(ctx, 7, 0)
	if err != nil || calls.Load() != before {
		t.Fatal("cache miss", err)
	}
	client.ClearCache()
	_, err = client.GetShowDetails(ctx, 7)
	if err != nil || calls.Load() != before+1 {
		t.Fatal("cache not cleared", err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = client.GetShowDetails(canceled, 7)
	if !errors.Is(err, context.Canceled) {
		t.Fatal("cached request ignored cancellation", err)
	}
}

func TestTVFailuresAreNotCached(t *testing.T) {
	for _, body := range []string{
		`{"id":8,"name":"wrong","aggregate_credits":{},"content_ratings":{},"external_ids":{},"videos":{}}`,
		`{"id":7,"name":"partial","aggregate_credits":{"success":false,"status_code":25},"content_ratings":{},"external_ids":{},"videos":{}}`,
		`{"id":7,"name":"missing","content_ratings":{},"external_ids":{},"videos":{}}`,
	} {
		t.Run(body, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); fmt.Fprint(w, body) }))
			defer server.Close()
			client := newTestClient(server.URL)
			for range 2 {
				_, err := client.GetShowDetails(context.Background(), 7)
				if err == nil {
					t.Fatal("accepted invalid response")
				}
			}
			if calls.Load() != 2 {
				t.Fatal("cached failure")
			}
		})
	}
}

func TestTVRateLimitAndMissingResults(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(429)
			return
		}
		fmt.Fprint(w, `{"results":[]}`)
	}))
	defer server.Close()
	client := newTestClient(server.URL)
	_, err := client.SearchShowsByTitleAndYear(context.Background(), "missing")
	if !errors.Is(err, ErrNoShowsFound) || calls.Load() != 2 {
		t.Fatal("rate-limit retry", err, calls.Load())
	}
	_, err = client.SearchShowsByTitleAndYear(context.Background(), "missing")
	if !errors.Is(err, ErrNoShowsFound) || calls.Load() != 3 {
		t.Fatal("cached empty result", err, calls.Load())
	}
}

func TestTVSeasonNumberValidation(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"season id missing", `{"id":0,"season_number":1,"episodes":[],"aggregate_credits":{},"videos":{}}`},
		{"season number mismatch", `{"id":1,"season_number":2,"episodes":[],"aggregate_credits":{},"videos":{}}`},
		{"episode id missing", `{"id":1,"season_number":1,"episodes":[{"id":0,"season_number":1,"episode_number":1}],"aggregate_credits":{},"videos":{}}`},
		{"episode season mismatch", `{"id":1,"season_number":1,"episodes":[{"id":2,"season_number":2,"episode_number":1}],"aggregate_credits":{},"videos":{}}`},
		{"episode number missing", `{"id":1,"season_number":1,"episodes":[{"id":2,"season_number":1,"episode_number":0}],"aggregate_credits":{},"videos":{}}`},
		{"episode number duplicated", `{"id":1,"season_number":1,"episodes":[{"id":2,"season_number":1,"episode_number":1},{"id":3,"season_number":1,"episode_number":1}],"aggregate_credits":{},"videos":{}}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, tc.body) }))
			defer server.Close()
			_, err := newTestClient(server.URL).GetSeasonDetails(context.Background(), 7, 1)
			if err == nil {
				t.Fatal("accepted invalid season response")
			}
		})
	}
}

func TestTVArgumentValidationSkipsTheNetwork(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	defer server.Close()
	client := newTestClient(server.URL)
	ctx := context.Background()

	_, err := client.SearchShowsByTitleAndYear(ctx, "  \t ")
	if err == nil || err.Error() != "show title is required" {
		t.Fatalf("blank title error = %v", err)
	}
	_, err = client.GetShowDetails(ctx, 0)
	if err == nil || err.Error() != "show ID is required" {
		t.Fatalf("show id 0 error = %v", err)
	}
	_, err = client.GetSeasonDetails(ctx, 0, 1)
	if err == nil || err.Error() != "invalid show ID or season" {
		t.Fatalf("season with show id 0 error = %v", err)
	}
	_, err = client.GetSeasonDetails(ctx, 7, -1)
	if err == nil || err.Error() != "invalid show ID or season" {
		t.Fatalf("negative season error = %v", err)
	}
	if calls.Load() != 0 {
		t.Fatalf("expected no HTTP requests, got %d", calls.Load())
	}
}

func TestTVResponseErrors(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		wantErr string
	}{
		{"not found", http.StatusNotFound, `{"status_message":"missing"}`, "tmdb status 404: unable to get TV metadata from tmdb"},
		{"malformed json", http.StatusOK, `{"id":`, "unexpected end of JSON input"},
		{"required field null", http.StatusOK, `{"id":7,"name":"Example","aggregate_credits":null,"content_ratings":{},"external_ids":{},"videos":{}}`, "missing TMDB TV field aggregate_credits"},
		{"identity mismatch", http.StatusOK, `{"id":8,"name":"Example","aggregate_credits":{},"content_ratings":{},"external_ids":{},"videos":{}}`, "invalid TMDB show identity or name"},
		{"blank name", http.StatusOK, `{"id":7,"name":" ","aggregate_credits":{},"content_ratings":{},"external_ids":{},"videos":{}}`, "invalid TMDB show identity or name"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			_, err := newTestClient(server.URL).GetShowDetails(context.Background(), 7)
			if err == nil || err.Error() != tc.wantErr {
				t.Fatalf("error = %v, want %q", err, tc.wantErr)
			}
			if tc.status == http.StatusOK {
				return
			}
			var status *StatusError
			if !errors.As(err, &status) || status.StatusCode != tc.status {
				t.Fatalf("expected StatusError with %d, got %v", tc.status, err)
			}
		})
	}
}

func TestTVSearchRejectsResultWithoutIdentity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"results":[{"id":7,"name":"Example"},{"id":0,"name":"Nameless"}]}`)
	}))
	defer server.Close()
	_, err := newTestClient(server.URL).SearchShowsByTitleAndYear(context.Background(), "Example")
	if err == nil || err.Error() != "invalid TMDB show identity" {
		t.Fatalf("error = %v, want invalid TMDB show identity", err)
	}
}

// A cached value that is not a byte slice (another cache user's type under a
// colliding key) is refetched instead of decoded.
func TestTVCacheEntryOfWrongTypeIsRefetched(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		fmt.Fprint(w, `{"id":7,"name":"Example","aggregate_credits":{},"content_ratings":{},"external_ids":{},"videos":{}}`)
	}))
	defer server.Close()
	client := newTestClient(server.URL)
	params := url.Values{"append_to_response": {"aggregate_credits,content_ratings,external_ids,videos"}, "language": {tmdbRequestLanguage}}
	client.movieCache.Set("tv:/tv/7?"+params.Encode(), &TmdbMovie{}, cache.DefaultExpiration)

	show, err := client.GetShowDetails(context.Background(), 7)
	if err != nil || show.Name != "Example" {
		t.Fatalf("GetShowDetails = %+v, %v", show, err)
	}
	if calls.Load() != 1 {
		t.Fatalf("expected the wrong-type entry to force one request, got %d", calls.Load())
	}
	_, err = client.GetShowDetails(context.Background(), 7)
	if err != nil || calls.Load() != 1 {
		t.Fatalf("expected the refetched body to be cached, got %d calls, %v", calls.Load(), err)
	}
}

func TestTVCertificationFallsBackToFirstRating(t *testing.T) {
	var show TVShow
	show.ContentRatings.Results = []struct {
		Country string `json:"iso_3166_1"`
		Rating  string `json:"rating"`
	}{
		{Country: "GB", Rating: " "},
		{Country: "DE", Rating: "16"},
		{Country: "FR", Rating: "12"},
	}
	got := show.Certification()
	if got != "16" {
		t.Fatalf("Certification() = %q, want the first non-blank rating 16", got)
	}
	show.ContentRatings.Results = nil
	got = show.Certification()
	if got != "" {
		t.Fatalf("Certification() = %q, want empty without ratings", got)
	}
}
