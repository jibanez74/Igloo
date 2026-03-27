package main

import (
	"errors"
	"igloo/cmd/internal/helpers"
	"net/http"
	"strings"
)

// simple middleware to make the scs session work correctly
func (app *Application) LoadAndSaveSession(next http.Handler) http.Handler {
	return app.SessionManager.LoadAndSave(next)
}

// a simple middleware to determine if the user is authenticated
func (app *Application) IsAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !app.SessionManager.Exists(r.Context(), helpers.COOKIE_USER_ID) {
			if strings.HasPrefix(r.URL.Path, "/api") {
				helpers.ErrorJSON(w, errors.New(helpers.NOT_AUTHORIZED_MESSAGE), http.StatusUnauthorized)
			} else {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
			}

			return
		}

		next.ServeHTTP(w, r)
	})
}

// requireAdmin checks that the current session belongs to an admin user.
// On failure it writes the appropriate HTTP error and returns 0.
// On success it returns the admin's user ID.
func (app *Application) requireAdmin(w http.ResponseWriter, r *http.Request) int64 {
	userID := app.SessionManager.GetInt64(r.Context(), helpers.COOKIE_USER_ID)
	if userID == 0 {
		helpers.ErrorJSON(w, errors.New(helpers.NOT_AUTHORIZED_MESSAGE), http.StatusUnauthorized)
		return 0
	}

	user, err := app.Queries.GetUser(r.Context(), userID)
	if err != nil {
		helpers.ErrorJSON(w, errors.New(helpers.NOT_AUTHORIZED_MESSAGE), http.StatusUnauthorized)
		return 0
	}

	if !user.IsAdmin {
		helpers.ErrorJSON(w, errors.New("admin access required"), http.StatusForbidden)
		return 0
	}

	return userID
}
