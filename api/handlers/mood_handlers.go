// description: defines the handlers for working with music moods
package handlers

import (
	"igloo/helpers"
	"igloo/models"
	"igloo/repository"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

type moodHandler struct {
	repo repository.MoodRepository
}

func NewMoodHandler(repo repository.MoodRepository) *moodHandler {
	return &moodHandler{repo: repo}
}

func (h *moodHandler) FindOrCreateByTag(w http.ResponseWriter, r *http.Request) {
	var mood models.Mood

	err := helpers.ReadJSON(w, r, &mood)
	if err != nil {
		helpers.ErrorJSON(w, err)
		return
	}

	err = h.repo.FindOrInsertByTag(&mood)
	if err != nil {
		helpers.ErrorJSON(w, err)
		return
	}

	res := helpers.JSONResponse{
		Error:   false,
		Message: "Mood created successfully",
		Data:    mood,
	}

	helpers.WriteJSON(w, http.StatusCreated, res)
}

func (h *moodHandler) GetMoodByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	moodID, err := strconv.Atoi(id)
	if err != nil {
		helpers.ErrorJSON(w, err)
		return
	}

	mood, err := h.repo.FindByID(moodID)
	if err != nil {
		helpers.ErrorJSON(w, err)
		return
	}

	res := helpers.JSONResponse{
		Error:   false,
		Message: "Mood retrieved successfully",
		Data:    mood,
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}
