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
const (
	notificationListLimit         = 50
	notificationTitleMovieRequest = "movie_request"
	notificationTitleAlbumRequest = "album_request"
	notificationTitleTrackRequest = "track_request"
	notificationTitleOther        = "other"
)

type CreateNotificationReq struct {
	Title   string `json:"title"`
	Message string `json:"message"`
	IsAdmin bool   `json:"isAdmin"`
}

// notificationResponse is the client-facing shape of a notification. It
// exposes the creator's display name and the viewer's read state.
type notificationResponse struct {
	ID            int64  `json:"id"`
	Title         string `json:"title"`
	Message       string `json:"message"`
	IsAdmin       bool   `json:"is_admin"`
	IsRead        bool   `json:"is_read"`
	CreatedByName string `json:"created_by_name"`
	CreatedAt     string `json:"created_at"`
}

func newNotificationResponse(row database.ListNotificationsForUserRow) notificationResponse {
	return notificationResponse{
		ID:            row.ID,
		Title:         row.Title,
		Message:       row.Message,
		IsAdmin:       row.IsAdmin,
		IsRead:        row.IsRead,
		CreatedByName: row.CreatedByName,
		CreatedAt:     row.CreatedAt,
	}
}

// writeNotificationViewerError answers a failed viewer lookup. No user row means
// the session outlived its user, which is a stale session (401) rather than a
// server fault.
func (app *Application) writeNotificationViewerError(w http.ResponseWriter, r *http.Request, userID int64, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		destroyErr := app.SessionManager.Destroy(r.Context())
		if destroyErr != nil {
			app.Logger.Error("failed to destroy stale notification session", "error", destroyErr, "user_id", userID)
		}
		helpers.ErrorJSON(w, errors.New(notAuthorizedMessage), http.StatusUnauthorized)
		return
	}

	app.Logger.Error("failed to load user for notifications", "error", err, "user_id", userID)
	helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
}

// notificationViewerIsAdmin reports whether the current user is an admin, which
// determines whether the shared admin request queue is visible to them. It
// writes the error response and returns ok=false on failure.
func (app *Application) notificationViewerIsAdmin(w http.ResponseWriter, r *http.Request, userID int64) (bool, bool) {
	isAdmin, err := app.Queries.GetUserIsAdmin(r.Context(), userID)
	if err != nil {
		app.writeNotificationViewerError(w, r, userID, err)
		return false, false
	}

	return isAdmin, true
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
		helpers.ErrorJSON(w, errors.New(invalidRequestBodyMessage), http.StatusBadRequest)
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

	// Notifications are the shared admin request queue; there is no per-user
	// targeting, so isAdmin must be true.
	if !req.IsAdmin {
		helpers.ErrorJSON(w, errors.New("isAdmin must be true: notifications are the shared admin queue"), http.StatusBadRequest)
		return
	}

	userID, ok := app.currentUserID(w, r)
	if !ok {
		return
	}

	notification, err := app.Queries.CreateNotification(r.Context(), database.CreateNotificationParams{
		CreatedByUserID: userID,
		Title:           req.Title,
		Message:         req.Message,
		IsAdmin:         req.IsAdmin,
	})

	if err != nil {
		app.Logger.Error("failed to create notification", "error", err, "user_id", userID)
		helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
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
	case notificationTitleMovieRequest,
		notificationTitleAlbumRequest,
		notificationTitleTrackRequest,
		notificationTitleOther:
		return true
	default:
		return false
	}
}

// ListNotifications returns the admin request queue, newest first, along with
// the unread count. Non-admins see an empty queue without touching the
// notifications table.
func (app *Application) ListNotifications(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.currentUserID(w, r)
	if !ok {
		return
	}

	isAdmin, ok := app.notificationViewerIsAdmin(w, r, userID)
	if !ok {
		return
	}

	if !isAdmin {
		helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
			Error: false,
			Data: map[string]any{
				"notifications": []notificationResponse{},
				"unread_count":  int64(0),
			},
		})
		return
	}

	ctx := r.Context()

	rows, err := app.Queries.ListNotificationsForUser(ctx, database.ListNotificationsForUserParams{
		UserID:   userID,
		RowLimit: notificationListLimit,
	})
	if err != nil {
		app.Logger.Error("failed to list notifications", "error", err, "user_id", userID)
		helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
		return
	}

	unreadCount, err := app.Queries.CountUnreadNotificationsForUser(ctx, userID)
	if err != nil {
		app.Logger.Error("failed to count unread notifications", "error", err, "user_id", userID)
		helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
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
// lightweight because the client polls it on an interval to drive the bell badge:
// GetNotificationBadgeForUser folds the admin check and the count into a single
// statement, so a poll costs one round trip on the shared connection.
func (app *Application) GetUnreadNotificationCount(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.currentUserID(w, r)
	if !ok {
		return
	}

	badge, err := app.Queries.GetNotificationBadgeForUser(r.Context(), userID)
	if err != nil {
		app.writeNotificationViewerError(w, r, userID, err)
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"unread_count": badge.UnreadCount,
		},
	})
}

// MarkNotificationRead records that the current user has read a single
// notification. It is idempotent and relevance-gated in SQL, so marking an
// already-read or out-of-scope notification is a harmless no-op.
func (app *Application) MarkNotificationRead(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.currentUserID(w, r)
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

	if isAdmin {
		err := app.Queries.MarkNotificationReadForUser(r.Context(), database.MarkNotificationReadForUserParams{
			UserID:         userID,
			NotificationID: notificationID,
		})
		if err != nil {
			app.Logger.Error("failed to mark notification read", "error", err, "user_id", userID, "notification_id", notificationID)
			helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
			return
		}
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error:   false,
		Message: "Notification marked as read",
	})
}

// MarkAllNotificationsRead marks every notification currently visible to the
// user as read.
func (app *Application) MarkAllNotificationsRead(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.currentUserID(w, r)
	if !ok {
		return
	}

	isAdmin, ok := app.notificationViewerIsAdmin(w, r, userID)
	if !ok {
		return
	}

	if isAdmin {
		err := app.Queries.MarkAllNotificationsReadForUser(r.Context(), userID)
		if err != nil {
			app.Logger.Error("failed to mark all notifications read", "error", err, "user_id", userID)
			helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
			return
		}
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
	userID, ok := app.currentUserID(w, r)
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

	// Non-admins can never see a notification, so for them the delete is the
	// same not-found it always was.
	var rowsAffected int64
	if isAdmin {
		var err error
		rowsAffected, err = app.Queries.DeleteNotificationForUser(r.Context(), notificationID)
		if err != nil {
			app.Logger.Error("failed to delete notification", "error", err, "user_id", userID, "notification_id", notificationID)
			helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
			return
		}
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
