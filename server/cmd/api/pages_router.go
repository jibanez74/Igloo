package main

import (
	"errors"
	"igloo/cmd/internal/helpers"
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

	data := struct {
		Name string
	}{
		Name: "User",
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
