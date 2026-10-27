package helpers

import (
	"io"
	"os"
	"path/filepath"
)

// tempFilePattern names the temporary files WriteFileAtomic creates beside
// their target, so anything left behind by a crash is recognizable.
const tempFilePattern = ".tmp-*"

// WriteFileAtomic writes src to path through a temporary file in the same
// directory renamed into place, so a reader never sees a partial file and a
// failed write leaves nothing under path, nor any temporary file.
func WriteFileAtomic(path string, src io.Reader) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), tempFilePattern)
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
