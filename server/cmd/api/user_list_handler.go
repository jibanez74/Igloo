package main

import (
	"errors"
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
	userID, ok := app.currentUserID(w, r)
	if !ok {
		return
	}

	q := strings.TrimSpace(r.URL.Query().Get("q"))

	rows, err := app.Queries.GetUsersExcluding(r.Context(), database.GetUsersExcludingParams{
		ExcludedID: userID,
		Search:     q,
	})
	if err != nil {
		app.Logger.Error("failed to fetch users", "error", err)
		helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		return
	}

	users := make([]userListSummary, 0, len(rows))
	for _, row := range rows {
		var avatar *string
		if row.Avatar.Valid {
			avatar = &row.Avatar.String
		}
		users = append(users, userListSummary{
			ID:     row.ID,
			Name:   row.Name,
			Email:  row.Email,
			Avatar: avatar,
		})
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error: false,
		Data:  map[string]any{"users": users},
	})
}
