package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
)

// The empty-library table in library_handler_contract_test.go always validates an
// empty array, so this is the only coverage that checks a LatestShow element against
// the contract. The unenriched row exercises the nullable poster and premiere year.
// The contract rows are validated by TestListHandlers_ConformToOpenAPIWithRows;
// this pins the order the client relies on.
func TestGetLatestShows_NewestFirst(t *testing.T) {
	app := setupSessionTestApp(t)

	user := createTestUser(t, app, "Shows Contract User", "shows-contract@example.com", false)

	_, err := app.DB.Exec(`
 INSERT INTO shows (directory_path, local_name, premiere_year, name, poster_path)
 VALUES ('/shows/Contract Show (2026)', 'Contract Show', 2026, 'Contract Show', '/contract-show.jpg');
 INSERT INTO shows (directory_path, local_name, name)
 VALUES ('/shows/Unenriched Show', 'Unenriched Show', 'Unenriched Show');
 `)
	if err != nil {
		t.Fatalf("seed shows: %v", err)
	}

	response := httptest.NewRecorder()
	authenticatedRouter(t, app, user.ID).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/shows/latest", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusOK, response.Body.String())
	}

	var body struct {
		Data struct {
			Shows []struct {
				Name string `json:"name"`
			} `json:"shows"`
		} `json:"data"`
	}
	err = json.Unmarshal(response.Body.Bytes(), &body)
	if err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(body.Data.Shows) != 2 || body.Data.Shows[0].Name != "Unenriched Show" {
		t.Fatalf("shows = %+v, want the last inserted show first", body.Data.Shows)
	}
}

type showDetailsBody struct {
	Data struct {
		Show struct {
			Name             string `json:"name"`
			TmdbEpisodeCount struct {
				Int64 int64 `json:"Int64"`
				Valid bool  `json:"Valid"`
			} `json:"tmdb_episode_count"`
		} `json:"show"`
		Seasons []struct {
			SeasonNumber     int64 `json:"season_number"`
			TmdbEpisodeCount struct {
				Int64 int64 `json:"Int64"`
				Valid bool  `json:"Valid"`
			} `json:"tmdb_episode_count"`
			AvailableEpisodeCount int64 `json:"available_episode_count"`
		} `json:"seasons"`
		Cast []struct {
			CreditID string `json:"credit_id"`
			ArtistID int64  `json:"artist_id"`
		} `json:"cast"`
		Crew []struct {
			CreditID string `json:"credit_id"`
		} `json:"crew"`
		Creators []struct {
			Name string `json:"name"`
		} `json:"creators"`
		Genres []struct {
			Tag string `json:"tag"`
		} `json:"genres"`
		Networks []struct {
			Name string `json:"name"`
		} `json:"networks"`
		ProductionCompanies []struct {
			Name string `json:"name"`
		} `json:"production_companies"`
		ExtraVideos []struct {
			Key string `json:"key"`
		} `json:"extra_videos"`
	} `json:"data"`
}

