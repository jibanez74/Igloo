package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
)

// userResponseMap is the canonical JSON shape for the authenticated user object
// returned by the auth and user endpoints. It takes explicit fields because the
// sqlc row types (GetUserRow, UpdateUserNameRow, ...) differ per query. The
// plaintext PIN is never included — only whether one is set.
func userResponseMap(id int64, name, email string, isAdmin bool, avatar sql.NullString, hasPin bool, createdAt, updatedAt string) map[string]any {
	var avatarValue any
	if avatar.Valid {
		avatarValue = avatar.String
	}

	return map[string]any{
		"id":         id,
		"name":       name,
		"email":      email,
		"is_admin":   isAdmin,
		"avatar":     avatarValue,
		"has_pin":    hasPin,
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
	passwordBytes := len(password)
	if passwordBytes > helpers.USER_PASSWORD_MAX_BYTES {
		return fmt.Errorf("%s must be at most %d UTF-8 bytes", label, helpers.USER_PASSWORD_MAX_BYTES)
	}
	return nil
}

const (
	userNameMaxLength  = 100
	userEmailMaxLength = 255
)

// Said by both the self-service and the admin user handlers.
const (
	userNotFoundMessage    = "user not found"
	nameRequiredMessage    = "name is required"
	emailRequiredMessage   = "email is required"
	isAdminRequiredMessage = "is_admin is required"

	// The fragment SQLite puts in the error when an insert or update collides
	// with a unique index, which is how a taken email is detected.
	uniqueConstraintErrorFragment = "UNIQUE constraint"
)

// validateUserName enforces the shared name length bound in characters
// (runes), matching the web client's validation.
func validateUserName(name string) error {
	if utf8.RuneCountInString(name) > userNameMaxLength {
		return fmt.Errorf("name must be %d characters or less", userNameMaxLength)
	}
	return nil
}

// validateUserEmail checks a trimmed email for the self-service and admin
// handlers alike: required, and at most userEmailMaxLength characters. The
// format is not validated.
func validateUserEmail(email string) error {
	if email == "" {
		return errors.New(emailRequiredMessage)
	}
	if utf8.RuneCountInString(email) > userEmailMaxLength {
		return fmt.Errorf("email must be %d characters or less", userEmailMaxLength)
	}
	return nil
}

type UpdateUserNameRequest struct {
	Name string `json:"name"`
}

func (app *Application) UpdateUserName(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)

	var req UpdateUserNameRequest
	if err := helpers.ReadJSON(w, r, &req); err != nil {
		helpers.ErrorJSON(w, errors.New(invalidRequestBodyMessage), http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		helpers.ErrorJSON(w, errors.New(nameRequiredMessage), http.StatusBadRequest)
		return
	}

	if err := validateUserName(req.Name); err != nil {
		helpers.ErrorJSON(w, err, http.StatusBadRequest)
		return
	}

	user, err := app.Queries.UpdateUserName(r.Context(), database.UpdateUserNameParams{
		Name: req.Name,
		ID:   userID,
	})
	if err != nil {
		app.serverError(w, err, "failed to update user name", internalServerErrorMessage, "user_id", userID)
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Data: map[string]any{
			"user": userResponseMap(user.ID, user.Name, user.Email, user.IsAdmin, user.Avatar, user.Pin.Valid, user.CreatedAt, user.UpdatedAt),
		},
	})
}

type UpdateUserEmailRequest struct {
	Email string `json:"email"`
}

// UpdateUserEmail takes a cookie session only: the email is the sign-in name,
// so a stolen device token must not be able to move the account.
func (app *Application) UpdateUserEmail(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.requireSessionUserID(w, r)
	if !ok {
		return
	}

	var req UpdateUserEmailRequest
	if err := helpers.ReadJSON(w, r, &req); err != nil {
		helpers.ErrorJSON(w, errors.New(invalidRequestBodyMessage), http.StatusBadRequest)
		return
	}

	req.Email = strings.TrimSpace(req.Email)

	if err := validateUserEmail(req.Email); err != nil {
		helpers.ErrorJSON(w, err, http.StatusBadRequest)
		return
	}

	user, err := app.Queries.UpdateUserEmail(r.Context(), database.UpdateUserEmailParams{
		Email: req.Email,
		ID:    userID,
	})
	if err != nil {
		if strings.Contains(err.Error(), uniqueConstraintErrorFragment) {
			helpers.ErrorJSON(w, errors.New("that email address is already in use"), http.StatusConflict)
			return
		}
		app.serverError(w, err, "failed to update user email", internalServerErrorMessage, "user_id", userID)
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Data: map[string]any{
			"user": userResponseMap(user.ID, user.Name, user.Email, user.IsAdmin, user.Avatar, user.Pin.Valid, user.CreatedAt, user.UpdatedAt),
		},
	})
}

type UpdateUserPasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// UpdateUserPassword takes a cookie session only: a stolen device token must
// not be able to try passwords or lock the owner out.
func (app *Application) UpdateUserPassword(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.requireSessionUserID(w, r)
	if !ok {
		return
	}

	var req UpdateUserPasswordRequest
	if err := helpers.ReadJSON(w, r, &req); err != nil {
		helpers.ErrorJSON(w, errors.New(invalidRequestBodyMessage), http.StatusBadRequest)
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
			helpers.ErrorJSON(w, errors.New(notAuthorizedMessage), http.StatusUnauthorized)
		} else {
			app.serverError(w, err, "failed to fetch user for password update", internalServerErrorMessage, "user_id", userID)
		}
		return
	}

	// The current-password check is a password oracle, so it shares the
	// login budget's size, per user rather than per address.
	if !app.AuthLimiter.Allow("password:"+strconv.FormatInt(userID, 10), loginAttemptLimit, loginAttemptWindow) {
		helpers.ErrorJSON(w, errors.New(tooManyAttemptsMessage), http.StatusTooManyRequests)
		return
	}

	match, err := helpers.PasswordMatches(req.CurrentPassword, user.Password)
	if err != nil {
		app.serverError(w, err, comparePasswordHashLogMessage, internalServerErrorMessage, "user_id", userID)
		return
	}

	if !match {
		helpers.ErrorJSON(w, errors.New("current password is incorrect"), http.StatusUnauthorized)
		return
	}

	hashedPassword, err := helpers.HashPassword(req.NewPassword)
	if err != nil {
		app.serverError(w, err, "failed to hash new password", internalServerErrorMessage, "user_id", userID)
		return
	}

	version, err := app.changePassword(r.Context(), userID, hashedPassword)
	if err != nil {
		app.serverError(w, err, "failed to update user password", internalServerErrorMessage, "user_id", userID)
		return
	}
	app.keepSessionAfterPasswordChange(r, userID, version)

	app.Logger.Info("user password updated successfully", "user_id", userID)

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{Message: "Password updated successfully"})
}

type UpdateUserAvatarRequest struct {
	// A pointer so a missing avatar is a 400 rather than a silent clear.
	Avatar *string `json:"avatar"`
}

func (app *Application) UpdateUserAvatar(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)

	var req UpdateUserAvatarRequest
	if err := helpers.ReadJSON(w, r, &req); err != nil {
		helpers.ErrorJSON(w, errors.New(invalidRequestBodyMessage), http.StatusBadRequest)
		return
	}

	if req.Avatar == nil {
		helpers.ErrorJSON(w, errors.New("avatar is required"), http.StatusBadRequest)
		return
	}
	avatar := *req.Avatar

	// Uploaded avatars are set only by UploadUserAvatar; this endpoint takes an
	// external image URL or clears the avatar.
	if !isOptionalHTTPURL(avatar) {
		helpers.ErrorJSON(w, errors.New("avatar must be an http or https URL"), http.StatusBadRequest)
		return
	}

	var avatarValue sql.NullString
	if avatar != "" {
		avatarValue = sql.NullString{String: avatar, Valid: true}
	}

	user, replaced, err := app.replaceUserAvatar(r.Context(), userID, avatarValue)
	if err != nil {
		app.serverError(w, err, "failed to update user avatar", internalServerErrorMessage, "user_id", userID)
		return
	}

	// Only now that the account no longer points at it.
	if replaced.Valid {
		app.deleteUploadedAvatar(userID, replaced.String)
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Data: map[string]any{
			"user": userResponseMap(user.ID, user.Name, user.Email, user.IsAdmin, user.Avatar, user.Pin.Valid, user.CreatedAt, user.UpdatedAt),
		},
	})
}

