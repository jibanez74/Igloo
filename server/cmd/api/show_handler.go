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
		app.Logger.Error("failed to get latest shows", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to fetch latest shows"))
		return
	}

	res := helpers.JSONResponse{
		Error: false,
		Data:  map[string]any{"shows": shows},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

// Read-only transaction keeps related show data in one snapshot. Seasons carry
// only their episode counts; the episodes themselves are fetched per selection
// by GetShowSeasonEpisodes, because a long-running show holds hundreds of
// episodes with overview text and the page renders one season at a time.
func (app *Application) GetShowDetails(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid show id"), http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	tx, err := app.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		app.Logger.Error(beginTransactionLogMessage, "error", err)
		helpers.ErrorJSON(w, errors.New("failed to fetch show from server"))
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

		app.Logger.Error("failed to get show", "error", err, "id", id)
		helpers.ErrorJSON(w, errors.New("failed to fetch show from server"))
		return
	}

	seasons, err := qtx.GetShowSeasonSummaries(ctx, id)
	if err != nil {
		app.Logger.Error("failed to get seasons for show", "error", err, "show_id", id)
		helpers.ErrorJSON(w, errors.New("failed to fetch show seasons from server"))
		return
	}

	cast, err := qtx.GetCastByShowID(ctx, id)
	if err != nil {
		app.Logger.Error("failed to get cast for show", "error", err, "show_id", id)
		helpers.ErrorJSON(w, errors.New("failed to fetch show cast from server"))
		return
	}

	crew, err := qtx.GetCrewByShowID(ctx, id)
	if err != nil {
		app.Logger.Error("failed to get crew for show", "error", err, "show_id", id)
		helpers.ErrorJSON(w, errors.New("failed to fetch show crew from server"))
		return
	}

	creators, err := qtx.GetCreatorsByShowID(ctx, id)
	if err != nil {
		app.Logger.Error("failed to get creators for show", "error", err, "show_id", id)
		helpers.ErrorJSON(w, errors.New("failed to fetch show creators from server"))
		return
	}

	genres, err := qtx.GetGenresByShowID(ctx, id)
	if err != nil {
		app.Logger.Error("failed to get genres for show", "error", err, "show_id", id)
		helpers.ErrorJSON(w, errors.New("failed to fetch show genres from server"))
		return
	}

	networks, err := qtx.GetNetworksByShowID(ctx, id)
	if err != nil {
		app.Logger.Error("failed to get networks for show", "error", err, "show_id", id)
		helpers.ErrorJSON(w, errors.New("failed to fetch show networks from server"))
		return
	}

	productionCompanies, err := qtx.GetProductionCompaniesByShowID(ctx, id)
	if err != nil {
		app.Logger.Error("failed to get production companies for show", "error", err, "show_id", id)
		helpers.ErrorJSON(w, errors.New("failed to fetch show production companies from server"))
		return
	}

	extraVideos, err := qtx.GetShowExtraVideos(ctx, id)
	if err != nil {
		app.Logger.Error("failed to get extra videos for show", "error", err, "show_id", id)
		helpers.ErrorJSON(w, errors.New("failed to fetch show extra videos from server"))
		return
	}

	res := helpers.JSONResponse{
		Error: false,
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
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

// One season of a show, addressed by season number rather than by the internal
// season id. Specials are season zero, so the number is only rejected when it
// does not parse or is negative. Each episode carries the caller's own watch
// progress, so the page needs no per-row query for resume or watched state.
func (app *Application) GetShowSeasonEpisodes(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.currentUserID(w, r)
	if !ok {
		return
	}

	idParam := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid show id"), http.StatusBadRequest)
		return
	}

	seasonParam := chi.URLParam(r, "seasonNumber")
	seasonNumber, err := strconv.ParseInt(seasonParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid season number"), http.StatusBadRequest)
		return
	}

	isNegativeSeason := seasonNumber < 0
	if isNegativeSeason {
		helpers.ErrorJSON(w, errors.New("invalid season number"), http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	tx, err := app.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		app.Logger.Error(beginTransactionLogMessage, "error", err)
		helpers.ErrorJSON(w, errors.New("failed to fetch season from server"))
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

		app.Logger.Error("failed to get season", "error", err, "show_id", id, "season_number", seasonNumber)
		helpers.ErrorJSON(w, errors.New("failed to fetch season from server"))
		return
	}

	episodes, err := qtx.GetShowEpisodesBySeasonNumber(ctx, database.GetShowEpisodesBySeasonNumberParams{
		UserID:       userID,
		ShowID:       id,
		SeasonNumber: seasonNumber,
	})
	if err != nil {
		app.Logger.Error("failed to get episodes for season", "error", err, "show_id", id, "season_number", seasonNumber)
		helpers.ErrorJSON(w, errors.New("failed to fetch season episodes from server"))
		return
	}

	res := helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"season":   season,
			"episodes": episodes,
		},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
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
	TotalShows int64 `json:"total_shows"`
}

func showLibraryRowsFromDesc(rows []database.GetShowsLibraryDescRow) []database.GetShowsLibraryAscRow {
	out := make([]database.GetShowsLibraryAscRow, len(rows))
	for i, r := range rows {
		out[i] = database.GetShowsLibraryAscRow{
			ID:            r.ID,
			Name:          r.Name,
			PosterPath:    r.PosterPath,
			PremiereYear:  r.PremiereYear,
			Certification: r.Certification,
		}
	}
	return out
}

func showLibraryRowsFromGenreAsc(rows []database.GetShowsByGenreAscRow) []database.GetShowsLibraryAscRow {
	out := make([]database.GetShowsLibraryAscRow, len(rows))
	for i, r := range rows {
		out[i] = database.GetShowsLibraryAscRow{
			ID:            r.ID,
			Name:          r.Name,
			PosterPath:    r.PosterPath,
			PremiereYear:  r.PremiereYear,
			Certification: r.Certification,
		}
	}
	return out
}

func showLibraryRowsFromGenreDesc(rows []database.GetShowsByGenreDescRow) []database.GetShowsLibraryAscRow {
	out := make([]database.GetShowsLibraryAscRow, len(rows))
	for i, r := range rows {
		out[i] = database.GetShowsLibraryAscRow{
			ID:            r.ID,
			Name:          r.Name,
			PosterPath:    r.PosterPath,
			PremiereYear:  r.PremiereYear,
			Certification: r.Certification,
		}
	}
	return out
}

func (app *Application) GetShowsLibrary(w http.ResponseWriter, r *http.Request) {
	page, perPage, sortParam := parseLibraryQuery(r)

	offset := (page - 1) * perPage
	ctx := r.Context()

	total, err := app.Queries.GetShowsCount(ctx)
	if err != nil {
		app.Logger.Error("failed to get shows count", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to fetch shows count"))
		return
	}

	var shows []database.GetShowsLibraryAscRow
	if sortParam == "desc" {
		descRows, err := app.Queries.GetShowsLibraryDesc(ctx, database.GetShowsLibraryDescParams{
			Limit:  perPage,
			Offset: offset,
		})
		if err != nil {
			app.Logger.Error("failed to get shows library", "error", err)
			helpers.ErrorJSON(w, errors.New("failed to fetch shows"))
			return
		}
		shows = showLibraryRowsFromDesc(descRows)
	} else {
		shows, err = app.Queries.GetShowsLibraryAsc(ctx, database.GetShowsLibraryAscParams{
			Limit:  perPage,
			Offset: offset,
		})
		if err != nil {
			app.Logger.Error("failed to get shows library", "error", err)
			helpers.ErrorJSON(w, errors.New("failed to fetch shows"))
			return
		}
	}

	totalPages := total / perPage
	if total%perPage > 0 {
		totalPages++
	}

	res := helpers.JSONResponse{
		Error: false,
		Data: showsLibraryData{
			Shows:      shows,
			Total:      total,
			Page:       page,
			PerPage:    perPage,
			TotalPages: totalPages,
			Sort:       sortParam,
		},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

func (app *Application) GetShowsStats(w http.ResponseWriter, r *http.Request) {
	total, err := app.Queries.GetShowsCount(r.Context())
	if err != nil {
		app.Logger.Error("failed to get shows count", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to fetch shows stats"))
		return
	}

	res := helpers.JSONResponse{
		Error: false,
		Data: showsStatsData{
			TotalShows: total,
		},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

func (app *Application) GetShowGenresList(w http.ResponseWriter, r *http.Request) {
	rows, err := app.Queries.GetShowGenresWithCounts(r.Context())
	if err != nil {
		app.Logger.Error("failed to get show genres with counts", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to fetch genres"))
		return
	}

	res := helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"genres": rows,
		},
	}
	helpers.WriteJSON(w, http.StatusOK, res)
}

func (app *Application) GetShowsByGenreLibrary(w http.ResponseWriter, r *http.Request) {
	genreID, err := strconv.ParseInt(chi.URLParam(r, "genreId"), 10, 64)
	if err != nil || genreID <= 0 {
		helpers.ErrorJSON(w, errors.New("invalid genre id"), http.StatusBadRequest)
		return
	}

	page, perPage, sortParam := parseLibraryQuery(r)
	offset := (page - 1) * perPage
	ctx := r.Context()

	total, err := app.Queries.CountShowsForGenre(ctx, genreID)
	if err != nil {
		app.Logger.Error("failed to count shows for genre", "error", err, "genre_id", genreID)
		helpers.ErrorJSON(w, errors.New("failed to fetch shows count for genre"))
		return
	}

	var shows []database.GetShowsLibraryAscRow
	if sortParam == "desc" {
		descRows, err := app.Queries.GetShowsByGenreDesc(ctx, database.GetShowsByGenreDescParams{
			GenreID: genreID,
			Limit:   perPage,
			Offset:  offset,
		})
		if err != nil {
			app.Logger.Error("failed to get shows by genre", "error", err, "genre_id", genreID)
			helpers.ErrorJSON(w, errors.New("failed to fetch shows"))
			return
		}
		shows = showLibraryRowsFromGenreDesc(descRows)
	} else {
		ascRows, err := app.Queries.GetShowsByGenreAsc(ctx, database.GetShowsByGenreAscParams{
			GenreID: genreID,
			Limit:   perPage,
			Offset:  offset,
		})
		if err != nil {
			app.Logger.Error("failed to get shows by genre", "error", err, "genre_id", genreID)
			helpers.ErrorJSON(w, errors.New("failed to fetch shows"))
			return
		}
		shows = showLibraryRowsFromGenreAsc(ascRows)
	}

	totalPages := total / perPage
	if total%perPage > 0 {
		totalPages++
	}

	res := helpers.JSONResponse{
		Error: false,
		Data: showsLibraryData{
			Shows:      shows,
			Total:      total,
			Page:       page,
			PerPage:    perPage,
			TotalPages: totalPages,
			Sort:       sortParam,
		},
	}
	helpers.WriteJSON(w, http.StatusOK, res)
}
