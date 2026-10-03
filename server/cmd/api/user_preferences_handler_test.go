package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"igloo/cmd/internal/helpers"
)

const trailerPreferencesPath = "/api/user/preferences/trailers"

func decodeTrailerPreferences(t *testing.T, body string) trailerPreferencesResponse {
	t.Helper()

	var envelope struct {
		Error bool                       `json:"error"`
		Data  trailerPreferencesResponse `json:"data"`
	}
	err := json.Unmarshal([]byte(body), &envelope)
	if err != nil {
		t.Fatalf("decode trailer preferences: %v; body = %s", err, body)
	}
	if envelope.Error {
		t.Fatalf("unexpected error envelope: %s", body)
	}
	return envelope.Data
}

func TestTrailerPreferences_DefaultsWithoutARow(t *testing.T) {
	app := setupSessionTestApp(t)
	user := createTestUser(t, app, "Viewer", "viewer@example.com", false)

	req := newOpenAPIJSONRequest(http.MethodGet, trailerPreferencesPath, "")
	req.AddCookie(newAuthSessionCookie(t, app, user.ID))
	app.InitRouter()
	resp := serveOpenAPIExchange(t, app.Router, "getTrailerPreferences", req, http.StatusOK)

	prefs := decodeTrailerPreferences(t, resp.Body.String())
	if prefs.Enabled || prefs.Count != 2 || prefs.Source != "both" {
		t.Fatalf("defaults = %+v, want disabled, 2, both", prefs)
	}
}

func TestTrailerPreferences_UpdateRoundTrip(t *testing.T) {
	app := setupSessionTestApp(t)
	user := createTestUser(t, app, "Viewer", "viewer@example.com", false)
	app.InitRouter()

	req := newOpenAPIJSONRequest(http.MethodPut, trailerPreferencesPath, `{"enabled":true,"count":4,"source":"theaters"}`)
	req.AddCookie(newAuthSessionCookie(t, app, user.ID))
	resp := serveOpenAPIExchange(t, app.Router, "updateTrailerPreferences", req, http.StatusOK)
	saved := decodeTrailerPreferences(t, resp.Body.String())
	if !saved.Enabled || saved.Count != 4 || saved.Source != "theaters" {
		t.Fatalf("saved = %+v", saved)
	}

	read := serveAs(t, app, user.ID, http.MethodGet, trailerPreferencesPath, "")
	if read.Code != http.StatusOK {
		t.Fatalf("read status = %d: %s", read.Code, read.Body.String())
	}
	stored := decodeTrailerPreferences(t, read.Body.String())
	if stored != saved {
		t.Fatalf("stored = %+v, want %+v", stored, saved)
	}

	// A second save replaces the row rather than failing on the primary key.
	again := serveAs(t, app, user.ID, http.MethodPut, trailerPreferencesPath, `{"enabled":false,"count":1,"source":"library"}`)
	if again.Code != http.StatusOK {
		t.Fatalf("second save status = %d: %s", again.Code, again.Body.String())
	}
	updated := decodeTrailerPreferences(t, again.Body.String())
	if updated.Enabled || updated.Count != 1 || updated.Source != "library" {
		t.Fatalf("updated = %+v", updated)
	}
}

func TestTrailerPreferences_RejectsInvalidInput(t *testing.T) {
	app := setupSessionTestApp(t)
	user := createTestUser(t, app, "Viewer", "viewer@example.com", false)

	cases := []struct {
		name string
		body string
	}{
		{"missing enabled", `{"count":2,"source":"both"}`},
		{"missing count", `{"enabled":true,"source":"both"}`},
		{"missing source", `{"enabled":true,"count":2}`},
		{"count below range", `{"enabled":true,"count":0,"source":"both"}`},
		{"count above range", `{"enabled":true,"count":6,"source":"both"}`},
		{"unknown source", `{"enabled":true,"count":2,"source":"cinema"}`},
		{"unknown field", `{"enabled":true,"count":2,"source":"both","extra":1}`},
		{"wrong type", `{"enabled":"yes","count":2,"source":"both"}`},
		{"not json", `trailers`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := serveAs(t, app, user.ID, http.MethodPut, trailerPreferencesPath, tc.body)
			if resp.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400: %s", resp.Code, resp.Body.String())
			}
			var envelope helpers.JSONResponse
			err := json.Unmarshal(resp.Body.Bytes(), &envelope)
			if err != nil || !envelope.Error || envelope.Message == "" {
				t.Fatalf("want an error envelope with a message, got %s", resp.Body.String())
			}
		})
	}

	read := serveAs(t, app, user.ID, http.MethodGet, trailerPreferencesPath, "")
	prefs := decodeTrailerPreferences(t, read.Body.String())
	if prefs.Enabled {
		t.Fatal("a rejected save must not change the stored preferences")
	}
}

func TestTrailerPreferences_RequiresAuthentication(t *testing.T) {
	app := setupSessionTestApp(t)

	for _, method := range []string{http.MethodGet, http.MethodPut} {
		resp := serveAs(t, app, 0, method, trailerPreferencesPath, `{"enabled":true,"count":2,"source":"both"}`)
		if resp.Code != http.StatusUnauthorized {
			t.Fatalf("%s status = %d, want 401: %s", method, resp.Code, resp.Body.String())
		}
	}
}

func TestTrailerPreferences_AcceptsDeviceTokens(t *testing.T) {
	app := setupSessionTestApp(t)
	user := createTestUser(t, app, "TV Viewer", "tv@example.com", false)
	token := createTestDevice(t, app, user.ID, "Living Room TV", "android_tv")
	app.InitRouter()

	put := httptest.NewRequest(http.MethodPut, trailerPreferencesPath, strings.NewReader(`{"enabled":true,"count":3,"source":"library"}`))
	put.Header.Set("Content-Type", "application/json")
	put.Header.Set("Authorization", "Bearer "+token)
	putResp := httptest.NewRecorder()
	app.Router.ServeHTTP(putResp, put)
	if putResp.Code != http.StatusOK {
		t.Fatalf("device PUT status = %d: %s", putResp.Code, putResp.Body.String())
	}

	get := httptest.NewRequest(http.MethodGet, trailerPreferencesPath, nil)
	get.Header.Set("Authorization", "Bearer "+token)
	getResp := httptest.NewRecorder()
	app.Router.ServeHTTP(getResp, get)
	if getResp.Code != http.StatusOK {
		t.Fatalf("device GET status = %d: %s", getResp.Code, getResp.Body.String())
	}
	prefs := decodeTrailerPreferences(t, getResp.Body.String())
	if !prefs.Enabled || prefs.Count != 3 || prefs.Source != "library" {
		t.Fatalf("device-token prefs = %+v", prefs)
	}
}
