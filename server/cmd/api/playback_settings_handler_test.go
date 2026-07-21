package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"

	"github.com/go-chi/chi/v5"
)

func setupPlaybackHTTPTestApp(t *testing.T) *Application {
	t.Helper()

	app := setupTestApp(t)
	app.InitSession()
	clearSettingsEnv(t)

	err := app.InitSettings(context.Background())
	if err != nil {
		t.Fatalf("InitSettings failed: %v", err)
	}

	return app
}

func mountPlaybackRouter(app *Application, userID int64) http.Handler {
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if userID != 0 {
				app.SessionManager.Put(r.Context(), cookieUserID, userID)
			}
			next.ServeHTTP(w, r)
		})
	})
	r.Group(func(r chi.Router) {
		r.Use(app.IsAuth)
		r.Get("/api/settings/playback", app.GetPlaybackSettings)
		r.Put("/api/settings/playback", app.UpdatePlaybackSettings)
	})

	return app.SessionManager.LoadAndSave(r)
}

func createTestUser(t *testing.T, app *Application, name, email string, isAdmin bool) database.User {
	t.Helper()
	user, err := app.Queries.CreateUser(context.Background(), database.CreateUserParams{
		Name:     name,
		Email:    email,
		Password: "hashed",
		IsAdmin:  isAdmin,
		Avatar:   sql.NullString{},
	})
	if err != nil {
		t.Fatalf("create user %q: %v", email, err)
	}
	return user
}

type playbackSettingsEnvelope struct {
	Data struct {
		Settings playbackSettingsResponse `json:"settings"`
	} `json:"data"`
}

func decodePlaybackResponse(t *testing.T, body []byte) playbackSettingsResponse {
	t.Helper()
	var env playbackSettingsEnvelope
	err := json.Unmarshal(body, &env)
	if err != nil {
		t.Fatalf("decode response: %v\nbody=%s", err, string(body))
	}
	return env.Data.Settings
}

