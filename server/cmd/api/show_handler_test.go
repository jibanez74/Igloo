package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"igloo/cmd/internal/database"
)

// The empty-library table in library_handler_contract_test.go always validates an
// empty array, so this is the only coverage that checks a LatestShow element against
// the contract. The unenriched row exercises the nullable poster and premiere year.
func TestGetLatestShows_ConformsToOpenAPIWithRows(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	user := createTestUser(t, app, "Shows Contract User", "shows-contract@example.com", false)

	_, err := app.DB.Exec(`
 INSERT INTO shows (directory_path, local_name, premiere_year, name, poster_path)
 VALUES ('/shows/Contract Show (2026)', 'Contract Show', 2026, 'Contract Show', '/contract-show.jpg');
 INSERT INTO shows (directory_path, local_name, name)
 VALUES ('/shows/Unenriched Show', 'Unenriched Show', 'Unenriched Show');
 `)
	if err != nil {
		t.Fatalf("seed shows: %v", err)
	}

	app.InitSession()
	app.InitRouter()
	cookie := newAuthSessionCookie(t, app, user.ID)

	request := httptest.NewRequest(http.MethodGet, "/api/shows/latest", nil)
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	app.Router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusOK, response.Body.String())
	}

	// Without this the seeded rows could silently fail to insert and the contract
	// assertion below would vacuously validate an empty array.
	var body struct {
		Data struct {
			Shows []struct {
				Name string `json:"name"`
			} `json:"shows"`
		} `json:"data"`
	}
	err = json.Unmarshal(response.Body.Bytes(), &body)
	if err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(body.Data.Shows) != 2 {
		t.Fatalf("shows length = %d, want 2, body = %s", len(body.Data.Shows), response.Body.String())
	}
	// Newest first: the unenriched show was inserted last.
	if body.Data.Shows[0].Name != "Unenriched Show" {
		t.Fatalf("first show = %q, want %q", body.Data.Shows[0].Name, "Unenriched Show")
	}

	assertOpenAPIExchange(t, "getLatestShows", request, response)
}

