package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The empty-library table in library_handler_contract_test.go always validates an
// empty array, so this is the only coverage that checks a LatestShow element against
// the contract. The unenriched row exercises the nullable poster and premiere year.
func TestGetLatestShows_ConformsToOpenAPIWithRows(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

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

	app.InitSession()
	app.InitRouter()
	cookie := newAuthSessionCookie(t, app, user.ID)

	request := httptest.NewRequest(http.MethodGet, "/api/shows/latest", nil)
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	app.Router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusOK, response.Body.String())
	}

	// Without this the seeded rows could silently fail to insert and the contract
	// assertion below would vacuously validate an empty array.
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
	if len(body.Data.Shows) != 2 {
		t.Fatalf("shows length = %d, want 2, body = %s", len(body.Data.Shows), response.Body.String())
	}
	// Newest first: the unenriched show was inserted last.
	if body.Data.Shows[0].Name != "Unenriched Show" {
		t.Fatalf("first show = %q, want %q", body.Data.Shows[0].Name, "Unenriched Show")
	}

	assertOpenAPIExchange(t, "getLatestShows", request, response)
}
