package main

import (
	"database/sql"
	"errors"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

// notificationListLimit caps how many notifications a single list request
// returns. The bell panel only shows the most recent ones.
const notificationListLimit = 50

type CreateNotificationReq struct {
	Title   string `json:"title"`
	Message string `json:"message"`
	IsAdmin bool   `json:"isAdmin"`
}

// notificationResponse is the client-facing shape of a notification. It avoids
// leaking the raw sql.NullInt64 user_id and exposes the creator's display name
// and the viewer's read state.
type notificationResponse struct {
	ID            int64  `json:"id"`
	Title         string `json:"title"`
	Message       string `json:"message"`
	IsAdmin       bool   `json:"is_admin"`
	IsRead        bool   `json:"is_read"`
	CreatedByName string `json:"created_by_name"`
	UserID        *int64 `json:"user_id"`
	CreatedAt     string `json:"created_at"`
}

func newNotificationResponse(row database.ListNotificationsForUserRow) notificationResponse {
	var userID *int64
	if row.UserID.Valid {
		id := row.UserID.Int64
		userID = &id
	}

	var createdByName string
	if row.CreatedByName.Valid {
		createdByName = row.CreatedByName.String
	}

	return notificationResponse{
		ID:            row.ID,
		Title:         row.Title,
		Message:       row.Message,
		IsAdmin:       row.IsAdmin,
		IsRead:        row.IsRead,
		CreatedByName: createdByName,
		UserID:        userID,
		CreatedAt:     row.CreatedAt,
	}
}

// notificationViewerIsAdmin reports whether the current user is an admin, which
// determines whether the shared admin request queue is visible to them. It
// writes the error response and returns ok=false on failure.
func (app *Application) notificationViewerIsAdmin(w http.ResponseWriter, r *http.Request, userID int64) (bool, bool) {
	user, err := app.Queries.GetUser(r.Context(), userID)
	if err != nil {
		app.Logger.Error("failed to load user for notifications", "error", err, "user_id", userID)
		helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		return false, false
	}

	return user.IsAdmin, true
}

func parseNotificationID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		helpers.ErrorJSON(w, errors.New("invalid notification id"), http.StatusBadRequest)
		return 0, false
	}

	return id, true
}

func (app *Application) CreateNotification(w http.ResponseWriter, r *http.Request) {
	var req CreateNotificationReq

	err := helpers.ReadJSON(w, r, &req, 0)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid request body"), http.StatusBadRequest)
		return
	}

	req.Title = strings.TrimSpace(req.Title)
	req.Message = strings.TrimSpace(req.Message)

	if !isValidNotificationTitle(req.Title) {
		helpers.ErrorJSON(w, errors.New("invalid notification title"), http.StatusBadRequest)
		return
	}

	if req.Message == "" {
		helpers.ErrorJSON(w, errors.New("message is required"), http.StatusBadRequest)
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

func isValidNotificationTitle(title string) bool {
	switch title {
	case helpers.NOTIFICATION_TITLE_MOVIE_REQUEST,
		helpers.NOTIFICATION_TITLE_ALBUM_REQUEST,
		helpers.NOTIFICATION_TITLE_TRACK_REQUEST,
		helpers.NOTIFICATION_TITLE_OTHER:
		return true
	default:
		return false
	}
}

// ListNotifications returns the notifications visible to the current user
// (their targeted notifications, plus the admin request queue for admins),
// newest first, along with the unread count.
func (app *Application) ListNotifications(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.requireSessionUserID(w, r)
	if !ok {
		return
	}

	isAdmin, ok := app.notificationViewerIsAdmin(w, r, userID)
	if !ok {
		return
	}

	ctx := r.Context()

	rows, err := app.Queries.ListNotificationsForUser(ctx, database.ListNotificationsForUserParams{
		UserID:        userID,
		ViewerIsAdmin: isAdmin,
		RowLimit:      notificationListLimit,
	})
	if err != nil {
		app.Logger.Error("failed to list notifications", "error", err, "user_id", userID)
		helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		return
	}

	unreadCount, err := app.Queries.CountUnreadNotificationsForUser(ctx, database.CountUnreadNotificationsForUserParams{
		UserID:        userID,
		ViewerIsAdmin: isAdmin,
	})
	if err != nil {
		app.Logger.Error("failed to count unread notifications", "error", err, "user_id", userID)
		helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		return
	}

	notifications := make([]notificationResponse, 0, len(rows))
	for _, row := range rows {
		notifications = append(notifications, newNotificationResponse(row))
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"notifications": notifications,
			"unread_count":  unreadCount,
		},
	})
}

