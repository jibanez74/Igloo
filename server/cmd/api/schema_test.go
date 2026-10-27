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
		// Reverse person credits and batched title matching.
		"idx_cast_artist",
		"idx_crew_artist",
		"idx_show_cast_artist",
		"idx_show_crew_artist",
		"idx_show_creators_artist",
		"idx_show_season_cast_artist",
		"idx_show_season_crew_artist",
		"idx_show_episode_crew_artist",
		"idx_show_episode_guest_cast_artist",
		"idx_shows_tmdb_id",

		// Ordering indexes for the paginated library listings.
		"idx_movies_title",
		"idx_movies_created_at",
		"idx_albums_alpha",
		"idx_albums_created_at",
		"idx_musicians_alpha",
		"idx_user_liked_tracks_user_created",
		"idx_shows_created_at",
		// Billing order for the capped show cast and crew lists.
		"idx_show_cast_order",
		"idx_show_crew_show_department_job",
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
		"idx_movie_production_companies_company",
		"idx_movie_extra_videos_extra",
		"idx_track_genres_genre",
		"idx_album_genres_genre",
		"idx_musician_genres_genre",
		"idx_audio_streams_language",
		"idx_subtitles_language",
		"idx_user_track_stats_last_played",
		"idx_show_cast_artist_id",
		"idx_show_crew_artist_id",
		"idx_show_creators_artist_id",
		"idx_show_season_cast_artist_id",
		"idx_show_season_crew_artist_id",
		"idx_show_episode_crew_artist_id",
		"idx_show_episode_guest_cast_artist_id",
		"idx_show_production_companies_production_company_id",
		"idx_show_networks_network_id",
		"idx_show_extra_videos_extra_video_id",
		"idx_show_season_extra_videos_extra_video_id",
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

	// The artist-sort vote groups credits by track, so track_id must follow
	// musician_id; with sort_name second every reconcile sorts the artist's credits.
	var creditVotesIndexSQL string
	err = db.QueryRow(
		"SELECT sql FROM sqlite_master WHERE type='index' AND name=?",
		"music_credit_metadata_votes",
	).Scan(&creditVotesIndexSQL)
	if err != nil {
		t.Fatalf("Failed to read music credit votes index definition: %v", err)
	}

	if !strings.Contains(creditVotesIndexSQL, "(musician_id, track_id, sort_name)") {
		t.Fatalf("Expected music credit votes index on (musician_id, track_id, sort_name), got %q", creditVotesIndexSQL)
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
	"artist":               "cast and crew rows are replaced per movie or show, artists themselves are never removed",
	"production_companies": "only upserted by the movie and show scanners",
	"extra_videos":         "only upserted by the movie and show scanners",
	"genres":               "only upserted via GetOrCreateGenre",
	"networks":             "only upserted by the show scanner",
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
		INSERT INTO music_provider_matches (provider, entity_type, entity_id, status) VALUES ('spotify', 'album', 1, 'unmatched');
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
		{"match cache", `SELECT entity_type || ':' || entity_id || ':' || status FROM music_provider_matches`, "album:1:unmatched"},
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
			_, err := app.Queries.UpsertUserPreferences(ctx, database.UpsertUserPreferencesParams{
				UserID:          userID,
				TrailersEnabled: true,
				TrailersCount:   3,
				TrailersSource:  "library",
			})
			if err != nil {
				t.Fatalf("seed user preferences: %v", err)
			}

			err = parent.delete(ctx, app, userID, movieID)
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
			if parent.name == "user" {
				_, err = app.Queries.GetUserPreferences(ctx, userID)
				if !errors.Is(err, sql.ErrNoRows) {
					t.Errorf("user preferences after user delete: %v, want sql.ErrNoRows", err)
				}
			}
		})
	}
}

