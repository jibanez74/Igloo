package main

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

// searchStaleStats is the sqlite_stat1 of a library analyzed while it held
// three movies and three tracks of one album, and one of everything else.
var searchStaleStats = [][3]string{
	{"albums", "idx_albums_alpha", "1 1 1"},
	{"albums", "idx_albums_artwork_pending", "0 0"},
	{"albums", "idx_albums_created_at", "1 1"},
	{"albums", "idx_albums_title_musician", "1 1 1"},
	{"albums", "sqlite_autoindex_albums_1", "1 1"},
	{"albums", "sqlite_autoindex_albums_2", "1 1"},
	{"albums_fts_config", "albums_fts_config", "1 1"},
	{"albums_fts_idx", "albums_fts_idx", "1 1 1"},
	{"movies", "idx_movies_artwork_pending", "0 0"},
	{"movies", "idx_movies_created_at", "3 3"},
	{"movies", "idx_movies_title", "3 1 1"},
	{"movies", "idx_movies_tmdb_id", "3 3"},
	{"movies", "sqlite_autoindex_movies_1", "3 1"},
	{"movies_fts_config", "movies_fts_config", "1 1"},
	{"movies_fts_idx", "movies_fts_idx", "3 1 1"},
	{"musician_albums", "idx_musician_albums_album", "1 1"},
	{"musician_albums", "sqlite_autoindex_musician_albums_1", "1 1 1"},
	{"musicians", "idx_musicians_alpha", "1 1 1"},
	{"musicians", "idx_musicians_artwork_pending", "0 0"},
	{"musicians", "sqlite_autoindex_musicians_1", "1 1"},
	{"musicians", "sqlite_autoindex_musicians_2", "1 1"},
	{"musicians_fts_config", "musicians_fts_config", "1 1"},
	{"musicians_fts_idx", "musicians_fts_idx", "2 1 1"},
	{"shows", "idx_shows_artwork_pending", "0 0"},
	{"shows", "idx_shows_created_at", "1 1"},
	{"shows", "idx_shows_name", "1 1 1"},
	{"shows", "idx_shows_tmdb_id", "1 1"},
	{"shows", "sqlite_autoindex_shows_1", "1 1"},
	{"shows_fts_config", "shows_fts_config", "1 1"},
	{"shows_fts_idx", "shows_fts_idx", "1 1 1"},
	{"track_musicians", "idx_track_musicians_musician_track", "3 3 1"},
	{"track_musicians", "sqlite_autoindex_track_musicians_1", "3 1 1"},
	{"tracks", "idx_track_album", "3 3 3 1"},
	{"tracks", "idx_track_alpha", "3 3 1"},
	{"tracks", "idx_track_musician", "3 3"},
	{"tracks", "music_track_dates", "3 3 3"},
	{"tracks", "sqlite_autoindex_tracks_1", "3 1"},
	{"tracks_search_fts_config", "tracks_search_fts_config", "1 1"},
	{"tracks_search_fts_idx", "tracks_search_fts_idx", "3 1 1"},
}

