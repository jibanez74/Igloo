package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	moviescanner "igloo/cmd/internal/scanner/movie"
	"igloo/cmd/internal/tmdb"
)

func TestIdentifyMovie_HTTPPersistsTmdbMetadataAndRelationships(t *testing.T) {
	app := setupTestApp(t)

	ctx := context.Background()
	movieID, err := app.Queries.UpsertMovie(ctx, database.UpsertMovieParams{
		Title:          "Unknown",
		RunTime:        helpers.NullInt64(120),
		Duration:       helpers.NullFloat64(7200),
		AudienceRating: helpers.NullFloat64(9),
		FilePath:       "/movies/unknown.mkv",
		FileName:       "unknown.mkv",
		Size:           1,
		Container:      "mkv",
		MimeType:       "video/x-matroska",
		Adult:          false,
	})
	if err != nil {
		t.Fatalf("insert movie: %v", err)
	}
	movie, err := app.Queries.GetMovieByID(ctx, movieID)
	if err != nil {
		t.Fatal(err)
	}

	err = app.Queries.MarkMovieTmdbRetry(ctx, movie.ID)
	if err != nil {
		t.Fatal(err)
	}
	tmdbDetails := tmdbMovieFromJSON(t, `{
		"id": 603,
		"title": "The Matrix",
		"release_date": "1999-03-31",
		"poster_path": "/poster.jpg",
		"original_language": "en",
		"runtime": 136,
		"genres": [{"id": 28, "name": "Action"}],
		"credits": {
			"cast": [{"id": 6384, "name": "Keanu Reeves", "character": "Neo", "order": 0}],
			"crew": [{"id": 9339, "name": "Lana Wachowski", "job": "Director", "department": "Directing"}]
		},
		"videos": {"results": [{"id": "trailer", "key": "abc", "name": "Trailer", "site": "YouTube", "type": "Trailer"}]}
	}`)
	app.Tmdb = &stubTmdbClient{
		detailMovies: map[int]tmdb.TmdbMovie{603: tmdbDetails},
	}

	actor := createTestUser(t, app, "Actor", "actor@example.com", true)
	router := authenticatedRouter(t, app, actor.ID)
	artwork := &artworkRequests{}
	app.Artwork = artwork

	w := httptest.NewRecorder()
	req := newOpenAPIJSONRequest(http.MethodPut, "/api/movies/"+strconv.FormatInt(movie.ID, 10)+"/identify", `{"tmdb_id":603}`)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	assertOpenAPIExchange(t, "identifyMovie", req, w)
	if artwork.count.Load() != 1 {
		t.Fatalf("artwork passes requested = %d, want one for the new match's artwork", artwork.count.Load())
	}

	updated, err := app.Queries.GetMovieByID(ctx, movie.ID)
	if err != nil {
		t.Fatalf("get updated movie: %v", err)
	}
	if updated.Title != "The Matrix" || !updated.TmdbID.Valid || updated.TmdbID.Int64 != 603 ||
		!updated.Year.Valid || updated.Year.Int64 != 1999 || !updated.RunTime.Valid || updated.RunTime.Int64 != 120 || updated.AudienceRating.Float64 != 9 || updated.Duration.Float64 != 7200 {
		t.Fatalf("updated movie = %+v, want TMDB metadata", updated)
	}

	var retries int
	err = app.DB.QueryRow("SELECT count(*) FROM movie_tmdb_retries").Scan(&retries)
	if err != nil || retries != 0 {
		t.Fatalf("retry cleanup: %d %v", retries, err)
	}
	genres, err := app.Queries.GetGenresByMovieID(ctx, movie.ID)
	if err != nil {
		t.Fatalf("get genres: %v", err)
	}
	if movieGenreTags(genres) != "Action" {
		t.Fatalf("genres = %+v, want Action", genres)
	}

	cast, err := app.Queries.GetCastByMovieID(ctx, movie.ID)
	if err != nil {
		t.Fatalf("get cast: %v", err)
	}
	if len(cast) != 1 || cast[0].ArtistName != "Keanu Reeves" {
		t.Fatalf("cast = %+v, want Keanu Reeves", cast)
	}

	crew, err := app.Queries.GetCrewByMovieID(ctx, movie.ID)
	if err != nil {
		t.Fatalf("get crew: %v", err)
	}
	if len(crew) != 1 || crew[0].ArtistName != "Lana Wachowski" {
		t.Fatalf("crew = %+v, want Lana Wachowski", crew)
	}

	extras, err := app.Queries.GetMovieExtraVideos(ctx, movie.ID)
	if err != nil {
		t.Fatalf("get extras: %v", err)
	}
	if len(extras) != 1 || extras[0].Title != "Trailer" {
		t.Fatalf("extras = %+v, want Trailer", extras)
	}
}

