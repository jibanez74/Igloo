package main

import (
	"errors"
	"fmt"
	"igloo/cmd/helpers"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const ffmpegPath = "/bin/ffmpeg"

func (app *config) DirectStreamVideo(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("filePath")
	if filePath == "" {
		helpers.ErrorJSON(w, errors.New("file path is required"), http.StatusBadRequest)
		return
	}

	fileInfo, err := os.Stat(filePath)
	if err != nil || os.IsNotExist(err) {
		helpers.ErrorJSON(w, errors.New("file not found"), http.StatusNotFound)
		return
	}

	file, err := os.Open(filePath)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("unable to open file"), http.StatusInternalServerError)
		return
	}
	defer file.Close()

	size := fileInfo.Size()

	contentType := r.URL.Query().Get("contentType")
	if contentType == "" {
		helpers.ErrorJSON(w, errors.New("container is required"), http.StatusBadRequest)
		return
	}

	rangeHeader := r.Header.Get("Range")

	w.Header().Set("Content-Type", contentType)

	var start int64 = 0
	var end int64 = size - 1

	if rangeHeader != "" {
		rangeHeader = strings.Replace(rangeHeader, "bytes=", "", 1)
		parts := strings.Split(rangeHeader, "-")

		if parts[0] != "" {
			start, err = strconv.ParseInt(parts[0], 10, 64)
			if err != nil || start < 0 || start >= size {
				helpers.ErrorJSON(w, errors.New("invalid start range"), http.StatusBadRequest)
				return
			}
		}

		if parts[1] != "" {
			end, err = strconv.ParseInt(parts[1], 10, 64)
			if err != nil || end >= size {
				end = size - 1 // Safeguard if end is out of bounds
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

		_, err = file.Seek(start, 0)
		if err != nil {
			helpers.ErrorJSON(w, errors.New("unable to seek file"), http.StatusInternalServerError)
			return
		}

		http.ServeContent(w, r, filePath, time.Now(), file)
	} else {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", size))
		w.WriteHeader(http.StatusOK)

		http.ServeContent(w, r, filePath, time.Now(), file)
	}
}

func (app *config) StreamTranscodedVideo(w http.ResponseWriter, r *http.Request) {
	var cmdArgs []string

	filePath := r.URL.Query().Get("filePath")
	if filePath == "" {
		helpers.ErrorJSON(w, errors.New("file path is required"), http.StatusBadRequest)
		return
	}

	_, err := os.Stat(filePath)
	if err != nil || os.IsNotExist(err) {
		helpers.ErrorJSON(w, errors.New("file not found"), http.StatusNotFound)
		return
	}
	cmdArgs = append(cmdArgs, "-i", filePath)

	videoCodec := r.URL.Query().Get("videoCodec")
	if videoCodec == "" {
		videoCodec = "libx264"
	}
	cmdArgs = append(cmdArgs, "-c:v", videoCodec)

	videoHeight := r.URL.Query().Get("videoHeight")
	if videoHeight == "" {
		helpers.ErrorJSON(w, errors.New("video height is required"), http.StatusBadRequest)
		return
	}
	cmdArgs = append(cmdArgs, "-vf", fmt.Sprintf("scale=-1:%s", videoHeight))

	videoBitRate := r.URL.Query().Get("videoBitRate")
	if videoBitRate == "" {
		videoBitRate = "2000k"
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
		helpers.ErrorJSON(w, err)
		return
	}

	err = cmd.Start()
	if err != nil {
		helpers.ErrorJSON(w, err)
		return
	}

	_, err = io.Copy(w, cmdOut)
	if err != nil {
		helpers.ErrorJSON(w, err)
		return
	}

	err = cmd.Wait()
	if err != nil {
		helpers.ErrorJSON(w, err)
		return
	}
}