func TestGetShowDetails_ConformsToOpenAPI(t *testing.T) {
	app := setupTestApp(t)

	user := createTestUser(t, app, "Show Details User", "show-details@example.com", false)
	showID := seedContractShow(t, app)

	handler := authenticatedRouter(t, app, user.ID)

	request := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/shows/details/%d", showID), nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusOK, response.Body.String())
	}

	// Every collection is asserted non-empty: an empty array validates against
	// the contract vacuously, so a seeding failure would pass silently.
	var body showDetailsBody
	err := json.Unmarshal(response.Body.Bytes(), &body)
	if err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if body.Data.Show.Name != "Contract Detail Show" {
		t.Fatalf("show name = %q, want %q", body.Data.Show.Name, "Contract Detail Show")
	}

	if len(body.Data.Seasons) != 3 {
		t.Fatalf("seasons length = %d, want 3, body = %s", len(body.Data.Seasons), response.Body.String())
	}

	// Specials sort last even though they were inserted first and number zero.
	gotOrder := []int64{}
	for _, s := range body.Data.Seasons {
		gotOrder = append(gotOrder, s.SeasonNumber)
	}
	wantOrder := []int64{1, 2, 0}
	if !slices.Equal(gotOrder, wantOrder) {
		t.Fatalf("season order = %v, want %v (specials last)", gotOrder, wantOrder)
	}

	// Season 2 is partially present: TMDB counts 8 episodes, one is on disk.
	season2 := body.Data.Seasons[1]
	if season2.AvailableEpisodeCount != 1 {
		t.Fatalf("season 2 available_episode_count = %d, want 1", season2.AvailableEpisodeCount)
	}
	if !season2.TmdbEpisodeCount.Valid || season2.TmdbEpisodeCount.Int64 != 8 {
		t.Fatalf("season 2 tmdb_episode_count = %+v, want 8", season2.TmdbEpisodeCount)
	}

	if len(body.Data.Cast) != 3 {
		t.Fatalf("cast length = %d, want 3", len(body.Data.Cast))
	}
	// One artist holds two credits, so credit_id is what keeps the rows distinct.
	if body.Data.Cast[0].ArtistID != body.Data.Cast[1].ArtistID {
		t.Fatalf("expected the first two cast rows to share an artist, got %d and %d",
			body.Data.Cast[0].ArtistID, body.Data.Cast[1].ArtistID)
	}
	if body.Data.Cast[0].CreditID == body.Data.Cast[1].CreditID {
		t.Fatalf("cast credit_id collision: %q", body.Data.Cast[0].CreditID)
	}

	if len(body.Data.Crew) != 2 {
		t.Fatalf("crew length = %d, want 2", len(body.Data.Crew))
	}
	if len(body.Data.Creators) != 1 {
		t.Fatalf("creators length = %d, want 1", len(body.Data.Creators))
	}
	if len(body.Data.Genres) != 1 {
		t.Fatalf("genres length = %d, want 1", len(body.Data.Genres))
	}
	if len(body.Data.Networks) != 1 {
		t.Fatalf("networks length = %d, want 1", len(body.Data.Networks))
	}
	if len(body.Data.ProductionCompanies) != 1 {
		t.Fatalf("production_companies length = %d, want 1", len(body.Data.ProductionCompanies))
	}
	if len(body.Data.ExtraVideos) != 1 {
		t.Fatalf("extra_videos length = %d, want 1", len(body.Data.ExtraVideos))
	}

	assertOpenAPIExchange(t, "getShowDetails", request, response)
}

func TestGetShowDetails_RejectsUnparsableID(t *testing.T) {
	app := setupTestApp(t)

	user := createTestUser(t, app, "Show Bad ID User", "show-bad-id@example.com", false)

	handler := authenticatedRouter(t, app, user.ID)

	request := httptest.NewRequest(http.MethodGet, "/api/shows/details/not-a-number", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusBadRequest, response.Body.String())
	}

	assertOpenAPIResponse(t, "getShowDetails", request, response)
}

func TestGetShowDetails_UnknownShowIsNotFound(t *testing.T) {
	app := setupTestApp(t)

	user := createTestUser(t, app, "Show Missing User", "show-missing@example.com", false)

	handler := authenticatedRouter(t, app, user.ID)

	request := httptest.NewRequest(http.MethodGet, "/api/shows/details/999999", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusNotFound, response.Body.String())
	}

	assertOpenAPIExchange(t, "getShowDetails", request, response)
}

type showSeasonEpisodesBody struct {
	Data struct {
		Season struct {
			SeasonNumber          int64 `json:"season_number"`
			AvailableEpisodeCount int64 `json:"available_episode_count"`
		} `json:"season"`
		Episodes []struct {
			EpisodeNumber int64  `json:"episode_number"`
			Name          string `json:"name"`
			TmdbRuntime   struct {
				Int64 int64 `json:"Int64"`
				Valid bool  `json:"Valid"`
			} `json:"tmdb_runtime"`
		} `json:"episodes"`
	} `json:"data"`
}

func TestGetShowSeasonEpisodes_ConformsToOpenAPI(t *testing.T) {
	app := setupTestApp(t)

	user := createTestUser(t, app, "Season Episodes User", "season-episodes@example.com", false)
	showID := seedContractShow(t, app)

	handler := authenticatedRouter(t, app, user.ID)

	request := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/shows/%d/seasons/1/episodes", showID), nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusOK, response.Body.String())
	}

	var body showSeasonEpisodesBody
	err := json.Unmarshal(response.Body.Bytes(), &body)
	if err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if body.Data.Season.SeasonNumber != 1 {
		t.Fatalf("season_number = %d, want 1", body.Data.Season.SeasonNumber)
	}
	if len(body.Data.Episodes) != 2 {
		t.Fatalf("episodes length = %d, want 2, body = %s", len(body.Data.Episodes), response.Body.String())
	}
	if body.Data.Episodes[0].EpisodeNumber != 1 || body.Data.Episodes[1].EpisodeNumber != 2 {
		t.Fatalf("episodes are not ordered by number: %+v", body.Data.Episodes)
	}
	// The second episode was left unenriched, so its nullable columns are null.
	if body.Data.Episodes[1].TmdbRuntime.Valid {
		t.Fatalf("expected the unenriched episode to have a null tmdb_runtime, got %+v", body.Data.Episodes[1].TmdbRuntime)
	}

	assertOpenAPIExchange(t, "getShowSeasonEpisodes", request, response)
}