func TestIdentifyMovie_HTTPErrorPaths(t *testing.T) {
	app := setupTestApp(t)

	actor := createTestUser(t, app, "Actor", "actor@example.com", true)
	router := authenticatedRouter(t, app, actor.ID)

	unconfigured := newOpenAPIJSONRequest(http.MethodPut, "/api/movies/1/identify", `{"tmdb_id":603}`)
	serveOpenAPIExchange(t, router, "identifyMovie", unconfigured, http.StatusServiceUnavailable)

	app.Tmdb = &stubTmdbClient{detailMovies: map[int]tmdb.TmdbMovie{603: {TmdbID: 603, Title: "The Matrix"}}}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/movies/bad/identify", strings.NewReader(`{"tmdb_id":603}`))
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid movie id status = %d, want 400", w.Code)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/api/movies/1/identify", strings.NewReader(`{"tmdb_id":0}`))
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid tmdb id status = %d, want 400", w.Code)
	}

	// The stub fails TMDB id 604, so a 404 proves the movie was checked first.
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/api/movies/999/identify", strings.NewReader(`{"tmdb_id":604}`))
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("missing movie status = %d, want 404", w.Code)
	}

	// For a movie that exists, an id TMDB does not know is the caller's bad
	// field, not a server failure.
	_, movieID := createTestUserAndMovie(t, app)
	unknown := newOpenAPIJSONRequest(http.MethodPut, "/api/movies/"+strconv.FormatInt(movieID, 10)+"/identify", `{"tmdb_id":604}`)
	w = serveOpenAPIExchange(t, router, "identifyMovie", unknown, http.StatusBadRequest)
	if !strings.Contains(w.Body.String(), "tmdb_id not found on TMDB") {
		t.Fatalf("unknown tmdb_id body = %s", w.Body.String())
	}
}

func TestUpdateMovieMetadata_PreservesOmittedNullableFields(t *testing.T) {
	app := setupTestApp(t)

	ctx := context.Background()
	movieID, err := app.Queries.UpsertMovie(ctx, database.UpsertMovieParams{
		Title:         "Original",
		FilePath:      "/movies/original-preserve.mkv",
		FileName:      "original-preserve.mkv",
		Size:          1,
		Container:     "mkv",
		MimeType:      "video/x-matroska",
		Adult:         false,
		Overview:      helpers.NullString("Keep overview"),
		ReleaseDate:   helpers.NullString("2001-01-01"),
		Year:          helpers.NullInt64(2001),
		Certification: sql.NullString{String: "PG", Valid: true},
	})
	if err != nil {
		t.Fatalf("insert movie: %v", err)
	}
	movie, err := app.Queries.GetMovieByID(ctx, movieID)
	if err != nil {
		t.Fatal(err)
	}

	actor := createTestUser(t, app, "Actor", "actor@example.com", true)
	router := authenticatedRouter(t, app, actor.ID)

	w := httptest.NewRecorder()
	req := newOpenAPIJSONRequest(http.MethodPatch, "/api/movies/"+strconv.FormatInt(movie.ID, 10), `{"title":"Updated"}`)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	assertOpenAPIExchange(t, "updateMovieMetadata", req, w)

	updated, err := app.Queries.GetMovieByID(ctx, movie.ID)
	if err != nil {
		t.Fatalf("get updated movie: %v", err)
	}
	if updated.Title != "Updated" ||
		!updated.Overview.Valid || updated.Overview.String != "Keep overview" ||
		!updated.Year.Valid || updated.Year.Int64 != 2001 ||
		!updated.Certification.Valid || updated.Certification.String != "PG" {
		t.Fatalf("updated movie = %+v, want omitted nullable fields preserved", updated)
	}
}

