package main

import (
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseLimitOffsetParams(t *testing.T) {
	tests := []struct {
		query                 string
		wantLimit, wantOffset int64
	}{
		{"", 50, 0},
		{"limit=20&offset=40", 20, 40},
		{"limit=500", 100, 0},
		{"limit=0&offset=-1", 50, 0},
		{"limit=abc&offset=abc", 50, 0},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			limit, offset := parseLimitOffsetParams(httptest.NewRequest(http.MethodGet, "/?"+tt.query, nil), 50, 100)
			if limit != tt.wantLimit || offset != tt.wantOffset {
				t.Fatalf("limit/offset = %d/%d, want %d/%d", limit, offset, tt.wantLimit, tt.wantOffset)
			}
		})
	}
}

// Handlers compute the offset as (page-1)*perPage, and SQLite reads a
// negative OFFSET as 0, so an overflowing page would serve page 1.
func TestParsePageAndPerPageKeepsTheOffsetInRange(t *testing.T) {
	for _, maxPerPage := range []int64{libraryMaxPerPage, 100} {
		request := httptest.NewRequest(http.MethodGet, "/?page=9223372036854775807&per_page=1000", nil)
		page, perPage := parsePageAndPerPage(request, libraryDefaultPerPage, maxPerPage)
		if perPage != maxPerPage {
			t.Fatalf("per_page = %d, want %d", perPage, maxPerPage)
		}
		if page != math.MaxInt64/perPage {
			t.Fatalf("page = %d, want %d", page, math.MaxInt64/perPage)
		}
		offset := (page - 1) * perPage
		if offset < 0 {
			t.Fatalf("offset = %d for per_page %d, want a nonnegative offset", offset, perPage)
		}
	}
}
