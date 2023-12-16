package handlers

import (
  "igloo/helpers"
  "igloo/models"
  "net/http"
  "strconv"
  "math"
  "strings"

  "github.com/go-chi/chi/v5"
  "gorm.io/gorm"
)

func (h *appHandler) GetMusicians(w http.ResponseWriter, r *http.Request) {
  page, _ := strconv.Atoi(r.URL.Query().Get("page"))
  if page < 1 {
    page = 1
  }

  pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
  if pageSize < 1 {
    pageSize = 10
  }

  offset := (page - 1) * pageSize

  var total int64
  h.db.Model(&models.Musician{}).Count(&total)
  totalPages := int(math.Ceil(float64(total) / float64(pageSize)))

  if total == 0 {
    res := helpers.JSONResponse{
      Error: false,
      Message: "No musicians found",
      Data: map[string]interface{}{
        "musicians": []models.Musician{},
        "page": page,
        "totalPages": totalPages,
      },
    }

    helpers.WriteJSON(w, http.StatusOK, res)

    return
  }

  var musicians []models.Musician

  result := h.db.Offset(offset).Limit(pageSize).Find(&musicians)
  if result.Error != nil {
    helpers.ErrorJSON(w, result.Error, http.StatusInternalServerError)
    return
  }

  res := helpers.JSONResponse{
    Error: false,
    Message: "Fetch musicians successfully",
    Data: map[string]interface{}{
      "musicians": musicians,
      "page": page,
      "totalPages": totalPages,
    },
  }

  helpers.WriteJSON(w, http.StatusOK, res)
}

func (h *appHandler) FindMusicianByName(w http.ResponseWriter, r *http.Request) {
  name := r.URL.Query().Get("name")

  var musician models.Musician

  result := h.db.Where("name = ?", name).First(&musician)
  if result.Error != nil {
    if result.Error == gorm.ErrRecordNotFound {
      helpers.ErrorJSON(w, result.Error, http.StatusNotFound)
    } else {
      helpers.ErrorJSON(w, result.Error, http.StatusInternalServerError)
    }

    return
  }

  res := helpers.JSONResponse{
    Error:   false,
    Message: "Musician found successfully",
    Data:    musician,
  }

  helpers.WriteJSON(w, http.StatusOK, res)
}

func (h *appHandler) FindMusicianByID(w http.ResponseWriter, r *http.Request) {
  id := chi.URLParam(r, "id")

  var musician models.Musician

  result := h.db.First(&musician, id)
  if result.Error != nil {
    if result.Error == gorm.ErrRecordNotFound {
      helpers.ErrorJSON(w, result.Error, http.StatusNotFound)
    } else {
      helpers.ErrorJSON(w, result.Error, http.StatusInternalServerError)
    }

    return
  }

  res := helpers.JSONResponse{
    Error:   false,
    Message: "Musician found successfully",
    Data:    musician,
  }

  helpers.WriteJSON(w, http.StatusOK, res)
}

func (h *appHandler) CreateMusician(w http.ResponseWriter, r *http.Request) {
  var musician models.Musician

  err := helpers.ReadJSON(w, r, &musician)
  if err != nil {
    helpers.ErrorJSON(w, err)
    return
  }

  err = h.db.Create(&musician).Error
  if err != nil {
    if strings.Contains(err.Error(), "validation failed") {
      helpers.ErrorJSON(w, err, http.StatusBadRequest)
    } else {
      helpers.ErrorJSON(w, err, http.StatusInternalServerError)
    }
  
    return
  }

  res := helpers.JSONResponse{
    Error:   false,
    Message: "Musician created successfully",
    Data:    musician,
  }

  helpers.WriteJSON(w, http.StatusCreated, res)
}

func (h *appHandler) DeleteMusician(w http.ResponseWriter, r *http.Request) {
  id := chi.URLParam(r, "id")

  var musician models.Musician

  result := h.db.First(&musician, id)
  if result.Error != nil {
    if result.Error == gorm.ErrRecordNotFound {
      helpers.ErrorJSON(w, result.Error, http.StatusNotFound)
    } else {
      helpers.ErrorJSON(w, result.Error, http.StatusInternalServerError)
    }

    return
  }

  result = h.db.Delete(&musician)
  if result.Error != nil {
    helpers.ErrorJSON(w, result.Error, http.StatusInternalServerError)
    return
  }

  res := helpers.JSONResponse{
    Error:   false,
    Message: "Musician deleted successfully",
    Data:    musician,
  }

  helpers.WriteJSON(w, http.StatusOK, res)
}