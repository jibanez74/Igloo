package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"igloo/cmd/internal/helpers"
)

func TestGetUsers_HTTP_ExcludesCurrentUser(t *testing.T) {
	app := setupSessionTestApp(t)

	user1 := createTestUser(t, app, "Alice", "alice@example.com", false)
	createTestUser(t, app, "Bob", "bob@example.com", false)

	handler := authenticatedRouter(t, app, user1.ID)

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	assertOpenAPIExchange(t, "getUsers", req, w)

	var resp helpers.JSONResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	data, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatal("expected data to be an object")
	}

	users, ok := data["users"].([]any)
	if !ok {
		t.Fatal("expected users array")
	}

	if len(users) != 1 {
		t.Errorf("expected 1 user in response (excluding self), got %d", len(users))
	}
}

func TestGetUsers_HTTP_SearchTreatsLikeMetacharactersLiterally(t *testing.T) {
	app := setupSessionTestApp(t)

	user1 := createTestUser(t, app, "Alice Smith", "alice@example.com", false)
	createTestUser(t, app, "Bob_One", "bob_one@example.com", false)
	createTestUser(t, app, "BobXOne", "bobxone@example.com", false)
	createTestUser(t, app, "Carol%Two", "carol%two@example.com", false)
	createTestUser(t, app, "CarolTwo", "caroltwo@example.com", false)

	handler := authenticatedRouter(t, app, user1.ID)

	req := httptest.NewRequest(http.MethodGet, "/api/users?q=carol%25", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for percent search, got %d", w.Code)
	}

	var resp helpers.JSONResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode percent search response: %v", err)
	}

	data := resp.Data.(map[string]any)
	users := data["users"].([]any)
	if len(users) != 1 {
		t.Fatalf("expected 1 user matching literal percent, got %d", len(users))
	}

	percentMatch := users[0].(map[string]any)
	if percentMatch["name"] != "Carol%Two" {
		t.Fatalf("expected Carol%%Two for percent search, got %#v", percentMatch["name"])
	}

	req = httptest.NewRequest(http.MethodGet, "/api/users?q=bob_", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for underscore search, got %d", w.Code)
	}

	resp = helpers.JSONResponse{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode underscore search response: %v", err)
	}

	data = resp.Data.(map[string]any)
	users = data["users"].([]any)
	if len(users) != 1 {
		t.Fatalf("expected 1 user matching literal underscore, got %d", len(users))
	}

	underscoreMatch := users[0].(map[string]any)
	if underscoreMatch["name"] != "Bob_One" {
		t.Fatalf("expected Bob_One for underscore search, got %#v", underscoreMatch["name"])
	}
}