// uploadedAvatarURLPrefix is the URL UploadUserAvatar publishes an avatar
// under; the file behind it is static/avatars/<userID>-<unix nanos><ext>.
const uploadedAvatarURLPrefix = "/api/static/avatars/"

var allowedAvatarMimeTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

const maxAvatarSize = 20 << 20

func (app *Application) UploadUserAvatar(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)

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
		helpers.ErrorJSON(w, errors.New("invalid file type. Allowed: JPEG, PNG, GIF, WebP"), http.StatusBadRequest)
		return
	}

	if _, err := file.Seek(0, 0); err != nil {
		helpers.ErrorJSON(w, errors.New("failed to process file"), http.StatusInternalServerError)
		return
	}

	avatarsDir := filepath.Join(app.CurrentSettings().StaticDir, "avatars")
	_, err = helpers.GetOrCreateDir(avatarsDir)
	if err != nil {
		app.serverError(w, err, "failed to create avatars directory", internalServerErrorMessage, "path", avatarsDir)
		return
	}

	// Every upload gets a file name of its own. Static files are served with
	// a year-long max-age, so a reused name would keep showing the previous
	// image, and writing over the file the account points at would change the
	// avatar even when the account update below fails.
	filename := fmt.Sprintf("%d-%d%s", userID, time.Now().UnixNano(), ext)
	filePath := filepath.Join(avatarsDir, filename)

	// A temp file renamed into place, so a failed write leaves nothing under
	// the new name.
	err = writeAvatarFile(avatarsDir, filePath, file)
	if err != nil {
		app.serverError(w, err, "failed to write avatar file", internalServerErrorMessage, "path", filePath)
		return
	}

	avatarURL := uploadedAvatarURLPrefix + filename

	user, replaced, err := app.replaceUserAvatar(r.Context(), userID, sql.NullString{String: avatarURL, Valid: true})
	if err != nil {
		app.Logger.Error("failed to update user avatar in database", "error", err, "user_id", userID)
		_ = os.Remove(filePath)
		helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
		return
	}

	if replaced.Valid {
		app.deleteUploadedAvatar(userID, replaced.String)
	}

	app.Logger.Info("avatar uploaded successfully",
		"user_id", userID,
		"filename", filename,
		"size", header.Size,
		"content_type", contentType,
	)

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Message: "Avatar uploaded successfully",
		Data: map[string]any{
			"user": userResponseMap(user.ID, user.Name, user.Email, user.IsAdmin, user.Avatar, user.Pin.Valid, user.CreatedAt, user.UpdatedAt),
		},
	})
}

// replaceUserAvatar points the account at avatar and returns the value it
// replaced, read in the same transaction. The pool's single connection
// serializes transactions, so of two concurrent changes each gets the value it
// actually replaced and deletes only that file; reading the old value first
// would let both delete the same file and orphan the first new upload.
func (app *Application) replaceUserAvatar(ctx context.Context, userID int64, avatar sql.NullString) (database.UpdateUserAvatarRow, sql.NullString, error) {
	tx, err := app.DB.BeginTx(ctx, nil)
	if err != nil {
		return database.UpdateUserAvatarRow{}, sql.NullString{}, err
	}
	defer func() { _ = tx.Rollback() }()

	qtx := app.Queries.WithTx(tx)
	current, err := qtx.GetUser(ctx, userID)
	if err != nil {
		return database.UpdateUserAvatarRow{}, sql.NullString{}, err
	}
	user, err := qtx.UpdateUserAvatar(ctx, database.UpdateUserAvatarParams{
		Avatar: avatar,
		ID:     userID,
	})
	if err != nil {
		return database.UpdateUserAvatarRow{}, sql.NullString{}, err
	}
	err = tx.Commit()
	if err != nil {
		return database.UpdateUserAvatarRow{}, sql.NullString{}, err
	}
	return user, current.Avatar, nil
}

