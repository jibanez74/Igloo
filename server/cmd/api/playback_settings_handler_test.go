package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"

	"github.com/go-chi/chi/v5"
)

// mountPlaybackRouter mirrors registerSettingsRoutes: the GET is available to
// any authenticated user, the PUT is admin-gated by middleware.
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
		r.With(app.RequireAdmin).Put("/api/settings/playback", app.UpdatePlaybackSettings)
	})

	return app.SessionManager.LoadAndSave(r)
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

func seedServerPlaybackSettings(t *testing.T, app *Application, uploadMbps float64) {
	t.Helper()
	settings, err := app.Queries.UpdatePlaybackServerSettings(context.Background(), database.UpdatePlaybackServerSettingsParams{
		ServerUploadMbps:           helpers.NullFloat64(uploadMbps),
		HardwareAccelerationDevice: helpers.NullString(helpers.HARDWARE_ACCELERATION_DEVICE_CPU),
	})
	if err != nil {
		t.Fatalf("seed server settings: %v", err)
	}
	app.settings = &settings
}

func putPlayback(t *testing.T, handler http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPut, "/api/settings/playback", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w
}

func TestGetPlaybackSettings_ReturnsProfileCatalog(t *testing.T) {
	app := setupSettingsTestApp(t)
	defer app.DB.Close()

	user := createTestUser(t, app, "Regular", "regular@example.com", false)
	handler := mountPlaybackRouter(app, user.ID)

	req := httptest.NewRequest(http.MethodGet, "/api/settings/playback", nil)
	addOpenAPITestCookie(req)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	assertOpenAPIExchange(t, "getPlaybackSettings", req, w)

	settings := decodePlaybackResponse(t, w.Body.Bytes())
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

// The response carries only server-owned data: per-device preferences live in
// the client's local storage and must never reappear on this contract.
func TestGetPlaybackSettings_OmitsPerDevicePreferences(t *testing.T) {
	app := setupSettingsTestApp(t)
	defer app.DB.Close()

	user := createTestUser(t, app, "Regular", "regular@example.com", false)
	handler := mountPlaybackRouter(app, user.ID)

	req := httptest.NewRequest(http.MethodGet, "/api/settings/playback", nil)
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

	want := map[string]struct{}{
		"profiles":                     {},
		"server_upload_mbps":           {},
		"hardware_acceleration_device": {},
	}
	for k := range settings {
		_, ok := want[k]
		if !ok {
			t.Fatalf("unexpected key %q in playback settings; per-device preferences belong in local storage", k)
		}
	}
	for k := range want {
		_, ok := settings[k]
		if !ok {
			t.Fatalf("missing required key %q in playback settings", k)
		}
	}
}

func TestPlaybackSettings_RequiresAuth(t *testing.T) {
	app := setupSettingsTestApp(t)
	defer app.DB.Close()

	handler := mountPlaybackRouter(app, 0)

	req := httptest.NewRequest(http.MethodGet, "/api/settings/playback", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for GET, got %d: %s", w.Code, w.Body.String())
	}

	putW := putPlayback(t, handler, `{"server_upload_mbps": 50}`)
	if putW.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for PUT, got %d: %s", putW.Code, putW.Body.String())
	}
}

