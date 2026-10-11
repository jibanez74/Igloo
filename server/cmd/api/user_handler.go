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
	userID, ok := app.currentUserID(w, r)
	if !ok {
		return
	}

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
		app.Logger.Error("failed to update user name", "error", err, "user_id", userID)
		helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
		return
	}

	res := helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"user": userResponseMap(user.ID, user.Name, user.Email, user.IsAdmin, user.Avatar, user.Pin.Valid, user.CreatedAt, user.UpdatedAt),
		},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
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
		app.Logger.Error("failed to update user email", "error", err, "user_id", userID)
		helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
		return
	}

	res := helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"user": userResponseMap(user.ID, user.Name, user.Email, user.IsAdmin, user.Avatar, user.Pin.Valid, user.CreatedAt, user.UpdatedAt),
		},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
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
			app.Logger.Error("failed to fetch user for password update", "error", err, "user_id", userID)
			helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
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
		app.Logger.Error(comparePasswordHashLogMessage, "error", err, "user_id", userID)
		helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
		return
	}

	if !match {
		helpers.ErrorJSON(w, errors.New("current password is incorrect"), http.StatusUnauthorized)
		return
	}

	hashedPassword, err := helpers.HashPassword(req.NewPassword)
	if err != nil {
		app.Logger.Error("failed to hash new password", "error", err, "user_id", userID)
		helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
		return
	}

	version, err := app.changePassword(r.Context(), userID, hashedPassword)
	if err != nil {
		app.Logger.Error("failed to update user password", "error", err, "user_id", userID)
		helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
		return
	}
	app.keepSessionAfterPasswordChange(r, userID, version)

	app.Logger.Info("user password updated successfully", "user_id", userID)

	res := helpers.JSONResponse{
		Error:   false,
		Message: "Password updated successfully",
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

type UpdateUserAvatarRequest struct {
	// A pointer so a missing avatar is a 400 rather than a silent clear.
	Avatar *string `json:"avatar"`
}

func (app *Application) UpdateUserAvatar(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.currentUserID(w, r)
	if !ok {
		return
	}

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

	currentUser, err := app.Queries.GetUser(r.Context(), userID)
	if err != nil {
		app.Logger.Error("failed to get user for avatar update", "error", err, "user_id", userID)
		helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
		return
	}

	var avatarValue sql.NullString
	if avatar != "" {
		avatarValue = sql.NullString{String: avatar, Valid: true}
	}

	user, err := app.Queries.UpdateUserAvatar(r.Context(), database.UpdateUserAvatarParams{
		Avatar: avatarValue,
		ID:     userID,
	})
	if err != nil {
		app.Logger.Error("failed to update user avatar", "error", err, "user_id", userID)
		helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
		return
	}

	// Only now that the account no longer points at it.
	if currentUser.Avatar.Valid {
		app.deleteUploadedAvatar(userID, currentUser.Avatar.String)
	}

	res := helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"user": userResponseMap(user.ID, user.Name, user.Email, user.IsAdmin, user.Avatar, user.Pin.Valid, user.CreatedAt, user.UpdatedAt),
		},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

// uploadedAvatarURLPrefix is the URL UploadUserAvatar publishes an avatar
// under; the file behind it is static/avatars/<userID><ext>.
const uploadedAvatarURLPrefix = "/api/static/avatars/"

var allowedAvatarMimeTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

const maxAvatarSize = 20 << 20

func (app *Application) UploadUserAvatar(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.currentUserID(w, r)
	if !ok {
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
		helpers.ErrorJSON(w, errors.New("invalid file type. Allowed: JPEG, PNG, GIF, WebP"), http.StatusBadRequest)
		return
	}

	if _, err := file.Seek(0, 0); err != nil {
		helpers.ErrorJSON(w, errors.New("failed to process file"), http.StatusInternalServerError)
		return
	}

	currentUser, err := app.Queries.GetUser(r.Context(), userID)
	if err != nil {
		app.Logger.Error("failed to get user for avatar upload", "error", err, "user_id", userID)
		helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
		return
	}

	avatarsDir := filepath.Join(app.CurrentSettings().StaticDir, "avatars")
	_, err = helpers.GetOrCreateDir(avatarsDir)
	if err != nil {
		app.Logger.Error("failed to create avatars directory", "error", err, "path", avatarsDir)
		helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
		return
	}

	filename := fmt.Sprintf("%d%s", userID, ext)
	filePath := filepath.Join(avatarsDir, filename)

	// A temp file renamed into place: a failed write never leaves a truncated
	// image under the name the account may already point at.
	err = writeAvatarFile(avatarsDir, filePath, file)
	if err != nil {
		app.Logger.Error("failed to write avatar file", "error", err, "path", filePath)
		helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
		return
	}

	// The file name repeats on every upload of the same type, and static
	// files are served with a year-long max-age, so the version makes each
	// upload a new URL that caches have not seen.
	avatarURL := fmt.Sprintf("%s%s?v=%d", uploadedAvatarURLPrefix, filename, time.Now().UnixMilli())
	previousFile := ""
	if currentUser.Avatar.Valid {
		previousFile = uploadedAvatarFileName(currentUser.Avatar.String)
	}

	user, err := app.Queries.UpdateUserAvatar(r.Context(), database.UpdateUserAvatarParams{
		Avatar: sql.NullString{String: avatarURL, Valid: true},
		ID:     userID,
	})
	if err != nil {
		app.Logger.Error("failed to update user avatar in database", "error", err, "user_id", userID)
		// The account still points at the previous upload, which shares this
		// name when the type is unchanged; only a new name can go.
		if previousFile != filename {
			_ = os.Remove(filePath)
		}
		helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
		return
	}

	if currentUser.Avatar.Valid && previousFile != filename {
		app.deleteUploadedAvatar(userID, currentUser.Avatar.String)
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
			"user": userResponseMap(user.ID, user.Name, user.Email, user.IsAdmin, user.Avatar, user.Pin.Valid, user.CreatedAt, user.UpdatedAt),
		},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

// uploadedAvatarFileName returns the file name an uploaded avatar URL points
// at, without its version query, or "" for any other avatar value.
func uploadedAvatarFileName(avatarURL string) string {
	name, ok := strings.CutPrefix(avatarURL, uploadedAvatarURLPrefix)
	if !ok {
		return ""
	}
	name, _, _ = strings.Cut(name, "?")
	return name
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
	if name == "" {
		return
	}

	ownUpload := false
	for _, ext := range allowedAvatarMimeTypes {
		if name == fmt.Sprintf("%d%s", userID, ext) {
			ownUpload = true
			break
		}
	}
	if !ownUpload {
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
			app.Logger.Error("failed to fetch user for deletion", "error", err, "user_id", userID)
			helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
		}
		return
	}

	if user.IsAdmin {
		helpers.ErrorJSON(w, errors.New("admin accounts cannot be deleted"), http.StatusForbidden)
		return
	}

	roomIDs, err := app.deleteUserWithRooms(r.Context(), userID)
	if err != nil {
		app.Logger.Error("failed to delete user", "error", err, "user_id", userID)
		helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
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

	res := helpers.JSONResponse{
		Error:   false,
		Message: "Account deleted successfully",
	}

	helpers.WriteJSON(w, http.StatusOK, res)
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
