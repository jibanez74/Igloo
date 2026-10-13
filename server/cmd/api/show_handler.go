package main

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"

	"github.com/go-chi/chi/v5"
)

func (app *Application) GetLatestShows(w http.ResponseWriter, r *http.Request) {
	shows, err := app.Queries.GetLatestShows(r.Context())
	if err != nil {
		app.serverError(w, err, "failed to get latest shows", "failed to fetch latest shows")
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{Data: map[string]any{"shows": shows}})
}

// Read-only transaction keeps related show data in one snapshot. Seasons carry
// only their episode counts; the episodes themselves are fetched per selection
// by GetShowSeasonEpisodes, because a long-running show holds hundreds of
// episodes with overview text and the page renders one season at a time.
func (app *Application) GetShowDetails(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r, "id", "invalid show id")
	if !ok {
		return
	}

	ctx := r.Context()

	tx, err := app.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		app.serverError(w, err, beginTransactionLogMessage, "failed to fetch show from server")
		return
	}
	defer tx.Rollback()

	qtx := app.Queries.WithTx(tx)

	show, err := qtx.GetShowDetails(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("show not found"), http.StatusNotFound)
			return
		}

		app.serverError(w, err, "failed to get show", "failed to fetch show from server", "id", id)
		return
	}

	seasons, err := qtx.GetShowSeasonSummaries(ctx, id)
	if err != nil {
		app.serverError(w, err, "failed to get seasons for show", "failed to fetch show seasons from server", "show_id", id)
		return
	}

	cast, err := qtx.GetCastByShowID(ctx, id)
	if err != nil {
		app.serverError(w, err, "failed to get cast for show", "failed to fetch show cast from server", "show_id", id)
		return
	}

	crew, err := qtx.GetCrewByShowID(ctx, id)
	if err != nil {
		app.serverError(w, err, "failed to get crew for show", "failed to fetch show crew from server", "show_id", id)
		return
	}

	creators, err := qtx.GetCreatorsByShowID(ctx, id)
	if err != nil {
		app.serverError(w, err, "failed to get creators for show", "failed to fetch show creators from server", "show_id", id)
		return
	}

	genres, err := qtx.GetGenresByShowID(ctx, id)
	if err != nil {
		app.serverError(w, err, "failed to get genres for show", "failed to fetch show genres from server", "show_id", id)
		return
	}

	networks, err := qtx.GetNetworksByShowID(ctx, id)
	if err != nil {
		app.serverError(w, err, "failed to get networks for show", "failed to fetch show networks from server", "show_id", id)
		return
	}

	productionCompanies, err := qtx.GetProductionCompaniesByShowID(ctx, id)
	if err != nil {
		app.serverError(w, err, "failed to get production companies for show", "failed to fetch show production companies from server",
			"show_id", id)
		return
	}

	extraVideos, err := qtx.GetShowExtraVideos(ctx, id)
	if err != nil {
		app.serverError(w, err, "failed to get extra videos for show", "failed to fetch show extra videos from server",
			"show_id", id)
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Data: map[string]any{
			"show":                 show,
			"seasons":              seasons,
			"cast":                 cast,
			"crew":                 crew,
			"creators":             creators,
			"genres":               genres,
			"networks":             networks,
			"production_companies": productionCompanies,
			"extra_videos":         extraVideos,
		},
	})
}

// One season of a show, addressed by season number rather than by the internal
// season id. Specials are season zero, so the number is only rejected when it
// does not parse or is negative. Each episode carries the caller's own watch
// progress, so the page needs no per-row query for resume or watched state.
func (app *Application) GetShowSeasonEpisodes(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)

	id, ok := parseIDParam(w, r, "id", "invalid show id")
	if !ok {
		return
	}

	seasonParam := chi.URLParam(r, "seasonNumber")
	seasonNumber, err := strconv.ParseInt(seasonParam, 10, 64)
	if err != nil || seasonNumber < 0 {
		helpers.ErrorJSON(w, errors.New("invalid season number"), http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	tx, err := app.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		app.serverError(w, err, beginTransactionLogMessage, "failed to fetch season from server")
		return
	}
	defer tx.Rollback()

	qtx := app.Queries.WithTx(tx)

	season, err := qtx.GetShowSeasonSummaryByNumber(ctx, database.GetShowSeasonSummaryByNumberParams{
		ShowID:       id,
		SeasonNumber: seasonNumber,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("season not found"), http.StatusNotFound)
			return
		}

		app.serverError(w, err, "failed to get season", "failed to fetch season from server",
			"show_id", id, "season_number", seasonNumber)
		return
	}

	episodes, err := qtx.GetShowEpisodesBySeasonNumber(ctx, database.GetShowEpisodesBySeasonNumberParams{
		UserID:       userID,
		ShowID:       id,
		SeasonNumber: seasonNumber,
	})
	if err != nil {
		app.serverError(w, err, "failed to get episodes for season", "failed to fetch season episodes from server",
			"show_id", id, "season_number", seasonNumber)
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Data: map[string]any{
			"season":   season,
			"episodes": episodes,
		},
	})
}

