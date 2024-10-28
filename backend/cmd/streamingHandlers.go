package main

import (
	"errors"
	"fmt"
	"igloo/cmd/database/models"
	"igloo/cmd/helpers"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

const ffmpegPath = "/bin/ffmpeg"

const bufferSize = 64 * 1024

func (app *config) DirectStreamVideo(w http.ResponseWriter, r *http.Request) {
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

	fileInfo, err := os.Stat(movie.FilePath)
	if err != nil {
		if os.IsNotExist(err) {
			helpers.ErrorJSON(w, errors.New("file not found"), http.StatusNotFound)
		} else if os.IsPermission(err) {
			helpers.ErrorJSON(w, errors.New("file access denied"), http.StatusForbidden)
		} else {
			helpers.ErrorJSON(w, errors.New("unable to retrieve file info"), http.StatusInternalServerError)
		}

		return
	}

	file, err := os.Open(movie.FilePath)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("unable to open file"), http.StatusInternalServerError)
		return
	}
	defer file.Close()

	rangeHeader := r.Header.Get("Range")
	w.Header().Set("Content-Type", movie.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", movie.FileName))

	size := fileInfo.Size()

	var start, end int64 = 0, size - 1

	if rangeHeader != "" {
		rangeHeader = strings.Replace(rangeHeader, "bytes=", "", 1)
		parts := strings.Split(rangeHeader, "-")

		if parts[0] != "" {
			start, err = strconv.ParseInt(parts[0], 10, 64)
			if err != nil || start < 0 || start >= size {
				helpers.ErrorJSON(w, errors.New("invalid range"), http.StatusBadRequest)
				return
			}
		}

		if parts[1] != "" {
			end, err = strconv.ParseInt(parts[1], 10, 64)
			if err != nil || end >= size {
				end = size - 1
			}
		}

		if start > end {
			helpers.ErrorJSON(w, errors.New("invalid range"), http.StatusBadRequest)
			return
		}

		contentLength := end - start + 1
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, size))
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", contentLength))
		w.WriteHeader(http.StatusPartialContent)

		_, err = file.Seek(start, io.SeekStart)
		if err != nil {
			helpers.ErrorJSON(w, errors.New("unable to seek file"), http.StatusInternalServerError)
			return
		}

		buffer := make([]byte, bufferSize)
		bytesToRead := contentLength

		for bytesToRead > 0 {
			bytesRead, err := file.Read(buffer)
			if err != nil && err != io.EOF {
				helpers.ErrorJSON(w, errors.New("error reading file"), http.StatusInternalServerError)
				return
			}

			if bytesRead == 0 {
				break
			}

			if int64(bytesRead) > bytesToRead {
				bytesRead = int(bytesToRead)
			}

			_, err = w.Write(buffer[:bytesRead])
			if err != nil {
				helpers.ErrorJSON(w, errors.New("error writing to response"), http.StatusInternalServerError)
				return
			}

			bytesToRead -= int64(bytesRead)
		}
	} else {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", size))
		w.WriteHeader(http.StatusOK)

		buffer := make([]byte, bufferSize)
		for {
			bytesRead, err := file.Read(buffer)
			if err != nil && err != io.EOF {
				helpers.ErrorJSON(w, errors.New("error reading file"), http.StatusInternalServerError)
				return
			}

			if bytesRead == 0 {
				break
			}

			_, err = w.Write(buffer[:bytesRead])
			if err != nil {
				helpers.ErrorJSON(w, errors.New("error writing to response"), http.StatusInternalServerError)
				return
			}
		}
	}
}

func (app *config) StreamTranscodedVideo(w http.ResponseWriter, r *http.Request) {
	processUUID := r.URL.Query().Get("processUUID")
	if processUUID == "" {
		helpers.ErrorJSON(w, errors.New("you must provide a pid as a uuid for ffmpeg process"), http.StatusBadRequest)
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

	var cmdArgs []string

	_, err = os.Stat(movie.FilePath)
	if err != nil || os.IsNotExist(err) {
		helpers.ErrorJSON(w, errors.New("file not found"), http.StatusNotFound)
		return
	}
	cmdArgs = append(cmdArgs, "-i", movie.FilePath)

	videoCodec := r.URL.Query().Get("videoCodec")
	if videoCodec == "" {
		videoCodec = "libx264"
	}
	cmdArgs = append(cmdArgs, "-c:v", videoCodec)

	videoHeight := r.URL.Query().Get("videoHeight")
	if videoHeight == "" {
		videoHeight = "480"
	}
	cmdArgs = append(cmdArgs, "-vf", fmt.Sprintf("scale=-1:%s", videoHeight))

	videoBitRate := r.URL.Query().Get("videoBitRate")
	if videoBitRate == "" {
		videoBitRate = "3000k"
	}
	cmdArgs = append(cmdArgs, "-b:v", videoBitRate)

	audioCodec := r.URL.Query().Get("audioCodec")
	if audioCodec == "" {
		audioCodec = "aac"
	}
	cmdArgs = append(cmdArgs, "-c:a", audioCodec)

	audioChannels := r.URL.Query().Get("audioChannels")
	if audioChannels == "" {
		audioChannels = "2"
	}
	cmdArgs = append(cmdArgs, "-ac", audioChannels)

	audioBitRate := r.URL.Query().Get("audioBitRate")
	if audioBitRate == "" {
		audioBitRate = "128kk"
	}
	cmdArgs = append(cmdArgs, "-b:a", audioBitRate)

	preset := r.URL.Query().Get("preset")
	if preset == "" {
		preset = "ultrafast"
	}
	cmdArgs = append(cmdArgs, "-preset", preset)

	container := r.URL.Query().Get("container")
	if container == "" {
		container = "mp4"
	}
	cmdArgs = append(cmdArgs, "-f", container)

	if container == "mp4" {
		cmdArgs = append(cmdArgs, "-movflags", "frag_keyframe+empty_moov")
	}

	cmdArgs = append(cmdArgs, "pipe:1")

	cmd := exec.Command(ffmpegPath, cmdArgs...)

	cmdOut, err := cmd.StdoutPipe()
	if err != nil {
		app.errorLog.Print(err)
		helpers.ErrorJSON(w, err)
		return
	}

	err = cmd.Start()
	if err != nil {
		app.errorLog.Print(err)
		app.infoLog.Print("unable to start ffmpeg command")
		helpers.ErrorJSON(w, err)
		return
	}

	w.Header().Set("Content-Type", fmt.Sprintf("video/%s", container))
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", movie.FileName))

	_, err = io.Copy(w, cmdOut)
	if err != nil {
		app.errorLog.Print(err)
		app.infoLog.Print("unable to copy data to response")
		helpers.ErrorJSON(w, err)
		return
	}

	err = cmd.Wait()
	if err != nil {
		app.errorLog.Print(err)
		app.infoLog.Print("unable to wait for ffmpeg process to finish")
		helpers.ErrorJSON(w, err)
		return
	}
}
