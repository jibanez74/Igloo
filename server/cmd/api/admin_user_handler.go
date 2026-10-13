package main

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
)

func (app *Application) AdminGetUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := app.Queries.GetAllUsers(r.Context())
	if err != nil {
		app.serverError(w, err, "admin: failed to fetch users", internalServerErrorMessage)
		return
	}

	users := make([]userResponse, 0, len(rows))
	for _, row := range rows {
		users = append(users, newUserResponse(
			row.ID, row.Name, row.Email, row.IsAdmin,
			row.Avatar, row.HasPin, row.CreatedAt, row.UpdatedAt,
		))
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{Data: map[string]any{"users": users}})
}

type AdminCreateUserRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
	// A pointer so a missing flag is a 400 instead of a silent false.
	IsAdmin *bool `json:"is_admin"`
}

func (app *Application) AdminCreateUser(w http.ResponseWriter, r *http.Request) {
	var req AdminCreateUserRequest
	if err := helpers.ReadJSON(w, r, &req); err != nil {
		helpers.ErrorJSON(w, errors.New(invalidRequestBodyMessage), http.StatusBadRequest)
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.TrimSpace(req.Email)

	if err := validateUserName(req.Name); err != nil {
		helpers.ErrorJSON(w, err, http.StatusBadRequest)
		return
	}

	if err := validateUserEmail(req.Email); err != nil {
		helpers.ErrorJSON(w, err, http.StatusBadRequest)
		return
	}

	if req.IsAdmin == nil {
		helpers.ErrorJSON(w, errors.New(isAdminRequiredMessage), http.StatusBadRequest)
		return
	}

	if err := validatePassword(req.Password, "password"); err != nil {
		helpers.ErrorJSON(w, err, http.StatusBadRequest)
		return
	}

	hashedPassword, err := helpers.HashPassword(req.Password)
	if err != nil {
		app.serverError(w, err, "admin: failed to hash password for new user", internalServerErrorMessage)
		return
	}

	user, err := app.Queries.CreateUser(r.Context(), database.CreateUserParams{
		Name:     req.Name,
		Email:    req.Email,
		Password: hashedPassword,
		IsAdmin:  *req.IsAdmin,
	})
	if err != nil {
		if strings.Contains(err.Error(), uniqueConstraintErrorFragment) {
			helpers.ErrorJSON(w, errors.New("a user with that email already exists"), http.StatusConflict)
			return
		}
		app.serverError(w, err, "admin: failed to create user", internalServerErrorMessage)
		return
	}

	app.Logger.Info("admin: user created", "user_id", user.ID, "email", user.Email)

	helpers.WriteJSON(w, http.StatusCreated, helpers.JSONResponse{
		Data: map[string]any{
			"user": newUserResponse(
				user.ID, user.Name, user.Email, user.IsAdmin,
				user.Avatar, user.Pin.Valid, user.CreatedAt, user.UpdatedAt,
			),
		},
	})
}

type AdminUpdateUserRequest struct {
	Name    string `json:"name"`
	Email   string `json:"email"`
	IsAdmin *bool  `json:"is_admin"`
}

func (app *Application) AdminUpdateUser(w http.ResponseWriter, r *http.Request) {
	targetID, ok := parseIDParam(w, r, "id", "invalid user ID")
	if !ok {
		return
	}

	var req AdminUpdateUserRequest
	if err := helpers.ReadJSON(w, r, &req); err != nil {
		helpers.ErrorJSON(w, errors.New(invalidRequestBodyMessage), http.StatusBadRequest)
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.TrimSpace(req.Email)

	if err := validateUserName(req.Name); err != nil {
		helpers.ErrorJSON(w, err, http.StatusBadRequest)
		return
	}

	if err := validateUserEmail(req.Email); err != nil {
		helpers.ErrorJSON(w, err, http.StatusBadRequest)
		return
	}

	// Missing must not read as false: that would demote another admin.
	if req.IsAdmin == nil {
		helpers.ErrorJSON(w, errors.New(isAdminRequiredMessage), http.StatusBadRequest)
		return
	}
	isAdmin := *req.IsAdmin

	// Do not let an admin lock themselves out.
	currentUserID := app.userIDFromRequest(r)
	if targetID == currentUserID && !isAdmin {
		helpers.ErrorJSON(w, errors.New("you cannot remove your own admin status"), http.StatusForbidden)
		return
	}

	tx, err := app.DB.BeginTx(r.Context(), nil)
	if err != nil {
		app.serverError(w, err, "admin: failed to begin user update transaction", internalServerErrorMessage,
			"target_id", targetID)
		return
	}

	defer func() { _ = tx.Rollback() }()

	qtx := app.Queries.WithTx(tx)

	existing, err := qtx.GetUser(r.Context(), targetID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New(userNotFoundMessage), http.StatusNotFound)
			return
		}
		app.serverError(w, err, "admin: failed to fetch user for update", internalServerErrorMessage, "target_id", targetID)
		return
	}

	// Keep at least one admin account.
	if !isAdmin && existing.IsAdmin {
		count, err := qtx.CountAdmins(r.Context())
		if err != nil {
			app.serverError(w, err, "admin: failed to count admins during update", internalServerErrorMessage,
				"target_id", targetID)
			return
		}

		if count <= 1 {
			helpers.ErrorJSON(w, errors.New("cannot remove admin status from the last admin account"), http.StatusForbidden)
			return
		}
	}

	user, err := qtx.AdminUpdateUser(r.Context(), database.AdminUpdateUserParams{
		Name:    req.Name,
		Email:   req.Email,
		IsAdmin: isAdmin,
		ID:      targetID,
	})
	if err != nil {
		if strings.Contains(err.Error(), uniqueConstraintErrorFragment) {
			helpers.ErrorJSON(w, errors.New("a user with that email already exists"), http.StatusConflict)
			return
		}
		app.serverError(w, err, "admin: failed to update user", internalServerErrorMessage, "target_id", targetID)
		return
	}

	err = tx.Commit()
	if err != nil {
		app.serverError(w, err, "admin: failed to commit user update transaction", internalServerErrorMessage,
			"target_id", targetID)
		return
	}

	app.Logger.Info("admin: user updated", "user_id", user.ID)

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Data: map[string]any{
			"user": newUserResponse(
				user.ID, user.Name, user.Email, user.IsAdmin,
				user.Avatar, user.Pin.Valid, user.CreatedAt, user.UpdatedAt,
			),
		},
	})
}

