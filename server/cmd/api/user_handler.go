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
	"unicode/utf8"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
)

// userResponseMap is the canonical JSON shape for the authenticated user object
// returned by the auth and user endpoints. It takes explicit fields because the
// sqlc row types (GetUserRow, UpdateUserNameRow, ...) differ per query. Avatar is
// kept as the raw sql.NullString to preserve the existing serialized shape.
func userResponseMap(id int64, name, email string, isAdmin bool, avatar sql.NullString, createdAt, updatedAt string) map[string]any {
	return map[string]any{
		"id":         id,
		"name":       name,
		"email":      email,
		"is_admin":   isAdmin,
		"avatar":     avatar,
		"created_at": createdAt,
		"updated_at": updatedAt,
	}
}

// validatePassword enforces the shared password length bounds. label is the noun
// used in the error message (e.g. "password" or "new password").
func validatePassword(password, label string) error {
	passwordLength := utf8.RuneCountInString(password)
	if passwordLength < 9 {
		return fmt.Errorf("%s must be at least 9 characters", label)
	}
	if passwordLength > 128 {
		return fmt.Errorf("%s must be 128 characters or less", label)
	}
	return nil
}

type UpdateUserNameRequest struct {
	Name string `json:"name"`
}

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
			"user": userResponseMap(user.ID, user.Name, user.Email, user.IsAdmin, user.Avatar, user.CreatedAt, user.UpdatedAt),
		},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

type UpdateUserEmailRequest struct {
	Email string `json:"email"`
}

func (app *Application) UpdateUserEmail(w http.ResponseWriter, r *http.Request) {
	userID := app.SessionManager.GetInt64(r.Context(), helpers.COOKIE_USER_ID)
	if userID == 0 {
		helpers.ErrorJSON(w, errors.New(helpers.NOT_AUTHORIZED_MESSAGE), http.StatusUnauthorized)
		return
	}

	var req UpdateUserEmailRequest
	if err := helpers.ReadJSON(w, r, &req, 0); err != nil {
		helpers.ErrorJSON(w, errors.New("invalid request body"), http.StatusBadRequest)
		return
	}

	req.Email = strings.TrimSpace(req.Email)

	if req.Email == "" {
		helpers.ErrorJSON(w, errors.New("email is required"), http.StatusBadRequest)
		return
	}

	if len(req.Email) > 255 {
		helpers.ErrorJSON(w, errors.New("email must be 255 characters or less"), http.StatusBadRequest)
		return
	}

	user, err := app.Queries.UpdateUserEmail(r.Context(), database.UpdateUserEmailParams{
		Email: req.Email,
		ID:    userID,
	})
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			helpers.ErrorJSON(w, errors.New("that email address is already in use"), http.StatusConflict)
			return
		}
		app.Logger.Error("failed to update user email", "error", err, "user_id", userID)
		helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		return
	}

	res := helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"user": userResponseMap(user.ID, user.Name, user.Email, user.IsAdmin, user.Avatar, user.CreatedAt, user.UpdatedAt),
		},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

type UpdateUserPasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

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

	if err := validatePassword(req.NewPassword, "new password"); err != nil {
		helpers.ErrorJSON(w, err, http.StatusBadRequest)
		return
	}

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

	hashedPassword, err := helpers.HashPassword(req.NewPassword)
	if err != nil {
		app.Logger.Error("failed to hash new password", "error", err, "user_id", userID)
		helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		return
	}

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

type UpdateUserAvatarRequest struct {
	Avatar string `json:"avatar"`
}

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

	currentUser, err := app.Queries.GetUser(r.Context(), userID)
	if err != nil {
		app.Logger.Error("failed to get user for avatar update", "error", err, "user_id", userID)
		helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		return
	}

	if currentUser.Avatar.Valid && isUploadedAvatar(currentUser.Avatar.String) {
		app.deleteAvatarFile(currentUser.Avatar.String)
	}

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
			"user": userResponseMap(user.ID, user.Name, user.Email, user.IsAdmin, user.Avatar, user.CreatedAt, user.UpdatedAt),
		},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

var allowedAvatarMimeTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/gif":  ".gif",
	"image/webp": ".webp",
	"image/avif": ".avif",
}

const maxAvatarSize = 20 << 20