// Specials are season zero, which the route must treat as a real season rather
// than as a missing or invalid one.
func TestGetShowSeasonEpisodes_SpecialsAreSeasonZero(t *testing.T) {
	app := setupTestApp(t)

	user := createTestUser(t, app, "Specials User", "specials@example.com", false)
	showID := seedContractShow(t, app)

	handler := authenticatedRouter(t, app, user.ID)

	request := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/shows/%d/seasons/0/episodes", showID), nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusOK, response.Body.String())
	}

	var body showSeasonEpisodesBody
	err := json.Unmarshal(response.Body.Bytes(), &body)
	if err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if body.Data.Season.SeasonNumber != 0 {
		t.Fatalf("season_number = %d, want 0", body.Data.Season.SeasonNumber)
	}
	if len(body.Data.Episodes) != 1 {
		t.Fatalf("episodes length = %d, want 1", len(body.Data.Episodes))
	}

	assertOpenAPIExchange(t, "getShowSeasonEpisodes", request, response)
}

func TestGetShowSeasonEpisodes_UnknownSeasonIsNotFound(t *testing.T) {
	app := setupTestApp(t)

	user := createTestUser(t, app, "Missing Season User", "missing-season@example.com", false)
	showID := seedContractShow(t, app)

	handler := authenticatedRouter(t, app, user.ID)

	request := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/shows/%d/seasons/47/episodes", showID), nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusNotFound, response.Body.String())
	}

	assertOpenAPIExchange(t, "getShowSeasonEpisodes", request, response)
}

// A season number that exists for some other show must not leak across shows.
func TestGetShowSeasonEpisodes_UnknownShowIsNotFound(t *testing.T) {
	app := setupTestApp(t)

	user := createTestUser(t, app, "Missing Show Season User", "missing-show-season@example.com", false)
	seedContractShow(t, app)

	handler := authenticatedRouter(t, app, user.ID)

	request := httptest.NewRequest(http.MethodGet, "/api/shows/999999/seasons/1/episodes", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusNotFound, response.Body.String())
	}

	assertOpenAPIExchange(t, "getShowSeasonEpisodes", request, response)
}

func TestGetShowSeasonEpisodes_RejectsUnparsableSeasonNumber(t *testing.T) {
	app := setupTestApp(t)

	user := createTestUser(t, app, "Bad Season User", "bad-season@example.com", false)
	showID := seedContractShow(t, app)

	handler := authenticatedRouter(t, app, user.ID)

	request := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/shows/%d/seasons/not-a-number/episodes", showID), nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusBadRequest, response.Body.String())
	}

	assertOpenAPIResponse(t, "getShowSeasonEpisodes", request, response)
}

// A negative season parses, but the contract's SeasonNumberPath starts at zero,
// so it is rejected as invalid rather than looked up and reported missing.
func TestGetShowSeasonEpisodes_RejectsNegativeSeasonNumber(t *testing.T) {
	app := setupTestApp(t)

	user := createTestUser(t, app, "Negative Season User", "negative-season@example.com", false)
	showID := seedContractShow(t, app)

	handler := authenticatedRouter(t, app, user.ID)

	request := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/shows/%d/seasons/-1/episodes", showID), nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusBadRequest, response.Body.String())
	}

	assertOpenAPIResponse(t, "getShowSeasonEpisodes", request, response)
}

