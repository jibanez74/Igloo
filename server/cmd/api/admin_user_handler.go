package main

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"

	"github.com/go-chi/chi/v5"
)

type adminUserSummary struct {
	ID        int64   `json:"id"`
	Name      string  `json:"name"`
	Email     string  `json:"email"`
	IsAdmin   bool    `json:"is_admin"`
	Avatar    *string `json:"avatar"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

func adminUserRow(id int64, name, email string, isAdmin bool, avatar sql.NullString, createdAt, updatedAt string) adminUserSummary {
	var av *string
	if avatar.Valid {
		av = &avatar.String
	}
	return adminUserSummary{
		ID:        id,
		Name:      name,
		Email:     email,
		IsAdmin:   isAdmin,
		Avatar:    av,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}
}

func (app *Application) AdminGetUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := app.Queries.GetAllUsers(r.Context())
	if err != nil {
		app.Logger.Error("admin: failed to fetch users", "error", err)
		helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
		return
	}

	users := make([]adminUserSummary, 0, len(rows))
	for _, row := range rows {
		users = append(users, adminUserRow(
			row.ID, row.Name, row.Email, row.IsAdmin,
			row.Avatar, row.CreatedAt, row.UpdatedAt,
		))
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error: false,
		Data:  map[string]any{"users": users},
	})
}

type AdminCreateUserRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
	IsAdmin  bool   `json:"is_admin"`
}

func (app *Application) AdminCreateUser(w http.ResponseWriter, r *http.Request) {
	var req AdminCreateUserRequest
	if err := helpers.ReadJSON(w, r, &req, 0); err != nil {
		helpers.ErrorJSON(w, errors.New("invalid request body"), http.StatusBadRequest)
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.TrimSpace(req.Email)

	if req.Name == "" {
		helpers.ErrorJSON(w, errors.New("name is required"), http.StatusBadRequest)
		return
	}

	if err := validateUserName(req.Name); err != nil {
		helpers.ErrorJSON(w, err, http.StatusBadRequest)
		return
	}

	if req.Email == "" {
		helpers.ErrorJSON(w, errors.New("email is required"), http.StatusBadRequest)
		return
	}

	if err := validatePassword(req.Password, "password"); err != nil {
		helpers.ErrorJSON(w, err, http.StatusBadRequest)
		return
	}

	hashedPassword, err := helpers.HashPassword(req.Password)
	if err != nil {
		app.Logger.Error("admin: failed to hash password for new user", "error", err)
		helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
		return
	}

	user, err := app.Queries.CreateUser(r.Context(), database.CreateUserParams{
		Name:     req.Name,
		Email:    req.Email,
		Password: hashedPassword,
		IsAdmin:  req.IsAdmin,
		Avatar:   sql.NullString{},
	})
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			helpers.ErrorJSON(w, errors.New("a user with that email already exists"), http.StatusConflict)
			return
		}
		app.Logger.Error("admin: failed to create user", "error", err)
		helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
		return
	}

	app.Logger.Info("admin: user created", "user_id", user.ID, "email", user.Email)

	helpers.WriteJSON(w, http.StatusCreated, helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"user": adminUserRow(
				user.ID, user.Name, user.Email, user.IsAdmin,
				user.Avatar, user.CreatedAt, user.UpdatedAt,
			),
		},
	})
}

type AdminUpdateUserRequest struct {
	Name    string `json:"name"`
	Email   string `json:"email"`
	IsAdmin bool   `json:"is_admin"`
}

func (app *Application) AdminUpdateUser(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	targetID, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid user ID"), http.StatusBadRequest)
		return
	}

	var req AdminUpdateUserRequest
	if err := helpers.ReadJSON(w, r, &req, 0); err != nil {
		helpers.ErrorJSON(w, errors.New("invalid request body"), http.StatusBadRequest)
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.TrimSpace(req.Email)

	if req.Name == "" {
		helpers.ErrorJSON(w, errors.New("name is required"), http.StatusBadRequest)
		return
	}

	if err := validateUserName(req.Name); err != nil {
		helpers.ErrorJSON(w, err, http.StatusBadRequest)
		return
	}

	if req.Email == "" {
		helpers.ErrorJSON(w, errors.New("email is required"), http.StatusBadRequest)
		return
	}

	// Do not let an admin lock themselves out.
	currentUserID := app.userIDFromRequest(r)
	if targetID == currentUserID && !req.IsAdmin {
		helpers.ErrorJSON(w, errors.New("you cannot remove your own admin status"), http.StatusForbidden)
		return
	}

	tx, err := app.DB.BeginTx(r.Context(), nil)
	if err != nil {
		app.Logger.Error("admin: failed to begin user update transaction", "error", err, "target_id", targetID)
		helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
		return
	}

	qtx := app.Queries.WithTx(tx)

	existing, err := qtx.GetUser(r.Context(), targetID)
	if err != nil {
		_ = tx.Rollback()
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("user not found"), http.StatusNotFound)
			return
		}
		app.Logger.Error("admin: failed to fetch user for update", "error", err, "target_id", targetID)
		helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
		return
	}

	// Keep at least one admin account.
	if !req.IsAdmin && existing.IsAdmin {
		count, err := qtx.CountAdmins(r.Context())
		if err != nil {
			_ = tx.Rollback()
			app.Logger.Error("admin: failed to count admins during update", "error", err, "target_id", targetID)
			helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
			return
		}

		if count <= 1 {
			_ = tx.Rollback()
			helpers.ErrorJSON(w, errors.New("cannot remove admin status from the last admin account"), http.StatusForbidden)
			return
		}
	}

	user, err := qtx.AdminUpdateUser(r.Context(), database.AdminUpdateUserParams{
		Name:    req.Name,
		Email:   req.Email,
		IsAdmin: req.IsAdmin,
		ID:      targetID,
	})
	if err != nil {
		_ = tx.Rollback()
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			helpers.ErrorJSON(w, errors.New("a user with that email already exists"), http.StatusConflict)
			return
		}
		app.Logger.Error("admin: failed to update user", "error", err, "target_id", targetID)
		helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
		return
	}

	err = tx.Commit()
	if err != nil {
		_ = tx.Rollback()
		app.Logger.Error("admin: failed to commit user update transaction", "error", err, "target_id", targetID)
		helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
		return
	}

	app.Logger.Info("admin: user updated", "user_id", user.ID)

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"user": adminUserRow(
				user.ID, user.Name, user.Email, user.IsAdmin,
				user.Avatar, user.CreatedAt, user.UpdatedAt,
			),
		},
	})
}

func (app *Application) AdminDeleteUser(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	targetID, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid user ID"), http.StatusBadRequest)
		return
	}

	currentUserID := app.userIDFromRequest(r)
	if targetID == currentUserID {
		helpers.ErrorJSON(w, errors.New("you cannot delete your own account"), http.StatusForbidden)
		return
	}

	tx, err := app.DB.BeginTx(r.Context(), nil)
	if err != nil {
		app.Logger.Error("admin: failed to begin user deletion transaction", "error", err, "target_id", targetID)
		helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
		return
	}

	qtx := app.Queries.WithTx(tx)

	user, err := qtx.GetUser(r.Context(), targetID)
	if err != nil {
		_ = tx.Rollback()
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("user not found"), http.StatusNotFound)
			return
		}
		app.Logger.Error("admin: failed to fetch user for deletion", "error", err, "target_id", targetID)
		helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
		return
	}

	// Keep at least one admin account.
	if user.IsAdmin {
		count, err := qtx.CountAdmins(r.Context())
		if err != nil {
			_ = tx.Rollback()
			app.Logger.Error("admin: failed to count admins during deletion", "error", err, "target_id", targetID)
			helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
			return
		}

		if count <= 1 {
			_ = tx.Rollback()
			helpers.ErrorJSON(w, errors.New("cannot delete the last admin account"), http.StatusForbidden)
			return
		}
	}

	err = qtx.DeleteUser(r.Context(), targetID)
	if err != nil {
		_ = tx.Rollback()
		app.Logger.Error("admin: failed to delete user", "error", err, "target_id", targetID)
		helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
		return
	}

	err = tx.Commit()
	if err != nil {
		_ = tx.Rollback()
		app.Logger.Error("admin: failed to commit user deletion transaction", "error", err, "target_id", targetID)
		helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
		return
	}

	if user.Avatar.Valid && isUploadedAvatar(user.Avatar.String) {
		app.deleteAvatarFile(user.Avatar.String)
	}

	app.Logger.Info("admin: user deleted", "user_id", targetID, "email", user.Email)

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error:   false,
		Message: "User deleted successfully",
	})
}

type AdminResetUserPasswordRequest struct {
	Password string `json:"password"`
}

func (app *Application) AdminResetUserPassword(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	targetID, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid user ID"), http.StatusBadRequest)
		return
	}

	var req AdminResetUserPasswordRequest
	if err := helpers.ReadJSON(w, r, &req, 0); err != nil {
		helpers.ErrorJSON(w, errors.New("invalid request body"), http.StatusBadRequest)
		return
	}

	if err := validatePassword(req.Password, "password"); err != nil {
		helpers.ErrorJSON(w, err, http.StatusBadRequest)
		return
	}

	hashedPassword, err := helpers.HashPassword(req.Password)
	if err != nil {
		app.Logger.Error("admin: failed to hash password for reset", "error", err)
		helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
		return
	}

	err = app.Queries.UpdateUserPassword(r.Context(), database.UpdateUserPasswordParams{
		Password: hashedPassword,
		ID:       targetID,
	})
	if err != nil {
		app.Logger.Error("admin: failed to reset user password", "error", err, "target_id", targetID)
		helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
		return
	}

	app.Logger.Info("admin: user password reset", "user_id", targetID)

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error:   false,
		Message: "Password reset successfully",
	})
}
