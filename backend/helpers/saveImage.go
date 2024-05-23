package helpers

import (
	"io"
	"net/http"
	"os"
)

func DownloadImage(url string, filepath string) {
	resp, err := http.Get(url)
	if err != nil {
		LogError("Failed to download image from URL " + url + ": " + err.Error())
		return
	}

	defer resp.Body.Close()

	out, err := os.Create(filepath)
	if err != nil {
		LogError("Failed to create file " + filepath + ": " + err.Error())
		return
	}

	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		LogError("Failed to copy image data to file " + filepath + ": " + err.Error())
	}
}
