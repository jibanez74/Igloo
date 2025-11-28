package main

import (
  "context"
  "errors"
  "fmt"
  "igloo/cmd/internal/helpers"
  "net/http"

  "github.com/jackc/pgx/v5"
)

func (app *Application) RouteLogin(w http.ResponseWriter, r *http.Request) {
  var request AuthRequest

  err := helpers.ReadJSON(w, r, &request, 0)
  if err != nil {
    app.Logger.Error(fmt.Sprintf("fail to parse request body in login process\n%s", err.Error()))
    helpers.ErrorJSON(w, errors.New(INVALID_REQUEST_BODY), http.StatusBadRequest)
    return
  }

  if request.Email == "" || request.Password == "" {
    helpers.ErrorJSON(w, errors.New(INVALID_CREDENTIALS))
    return
  }

  user, err := app.Queries.GetUserForLogin(context.Background(), request.Email)
  if err != nil {
    app.Logger.Error(fmt.Sprintf("fail to fetch user from data base for login process\n%s", err.Error()))

    if err == pgx.ErrNoRows {
      helpers.ErrorJSON(w, errors.New(INVALID_CREDENTIALS), http.StatusUnauthorized)
    } else {
      helpers.ErrorJSON(w, errors.New(INTERNAL_SERVER_ERROR))
    }

    return
  }

  match, err := helpers.PasswordMatches(request.Password, user.Password)
  if err != nil {
    app.Logger.Error(fmt.Sprintf("fail to compare hash with plain string  password in login process for user %s\n%s", request.Email, err.Error()))
    helpers.ErrorJSON(w, errors.New(INTERNAL_SERVER_ERROR))
    return
  }

  if !match {
    app.Logger.Error((fmt.Sprintf("incorrect password entered for user %s", user.Name)))
    helpers.ErrorJSON(w, errors.New(INVALID_CREDENTIALS), http.StatusUnauthorized)
    return
  }

  err = app.Session.RenewToken(r.Context())
  if err != nil {
    app.Logger.Error(fmt.Sprintf("fail to renew session token for user %s\n%s", user.Name, err.Error()))
    helpers.ErrorJSON(w, errors.New(INTERNAL_SERVER_ERROR))
    return
  }

  app.Session.Put(r.Context(), COOKIE_USER_ID, user.ID)
  app.Session.Put(r.Context(), COOKIE_EMAIL, user.Email)

  res := helpers.JSONResponse{
    Error:   false,
    Message: fmt.Sprintf("Hello %s, welcome to your media library!", user.Name),
  }

  app.Logger.Info(fmt.Sprintf(SUCCESS_LOGIN, user.Name, user.ID))

  helpers.WriteJSON(w, http.StatusOK, res)
}

func (app *Application) RouteGetCurrentUser(w http.ResponseWriter, r *http.Request) {
  userID, ok := app.Session.Get(r.Context(), COOKIE_USER_ID).(int32)
  if !ok {
    app.Logger.Error("fail to get user_id from session in get current user process")
    helpers.ErrorJSON(w, errors.New(NOT_AUTHORIZED), http.StatusUnauthorized)
    return
  }

  user, err := app.Queries.GetUserByID(context.Background(), userID)
  if err != nil {
    app.Logger.Error(fmt.Sprintf("fail to fetch user from database for get current user process\n%s", err.Error()))

    if err == pgx.ErrNoRows {
      helpers.ErrorJSON(w, errors.New(NOT_AUTHORIZED), http.StatusUnauthorized)
    } else {
      helpers.ErrorJSON(w, errors.New(INTERNAL_SERVER_ERROR))
    }

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

func (app *Application) RouteLogout(w http.ResponseWriter, r *http.Request) {
  err := app.Session.Destroy(r.Context())
  if err != nil {
    app.Logger.Error(fmt.Sprintf("fail to destroy session during logout\n%s", err.Error()))
    helpers.ErrorJSON(w, errors.New(INTERNAL_SERVER_ERROR))
    return
  }

  res := helpers.JSONResponse{
    Error:   false,
    Message: SUCCESS_LOGOUT,
  }

  helpers.WriteJSON(w, http.StatusOK, res)
}
