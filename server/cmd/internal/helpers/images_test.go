package helpers

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testImage returns size bytes that http.DetectContentType reads as the
// format whose signature they start with.
func testImage(signature string, size int) []byte {
	data := bytes.Repeat([]byte{0x01}, size)
	copy(data, signature)
	return data
}

const (
	jpegSignature = "\xff\xd8\xff\xe0"
	pngSignature  = "\x89PNG\r\n\x1a\n"
	gifSignature  = "GIF89a"
	webpSignature = "RIFF\x00\x00\x00\x00WEBPVP8 "
)

// dirEntries lists the names in dir, or none when dir does not exist.
func dirEntries(t *testing.T, dir string) []string {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

func TestSaveImage_NamesTheFileByItsSniffedType(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		wantExt string
	}{
		{"jpeg", testImage(jpegSignature, 2048), ".jpg"},
		{"png", testImage(pngSignature, 2048), ".png"},
		{"gif", testImage(gifSignature, 2048), ".gif"},
		{"webp", testImage(webpSignature, 2048), ".webp"},
		{"shorter than the sniff window", testImage(pngSignature, 64), ".png"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "movies", "603")

			path, err := SaveImage(dir, "the_matrix_poster", bytes.NewReader(tt.data), 1<<20)
			if err != nil {
				t.Fatalf("SaveImage: %v", err)
			}

			wantPath := filepath.Join(dir, "the_matrix_poster"+tt.wantExt)
			if path != wantPath {
				t.Fatalf("path = %q, want %q", path, wantPath)
			}
			saved, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read saved image: %v", err)
			}
			if !bytes.Equal(saved, tt.data) {
				t.Fatalf("saved %d bytes, want the %d bytes read", len(saved), len(tt.data))
			}
			if !IsImageExtension(tt.wantExt) {
				t.Fatalf("IsImageExtension(%q) = false for an extension SaveImage gives", tt.wantExt)
			}
		})
	}
}

// SVG is refused along with other non-raster content: served from the API's
// origin, it could run script.
func TestSaveImage_RejectsContentThatIsNotARasterImage(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{"svg document", `<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"></svg>`},
		{"bare svg", `<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`},
		{"html", "<!DOCTYPE html><html><body>not an image</body></html>"},
		{"text", "plain text is not an image"},
		{"empty", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "networks", "49")

			_, err := SaveImage(dir, "hbo_logo", strings.NewReader(tt.data), 1<<20)
			if !errors.Is(err, ErrUnsupportedImageType) {
				t.Fatalf("err = %v, want ErrUnsupportedImageType", err)
			}
			names := dirEntries(t, dir)
			if len(names) != 0 {
				t.Fatalf("rejected image left %v behind", names)
			}
		})
	}
	if IsImageExtension(".svg") {
		t.Fatal(`IsImageExtension(".svg") = true, want false`)
	}
}

func TestSaveImage_SizeLimitKeepsTheExistingFile(t *testing.T) {
	dir := t.TempDir()
	existing := testImage(pngSignature, 100)
	err := os.WriteFile(filepath.Join(dir, "cover.png"), existing, 0o644)
	if err != nil {
		t.Fatal(err)
	}

	_, err = SaveImage(dir, "cover", bytes.NewReader(testImage(pngSignature, 4097)), 4096)
	if !errors.Is(err, ErrImageTooLarge) {
		t.Fatalf("err = %v, want ErrImageTooLarge", err)
	}

	kept, err := os.ReadFile(filepath.Join(dir, "cover.png"))
	if err != nil {
		t.Fatalf("read existing image: %v", err)
	}
	if !bytes.Equal(kept, existing) {
		t.Fatal("an oversized image replaced the existing file")
	}
	names := dirEntries(t, dir)
	if len(names) != 1 {
		t.Fatalf("dir holds %v, want only cover.png", names)
	}

	path, err := SaveImage(dir, "cover", bytes.NewReader(testImage(pngSignature, 4096)), 4096)
	if err != nil {
		t.Fatalf("an image of exactly the limit: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != 4096 {
		t.Fatalf("saved %d bytes, want 4096", info.Size())
	}
}

func TestSaveImage_ReplacesAnExistingFile(t *testing.T) {
	dir := t.TempDir()

	_, err := SaveImage(dir, "poster", bytes.NewReader(testImage(jpegSignature, 600)), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	replacement := testImage(jpegSignature, 900)
	path, err := SaveImage(dir, "poster", bytes.NewReader(replacement), 1<<20)
	if err != nil {
		t.Fatal(err)
	}

	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(saved, replacement) {
		t.Fatal("the second save did not replace the first")
	}
	names := dirEntries(t, dir)
	if len(names) != 1 {
		t.Fatalf("dir holds %v, want only poster.jpg", names)
	}
}

func TestSaveImage_RejectsNamesThatAreNotASingleFileName(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "people")

	for _, name := range []string{"", ".", "..", "../escape", "a/b", ".hidden"} {
		_, err := SaveImage(dir, name, bytes.NewReader(testImage(pngSignature, 100)), 1<<20)
		if err == nil {
			t.Errorf("SaveImage accepted name %q", name)
		}
	}
	names := dirEntries(t, root)
	if len(names) != 0 {
		t.Fatalf("rejected names left %v behind", names)
	}
}

// failingReader yields data and then fails, like a download cut off midway.
type failingReader struct {
	data []byte
	err  error
}

func (f *failingReader) Read(p []byte) (int, error) {
	if len(f.data) == 0 {
		return 0, f.err
	}
	n := copy(p, f.data)
	f.data = f.data[n:]
	return n, nil
}

func TestSaveImage_ReadFailureLeavesNothingBehind(t *testing.T) {
	cutOff := errors.New("connection reset")
	tests := []struct {
		name string
		data []byte
	}{
		{"before the type is known", []byte(pngSignature)},
		{"after the type is known", testImage(pngSignature, 2048)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "albums")

			_, err := SaveImage(dir, "cover", &failingReader{data: tt.data, err: cutOff}, 1<<20)
			if !errors.Is(err, cutOff) {
				t.Fatalf("err = %v, want the read error", err)
			}
			names := dirEntries(t, dir)
			if len(names) != 0 {
				t.Fatalf("failed read left %v behind", names)
			}
		})
	}
}
