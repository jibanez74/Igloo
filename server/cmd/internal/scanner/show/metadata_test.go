package show

import (
	"context"
	"database/sql"
	"testing"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/scanner/scannertest"
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
	db, q := scannertest.OpenDB(t, ":memory:?_foreign_keys=on")
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
	db, q := scannertest.OpenDB(t, ":memory:?_foreign_keys=on")
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

// applyShow replaces every relation it owns from the payload, so a refresh
// that drops a genre, company, network, creator, crew credit or video removes
// the stale row; videos without an id or key are skipped and untitled ones
// fall back to their key.
func TestApplyShowReplacesRelationsAndVideos(t *testing.T) {
	db, q := scannertest.OpenDB(t, ":memory:?_foreign_keys=on")
	ctx := context.Background()
	show, err := q.UpsertLocalShow(ctx, database.UpsertLocalShowParams{DirectoryPath: "/shows/Full", LocalName: "Full", Name: "Full"})
	if err != nil {
		t.Fatalf("create show: %v", err)
	}
	full := &tmdb.TVShow{
		ID:                  10,
		Name:                "Full",
		Genres:              []tmdb.Genre{{ID: 18, Name: "Drama"}},
		ProductionCompanies: []tmdb.ProductionCompany{{ID: 7, Name: "Studio"}},
		Networks:            []tmdb.ProductionCompany{{ID: 8, Name: "Network", LogoPath: "/n.png", OriginCountry: "US"}},
		CreatedBy:           []tmdb.TVPerson{{ID: 3, Name: "Creator"}},
		AggregateCredits: tmdb.TVAggregateCredits{
			Cast: []tmdb.TVAggregateCast{{TVPerson: tmdb.TVPerson{ID: 1, Name: "Actor"}, Roles: []tmdb.TVRole{{Character: "A", CreditID: "a", EpisodeCount: 3}}}},
			Crew: []tmdb.TVAggregateCrew{{TVPerson: tmdb.TVPerson{ID: 4, Name: "Writer"}, Department: "Writing", Jobs: []tmdb.TVRole{{Job: "Writer", CreditID: "w", EpisodeCount: 2}}}},
		},
		Videos: tmdb.TVVideos{Results: []tmdb.TmdbVideoResult{
			{ID: "v1", Key: "k1", Name: "Trailer", Site: "YouTube", Type: "Trailer"},
			{Key: "k2", Name: "No id", Site: "YouTube", Type: "Trailer"},
			{ID: "v3", Key: "k3", Name: "   ", Site: "YouTube", Type: "Featurette"},
		}},
	}
	tables := []string{"show_genres", "show_production_companies", "show_networks", "show_creators", "show_cast", "show_crew", "show_extra_videos"}
	err = applyShow(ctx, q, show.ID, full)
	if err != nil {
		t.Fatalf("applyShow: %v", err)
	}
	for _, table := range tables {
		want := 1
		if table == "show_extra_videos" {
			want = 2
		}
		count := scannertest.CountRows(t, db, "SELECT count(*) FROM "+table)
		if count != want {
			t.Errorf("%s rows = %d, want %d", table, count, want)
		}
	}
	if scannertest.CountRows(t, db, "SELECT count(*) FROM extra_videos WHERE key = 'k3' AND title = 'k3'") != 1 {
		t.Error("untitled video did not fall back to its key")
	}
	if scannertest.CountRows(t, db, "SELECT count(*) FROM show_crew WHERE job = 'Writer' AND department = 'Writing' AND episode_count = 2") != 1 {
		t.Error("crew credit not stored")
	}

	err = applyShow(ctx, q, show.ID, &tmdb.TVShow{ID: 10, Name: "Bare"})
	if err != nil {
		t.Fatalf("applyShow without relations: %v", err)
	}
	for _, table := range tables {
		count := scannertest.CountRows(t, db, "SELECT count(*) FROM "+table)
		if count != 0 {
			t.Errorf("%s rows after refresh = %d, want 0", table, count)
		}
	}
}

// TMDB never issues person id 0, so a creator without an identity aborts the
// whole apply rather than storing an artist row that can never be refreshed.
func TestApplyShowRejectsCreatorWithoutIdentity(t *testing.T) {
	_, q := scannertest.OpenDB(t, ":memory:?_foreign_keys=on")
	ctx := context.Background()
	show, err := q.UpsertLocalShow(ctx, database.UpsertLocalShowParams{DirectoryPath: "/shows/Anon", LocalName: "Anon", Name: "Anon"})
	if err != nil {
		t.Fatalf("create show: %v", err)
	}
	err = applyShow(ctx, q, show.ID, &tmdb.TVShow{ID: 10, Name: "Anon", CreatedBy: []tmdb.TVPerson{{Name: "Nobody"}}})
	if err == nil || err.Error() != "invalid TMDB person identity" {
		t.Fatalf("applyShow error = %v, want the invalid person identity", err)
	}
}

func TestApplyEpisodeStoresCrew(t *testing.T) {
	db, q := scannertest.OpenDB(t, ":memory:?_foreign_keys=on")
	ctx := context.Background()
	show, err := q.UpsertLocalShow(ctx, database.UpsertLocalShowParams{DirectoryPath: "/shows/Crew", LocalName: "Crew", Name: "Crew"})
	if err != nil {
		t.Fatalf("create show: %v", err)
	}
	season, err := q.UpsertLocalShowSeason(ctx, database.UpsertLocalShowSeasonParams{ShowID: show.ID, SeasonNumber: 1, Name: "Season 1"})
	if err != nil {
		t.Fatalf("create season: %v", err)
	}
	episode, err := q.UpsertLocalShowEpisode(ctx, database.UpsertLocalShowEpisodeParams{SeasonID: season.ID, EpisodeNumber: 1, Name: "Episode 1"})
	if err != nil {
		t.Fatalf("create episode: %v", err)
	}
	err = applyEpisode(ctx, q, episode.ID, tmdb.TVEpisode{Name: "Pilot", Crew: []tmdb.TVCrewCredit{{TVPerson: tmdb.TVPerson{ID: 5, Name: "Director"}, Department: "Directing", Job: "Director", CreditID: "d"}}})
	if err != nil {
		t.Fatalf("applyEpisode: %v", err)
	}
	if scannertest.CountRows(t, db, "SELECT count(*) FROM show_episode_crew WHERE job = 'Director' AND department = 'Directing'") != 1 {
		t.Fatal("episode crew credit not stored")
	}
}