// The edit starts from the stored provider references, not the returned
// values, so editing other fields of a movie whose artwork was downloaded
// neither stores the local URL as its source nor drops the downloaded copy.
func TestUpdateMovieMetadata_KeepsDownloadedArtwork(t *testing.T) {
	app := setupTestApp(t)

	ctx := context.Background()
	movieID, err := app.Queries.UpsertMovie(ctx, database.UpsertMovieParams{
		Title:              "Original",
		FilePath:           "/movies/original-artwork.mkv",
		FileName:           "original-artwork.mkv",
		Size:               1,
		Container:          "mkv",
		MimeType:           "video/x-matroska",
		PosterPathSource:   helpers.NullString("/poster.jpg"),
		BackdropPathSource: helpers.NullString("/backdrop.jpg"),
	})
	if err != nil {
		t.Fatalf("insert movie: %v", err)
	}
	const posterCopy = "/api/static/movies/1/original_poster.jpg?v=1a2b3c4d"
	_, err = app.DB.Exec(`UPDATE movies SET poster_path = ? WHERE id = ?`, posterCopy, movieID)
	if err != nil {
		t.Fatal(err)
	}

	actor := createTestUser(t, app, "Editor", "editor@example.com", true)
	router := authenticatedRouter(t, app, actor.ID)
	path := "/api/movies/" + strconv.FormatInt(movieID, 10)
	artwork := &artworkRequests{}
	app.Artwork = artwork

	// A client may send back every field it was shown, the copy's URL
	// included, with only the title changed. A form opened before a settings
	// save reverted the columns sends a copy's URL that differs from the one
	// the movie returns now; no copy's URL is a source.
	staleCopies := `{"title":"Updated","poster_path":"/api/static/movies/2/other_poster.jpg?v=9","backdrop_path":"/api/static/movies/1/original_backdrop.jpg?v=5"}`
	for _, body := range []string{`{"title":"Updated"}`, fmt.Sprintf(`{"title":"Updated","poster_path":%q}`, posterCopy), staleCopies} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, newOpenAPIJSONRequest(http.MethodPatch, path, body))
		if w.Code != http.StatusOK {
			t.Fatalf("title edit status = %d, body = %s", w.Code, w.Body.String())
		}
		updated, err := app.Queries.GetMovieByID(ctx, movieID)
		if err != nil {
			t.Fatal(err)
		}
		if updated.PosterPath.String != posterCopy || updated.PosterPathSource.String != "/poster.jpg" {
			t.Fatalf("after editing %s poster = %q from %q, want the copy of /poster.jpg kept", body, updated.PosterPath.String, updated.PosterPathSource.String)
		}
		if updated.BackdropPath.String != "/backdrop.jpg" || updated.BackdropPathSource.String != "/backdrop.jpg" {
			t.Fatalf("after editing %s backdrop = %q from %q, want /backdrop.jpg kept", body, updated.BackdropPath.String, updated.BackdropPathSource.String)
		}
	}
	if artwork.count.Load() != 3 {
		t.Fatalf("artwork passes requested = %d, want one per edit", artwork.count.Load())
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, newOpenAPIJSONRequest(http.MethodPatch, path, `{"poster_path":"/new-poster.jpg"}`))
	if w.Code != http.StatusOK {
		t.Fatalf("poster edit status = %d, body = %s", w.Code, w.Body.String())
	}
	updated, err := app.Queries.GetMovieByID(ctx, movieID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.PosterPath.String != "/new-poster.jpg" || updated.PosterPathSource.String != "/new-poster.jpg" {
		t.Fatalf("after a poster edit poster = %q from %q, want the new path", updated.PosterPath.String, updated.PosterPathSource.String)
	}
	if updated.BackdropPath.String != "/backdrop.jpg" {
		t.Fatalf("backdrop = %q, want it untouched", updated.BackdropPath.String)
	}
}

