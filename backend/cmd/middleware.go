package main

import (
	"errors"
	"igloo/cmd/helpers"
	"net/http"
)

func (app *config) sessionLoad(next http.Handler) http.Handler {
	return app.session.LoadAndSave(next)
}

func (app *config) reloadSessionToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if app.session.Lifetime > 0 {
			err := app.session.RenewToken(r.Context())
			if err != nil {
				helpers.ErrorJSON(w, err, http.StatusInternalServerError)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

func (app *config) isAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !app.session.Exists(r.Context(), "user_id") {
			helpers.ErrorJSON(w, errors.New("not authorized"), http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}
