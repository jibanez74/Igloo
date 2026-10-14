package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"igloo/cmd/internal/tmdb"
)

func seedPersonCredits(t *testing.T, app *Application) {
	t.Helper()
	_, err := app.DB.Exec(`
 INSERT INTO artist(id,tmdb_id,name,profile) VALUES (1,7,'Stored Person','/stored.jpg');
 INSERT INTO movies(id,title,file_path,file_name,size,container,mime_type,adult,tmdb_id,year) VALUES
 (1,'Movie','/one.mkv','one.mkv',1,'mkv','video/x-matroska',0,10,2024),
 (2,'Duplicate Movie','/two.mkv','two.mkv',1,'mkv','video/x-matroska',0,10,2024),
 (3,'Local only','/three.mkv','three.mkv',1,'mkv','video/x-matroska',0,NULL,2020);
 INSERT INTO cast(movie_id,artist_id,character,cast_order) VALUES (2,1,'Hero',0),(2,1,'Other',1),(3,1,'Local',0);
 INSERT INTO crew(movie_id,artist_id,department,job) VALUES (2,1,'Directing','Director');
 INSERT INTO shows(id,directory_path,local_name,name,tmdb_id,first_air_date) VALUES (1,'/show','Show','Show',20,'2023-01-01'),(2,'/show2','Show','Duplicate Show',20,'2023-01-01');
 INSERT INTO show_seasons(id,show_id,season_number,name) VALUES (1,2,1,'Season 1');
 INSERT INTO show_episodes(id,season_id,episode_number,name) VALUES (1,1,1,'Episode');
 INSERT INTO show_creators(show_id,artist_id) VALUES (2,1);
 INSERT INTO show_cast VALUES (2,1,'Regular',0,'aggregate-cast',2);
 INSERT INTO show_crew VALUES (2,1,'Production','Producer','aggregate-crew',2);
 INSERT INTO show_season_cast VALUES (1,1,'Season role',0,'season-cast',1);
 INSERT INTO show_season_crew VALUES (1,1,'Writing','Writer','season-crew',1);
 INSERT INTO show_episode_guest_cast VALUES (1,1,'Guest',0,'guest',1);
 INSERT INTO show_episode_crew VALUES (1,1,'Directing','Director','episode-crew',1);
 `)
	if err != nil {
		t.Fatal(err)
	}
}

func TestPersonProfile(t *testing.T) {
	for _, state := range []string{"available", "unconfigured", "unavailable", "not_found"} {
		t.Run(state, func(t *testing.T) {
			app := setupTestApp(t)
			user := createTestUser(t, app, "Person User", "person@example.com", false)
			seedPersonCredits(t, app)
			switch state {
			case "available":
				person := &tmdb.Person{ID: 7, Name: "Provider Person", Biography: "Biography", KnownForDepartment: "Acting"}
				person.CombinedCredits.Cast = []tmdb.PersonCredit{{ID: 10, MediaType: "movie", Title: "Movie", Character: "Hero", ReleaseDate: "2024-01-01"}, {ID: 10, MediaType: "movie", Title: "Movie", Character: "Hero"}, {ID: 30, MediaType: "movie", Title: "Remote", Character: "Self", Popularity: 90}}
				person.CombinedCredits.Crew = []tmdb.PersonCredit{{ID: 10, MediaType: "movie", Title: "Movie", Job: "Director", Department: "Directing"}}
				app.Tmdb = &stubTmdbClient{person: person}
			case "unavailable":
				app.Tmdb = &stubTmdbClient{personErr: errors.New("offline")}
			case "not_found":
				app.Tmdb = &stubTmdbClient{personErr: &tmdb.StatusError{StatusCode: 404}}
			}
			response := serveOpenAPIExchange(t, authenticatedRouter(t, app, user.ID), "getPerson", httptest.NewRequest(http.MethodGet, "/api/people/7", nil), http.StatusOK)
			var body struct {
				Data personData `json:"data"`
			}
			err := json.Unmarshal(response.Body.Bytes(), &body)
			if err != nil {
				t.Fatal(err)
			}
			data := body.Data
			if data.EnrichmentStatus != state || data.Person.ProfilePath != "/stored.jpg" {
				t.Fatalf("profile %+v", data)
			}
			foundMovie, foundShow := false, false
			for _, title := range data.Filmography {
				if title.TmdbID == 10 {
					foundMovie = true
					if title.LibraryID == nil || *title.LibraryID != 1 || len(title.Roles) != 3 {
						t.Fatalf("movie %+v", title)
					}
				}
				if title.TmdbID == 20 {
					foundShow = true
					if title.LibraryID == nil || *title.LibraryID != 1 || len(title.Roles) != 7 {
						t.Fatalf("show %+v", title)
					}
				}
			}
			if !foundMovie || !foundShow {
				t.Fatalf("missing credits %+v", data.Filmography)
			}
		})
	}
}

