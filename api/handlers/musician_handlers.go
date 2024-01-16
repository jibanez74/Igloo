package handlers

import (
	"igloo/models"
	"igloo/utils"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

func (h *appHandlers) GetMusicianByID(w http.ResponseWriter, r *http.Request) {
	var musician models.Musician
	id := chi.URLParam(r, "id")

	musicianId, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		utils.ErrorJSON(w, err)
		return
	}

	err = h.db.First(&musician, uint(musicianId)).Error
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
		Message: "Musician fetched successfully",
		Data:    musician,
	}

	utils.WriteJSON(w, 200, res)
}

func (h *appHandlers) GetMusicianByName(w http.ResponseWriter, r *http.Request) {
	var musician models.Musician
	name := chi.URLParam(r, "name")

	err := h.db.Where("Name = ?", name).Error
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
		Message: "Musician fetched successfully",
		Data:    musician,
	}

	utils.WriteJSON(w, 200, res)
}

func (h *appHandlers) GetMusicians(w http.ResponseWriter, r *http.Request) {
	var musicians models.Musician

	err := h.db.Find(&musicians).Error
	if err != nil {
		utils.ErrorJSON(w, err, 500)
		return
	}

	res := utils.JSONResponse{
		Error:   false,
		Message: "Musician fetched successfully",
		Data:    musicians,
	}

	utils.WriteJSON(w, 200, res)
}

func (h *appHandlers) CreateMusician(w http.ResponseWriter, r *http.Request) {
	var musician models.Musician

	err := utils.ReadJSON(w, r, &musician)
	if err != nil {
		utils.ErrorJSON(w, err)
		return
	}

	err = h.db.Create(&musician).Error
	if err != nil {
		utils.ErrorJSON(w, err)
		return
	}

	res := utils.JSONResponse{
		Error:   false,
		Message: "Musician was created successfully",
		Data:    musician,
	}

	utils.WriteJSON(w, 201, res)
}