// Under statistics taken while the library was tiny, each page query must
// still run its MATCH once, as the outer loop, and look every joined row up
// by its key, rather than re-running the prefix match for every row.
func TestSearchPageQueriesScanTheFTSTableFirstUnderStaleStatistics(t *testing.T) {
	app := newTestDBApp(t)
	ctx := context.Background()

	_, err := app.DB.ExecContext(ctx, "ANALYZE; DELETE FROM sqlite_stat1;")
	if err != nil {
		t.Fatalf("reset statistics: %v", err)
	}
	for _, stat := range searchStaleStats {
		_, err = app.DB.ExecContext(ctx, "INSERT INTO sqlite_stat1 (tbl, idx, stat) VALUES (?, ?, ?)", stat[0], stat[1], stat[2])
		if err != nil {
			t.Fatalf("insert statistics for %s: %v", stat[1], err)
		}
	}
	_, err = app.DB.ExecContext(ctx, "ANALYZE sqlite_schema;")
	if err != nil {
		t.Fatalf("reload statistics: %v", err)
	}

	cases := []struct {
		sql string
		fts string
	}{
		{searchMoviesSQL, "movies_fts"},
		{searchShowsSQL, "shows_fts"},
		{searchAlbumsSQL, "albums_fts"},
		{searchMusiciansSQL, "musicians_fts"},
		{searchTracksJoinedSQL, "tracks_search_fts"},
	}
	for _, tc := range cases {
		plan := searchQueryPlan(t, app.DB, tc.sql)
		outer := "SCAN " + tc.fts + " VIRTUAL TABLE"
		hasOuterFTS := len(plan) > 0 && strings.HasPrefix(plan[0], outer)
		if !hasOuterFTS {
			t.Errorf("%s: want the FTS table as the outer loop, plan %q", tc.fts, plan)
		}
	}
}

func searchQueryPlan(t *testing.T, db *sql.DB, query string) []string {
	t.Helper()

	args := make([]any, strings.Count(query, "?"))
	rows, err := db.QueryContext(context.Background(), "EXPLAIN QUERY PLAN "+query, args...)
	if err != nil {
		t.Fatalf("explain: %v", err)
	}
	defer rows.Close()

	var plan []string
	for rows.Next() {
		var id, parent, unused int
		var detail string
		err = rows.Scan(&id, &parent, &unused, &detail)
		if err != nil {
			t.Fatalf("scan plan: %v", err)
		}
		plan = append(plan, detail)
	}
	err = rows.Err()
	if err != nil {
		t.Fatalf("read plan: %v", err)
	}
	return plan
}

// A library analyzed while it held one album grew to forty before the server
// stopped without refreshing its statistics. The next start has queried
// nothing yet, so a plain PRAGMA optimize would skip the table and keep
// planning for one album: a full scan of albums for every match.
func TestRefreshQueryPlannerStatsReanalyzesTablesThatGrew(t *testing.T) {
	path := filepath.Join(t.TempDir(), "igloo.db")
	ctx := context.Background()

	seed := openSearchStatsTestApp(t, path)
	err := seed.InitTables()
	if err != nil {
		t.Fatalf("InitTables failed: %v", err)
	}
	_, err = seed.DB.ExecContext(ctx, `
INSERT INTO albums (title, sort_title) VALUES ('First', 'first');
ANALYZE;
WITH RECURSIVE n(i) AS (SELECT 2 UNION ALL SELECT i + 1 FROM n WHERE i < 40)
INSERT INTO albums (title, sort_title) SELECT 'Album ' || i, 'album ' || i FROM n;`)
	if err != nil {
		t.Fatalf("seed albums: %v", err)
	}
	err = seed.DB.Close()
	if err != nil {
		t.Fatalf("close database: %v", err)
	}

	app := openSearchStatsTestApp(t, path)
	err = app.RefreshQueryPlannerStats(t.Context())
	if err != nil {
		t.Fatalf("RefreshQueryPlannerStats failed: %v", err)
	}

	var stat string
	err = app.DB.QueryRowContext(ctx, "SELECT stat FROM sqlite_stat1 WHERE tbl = 'albums' AND idx = 'sqlite_autoindex_albums_1'").Scan(&stat)
	if err != nil {
		t.Fatalf("read albums statistics: %v", err)
	}
	isFresh := strings.HasPrefix(stat, "40 ")
	if !isFresh {
		t.Errorf("albums statistics = %q, want them taken over 40 rows", stat)
	}
}

// openSearchStatsTestApp opens the database at path through InitDB, so the
// statistics refresh runs on production's driver, connection pragmas and WAL.
func openSearchStatsTestApp(t *testing.T, path string) *Application {
	t.Helper()

	app := &Application{Config: RuntimeConfig{DBPath: path}}
	setupTestLogger(t, app)
	err := app.InitDB()
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	t.Cleanup(func() {
		app.DB.Close()
	})
	return app
}

