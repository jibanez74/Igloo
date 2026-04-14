package main

import (
	"errors"
	"net/http"
	"strings"

	"igloo/cmd/internal/helpers"
)

type userListSummary struct {
	ID     int64   `json:"id"`
	Name   string  `json:"name"`
	Email  string  `json:"email"`
	Avatar *string `json:"avatar"`
}

// GetUsers serves GET /api/users.
// Returns all users except the authenticated user, for use in room invite selection.
// An optional ?q= query parameter filters results by name or email (case-insensitive substring match).
func (app *Application) GetUsers(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.requireSessionUserID(w, r)
	if !ok {
		return
	}

	rows, err := app.Queries.GetUsersExcluding(r.Context(), userID)
	if err != nil {
		app.Logger.Error("failed to fetch users", "error", err)
		helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		return
	}

	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))

	users := make([]userListSummary, 0, len(rows))
	for _, row := range rows {
		if q != "" {
			name := strings.ToLower(row.Name)
			email := strings.ToLower(row.Email)
			if !strings.Contains(name, q) && !strings.Contains(email, q) {
				continue
			}
		}
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
