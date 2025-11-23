package main

import (
	"errors"
	"igloo/cmd/internal/helpers"
	"net/http"
	"strings"
)

func (app *Application) LoadAndSaveSession(next http.Handler) http.Handler {
	return app.Session.LoadAndSave(next)
}

func (app *Application) IsAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !app.Session.Exists(r.Context(), COOKIE_USER_ID) {
			if strings.HasPrefix(r.URL.Path, "/api") {
				helpers.ErrorJSON(w, errors.New(NOT_AUTHORIZED), http.StatusUnauthorized)
			} else {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
			}

			return
		}

		next.ServeHTTP(w, r)
	})
}
