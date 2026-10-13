package main

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"igloo/cmd/internal/helpers"

	"github.com/go-chi/chi/v5"
)

// parseIDParam parses the named URL parameter as an int64, answering 400 with
// message when it is not one.
func parseIDParam(w http.ResponseWriter, r *http.Request, name, message string) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, name), 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New(message), http.StatusBadRequest)
		return 0, false
	}
	return id, true
}

// positiveQueryInt reads a positive integer query parameter. Missing,
// unparseable and nonpositive values fall back rather than failing the request.
func positiveQueryInt(r *http.Request, name string, fallback int64) int64 {
	parsed, err := strconv.ParseInt(r.URL.Query().Get(name), 10, 64)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

// parsePageAndPerPage reads the page and per_page query parameters every
// paginated listing shares, clamping per_page to maxPerPage; docs/openapi.json
// documents that on PerPageQuery.
func parsePageAndPerPage(r *http.Request, defaultPerPage, maxPerPage int64) (page, perPage int64) {
	page = positiveQueryInt(r, "page", 1)
	perPage = min(positiveQueryInt(r, "per_page", defaultPerPage), maxPerPage)
	return page, perPage
}

func parseLibraryQuery(r *http.Request) (page, perPage int64, sort string) {
	page, perPage = parsePageAndPerPage(r, libraryDefaultPerPage, libraryMaxPerPage)

	sort = strings.ToLower(strings.TrimSpace(r.URL.Query().Get("sort")))
	if sort != "desc" {
		sort = "asc"
	}
	return page, perPage, sort
}

// parseLimitOffsetParams reads limit and offset. A missing, non-integer or
// nonpositive limit uses defaultLimit and a larger one is clamped to maxLimit;
// an invalid offset is 0.
func parseLimitOffsetParams(r *http.Request, defaultLimit, maxLimit int64) (int64, int64) {
	limit := min(positiveQueryInt(r, "limit", defaultLimit), maxLimit)

	offset, err := strconv.ParseInt(r.URL.Query().Get("offset"), 10, 64)
	if err != nil || offset < 0 {
		offset = 0
	}
	return limit, offset
}

func totalPages(total, perPage int64) int64 {
	pages := total / perPage
	if total%perPage > 0 {
		pages++
	}
	return pages
}
