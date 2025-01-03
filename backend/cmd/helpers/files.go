package helpers

import (
	"net/http"
	"os"
)

func CheckFileExist(path string) (int, error) {
	_, err := os.Stat(path)
	if err == nil {
		return http.StatusOK, nil
	}

	if err == os.ErrNotExist {
		return http.StatusNotFound, err
	}

	if err == os.ErrPermission {
		return http.StatusForbidden, err
	}

	return http.StatusInternalServerError, err
}

func RemoveFile(path string) (int, error) {
	status, err := CheckFileExist(path)
	if err != nil {
		return status, err
	}

	err = os.Remove(path)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	return http.StatusOK, nil
}
