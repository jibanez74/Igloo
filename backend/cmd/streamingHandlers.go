package main

import (
	"errors"
	"fmt"
	"igloo/cmd/database/models"
	"igloo/cmd/helpers"
	"net/http"
	"os"
	"strconv"

	"github.com/go-chi/chi/v5"
)

func (app *config) SimpleTranscodeVideoStream(w http.ResponseWriter, r *http.Request) {
	uuid := r.URL.Query().Get("uuid")
	if uuid == "" {
		helpers.ErrorJSON(w, errors.New("uuid is required"), http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, err, http.StatusBadRequest)
		return
	}

	var movie models.Movie
	movie.ID = uint(id)

	status, err := app.repo.GetMovieByID(&movie)
	if err != nil {
		helpers.ErrorJSON(w, err, status)
		return
	}

	status, err = helpers.CheckFileExist(movie.FilePath)
	if err != nil {
		helpers.ErrorJSON(w, err, status)
		return
	}

	fileName := fmt.Sprintf("%s.mp4", uuid)

	err = helpers.TranscodeVideo(movie.FilePath, fileName)
	if err != nil {
		helpers.ErrorJSON(w, err, http.StatusInternalServerError)
		return
	}

	file, err := os.Open(fileName)
	if err != nil {
		helpers.ErrorJSON(w, err)
		return
	}
	defer file.Close()

	w.Header().Set("Content-Type", "video/mp4")

	http.ServeContent(w, r, fileName, movie.CreatedAt, file)
}

func (app *config) DeleteTranscodedFile(w http.ResponseWriter, r *http.Request) {
	fileUUID := chi.URLParam(r, "uuid")
	if fileUUID == "" {
		helpers.ErrorJSON(w, errors.New("uuid is required"), http.StatusBadRequest)
		return
	}

	fileName := fmt.Sprintf("%s.mp4", fileUUID)

	status, err := helpers.RemoveFile(fileName)
	if err != nil {
		helpers.ErrorJSON(w, err, status)
		return
	}

	helpers.WriteJSON(w, status, map[string]any{
		"message": "file deleted",
	})
}

// func (app *config) StreamTranscodeVideo(w http.ResponseWriter, r *http.Request) {
// 	processUUID := r.URL.Query().Get("processUUID")
// 	if processUUID == "" {
// 		helpers.ErrorJSON(w, errors.New("you must provide a pid as a uuid for ffmpeg process"), http.StatusBadRequest)
// 		return
// 	}

// 	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
// 	if err != nil {
// 		helpers.ErrorJSON(w, err, http.StatusBadRequest)
// 		return
// 	}

// 	var movie models.Movie
// 	movie.ID = uint(id)

// 	status, err := app.repo.GetMovieByID(&movie)
// 	if err != nil {
// 		helpers.ErrorJSON(w, err, status)
// 		return
// 	}

// 	_, err = os.Stat(movie.FilePath)
// 	if err != nil || os.IsNotExist(err) {
// 		helpers.ErrorJSON(w, errors.New("file not found"), http.StatusNotFound)
// 		return
// 	}

// 	videoCodec := r.URL.Query().Get("videoCodec")
// 	if videoCodec == "" {
// 		videoCodec = "libx264"
// 	}

// 	videoHeight := r.URL.Query().Get("videoHeight")
// 	if videoHeight == "" {
// 		videoHeight = "480"
// 	}

// 	videoBitRate := r.URL.Query().Get("videoBitRate")
// 	if videoBitRate == "" {
// 		videoBitRate = "3000k"
// 	}

// 	audioCodec := r.URL.Query().Get("audioCodec")
// 	if audioCodec == "" {
// 		audioCodec = "aac"
// 	}

// 	audioChannels := r.URL.Query().Get("audioChannels")
// 	if audioChannels == "" {
// 		audioChannels = "2"
// 	}

// 	audioBitRate := r.URL.Query().Get("audioBitRate")
// 	if audioBitRate == "" {
// 		audioBitRate = "128k" // Fixed typo from "128kk" to "128k"
// 	}

// 	preset := r.URL.Query().Get("preset")
// 	if preset == "" {
// 		preset = "ultrafast"
// 	}

// }

// func (app *config) RemoveTranscodedVideo(w http.ResponseWriter, r *http.Request) {
// 	fileName := r.URL.Query().Get("filename")
// 	if fileName == "" {
// 		helpers.ErrorJSON(w, errors.New("file name is required"), http.StatusBadRequest)
// 		return
// 	}
// }
