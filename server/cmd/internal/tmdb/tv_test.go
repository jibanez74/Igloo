package tmdb

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
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
			fmt.Fprint(w, `{"id":70,"season_number":0,"episodes":[{"id":71,"season_number":0,"episode_number":1}],"aggregate_credits":{},"videos":{"results":[]}}`)
		case "/tv/7/season/0/episode/1/credits":
			fmt.Fprint(w, `{"id":71,"cast":[],"guest_stars":[{"id":9,"character":"Guest"}],"crew":[]}`)
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
	if err != nil || season.Episodes[0].ID != 71 {
		t.Fatal("season", err)
	}
	credits, err := client.GetEpisodeCredits(ctx, 7, 0, 1)
	if err != nil || len(credits.GuestStars) != 1 {
		t.Fatal("credits", err)
	}
	before := calls.Load()
	_, err = client.GetSeasonDetails(ctx, 7, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.GetEpisodeCredits(ctx, 7, 0, 1)
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

func TestTVSeasonNumberValidationAndCancellation(t *testing.T) {
	for _, body := range []string{
		`{"id":1,"season_number":2,"episodes":[],"aggregate_credits":{},"videos":{}}`,
		`{"id":1,"season_number":1,"episodes":[{"id":2,"season_number":2,"episode_number":1}],"aggregate_credits":{},"videos":{}}`,
		`{"id":1,"season_number":1,"episodes":[{"id":2,"season_number":1,"episode_number":1},{"id":3,"season_number":1,"episode_number":1}],"aggregate_credits":{},"videos":{}}`,
	} {
		t.Run(body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
			defer server.Close()
			_, err := newTestClient(server.URL).GetSeasonDetails(context.Background(), 7, 1)
			if err == nil {
				t.Fatal("accepted invalid season response")
			}
		})
	}
	entered := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(entered); <-r.Context().Done() }))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	client := newTestClient(server.URL)
	go func() { _, err := client.GetEpisodeCredits(ctx, 7, 1, 1); done <- err }()
	<-entered
	cancel()
	err := <-done
	if !errors.Is(err, context.Canceled) {
		t.Fatal("in-flight TV request ignored cancellation", err)
	}
}