func TestSearchStatementsArePreparedAtStartupAndClosed(t *testing.T) {
	app := setupTestApp(t)
	createTestMovie(t, app, "Casino Royale", "/movies/casino-royale.mkv")

	if len(app.SearchStatements) != len(searchStatementQueries) {
		t.Fatalf("prepared %d statements, want %d", len(app.SearchStatements), len(searchStatementQueries))
	}
	prepared := make(map[string]*sql.Stmt, len(searchStatementQueries))
	for _, query := range searchStatementQueries {
		stmt, ok := app.SearchStatements[query]
		if !ok {
			t.Fatalf("statement not prepared at startup: %s", query)
		}
		prepared[query] = stmt
	}

	// The typo resolves at stage 2, so the count, the vocabulary generation
	// and the page query all run, and none of them may prepare again.
	for range 2 {
		results := searchEntityResults(t, app, movieSearchEntity, "Casino Roayle")
		if len(results) != 1 {
			t.Fatalf("want Casino Royale, got %d results", len(results))
		}
	}
	for query, stmt := range prepared {
		if app.SearchStatements[query] != stmt {
			t.Errorf("statement prepared again: %s", query)
		}
	}

	err := app.SearchStatements.close()
	if err != nil {
		t.Fatalf("close: %v", err)
	}
	if len(app.SearchStatements) != 0 {
		t.Errorf("%d statements left after close", len(app.SearchStatements))
	}
}

// The ranking buckets compare with NOCASE and a plain LIKE rather than
// LOWER() on both sides. Without ICU, lower(), NOCASE and LIKE all fold ASCII
// letters only, so the two forms put every title in the same bucket.
func TestSearchRankBucketsMatchLowerComparisons(t *testing.T) {
	app := newTestDBApp(t)
	ctx := context.Background()

	values := []any{
		"The Beatles", "the beatles", "THE BEATLES", "The Beatles Anthology",
		"Élan", "élan", "ÉLAN", "Straße", "STRASSE", "Ωmega", "ωmega",
		"100% Pure_Fun\\", "100x Purexfun", "", nil,
	}
	queries := []string{"the beatles", "THE", "élan", "ÉLAN", "straße", "ωmega", "Ωmega", "100% pure_", "100%", "pure_fun\\"}

	_, err := app.DB.ExecContext(ctx, "CREATE TEMP TABLE rank_values (x TEXT)")
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	for _, value := range values {
		_, err = app.DB.ExecContext(ctx, "INSERT INTO rank_values (x) VALUES (?)", value)
		if err != nil {
			t.Fatalf("insert %v: %v", value, err)
		}
	}

	const compare = `
SELECT x,
  CASE
    WHEN LOWER(COALESCE(x, '')) = LOWER(?1) THEN 0
    WHEN LOWER(COALESCE(x, '')) LIKE ?2 ESCAPE '\' THEN 1
    ELSE 2
  END,
  CASE
    WHEN COALESCE(x, '') = ?1 COLLATE NOCASE THEN 0
    WHEN COALESCE(x, '') LIKE ?2 ESCAPE '\' THEN 1
    ELSE 2
  END
FROM rank_values`
	for _, query := range queries {
		exact, prefix := searchRankArgs(query)
		rows, err := app.DB.QueryContext(ctx, compare, exact, prefix)
		if err != nil {
			t.Fatalf("compare %q: %v", query, err)
		}
		for rows.Next() {
			var value sql.NullString
			var lowered, folded int
			err = rows.Scan(&value, &lowered, &folded)
			if err != nil {
				rows.Close()
				t.Fatalf("scan: %v", err)
			}
			if lowered != folded {
				t.Errorf("query %q, value %q: LOWER bucket %d, NOCASE bucket %d", query, value.String, lowered, folded)
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			t.Fatalf("read: %v", err)
		}
	}
}