// uploadedAvatarFileName returns the file name an uploaded avatar URL points
// at, or "" for any other avatar value.
func uploadedAvatarFileName(avatarURL string) string {
	name, ok := strings.CutPrefix(avatarURL, uploadedAvatarURLPrefix)
	if !ok {
		return ""
	}
	return name
}

// isOwnUploadedAvatarName reports whether name has the exact shape
// UploadUserAvatar gives userID's files: <userID>-<digits><allowed ext>. The
// name comes from a stored value, and this shape cannot reach another user's
// file or leave the avatars directory.
func isOwnUploadedAvatarName(userID int64, name string) bool {
	rest, ok := strings.CutPrefix(name, strconv.FormatInt(userID, 10)+"-")
	if !ok {
		return false
	}
	ext := filepath.Ext(rest)
	stamp := strings.TrimSuffix(rest, ext)
	if stamp == "" || strings.Trim(stamp, "0123456789") != "" {
		return false
	}
	for _, allowed := range allowedAvatarMimeTypes {
		if ext == allowed {
			return true
		}
	}
	return false
}

func writeAvatarFile(dir, path string, src io.Reader) error {
	tmp, err := os.CreateTemp(dir, ".avatar-*")
	if err != nil {
		return err
	}
	_, err = io.Copy(tmp, src)
	closeErr := tmp.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(tmp.Name(), path)
	}
	if err != nil {
		_ = os.Remove(tmp.Name())
	}
	return err
}

// deleteUploadedAvatar removes the file UploadUserAvatar wrote for userID.
// The path comes from a stored value, so only the exact name the upload
// produces for this user is deleted; any other value, including another user's
// file or one that climbs out of the static directory, is left alone.
func (app *Application) deleteUploadedAvatar(userID int64, avatarURL string) {
	name := uploadedAvatarFileName(avatarURL)
	if !isOwnUploadedAvatarName(userID, name) {
		return
	}

	fullPath := filepath.Join(app.CurrentSettings().StaticDir, "avatars", name)
	if err := os.Remove(fullPath); err != nil {
		if !os.IsNotExist(err) {
			app.Logger.Error("failed to delete old avatar file", "error", err, "path", fullPath)
		}
	} else {
		app.Logger.Info("deleted old avatar file", "path", fullPath)
	}
}

// DeleteUserAccount takes a cookie session only, like the other account
// changes a stolen device token must not reach.
func (app *Application) DeleteUserAccount(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.requireSessionUserID(w, r)
	if !ok {
		return
	}

	user, err := app.Queries.GetUser(r.Context(), userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New(notAuthorizedMessage), http.StatusUnauthorized)
		} else {
			app.serverError(w, err, "failed to fetch user for deletion", internalServerErrorMessage, "user_id", userID)
		}
		return
	}

	if user.IsAdmin {
		helpers.ErrorJSON(w, errors.New("admin accounts cannot be deleted"), http.StatusForbidden)
		return
	}

	roomIDs, err := app.deleteUserWithRooms(r.Context(), userID)
	if err != nil {
		app.serverError(w, err, "failed to delete user", internalServerErrorMessage, "user_id", userID)
		return
	}

	app.endDeletedOwnerRooms(roomIDs)
	app.forgetDeletedSessionUser(userID)
	app.forgetUserDevices(userID)

	if user.Avatar.Valid {
		app.deleteUploadedAvatar(userID, user.Avatar.String)
	}

	err = app.SessionManager.Destroy(r.Context())
	if err != nil {
		app.Logger.Error("failed to destroy session after account deletion", "error", err)
	}

	app.Logger.Info("user account deleted", "user_id", userID, "email", user.Email)

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{Message: "Account deleted successfully"})
}

// deleteUserWithRooms deletes the user and returns the ids of the watch rooms
// the cascade removed with them, read in the same transaction.
func (app *Application) deleteUserWithRooms(ctx context.Context, userID int64) ([]int64, error) {
	tx, err := app.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	qtx := app.Queries.WithTx(tx)
	roomIDs, err := qtx.ListWatchRoomIDsByOwnerID(ctx, userID)
	if err != nil {
		return nil, err
	}
	err = qtx.DeleteUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	return roomIDs, tx.Commit()
}