// Each image column is the value the API returns and its _source column the
// provider reference writers set. A new source replaces a downloaded copy,
// writing the same source keeps it, and neither touches updated_at, which
// movie playback fingerprints include.
func TestSchema_ArtworkSourceDecidesTheReturnedImage(t *testing.T) {
	const showSeason = `
		INSERT INTO shows (id, directory_path, local_name, name) VALUES (1, '/shows/Nightfall', 'Nightfall', 'Nightfall');
		INSERT INTO show_seasons (id, show_id, season_number, name) VALUES (1, 1, 1, 'Season 1');`

	tests := []struct {
		table        string
		column       string
		insert       string // the row with id 1, its source bound to ?
		hasUpdatedAt bool
	}{
		{"movies", "poster_path", `INSERT INTO movies (id, title, file_path, file_name, size, container, mime_type, adult, poster_path_source)
			VALUES (1, 'Moonrise', '/movies/moonrise.mkv', 'moonrise.mkv', 1, 'mkv', 'video/x-matroska', false, ?)`, true},
		{"movies", "backdrop_path", `INSERT INTO movies (id, title, file_path, file_name, size, container, mime_type, adult, backdrop_path_source)
			VALUES (1, 'Moonrise', '/movies/moonrise.mkv', 'moonrise.mkv', 1, 'mkv', 'video/x-matroska', false, ?)`, true},
		{"artist", "profile", `INSERT INTO artist (id, name, tmdb_id, profile_source) VALUES (1, 'Ada', 1, ?)`, false},
		{"shows", "poster_path", `INSERT INTO shows (id, directory_path, local_name, name, poster_path_source) VALUES (1, '/shows/Nightfall', 'Nightfall', 'Nightfall', ?)`, true},
		{"shows", "backdrop_path", `INSERT INTO shows (id, directory_path, local_name, name, backdrop_path_source) VALUES (1, '/shows/Nightfall', 'Nightfall', 'Nightfall', ?)`, true},
		{"show_seasons", "poster_path", `
			INSERT INTO shows (id, directory_path, local_name, name) VALUES (1, '/shows/Nightfall', 'Nightfall', 'Nightfall');
			INSERT INTO show_seasons (id, show_id, season_number, name, poster_path_source) VALUES (1, 1, 1, 'Season 1', ?)`, true},
		{"show_episodes", "still_path", showSeason + `
			INSERT INTO show_episodes (id, season_id, episode_number, name, still_path_source) VALUES (1, 1, 1, 'Pilot', ?)`, true},
		{"networks", "logo", `INSERT INTO networks (id, tmdb_id, name, logo_source) VALUES (1, 49, 'HBO', ?)`, false},
		{"albums", "cover", `INSERT INTO albums (id, title, sort_title, cover_source) VALUES (1, 'Daylight', 'daylight', ?)`, true},
		{"musicians", "thumb", `INSERT INTO musicians (id, name, sort_name, thumb_source) VALUES (1, 'Aurora', 'aurora', ?)`, true},
	}
	for _, tt := range tests {
		t.Run(tt.table+"."+tt.column, func(t *testing.T) {
			db := newTestDBApp(t).DB
			source := tt.column + "_source"
			const downloaded = "/api/static/artwork/1/first.jpg?v=1a2b3c4d"
			const stamp = "2000-01-01 00:00:00"

			returned := func() sql.NullString {
				t.Helper()
				var value sql.NullString
				err := db.QueryRow(fmt.Sprintf(`SELECT %s FROM %s WHERE id = 1`, tt.column, tt.table)).Scan(&value)
				if err != nil {
					t.Fatalf("read %s: %v", tt.column, err)
				}
				return value
			}

			_, err := db.Exec(tt.insert, "/first.jpg")
			if err != nil {
				t.Fatalf("insert: %v", err)
			}
			got := returned()
			if got != (sql.NullString{String: "/first.jpg", Valid: true}) {
				t.Fatalf("after insert %s = %+v, want the source", tt.column, got)
			}

			_, err = db.Exec(fmt.Sprintf(`UPDATE %s SET %s = ? WHERE id = 1`, tt.table, tt.column), downloaded)
			if err != nil {
				t.Fatalf("store downloaded copy: %v", err)
			}
			if tt.hasUpdatedAt {
				_, err = db.Exec(fmt.Sprintf(`UPDATE %s SET updated_at = ? WHERE id = 1`, tt.table), stamp)
				if err != nil {
					t.Fatalf("pin updated_at: %v", err)
				}
			}

			steps := []struct {
				name   string
				source any
				want   sql.NullString
			}{
				{"same source keeps the downloaded copy", "/first.jpg", sql.NullString{String: downloaded, Valid: true}},
				{"new source replaces it", "/second.jpg", sql.NullString{String: "/second.jpg", Valid: true}},
				{"cleared source clears it", nil, sql.NullString{}},
			}
			for _, step := range steps {
				_, err = db.Exec(fmt.Sprintf(`UPDATE %s SET %s = ? WHERE id = 1`, tt.table, source), step.source)
				if err != nil {
					t.Fatalf("%s: write source: %v", step.name, err)
				}
				got := returned()
				if got != step.want {
					t.Errorf("%s: %s = %+v, want %+v", step.name, tt.column, got, step.want)
				}
				if !tt.hasUpdatedAt {
					continue
				}
				var updatedAt string
				err = db.QueryRow(fmt.Sprintf(`SELECT updated_at FROM %s WHERE id = 1`, tt.table)).Scan(&updatedAt)
				if err != nil {
					t.Fatalf("read updated_at: %v", err)
				}
				if updatedAt != stamp {
					t.Errorf("%s: updated_at = %q, want it left at %q", step.name, updatedAt, stamp)
				}
			}
		})
	}
}

