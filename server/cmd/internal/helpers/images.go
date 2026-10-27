package helpers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
)

// imageSniffLength is the most http.DetectContentType looks at.
const imageSniffLength = 512

// maxDownloadedImageSize is far above any artwork size Igloo requests; it only
// bounds what a misbehaving server can write to disk.
const maxDownloadedImageSize = 10 << 20

var (
	ErrUnsupportedImageType = errors.New("unsupported image type")
	ErrImageTooLarge        = errors.New("image too large")
)

// imageExtensions maps the image types SaveImage accepts to the extension the
// saved file gets. SVG is left out on purpose: served from the API's origin,
// an SVG document can run script.
var imageExtensions = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

// IsImageExtension reports whether ext is an extension SaveImage gives a file.
func IsImageExtension(ext string) bool {
	for _, allowed := range imageExtensions {
		if ext == allowed {
			return true
		}
	}

	return false
}

// SaveImage stores the image src carries as name, plus the extension of the
// type its bytes sniff as, in dir, which it creates if needed, and returns the
// full path of the saved file. A type outside imageExtensions returns
// ErrUnsupportedImageType, and more than maxBytes returns ErrImageTooLarge;
// on any failure nothing is left in dir, and an existing file of the same
// name is kept.
func SaveImage(dir, name string, src io.Reader, maxBytes int64) (string, error) {
	isPlainName := name != "" && !strings.HasPrefix(name, ".") && filepath.Base(name) == name
	if !isPlainName {
		return "", fmt.Errorf("invalid image file name %q", name)
	}

	prefix := make([]byte, imageSniffLength)
	n, err := io.ReadFull(src, prefix)
	isShortImage := errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF)
	if err != nil && !isShortImage {
		return "", fmt.Errorf("failed to read image: %w", err)
	}
	prefix = prefix[:n]

	ext, ok := imageExtensions[http.DetectContentType(prefix)]
	if !ok {
		return "", ErrUnsupportedImageType
	}

	_, err = GetOrCreateDir(dir)
	if err != nil {
		return "", err
	}

	path := filepath.Join(dir, name+ext)
	body := &cappedReader{src: io.MultiReader(bytes.NewReader(prefix), src), remaining: maxBytes}

	err = WriteFileAtomic(path, body)
	if err != nil {
		return "", err
	}

	return path, nil
}

// ImageStatusError reports a download the server answered with a status other
// than 200.
type ImageStatusError struct {
	StatusCode int
}

func (e *ImageStatusError) Error() string {
	return fmt.Sprintf("image request returned status %d", e.StatusCode)
}

// DownloadImage fetches the image at imageURL and stores it through SaveImage
// as name in dir, returning the full path of the saved file. A response other
// than 200 returns an *ImageStatusError, and a body that is not a supported
// image returns ErrUnsupportedImageType.
func DownloadImage(ctx context.Context, client *http.Client, imageURL, dir, name string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	isOK := resp.StatusCode == http.StatusOK
	if !isOK {
		return "", &ImageStatusError{StatusCode: resp.StatusCode}
	}

	return SaveImage(dir, name, resp.Body, maxDownloadedImageSize)
}

// cappedReader passes through at most remaining bytes of src and fails with
// ErrImageTooLarge once src turns out to hold more.
type cappedReader struct {
	src       io.Reader
	remaining int64
}

func (c *cappedReader) Read(p []byte) (int, error) {
	if c.remaining <= 0 {
		var probe [1]byte
		n, err := c.src.Read(probe[:])
		if n > 0 {
			return 0, ErrImageTooLarge
		}

		return 0, err
	}

	if int64(len(p)) > c.remaining {
		p = p[:c.remaining]
	}

	n, err := c.src.Read(p)
	c.remaining -= int64(n)

	return n, err
}
