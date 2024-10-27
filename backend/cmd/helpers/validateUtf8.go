package helpers

import "unicode/utf8"

func SanitizeString(input string) string {
	validRunes := make([]rune, 0, len(input))
	for _, r := range input {
		if r == utf8.RuneError {
			continue // skip invalid runes
		}

		validRunes = append(validRunes, r)
	}

	return string(validRunes)
}
