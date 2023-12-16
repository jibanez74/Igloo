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

// create a handler to find an album by its name
func (h *appHandler) FindAlbumByName(w http.ResponseWriter, r *http.Request) {
  name := chi.URLParam(r, "name")

  var album models.Album

  result := h.db.Where("name = ?", name).First(&album)
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
    Message: "Fetch album successfully",
    Data: album,
  }

  helpers.WriteJSON(w, http.StatusOK, res)
}

func (h *appHandler) GetAlbums(w http.ResponseWriter, r *http.Request) {
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
  var albums models.Album
  h.db.Model(&models.Album{}).Count(&total)
  totalPages := int(math.Ceil(float64(total) / float64(pageSize)))

  if total == 0 {
    res := helpers.JSONResponse{
      Error: false,
      Message: "No albums found",
      Data: map[string]interface{}{
        "albums": []models.Album{},
        "page": page,
        "totalPages": totalPages,
      },
    }

    helpers.WriteJSON(w, http.StatusOK, res)

    return
  }

  result := h.db.Offset(offset).Limit(pageSize).Find(&albums)
  if result.Error != nil {
    helpers.ErrorJSON(w, result.Error, http.StatusInternalServerError)
    return
  }

  res := helpers.JSONResponse{
    Error: false,
    Message: "Fetch albums successfully",
    Data: map[string]interface{}{
      "albums": albums,
      "page": page,
      "totalPages": totalPages,
    },
  }

  helpers.WriteJSON(w, http.StatusOK, res)
}

func (h *appHandler) CreateAlbum(w http.ResponseWriter, r *http.Request) {
  var album models.Album

  err := helpers.ReadJSON(w, r, &album)
  if err != nil {
    helpers.ErrorJSON(w, err, http.StatusInternalServerError)
    return
  }

  err = h.db.Create(&album).Error
  if err != nil {
    if strings.Contains(err.Error(), "validation failed") {
      helpers.ErrorJSON(w, err, http.StatusBadRequest)
    } else {
      helpers.ErrorJSON(w, err, http.StatusInternalServerError)
    }
  
    return
  }

  res := helpers.JSONResponse{
    Error: false,
    Message: "Album created successfully",
    Data: album,
  }

  helpers.WriteJSON(w, http.StatusOK, res)
}

func (h *appHandler) DeleteAlbum(w http.ResponseWriter, r *http.Request) {
  id := chi.URLParam(r, "id")

  var album models.Album

  result := h.db.Where("id = ?", id).First(&album)
  if result.Error != nil {
    if result.Error == gorm.ErrRecordNotFound {
      helpers.ErrorJSON(w, result.Error, http.StatusNotFound)
    } else {
      helpers.ErrorJSON(w, result.Error, http.StatusInternalServerError)
    }

    return
  }

  result = h.db.Delete(&album)
  if result.Error != nil {
    helpers.ErrorJSON(w, result.Error, http.StatusInternalServerError)
    return
  }

  res := helpers.JSONResponse{
    Error: false,
    Message: "Album deleted successfully",
  }

  helpers.WriteJSON(w, http.StatusOK, res)
}