package handlers

import (
	"igloo/helpers"
	"igloo/models"
	"igloo/repository"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

type musicianHandler struct {
	repo repository.MusicianRepository
}

func NewMusicianHandler(repo repository.MusicianRepository) *musicianHandler {
	return &musicianHandler{repo: repo}
}

func (h *musicianHandler) GetMusicians(w http.ResponseWriter, r *http.Request) {
	musicians, err := h.repo.GetMusicians()
	if err != nil {
		helpers.ErrorJSON(w, err)
		return
	}

	res := helpers.JSONResponse{
		Error:   false,
		Message: "Musicians found successfully",
		Data:    musicians,
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

func (h *musicianHandler) GetMusicianByName(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	musician, err := h.repo.FindByName(name)
	if err != nil {
		helpers.ErrorJSON(w, err, http.StatusNotFound)
		return
	}

	res := helpers.JSONResponse{
		Error:   false,
		Message: "Musician found successfully",
		Data:    musician,
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

func (h *musicianHandler) GetMusicianByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	musicianID, err := strconv.Atoi(id)
	if err != nil {
		helpers.ErrorJSON(w, err)
		return
	}

	musician, err := h.repo.FindByID(musicianID)
	if err != nil {
		helpers.ErrorJSON(w, err)
		return
	}

	res := helpers.JSONResponse{
		Error:   false,
		Message: "Musician found successfully",
		Data:    musician,
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

func (h *musicianHandler) CreateMusician(w http.ResponseWriter, r *http.Request) {
	var musician models.Musician

	err := helpers.ReadJSON(w, r, &musician)
	if err != nil {
		helpers.ErrorJSON(w, err)
		return
	}

	err = h.repo.CreateMusician(&musician)
	if err != nil {
		helpers.ErrorJSON(w, err)
		return
	}

	res := helpers.JSONResponse{
		Error:   false,
		Message: "Musician created successfully",
		Data:    musician,
	}

	helpers.WriteJSON(w, http.StatusCreated, res)
}
