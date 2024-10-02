package main

import "net/http"

const redirect = "/"

func (app *config) SessionLoad(next http.Handler) http.Handler {
	return app.Session.LoadAndSave(next)
}

func (app *config) isAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !app.Session.Exists(r.Context(), "userID") {
			http.Redirect(w, r, redirect, http.StatusTemporaryRedirect)
			return
		}

		next.ServeHTTP(w, r)
	})
}
