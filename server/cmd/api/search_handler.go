package main

import (
	"context"
	"database/sql"
	"errors"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"net/http"
	"strconv"
	"strings"
	"unicode"
)

type movieSearchSection struct {
	Results []database.GetMoviesLibraryAscRow `json:"results"`
	Total   int64                             `json:"total"`
}

type albumSearchSection struct {
	Results []database.GetAlbumsAlphabeticalRow `json:"results"`
	Total   int64                               `json:"total"`
}

type musicianSearchSection struct {
	Results []database.GetMusiciansAlphabeticalRow `json:"results"`
	Total   int64                                  `json:"total"`
}

type trackSearchSection struct {
	Results []database.GetTracksAlphabeticalRow `json:"results"`
	Total   int64                               `json:"total"`
}

type searchAllData struct {
	Query     string                `json:"query"`
	Movies    movieSearchSection    `json:"movies"`
	Albums    albumSearchSection    `json:"albums"`
	Musicians musicianSearchSection `json:"musicians"`
	Tracks    trackSearchSection    `json:"tracks"`
}

type searchMoviesData struct {
	Query      string                            `json:"query"`
	Results    []database.GetMoviesLibraryAscRow `json:"results"`
	Total      int64                             `json:"total"`
	Page       int64                             `json:"page"`
	PerPage    int64                             `json:"per_page"`
	TotalPages int64                             `json:"total_pages"`
}

type searchAlbumsData struct {
	Query      string                              `json:"query"`
	Results    []database.GetAlbumsAlphabeticalRow `json:"results"`
	Total      int64                               `json:"total"`
	Page       int64                               `json:"page"`
	PerPage    int64                               `json:"per_page"`
	TotalPages int64                               `json:"total_pages"`
}

type searchMusiciansData struct {
	Query      string                                 `json:"query"`
	Results    []database.GetMusiciansAlphabeticalRow `json:"results"`
	Total      int64                                  `json:"total"`
	Page       int64                                  `json:"page"`
	PerPage    int64                                  `json:"per_page"`
	TotalPages int64                                  `json:"total_pages"`
}

type searchTracksData struct {
	Query      string                              `json:"query"`
	Results    []database.GetTracksAlphabeticalRow `json:"results"`
	Total      int64                               `json:"total"`
	Page       int64                               `json:"page"`
	PerPage    int64                               `json:"per_page"`
	TotalPages int64                               `json:"total_pages"`
}

// sqlc cannot parse FTS5 MATCH expressions, so these queries are hand-written
// and executed directly against app.DB. Result columns are scanned into the
// existing sqlc-generated row types so the JSON shape matches the regular
// list endpoints and the frontend cards work unchanged.

const searchMoviesSQL = `
SELECT m.id, m.title, m.poster_path, m.year, m.certification
FROM movies_fts
INNER JOIN movies AS m ON m.id = movies_fts.rowid
WHERE movies_fts MATCH ?
ORDER BY
  CASE
    WHEN LOWER(m.title) = LOWER(?) THEN 0
    WHEN LOWER(m.title) LIKE ? ESCAPE '\' THEN 1
    ELSE 2
  END,
  bm25(movies_fts),
  m.title
LIMIT ? OFFSET ?`

const searchMoviesCountSQL = `
SELECT COUNT(*) FROM movies_fts WHERE movies_fts MATCH ?`

const searchAlbumsSQL = `
SELECT a.id, a.title, a.cover, a.musician, a.year
FROM albums_fts
INNER JOIN albums AS a ON a.id = albums_fts.rowid
WHERE albums_fts MATCH ?
ORDER BY
  CASE
    WHEN LOWER(a.title) = LOWER(?) THEN 0
    WHEN LOWER(a.title) LIKE ? ESCAPE '\' THEN 1
    WHEN LOWER(COALESCE(a.musician, '')) = LOWER(?) THEN 2
    WHEN LOWER(COALESCE(a.musician, '')) LIKE ? ESCAPE '\' THEN 3
    ELSE 4
  END,
  bm25(albums_fts),
  a.title
LIMIT ? OFFSET ?`

