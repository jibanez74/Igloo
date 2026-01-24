package main

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
)

// UpdateUserNameRequest represents the request body for updating user name
type UpdateUserNameRequest struct {
	Name string `json:"name"`
}

// UpdateUserName updates the authenticated user's name
func (app *Application) UpdateUserName(w http.ResponseWriter, r *http.Request) {
	userID := app.SessionManager.GetInt64(r.Context(), helpers.COOKIE_USER_ID)
	if userID == 0 {
		helpers.ErrorJSON(w, errors.New(helpers.NOT_AUTHORIZED_MESSAGE), http.StatusUnauthorized)
		return
	}

	var req UpdateUserNameRequest
	if err := helpers.ReadJSON(w, r, &req, 0); err != nil {
		helpers.ErrorJSON(w, errors.New("invalid request body"), http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		helpers.ErrorJSON(w, errors.New("name is required"), http.StatusBadRequest)
		return
	}

	if len(req.Name) > 100 {
		helpers.ErrorJSON(w, errors.New("name must be 100 characters or less"), http.StatusBadRequest)
		return
	}

	user, err := app.Queries.UpdateUserName(r.Context(), database.UpdateUserNameParams{
		Name: req.Name,
		ID:   userID,
	})
	if err != nil {
		app.Logger.Error("failed to update user name", "error", err, "user_id", userID)
		helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		return
	}

	res := helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"user": map[string]any{
				"id":         user.ID,
				"name":       user.Name,
				"email":      user.Email,
				"is_admin":   user.IsAdmin,
				"avatar":     user.Avatar,
				"created_at": user.CreatedAt,
				"updated_at": user.UpdatedAt,
			},
		},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

// UpdateUserPasswordRequest represents the request body for updating password
type UpdateUserPasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// UpdateUserPassword updates the authenticated user's password
func (app *Application) UpdateUserPassword(w http.ResponseWriter, r *http.Request) {
	userID := app.SessionManager.GetInt64(r.Context(), helpers.COOKIE_USER_ID)
	if userID == 0 {
		helpers.ErrorJSON(w, errors.New(helpers.NOT_AUTHORIZED_MESSAGE), http.StatusUnauthorized)
		return
	}

	var req UpdateUserPasswordRequest
	if err := helpers.ReadJSON(w, r, &req, 0); err != nil {
		helpers.ErrorJSON(w, errors.New("invalid request body"), http.StatusBadRequest)
		return
	}

	if req.CurrentPassword == "" || req.NewPassword == "" {
		helpers.ErrorJSON(w, errors.New("current and new password are required"), http.StatusBadRequest)
		return
	}

	if len(req.NewPassword) < 9 {
		helpers.ErrorJSON(w, errors.New("new password must be at least 9 characters"), http.StatusBadRequest)
		return
	}

	if len(req.NewPassword) > 128 {
		helpers.ErrorJSON(w, errors.New("new password must be 128 characters or less"), http.StatusBadRequest)
		return
	}

	// Get current user to verify password
	user, err := app.Queries.GetUser(r.Context(), userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New(helpers.NOT_AUTHORIZED_MESSAGE), http.StatusUnauthorized)
		} else {
			app.Logger.Error("failed to fetch user for password update", "error", err, "user_id", userID)
			helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		}
		return
	}

	// Verify current password
	match, err := helpers.PasswordMatches(req.CurrentPassword, user.Password)
	if err != nil {
		app.Logger.Error("failed to compare password hash", "error", err, "user_id", userID)
		helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		return
	}

	if !match {
		helpers.ErrorJSON(w, errors.New("current password is incorrect"), http.StatusUnauthorized)
		return
	}

	// Hash new password
	hashedPassword, err := helpers.HashPassword(req.NewPassword)
	if err != nil {
		app.Logger.Error("failed to hash new password", "error", err, "user_id", userID)
		helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		return
	}

	// Update password
	err = app.Queries.UpdateUserPassword(r.Context(), database.UpdateUserPasswordParams{
		Password: hashedPassword,
		ID:       userID,
	})
	if err != nil {
		app.Logger.Error("failed to update user password", "error", err, "user_id", userID)
		helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		return
	}

	app.Logger.Info("user password updated successfully", "user_id", userID)

	res := helpers.JSONResponse{
		Error:   false,
		Message: "Password updated successfully",
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

// UpdateUserAvatarRequest represents the request body for updating avatar
type UpdateUserAvatarRequest struct {
	Avatar string `json:"avatar"`
}

// UpdateUserAvatar updates the authenticated user's avatar URL
func (app *Application) UpdateUserAvatar(w http.ResponseWriter, r *http.Request) {
	userID := app.SessionManager.GetInt64(r.Context(), helpers.COOKIE_USER_ID)
	if userID == 0 {
		helpers.ErrorJSON(w, errors.New(helpers.NOT_AUTHORIZED_MESSAGE), http.StatusUnauthorized)
		return
	}

	var req UpdateUserAvatarRequest
	if err := helpers.ReadJSON(w, r, &req, 0); err != nil {
		helpers.ErrorJSON(w, errors.New("invalid request body"), http.StatusBadRequest)
		return
	}

	// Get current user to check for existing uploaded avatar
	currentUser, err := app.Queries.GetUser(r.Context(), userID)
	if err != nil {
		app.Logger.Error("failed to get user for avatar update", "error", err, "user_id", userID)
		helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		return
	}

	// Delete old uploaded avatar file if it exists
	if currentUser.Avatar.Valid && isUploadedAvatar(currentUser.Avatar.String) {
		app.deleteAvatarFile(currentUser.Avatar.String)
	}

	// Allow empty avatar to remove it
	var avatarValue sql.NullString
	if req.Avatar != "" {
		avatarValue = sql.NullString{String: req.Avatar, Valid: true}
	}

	user, err := app.Queries.UpdateUserAvatar(r.Context(), database.UpdateUserAvatarParams{
		Avatar: avatarValue,
		ID:     userID,
	})
	if err != nil {
		app.Logger.Error("failed to update user avatar", "error", err, "user_id", userID)
		helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		return
	}

	res := helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"user": map[string]any{
				"id":         user.ID,
				"name":       user.Name,
				"email":      user.Email,
				"is_admin":   user.IsAdmin,
				"avatar":     user.Avatar,
				"created_at": user.CreatedAt,
				"updated_at": user.UpdatedAt,
			},
		},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

// Allowed MIME types for avatar uploads
var allowedAvatarMimeTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/gif":  ".gif",
	"image/webp": ".webp",
	"image/avif": ".avif",
}

