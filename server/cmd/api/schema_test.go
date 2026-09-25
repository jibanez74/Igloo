package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"igloo/cmd/internal/database"
	"strings"
	"testing"
)

func TestInitTables_Indexes(t *testing.T) {
	db := newTestDBApp(t).DB

	expectedIndexes := []string{
		"idx_user_name",
		"idx_track_album",
		"idx_track_musician",
		"idx_track_alpha",
		"idx_musician_albums_album",
		"idx_sessions_expiry",
		"idx_movie_watch_progress_user_updated_at",
		"idx_settings_singleton",
		"idx_devices_last_used_at",
		// Ordering indexes for the paginated library listings.
		"idx_movies_title",
		"idx_movies_created_at",
		"idx_albums_alpha",
		"idx_albums_created_at",
		"idx_musicians_alpha",
		"idx_user_liked_tracks_user_created",
		// Foreign key columns that no other index covers. See
		// TestSchema_ForeignKeysAreIndexed for the general rule.
		"idx_chapters_movie",
		"idx_movie_watch_progress_movie",
		"idx_playlists_movie",
		"idx_playlist_tracks_track",
		"idx_playlist_tracks_added_by",
		"idx_playlist_movies_movie",
		"idx_playlist_movies_added_by",
		"idx_user_track_stats_track",
		// Album identity, which must tolerate a NULL musician.
		"idx_albums_title_musician",
	}

	for _, indexName := range expectedIndexes {
		t.Run("Index_"+indexName, func(t *testing.T) {

			var name string

			err := db.QueryRow(
				"SELECT name FROM sqlite_master WHERE type='index' AND name=?",
				indexName,
			).Scan(&name)

			if err != nil {
				t.Errorf("Index '%s' does not exist: %v", indexName, err)
			}
		})
	}

	// These indexes are redundant or serve no query or reachable cascade.
	// Re-adding them would add unnecessary writes to library scans.
	removedIndexes := []string{
		"idx_crew_department",
		"idx_playlist_user",
		"idx_playlist_content_type",
		"idx_cast_artist",
		"idx_crew_artist",
		"idx_movie_production_companies_company",
		"idx_movie_extra_videos_extra",
		"idx_track_genres_genre",
		"idx_album_genres_genre",
		"idx_musician_genres_genre",
		"idx_audio_streams_language",
		"idx_subtitles_language",
		"idx_user_track_stats_last_played",
	}

	for _, indexName := range removedIndexes {
		t.Run("RemovedIndex_"+indexName, func(t *testing.T) {

			var name string

			err := db.QueryRow(
				"SELECT name FROM sqlite_master WHERE type='index' AND name=?",
				indexName,
			).Scan(&name)

			if !errors.Is(err, sql.ErrNoRows) {
				t.Errorf("Index '%s' was removed deliberately but still exists", indexName)
			}
		})
	}

	var watchProgressIndexSQL string
	err := db.QueryRow(
		"SELECT sql FROM sqlite_master WHERE type='index' AND name=?",
		"idx_movie_watch_progress_user_updated_at",
	).Scan(&watchProgressIndexSQL)
	if err != nil {
		t.Fatalf("Failed to read movie watch progress index definition: %v", err)
	}

	if !strings.Contains(watchProgressIndexSQL, "WHERE watched = false") {
		t.Fatalf("Expected movie watch progress index to exclude watched movies, got %q", watchProgressIndexSQL)
	}
}

// parentTablesNeverDeleted lists the catalog tables that no code path ever deletes
// a row from -- they are only ever inserted or upserted by the scanners. A foreign
// key pointing at one of them needs no backing index, because the cascade it would
// serve can never fire, and the index would cost write time on every library scan.
//
// If you add a delete path for one of these tables, remove it from this list and
// add the index the test then asks for.
var parentTablesNeverDeleted = map[string]string{
	"artist":               "cast and crew rows are replaced per movie, artists themselves are never removed",
	"production_companies": "only upserted by the movie scanner",
	"extra_videos":         "only upserted by the movie scanner",
	"genres":               "only upserted via GetOrCreateGenre",
}

