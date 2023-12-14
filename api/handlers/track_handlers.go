// description: manages the handlers for working with music tracks
package handlers

import (
	"igloo/helpers"
	"igloo/models"
	"igloo/repository"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

type trackHandler struct {
	repo repository.TrackRepository
}

func NewTrackHandler(repo repository.TrackRepository) *trackHandler {
	return &trackHandler{
		repo: repo,
	}
}

func (h *trackHandler) CreateTrack(w http.ResponseWriter, r *http.Request) {
	var track models.Track

	err := helpers.ReadJSON(w, r, &track)
	if err != nil {
		helpers.ErrorJSON(w, err)
		return
	}

	err = h.repo.Create(&track)
	if err != nil {
		helpers.ErrorJSON(w, err)
		return
	}

	res := helpers.JSONResponse{
		Error:   false,
		Message: "Track created successfully",
		Data:    track,
	}

	helpers.WriteJSON(w, http.StatusCreated, res)
}

func (h *trackHandler) GetTrackByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	trackID, err := strconv.Atoi(id)
	if err != nil {
		helpers.ErrorJSON(w, err)
		return
	}

	track, err := h.repo.FindByID(trackID)
	if err != nil {
		helpers.ErrorJSON(w, err)
		return
	}

	res := helpers.JSONResponse{
		Error:   false,
		Message: "Track retrieved successfully",
		Data:    track,
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}
