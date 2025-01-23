package helpers

import "errors"

func ValidateInArray[T comparable](value T, validValues []T, errMsg string) error {
	for _, v := range validValues {
		if value == v {
			return nil
		}
	}

	return errors.New(errMsg)
}