func TestPlaybackSettings_AdminServerSettingsRoundTrip(t *testing.T) {
	app := setupSettingsTestApp(t)
	defer app.DB.Close()

	admin := createTestUser(t, app, "Admin", "admin@example.com", true)
	handler := mountPlaybackRouter(app, admin.ID)

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

	w := putPlayback(t, handler, `{"server_upload_mbps": 50, "hardware_acceleration_device": "nvidia"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("put playback: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	settings := performGet(t)
	if settings.ServerUploadMbps == nil || *settings.ServerUploadMbps != 50 {
		t.Fatalf("expected server_upload_mbps 50 after admin set, got %+v", settings.ServerUploadMbps)
	}
	if settings.HardwareAccelerationDevice != helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA {
		t.Fatalf("expected hardware device nvidia after admin set, got %q", settings.HardwareAccelerationDevice)
	}

	w2 := putPlayback(t, handler, `{"server_upload_mbps": null}`)
	if w2.Code != http.StatusOK {
		t.Fatalf("put playback: expected 200, got %d: %s", w2.Code, w2.Body.String())
	}

	settings2 := performGet(t)
	if settings2.ServerUploadMbps != nil {
		t.Fatalf("expected server_upload_mbps nil after admin clear, got %+v", *settings2.ServerUploadMbps)
	}
	if settings2.HardwareAccelerationDevice != helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA {
		t.Fatalf("expected hardware device to stay nvidia when field omitted, got %q", settings2.HardwareAccelerationDevice)
	}
}

// The PUT echoes the same envelope the GET returns, so the client can seed its
// cache straight from the response.
func TestUpdatePlaybackSettings_ReturnsFullSettingsEnvelope(t *testing.T) {
	app := setupSettingsTestApp(t)
	defer app.DB.Close()

	admin := createTestUser(t, app, "Admin", "admin@example.com", true)
	handler := mountPlaybackRouter(app, admin.ID)

	req := newOpenAPIJSONRequest(http.MethodPut, "/api/settings/playback", `{"server_upload_mbps": 40, "hardware_acceleration_device": "intel"}`)
	addOpenAPITestCookie(req)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	assertOpenAPIExchange(t, "updatePlaybackSettings", req, w)

	settings := decodePlaybackResponse(t, w.Body.Bytes())
	if settings.ServerUploadMbps == nil || *settings.ServerUploadMbps != 40 {
		t.Fatalf("expected server_upload_mbps 40, got %+v", settings.ServerUploadMbps)
	}
	if settings.HardwareAccelerationDevice != helpers.HARDWARE_ACCELERATION_DEVICE_INTEL {
		t.Fatalf("expected hardware device intel, got %q", settings.HardwareAccelerationDevice)
	}
	if len(settings.Profiles) == 0 {
		t.Fatal("expected the PUT response to carry the profile catalog")
	}
}

func TestGetPlaybackSettings_ReportsServerUploadCap(t *testing.T) {
	app := setupSettingsTestApp(t)
	defer app.DB.Close()

	app.settings.ServerUploadMbps = sql.NullFloat64{Float64: 30, Valid: true}

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
	app := setupSettingsTestApp(t)
	defer app.DB.Close()

	admin := createTestUser(t, app, "Admin", "admin@example.com", true)
	handler := mountPlaybackRouter(app, admin.ID)

	w := putPlayback(t, handler, `{"server_upload_mbps": 12.5}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	settings, err := app.Queries.GetSettings(context.Background())
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if !settings.ServerUploadMbps.Valid || settings.ServerUploadMbps.Float64 != 12.5 {
		t.Fatalf("expected server_upload_mbps 12.5, got valid=%v %v", settings.ServerUploadMbps.Valid, settings.ServerUploadMbps.Float64)
	}
	if !app.settings.ServerUploadMbps.Valid || app.settings.ServerUploadMbps.Float64 != 12.5 {
		t.Fatalf("expected app.settings server_upload_mbps 12.5, got %+v", app.settings)
	}
}

func TestUpdatePlaybackSettings_RegularUserForbidden(t *testing.T) {
	app := setupSettingsTestApp(t)
	defer app.DB.Close()

	seedServerPlaybackSettings(t, app, 22)

	user := createTestUser(t, app, "Regular", "regular@example.com", false)
	handler := mountPlaybackRouter(app, user.ID)

	for _, body := range []string{
		`{"server_upload_mbps": 10}`,
		`{"hardware_acceleration_device": "nvidia"}`,
	} {
		w := putPlayback(t, handler, body)
		if w.Code != http.StatusForbidden {
			t.Fatalf("body=%s expected 403, got %d: %s", body, w.Code, w.Body.String())
		}
	}

	gotSettings, err := app.Queries.GetSettings(context.Background())
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if !gotSettings.ServerUploadMbps.Valid || gotSettings.ServerUploadMbps.Float64 != 22 {
		t.Fatalf("expected server_upload_mbps unchanged at 22, got valid=%v %v", gotSettings.ServerUploadMbps.Valid, gotSettings.ServerUploadMbps.Float64)
	}
	if gotSettings.HardwareAccelerationDevice.String == helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA {
		t.Fatal("expected hardware device unchanged for regular user")
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
			app := setupSettingsTestApp(t)
			defer app.DB.Close()

			seedServerPlaybackSettings(t, app, 25)

			admin := createTestUser(t, app, "Admin", "admin@example.com", true)
			handler := mountPlaybackRouter(app, admin.ID)

			w := putPlayback(t, handler, `{"server_upload_mbps": `+tc.value+`}`)
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
	app := setupSettingsTestApp(t)
	defer app.DB.Close()

	admin := createTestUser(t, app, "Admin", "admin@example.com", true)
	handler := mountPlaybackRouter(app, admin.ID)

	w := putPlayback(t, handler, `{"hardware_acceleration_device": "apple"}`)
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
	if app.settings.HardwareAccelerationDevice.String != helpers.HARDWARE_ACCELERATION_DEVICE_APPLE {
		t.Fatalf("expected app.settings hardware device apple, got %+v", app.settings)
	}
}

func TestUpdatePlaybackSettings_RejectsInvalidHardwareDevice(t *testing.T) {
	app := setupSettingsTestApp(t)
	defer app.DB.Close()

	admin := createTestUser(t, app, "Admin", "admin@example.com", true)
	handler := mountPlaybackRouter(app, admin.ID)

	for _, body := range []string{
		`{"hardware_acceleration_device": "unsupported"}`,
		`{"hardware_acceleration_device": null}`,
	} {
		w := putPlayback(t, handler, body)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("body=%s expected 400, got %d: %s", body, w.Code, w.Body.String())
		}
	}
}

// Two admins each send a partial PUT for a different field. The query writes
// both columns, so without serializing the read-modify-write one request would
// restore the other's column to the value it read before either wrote.
func TestUpdatePlaybackSettings_ConcurrentPartialUpdatesBothLand(t *testing.T) {
	app := setupSettingsTestApp(t)
	defer app.DB.Close()

	admin := createTestUser(t, app, "Admin", "admin@example.com", true)
	handler := mountPlaybackRouter(app, admin.ID)
	seedServerPlaybackSettings(t, app, 25)

	bodies := []string{
		`{"server_upload_mbps": 42.5}`,
		`{"hardware_acceleration_device": "nvidia"}`,
	}

	var wg sync.WaitGroup
	codes := make([]int, len(bodies))
	start := make(chan struct{})
	for i, body := range bodies {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			req := httptest.NewRequest(http.MethodPut, "/api/settings/playback", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			codes[i] = w.Code
		}()
	}
	close(start)
	wg.Wait()

	for i, code := range codes {
		if code != http.StatusOK {
			t.Fatalf("request %d expected 200, got %d", i, code)
		}
	}

	settings, err := app.Queries.GetSettings(context.Background())
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if !settings.ServerUploadMbps.Valid || settings.ServerUploadMbps.Float64 != 42.5 {
		t.Errorf("expected server upload 42.5 to survive, got valid=%v %v", settings.ServerUploadMbps.Valid, settings.ServerUploadMbps.Float64)
	}
	if settings.HardwareAccelerationDevice.String != helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA {
		t.Errorf("expected hardware device nvidia to survive, got %q", settings.HardwareAccelerationDevice.String)
	}
}
