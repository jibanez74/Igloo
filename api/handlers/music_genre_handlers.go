package handlers

import (
	"igloo/helpers"
	"igloo/models"
	"igloo/repository"
	"strconv"

	"net/http"

	"github.com/go-chi/chi/v5"
)

type musicGenreHandler struct {
	repo repository.MusicGenreRepository
}

func NewMusicGenreHandler(repo repository.MusicGenreRepository) *musicGenreHandler {
	return &musicGenreHandler{
		repo: repo,
	}
}

func (h *musicGenreHandler) FindOrCreateByTag(w http.ResponseWriter, r *http.Request) {
	var musicGenre models.MusicGenre

	error := helpers.ReadJSON(w, r, &musicGenre)
	if error != nil {
		helpers.ErrorJSON(w, error)
		return
	}

	error = h.repo.FindOrInsertByTag(&musicGenre)
	if error != nil {
		helpers.ErrorJSON(w, error, http.StatusInternalServerError)
		return
	}

	res := helpers.JSONResponse{
		Error:   false,
		Message: "music genre retrieved or created successfully",
		Data:    musicGenre,
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

func (h *musicGenreHandler) FindMusicGenreByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	genreID, err := strconv.Atoi(id)
	if err != nil {
		helpers.ErrorJSON(w, err)
		return
	}

	genre, err := h.repo.FindByID(genreID)
	if err != nil {
		helpers.ErrorJSON(w, err)
		return
	}

	res := helpers.JSONResponse{
		Error:   false,
		Message: "Genre retrieved successfully",
		Data:    genre,
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}
