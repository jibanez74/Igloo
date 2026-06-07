package main

import (
	"database/sql"
	"errors"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"net/http"
)

type CreateNotificationReq struct {
	Title   string `json:"title"`
	Message string `json:"message"`
	IsAdmin bool   `json:"isAdmin"`
}

func (app *Application) CreateNotification(w http.ResponseWriter, r *http.Request) {
	var req CreateNotificationReq

	err := helpers.ReadJSON(w, r, &req, 0)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid request body"), http.StatusBadRequest)
		return
	}

	userID, ok := app.requireSessionUserID(w, r)
	if !ok {
		return
	}

	notification, err := app.Queries.CreateNotification(r.Context(), database.CreateNotificationParams{
		CreatedByUserID: userID,
		UserID:          sql.NullInt64{},
		Title:           req.Title,
		Message:         req.Message,
		IsAdmin:         req.IsAdmin,
	})

	if err != nil {
		app.Logger.Error("failed to create notification", "error", err, "user_id", userID)
		helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		return
	}

	helpers.WriteJSON(w, http.StatusCreated, helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"notification": notification,
		},
	})
}