// TestSchema_ForeignKeysAreIndexed asserts that every foreign key's child columns
// are the left prefix of some index. SQLite does not index the child side of a
// foreign key automatically, so an unindexed one turns each parent delete into a
// full scan of the child table -- and those scans multiply, because deleting an
// album cascades through every one of its tracks. Failing here means a new table
// or column needs an index, not that this test needs relaxing.
func TestSchema_ForeignKeysAreIndexed(t *testing.T) {
	db := newTestDBApp(t).DB

	tables, err := schemaTableNames(db)
	if err != nil {
		t.Fatalf("Failed to list tables: %v", err)
	}

	for _, table := range tables {
		foreignKeys, err := tableForeignKeys(db, table)
		if err != nil {
			t.Fatalf("Failed to read foreign keys for %q: %v", table, err)
		}

		if len(foreignKeys) == 0 {
			continue
		}

		indexes, err := tableIndexPrefixes(db, table)
		if err != nil {
			t.Fatalf("Failed to read indexes for %q: %v", table, err)
		}

		for _, foreignKey := range foreignKeys {
			_, exempt := parentTablesNeverDeleted[strings.ToLower(foreignKey.parent)]
			if exempt {
				continue
			}

			name := fmt.Sprintf("%s(%s)", table, strings.Join(foreignKey.columns, ","))

			t.Run(name, func(t *testing.T) {
				covered := false

				for _, indexColumns := range indexes {
					if isColumnPrefix(foreignKey.columns, indexColumns) {
						covered = true
						break
					}
				}

				if !covered {
					t.Errorf(
						"Foreign key %s -> %s has no index leading with those columns; deletes on %s will full-scan %s",
						name, foreignKey.parent, foreignKey.parent, table,
					)
				}
			})
		}
	}
}

