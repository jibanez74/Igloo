package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDeviceNameValidationConformsToOpenAPI(t *testing.T) {
	document, _ := loadOpenAPIContract(t)
	endpoints := []struct {
		schema, operation, field, method, path string
		status                                 int
	}{
		{"DeviceLoginRequest", "authenticateDevice", "device_name", http.MethodPost, "/api/auth/device-login", http.StatusOK},
		{"QuickConnectInitiateRequest", "initiateQuickConnect", "device_name", http.MethodPost, "/api/quick-connect/initiate", http.StatusCreated},
		{"RenameDeviceRequest", "renameDevice", "name", http.MethodPatch, "/api/devices/", http.StatusOK},
	}
	names := []struct {
		label, name, stored string
		valid               bool
	}{
		{"empty", "", "", false},
		{"whitespace", " \t\u2003", "", false},
		{"100 bytes", strings.Repeat("é", 50), strings.Repeat("é", 50), true},
		{"102 bytes", strings.Repeat("é", 51), "", false},
		{"101 ASCII bytes", strings.Repeat("a", 101), "", false},
		{"trim before length", strings.Repeat(" ", 101) + "\u2003TV\t", "TV", true},
	}
	for _, endpoint := range endpoints {
		for _, tc := range names {
			t.Run(endpoint.schema+"/"+tc.label, func(t *testing.T) {
				app := setupSessionTestApp(t)
				user := createTestUser(t, app, "Device User", "device-name@example.com", false)
				path := endpoint.path
				var deviceID int64
				if endpoint.schema == "RenameDeviceRequest" {
					token := createTestDevice(t, app, user.ID, "Old name", "ios")
					device, err := app.Queries.GetDeviceByTokenHash(context.Background(), hashDeviceToken(token))
					if err != nil {
						t.Fatal(err)
					}
					deviceID = device.ID
					path += fmt.Sprint(device.ID)
				}
				app.InitRouter()
				body := map[string]any{endpoint.field: tc.name}
				if endpoint.schema == "DeviceLoginRequest" {
					body["email"] = user.Email
					body["password"] = testUserPassword
				}
				data, err := json.Marshal(body)
				if err != nil {
					t.Fatal(err)
				}
				schemaErr := document.Components.Schemas[endpoint.schema].Value.VisitJSON(body)
				if (schemaErr == nil) != (tc.name != "") {
					t.Fatalf("nonempty schema constraint: %v", schemaErr)
				}
				request := newOpenAPIJSONRequest(endpoint.method, path, string(data))
				request.AddCookie(newAuthSessionCookie(t, app, user.ID))
				response := httptest.NewRecorder()
				app.Router.ServeHTTP(response, request)
				wantStatus := http.StatusBadRequest
				if tc.valid {
					wantStatus = endpoint.status
				}
				if response.Code != wantStatus {
					t.Fatalf("status = %d, want %d: %s", response.Code, wantStatus, response.Body.String())
				}
				if tc.name == "" {
					assertOpenAPIResponse(t, endpoint.operation, request, response)
				} else {
					assertOpenAPIExchange(t, endpoint.operation, request, response)
				}
				if tc.valid && endpoint.schema == "RenameDeviceRequest" {
					devices, queryErr := app.Queries.GetDevicesByUser(context.Background(), user.ID)
					if queryErr != nil {
						t.Fatal(queryErr)
					}
					if len(devices) != 1 || devices[0].ID != deviceID || devices[0].Name != tc.stored {
						t.Fatalf("unexpected renamed devices: %+v", devices)
					}
				}
				if tc.valid && endpoint.schema == "DeviceLoginRequest" {
					var result struct {
						Data struct {
							Device struct {
								Name string `json:"name"`
							} `json:"device"`
						} `json:"data"`
					}
					err = json.Unmarshal(response.Body.Bytes(), &result)
					if err != nil {
						t.Fatal(err)
					}
					if result.Data.Device.Name != tc.stored {
						t.Fatalf("stored name = %q, want %q", result.Data.Device.Name, tc.stored)
					}
				}
			})
		}
	}
}

