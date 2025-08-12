package helpers

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
)

func CreateDir(dirPath string) error {
	if dirPath == "" {
		return errors.New("directory path is required")
	}

	_, err := os.Stat(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			err = os.MkdirAll(dirPath, 0755)
			if err != nil {
				return errors.New("could not create directory: " + err.Error())
			}
		} else {
			if os.IsPermission(err) {
				return errors.New("permission denied accessing directory: " + dirPath)
			}

			return err
		}
	}

	return nil
}

func AddWatcherDirs(baseDir string, w *fsnotify.Watcher) error {
	err := w.Add(baseDir)
	if err != nil {
		return err
	}

	err = filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			w.Add(path)
		}

		return nil
	})

	if err != nil {
		return err
	}

	return nil
}

func IsDir(dirPath string) (bool, error) {
	info, err := os.Stat(dirPath)
	if err != nil {
		return false, err
	}

	return info.IsDir(), nil
}
