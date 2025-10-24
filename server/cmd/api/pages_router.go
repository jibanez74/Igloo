package main

import (
	"context"
	"errors"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/tmdb"
	"net/http"
	"path/filepath"
	"text/template"
)

var TEMPLATES_BASE_PATH = filepath.Join("cmd", "templates")

func (app *Application) RouteGetIndexPage(w http.ResponseWriter, r *http.Request) {
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

	var moviesInTheaters []*tmdb.TmdbMovie

	if app.Tmdb != nil {
		movies, err := app.Tmdb.GetMoviesInTheaters()
		if err != nil {
			// Log error but don't fail the page - movies are optional
			// You might want to add logging here
		} else {
			moviesInTheaters = movies
		}
	}

	data := struct {
		Name             string
		Albums           []database.GetLatestAlbumsRow
		MoviesInTheaters []*tmdb.TmdbMovie
	}{
		Name:             "User",
		Albums:           albums,
		MoviesInTheaters: moviesInTheaters,
	}

	tmpl.Execute(w, data)
}

func (app *Application) RouteGetLoginPage(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles(
		filepath.Join(TEMPLATES_BASE_PATH, "base.html"),
		filepath.Join(TEMPLATES_BASE_PATH, "login.html"),
	)

	if err != nil {
		helpers.ErrorJSON(w, errors.New("fail to parse templates"), http.StatusInternalServerError)
		return
	}

	data := struct{}{}

	tmpl.Execute(w, data)
}
