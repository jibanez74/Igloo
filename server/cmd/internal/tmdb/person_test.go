package tmdb

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

func TestPersonDetailsCache(t *testing.T) {
	var calls atomic.Int32
	client := newServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path != "/person/7" || r.URL.Query().Get("language") != "en-US" || r.URL.Query().Get("append_to_response") != "combined_credits,external_ids,images" {
			t.Errorf("unexpected request %s", r.URL.Path)
		}
		fmt.Fprint(w, `{"id":7,"name":"Person","birthday":null,"biography":"First.\n\nSecond.","also_known_as":["Alias"],"combined_credits":{"cast":[{"id":1,"media_type":"movie","character":"A"},{"id":2,"media_type":"tv","name":"Show","first_air_date":"2020-01-01"}],"crew":[{"id":1,"media_type":"movie","job":"Director"}]},"external_ids":{"imdb_id":"nm7","instagram_id":"person"},"images":{"profiles":[{"file_path":"/p.jpg"}]}}`)
	}))
	person, err := client.GetPersonDetails(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if person.ID != 7 || person.Birthday != "" || len(person.CombinedCredits.Cast) != 2 || person.ExternalIDs.IMDbID != "nm7" || len(person.Images.Profiles) != 1 {
		t.Fatalf("person = %+v", person)
	}
	person.AlsoKnownAs[0] = "mutated"
	person.CombinedCredits.Cast[0].Character = "mutated"
	again, err := client.GetPersonDetails(context.Background(), 7)
	if err != nil || calls.Load() != 1 || again.AlsoKnownAs[0] != "Alias" || again.CombinedCredits.Cast[0].Character != "A" {
		t.Fatalf("cache: %+v %v calls=%d", again, err, calls.Load())
	}
	for _, item := range client.movieCache.Items() {
		remaining := time.Until(time.Unix(0, item.Expiration))
		if remaining < 14*time.Minute || remaining > 15*time.Minute {
			t.Fatalf("cache TTL %v", remaining)
		}
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = client.GetPersonDetails(canceled, 7)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled cache lookup: %v", err)
	}
	client.ClearCache()
	_, err = client.GetPersonDetails(context.Background(), 7)
	if err != nil || calls.Load() != 2 {
		t.Fatalf("clear: %v calls=%d", err, calls.Load())
	}
}

func TestPersonFailuresNotCached(t *testing.T) {
	for _, payload := range []string{`{"id":8,"name":"Wrong","combined_credits":{},"external_ids":{},"images":{}}`, `{"id":7,"name":"","combined_credits":{},"external_ids":{},"images":{}}`, `{"id":7,"name":"Missing"}`, `{"id":7,"name":"Failed","combined_credits":{"success":false,"status_code":34},"external_ids":{},"images":{}}`, `{"id":7,"name":"Null","combined_credits":null,"external_ids":{},"images":{}}`, `{`} {
		t.Run(payload, func(t *testing.T) {
			calls := 0
			client := newServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; fmt.Fprint(w, payload) }))
			for range 2 {
				_, err := client.GetPersonDetails(context.Background(), 7)
				if err == nil {
					t.Fatal("expected error")
				}
			}
			if calls != 2 {
				t.Fatalf("failed response cached: %d", calls)
			}
			_, err := client.GetPersonDetails(context.Background(), 0)
			if err == nil || calls != 2 {
				t.Fatal("invalid ID reached provider")
			}
		})
	}
	client := newServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNotFound) }))
	_, err := client.GetPersonDetails(context.Background(), 7)
	if !IsNotFound(err) {
		t.Fatalf("not found: %v", err)
	}
}