// Rows written without a source, as fixtures and hand edits are, keep the
// image they were given.
func TestSchema_ArtworkWithoutSourceIsKept(t *testing.T) {
	db := newTestDBApp(t).DB

	_, err := db.Exec(`INSERT INTO movies (id, title, file_path, file_name, size, container, mime_type, adult, poster_path)
		VALUES (1, 'Moonrise', '/movies/moonrise.mkv', 'moonrise.mkv', 1, 'mkv', 'video/x-matroska', false, '/given.jpg')`)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	var poster string
	err = db.QueryRow(`SELECT poster_path FROM movies WHERE id = 1`).Scan(&poster)
	if err != nil {
		t.Fatal(err)
	}
	if poster != "/given.jpg" {
		t.Fatalf("poster_path = %q, want the value it was inserted with", poster)
	}
}

// People are upserted from every movie and show that credits them, often
// without a photo; that keeps the stored source, and with it a downloaded copy.
func TestUpsertArtist_KeepsADownloadedProfileWhileItsSourceHolds(t *testing.T) {
	app := setupTestApp(t)
	ctx := context.Background()
	const downloaded = "/api/static/people/1/ada_profile.jpg?v=1a2b3c4d"

	id, err := app.Queries.UpsertArtist(ctx, database.UpsertArtistParams{Name: "Ada", TmdbID: 1, ProfileSource: sql.NullString{String: "/ada.jpg", Valid: true}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.DB.Exec(`UPDATE artist SET profile = ? WHERE id = ?`, downloaded, id)
	if err != nil {
		t.Fatal(err)
	}

	upserts := []struct {
		name    string
		profile sql.NullString
		want    string
	}{
		{"credit without a photo", sql.NullString{}, downloaded},
		{"credit with the same photo", sql.NullString{String: "/ada.jpg", Valid: true}, downloaded},
		{"credit with a new photo", sql.NullString{String: "/ada-2.jpg", Valid: true}, "/ada-2.jpg"},
	}
	for _, upsert := range upserts {
		_, err = app.Queries.UpsertArtist(ctx, database.UpsertArtistParams{Name: "Ada", TmdbID: 1, ProfileSource: upsert.profile})
		if err != nil {
			t.Fatalf("%s: %v", upsert.name, err)
		}
		var profile string
		err = app.DB.QueryRow(`SELECT profile FROM artist WHERE id = ?`, id).Scan(&profile)
		if err != nil {
			t.Fatal(err)
		}
		if profile != upsert.want {
			t.Errorf("%s: profile = %q, want %q", upsert.name, profile, upsert.want)
		}
	}
}