// seedContractShow builds a show through the scanner's own upserts, so the
// fixture cannot drift from what a real scan produces. It returns the show id.
//
// Shape: two seasons plus specials. Season 1 is complete (2 of 2 episodes),
// season 2 is partial (1 episode present against a TMDB count of 8), and the
// specials season sorts last despite having the lowest number.
func seedContractShow(t *testing.T, app *Application) int64 {
	t.Helper()

	ctx := context.Background()
	q := app.Queries

	show, err := q.UpsertLocalShow(ctx, database.UpsertLocalShowParams{
		DirectoryPath: "/shows/Contract Detail Show (2024)",
		LocalName:     "Contract Detail Show",
		PremiereYear:  sql.NullInt64{Int64: 2024, Valid: true},
		Name:          "Contract Detail Show",
	})
	if err != nil {
		t.Fatalf("upsert show: %v", err)
	}

	err = q.UpdateShowMetadata(ctx, database.UpdateShowMetadataParams{
		Name:             "Contract Detail Show",
		TmdbID:           sql.NullInt64{Int64: 90210, Valid: true},
		OriginalName:     sql.NullString{String: "Contract Detail Show", Valid: true},
		Overview:         sql.NullString{String: "A show that exists to validate the details contract.", Valid: true},
		Tagline:          sql.NullString{String: "Every field, once.", Valid: true},
		Language:         sql.NullString{String: "en", Valid: true},
		OriginCountries:  sql.NullString{String: "US", Valid: true},
		FirstAirDate:     sql.NullString{String: "2024-03-01", Valid: true},
		LastAirDate:      sql.NullString{String: "2026-05-20", Valid: true},
		Status:           sql.NullString{String: "Returning Series", Valid: true},
		Type:             sql.NullString{String: "Scripted", Valid: true},
		PosterPath:       sql.NullString{String: "/contract-detail-poster.jpg", Valid: true},
		BackdropPath:     sql.NullString{String: "/contract-detail-backdrop.jpg", Valid: true},
		VoteAverage:      sql.NullFloat64{Float64: 8.4, Valid: true},
		VoteCount:        sql.NullInt64{Int64: 1200, Valid: true},
		Certification:    sql.NullString{String: "TV-14", Valid: true},
		TmdbSeasonCount:  sql.NullInt64{Int64: 2, Valid: true},
		TmdbEpisodeCount: sql.NullInt64{Int64: 10, Valid: true},
		ID:               show.ID,
	})
	if err != nil {
		t.Fatalf("update show metadata: %v", err)
	}

	// Specials are created first so ordering cannot pass by insertion accident.
	seasons := []struct {
		number       int64
		name         string
		tmdbEpisodes int64
		episodes     int64
	}{
		{0, "Specials", 1, 1},
		{1, "Season 1", 2, 2},
		{2, "Season 2", 8, 1},
	}

	for _, s := range seasons {
		season, err := q.UpsertLocalShowSeason(ctx, database.UpsertLocalShowSeasonParams{
			ShowID:       show.ID,
			SeasonNumber: s.number,
			Name:         s.name,
		})
		if err != nil {
			t.Fatalf("upsert season %d: %v", s.number, err)
		}

		err = q.UpdateShowSeasonMetadata(ctx, database.UpdateShowSeasonMetadataParams{
			Name:             s.name,
			TmdbID:           sql.NullInt64{Int64: 5000 + s.number, Valid: true},
			Overview:         sql.NullString{String: s.name + " overview.", Valid: true},
			AirDate:          sql.NullString{String: "2024-03-01", Valid: true},
			PosterPath:       sql.NullString{String: "/season.jpg", Valid: true},
			VoteAverage:      sql.NullFloat64{Float64: 7.9, Valid: true},
			TmdbEpisodeCount: sql.NullInt64{Int64: s.tmdbEpisodes, Valid: true},
			ID:               season.ID,
		})
		if err != nil {
			t.Fatalf("update season %d metadata: %v", s.number, err)
		}

		for n := int64(1); n <= s.episodes; n++ {
			episode, err := q.UpsertLocalShowEpisode(ctx, database.UpsertLocalShowEpisodeParams{
				SeasonID:      season.ID,
				EpisodeNumber: n,
				Name:          fmt.Sprintf("%s Episode %d", s.name, n),
			})
			if err != nil {
				t.Fatalf("upsert episode s%de%d: %v", s.number, n, err)
			}

			// The last episode of season 1 stays unenriched, so the contract is
			// checked against null overview, still, runtime, and votes too.
			if s.number == 1 && n == s.episodes {
				continue
			}

			err = q.UpdateShowEpisodeMetadata(ctx, database.UpdateShowEpisodeMetadataParams{
				Name:        fmt.Sprintf("%s Episode %d", s.name, n),
				TmdbID:      sql.NullInt64{Int64: 70000 + s.number*100 + n, Valid: true},
				Overview:    sql.NullString{String: "Episode overview.", Valid: true},
				AirDate:     sql.NullString{String: "2024-03-08", Valid: true},
				StillPath:   sql.NullString{String: "/still.jpg", Valid: true},
				TmdbRuntime: sql.NullInt64{Int64: 47, Valid: true},
				VoteAverage: sql.NullFloat64{Float64: 8.1, Valid: true},
				VoteCount:   sql.NullInt64{Int64: 220, Valid: true},
				ID:          episode.ID,
			})
			if err != nil {
				t.Fatalf("update episode s%de%d metadata: %v", s.number, n, err)
			}
		}
	}

	leadArtist, err := q.UpsertArtist(ctx, database.UpsertArtistParams{
		Name:    "Ada Contract",
		TmdbID:  4242,
		Profile: sql.NullString{String: "/ada.jpg", Valid: true},
	})
	if err != nil {
		t.Fatalf("upsert lead artist: %v", err)
	}

	// No profile: exercises the nullable artist_profile branch.
	guestArtist, err := q.UpsertArtist(ctx, database.UpsertArtistParams{
		Name:   "Bo Contract",
		TmdbID: 4243,
	})
	if err != nil {
		t.Fatalf("upsert guest artist: %v", err)
	}

	// Two credits for one artist: TMDB aggregate credits do this, and it is why
	// credit_id rather than artist_id is the row identity.
	castRows := []database.CreateShowCastParams{
		{ShowID: show.ID, ArtistID: leadArtist, Character: "Captain", CastOrder: 0, CreditID: "credit-lead-1", EpisodeCount: 10},
		{ShowID: show.ID, ArtistID: leadArtist, Character: "Captain's Double", CastOrder: 1, CreditID: "credit-lead-2", EpisodeCount: 2},
		{ShowID: show.ID, ArtistID: guestArtist, Character: "Navigator", CastOrder: 2, CreditID: "credit-guest-1", EpisodeCount: 4},
	}
	for _, row := range castRows {
		if err := q.CreateShowCast(ctx, row); err != nil {
			t.Fatalf("create cast %s: %v", row.CreditID, err)
		}
	}

	crewRows := []database.CreateShowCrewParams{
		{ShowID: show.ID, ArtistID: leadArtist, Department: "Directing", Job: "Director", CreditID: "crew-1", EpisodeCount: 6},
		{ShowID: show.ID, ArtistID: guestArtist, Department: "Writing", Job: "Writer", CreditID: "crew-2", EpisodeCount: 10},
	}
	for _, row := range crewRows {
		if err := q.CreateShowCrew(ctx, row); err != nil {
			t.Fatalf("create crew %s: %v", row.CreditID, err)
		}
	}

	err = q.CreateShowCreator(ctx, database.CreateShowCreatorParams{ShowID: show.ID, ArtistID: leadArtist})
	if err != nil {
		t.Fatalf("create creator: %v", err)
	}

	genreID, err := q.GetOrCreateGenre(ctx, database.GetOrCreateGenreParams{Tag: "Drama", GenreType: "show"})
	if err != nil {
		t.Fatalf("create genre: %v", err)
	}
	err = q.CreateShowGenre(ctx, database.CreateShowGenreParams{ShowID: show.ID, GenreID: genreID})
	if err != nil {
		t.Fatalf("link genre: %v", err)
	}

	network, err := q.UpsertNetwork(ctx, database.UpsertNetworkParams{
		TmdbID:  77,
		Name:    "Contract Network",
		Logo:    sql.NullString{String: "/network.png", Valid: true},
		Country: sql.NullString{String: "US", Valid: true},
	})
	if err != nil {
		t.Fatalf("upsert network: %v", err)
	}
	err = q.CreateShowNetwork(ctx, database.CreateShowNetworkParams{ShowID: show.ID, NetworkID: network.ID})
	if err != nil {
		t.Fatalf("link network: %v", err)
	}

	companyID, err := q.UpsertProductionCompany(ctx, database.UpsertProductionCompanyParams{
		Name:   "Contract Pictures",
		TmdbID: 88,
	})
	if err != nil {
		t.Fatalf("upsert production company: %v", err)
	}
	err = q.CreateShowProductionCompany(ctx, database.CreateShowProductionCompanyParams{
		ShowID:              show.ID,
		ProductionCompanyID: companyID,
	})
	if err != nil {
		t.Fatalf("link production company: %v", err)
	}

	videoID, err := q.UpsertExtraVideo(ctx, database.UpsertExtraVideoParams{
		Title:      "Contract Trailer",
		ExternalID: sql.NullString{String: "tmdb-video-1", Valid: true},
		Key:        "dQw4w9WgXcQ",
		Type:       "trailer",
		Site:       "youtube",
	})
	if err != nil {
		t.Fatalf("upsert extra video: %v", err)
	}
	err = q.CreateShowExtraVideo(ctx, database.CreateShowExtraVideoParams{ShowID: show.ID, ExtraVideoID: videoID})
	if err != nil {
		t.Fatalf("link extra video: %v", err)
	}

	return show.ID
}