// schemaTableNames returns the ordinary tables in the database. FTS5 shadow tables
// are included but harmless: they declare no foreign keys, so they are skipped by
// the caller.
func schemaTableNames(db *sql.DB) ([]string, error) {
	rows, err := db.Query(
		"SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var names []string

	for rows.Next() {
		var name string

		err := rows.Scan(&name)
		if err != nil {
			return nil, err
		}

		names = append(names, name)
	}

	return names, rows.Err()
}

// schemaForeignKey is one foreign key: the child columns that need indexing and the
// parent table whose deletes would use them.
type schemaForeignKey struct {
	columns []string
	parent  string
}

// tableForeignKeys returns each foreign key on the table, with its child columns in
// declaration order. Composite keys arrive as multiple rows sharing an id.
func tableForeignKeys(db *sql.DB, table string) ([]schemaForeignKey, error) {
	rows, err := db.Query(fmt.Sprintf("PRAGMA foreign_key_list(%q)", table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byID := make(map[int]*schemaForeignKey)
	var order []int

	for rows.Next() {
		var (
			id       int
			seq      int
			refTable string
			from     sql.NullString
			to       sql.NullString
			onUpdate string
			onDelete string
			match    string
		)

		err := rows.Scan(&id, &seq, &refTable, &from, &to, &onUpdate, &onDelete, &match)
		if err != nil {
			return nil, err
		}

		existing, seen := byID[id]
		if !seen {
			order = append(order, id)
			byID[id] = &schemaForeignKey{columns: []string{from.String}, parent: refTable}
			continue
		}

		existing.columns = append(existing.columns, from.String)
	}

	err = rows.Err()
	if err != nil {
		return nil, err
	}

	keys := make([]schemaForeignKey, 0, len(order))
	for _, id := range order {
		keys = append(keys, *byID[id])
	}

	return keys, nil
}

// tableIndexPrefixes returns the indexed column names of every usable index on the
// table, including the implicit indexes behind PRIMARY KEY and UNIQUE. Partial
// indexes are excluded because they cannot serve an arbitrary cascade lookup.
func tableIndexPrefixes(db *sql.DB, table string) ([][]string, error) {
	rows, err := db.Query(fmt.Sprintf("PRAGMA index_list(%q)", table))
	if err != nil {
		return nil, err
	}

	type indexRow struct {
		name    string
		partial bool
	}

	var indexRows []indexRow

	for rows.Next() {
		var (
			seq     int
			name    string
			unique  bool
			origin  string
			partial bool
		)

		err := rows.Scan(&seq, &name, &unique, &origin, &partial)
		if err != nil {
			rows.Close()
			return nil, err
		}

		indexRows = append(indexRows, indexRow{name: name, partial: partial})
	}

	err = rows.Err()
	if err != nil {
		rows.Close()
		return nil, err
	}

	rows.Close()

	var indexes [][]string

	for _, index := range indexRows {
		if index.partial {
			continue
		}

		columns, err := indexColumns(db, index.name)
		if err != nil {
			return nil, err
		}

		indexes = append(indexes, columns)
	}

	// An INTEGER PRIMARY KEY is the rowid and has no entry in index_list, so add it
	// explicitly; otherwise a self-referencing foreign key would look uncovered.
	rowidColumn, err := integerPrimaryKeyColumn(db, table)
	if err != nil {
		return nil, err
	}

	if rowidColumn != "" {
		indexes = append(indexes, []string{rowidColumn})
	}

	return indexes, nil
}

// indexColumns returns an index's columns in order. Expression columns report a
// NULL name and terminate the usable prefix.
func indexColumns(db *sql.DB, index string) ([]string, error) {
	rows, err := db.Query(fmt.Sprintf("PRAGMA index_info(%q)", index))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []string

	for rows.Next() {
		var (
			seqno int
			cid   int
			name  sql.NullString
		)

		err := rows.Scan(&seqno, &cid, &name)
		if err != nil {
			return nil, err
		}

		if !name.Valid {
			break
		}

		columns = append(columns, name.String)
	}

	return columns, rows.Err()
}

// integerPrimaryKeyColumn returns the name of the table's INTEGER PRIMARY KEY
// column, or "" when the table has none or uses a composite primary key.
func integerPrimaryKeyColumn(db *sql.DB, table string) (string, error) {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%q)", table))
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var (
		found string
		count int
	)

	for rows.Next() {
		var (
			cid        int
			name       string
			columnType string
			notNull    bool
			defaultVal sql.NullString
			pk         int
		)

		err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultVal, &pk)
		if err != nil {
			return "", err
		}

		if pk == 0 {
			continue
		}

		count++

		if strings.EqualFold(columnType, "INTEGER") {
			found = name
		}
	}

	err = rows.Err()
	if err != nil {
		return "", err
	}

	if count != 1 {
		return "", nil
	}

	return found, nil
}

// isColumnPrefix reports whether want is the leading run of have.
func isColumnPrefix(want []string, have []string) bool {
	if len(want) > len(have) {
		return false
	}

	for i, column := range want {
		if !strings.EqualFold(column, have[i]) {
			return false
		}
	}

	return true
}

func TestInitTables_Idempotent(t *testing.T) {
	app := newTestDBApp(t)
	db := app.DB

	_, err := db.Exec(`
		INSERT INTO movies (id, title, file_path, file_name, size, container, mime_type, adult)
		VALUES (1, 'Moonrise', '/movies/moonrise.mkv', 'moonrise.mkv', 1, 'mkv', 'video/x-matroska', false);
		INSERT INTO shows (id, directory_path, local_name, name, overview)
		VALUES (1, '/shows/Nightfall', 'Nightfall', 'Nightfall', 'Dusk falls on a quiet town.');
		INSERT INTO musicians (id, name, sort_name) VALUES (1, 'Aurora', 'aurora');
		INSERT INTO albums (id, title, sort_title, musician) VALUES (1, 'Daylight', 'daylight', 'Aurora');
		INSERT INTO tracks (
			id, title, sort_title, file_path, file_name, container, mime_type, codec,
			size, track_index, duration, disc, channels, channel_layout, bit_rate, profile,
			album_id, musician_id
		) VALUES (
			1, 'Sunrise', 'sunrise', '/music/sunrise.flac', 'sunrise.flac', 'flac', 'audio/flac', 'flac',
			1, 1, 180, 1, '2', 'stereo', 1000, '', 1, 1
		);
		INSERT INTO genres (id, tag, genre_type) VALUES (1, 'Pop', 'music');
		INSERT INTO track_musicians (track_id, musician_id) VALUES (1, 1);
		INSERT INTO track_genres (track_id, genre_id) VALUES (1, 1);
		INSERT INTO album_genres (album_id, genre_id, source) VALUES (1, 1, 'spotify');
		INSERT INTO musician_genres (musician_id, genre_id, source) VALUES (1, 1, 'spotify');
		INSERT INTO music_artist_identity (identity_key, musician_id) VALUES ('aurora', 1);
		INSERT INTO music_album_identity (title_key, artist_key, album_id) VALUES ('daylight', 'aurora', 1);
		INSERT INTO music_credit_metadata (track_id, musician_id, sort_name) VALUES (1, 1, 'aurora');
		INSERT INTO music_spotify_matches (entity_type, entity_id, status) VALUES ('album', 1, 'unmatched');
	`)
	if err != nil {
		t.Fatalf("populate catalog: %v", err)
	}

	checks := []struct {
		name  string
		query string
		want  string
	}{
		{"movie", `SELECT id || ':' || title || ':' || file_path FROM movies`, "1:Moonrise:/movies/moonrise.mkv"},
		{"musician", `SELECT id || ':' || name || ':' || sort_name FROM musicians`, "1:Aurora:aurora"},
		{"album", `SELECT id || ':' || title || ':' || musician FROM albums`, "1:Daylight:Aurora"},
		{"track", `SELECT id || ':' || title || ':' || file_path || ':' || album_id || ':' || musician_id FROM tracks`, "1:Sunrise:/music/sunrise.flac:1:1"},
		{"genre", `SELECT id || ':' || tag || ':' || genre_type FROM genres`, "1:Pop:music"},
		{"track credit", `SELECT track_id || ':' || musician_id FROM track_musicians`, "1:1"},
		{"track genre", `SELECT track_id || ':' || genre_id FROM track_genres`, "1:1"},
		{"derived musician album", `SELECT musician_id || ':' || album_id FROM musician_albums`, "1:1"},
		{"musician genre provenance", `SELECT group_concat(source) FROM (SELECT source FROM musician_genres WHERE musician_id = 1 AND genre_id = 1 ORDER BY source)`, "local,spotify"},
		{"album genre provenance", `SELECT group_concat(source) FROM (SELECT source FROM album_genres WHERE album_id = 1 AND genre_id = 1 ORDER BY source)`, "local,spotify"},
		{"artist alias", `SELECT identity_key || ':' || musician_id FROM music_artist_identity`, "aurora:1"},
		{"album alias", `SELECT title_key || ':' || artist_key || ':' || album_id FROM music_album_identity`, "daylight:aurora:1"},
		{"sort contribution", `SELECT track_id || ':' || musician_id || ':' || sort_name FROM music_credit_metadata`, "1:1:aurora"},
		{"match cache", `SELECT entity_type || ':' || entity_id || ':' || status FROM music_spotify_matches`, "album:1:unmatched"},
		{"movie search", `SELECT group_concat(rowid) FROM movies_fts WHERE movies_fts MATCH 'moonrise'`, "1"},
		{"show search", `SELECT group_concat(rowid) FROM shows_fts WHERE shows_fts MATCH 'nightfall dusk'`, "1"},
		{"album search", `SELECT group_concat(rowid) FROM albums_fts WHERE albums_fts MATCH 'daylight aurora'`, "1"},
		{"musician search", `SELECT group_concat(rowid) FROM musicians_fts WHERE musicians_fts MATCH 'aurora'`, "1"},
		{"track search", `SELECT group_concat(rowid) FROM tracks_search_fts WHERE tracks_search_fts MATCH 'sunrise daylight aurora'`, "1"},
		{"vocabulary generations", `SELECT group_concat(vocab_table || ':' || generation) FROM (SELECT * FROM search_vocab_generations ORDER BY vocab_table)`, "albums_fts_vocab:1,movies_fts_vocab:1,musicians_fts_vocab:1,shows_fts_vocab:1,tracks_search_fts_vocab:1"},
		{"foreign key integrity", `SELECT COUNT(*) FROM pragma_foreign_key_check`, "0"},
	}

	assertPopulatedState := func(t *testing.T) {
		t.Helper()
		for _, check := range checks {
			t.Run(check.name, func(t *testing.T) {
				var got string
				err := db.QueryRow(check.query).Scan(&got)
				if err != nil {
					t.Fatalf("read populated state: %v", err)
				}
				if got != check.want {
					t.Errorf("got %q, want %q", got, check.want)
				}
			})
		}
	}

	t.Run("before reinitialization", assertPopulatedState)

	err = app.InitTables()
	if err != nil {
		t.Fatalf("Second InitTables call failed (not idempotent): %v", err)
	}

	t.Run("after reinitialization", assertPopulatedState)
}

func TestInitTables_WatchRoomTrackConstraints(t *testing.T) {
	db := newTestDBApp(t).DB

	_, err := db.Exec(`
		INSERT INTO users (name, email, password)
		VALUES ('Owner', 'owner@example.com', 'hashed')
	`)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO movies (title, file_path, file_name, size, container, mime_type, adult)
		VALUES ('Movie', '/tmp/movie.mkv', 'movie.mkv', 1, 'mkv', 'video/x-matroska', 0)
	`)
	if err != nil {
		t.Fatalf("insert movie: %v", err)
	}

	// The valid row proves the fixture reaches the constraint at all, so the
	// rejections below cannot pass on a renamed column or a missing parent.
	_, err = db.Exec(`
		INSERT INTO watch_rooms (owner_user_id, movie_id, playback_mode, audio_track, subtitle_track)
		VALUES (1, 1, 'direct', 0, 0)
	`)
	if err != nil {
		t.Fatalf("insert valid watch room: %v", err)
	}

	for column, insert := range map[string]string{
		"audio_track": `
			INSERT INTO watch_rooms (owner_user_id, movie_id, playback_mode, audio_track)
			VALUES (1, 1, 'direct', -1)`,
		"subtitle_track": `
			INSERT INTO watch_rooms (owner_user_id, movie_id, playback_mode, audio_track, subtitle_track)
			VALUES (1, 1, 'direct', 0, -1)`,
	} {
		_, err = db.Exec(insert)
		if err == nil || !strings.Contains(err.Error(), "CHECK constraint failed") {
			t.Errorf("negative %s insert error = %v, want a CHECK constraint failure", column, err)
		}
	}
}

func TestInitTables_SettingsSingleton(t *testing.T) {
	db := newTestDBApp(t).DB

	_, err := db.Exec(`
		INSERT INTO settings (tmdb_key, static_dir)
		VALUES ('first-key', 'first-static')
	`)
	if err != nil {
		t.Fatalf("insert first settings row: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO settings (tmdb_key, static_dir)
		VALUES ('second-key', 'second-static')
	`)
	if err == nil {
		t.Fatal("expected second settings insert to fail")
	}

	var count int
	err = db.QueryRow(`SELECT COUNT(*) FROM settings`).Scan(&count)
	if err != nil {
		t.Fatalf("count settings: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one settings row, got %d", count)
	}
}

