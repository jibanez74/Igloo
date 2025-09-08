package helpers

import (
	"errors"
	"os"
)

func CreateDir(dirPath string) error {
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
