package show

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"igloo/cmd/internal/database"
	"igloo/sqlc"
)

func testDB(t *testing.T) (*sql.DB, *database.Queries) {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:?_foreign_keys=on")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	_, err = db.Exec(sqlc.Schema)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db, database.New(db)
}

func countRows(t *testing.T, db *sql.DB, table string) int {
	t.Helper()
	var n int
	err := db.QueryRow("SELECT count(*) FROM " + table).Scan(&n)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestCatalogOwnershipAndRollback(t *testing.T) {
	db, q := testDB(t)
	ctx := context.Background()
	a, err := q.UpsertLocalShow(ctx, database.UpsertLocalShowParams{DirectoryPath: "/tv/A", LocalName: "A", Name: "A"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := q.UpsertLocalShow(ctx, database.UpsertLocalShowParams{DirectoryPath: "/tv/B", LocalName: "A", Name: "A"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec("UPDATE shows SET tmdb_id = 1")
	if err != nil {
		t.Fatal(err)
	}
	if a.ID == b.ID {
		t.Fatal("folders collapsed")
	}
	season, err := q.UpsertLocalShowSeason(ctx, database.UpsertLocalShowSeasonParams{ShowID: a.ID, SeasonNumber: 1, Name: "Season 1"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := q.UpsertLocalShowSeason(ctx, database.UpsertLocalShowSeasonParams{ShowID: b.ID, SeasonNumber: 0, Name: "Specials"})
	if err != nil {
		t.Fatal(err)
	}
	ep, err := q.UpsertLocalShowEpisode(ctx, database.UpsertLocalShowEpisodeParams{SeasonID: season.ID, EpisodeNumber: 1, Name: "Episode 1"})
	if err != nil {
		t.Fatal(err)
	}
	ep2, err := q.UpsertLocalShowEpisode(ctx, database.UpsertLocalShowEpisodeParams{SeasonID: season.ID, EpisodeNumber: 2, Name: "Episode 2"})
	if err != nil {
		t.Fatal(err)
	}
	foreignEpisode, err := q.UpsertLocalShowEpisode(ctx, database.UpsertLocalShowEpisodeParams{SeasonID: other.ID, EpisodeNumber: 1, Name: "Other episode"})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/tv/A/Season 1/S01E01E02.mkv", "/tv/A/Season 1/S01E01.mkv"} {
		f, err := q.UpsertShowFile(ctx, database.UpsertShowFileParams{SeasonID: season.ID, FilePath: path, FileName: path, Container: "mkv", MimeType: "video/x-matroska"})
		if err != nil {
			t.Fatal(err)
		}
		err = q.LinkShowEpisodeFile(ctx, database.LinkShowEpisodeFileParams{EpisodeID: ep.ID, FileID: f.ID, SeasonID: season.ID})
		if err != nil {
			t.Fatal(err)
		}
		err = q.LinkShowEpisodeFile(ctx, database.LinkShowEpisodeFileParams{EpisodeID: ep2.ID, FileID: f.ID, SeasonID: season.ID, EpisodeOrder: 1})
		if err != nil {
			t.Fatal(err)
		}
		// Neither attempt collides with an existing relationship key: these
		// failures must come from ownership constraints, not uniqueness.
		for _, seasonID := range []int64{season.ID, other.ID} {
			err = q.LinkShowEpisodeFile(ctx, database.LinkShowEpisodeFileParams{EpisodeID: foreignEpisode.ID, FileID: f.ID, SeasonID: seasonID, EpisodeOrder: 2})
			if err == nil {
				t.Fatal("cross-season link accepted")
			}
		}
	}
	for _, statement := range []string{
		"INSERT INTO show_seasons(show_id,season_number,name) VALUES (1,-1,'bad')",
		"INSERT INTO show_episodes(season_id,episode_number,name) VALUES (1,0,'bad')",
		"INSERT INTO show_episode_files VALUES (1,1,1,0)",
		"INSERT INTO show_seasons(show_id,season_number,name) VALUES (1,1,'duplicate')",
	} {
		_, err = db.Exec(statement)
		if err == nil {
			t.Fatalf("accepted invalid statement: %s", statement)
		}
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tx.Exec("DELETE FROM show_files WHERE id=1")
	if err != nil {
		t.Fatal(err)
	}
	err = tx.Rollback()
	if err != nil {
		t.Fatal(err)
	}
	if countRows(t, db, "show_episode_files") != 4 {
		t.Fatal("rollback lost relationships")
	}
	_, err = db.Exec("DELETE FROM show_files WHERE id=1")
	if err != nil {
		t.Fatal(err)
	}
	err = q.PruneShowEpisodes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if countRows(t, db, "show_episodes") != 2 || countRows(t, db, "show_episode_files") != 2 {
		t.Fatal("deleting a copy deleted logical episodes")
	}
	_, err = db.Exec("DELETE FROM shows")
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"show_seasons", "show_episodes", "show_files", "show_episode_files"} {
		if countRows(t, db, table) != 0 {
			t.Fatalf("cascade left %s", table)
		}
	}
}
