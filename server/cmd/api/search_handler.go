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
)

const (
	searchDefaultPerPage = 24
	searchMaxPerPage     = 48
	searchAllTopN        = 8
)

type searchSection[T any] struct {
	Results []T   `json:"results"`
	Total   int64 `json:"total"`
}

type searchAllData struct {
	Query     string                                              `json:"query"`
	Movies    searchSection[database.GetMoviesLibraryAscRow]      `json:"movies"`
	Albums    searchSection[database.GetAlbumsAlphabeticalRow]    `json:"albums"`
	Musicians searchSection[database.GetMusiciansAlphabeticalRow] `json:"musicians"`
	Tracks    searchSection[database.GetTracksAlphabeticalRow]    `json:"tracks"`
}

type searchCategoryData[T any] struct {
	Query      string `json:"query"`
	Results    []T    `json:"results"`
	Total      int64  `json:"total"`
	Page       int64  `json:"page"`
	PerPage    int64  `json:"per_page"`
	TotalPages int64  `json:"total_pages"`
}

// sqlc cannot parse FTS5 MATCH expressions, so these queries are hand-written
// and executed directly against app.DB. Result columns are scanned into the
// existing sqlc-generated row types so the JSON shape matches the regular
// list endpoints and the frontend cards work unchanged.
//
// bm25() column weights bias ranking toward name/title matches over
// descriptive fields when the exact/prefix CASE buckets tie.

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
  bm25(movies_fts, 10.0, 1.0, 3.0),
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
  bm25(albums_fts, 10.0, 4.0),
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
  bm25(musicians_fts, 10.0, 5.0),
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
  bm25(tracks_search_fts, 10.0, 4.0, 4.0),
  t.title
