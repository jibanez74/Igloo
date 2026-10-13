package main

import (
	"net/http"
	"strings"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
)

type userListSummary struct {
	ID     int64   `json:"id"`
	Name   string  `json:"name"`
	Email  string  `json:"email"`
	Avatar *string `json:"avatar"`
}

func (app *Application) GetUsers(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)

	q := strings.TrimSpace(r.URL.Query().Get("q"))

	rows, err := app.Queries.GetUsersExcluding(r.Context(), database.GetUsersExcludingParams{
		ExcludedID: userID,
		Search:     q,
	})
	if err != nil {
		app.serverError(w, err, "failed to fetch users", internalServerErrorMessage)
		return
	}

	users := make([]userListSummary, 0, len(rows))
	for _, row := range rows {
		users = append(users, userListSummary{
			ID:     row.ID,
			Name:   row.Name,
			Email:  row.Email,
			Avatar: helpers.StringPtrFromNull(row.Avatar),
		})
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{Data: map[string]any{"users": users}})
}
