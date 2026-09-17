package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

// The empty-library table in library_handler_contract_test.go validates every
// show list operation against an empty array, where the item schema is never
// exercised. These run with a seeded show so the elements are validated too.
func TestShowListHandlers_ConformToOpenAPIWithRows(t *testing.T) {
	app := setupSessionTestApp(t)
	defer app.DB.Close()

	user := createTestUser(t, app, "Show List User", "show-list-contract@example.com", false)
	showID := seedContractShow(t, app)

	var genreID int64
	err := app.DB.QueryRow("SELECT genre_id FROM show_genres WHERE show_id = ?", showID).Scan(&genreID)
	if err != nil {
		t.Fatalf("read seeded show genre: %v", err)
	}

	app.InitRouter()
	cookie := newAuthSessionCookie(t, app, user.ID)

	operations := []struct {
		operationID string
		path        string
		dataKey     string
	}{
		{operationID: "getShowsLibrary", path: "/api/shows/library", dataKey: "shows"},
		{operationID: "getShowsByGenreLibrary", path: "/api/shows/genres/" + strconv.FormatInt(genreID, 10) + "/shows", dataKey: "shows"},
		{operationID: "getShowGenresList", path: "/api/shows/genres", dataKey: "genres"},
	}

	for _, operation := range operations {
		t.Run(operation.operationID, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, operation.path, nil)
			request.AddCookie(cookie)
			response := httptest.NewRecorder()
			app.Router.ServeHTTP(response, request)

			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusOK, response.Body.String())
			}

			assertResponseListNotEmpty(t, operation.operationID, response.Body.Bytes(), operation.dataKey)
			assertOpenAPIExchange(t, operation.operationID, request, response)
		})
	}
}

// Sorting is case-insensitive on name and per_page is clamped, never rejected.
func TestGetShowsLibrary_SortsAndClampsPerPage(t *testing.T) {
	app := setupSessionTestApp(t)
	defer app.DB.Close()

	user := createTestUser(t, app, "Show Sort User", "show-sort@example.com", false)

	_, err := app.DB.Exec(`
 INSERT INTO shows (directory_path, local_name, name) VALUES ('/shows/beta', 'beta', 'beta');
 INSERT INTO shows (directory_path, local_name, name) VALUES ('/shows/Alpha', 'Alpha', 'Alpha');
 INSERT INTO shows (directory_path, local_name, name) VALUES ('/shows/gamma', 'gamma', 'gamma');
 `)
	if err != nil {
		t.Fatalf("seed shows: %v", err)
	}

	app.InitRouter()
	cookie := newAuthSessionCookie(t, app, user.ID)

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
			request.AddCookie(cookie)
			response := httptest.NewRecorder()
			app.Router.ServeHTTP(response, request)

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