type showDetailsBody struct {
	Data struct {
		Show struct {
			Name             string `json:"name"`
			TmdbEpisodeCount struct {
				Int64 int64 `json:"Int64"`
				Valid bool  `json:"Valid"`
			} `json:"tmdb_episode_count"`
		} `json:"show"`
		Seasons []struct {
			SeasonNumber     int64 `json:"season_number"`
			TmdbEpisodeCount struct {
				Int64 int64 `json:"Int64"`
				Valid bool  `json:"Valid"`
			} `json:"tmdb_episode_count"`
			AvailableEpisodeCount int64 `json:"available_episode_count"`
		} `json:"seasons"`
		Cast []struct {
			CreditID string `json:"credit_id"`
			ArtistID int64  `json:"artist_id"`
		} `json:"cast"`
		Crew []struct {
			CreditID string `json:"credit_id"`
		} `json:"crew"`
		Creators []struct {
			Name string `json:"name"`
		} `json:"creators"`
		Genres []struct {
			Tag string `json:"tag"`
		} `json:"genres"`
		Networks []struct {
			Name string `json:"name"`
		} `json:"networks"`
		ProductionCompanies []struct {
			Name string `json:"name"`
		} `json:"production_companies"`
		ExtraVideos []struct {
			Key string `json:"key"`
		} `json:"extra_videos"`
	} `json:"data"`
}

