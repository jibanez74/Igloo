package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"

	"igloo/cmd/internal/helpers"
)

func TestPasswordLimitsConformToOpenAPI(t *testing.T) {
	passwords := []struct {
		name, password, errorSuffix string
	}{
		{"72 ASCII bytes", strings.Repeat("a", 72), ""},
		{"72 multibyte bytes", strings.Repeat("é", 36), ""},
		{"72 emoji bytes", strings.Repeat("🔒", 18), ""},
		{"73 ASCII bytes", strings.Repeat("a", 73), " must be at most 72 UTF-8 bytes"},
		{"73 multibyte bytes", strings.Repeat("é", 36) + "a", " must be at most 72 UTF-8 bytes"},
		{"74 multibyte bytes", strings.Repeat("é", 37), " must be at most 72 UTF-8 bytes"},
		{"76 emoji bytes", strings.Repeat("🔒", 19), " must be at most 72 UTF-8 bytes"},
		{"eight ASCII characters", "abcdefgh", " must be at least 9 characters"},
		{"nine ASCII characters", "abcdefghi", ""},
		{"eight emoji characters", strings.Repeat("🔒", 8), " must be at least 9 characters"},
		{"nine emoji characters", strings.Repeat("🔒", 9), ""},
		{"literal whitespace and combining characters", " ée\u0301🔒abcd ", ""},
	}
	for _, operation := range []string{"adminCreateUser", "adminResetUserPassword", "updateUserPassword"} {
		// One application per operation; every case gets its own user because a
		// successful password change would otherwise invalidate the next login.
		app := setupSessionTestApp(t)
		app.InitRouter()
		for i, tc := range passwords {
			t.Run(operation+"/"+tc.name, func(t *testing.T) {
				// Device logins are rate limited per client IP, which every case
				// here shares.
				app.AuthLimiter = newRateLimiter()
				user := createTestUser(t, app, "Password User", fmt.Sprintf("password-limit-%d@example.com", i), true)
				cookie := newAuthSessionCookie(t, app, user.ID)
				method, path := http.MethodPut, fmt.Sprintf("/api/admin/users/%d/password", user.ID)
				body := fmt.Sprintf(`{"password":%q}`, tc.password)
				label, email := "password", user.Email
				wantStatus := http.StatusOK
				switch operation {
				case "adminCreateUser":
					method, path = http.MethodPost, "/api/admin/users"
					email = fmt.Sprintf("new-limit-%d@example.com", i)
					body = fmt.Sprintf(`{"name":"New User","email":%q,"password":%q,"is_admin":false}`, email, tc.password)
					wantStatus = http.StatusCreated
				case "updateUserPassword":
					path, label = "/api/user/password", "new password"
					body = fmt.Sprintf(`{"current_password":%q,"new_password":%q}`, testUserPassword, tc.password)
				}
				invalid := tc.errorSuffix != ""
				if invalid {
					wantStatus = http.StatusBadRequest
				}
				request := newOpenAPIJSONRequest(method, path, body)
				request.AddCookie(cookie)
				response := httptest.NewRecorder()
				app.Router.ServeHTTP(response, request)
				if response.Code != wantStatus {
					t.Fatalf("status = %d, want %d: %s", response.Code, wantStatus, response.Body.String())
				}
				characters := utf8.RuneCountInString(tc.password)
				invalidSchema := characters < 9 || characters > 72
				if invalidSchema {
					assertOpenAPIResponse(t, operation, request, response)
				} else {
					assertOpenAPIExchange(t, operation, request, response)
				}
				stored, err := app.Queries.GetUserByEmail(context.Background(), email)
				if invalid {
					var envelope helpers.JSONResponse
					decodeErr := json.Unmarshal(response.Body.Bytes(), &envelope)
					if decodeErr != nil {
						t.Fatal(decodeErr)
					}
					if !envelope.Error || envelope.Message != label+tc.errorSuffix {
						t.Fatalf("unexpected error envelope: %s", response.Body.String())
					}
					if operation == "adminCreateUser" {
						missing := errors.Is(err, sql.ErrNoRows)
						if !missing {
							t.Fatalf("rejected creation inserted a user: %v", err)
						}
					} else {
						if err != nil {
							t.Fatal(err)
						}
						if stored.Password != user.Password {
							t.Fatal("rejected password update changed the stored hash")
						}
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				match, err := helpers.PasswordMatches(tc.password, stored.Password)
				if err != nil || !match {
					t.Fatalf("stored password is not usable: match=%v err=%v", match, err)
				}
				for _, login := range []struct{ operation, path, extra string }{
					{"authenticateUser", "/api/auth/login", ""},
					{"authenticateDevice", "/api/auth/device-login", `,"device_name":"Test device"`},
				} {
					for _, suffix := range []string{"", "suffix"} {
						loginRequest := newOpenAPIJSONRequest(http.MethodPost, login.path,
							fmt.Sprintf(`{"email":%q,"password":%q%s}`, email, tc.password+suffix, login.extra))
						loginResponse := httptest.NewRecorder()
						app.Router.ServeHTTP(loginResponse, loginRequest)
						status := http.StatusOK
						if suffix != "" {
							status = http.StatusUnauthorized
						}
						if loginResponse.Code != status {
							t.Fatalf("%s with suffix %q: status = %d: %s", login.operation, suffix, loginResponse.Code, loginResponse.Body.String())
						}
						assertOpenAPIExchange(t, login.operation, loginRequest, loginResponse)
					}
				}
				changeRequest := newOpenAPIJSONRequest(http.MethodPut, "/api/user/password",
					fmt.Sprintf(`{"current_password":%q,"new_password":"replacement password"}`, tc.password+"suffix"))
				changeRequest.AddCookie(newAuthSessionCookie(t, app, stored.ID))
				changeResponse := httptest.NewRecorder()
				app.Router.ServeHTTP(changeResponse, changeRequest)
				if changeResponse.Code != http.StatusUnauthorized {
					t.Fatalf("suffixed current password status = %d: %s", changeResponse.Code, changeResponse.Body.String())
				}
				assertOpenAPIExchange(t, "updateUserPassword", changeRequest, changeResponse)
				after, err := app.Queries.GetUserByEmail(context.Background(), email)
				if err != nil {
					t.Fatal(err)
				}
				if after.Password != stored.Password {
					t.Fatal("incorrect current password changed the stored hash")
				}
			})
		}
	}
}