func TestPersonErrors(t *testing.T) {
	for _, test := range []struct {
		id       string
		provider *stubTmdbClient
		status   int
	}{
		{"bad", nil, 400}, {"0", nil, 400}, {"-1", nil, 400}, {"7", nil, 503},
		{"7", &stubTmdbClient{personErr: errors.New("offline")}, 503},
		{"7", &stubTmdbClient{personErr: &tmdb.StatusError{StatusCode: 404}}, 404},
	} {
		t.Run(fmt.Sprintf("%s/%d", test.id, test.status), func(t *testing.T) {
			app := setupTestApp(t)
			user := createTestUser(t, app, "Person User", "person@example.com", false)
			if test.provider != nil {
				app.Tmdb = test.provider
			}
			response := httptest.NewRecorder()
			authenticatedRouter(t, app, user.ID).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/people/"+test.id, nil))
			if response.Code != test.status {
				t.Fatalf("status %d want %d: %s", response.Code, test.status, response.Body)
			}
		})
	}
	app := setupTestApp(t)
	response := httptest.NewRecorder()
	authenticatedRouter(t, app, 0).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/people/7", nil))
	if response.Code != 401 {
		t.Fatalf("unauthenticated: %d", response.Code)
	}
}

func TestPersonProviderOnlyAndLargeFilmography(t *testing.T) {
	app := setupTestApp(t)
	user := createTestUser(t, app, "Person User", "person@example.com", false)
	seedPersonCredits(t, app)
	provider := &tmdb.Person{ID: 99, Name: "Remote Person"}
	for id := int64(1); id <= 1100; id++ {
		provider.CombinedCredits.Cast = append(provider.CombinedCredits.Cast, tmdb.PersonCredit{ID: id, MediaType: "movie", Title: fmt.Sprintf("Movie %d", id)})
	}
	provider.CombinedCredits.Cast = append(provider.CombinedCredits.Cast, tmdb.PersonCredit{ID: 20, MediaType: "tv", Name: "Show"})
	app.Tmdb = &stubTmdbClient{person: provider}
	response := serveOpenAPIExchange(t, authenticatedRouter(t, app, user.ID), "getPerson", httptest.NewRequest(http.MethodGet, "/api/people/99", nil), http.StatusOK)
	var body struct {
		Data personData `json:"data"`
	}
	err := json.Unmarshal(response.Body.Bytes(), &body)
	if err != nil {
		t.Fatal(err)
	}
	if len(body.Data.Filmography) != 1101 {
		t.Fatalf("filmography length %d", len(body.Data.Filmography))
	}
	matched := 0
	for _, title := range body.Data.Filmography {
		if title.LibraryID != nil {
			matched++
			if *title.LibraryID != 1 {
				t.Fatalf("library match %+v", title)
			}
		}
	}
	if matched != 2 {
		t.Fatalf("matched %d titles", matched)
	}
}

func TestPersonStoredIdentityWithoutCredits(t *testing.T) {
	app := setupTestApp(t)
	user := createTestUser(t, app, "Person User", "person@example.com", false)
	_, err := app.DB.Exec(`INSERT INTO artist(tmdb_id,name) VALUES (7,'Stored Person')`)
	if err != nil {
		t.Fatal(err)
	}
	serveOpenAPIExchange(t, authenticatedRouter(t, app, user.ID), "getPerson", httptest.NewRequest(http.MethodGet, "/api/people/7", nil), http.StatusOK)
}

func TestPersonDatabaseFailure(t *testing.T) {
	app := setupTestApp(t)
	user := createTestUser(t, app, "Person User", "person@example.com", false)
	_, err := app.DB.Exec("DROP TABLE artist")
	if err != nil {
		t.Fatal(err)
	}
	serveOpenAPIExchange(t, authenticatedRouter(t, app, user.ID), "getPerson", httptest.NewRequest(http.MethodGet, "/api/people/7", nil), http.StatusInternalServerError)
}
