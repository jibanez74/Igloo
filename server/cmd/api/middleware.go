package main

import (
	"errors"
	"igloo/cmd/internal/helpers"
	"net/http"
)

// LoadAndSaveSession wraps the scs session middleware.
func (app *Application) LoadAndSaveSession(next http.Handler) http.Handler {
	return app.SessionManager.LoadAndSave(next)
}

// IsAuth rejects unauthenticated requests with 401.
func (app *Application) IsAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !app.SessionManager.Exists(r.Context(), helpers.COOKIE_USER_ID) {
			helpers.ErrorJSON(w, errors.New(helpers.NOT_AUTHORIZED_MESSAGE), http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// RequireAdmin rejects requests from non-admin users with 403.
func (app *Application) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := app.SessionManager.GetInt64(r.Context(), helpers.COOKIE_USER_ID)
		if userID == 0 {
			helpers.ErrorJSON(w, errors.New(helpers.NOT_AUTHORIZED_MESSAGE), http.StatusUnauthorized)
			return
		}

		user, err := app.Queries.GetUser(r.Context(), userID)
		if err != nil {
			helpers.ErrorJSON(w, errors.New(helpers.NOT_AUTHORIZED_MESSAGE), http.StatusUnauthorized)
			return
		}

		if !user.IsAdmin {
			helpers.ErrorJSON(w, errors.New("admin access required"), http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}