const searchAlbumsCountSQL = `
SELECT COUNT(*) FROM albums_fts WHERE albums_fts MATCH ?`

const searchMusiciansSQL = `
SELECT
  m.id,
  m.name,
  m.thumb,
  m.sort_name,
  (SELECT COUNT(*) FROM musician_albums AS ma WHERE ma.musician_id = m.id) AS album_count,
  (SELECT COUNT(*) FROM tracks AS t WHERE t.musician_id = m.id) AS track_count
FROM musicians_fts
INNER JOIN musicians AS m ON m.id = musicians_fts.rowid
WHERE musicians_fts MATCH ?
ORDER BY
  CASE
    WHEN LOWER(m.name) = LOWER(?) OR LOWER(m.sort_name) = LOWER(?) THEN 0
    WHEN LOWER(m.name) LIKE ? ESCAPE '\' OR LOWER(m.sort_name) LIKE ? ESCAPE '\' THEN 1
    ELSE 2
  END,
  bm25(musicians_fts),
  m.sort_name
LIMIT ? OFFSET ?`

const searchMusiciansCountSQL = `
SELECT COUNT(*) FROM musicians_fts WHERE musicians_fts MATCH ?`

const searchTracksCountSQL = `
SELECT COUNT(*) FROM tracks_search_fts WHERE tracks_search_fts MATCH ?`

const searchTracksJoinedSQL = `
SELECT
  t.id, t.title, t.duration, t.codec, t.bit_rate, t.file_path,
  a.id AS album_id, a.title AS album_title, a.cover AS album_cover,
  mu.id AS musician_id, mu.name AS musician_name
FROM tracks_search_fts
INNER JOIN tracks AS t ON t.id = tracks_search_fts.rowid
LEFT JOIN albums AS a ON t.album_id = a.id
LEFT JOIN musicians AS mu ON t.musician_id = mu.id
WHERE tracks_search_fts MATCH ?
ORDER BY
  CASE
    WHEN LOWER(t.title) = LOWER(?) THEN 0
    WHEN LOWER(t.title) LIKE ? ESCAPE '\' THEN 1
    WHEN LOWER(COALESCE(a.title, '')) = LOWER(?) OR LOWER(COALESCE(mu.name, '')) = LOWER(?) OR LOWER(COALESCE(a.musician, '')) = LOWER(?) THEN 2
    WHEN LOWER(COALESCE(a.title, '')) LIKE ? ESCAPE '\' OR LOWER(COALESCE(mu.name, '')) LIKE ? ESCAPE '\' OR LOWER(COALESCE(a.musician, '')) LIKE ? ESCAPE '\' THEN 3
    ELSE 4
  END,
  bm25(tracks_search_fts),
  t.title
LIMIT ? OFFSET ?`

// buildFTSQuery converts a user-supplied query into a safe FTS5 MATCH expression.
// It strips characters that are not letters, digits, or whitespace so the user
// can't break out of the expression with FTS5 special syntax (quotes,
// parentheses, AND/OR/NEAR, column filters), then appends '*' to each token for
// broad prefix matching.
//
// Returns ok=false when the sanitized input has no usable tokens.
func buildFTSQuery(raw string) (string, bool) {
	lowered := strings.ToLower(strings.TrimSpace(raw))
	if lowered == "" {
		return "", false
	}

	var b strings.Builder
	b.Grow(len(lowered))
	for _, r := range lowered {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) {
			b.WriteRune(r)
			continue
		}
		b.WriteRune(' ')
	}

	tokens := strings.Fields(b.String())
	if len(tokens) == 0 {
		return "", false
	}

	parts := make([]string, len(tokens))
	for i, t := range tokens {
		parts[i] = t + "*"
	}
	return strings.Join(parts, " OR "), true
}