type identifyDuringScanTmdb struct {
	tmdb.TmdbInterface
	entered chan struct{}
	release chan struct{}
}

// The scanner reaches the blocking detail lookup through a search, because a
// movie that already carries a confirmed match is never re-enriched by a scan.
func (c *identifyDuringScanTmdb) SearchMoviesByTitleAndYear(_ context.Context, title string, _ ...int) ([]tmdb.TmdbMovie, error) {
	return []tmdb.TmdbMovie{{TmdbID: 42, Title: title}}, nil
}

func (c *identifyDuringScanTmdb) GetTmdbMovieByID(ctx context.Context, movie *tmdb.TmdbMovie) error {
	if movie.TmdbID == 42 {
		close(c.entered)
		select {
		case <-c.release:
		case <-ctx.Done():
			return ctx.Err()
		}
		*movie = tmdb.TmdbMovie{TmdbID: 42, Title: "Stale scan response"}
	} else {
		*movie = tmdb.TmdbMovie{TmdbID: 99, Title: "Manual identity", Runtime: 999}
	}
	return nil
}

func TestIdentifyMovieDuringScannerLookup(t *testing.T) {
	for _, changed := range []bool{false, true} {
		t.Run(strconv.FormatBool(changed), func(t *testing.T) {
			app := setupTestApp(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			root := t.TempDir()
			path := filepath.Join(root, "movie.mkv")
			err := os.WriteFile(path, []byte("movie"), 0600)
			if err != nil {
				t.Fatal(err)
			}
			// Seeded without a match so the scan below actually enriches it.
			localID, err := app.Queries.UpsertMovie(ctx, database.UpsertMovieParams{Title: "Original", FilePath: path, FileName: "movie.mkv", Container: "mkv", MimeType: "video/x-matroska", AudienceRating: helpers.NullFloat64(9)})
			if err != nil {
				t.Fatal(err)
			}
			local, err := app.Queries.GetMovieByID(ctx, localID)
			if err != nil {
				t.Fatal(err)
			}
			var wait sync.WaitGroup
			probe := &cleanupRescanProbe{}
			deps := moviescanner.Dependencies{DB: app.DB, Queries: app.Queries, Logger: app.Logger, Ffprobe: probe, ScanContext: ctx, Wait: &wait, ScannerDBMu: &app.ScannerDBMu, Now: func() time.Time { return time.Now().Add(2 * time.Minute) }, CurrentMoviesDirectory: func() sql.NullString { return helpers.NullString(root) }}
			initial := moviescanner.New(deps)
			initial.Start()
			wait.Wait()
			if changed {
				err = os.WriteFile(path, []byte("changed movie"), 0600)
				if err != nil {
					t.Fatal(err)
				}
			}
			client := &identifyDuringScanTmdb{entered: make(chan struct{}), release: make(chan struct{})}
			app.Tmdb = client
			deps.Tmdb = client
			s := moviescanner.New(deps)
			s.Start()
			defer func() { cancel(); wait.Wait() }()
			select {
			case <-client.entered:
			case <-time.After(5 * time.Second):
				t.Fatal("scanner did not start identity lookup")
			}
			actor := createTestUser(t, app, "Actor", "actor@example.com", true)
			router := authenticatedRouter(t, app, actor.ID)
			recorder := httptest.NewRecorder()
			req := newOpenAPIJSONRequest(http.MethodPut, "/api/movies/"+strconv.FormatInt(local.ID, 10)+"/identify", `{"tmdb_id":99}`)
			done := make(chan struct{})
			go func() { router.ServeHTTP(recorder, req); close(done) }()
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("Identify blocked on scanner's remote request")
			}
			if recorder.Code != http.StatusOK {
				t.Fatalf("identify status=%d body=%s", recorder.Code, recorder.Body.String())
			}
			close(client.release)
			wait.Wait()
			current, err := app.Queries.GetMovieByID(ctx, local.ID)
			if err != nil {
				t.Fatal(err)
			}
			if current.Title != "Manual identity" || current.TmdbID.Int64 != 99 || current.AudienceRating.Float64 != 9 || current.Duration.Float64 != 120 || current.RunTime.Int64 != 2 {
				t.Fatalf("scan overwrote Identify: %+v", current)
			}
			wantCalls := 1
			if changed {
				wantCalls = 2
			}
			if probe.calls != wantCalls {
				t.Fatalf("probes=%d want=%d", probe.calls, wantCalls)
			}
			if changed && current.Size != 13 {
				t.Fatal("scan lost valid technical update")
			}
			var pending int
			err = app.DB.QueryRow("SELECT count(*) FROM movie_tmdb_retries").Scan(&pending)
			if err != nil || pending != 0 {
				t.Fatalf("retry cleanup=%d %v", pending, err)
			}
		})
	}
}

