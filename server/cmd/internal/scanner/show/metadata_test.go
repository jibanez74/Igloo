package show

import (
	"context"
	"database/sql"
	"testing"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/tmdb"
)

// applyShow, applySeason and applyEpisode mix two null policies on purpose:
// helpers.NullInt64/NullString turn a zero value into NULL, while the TMDB
// rating and count fields store a genuine 0. Nothing asserted that before, and
// the failure mode is a wrong value in the database rather than a compile
// error, so these tests pin the persisted outcome of both policies.

func nullableColumn(t *testing.T, db *sql.DB, query string, id int64) any {
	t.Helper()

	var value any
	err := db.QueryRow(query, id).Scan(&value)
	if err != nil {
		t.Fatalf("read %q: %v", query, err)
	}

	return value
}

func TestApplyShowStoresZeroRatingsAndNullsMeaninglessZeros(t *testing.T) {
	db, q := testDB(t)
	ctx := context.Background()

	show, err := q.UpsertLocalShow(ctx, database.UpsertLocalShowParams{
		DirectoryPath: "/shows/Unrated",
		LocalName:     "Unrated",
		Name:          "Unrated",
	})
	if err != nil {
		t.Fatalf("create show: %v", err)
	}

	// Everything TMDB could report as zero, reported as zero.
	err = applyShow(ctx, q, show.ID, &tmdb.TVShow{Name: "Unrated"})
	if err != nil {
		t.Fatalf("applyShow: %v", err)
	}

	// A show nobody has rated genuinely has 0 votes and a 0.0 average. That is
	// a fact about the show, not a missing field, so it must persist as 0.
	stored := []struct {
		column string
		want   any
	}{
		{"vote_average", float64(0)},
		{"vote_count", int64(0)},
		{"popularity", float64(0)},
		{"tmdb_season_count", int64(0)},
		{"tmdb_episode_count", int64(0)},
	}
	for _, tt := range stored {
		got := nullableColumn(t, db, "SELECT "+tt.column+" FROM shows WHERE id = ?", show.ID)
		if got != tt.want {
			t.Errorf("shows.%s = %#v, want %#v", tt.column, got, tt.want)
		}
	}

	// TMDB never issues id 0, and an empty string is an absent field, so these
	// go through the helpers and land as NULL.
	nulled := []string{"tmdb_id", "imdb_id", "overview", "tagline", "homepage"}
	for _, column := range nulled {
		got := nullableColumn(t, db, "SELECT "+column+" FROM shows WHERE id = ?", show.ID)
		if got != nil {
			t.Errorf("shows.%s = %#v, want NULL", column, got)
		}
	}
}

func TestApplySeasonAndEpisodeStoreZeroRatingsAndNullUnknownRuntime(t *testing.T) {
	db, q := testDB(t)
	ctx := context.Background()

	show, err := q.UpsertLocalShow(ctx, database.UpsertLocalShowParams{
		DirectoryPath: "/shows/Unrated",
		LocalName:     "Unrated",
		Name:          "Unrated",
	})
	if err != nil {
		t.Fatalf("create show: %v", err)
	}

	season, err := q.UpsertLocalShowSeason(ctx, database.UpsertLocalShowSeasonParams{
		ShowID:       show.ID,
		SeasonNumber: 1,
		Name:         "Season 1",
	})
	if err != nil {
		t.Fatalf("create season: %v", err)
	}

	episode, err := q.UpsertLocalShowEpisode(ctx, database.UpsertLocalShowEpisodeParams{
		SeasonID:      season.ID,
		EpisodeNumber: 1,
		Name:          "Episode 1",
	})
	if err != nil {
		t.Fatalf("create episode: %v", err)
	}

	err = applySeason(ctx, q, season.ID, &tmdb.TVSeason{Name: "Season 1"})
	if err != nil {
		t.Fatalf("applySeason: %v", err)
	}

	err = applyEpisode(ctx, q, episode.ID, tmdb.TVEpisode{Name: "Episode 1"})
	if err != nil {
		t.Fatalf("applyEpisode: %v", err)
	}

	got := nullableColumn(t, db, "SELECT vote_average FROM show_seasons WHERE id = ?", season.ID)
	if got != float64(0) {
		t.Errorf("show_seasons.vote_average = %#v, want 0", got)
	}

	// A season with no episodes listed reports 0, which is a count, not an
	// absence.
	got = nullableColumn(t, db, "SELECT tmdb_episode_count FROM show_seasons WHERE id = ?", season.ID)
	if got != int64(0) {
		t.Errorf("show_seasons.tmdb_episode_count = %#v, want 0", got)
	}

	got = nullableColumn(t, db, "SELECT vote_count FROM show_episodes WHERE id = ?", episode.ID)
	if got != int64(0) {
		t.Errorf("show_episodes.vote_count = %#v, want 0", got)
	}

	// A 0-minute episode does not exist, so a 0 runtime means TMDB did not
	// report one. That is the case the helpers exist for.
	got = nullableColumn(t, db, "SELECT tmdb_runtime FROM show_episodes WHERE id = ?", episode.ID)
	if got != nil {
		t.Errorf("show_episodes.tmdb_runtime = %#v, want NULL", got)
	}
}