LIMIT ? OFFSET ?`

// searchEntity describes one searchable category: its SQL, the fts5vocab
// table used for typo correction, the positional ranking arguments its page
// query expects, and how to scan a result row.
type searchEntity[T any] struct {
	name       string
	pageSQL    string
	countSQL   string
	vocabTable string
	rankArgs   func(exact, prefix string) []any
	scan       func(*sql.Rows) (T, error)
}

var movieSearchEntity = searchEntity[database.GetMoviesLibraryAscRow]{
	name:       "movies",
	pageSQL:    searchMoviesSQL,
	countSQL:   searchMoviesCountSQL,
	vocabTable: "movies_fts_vocab",
	rankArgs: func(exact, prefix string) []any {
		return []any{exact, prefix}
	},
	scan: func(rows *sql.Rows) (database.GetMoviesLibraryAscRow, error) {
		var row database.GetMoviesLibraryAscRow
		err := rows.Scan(&row.ID, &row.Title, &row.PosterPath, &row.Year, &row.Certification)
		return row, err
	},
}

var albumSearchEntity = searchEntity[database.GetAlbumsAlphabeticalRow]{
	name:       "albums",
	pageSQL:    searchAlbumsSQL,
	countSQL:   searchAlbumsCountSQL,
	vocabTable: "albums_fts_vocab",
	rankArgs: func(exact, prefix string) []any {
		return []any{exact, prefix, exact, prefix}
	},
	scan: func(rows *sql.Rows) (database.GetAlbumsAlphabeticalRow, error) {
		var row database.GetAlbumsAlphabeticalRow
		err := rows.Scan(&row.ID, &row.Title, &row.Cover, &row.Musician, &row.Year)
		return row, err
	},
}

var musicianSearchEntity = searchEntity[database.GetMusiciansAlphabeticalRow]{
	name:       "musicians",
	pageSQL:    searchMusiciansSQL,
	countSQL:   searchMusiciansCountSQL,
	vocabTable: "musicians_fts_vocab",
	rankArgs: func(exact, prefix string) []any {
		return []any{exact, exact, prefix, prefix}
	},
	scan: func(rows *sql.Rows) (database.GetMusiciansAlphabeticalRow, error) {
		var row database.GetMusiciansAlphabeticalRow
		err := rows.Scan(&row.ID, &row.Name, &row.Thumb, &row.SortName, &row.AlbumCount, &row.TrackCount)
		return row, err
	},
}

var trackSearchEntity = searchEntity[database.GetTracksAlphabeticalRow]{
	name:       "tracks",
	pageSQL:    searchTracksJoinedSQL,
	countSQL:   searchTracksCountSQL,
	vocabTable: "tracks_search_fts_vocab",
	rankArgs: func(exact, prefix string) []any {
		return []any{exact, prefix, exact, exact, exact, prefix, prefix, prefix}
	},
	scan: func(rows *sql.Rows) (database.GetTracksAlphabeticalRow, error) {
		var row database.GetTracksAlphabeticalRow
		err := rows.Scan(
			&row.ID, &row.Title, &row.Duration, &row.Codec, &row.BitRate, &row.FilePath,
			&row.AlbumID, &row.AlbumTitle, &row.AlbumCover,
			&row.MusicianID, &row.MusicianName,
		)
		return row, err
	},
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

	perPage = int64(searchDefaultPerPage)
	if pp := r.URL.Query().Get("per_page"); pp != "" {
		parsed, err := strconv.ParseInt(pp, 10, 64)
		if err == nil && parsed > 0 {
			perPage = parsed
		}
	}
	if perPage > searchMaxPerPage {
		perPage = searchMaxPerPage
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

func (app *Application) searchCount(ctx context.Context, query, match string) (int64, error) {
	var total int64
	err := app.DB.QueryRowContext(ctx, query, match).Scan(&total)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	return total, nil
}

// searchEntityPage runs the entity's page query for an already-resolved MATCH
// expression. raw is the user's original query, used only for ranking.
func searchEntityPage[T any](ctx context.Context, app *Application, e searchEntity[T], raw, match string, limit, offset int64) ([]T, error) {
	exact, prefix := searchRankArgs(raw)
	args := append([]any{match}, e.rankArgs(exact, prefix)...)
	args = append(args, limit, offset)

	rows, err := app.DB.QueryContext(ctx, e.pageSQL, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []T{}
	for rows.Next() {
		row, scanErr := e.scan(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, row)
	}
	err = rows.Err()
	if err != nil {
		return nil, err
	}
	return out, nil
}

// searchEntityTopN resolves the match for one entity and returns its top
// results plus the total match count, for the combined /api/search response.
func searchEntityTopN[T any](ctx context.Context, app *Application, e searchEntity[T], raw string, limit int64) (searchSection[T], error) {
	section := searchSection[T]{Results: []T{}}

	match, total, ok, err := app.resolveSearchMatch(ctx, e.countSQL, e.vocabTable, raw)
	if err != nil {
		return section, err
	}
	if !ok || total == 0 {
		return section, nil
	}

	results, err := searchEntityPage(ctx, app, e, raw, match, limit, 0)
	if err != nil {
		return section, err
	}
	section.Results = results
	section.Total = total
	return section, nil
}

func (app *Application) SearchAll(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	ctx := r.Context()
	limit := int64(searchAllTopN)

	movies, err := searchEntityTopN(ctx, app, movieSearchEntity, q, limit)
	if err != nil {
		app.Logger.Error("search failed", "entity", movieSearchEntity.name, "error", err)
		helpers.ErrorJSON(w, errors.New("search failed"))
		return
	}

	albums, err := searchEntityTopN(ctx, app, albumSearchEntity, q, limit)
	if err != nil {
		app.Logger.Error("search failed", "entity", albumSearchEntity.name, "error", err)
		helpers.ErrorJSON(w, errors.New("search failed"))
		return
	}

	musicians, err := searchEntityTopN(ctx, app, musicianSearchEntity, q, limit)
	if err != nil {
		app.Logger.Error("search failed", "entity", musicianSearchEntity.name, "error", err)
		helpers.ErrorJSON(w, errors.New("search failed"))
		return
	}

	tracks, err := searchEntityTopN(ctx, app, trackSearchEntity, q, limit)
	if err != nil {
		app.Logger.Error("search failed", "entity", trackSearchEntity.name, "error", err)
		helpers.ErrorJSON(w, errors.New("search failed"))
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error: false,
		Data: searchAllData{
			Query:     q,
			Movies:    movies,
			Albums:    albums,
			Musicians: musicians,
			Tracks:    tracks,
		},
	})
}

// handleSearchCategory implements the shared paginated category endpoint
// behavior. Exported per-category methods below exist so the router and
// OpenAPI coverage keep stable handler names.
func handleSearchCategory[T any](app *Application, e searchEntity[T], w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	page, perPage := parseSearchPagination(r)
	ctx := r.Context()

	match, total, ok, err := app.resolveSearchMatch(ctx, e.countSQL, e.vocabTable, q)
	if err != nil {
		app.Logger.Error("search failed", "entity", e.name, "error", err)
		helpers.ErrorJSON(w, errors.New("search failed"))
		return
	}
	if !ok {
		helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
			Error: false,
			Data: searchCategoryData[T]{
				Query:   q,
				Results: []T{},
				Page:    page,
				PerPage: perPage,
			},
		})
		return
	}

	page, pages := normalizeSearchPage(page, total, perPage)

	results := []T{}
	if total > 0 {
		offset := (page - 1) * perPage
		results, err = searchEntityPage(ctx, app, e, q, match, perPage, offset)
		if err != nil {
			app.Logger.Error("search failed", "entity", e.name, "error", err)
			helpers.ErrorJSON(w, errors.New("search failed"))
			return
		}
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error: false,
		Data: searchCategoryData[T]{
			Query:      q,
			Results:    results,
			Total:      total,
			Page:       page,
			PerPage:    perPage,
			TotalPages: pages,
		},
	})
}

func (app *Application) SearchMovies(w http.ResponseWriter, r *http.Request) {
	handleSearchCategory(app, movieSearchEntity, w, r)
}

func (app *Application) SearchAlbums(w http.ResponseWriter, r *http.Request) {
	handleSearchCategory(app, albumSearchEntity, w, r)
}

func (app *Application) SearchMusicians(w http.ResponseWriter, r *http.Request) {
	handleSearchCategory(app, musicianSearchEntity, w, r)
}

func (app *Application) SearchTracks(w http.ResponseWriter, r *http.Request) {
	handleSearchCategory(app, trackSearchEntity, w, r)
}
