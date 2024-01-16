package handlers

import (
	"igloo/models"
	"igloo/utils"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

func (h *appHandlers) GetMusicGenreByTag(w http.ResponseWriter, r *http.Request) {
	var genre models.MusicGenre
	tag := chi.URLParam(r, "tag")

	err := h.db.Where("Tag = ?", tag).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			utils.ErrorJSON(w, err, 404)
		} else {
			utils.ErrorJSON(w, err, 500)
		}

		return
	}

	res := utils.JSONResponse{
		Error:   false,
		Message: "Music genre was fetched successfully",
		Data:    genre,
	}

	utils.WriteJSON(w, 200, res)
}

func (h *appHandlers) GetMusicGenreByID(w http.ResponseWriter, r *http.Request) {
	var genre models.MusicGenre
	id := chi.URLParam(r, "id")

	genreId, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		utils.ErrorJSON(w, err)
		return
	}

	err = h.db.First(&genre, uint(genreId)).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			utils.ErrorJSON(w, err, 404)
		} else {
			utils.ErrorJSON(w, err, 500)
		}

		return
	}

	res := utils.JSONResponse{
		Error:   false,
		Message: "Music genre fetch successfully",
		Data:    genre,
	}

	utils.WriteJSON(w, 200, res)
}

func (h *appHandlers) GetMusicGenres(w http.ResponseWriter, r *http.Request) {
	var genres models.MusicGenre

	err := h.db.Find(&genres).Error
	if err != nil {
		utils.ErrorJSON(w, err, 500)
		return
	}

	res := utils.JSONResponse{
		Error:   false,
		Message: "Successfully fetch music genres",
		Data:    genres,
	}

	utils.WriteJSON(w, 200, res)
}

func (h *appHandlers) FindOrCreateMusicGenre(w http.ResponseWriter, r *http.Request) {
	var genre models.MusicGenre

	err := utils.ReadJSON(w, r, &genre)
	if err != nil {
		utils.ErrorJSON(w, err)
		return
	}

	err = h.db.FirstOrCreate(&genre).Error
	if err != nil {
		utils.ErrorJSON(w, err, 500)
		return
	}

	res := utils.JSONResponse{
		Error:   false,
		Message: "Music Genre fetch successfully",
		Data:    genre,
	}

	utils.WriteJSON(w, 200, res)
}