func (app *Application) AdminDeleteUser(w http.ResponseWriter, r *http.Request) {
	targetID, ok := parseIDParam(w, r, "id", "invalid user ID")
	if !ok {
		return
	}

	currentUserID := app.userIDFromRequest(r)
	if targetID == currentUserID {
		helpers.ErrorJSON(w, errors.New("you cannot delete your own account"), http.StatusForbidden)
		return
	}

	tx, err := app.DB.BeginTx(r.Context(), nil)
	if err != nil {
		app.serverError(w, err, "admin: failed to begin user deletion transaction", internalServerErrorMessage,
			"target_id", targetID)
		return
	}

	defer func() { _ = tx.Rollback() }()

	qtx := app.Queries.WithTx(tx)

	user, err := qtx.GetUser(r.Context(), targetID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New(userNotFoundMessage), http.StatusNotFound)
			return
		}
		app.serverError(w, err, "admin: failed to fetch user for deletion", internalServerErrorMessage, "target_id", targetID)
		return
	}

	// Keep at least one admin account.
	if user.IsAdmin {
		count, err := qtx.CountAdmins(r.Context())
		if err != nil {
			app.serverError(w, err, "admin: failed to count admins during deletion", internalServerErrorMessage,
				"target_id", targetID)
			return
		}

		if count <= 1 {
			helpers.ErrorJSON(w, errors.New("cannot delete the last admin account"), http.StatusForbidden)
			return
		}
	}

	roomIDs, err := qtx.ListWatchRoomIDsByOwnerID(r.Context(), targetID)
	if err != nil {
		app.serverError(w, err, "admin: failed to list the user's watch rooms", internalServerErrorMessage, "target_id", targetID)
		return
	}

	err = qtx.DeleteUser(r.Context(), targetID)
	if err != nil {
		app.serverError(w, err, "admin: failed to delete user", internalServerErrorMessage, "target_id", targetID)
		return
	}

	err = tx.Commit()
	if err != nil {
		app.serverError(w, err, "admin: failed to commit user deletion transaction", internalServerErrorMessage,
			"target_id", targetID)
		return
	}

	app.endDeletedOwnerRooms(roomIDs)
	app.forgetDeletedSessionUser(targetID)
	app.forgetUserDevices(targetID)

	if user.Avatar.Valid {
		app.deleteUploadedAvatar(targetID, user.Avatar.String)
	}

	app.Logger.Info("admin: user deleted", "user_id", targetID, "email", user.Email)

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{Message: "User deleted successfully"})
}

type AdminResetUserPasswordRequest struct {
	Password string `json:"password"`
}

func (app *Application) AdminResetUserPassword(w http.ResponseWriter, r *http.Request) {
	targetID, ok := parseIDParam(w, r, "id", "invalid user ID")
	if !ok {
		return
	}

	var req AdminResetUserPasswordRequest
	if err := helpers.ReadJSON(w, r, &req); err != nil {
		helpers.ErrorJSON(w, errors.New(invalidRequestBodyMessage), http.StatusBadRequest)
		return
	}

	if err := validatePassword(req.Password, "password"); err != nil {
		helpers.ErrorJSON(w, err, http.StatusBadRequest)
		return
	}

	exists, err := app.Queries.UserExists(r.Context(), targetID)
	if err != nil {
		app.serverError(w, err, "admin: failed to check user for password reset", internalServerErrorMessage,
			"target_id", targetID)
		return
	}
	if !exists {
		helpers.ErrorJSON(w, errors.New(userNotFoundMessage), http.StatusNotFound)
		return
	}

	hashedPassword, err := helpers.HashPassword(req.Password)
	if err != nil {
		app.serverError(w, err, "admin: failed to hash password for reset", internalServerErrorMessage)
		return
	}

	version, err := app.changePassword(r.Context(), targetID, hashedPassword)
	if err != nil {
		app.serverError(w, err, "admin: failed to reset user password", internalServerErrorMessage, "target_id", targetID)
		return
	}
	// An admin resetting their own password from a browser keeps that session.
	if targetID == app.userIDFromRequest(r) && deviceAuthFrom(r.Context()) == nil {
		app.keepSessionAfterPasswordChange(r, targetID, version)
	}

	app.Logger.Info("admin: user password reset", "user_id", targetID)

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{Message: "Password reset successfully"})
}
