package main

import (
	"fmt"
	"igloo/cmd/database/models"
	"igloo/cmd/helpers"
	"net/http"
	"os"
	"os/exec"
	"strconv"

	"github.com/go-chi/chi/v5"
)

func (app *config) StreamMovie(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, err, http.StatusBadRequest)
		return
	}

	uuid := r.URL.Query().Get("uuid")
	if uuid == "" {
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

	videoCodec := r.URL.Query().Get("videoCodec")
	if videoCodec == "" {
		videoCodec = "libx264"
	}

	audioCodec := r.URL.Query().Get("audioCodec")
	if audioCodec == "" {
		audioCodec = "aac"
	}

	if audioCodec == "copy" && videoCodec == "copy" {
		fileName := fmt.Sprintf("%s.mp4", uuid)

		cmd := exec.Command(app.ffmpeg, "-i", movie.FilePath, "-c", "copy", "-movflags", "+faststart", fileName)

		err = cmd.Run()
		if err != nil {
			helpers.ErrorJSON(w, err, http.StatusInternalServerError)
			return
		}

		fileInfo, err := os.Stat(fileName)
		if err != nil {
			helpers.ErrorJSON(w, err, http.StatusInternalServerError)
			return
		}

		size := fileInfo.Size()

		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))
		w.Header().Set("Content-Type", "video/mp4")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", size))
		w.Header().Set("Accept-Ranges", "bytes")

		http.ServeFile(w, r, fileName)

	}
}