// showsLibraryData is the JSON shape of helpers.JSONResponse.Data for GET /api/shows/library
// and GET /api/shows/genres/{genreId}/shows.
type showsLibraryData struct {
	Shows      []database.GetShowsLibraryAscRow `json:"shows"`
	Total      int64                            `json:"total"`
	Page       int64                            `json:"page"`
	PerPage    int64                            `json:"per_page"`
	TotalPages int64                            `json:"total_pages"`
	Sort       string                           `json:"sort"`
}

// showsStatsData is the JSON shape of helpers.JSONResponse.Data for GET /api/shows/stats.
type showsStatsData struct {
	TotalShows    int64 `json:"total_shows"`
	TotalSeasons  int64 `json:"total_seasons"`
	TotalEpisodes int64 `json:"total_episodes"`
}

// showLibraryRow is every generated show listing row. Each has exactly
// GetShowsLibraryAscRow's fields, so it converts to that type directly.
type showLibraryRow interface {
	database.GetShowsLibraryDescRow | database.GetShowsByGenreAscRow | database.GetShowsByGenreDescRow
}

func showLibraryRows[T showLibraryRow](rows []T) []database.GetShowsLibraryAscRow {
	out := make([]database.GetShowsLibraryAscRow, len(rows))
	for i, row := range rows {
		out[i] = database.GetShowsLibraryAscRow(row)
	}
	return out
}

func writeShowsLibrary(w http.ResponseWriter, shows []database.GetShowsLibraryAscRow, total, page, perPage int64, sort string) {
	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Data: showsLibraryData{
			Shows:      shows,
			Total:      total,
			Page:       page,
			PerPage:    perPage,
			TotalPages: totalPages(total, perPage),
			Sort:       sort,
		},
	})
}

func (app *Application) GetShowsLibrary(w http.ResponseWriter, r *http.Request) {
	page, perPage, sortParam := parseLibraryQuery(r)
	ctx := r.Context()

	total, err := app.Queries.GetShowsCount(ctx)
	if err != nil {
		app.serverError(w, err, "failed to get shows count", "failed to fetch shows count")
		return
	}

	params := database.GetShowsLibraryAscParams{Limit: perPage, Offset: (page - 1) * perPage}

	var shows []database.GetShowsLibraryAscRow
	if sortParam == "desc" {
		var rows []database.GetShowsLibraryDescRow
		rows, err = app.Queries.GetShowsLibraryDesc(ctx, database.GetShowsLibraryDescParams(params))
		shows = showLibraryRows(rows)
	} else {
		shows, err = app.Queries.GetShowsLibraryAsc(ctx, params)
	}
	if err != nil {
		app.serverError(w, err, "failed to get shows library", "failed to fetch shows")
		return
	}

	writeShowsLibrary(w, shows, total, page, perPage, sortParam)
}

func (app *Application) GetShowsStats(w http.ResponseWriter, r *http.Request) {
	counts, err := app.Queries.GetShowsLibraryCounts(r.Context())
	if err != nil {
		app.serverError(w, err, "failed to get shows library counts", "failed to fetch shows stats")
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Data: showsStatsData{
			TotalShows:    counts.ShowsCount,
			TotalSeasons:  counts.SeasonsCount,
			TotalEpisodes: counts.EpisodesCount,
		},
	})
}

func (app *Application) GetShowGenresList(w http.ResponseWriter, r *http.Request) {
	rows, err := app.Queries.GetShowGenresWithCounts(r.Context())
	if err != nil {
		app.serverError(w, err, "failed to get show genres with counts", "failed to fetch genres")
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Data: map[string]any{
			"genres": rows,
		},
	})
}

func (app *Application) GetShowsByGenreLibrary(w http.ResponseWriter, r *http.Request) {
	genreID, err := strconv.ParseInt(chi.URLParam(r, "genreId"), 10, 64)
	if err != nil || genreID <= 0 {
		helpers.ErrorJSON(w, errors.New("invalid genre id"), http.StatusBadRequest)
		return
	}

	page, perPage, sortParam := parseLibraryQuery(r)
	ctx := r.Context()

	total, err := app.Queries.CountShowsForGenre(ctx, genreID)
	if err != nil {
		app.serverError(w, err, "failed to count shows for genre", "failed to fetch shows count for genre", "genre_id", genreID)
		return
	}

	params := database.GetShowsByGenreAscParams{GenreID: genreID, Limit: perPage, Offset: (page - 1) * perPage}

	var shows []database.GetShowsLibraryAscRow
	if sortParam == "desc" {
		var rows []database.GetShowsByGenreDescRow
		rows, err = app.Queries.GetShowsByGenreDesc(ctx, database.GetShowsByGenreDescParams(params))
		shows = showLibraryRows(rows)
	} else {
		var rows []database.GetShowsByGenreAscRow
		rows, err = app.Queries.GetShowsByGenreAsc(ctx, params)
		shows = showLibraryRows(rows)
	}
	if err != nil {
		app.serverError(w, err, "failed to get shows by genre", "failed to fetch shows", "genre_id", genreID)
		return
	}

	writeShowsLibrary(w, shows, total, page, perPage, sortParam)
}