// Maximum file size for avatar uploads (20MB)
const maxAvatarSize = 20 << 20

// UploadUserAvatar handles file upload for user avatars
func (app *Application) UploadUserAvatar(w http.ResponseWriter, r *http.Request) {
	userID := app.SessionManager.GetInt64(r.Context(), helpers.COOKIE_USER_ID)
	if userID == 0 {
		helpers.ErrorJSON(w, errors.New(helpers.NOT_AUTHORIZED_MESSAGE), http.StatusUnauthorized)
		return
	}

	// Limit request body size
	r.Body = http.MaxBytesReader(w, r.Body, maxAvatarSize)

	// Parse multipart form
	if err := r.ParseMultipartForm(maxAvatarSize); err != nil {
		if strings.Contains(err.Error(), "request body too large") {
			helpers.ErrorJSON(w, errors.New("file too large, maximum size is 20MB"), http.StatusRequestEntityTooLarge)
		} else {
			helpers.ErrorJSON(w, errors.New("failed to parse form data"), http.StatusBadRequest)
		}
		return
	}

	// Get the uploaded file
	file, header, err := r.FormFile("avatar")
	if err != nil {
		helpers.ErrorJSON(w, errors.New("no file uploaded"), http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Read first 512 bytes to detect content type
	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		helpers.ErrorJSON(w, errors.New("failed to read file"), http.StatusBadRequest)
		return
	}

	// Detect content type from file content
	contentType := http.DetectContentType(buffer[:n])

	// Validate MIME type
	ext, ok := allowedAvatarMimeTypes[contentType]
	if !ok {
		helpers.ErrorJSON(w, errors.New("invalid file type. Allowed: JPEG, PNG, GIF, WebP, AVIF"), http.StatusBadRequest)
		return
	}

	// Reset file position to beginning
	if _, err := file.Seek(0, 0); err != nil {
		helpers.ErrorJSON(w, errors.New("failed to process file"), http.StatusInternalServerError)
		return
	}

	// Get current user to check for existing avatar
	currentUser, err := app.Queries.GetUser(r.Context(), userID)
	if err != nil {
		app.Logger.Error("failed to get user for avatar upload", "error", err, "user_id", userID)
		helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		return
	}

	// Delete old uploaded avatar file if it exists
	if currentUser.Avatar.Valid && isUploadedAvatar(currentUser.Avatar.String) {
		app.deleteAvatarFile(currentUser.Avatar.String)
	}

	// Ensure avatars directory exists
	avatarsDir := filepath.Join(app.Settings.StaticDir, "avatars")
	if err := os.MkdirAll(avatarsDir, 0755); err != nil {
		app.Logger.Error("failed to create avatars directory", "error", err)
		helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		return
	}

	// Create the file path: avatars/{user_id}.{ext}
	filename := fmt.Sprintf("%d%s", userID, ext)
	filePath := filepath.Join(avatarsDir, filename)

	// Create destination file
	dst, err := os.Create(filePath)
	if err != nil {
		app.Logger.Error("failed to create avatar file", "error", err, "path", filePath)
		helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		return
	}
	defer dst.Close()

	// Copy uploaded file to destination
	if _, err := io.Copy(dst, file); err != nil {
		app.Logger.Error("failed to write avatar file", "error", err, "path", filePath)
		helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		return
	}

	// Build the avatar URL path
	avatarURL := fmt.Sprintf("/api/static/avatars/%s", filename)

	// Update user avatar in database
	user, err := app.Queries.UpdateUserAvatar(r.Context(), database.UpdateUserAvatarParams{
		Avatar: sql.NullString{String: avatarURL, Valid: true},
		ID:     userID,
	})
	if err != nil {
		app.Logger.Error("failed to update user avatar in database", "error", err, "user_id", userID)
		// Try to clean up the uploaded file
		os.Remove(filePath)
		helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		return
	}

	app.Logger.Info("avatar uploaded successfully",
		"user_id", userID,
		"filename", filename,
		"size", header.Size,
		"content_type", contentType,
	)

	res := helpers.JSONResponse{
		Error:   false,
		Message: "Avatar uploaded successfully",
		Data: map[string]any{
			"user": map[string]any{
				"id":         user.ID,
				"name":       user.Name,
				"email":      user.Email,
				"is_admin":   user.IsAdmin,
				"avatar":     user.Avatar,
				"created_at": user.CreatedAt,
				"updated_at": user.UpdatedAt,
			},
		},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

// isUploadedAvatar checks if an avatar URL is a locally uploaded file
func isUploadedAvatar(avatarURL string) bool {
	return strings.HasPrefix(avatarURL, "/api/static/")
}

// deleteAvatarFile deletes an uploaded avatar file from disk
func (app *Application) deleteAvatarFile(avatarURL string) {
	// Extract the file path from the URL: /api/static/avatars/1.jpg -> avatars/1.jpg
	relativePath := strings.TrimPrefix(avatarURL, "/api/static/")
	fullPath := filepath.Join(app.Settings.StaticDir, relativePath)

	if err := os.Remove(fullPath); err != nil {
		if !os.IsNotExist(err) {
			app.Logger.Error("failed to delete old avatar file", "error", err, "path", fullPath)
		}
	} else {
		app.Logger.Info("deleted old avatar file", "path", fullPath)
	}
}

// DeleteUserAccount deletes the authenticated user's account
func (app *Application) DeleteUserAccount(w http.ResponseWriter, r *http.Request) {
	userID := app.SessionManager.GetInt64(r.Context(), helpers.COOKIE_USER_ID)
	if userID == 0 {
		helpers.ErrorJSON(w, errors.New(helpers.NOT_AUTHORIZED_MESSAGE), http.StatusUnauthorized)
		return
	}

	// Get user to check if they're an admin
	user, err := app.Queries.GetUser(r.Context(), userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New(helpers.NOT_AUTHORIZED_MESSAGE), http.StatusUnauthorized)
		} else {
			app.Logger.Error("failed to fetch user for deletion", "error", err, "user_id", userID)
			helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		}
		return
	}

	// Prevent admin from deleting their own account (to always have at least one admin)
	if user.IsAdmin {
		helpers.ErrorJSON(w, errors.New("admin accounts cannot be deleted"), http.StatusForbidden)
		return
	}

	// Delete the user
	err = app.Queries.DeleteUser(r.Context(), userID)
	if err != nil {
		app.Logger.Error("failed to delete user", "error", err, "user_id", userID)
		helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		return
	}

	// Destroy the session
	err = app.SessionManager.Destroy(r.Context())
	if err != nil {
		app.Logger.Error("failed to destroy session after account deletion", "error", err)
	}

	app.Logger.Info("user account deleted", "user_id", userID, "email", user.Email)

	res := helpers.JSONResponse{
		Error:   false,
		Message: "Account deleted successfully",
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}