func TestGetPlaybackSettings_ReturnsDefaultsForNewUser(t *testing.T) {
	app := setupPlaybackHTTPTestApp(t)
	defer app.DB.Close()

	user := createTestUser(t, app, "Regular", "regular@example.com", false)
	handler := mountPlaybackRouter(app, user.ID)

	req := httptest.NewRequest(http.MethodGet, "/api/settings/playback", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	settings := decodePlaybackResponse(t, w.Body.Bytes())
	if settings.PreferredProfile != nil {
		t.Fatalf("expected preferred_profile nil, got %v", *settings.PreferredProfile)
	}
	if settings.DownloadMbps != nil {
		t.Fatalf("expected download_mbps nil, got %v", *settings.DownloadMbps)
	}
	if settings.IsAdmin {
		t.Fatal("expected is_admin false for regular user")
	}
	if settings.HardwareAccelerationDevice != helpers.HARDWARE_ACCELERATION_DEVICE_CPU {
		t.Fatalf("expected default hardware device cpu, got %q", settings.HardwareAccelerationDevice)
	}
	if len(settings.Profiles) == 0 {
		t.Fatal("expected non-empty profile catalog")
	}
	for _, p := range settings.Profiles {
		if p.ID == helpers.HLS_PROFILE_REMUX {
			t.Fatalf("remux profile should be excluded from catalog")
		}
		if p.VideoMbps <= 0 || p.Height <= 0 || p.Label == "" {
			t.Fatalf("malformed profile entry: %+v", p)
		}
	}
}

func TestUpdatePlaybackSettings_RoundTrips(t *testing.T) {
	app := setupPlaybackHTTPTestApp(t)
	defer app.DB.Close()

	user := createTestUser(t, app, "Regular", "regular@example.com", false)
	handler := mountPlaybackRouter(app, user.ID)

	body := fmt.Sprintf(`{"preferred_profile": %q, "download_mbps": 50}`, helpers.HLS_PROFILE_1080P_6MBPS)
	req := httptest.NewRequest(http.MethodPut, "/api/settings/playback", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	prefs, err := app.Queries.GetUserPlaybackPreferences(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("GetUserPlaybackPreferences: %v", err)
	}
	if !prefs.PreferredHlsProfile.Valid || prefs.PreferredHlsProfile.String != helpers.HLS_PROFILE_1080P_6MBPS {
		t.Fatalf("expected preferred profile %q, got valid=%v %q", helpers.HLS_PROFILE_1080P_6MBPS, prefs.PreferredHlsProfile.Valid, prefs.PreferredHlsProfile.String)
	}
	if !prefs.DownloadMbps.Valid || prefs.DownloadMbps.Float64 != 50 {
		t.Fatalf("expected download Mbps 50, got valid=%v %v", prefs.DownloadMbps.Valid, prefs.DownloadMbps.Float64)
	}

	settings := decodePlaybackResponse(t, w.Body.Bytes())
	if settings.PreferredProfile == nil || *settings.PreferredProfile != helpers.HLS_PROFILE_1080P_6MBPS {
		t.Fatalf("response preferred_profile mismatch: %+v", settings.PreferredProfile)
	}
}

func TestUpdatePlaybackSettings_RejectsUnknownProfile(t *testing.T) {
	app := setupPlaybackHTTPTestApp(t)
	defer app.DB.Close()

	user := createTestUser(t, app, "Regular", "regular@example.com", false)
	handler := mountPlaybackRouter(app, user.ID)

	body := `{"preferred_profile": "8k_supreme", "download_mbps": 50}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings/playback", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdatePlaybackSettings_RejectsRemuxProfile(t *testing.T) {
	app := setupPlaybackHTTPTestApp(t)
	defer app.DB.Close()

	user := createTestUser(t, app, "Regular", "regular@example.com", false)
	handler := mountPlaybackRouter(app, user.ID)

	body := `{"preferred_profile": "remux", "download_mbps": 50}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings/playback", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdatePlaybackSettings_RejectsInvalidDownloadMbps(t *testing.T) {
	app := setupPlaybackHTTPTestApp(t)
	defer app.DB.Close()

	user := createTestUser(t, app, "Regular", "regular@example.com", false)
	handler := mountPlaybackRouter(app, user.ID)

	body := fmt.Sprintf(`{"preferred_profile": %q, "download_mbps": -5}`, helpers.HLS_PROFILE_1080P_6MBPS)
	req := httptest.NewRequest(http.MethodPut, "/api/settings/playback", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdatePlaybackSettings_DownloadMbpsBoundaries(t *testing.T) {
	cases := []struct {
		name       string
		value      string
		wantStatus int
		wantStored float64
	}{
		{"zero rejected", "0", http.StatusBadRequest, 0},
		{"just above zero accepted", "0.1", http.StatusOK, 0.1},
		{"just below ceiling accepted", "9999.99", http.StatusOK, 9999.99},
		{"ceiling rejected", "10000", http.StatusBadRequest, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := setupPlaybackHTTPTestApp(t)
			defer app.DB.Close()

			user := createTestUser(t, app, "Regular", "regular@example.com", false)
			handler := mountPlaybackRouter(app, user.ID)

			body := `{"download_mbps": ` + tc.value + `}`
			req := httptest.NewRequest(http.MethodPut, "/api/settings/playback", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != tc.wantStatus {
				t.Fatalf("value=%s expected status %d, got %d: %s", tc.value, tc.wantStatus, w.Code, w.Body.String())
			}

			if tc.wantStatus == http.StatusOK {
				prefs, err := app.Queries.GetUserPlaybackPreferences(context.Background(), user.ID)
				if err != nil {
					t.Fatalf("GetUserPlaybackPreferences: %v", err)
				}
				if !prefs.DownloadMbps.Valid || prefs.DownloadMbps.Float64 != tc.wantStored {
					t.Fatalf("expected stored download_mbps %v, got valid=%v %v", tc.wantStored, prefs.DownloadMbps.Valid, prefs.DownloadMbps.Float64)
				}
			}
		})
	}
}

func TestPlaybackSettings_RequiresAuth(t *testing.T) {
	app := setupPlaybackHTTPTestApp(t)
	defer app.DB.Close()

	handler := mountPlaybackRouter(app, 0)

	req := httptest.NewRequest(http.MethodGet, "/api/settings/playback", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for GET, got %d: %s", w.Code, w.Body.String())
	}

	putBody := fmt.Sprintf(`{"preferred_profile": %q, "download_mbps": 50}`, helpers.HLS_PROFILE_1080P_6MBPS)
	putReq := httptest.NewRequest(http.MethodPut, "/api/settings/playback", strings.NewReader(putBody))
	putReq.Header.Set("Content-Type", "application/json")
	putW := httptest.NewRecorder()
	handler.ServeHTTP(putW, putReq)

	if putW.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for PUT, got %d: %s", putW.Code, putW.Body.String())
	}
}

func TestUpdatePlaybackSettings_PutResponseShape(t *testing.T) {
	app := setupPlaybackHTTPTestApp(t)
	defer app.DB.Close()

	user := createTestUser(t, app, "Regular", "regular@example.com", false)
	handler := mountPlaybackRouter(app, user.ID)

	body := fmt.Sprintf(`{"preferred_profile": %q, "download_mbps": 50}`, helpers.HLS_PROFILE_1080P_6MBPS)
	req := httptest.NewRequest(http.MethodPut, "/api/settings/playback", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var raw map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &raw)
	if err != nil {
		t.Fatalf("decode raw: %v\nbody=%s", err, w.Body.String())
	}

	data, ok := raw["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data object, got %T", raw["data"])
	}
	settings, ok := data["settings"].(map[string]any)
	if !ok {
		t.Fatalf("expected settings object, got %T", data["settings"])
	}

	allowed := map[string]struct{}{
		"preferred_profile":           {},
		"download_mbps":               {},
		"preferred_audio_language":    {},
		"preferred_subtitle_language": {},
	}
	for k := range settings {
		_, ok := allowed[k]
		if !ok {
			t.Fatalf("unexpected key %q in PUT response settings; PUT payload should not include GET-only fields like profiles/server_upload_mbps/is_admin", k)
		}
	}
	for k := range allowed {
		_, ok := settings[k]
		if !ok {
			t.Fatalf("missing required key %q in PUT response settings", k)
		}
	}
}

func TestUpdatePlaybackSettings_EmptyStringPreferredProfileClearsColumn(t *testing.T) {
	app := setupPlaybackHTTPTestApp(t)
	defer app.DB.Close()

	user := createTestUser(t, app, "Regular", "regular@example.com", false)
	handler := mountPlaybackRouter(app, user.ID)

	seedBody := fmt.Sprintf(`{"preferred_profile": %q, "download_mbps": 50}`, helpers.HLS_PROFILE_1080P_6MBPS)
	seedReq := httptest.NewRequest(http.MethodPut, "/api/settings/playback", strings.NewReader(seedBody))
	seedReq.Header.Set("Content-Type", "application/json")
	seedW := httptest.NewRecorder()
	handler.ServeHTTP(seedW, seedReq)
	if seedW.Code != http.StatusOK {
		t.Fatalf("seed PUT: expected 200, got %d: %s", seedW.Code, seedW.Body.String())
	}

	clearBody := `{"preferred_profile": "", "download_mbps": 50}`
	clearReq := httptest.NewRequest(http.MethodPut, "/api/settings/playback", strings.NewReader(clearBody))
	clearReq.Header.Set("Content-Type", "application/json")
	clearW := httptest.NewRecorder()
	handler.ServeHTTP(clearW, clearReq)
	if clearW.Code != http.StatusOK {
		t.Fatalf("clear PUT: expected 200, got %d: %s", clearW.Code, clearW.Body.String())
	}

	prefs, err := app.Queries.GetUserPlaybackPreferences(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("GetUserPlaybackPreferences: %v", err)
	}
	if prefs.PreferredHlsProfile.Valid {
		t.Fatalf("expected preferred_hls_profile NULL after empty-string clear, got %q", prefs.PreferredHlsProfile.String)
	}
}

func TestUpdatePlaybackSettings_PartialUpdatePreservesOmittedField(t *testing.T) {
	app := setupPlaybackHTTPTestApp(t)
	defer app.DB.Close()

	user := createTestUser(t, app, "Regular", "regular@example.com", false)
	handler := mountPlaybackRouter(app, user.ID)

	seedBody := fmt.Sprintf(`{"preferred_profile": %q, "download_mbps": 50}`, helpers.HLS_PROFILE_1080P_6MBPS)
	seedReq := httptest.NewRequest(http.MethodPut, "/api/settings/playback", strings.NewReader(seedBody))
	seedReq.Header.Set("Content-Type", "application/json")
	seedW := httptest.NewRecorder()
	handler.ServeHTTP(seedW, seedReq)
	if seedW.Code != http.StatusOK {
		t.Fatalf("seed PUT: expected 200, got %d: %s", seedW.Code, seedW.Body.String())
	}

	body := `{"download_mbps": 75}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings/playback", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	prefs, err := app.Queries.GetUserPlaybackPreferences(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("GetUserPlaybackPreferences: %v", err)
	}
	if !prefs.PreferredHlsProfile.Valid || prefs.PreferredHlsProfile.String != helpers.HLS_PROFILE_1080P_6MBPS {
		t.Fatalf("expected preferred_hls_profile %q preserved, got valid=%v %q", helpers.HLS_PROFILE_1080P_6MBPS, prefs.PreferredHlsProfile.Valid, prefs.PreferredHlsProfile.String)
	}
	if !prefs.DownloadMbps.Valid || prefs.DownloadMbps.Float64 != 75 {
		t.Fatalf("expected download_mbps 75, got valid=%v %v", prefs.DownloadMbps.Valid, prefs.DownloadMbps.Float64)
	}
}

func TestUpdatePlaybackSettings_NullClearsBothColumns(t *testing.T) {
	app := setupPlaybackHTTPTestApp(t)
	defer app.DB.Close()

	user := createTestUser(t, app, "Regular", "regular@example.com", false)
	handler := mountPlaybackRouter(app, user.ID)

	seedBody := fmt.Sprintf(`{"preferred_profile": %q, "download_mbps": 50}`, helpers.HLS_PROFILE_1080P_6MBPS)
	seedReq := httptest.NewRequest(http.MethodPut, "/api/settings/playback", strings.NewReader(seedBody))
	seedReq.Header.Set("Content-Type", "application/json")
	seedW := httptest.NewRecorder()
	handler.ServeHTTP(seedW, seedReq)
	if seedW.Code != http.StatusOK {
		t.Fatalf("seed PUT: expected 200, got %d: %s", seedW.Code, seedW.Body.String())
	}

	clearBody := `{"preferred_profile": null, "download_mbps": null}`
	clearReq := httptest.NewRequest(http.MethodPut, "/api/settings/playback", strings.NewReader(clearBody))
	clearReq.Header.Set("Content-Type", "application/json")
	clearW := httptest.NewRecorder()
	handler.ServeHTTP(clearW, clearReq)
	if clearW.Code != http.StatusOK {
		t.Fatalf("clear PUT: expected 200, got %d: %s", clearW.Code, clearW.Body.String())
	}

	prefs, err := app.Queries.GetUserPlaybackPreferences(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("GetUserPlaybackPreferences: %v", err)
	}
	if prefs.PreferredHlsProfile.Valid {
		t.Fatalf("expected preferred_hls_profile NULL after clear, got %q", prefs.PreferredHlsProfile.String)
	}
	if prefs.DownloadMbps.Valid {
		t.Fatalf("expected download_mbps NULL after clear, got %v", prefs.DownloadMbps.Float64)
	}
}

func TestPlaybackSettings_AdminServerSettingsRoundTrip(t *testing.T) {
	app := setupPlaybackHTTPTestApp(t)
	defer app.DB.Close()

	admin := createTestUser(t, app, "Admin", "admin@example.com", true)
	handler := mountPlaybackRouter(app, admin.ID)

	performPut := func(t *testing.T, body string) {
		t.Helper()
		req := httptest.NewRequest(http.MethodPut, "/api/settings/playback", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("put playback: expected 200, got %d: %s", w.Code, w.Body.String())
		}
	}
	performGet := func(t *testing.T) playbackSettingsResponse {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/api/settings/playback", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("get playback: expected 200, got %d: %s", w.Code, w.Body.String())
		}
		return decodePlaybackResponse(t, w.Body.Bytes())
	}

	performPut(t, `{"server_upload_mbps": 50, "hardware_acceleration_device": "nvidia"}`)
	settings := performGet(t)
	if settings.ServerUploadMbps == nil || *settings.ServerUploadMbps != 50 {
		t.Fatalf("expected server_upload_mbps 50 after admin set, got %+v", settings.ServerUploadMbps)
	}
	if settings.HardwareAccelerationDevice != helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA {
		t.Fatalf("expected hardware device nvidia after admin set, got %q", settings.HardwareAccelerationDevice)
	}

	performPut(t, `{"server_upload_mbps": null}`)
	settings2 := performGet(t)
	if settings2.ServerUploadMbps != nil {
		t.Fatalf("expected server_upload_mbps nil after admin clear, got %+v", *settings2.ServerUploadMbps)
	}
	if settings2.HardwareAccelerationDevice != helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA {
		t.Fatalf("expected hardware device to stay nvidia when field omitted, got %q", settings2.HardwareAccelerationDevice)
	}
}

func TestGetPlaybackSettings_ReportsServerUploadCap(t *testing.T) {
	app := setupPlaybackHTTPTestApp(t)
	defer app.DB.Close()

	app.Settings.ServerUploadMbps = sql.NullFloat64{Float64: 30, Valid: true}

	user := createTestUser(t, app, "Regular", "regular@example.com", false)
	handler := mountPlaybackRouter(app, user.ID)

	req := httptest.NewRequest(http.MethodGet, "/api/settings/playback", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	settings := decodePlaybackResponse(t, w.Body.Bytes())
	if settings.ServerUploadMbps == nil || *settings.ServerUploadMbps != 30 {
		t.Fatalf("expected server_upload_mbps 30, got %+v", settings.ServerUploadMbps)
	}
}

func TestUpdatePlaybackSettings_AdminCanUpdateServerUploadCap(t *testing.T) {
	app := setupPlaybackHTTPTestApp(t)
	defer app.DB.Close()

	admin := createTestUser(t, app, "Admin", "admin@example.com", true)
	handler := mountPlaybackRouter(app, admin.ID)

	body := fmt.Sprintf(`{
		"preferred_profile": %q,
		"download_mbps": 75,
		"preferred_audio_language": "en",
		"preferred_subtitle_language": "off",
		"server_upload_mbps": 12.5
	}`, helpers.HLS_PROFILE_1080P_6MBPS)
	req := httptest.NewRequest(http.MethodPut, "/api/settings/playback", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	prefs, err := app.Queries.GetUserPlaybackPreferences(context.Background(), admin.ID)
	if err != nil {
		t.Fatalf("GetUserPlaybackPreferences: %v", err)
	}
	if !prefs.PreferredHlsProfile.Valid || prefs.PreferredHlsProfile.String != helpers.HLS_PROFILE_1080P_6MBPS {
		t.Fatalf("expected preferred profile %q, got valid=%v %q", helpers.HLS_PROFILE_1080P_6MBPS, prefs.PreferredHlsProfile.Valid, prefs.PreferredHlsProfile.String)
	}
	if !prefs.DownloadMbps.Valid || prefs.DownloadMbps.Float64 != 75 {
		t.Fatalf("expected download Mbps 75, got valid=%v %v", prefs.DownloadMbps.Valid, prefs.DownloadMbps.Float64)
	}
	if !prefs.PreferredAudioLanguage.Valid || prefs.PreferredAudioLanguage.String != "en" {
		t.Fatalf("expected preferred audio language en, got valid=%v %q", prefs.PreferredAudioLanguage.Valid, prefs.PreferredAudioLanguage.String)
	}
	if !prefs.PreferredSubtitleLanguage.Valid || prefs.PreferredSubtitleLanguage.String != "off" {
		t.Fatalf("expected preferred subtitle language off, got valid=%v %q", prefs.PreferredSubtitleLanguage.Valid, prefs.PreferredSubtitleLanguage.String)
	}

	settings, err := app.Queries.GetSettings(context.Background())
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if !settings.ServerUploadMbps.Valid || settings.ServerUploadMbps.Float64 != 12.5 {
		t.Fatalf("expected server_upload_mbps 12.5, got valid=%v %v", settings.ServerUploadMbps.Valid, settings.ServerUploadMbps.Float64)
	}
	if app.Settings == nil || !app.Settings.ServerUploadMbps.Valid || app.Settings.ServerUploadMbps.Float64 != 12.5 {
		t.Fatalf("expected app.Settings server_upload_mbps 12.5, got %+v", app.Settings)
	}
}

func TestUpdatePlaybackSettings_RegularUserCannotUpdateServerUploadCap(t *testing.T) {
	app := setupPlaybackHTTPTestApp(t)
	defer app.DB.Close()

	settings, err := app.Queries.UpdatePlaybackServerSettings(context.Background(), database.UpdatePlaybackServerSettingsParams{
		ServerUploadMbps:           helpers.NullFloat64(22),
		HardwareAccelerationDevice: helpers.NullString(helpers.HARDWARE_ACCELERATION_DEVICE_CPU),
	})
	if err != nil {
		t.Fatalf("seed server settings: %v", err)
	}
	app.Settings = &settings

	user := createTestUser(t, app, "Regular", "regular@example.com", false)
	handler := mountPlaybackRouter(app, user.ID)

	body := fmt.Sprintf(`{"preferred_profile": %q, "download_mbps": 50, "server_upload_mbps": 10}`, helpers.HLS_PROFILE_1080P_6MBPS)
	req := httptest.NewRequest(http.MethodPut, "/api/settings/playback", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}

	prefs, err := app.Queries.GetUserPlaybackPreferences(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("GetUserPlaybackPreferences: %v", err)
	}
	if prefs.PreferredHlsProfile.Valid {
		t.Fatalf("expected preferred_hls_profile unchanged, got %q", prefs.PreferredHlsProfile.String)
	}
	if prefs.DownloadMbps.Valid {
		t.Fatalf("expected download_mbps unchanged, got %v", prefs.DownloadMbps.Float64)
	}

	gotSettings, err := app.Queries.GetSettings(context.Background())
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if !gotSettings.ServerUploadMbps.Valid || gotSettings.ServerUploadMbps.Float64 != 22 {
		t.Fatalf("expected server_upload_mbps unchanged at 22, got valid=%v %v", gotSettings.ServerUploadMbps.Valid, gotSettings.ServerUploadMbps.Float64)
	}
}

func TestUpdatePlaybackSettings_ServerUploadMbpsBoundaries(t *testing.T) {
	cases := []struct {
		name       string
		value      string
		wantStatus int
		wantValid  bool
		wantStored float64
	}{
		{"null clears", "null", http.StatusOK, false, 0},
		{"zero rejected", "0", http.StatusBadRequest, true, 25},
		{"just above zero accepted", "0.1", http.StatusOK, true, 0.1},
		{"just below ceiling accepted", "99999.99", http.StatusOK, true, 99999.99},
		{"ceiling rejected", "100000", http.StatusBadRequest, true, 25},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := setupPlaybackHTTPTestApp(t)
			defer app.DB.Close()

			settings, err := app.Queries.UpdatePlaybackServerSettings(context.Background(), database.UpdatePlaybackServerSettingsParams{
				ServerUploadMbps:           helpers.NullFloat64(25),
				HardwareAccelerationDevice: helpers.NullString(helpers.HARDWARE_ACCELERATION_DEVICE_CPU),
			})
			if err != nil {
				t.Fatalf("seed server settings: %v", err)
			}
			app.Settings = &settings

			admin := createTestUser(t, app, "Admin", "admin@example.com", true)
			handler := mountPlaybackRouter(app, admin.ID)

			body := `{"server_upload_mbps": ` + tc.value + `}`
			req := httptest.NewRequest(http.MethodPut, "/api/settings/playback", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != tc.wantStatus {
				t.Fatalf("value=%s expected status %d, got %d: %s", tc.value, tc.wantStatus, w.Code, w.Body.String())
			}

			gotSettings, err := app.Queries.GetSettings(context.Background())
			if err != nil {
				t.Fatalf("GetSettings: %v", err)
			}
			if gotSettings.ServerUploadMbps.Valid != tc.wantValid {
				t.Fatalf("expected server_upload_mbps valid=%v, got %v", tc.wantValid, gotSettings.ServerUploadMbps.Valid)
			}
			if tc.wantValid && gotSettings.ServerUploadMbps.Float64 != tc.wantStored {
				t.Fatalf("expected stored server_upload_mbps %v, got %v", tc.wantStored, gotSettings.ServerUploadMbps.Float64)
			}
		})
	}
}

func TestUpdatePlaybackSettings_AdminCanUpdateHardwareDevice(t *testing.T) {
	app := setupPlaybackHTTPTestApp(t)
	defer app.DB.Close()

	admin := createTestUser(t, app, "Admin", "admin@example.com", true)
	handler := mountPlaybackRouter(app, admin.ID)

	body := `{"hardware_acceleration_device": "apple"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings/playback", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	settings, err := app.Queries.GetSettings(context.Background())
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if !settings.HardwareAccelerationDevice.Valid || settings.HardwareAccelerationDevice.String != helpers.HARDWARE_ACCELERATION_DEVICE_APPLE {
		t.Fatalf("expected stored hardware device apple, got valid=%v %q", settings.HardwareAccelerationDevice.Valid, settings.HardwareAccelerationDevice.String)
	}
	if app.Settings == nil || app.Settings.HardwareAccelerationDevice.String != helpers.HARDWARE_ACCELERATION_DEVICE_APPLE {
		t.Fatalf("expected app.Settings hardware device apple, got %+v", app.Settings)
	}
}

func TestUpdatePlaybackSettings_RegularUserCannotUpdateHardwareDevice(t *testing.T) {
	app := setupPlaybackHTTPTestApp(t)
	defer app.DB.Close()

	user := createTestUser(t, app, "Regular", "regular@example.com", false)
	handler := mountPlaybackRouter(app, user.ID)

	body := `{"hardware_acceleration_device": "nvidia"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings/playback", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}

	settings, err := app.Queries.GetSettings(context.Background())
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if settings.HardwareAccelerationDevice.Valid && settings.HardwareAccelerationDevice.String == helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA {
		t.Fatal("expected hardware device unchanged for regular user")
	}
}

func TestUpdatePlaybackSettings_RejectsInvalidHardwareDevice(t *testing.T) {
	app := setupPlaybackHTTPTestApp(t)
	defer app.DB.Close()

	admin := createTestUser(t, app, "Admin", "admin@example.com", true)
	handler := mountPlaybackRouter(app, admin.ID)

	for _, body := range []string{
		`{"hardware_acceleration_device": "unsupported"}`,
		`{"hardware_acceleration_device": null}`,
	} {
		req := httptest.NewRequest(http.MethodPut, "/api/settings/playback", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("body=%s expected 400, got %d: %s", body, w.Code, w.Body.String())
		}
	}
}

func TestUpdatePlaybackSettings_LanguagePrefsRoundTrip(t *testing.T) {
	app := setupPlaybackHTTPTestApp(t)
	defer app.DB.Close()

	user := createTestUser(t, app, "Regular", "regular@example.com", false)
	handler := mountPlaybackRouter(app, user.ID)

	body := `{"preferred_audio_language": "en", "preferred_subtitle_language": "es"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings/playback", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	prefs, err := app.Queries.GetUserPlaybackPreferences(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("GetUserPlaybackPreferences: %v", err)
	}
	if !prefs.PreferredAudioLanguage.Valid || prefs.PreferredAudioLanguage.String != "en" {
		t.Fatalf("expected preferred_audio_language en, got valid=%v %q", prefs.PreferredAudioLanguage.Valid, prefs.PreferredAudioLanguage.String)
	}
	if !prefs.PreferredSubtitleLanguage.Valid || prefs.PreferredSubtitleLanguage.String != "es" {
		t.Fatalf("expected preferred_subtitle_language es, got valid=%v %q", prefs.PreferredSubtitleLanguage.Valid, prefs.PreferredSubtitleLanguage.String)
	}
}

func TestUpdatePlaybackSettings_SubtitleOffAccepted(t *testing.T) {
	app := setupPlaybackHTTPTestApp(t)
	defer app.DB.Close()

	user := createTestUser(t, app, "Regular", "regular@example.com", false)
	handler := mountPlaybackRouter(app, user.ID)

	body := `{"preferred_subtitle_language": "off"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings/playback", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	prefs, err := app.Queries.GetUserPlaybackPreferences(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("GetUserPlaybackPreferences: %v", err)
	}
	if !prefs.PreferredSubtitleLanguage.Valid || prefs.PreferredSubtitleLanguage.String != "off" {
		t.Fatalf("expected preferred_subtitle_language off, got valid=%v %q", prefs.PreferredSubtitleLanguage.Valid, prefs.PreferredSubtitleLanguage.String)
	}
}

func TestUpdatePlaybackSettings_AudioOffRejected(t *testing.T) {
	app := setupPlaybackHTTPTestApp(t)
	defer app.DB.Close()

	user := createTestUser(t, app, "Regular", "regular@example.com", false)
	handler := mountPlaybackRouter(app, user.ID)

	body := `{"preferred_audio_language": "off"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings/playback", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for audio=off, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdatePlaybackSettings_RejectsMalformedLanguageCodes(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"audio english word", `{"preferred_audio_language": "english"}`},
		{"audio uppercase", `{"preferred_audio_language": "EN"}`},
		{"audio single letter", `{"preferred_audio_language": "e"}`},
		{"audio digits", `{"preferred_audio_language": "e1"}`},
		{"subtitle english word", `{"preferred_subtitle_language": "english"}`},
		{"subtitle uppercase", `{"preferred_subtitle_language": "EN"}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := setupPlaybackHTTPTestApp(t)
			defer app.DB.Close()

			user := createTestUser(t, app, "Regular", "regular@example.com", false)
			handler := mountPlaybackRouter(app, user.ID)

			req := httptest.NewRequest(http.MethodPut, "/api/settings/playback", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestUpdatePlaybackSettings_LanguageEmptyStringClears(t *testing.T) {
	app := setupPlaybackHTTPTestApp(t)
	defer app.DB.Close()

	user := createTestUser(t, app, "Regular", "regular@example.com", false)
	handler := mountPlaybackRouter(app, user.ID)

	seedBody := `{"preferred_audio_language": "en", "preferred_subtitle_language": "off"}`
	seedReq := httptest.NewRequest(http.MethodPut, "/api/settings/playback", strings.NewReader(seedBody))
	seedReq.Header.Set("Content-Type", "application/json")
	seedW := httptest.NewRecorder()
	handler.ServeHTTP(seedW, seedReq)
	if seedW.Code != http.StatusOK {
		t.Fatalf("seed PUT: expected 200, got %d: %s", seedW.Code, seedW.Body.String())
	}

	clearBody := `{"preferred_audio_language": "", "preferred_subtitle_language": ""}`
	clearReq := httptest.NewRequest(http.MethodPut, "/api/settings/playback", strings.NewReader(clearBody))
	clearReq.Header.Set("Content-Type", "application/json")
	clearW := httptest.NewRecorder()
	handler.ServeHTTP(clearW, clearReq)
	if clearW.Code != http.StatusOK {
		t.Fatalf("clear PUT: expected 200, got %d: %s", clearW.Code, clearW.Body.String())
	}

	prefs, err := app.Queries.GetUserPlaybackPreferences(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("GetUserPlaybackPreferences: %v", err)
	}
	if prefs.PreferredAudioLanguage.Valid {
		t.Fatalf("expected preferred_audio_language NULL after empty-string clear, got %q", prefs.PreferredAudioLanguage.String)
	}
	if prefs.PreferredSubtitleLanguage.Valid {
		t.Fatalf("expected preferred_subtitle_language NULL after empty-string clear, got %q", prefs.PreferredSubtitleLanguage.String)
	}
}
