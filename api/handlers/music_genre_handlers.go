package handlers

import (
  "igloo/helpers"
  "igloo/models"
  "net/http"
  "gorm.io/gorm"
)

func (h *appHandler) GetAllMusicGenres(w http.ResponseWriter, r *http.Request) {
  var music_genres []models.MusicGenre
  
  result := h.db.Find(&music_genres)
  if result.Error != nil {
    if result.Error == gorm.ErrRecordNotFound {
      helpers.ErrorJSON(w, result.Error, http.StatusNotFound)
    } else {
      helpers.ErrorJSON(w, result.Error, http.StatusInternalServerError)
    }

    return
  }

  res := helpers.JSONResponse{
    Error: false,
    Message: "Music genres found",
    Data: music_genres,
  }

  helpers.WriteJSON(w, http.StatusOK, res)
}