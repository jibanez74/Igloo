// description: all handlers related to the user management
package handlers

import (
	"igloo/helpers"
	"igloo/models"
	"igloo/repository"
	"net/http"
)

type userHandler struct {
	repo repository.UserRepository
}

func NewUserHandler(repo repository.UserRepository) *userHandler {
	return &userHandler{repo: repo}
}

func (h *userHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var user models.User

	err := helpers.ReadJSON(w, r, &user)
	if err != nil {
		helpers.ErrorJSON(w, err)
		return
	}

	err = h.repo.Create(&user)
	if err != nil {
		helpers.ErrorJSON(w, err)
		return
	}

	res := helpers.JSONResponse{
		Error:   false,
		Message: "user created successfully",
		Data:    user,
	}

	helpers.WriteJSON(w, http.StatusCreated, res)
}

func (h *userHandler) Authenticate(w http.ResponseWriter, r *http.Request) {
	var requestPayload struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	err := helpers.ReadJSON(w, r, &requestPayload)
	if err != nil {
		helpers.ErrorJSON(w, err, http.StatusBadRequest)
		return
	}

	user, err := h.repo.FindByEmailAndUsername(requestPayload.Email, requestPayload.Username)
	if err != nil {
		helpers.ErrorJSON(w, err, http.StatusUnauthorized)
		return
	}

	err = user.PasswordMatches(requestPayload.Password)
	if err != nil {
		helpers.ErrorJSON(w, err, http.StatusUnauthorized)
		return
	}

	tokens, err := helpers.GenerateTokenPair(user.ID)
	if err != nil {
		helpers.ErrorJSON(w, err)
		return
	}

	refreshCookie := helpers.GetRefreshCookie(tokens.RefreshToken)
	http.SetCookie(w, refreshCookie)

	helpers.WriteJSON(w, http.StatusAccepted, tokens)
}
