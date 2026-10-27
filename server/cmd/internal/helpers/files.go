package helpers

import (
	"io"
	"os"
	"path/filepath"
)

// WriteFileAtomic writes src to path through a temporary file in the same
// directory renamed into place, so a reader never sees a partial file and a
// failed write leaves nothing under path, nor any temporary file.
func WriteFileAtomic(path string, src io.Reader) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), TEMP_FILE_PREFIX+"*")
	if err != nil {
		return err
	}

	_, err = io.Copy(tmp, src)
	closeErr := tmp.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(tmp.Name(), path)
	}
	if err != nil {
		_ = os.Remove(tmp.Name())
	}

	return err
}