func TestGetShowDetails_ConformsToOpenAPI(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	user := createTestUser(t, app, "Show Details User", "show-details@example.com", false)
	showID := seedContractShow(t, app)

	app.InitSession()
	app.InitRouter()
	cookie := newAuthSessionCookie(t, app, user.ID)

	request := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/shows/details/%d", showID), nil)
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	app.Router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusOK, response.Body.String())
	}

	// Every collection is asserted non-empty: an empty array validates against
	// the contract vacuously, so a seeding failure would pass silently.
	var body showDetailsBody
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if body.Data.Show.Name != "Contract Detail Show" {
		t.Fatalf("show name = %q, want %q", body.Data.Show.Name, "Contract Detail Show")
	}

	if len(body.Data.Seasons) != 3 {
		t.Fatalf("seasons length = %d, want 3, body = %s", len(body.Data.Seasons), response.Body.String())
	}

	// Specials sort last even though they were inserted first and number zero.
	gotOrder := []int64{}
	for _, s := range body.Data.Seasons {
		gotOrder = append(gotOrder, s.SeasonNumber)
	}
	wantOrder := []int64{1, 2, 0}
	if !slices.Equal(gotOrder, wantOrder) {
		t.Fatalf("season order = %v, want %v (specials last)", gotOrder, wantOrder)
	}

	// Season 2 is partially present: TMDB counts 8 episodes, one is on disk.
	season2 := body.Data.Seasons[1]
	if season2.AvailableEpisodeCount != 1 {
		t.Fatalf("season 2 available_episode_count = %d, want 1", season2.AvailableEpisodeCount)
	}
	if !season2.TmdbEpisodeCount.Valid || season2.TmdbEpisodeCount.Int64 != 8 {
		t.Fatalf("season 2 tmdb_episode_count = %+v, want 8", season2.TmdbEpisodeCount)
	}

	if len(body.Data.Cast) != 3 {
		t.Fatalf("cast length = %d, want 3", len(body.Data.Cast))
	}
	// One artist holds two credits, so credit_id is what keeps the rows distinct.
	if body.Data.Cast[0].ArtistID != body.Data.Cast[1].ArtistID {
		t.Fatalf("expected the first two cast rows to share an artist, got %d and %d",
			body.Data.Cast[0].ArtistID, body.Data.Cast[1].ArtistID)
	}
	if body.Data.Cast[0].CreditID == body.Data.Cast[1].CreditID {
		t.Fatalf("cast credit_id collision: %q", body.Data.Cast[0].CreditID)
	}

	if len(body.Data.Crew) != 2 {
		t.Fatalf("crew length = %d, want 2", len(body.Data.Crew))
	}
	if len(body.Data.Creators) != 1 {
		t.Fatalf("creators length = %d, want 1", len(body.Data.Creators))
	}
	if len(body.Data.Genres) != 1 {
		t.Fatalf("genres length = %d, want 1", len(body.Data.Genres))
	}
	if len(body.Data.Networks) != 1 {
		t.Fatalf("networks length = %d, want 1", len(body.Data.Networks))
	}
	if len(body.Data.ProductionCompanies) != 1 {
		t.Fatalf("production_companies length = %d, want 1", len(body.Data.ProductionCompanies))
	}
	if len(body.Data.ExtraVideos) != 1 {
		t.Fatalf("extra_videos length = %d, want 1", len(body.Data.ExtraVideos))
	}

	assertOpenAPIExchange(t, "getShowDetails", request, response)
}

func TestGetShowDetails_RejectsUnparsableID(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	user := createTestUser(t, app, "Show Bad ID User", "show-bad-id@example.com", false)

	app.InitSession()
	app.InitRouter()
	cookie := newAuthSessionCookie(t, app, user.ID)

	request := httptest.NewRequest(http.MethodGet, "/api/shows/details/not-a-number", nil)
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	app.Router.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusBadRequest, response.Body.String())
	}

	assertOpenAPIResponse(t, "getShowDetails", request, response)
}

func TestGetShowDetails_UnknownShowIsNotFound(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	user := createTestUser(t, app, "Show Missing User", "show-missing@example.com", false)

	app.InitSession()
	app.InitRouter()
	cookie := newAuthSessionCookie(t, app, user.ID)

	request := httptest.NewRequest(http.MethodGet, "/api/shows/details/999999", nil)
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	app.Router.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusNotFound, response.Body.String())
	}

	assertOpenAPIExchange(t, "getShowDetails", request, response)
}

func TestGetShowDetails_RequiresAuthentication(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	showID := seedContractShow(t, app)

	app.InitSession()
	app.InitRouter()

	request := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/shows/details/%d", showID), nil)
	response := httptest.NewRecorder()
	app.Router.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusUnauthorized, response.Body.String())
	}
}