func (app *Application) UploadUserAvatar(w http.ResponseWriter, r *http.Request) {
	userID := app.SessionManager.GetInt64(r.Context(), helpers.COOKIE_USER_ID)
	if userID == 0 {
		helpers.ErrorJSON(w, errors.New(helpers.NOT_AUTHORIZED_MESSAGE), http.StatusUnauthorized)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxAvatarSize)

	if err := r.ParseMultipartForm(maxAvatarSize); err != nil {
		if strings.Contains(err.Error(), "request body too large") {
			helpers.ErrorJSON(w, errors.New("file too large, maximum size is 20MB"), http.StatusRequestEntityTooLarge)
		} else {
			helpers.ErrorJSON(w, errors.New("failed to parse form data"), http.StatusBadRequest)
		}
		return
	}

	file, header, err := r.FormFile("avatar")
	if err != nil {
		helpers.ErrorJSON(w, errors.New("no file uploaded"), http.StatusBadRequest)
		return
	}
	defer file.Close()

	var buffer [512]byte
	n, err := file.Read(buffer[:])
	if err != nil && err != io.EOF {
		helpers.ErrorJSON(w, errors.New("failed to read file"), http.StatusBadRequest)
		return
	}

	contentType := http.DetectContentType(buffer[:n])

	ext, ok := allowedAvatarMimeTypes[contentType]
	if !ok {
		helpers.ErrorJSON(w, errors.New("invalid file type. Allowed: JPEG, PNG, GIF, WebP, AVIF"), http.StatusBadRequest)
		return
	}

	if _, err := file.Seek(0, 0); err != nil {
		helpers.ErrorJSON(w, errors.New("failed to process file"), http.StatusInternalServerError)
		return
	}

	currentUser, err := app.Queries.GetUser(r.Context(), userID)
	if err != nil {
		app.Logger.Error("failed to get user for avatar upload", "error", err, "user_id", userID)
		helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		return
	}

	if currentUser.Avatar.Valid && isUploadedAvatar(currentUser.Avatar.String) {
		app.deleteAvatarFile(currentUser.Avatar.String)
	}

	avatarsDir := filepath.Join(app.Settings.StaticDir, "avatars")
	if err := os.MkdirAll(avatarsDir, 0755); err != nil {
		app.Logger.Error("failed to create avatars directory", "error", err)
		helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		return
	}

	filename := fmt.Sprintf("%d%s", userID, ext)
	filePath := filepath.Join(avatarsDir, filename)

	dst, err := os.Create(filePath)
	if err != nil {
		app.Logger.Error("failed to create avatar file", "error", err, "path", filePath)
		helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		app.Logger.Error("failed to write avatar file", "error", err, "path", filePath)
		helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		return
	}

	avatarURL := fmt.Sprintf("/api/static/avatars/%s", filename)

	user, err := app.Queries.UpdateUserAvatar(r.Context(), database.UpdateUserAvatarParams{
		Avatar: sql.NullString{String: avatarURL, Valid: true},
		ID:     userID,
	})
	if err != nil {
		app.Logger.Error("failed to update user avatar in database", "error", err, "user_id", userID)
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
			"user": userResponseMap(user.ID, user.Name, user.Email, user.IsAdmin, user.Avatar, user.CreatedAt, user.UpdatedAt),
		},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

func isUploadedAvatar(avatarURL string) bool {
	return strings.HasPrefix(avatarURL, "/api/static/")
}

func (app *Application) deleteAvatarFile(avatarURL string) {
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

func (app *Application) DeleteUserAccount(w http.ResponseWriter, r *http.Request) {
	userID := app.SessionManager.GetInt64(r.Context(), helpers.COOKIE_USER_ID)
	if userID == 0 {
		helpers.ErrorJSON(w, errors.New(helpers.NOT_AUTHORIZED_MESSAGE), http.StatusUnauthorized)
		return
	}

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

	if user.IsAdmin {
		helpers.ErrorJSON(w, errors.New("admin accounts cannot be deleted"), http.StatusForbidden)
		return
	}

	err = app.Queries.DeleteUser(r.Context(), userID)
	if err != nil {
		app.Logger.Error("failed to delete user", "error", err, "user_id", userID)
		helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		return
	}

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
