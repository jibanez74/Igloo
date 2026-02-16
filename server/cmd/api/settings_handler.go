package main

import (
	"errors"
	"igloo/cmd/internal/helpers"
	"net/http"
	"sync"
)

var (
	// scanMutex prevents multiple simultaneous music scans
	scanMutex  sync.Mutex
	isScanning bool

	// movieScanMutex prevents multiple simultaneous movie scans
	movieScanMutex  sync.Mutex
	isMovieScanning bool
)

// GetSettings returns the application settings including library paths
func (app *Application) GetSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	settings, err := app.Queries.GetSettings(ctx)
	if err != nil {
		app.Logger.Error("failed to get settings", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to fetch settings"))
		return
	}

	// Build response with library paths
	// Only include paths that are configured (Valid = true)
	responseData := map[string]any{
		"music_dir":  nil,
		"movies_dir": nil,
		"shows_dir":  nil,
	}

	if settings.MusicDir.Valid {
		responseData["music_dir"] = settings.MusicDir.String
	}

	if settings.MoviesDir.Valid {
		responseData["movies_dir"] = settings.MoviesDir.String
	}

	if settings.ShowsDir.Valid {
		responseData["shows_dir"] = settings.ShowsDir.String
	}

	res := helpers.JSONResponse{
		Error: false,
		Data:  responseData,
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

// TriggerMusicScan triggers a new music library scan
// The scan runs asynchronously in a goroutine and returns immediately
func (app *Application) TriggerMusicScan(w http.ResponseWriter, r *http.Request) {
	scanMutex.Lock()
	if isScanning {
		scanMutex.Unlock()
		helpers.ErrorJSON(w, errors.New("music library scan is already in progress"))
		return
	}

	isScanning = true
	scanMutex.Unlock()

	// Check if music directory is configured
	if !app.Settings.MusicDir.Valid || app.Settings.MusicDir.String == "" {
		scanMutex.Lock()
		isScanning = false
		scanMutex.Unlock()
		helpers.ErrorJSON(w, errors.New("music directory is not configured"))
		return
	}

	// Start scan in background goroutine
	go func() {
		defer func() {
			scanMutex.Lock()
			isScanning = false
			scanMutex.Unlock()
		}()

		app.ScanMusicLibrary()
	}()

	app.Logger.Info("music library scan triggered via API", "path", app.Settings.MusicDir.String)

	res := helpers.JSONResponse{
		Error:   false,
		Message: "Music library scan started",
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

// TriggerMovieScan triggers a new movie library scan
// The scan runs asynchronously in a goroutine and returns immediately
func (app *Application) TriggerMovieScan(w http.ResponseWriter, r *http.Request) {
	movieScanMutex.Lock()
	if isMovieScanning {
		movieScanMutex.Unlock()
		helpers.ErrorJSON(w, errors.New("movie library scan is already in progress"))
		return
	}

	isMovieScanning = true
	movieScanMutex.Unlock()

	// Check if movies directory is configured
	if !app.Settings.MoviesDir.Valid || app.Settings.MoviesDir.String == "" {
		movieScanMutex.Lock()
		isMovieScanning = false
		movieScanMutex.Unlock()
		helpers.ErrorJSON(w, errors.New("movies directory is not configured"))
		return
	}

	// Start scan in background goroutine
	go func() {
		defer func() {
			movieScanMutex.Lock()
			isMovieScanning = false
			movieScanMutex.Unlock()
		}()

		app.ScanMoviesLibrary()
	}()

	app.Logger.Info("movie library scan triggered via API", "path", app.Settings.MoviesDir.String)

	res := helpers.JSONResponse{
		Error:   false,
		Message: "Movie library scan started",
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}
