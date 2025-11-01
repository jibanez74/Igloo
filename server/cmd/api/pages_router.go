package main

import (
	"context"
	"errors"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"net/http"
	"path/filepath"
	"text/template"
)

var TEMPLATES_BASE_PATH = filepath.Join("cmd", "templates")

func (app *Application) RouteGetHomePage(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles(
		filepath.Join(TEMPLATES_BASE_PATH, "base.html"),
		filepath.Join(TEMPLATES_BASE_PATH, "home.html"),
	)

	if err != nil {
		helpers.ErrorJSON(w, errors.New("fail to parse templates"), http.StatusInternalServerError)
		return
	}

	albums, err := app.Queries.GetLatestAlbums(context.Background())
	if err != nil {
		helpers.ErrorJSON(w, errors.New("fail to get latest albums"), http.StatusInternalServerError)
		return
	}

	data := struct {
		Albums []database.GetLatestAlbumsRow
	}{
		Albums: albums,
	}

	tmpl.Execute(w, data)
}

func (app *Application) RouteGetLoginPage(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles(
		filepath.Join(TEMPLATES_BASE_PATH, "login.html"),
	)

	if err != nil {
		helpers.ErrorJSON(w, errors.New("fail to parse templates"), http.StatusInternalServerError)
		return
	}

	data := struct{}{}

	tmpl.Execute(w, data)
}