func TestIdentifyMovieRollsBackRetryCleanupFailure(t *testing.T) {
	app := setupTestApp(t)
	ctx := context.Background()
	beforeID, err := app.Queries.UpsertMovie(ctx, database.UpsertMovieParams{Title: "Original", FilePath: "/rollback.mkv", FileName: "rollback.mkv", Container: "mkv", MimeType: "video/x-matroska", TmdbID: helpers.NullInt64(42), RunTime: helpers.NullInt64(120), AudienceRating: helpers.NullFloat64(9)})
	if err != nil {
		t.Fatal(err)
	}
	before, err := app.Queries.GetMovieByID(ctx, beforeID)
	if err != nil {
		t.Fatal(err)
	}
	err = app.Queries.MarkMovieTmdbRetry(ctx, before.ID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.DB.Exec("CREATE TRIGGER reject_retry_cleanup BEFORE DELETE ON movie_tmdb_retries BEGIN SELECT RAISE(ABORT,'failure'); END")
	if err != nil {
		t.Fatal(err)
	}
	app.Tmdb = &stubTmdbClient{detailMovies: map[int]tmdb.TmdbMovie{99: {TmdbID: 99, Title: "New identity", Genres: []tmdb.Genre{{ID: 1, Name: "Drama"}}}}}
	actor := createTestUser(t, app, "Actor", "actor@example.com", true)
	router := authenticatedRouter(t, app, actor.ID)
	w := httptest.NewRecorder()
	req := newOpenAPIJSONRequest(http.MethodPut, "/api/movies/"+strconv.FormatInt(before.ID, 10)+"/identify", `{"tmdb_id":99}`)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", w.Code)
	}
	after, err := app.Queries.GetMovieByID(ctx, before.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("failed Identify published metadata")
	}
	pending, err := app.Queries.HasMovieTmdbRetry(ctx, before.ID)
	if err != nil || !pending {
		t.Fatalf("lost retry: %v %v", pending, err)
	}
	genres, err := app.Queries.GetGenresByMovieID(ctx, before.ID)
	if err != nil || len(genres) != 0 {
		t.Fatalf("partial relationships: %+v %v", genres, err)
	}
}

func TestUpdateMovieMetadata_MissingUpdateRollsBack(t *testing.T) {
	app := setupTestApp(t)
	movieID := createTestMovie(t, app, "Original", "/movies/update-rollback.mkv")
	// A row removed inside the UPDATE must retain the old RETURNING failure
	// behavior, including rollback of the trigger's deletion.
	_, err := app.DB.Exec(`CREATE TRIGGER remove_movie_before_update BEFORE UPDATE ON movies
	BEGIN DELETE FROM movies WHERE id = OLD.id; END`)
	if err != nil {
		t.Fatal(err)
	}
	actor := createTestUser(t, app, "Actor", "actor@example.com", true)
	router := authenticatedRouter(t, app, actor.ID)
	request := newOpenAPIJSONRequest(http.MethodPatch, fmt.Sprintf("/api/movies/%d", movieID), `{"title":"Changed"}`)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500: %s", response.Code, response.Body.String())
	}
	movie, err := app.Queries.GetMovieByID(context.Background(), movieID)
	if err != nil {
		t.Fatal(err)
	}
	if movie.Title != "Original" {
		t.Fatalf("failed update changed stored title to %q", movie.Title)
	}
}
