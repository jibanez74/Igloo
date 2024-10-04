package main

import (
	"errors"
	"igloo/cmd/helpers"
	"net/http"
)

func (app *config) SessionLoad(next http.Handler) http.Handler {
	return app.Session.LoadAndSave(next)
}

func (app *config) RenewToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := app.Session.RenewToken(r.Context())
		if err != nil {
			helpers.ErrorJSON(w, errors.New("unable to refresh session token"))
			return
		}

		next.ServeHTTP(w, r)
	})
}

// func (app *config) isAuth(next http.Handler) http.Handler {
// 	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		if !app.Session.Exists(r.Context(), "user_id") {
// 			helpers.ErrorJSON(w, errors.New("not authorized"), http.StatusUnauthorized)
// 			return
// 		}

// 		next.ServeHTTP(w, r)
// 	})
// }
