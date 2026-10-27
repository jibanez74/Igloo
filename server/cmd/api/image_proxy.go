package main

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"igloo/cmd/internal/helpers"
)

// proxyImage streams the image at baseURL/elem... to the client with its
// caching headers. label names the image in logs and in the 502 every upstream
// failure answers with.
func (app *Application) proxyImage(w http.ResponseWriter, r *http.Request, client *http.Client, label, baseURL string, elem ...string) {
	fail := func(logMessage string, attrs ...any) {
		app.Logger.Error(logMessage, attrs...)
		helpers.ErrorJSON(w, errors.New("failed to fetch "+label), http.StatusBadGateway)
	}

	imageURL, err := url.JoinPath(baseURL, elem...)
	if err != nil {
		fail("failed to build "+label+" proxy URL", "error", err, "base_url", baseURL)
		return
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, imageURL, nil)
	if err != nil {
		fail("failed to build "+label+" proxy request", "error", err, "url", imageURL)
		return
	}

	if client == nil {
		client = &http.Client{Timeout: helpers.PROVIDER_HTTP_TIMEOUT}
	}

	resp, err := client.Do(req)
	if err != nil {
		fail("failed to fetch "+label, "error", err, "url", imageURL)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fail(label+" upstream returned an error", "status", resp.StatusCode, "url", imageURL)
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
		app.Logger.Error("failed to stream "+label+" response", "error", err, "url", imageURL)
	}
}
