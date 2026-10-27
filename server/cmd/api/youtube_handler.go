package main

import (
	"cmp"
	"errors"
	"igloo/cmd/internal/helpers"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

const (
	youtubeThumbnailBaseURL  = "https://i.ytimg.com/vi"
	youtubeThumbnailFileName = "hqdefault.jpg"
	youtubeVideoKeyMaxLength = 64
)

func (app *Application) ProxyYouTubeThumbnail(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	if !isSafeYouTubeVideoKey(key) {
		helpers.ErrorJSON(w, errors.New("invalid YouTube video key"), http.StatusBadRequest)
		return
	}

	baseURL := cmp.Or(strings.TrimRight(app.YouTubeThumbBaseURL, "/"), youtubeThumbnailBaseURL)
	app.proxyImage(w, r, app.YouTubeThumbHTTPClient, "YouTube thumbnail", baseURL, key, youtubeThumbnailFileName)
}

func isSafeYouTubeVideoKey(key string) bool {
	return len(key) <= youtubeVideoKeyMaxLength && helpers.IsPlainPathSegment(key, "")
}
