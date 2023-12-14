// description: defines the http handlers for working with music albums
package handlers

import (
	"igloo/helpers"
	"igloo/models"
	"igloo/repository"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

type albumHandler struct {
	repo repository.AlbumRepository
}

func NewAlbumHandler(repo repository.AlbumRepository) *albumHandler {
	return &albumHandler{
		repo: repo,
	}
}

func (h *albumHandler) FindOrCreateByTitle(w http.ResponseWriter, r *http.Request) {
	var album models.Album

	err := helpers.ReadJSON(w, r, &album)
	if err != nil {
		helpers.ErrorJSON(w, err)
		return
	}

	err = h.repo.FindOrInsertByTitle(&album)
	if err != nil {
		helpers.ErrorJSON(w, err)
		return
	}

	res := helpers.JSONResponse{
		Error:   false,
		Message: "album created successfully",
		Data:    album,
	}

	helpers.WriteJSON(w, http.StatusCreated, res)
}

func (h *albumHandler) GetAlbumByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	albumID, err := strconv.Atoi(id)
	if err != nil {
		helpers.ErrorJSON(w, err)
		return
	}

	album, err := h.repo.FindByID(albumID)
	if err != nil {
		helpers.ErrorJSON(w, err)
		return
	}

	res := helpers.JSONResponse{
		Error:   false,
		Message: "album retrieved successfully",
		Data:    album,
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}