// GetUnreadNotificationCount returns just the unread count. It is intentionally
// lightweight because the client polls it on an interval to drive the bell badge.
func (app *Application) GetUnreadNotificationCount(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.requireSessionUserID(w, r)
	if !ok {
		return
	}

	isAdmin, ok := app.notificationViewerIsAdmin(w, r, userID)
	if !ok {
		return
	}

	unreadCount, err := app.Queries.CountUnreadNotificationsForUser(r.Context(), database.CountUnreadNotificationsForUserParams{
		UserID:        userID,
		ViewerIsAdmin: isAdmin,
	})
	if err != nil {
		app.Logger.Error("failed to count unread notifications", "error", err, "user_id", userID)
		helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"unread_count": unreadCount,
		},
	})
}

// MarkNotificationRead records that the current user has read a single
// notification. It is idempotent and relevance-gated in SQL, so marking an
// already-read or out-of-scope notification is a harmless no-op.
func (app *Application) MarkNotificationRead(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.requireSessionUserID(w, r)
	if !ok {
		return
	}

	notificationID, ok := parseNotificationID(w, r)
	if !ok {
		return
	}

	isAdmin, ok := app.notificationViewerIsAdmin(w, r, userID)
	if !ok {
		return
	}

	err := app.Queries.MarkNotificationReadForUser(r.Context(), database.MarkNotificationReadForUserParams{
		UserID:         userID,
		NotificationID: notificationID,
		ViewerIsAdmin:  isAdmin,
	})
	if err != nil {
		app.Logger.Error("failed to mark notification read", "error", err, "user_id", userID, "notification_id", notificationID)
		helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error:   false,
		Message: "Notification marked as read",
	})
}

// MarkAllNotificationsRead marks every notification currently visible to the
// user as read.
func (app *Application) MarkAllNotificationsRead(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.requireSessionUserID(w, r)
	if !ok {
		return
	}

	isAdmin, ok := app.notificationViewerIsAdmin(w, r, userID)
	if !ok {
		return
	}

	err := app.Queries.MarkAllNotificationsReadForUser(r.Context(), database.MarkAllNotificationsReadForUserParams{
		UserID:        userID,
		ViewerIsAdmin: isAdmin,
	})
	if err != nil {
		app.Logger.Error("failed to mark all notifications read", "error", err, "user_id", userID)
		helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error:   false,
		Message: "All notifications marked as read",
	})
}

// DeleteNotification removes a notification the current user can see. Deleting a
// shared admin-queue notification clears it for all admins, which is the
// intended "request handled" behavior.
func (app *Application) DeleteNotification(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.requireSessionUserID(w, r)
	if !ok {
		return
	}

	notificationID, ok := parseNotificationID(w, r)
	if !ok {
		return
	}

	isAdmin, ok := app.notificationViewerIsAdmin(w, r, userID)
	if !ok {
		return
	}

	rowsAffected, err := app.Queries.DeleteNotificationForUser(r.Context(), database.DeleteNotificationForUserParams{
		NotificationID: notificationID,
		UserID:         sql.NullInt64{Int64: userID, Valid: true},
		ViewerIsAdmin:  isAdmin,
	})
	if err != nil {
		app.Logger.Error("failed to delete notification", "error", err, "user_id", userID, "notification_id", notificationID)
		helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		return
	}

	if rowsAffected == 0 {
		helpers.ErrorJSON(w, errors.New("notification not found"), http.StatusNotFound)
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error:   false,
		Message: "Notification deleted",
	})
}
