package helpers

import (
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
)

func DownloadImage(urlStr string, filepathStr string) {
	filepathStr = filepath.Clean(filepathStr)
	if filepath.IsAbs(filepathStr) || filepath.HasPrefix(filepathStr, "..") {
		LogError("Invalid file path: " + filepathStr)
		return
	}

	_, err := os.Stat(filepathStr)
	if err == nil {
		return
	}

	parsedUrl, err := url.Parse(urlStr)
	if err != nil || !parsedUrl.IsAbs() {
		LogError("Invalid URL: " + urlStr)
		return
	}

	resp, err := http.Get(urlStr)
	if err != nil {
		LogError("Failed to download image from URL " + urlStr + ": " + err.Error())
		return
	}

	defer resp.Body.Close()

	out, err := os.Create(filepathStr)
	if err != nil {
		LogError("Failed to create file " + filepathStr + ": " + err.Error())
		return
	}

	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		LogError("Failed to copy image data to file " + filepathStr + ": " + err.Error())
	}
}