// Sorting is case-insensitive on name and per_page is clamped, never rejected.
func TestGetShowsLibrary_SortsAndClampsPerPage(t *testing.T) {
	app := setupSessionTestApp(t)

	user := createTestUser(t, app, "Show Sort User", "show-sort@example.com", false)

	_, err := app.DB.Exec(`
 INSERT INTO shows (directory_path, local_name, name) VALUES ('/shows/beta', 'beta', 'beta');
 INSERT INTO shows (directory_path, local_name, name) VALUES ('/shows/Alpha', 'Alpha', 'Alpha');
 INSERT INTO shows (directory_path, local_name, name) VALUES ('/shows/gamma', 'gamma', 'gamma');
 `)
	if err != nil {
		t.Fatalf("seed shows: %v", err)
	}

	handler := authenticatedRouter(t, app, user.ID)

	cases := []struct {
		name        string
		query       string
		wantPerPage int64
		wantNames   []string
	}{
		{name: "asc default", query: "", wantPerPage: libraryDefaultPerPage, wantNames: []string{"Alpha", "beta", "gamma"}},
		{name: "desc clamped", query: "?sort=desc&per_page=999", wantPerPage: libraryMaxPerPage, wantNames: []string{"gamma", "beta", "Alpha"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/api/shows/library"+tc.query, nil)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusOK, response.Body.String())
			}

			var body struct {
				Data struct {
					Shows []struct {
						Name string `json:"name"`
					} `json:"shows"`
					Total   int64 `json:"total"`
					PerPage int64 `json:"per_page"`
				} `json:"data"`
			}
			err := json.Unmarshal(response.Body.Bytes(), &body)
			if err != nil {
				t.Fatalf("decode body: %v", err)
			}

			if body.Data.Total != 3 || body.Data.PerPage != tc.wantPerPage {
				t.Fatalf("total = %d, per_page = %d, want 3 and %d", body.Data.Total, body.Data.PerPage, tc.wantPerPage)
			}
			if len(body.Data.Shows) != len(tc.wantNames) {
				t.Fatalf("shows length = %d, want %d", len(body.Data.Shows), len(tc.wantNames))
			}
			for i, want := range tc.wantNames {
				if body.Data.Shows[i].Name != want {
					t.Fatalf("shows[%d] = %q, want %q", i, body.Data.Shows[i].Name, want)
				}
			}

			// per_page=999 is outside the documented input range, so only the
			// response is validated against the contract.
			assertOpenAPIResponse(t, "getShowsLibrary", request, response)
		})
	}
}

// The genre listing has its own descending query and row mapping.
func TestGetShowsByGenre_SortDescendingReversesTheAscendingOrder(t *testing.T) {
	app := setupSessionTestApp(t)
	user := createTestUser(t, app, "Genre Sort User", "genre-sort@example.com", false)
	showID := seedContractShow(t, app)
	var genreID int64
	err := app.DB.QueryRow("SELECT genre_id FROM show_genres WHERE show_id = ?", showID).Scan(&genreID)
	if err != nil {
		t.Fatalf("read seeded show genre: %v", err)
	}
	for _, name := range []string{"beta", "gamma"} {
		var siblingID int64
		err = app.DB.QueryRow("INSERT INTO shows (directory_path, local_name, name) VALUES (?, ?, ?) RETURNING id", "/shows/"+name, name, name).Scan(&siblingID)
		if err != nil {
			t.Fatalf("seed show %q: %v", name, err)
		}
		_, err = app.DB.Exec("INSERT INTO show_genres (show_id, genre_id) VALUES (?, ?)", siblingID, genreID)
		if err != nil {
			t.Fatalf("link show %q: %v", name, err)
		}
	}
	handler := authenticatedRouter(t, app, user.ID)

	listed := func(t *testing.T, sort string) []string {
		t.Helper()
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/shows/genres/%d/shows?sort=%s", genreID, sort), nil))
		if response.Code != http.StatusOK {
			t.Fatalf("sort=%s status = %d: %s", sort, response.Code, response.Body.String())
		}
		var body struct {
			Data struct {
				Shows []struct {
					Name string `json:"name"`
				} `json:"shows"`
			} `json:"data"`
		}
		err := json.Unmarshal(response.Body.Bytes(), &body)
		if err != nil {
			t.Fatalf("decode sort=%s: %v", sort, err)
		}
		names := make([]string, 0, len(body.Data.Shows))
		for _, show := range body.Data.Shows {
			names = append(names, show.Name)
		}
		return names
	}

	ascending := listed(t, "asc")
	descending := listed(t, "desc")
	if len(ascending) != 3 {
		t.Fatalf("ascending shows = %v, want all three", ascending)
	}
	slices.Reverse(ascending)
	if !slices.Equal(descending, ascending) {
		t.Fatalf("descending shows = %v, want %v", descending, ascending)
	}
}