func TestEmptyPlaylistArraysRejectedByHandlerAndOpenAPI(t *testing.T) {
	app := setupSessionTestApp(t)
	fixtures := createPlaylistFixtures(t, app)
	handler := authenticatedRouter(t, app, fixtures.owner.ID)
	document, _ := loadOpenAPIContract(t)
	cases := []struct{ schema, operation, method, path, field string }{
		{"AddTracksRequest", "addTracksToPlaylist", http.MethodPost, fmt.Sprintf("/api/music/playlists/%d/tracks", fixtures.trackPlaylist.ID), "track_ids"},
		{"ReorderTracksRequest", "reorderPlaylistTracks", http.MethodPut, fmt.Sprintf("/api/music/playlists/%d/tracks/reorder", fixtures.trackPlaylist.ID), "track_ids"},
		{"AddMoviesRequest", "addMoviesToMoviePlaylist", http.MethodPost, fmt.Sprintf("/api/movies/playlists/%d/movies", fixtures.moviePlaylist.ID), "movie_ids"},
	}
	for _, tc := range cases {
		t.Run(tc.operation, func(t *testing.T) {
			body := map[string]any{tc.field: []any{}}
			schema := document.Components.Schemas[tc.schema].Value
			err := schema.VisitJSON(body)
			if err == nil {
				t.Fatal("schema accepted empty array")
			}
			body[tc.field] = []any{float64(1)}
			err = schema.VisitJSON(body)
			if err != nil {
				t.Fatalf("schema rejected nonempty array: %v", err)
			}
			request := newOpenAPIJSONRequest(tc.method, tc.path, fmt.Sprintf(`{"%s":[]}`, tc.field))
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d: %s", response.Code, response.Body.String())
			}
			assertOpenAPIResponse(t, tc.operation, request, response)
		})
	}
}

func TestTrimmedUserFieldsConformToOpenAPI(t *testing.T) {
	endpoints := []struct {
		schema, operation, method, path, field string
		limit, status                          int
	}{
		{"UpdateUserEmailRequest", "updateUserEmail", http.MethodPut, "/api/user/email", "email", userEmailMaxLength, http.StatusOK},
		{"AdminCreateUserRequest", "adminCreateUser", http.MethodPost, "/api/admin/users", "name", userNameMaxLength, http.StatusCreated},
		{"AdminCreateUserRequest", "adminCreateUser", http.MethodPost, "/api/admin/users", "email", userEmailMaxLength, http.StatusCreated},
		{"AdminUpdateUserRequest", "adminUpdateUser", http.MethodPatch, "/api/admin/users/", "name", userNameMaxLength, http.StatusOK},
		{"AdminUpdateUserRequest", "adminUpdateUser", http.MethodPatch, "/api/admin/users/", "email", userEmailMaxLength, http.StatusOK},
	}
	document, _ := loadOpenAPIContract(t)
	for _, endpoint := range endpoints {
		cases := []struct {
			name, value, stored string
			valid               bool
		}{
			{"trimmed Unicode boundary", "\u2003" + strings.Repeat("é", endpoint.limit) + "\t", strings.Repeat("é", endpoint.limit), true},
			{"long raw whitespace", strings.Repeat(" ", endpoint.limit+1) + "value\u2003", "value", true},
			{"overlong trimmed value", " " + strings.Repeat("é", endpoint.limit+1) + " ", "", false},
			{"whitespace only", " \t\u2003", "", false},
			{"empty", "", "", false},
		}
		for _, tc := range cases {
			t.Run(endpoint.operation+"/"+endpoint.field+"/"+tc.name, func(t *testing.T) {
				app := setupSessionTestApp(t)
				admin := createTestUser(t, app, "Admin", "admin@example.com", true)
				handler := authenticatedRouter(t, app, admin.ID)
				path := endpoint.path
				body := map[string]any{}
				if endpoint.operation != "updateUserEmail" {
					body["name"] = "New User"
					body["email"] = "new@example.com"
					body["is_admin"] = false
				}
				if endpoint.operation == "adminCreateUser" {
					body["password"] = testUserPassword
				}
				if endpoint.operation == "adminUpdateUser" {
					target := createTestUser(t, app, "Target", "target@example.com", false)
					path += fmt.Sprint(target.ID)
				}
				body[endpoint.field] = tc.value
				encoded, err := json.Marshal(body)
				if err != nil {
					t.Fatal(err)
				}
				request := newOpenAPIJSONRequest(endpoint.method, path, string(encoded))
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, request)
				wantStatus := http.StatusBadRequest
				if tc.valid {
					wantStatus = endpoint.status
				}
				if response.Code != wantStatus {
					t.Fatalf("status = %d, want %d: %s", response.Code, wantStatus, response.Body.String())
				}
				if tc.value == "" {
					err = document.Components.Schemas[endpoint.schema].Value.VisitJSON(body)
					if err == nil {
						t.Fatal("request schema accepted an empty required field")
					}
					assertOpenAPIResponse(t, endpoint.operation, request, response)
				} else {
					assertOpenAPIExchange(t, endpoint.operation, request, response)
				}
				if tc.valid {
					var result struct {
						Data struct {
							User map[string]any `json:"user"`
						} `json:"data"`
					}
					err = json.Unmarshal(response.Body.Bytes(), &result)
					if err != nil {
						t.Fatal(err)
					}
					if result.Data.User[endpoint.field] != tc.stored {
						t.Fatalf("stored %s = %q, want %q", endpoint.field, result.Data.User[endpoint.field], tc.stored)
					}
				}
			})
		}
	}
}
