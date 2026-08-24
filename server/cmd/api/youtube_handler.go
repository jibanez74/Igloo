package main

import (
	"errors"
	"igloo/cmd/internal/helpers"
	"io"
	"net/http"
	"net/url"
	"strconv"
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

	baseURL := strings.TrimRight(app.YouTubeThumbBaseURL, "/")
	if baseURL == "" {
		baseURL = youtubeThumbnailBaseURL
	}

	thumbURL, err := url.JoinPath(baseURL, key, youtubeThumbnailFileName)
	if err != nil {
		app.Logger.Error("failed to build YouTube thumbnail proxy URL", "error", err, "base_url", baseURL)
		helpers.ErrorJSON(w, errors.New("failed to fetch YouTube thumbnail"), http.StatusBadGateway)
		return
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, thumbURL, nil)
	if err != nil {
		app.Logger.Error("failed to build YouTube thumbnail proxy request", "error", err, "url", thumbURL)
		helpers.ErrorJSON(w, errors.New("failed to fetch YouTube thumbnail"), http.StatusBadGateway)
		return
	}

	client := app.YouTubeThumbHTTPClient
	if client == nil {
		client = &http.Client{Timeout: helpers.TMDB_HTTP_TIMEOUT}
	}

	resp, err := client.Do(req)
	if err != nil {
		app.Logger.Error("failed to fetch YouTube thumbnail", "error", err, "url", thumbURL)
		helpers.ErrorJSON(w, errors.New("failed to fetch YouTube thumbnail"), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		app.Logger.Error("YouTube thumbnail upstream returned an error", "status", resp.StatusCode, "url", thumbURL)
		helpers.ErrorJSON(w, errors.New("failed to fetch YouTube thumbnail"), http.StatusBadGateway)
		return
	}

	for _, header := range []string{"Content-Type", "Cache-Control", "ETag", "Last-Modified"} {
		value := resp.Header.Get(header)
		if value != "" {
			w.Header().Set(header, value)
		}
	}
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	if resp.ContentLength >= 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(resp.ContentLength, 10))
	}
	w.WriteHeader(http.StatusOK)

	_, err = io.Copy(w, resp.Body)
	if err != nil {
		app.Logger.Error("failed to stream YouTube thumbnail response", "error", err, "url", thumbURL)
	}
}

func isSafeYouTubeVideoKey(key string) bool {
	if key == "" || len(key) > youtubeVideoKeyMaxLength {
		return false
	}

	for _, char := range key {
		if char >= 'a' && char <= 'z' {
			continue
		}
		if char >= 'A' && char <= 'Z' {
			continue
		}
		if char >= '0' && char <= '9' {
			continue
		}
		if char == '_' || char == '-' {
			continue
		}
		return false
	}

	return true
}