func escapeLikePattern(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r == '\\' || r == '%' || r == '_' {
			b.WriteRune('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

func searchRankArgs(raw string) (exact, prefix string) {
	exact = strings.TrimSpace(raw)
	prefix = escapeLikePattern(strings.ToLower(exact)) + "%"
	return exact, prefix
}

func parseSearchPagination(r *http.Request) (page, perPage int64) {
	page = 1
	if p := r.URL.Query().Get("page"); p != "" {
		parsed, err := strconv.ParseInt(p, 10, 64)
		if err == nil && parsed > 0 {
			page = parsed
		}
	}

	perPage = int64(helpers.SEARCH_DEFAULT_PER_PAGE)
	if pp := r.URL.Query().Get("per_page"); pp != "" {
		parsed, err := strconv.ParseInt(pp, 10, 64)
		if err == nil && parsed > 0 {
			perPage = parsed
		}
	}
	if perPage > helpers.SEARCH_MAX_PER_PAGE {
		perPage = helpers.SEARCH_MAX_PER_PAGE
	}
	return page, perPage
}

func totalPages(total, perPage int64) int64 {
	pages := total / perPage
	if total%perPage > 0 {
		pages++
	}
	return pages
}

func normalizeSearchPage(page, total, perPage int64) (int64, int64) {
	pages := totalPages(total, perPage)
	if pages == 0 {
		return 1, pages
	}
	if page > pages {
		return pages, pages
	}
	return page, pages
}

func (app *Application) searchMovies(ctx context.Context, raw, match string, limit, offset int64) ([]database.GetMoviesLibraryAscRow, error) {
	exact, prefix := searchRankArgs(raw)
	rows, err := app.DB.QueryContext(ctx, searchMoviesSQL, match, exact, prefix, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []database.GetMoviesLibraryAscRow{}
	for rows.Next() {
		var row database.GetMoviesLibraryAscRow
		err = rows.Scan(&row.ID, &row.Title, &row.PosterPath, &row.Year, &row.Certification)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	err = rows.Err()
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (app *Application) searchAlbums(ctx context.Context, raw, match string, limit, offset int64) ([]database.GetAlbumsAlphabeticalRow, error) {
	exact, prefix := searchRankArgs(raw)
	rows, err := app.DB.QueryContext(ctx, searchAlbumsSQL, match, exact, prefix, exact, prefix, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []database.GetAlbumsAlphabeticalRow{}
	for rows.Next() {
		var row database.GetAlbumsAlphabeticalRow
		err = rows.Scan(&row.ID, &row.Title, &row.Cover, &row.Musician, &row.Year)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	err = rows.Err()
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (app *Application) searchMusicians(ctx context.Context, raw, match string, limit, offset int64) ([]database.GetMusiciansAlphabeticalRow, error) {
	exact, prefix := searchRankArgs(raw)
	rows, err := app.DB.QueryContext(ctx, searchMusiciansSQL, match, exact, exact, prefix, prefix, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []database.GetMusiciansAlphabeticalRow{}
	for rows.Next() {
		var row database.GetMusiciansAlphabeticalRow
		err = rows.Scan(&row.ID, &row.Name, &row.Thumb, &row.SortName, &row.AlbumCount, &row.TrackCount)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	err = rows.Err()
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (app *Application) searchTracks(ctx context.Context, raw, match string, limit, offset int64) ([]database.GetTracksAlphabeticalRow, error) {
	exact, prefix := searchRankArgs(raw)
	rows, err := app.DB.QueryContext(ctx, searchTracksJoinedSQL, match, exact, prefix, exact, exact, exact, prefix, prefix, prefix, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []database.GetTracksAlphabeticalRow{}
	for rows.Next() {
		var row database.GetTracksAlphabeticalRow
		err = rows.Scan(
			&row.ID, &row.Title, &row.Duration, &row.Codec, &row.BitRate, &row.FilePath,
			&row.AlbumID, &row.AlbumTitle, &row.AlbumCover,
			&row.MusicianID, &row.MusicianName,
		)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	err = rows.Err()
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (app *Application) searchCount(ctx context.Context, query, match string) (int64, error) {
	var total int64
	err := app.DB.QueryRowContext(ctx, query, match).Scan(&total)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	return total, nil
}

func (app *Application) SearchAll(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	match, ok := buildFTSQuery(q)
	if !ok {
		helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
			Error: false,
			Data: searchAllData{
				Query:     q,
				Movies:    movieSearchSection{Results: []database.GetMoviesLibraryAscRow{}},
				Albums:    albumSearchSection{Results: []database.GetAlbumsAlphabeticalRow{}},
				Musicians: musicianSearchSection{Results: []database.GetMusiciansAlphabeticalRow{}},
				Tracks:    trackSearchSection{Results: []database.GetTracksAlphabeticalRow{}},
			},
		})
		return
	}

	ctx := r.Context()
	limit := int64(helpers.SEARCH_ALL_TOP_N)

	movies, err := app.searchMovies(ctx, q, match, limit, 0)
	if err != nil {
		app.Logger.Error("search movies failed", "error", err)
		helpers.ErrorJSON(w, errors.New("search failed"))
		return
	}
	moviesTotal, err := app.searchCount(ctx, searchMoviesCountSQL, match)
	if err != nil {
		app.Logger.Error("search movies count failed", "error", err)
		helpers.ErrorJSON(w, errors.New("search failed"))
		return
	}

	albums, err := app.searchAlbums(ctx, q, match, limit, 0)
	if err != nil {
		app.Logger.Error("search albums failed", "error", err)
		helpers.ErrorJSON(w, errors.New("search failed"))
		return
	}
	albumsTotal, err := app.searchCount(ctx, searchAlbumsCountSQL, match)
	if err != nil {
		app.Logger.Error("search albums count failed", "error", err)
		helpers.ErrorJSON(w, errors.New("search failed"))
		return
	}

	musicians, err := app.searchMusicians(ctx, q, match, limit, 0)
	if err != nil {
		app.Logger.Error("search musicians failed", "error", err)
		helpers.ErrorJSON(w, errors.New("search failed"))
		return
	}
	musiciansTotal, err := app.searchCount(ctx, searchMusiciansCountSQL, match)
	if err != nil {
		app.Logger.Error("search musicians count failed", "error", err)
		helpers.ErrorJSON(w, errors.New("search failed"))
		return
	}

	tracks, err := app.searchTracks(ctx, q, match, limit, 0)
	if err != nil {
		app.Logger.Error("search tracks failed", "error", err)
		helpers.ErrorJSON(w, errors.New("search failed"))
		return
	}
	tracksTotal, err := app.searchCount(ctx, searchTracksCountSQL, match)
	if err != nil {
		app.Logger.Error("search tracks count failed", "error", err)
		helpers.ErrorJSON(w, errors.New("search failed"))
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error: false,
		Data: searchAllData{
			Query:     q,
			Movies:    movieSearchSection{Results: movies, Total: moviesTotal},
			Albums:    albumSearchSection{Results: albums, Total: albumsTotal},
			Musicians: musicianSearchSection{Results: musicians, Total: musiciansTotal},
			Tracks:    trackSearchSection{Results: tracks, Total: tracksTotal},
		},
	})
}

func (app *Application) SearchMovies(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	page, perPage := parseSearchPagination(r)

	match, ok := buildFTSQuery(q)
	if !ok {
		helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
			Error: false,
			Data: searchMoviesData{
				Query:   q,
				Results: []database.GetMoviesLibraryAscRow{},
				Page:    page,
				PerPage: perPage,
			},
		})
		return
	}

	ctx := r.Context()
	total, err := app.searchCount(ctx, searchMoviesCountSQL, match)
	if err != nil {
		app.Logger.Error("search movies count failed", "error", err)
		helpers.ErrorJSON(w, errors.New("search failed"))
		return
	}
	page, pages := normalizeSearchPage(page, total, perPage)

	results := []database.GetMoviesLibraryAscRow{}
	if total > 0 {
		offset := (page - 1) * perPage
		results, err = app.searchMovies(ctx, q, match, perPage, offset)
		if err != nil {
			app.Logger.Error("search movies failed", "error", err)
			helpers.ErrorJSON(w, errors.New("search failed"))
			return
		}
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error: false,
		Data: searchMoviesData{
			Query:      q,
			Results:    results,
			Total:      total,
			Page:       page,
			PerPage:    perPage,
			TotalPages: pages,
		},
	})
}

func (app *Application) SearchAlbums(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	page, perPage := parseSearchPagination(r)

	match, ok := buildFTSQuery(q)
	if !ok {
		helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
			Error: false,
			Data: searchAlbumsData{
				Query:   q,
				Results: []database.GetAlbumsAlphabeticalRow{},
				Page:    page,
				PerPage: perPage,
			},
		})
		return
	}

	ctx := r.Context()
	total, err := app.searchCount(ctx, searchAlbumsCountSQL, match)
	if err != nil {
		app.Logger.Error("search albums count failed", "error", err)
		helpers.ErrorJSON(w, errors.New("search failed"))
		return
	}
	page, pages := normalizeSearchPage(page, total, perPage)

	results := []database.GetAlbumsAlphabeticalRow{}
	if total > 0 {
		offset := (page - 1) * perPage
		results, err = app.searchAlbums(ctx, q, match, perPage, offset)
		if err != nil {
			app.Logger.Error("search albums failed", "error", err)
			helpers.ErrorJSON(w, errors.New("search failed"))
			return
		}
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error: false,
		Data: searchAlbumsData{
			Query:      q,
			Results:    results,
			Total:      total,
			Page:       page,
			PerPage:    perPage,
			TotalPages: pages,
		},
	})
}

func (app *Application) SearchMusicians(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	page, perPage := parseSearchPagination(r)

	match, ok := buildFTSQuery(q)
	if !ok {
		helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
			Error: false,
			Data: searchMusiciansData{
				Query:   q,
				Results: []database.GetMusiciansAlphabeticalRow{},
				Page:    page,
				PerPage: perPage,
			},
		})
		return
	}

	ctx := r.Context()
	total, err := app.searchCount(ctx, searchMusiciansCountSQL, match)
	if err != nil {
		app.Logger.Error("search musicians count failed", "error", err)
		helpers.ErrorJSON(w, errors.New("search failed"))
		return
	}
	page, pages := normalizeSearchPage(page, total, perPage)

	results := []database.GetMusiciansAlphabeticalRow{}
	if total > 0 {
		offset := (page - 1) * perPage
		results, err = app.searchMusicians(ctx, q, match, perPage, offset)
		if err != nil {
			app.Logger.Error("search musicians failed", "error", err)
			helpers.ErrorJSON(w, errors.New("search failed"))
			return
		}
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error: false,
		Data: searchMusiciansData{
			Query:      q,
			Results:    results,
			Total:      total,
			Page:       page,
			PerPage:    perPage,
			TotalPages: pages,
		},
	})
}

func (app *Application) SearchTracks(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	page, perPage := parseSearchPagination(r)

	match, ok := buildFTSQuery(q)
	if !ok {
		helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
			Error: false,
			Data: searchTracksData{
				Query:   q,
				Results: []database.GetTracksAlphabeticalRow{},
				Page:    page,
				PerPage: perPage,
			},
		})
		return
	}

	ctx := r.Context()
	total, err := app.searchCount(ctx, searchTracksCountSQL, match)
	if err != nil {
		app.Logger.Error("search tracks count failed", "error", err)
		helpers.ErrorJSON(w, errors.New("search failed"))
		return
	}
	page, pages := normalizeSearchPage(page, total, perPage)

	results := []database.GetTracksAlphabeticalRow{}
	if total > 0 {
		offset := (page - 1) * perPage
		results, err = app.searchTracks(ctx, q, match, perPage, offset)
		if err != nil {
			app.Logger.Error("search tracks failed", "error", err)
			helpers.ErrorJSON(w, errors.New("search failed"))
			return
		}
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error: false,
		Data: searchTracksData{
			Query:      q,
			Results:    results,
			Total:      total,
			Page:       page,
			PerPage:    perPage,
			TotalPages: pages,
		},
	})
}