func TestInitTables_UsersSchema(t *testing.T) {
	db := newTestDBApp(t).DB

	// A user inserted without an explicit is_admin must not come back as an
	// admin: the column default is the only thing standing between a fresh
	// signup and full admin rights.
	_, err := db.Exec(`
		INSERT INTO users (name, email, password)
		VALUES ('Test User', 'test@example.com', 'hashedpassword')
	`)

	if err != nil {
		t.Fatalf("Failed to insert user: %v", err)
	}

	var isAdmin bool

	err = db.QueryRow("SELECT is_admin FROM users WHERE email = ?", "test@example.com").Scan(&isAdmin)
	if err != nil {
		t.Fatalf("Failed to query user: %v", err)
	}

	if isAdmin {
		t.Error("Expected is_admin to be false by default")
	}
}

// Deleting a movie or a user must take its watch rooms and watch progress
// with it: the scanners rely on the cascade when files disappear, and account
// deletion must leave no orphans.
func TestSchema_ParentDeletesCascade(t *testing.T) {
	parents := []struct {
		name   string
		delete func(ctx context.Context, app *Application, userID, movieID int64) error
	}{
		{"movie", func(ctx context.Context, app *Application, _, movieID int64) error {
			return app.Queries.DeleteMovie(ctx, movieID)
		}},
		{"user", func(ctx context.Context, app *Application, userID, _ int64) error {
			return app.Queries.DeleteUser(ctx, userID)
		}},
	}
	for _, parent := range parents {
		t.Run(parent.name, func(t *testing.T) {
			app := setupTestApp(t)
			ctx := context.Background()
			userID, movieID := createTestUserAndMovie(t, app)
			room := createTestRoom(t, app, userID, movieID)
			seedWatchProgress(t, app, userID, movieID)

			err := parent.delete(ctx, app, userID, movieID)
			if err != nil {
				t.Fatalf("delete %s: %v", parent.name, err)
			}

			_, err = app.Queries.GetWatchRoomByID(ctx, room.ID)
			if !errors.Is(err, sql.ErrNoRows) {
				t.Errorf("watch room after %s delete: %v, want sql.ErrNoRows", parent.name, err)
			}
			_, err = app.Queries.GetMovieWatchProgress(ctx, database.GetMovieWatchProgressParams{UserID: userID, MovieID: movieID})
			if !errors.Is(err, sql.ErrNoRows) {
				t.Errorf("watch progress after %s delete: %v, want sql.ErrNoRows", parent.name, err)
			}
		})
	}
}