type showSeasonEpisodesBody struct {
	Data struct {
		Season struct {
			SeasonNumber          int64 `json:"season_number"`
			AvailableEpisodeCount int64 `json:"available_episode_count"`
		} `json:"season"`
		Episodes []struct {
			EpisodeNumber int64  `json:"episode_number"`
			Name          string `json:"name"`
			TmdbRuntime   struct {
				Int64 int64 `json:"Int64"`
				Valid bool  `json:"Valid"`
			} `json:"tmdb_runtime"`
		} `json:"episodes"`
	} `json:"data"`
}

func TestGetShowSeasonEpisodes_ConformsToOpenAPI(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	user := createTestUser(t, app, "Season Episodes User", "season-episodes@example.com", false)
	showID := seedContractShow(t, app)

	app.InitSession()
	app.InitRouter()
	cookie := newAuthSessionCookie(t, app, user.ID)

	request := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/shows/%d/seasons/1/episodes", showID), nil)
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	app.Router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusOK, response.Body.String())
	}

	var body showSeasonEpisodesBody
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if body.Data.Season.SeasonNumber != 1 {
		t.Fatalf("season_number = %d, want 1", body.Data.Season.SeasonNumber)
	}
	if len(body.Data.Episodes) != 2 {
		t.Fatalf("episodes length = %d, want 2, body = %s", len(body.Data.Episodes), response.Body.String())
	}
	if body.Data.Episodes[0].EpisodeNumber != 1 || body.Data.Episodes[1].EpisodeNumber != 2 {
		t.Fatalf("episodes are not ordered by number: %+v", body.Data.Episodes)
	}
	// The second episode was left unenriched, so its nullable columns are null.
	if body.Data.Episodes[1].TmdbRuntime.Valid {
		t.Fatalf("expected the unenriched episode to have a null tmdb_runtime, got %+v", body.Data.Episodes[1].TmdbRuntime)
	}

	assertOpenAPIExchange(t, "getShowSeasonEpisodes", request, response)
}

// Specials are season zero, which the route must treat as a real season rather
// than as a missing or invalid one.
func TestGetShowSeasonEpisodes_SpecialsAreSeasonZero(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	user := createTestUser(t, app, "Specials User", "specials@example.com", false)
	showID := seedContractShow(t, app)

	app.InitSession()
	app.InitRouter()
	cookie := newAuthSessionCookie(t, app, user.ID)

	request := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/shows/%d/seasons/0/episodes", showID), nil)
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	app.Router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusOK, response.Body.String())
	}

	var body showSeasonEpisodesBody
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if body.Data.Season.SeasonNumber != 0 {
		t.Fatalf("season_number = %d, want 0", body.Data.Season.SeasonNumber)
	}
	if len(body.Data.Episodes) != 1 {
		t.Fatalf("episodes length = %d, want 1", len(body.Data.Episodes))
	}
}

func TestGetShowSeasonEpisodes_UnknownSeasonIsNotFound(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	user := createTestUser(t, app, "Missing Season User", "missing-season@example.com", false)
	showID := seedContractShow(t, app)

	app.InitSession()
	app.InitRouter()
	cookie := newAuthSessionCookie(t, app, user.ID)

	request := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/shows/%d/seasons/47/episodes", showID), nil)
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	app.Router.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusNotFound, response.Body.String())
	}

	assertOpenAPIExchange(t, "getShowSeasonEpisodes", request, response)
}

// A season number that exists for some other show must not leak across shows.
func TestGetShowSeasonEpisodes_UnknownShowIsNotFound(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	user := createTestUser(t, app, "Missing Show Season User", "missing-show-season@example.com", false)
	seedContractShow(t, app)

	app.InitSession()
	app.InitRouter()
	cookie := newAuthSessionCookie(t, app, user.ID)

	request := httptest.NewRequest(http.MethodGet, "/api/shows/999999/seasons/1/episodes", nil)
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	app.Router.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusNotFound, response.Body.String())
	}
}

func TestGetShowSeasonEpisodes_RejectsUnparsableSeasonNumber(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	user := createTestUser(t, app, "Bad Season User", "bad-season@example.com", false)
	showID := seedContractShow(t, app)

	app.InitSession()
	app.InitRouter()
	cookie := newAuthSessionCookie(t, app, user.ID)

	request := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/shows/%d/seasons/not-a-number/episodes", showID), nil)
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	app.Router.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusBadRequest, response.Body.String())
	}

	assertOpenAPIResponse(t, "getShowSeasonEpisodes", request, response)
}

func TestGetShowSeasonEpisodes_RequiresAuthentication(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	showID := seedContractShow(t, app)

	app.InitSession()
	app.InitRouter()

	request := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/shows/%d/seasons/1/episodes", showID), nil)
	response := httptest.NewRecorder()
	app.Router.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusUnauthorized, response.Body.String())
	}
}
