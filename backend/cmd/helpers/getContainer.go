package helpers

import (
	"os"

	"github.com/h2non/filetype"
)

func GetContainerFormat(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	header := make([]byte, 261)
	_, err = file.Read(header)
	if err != nil {
		return "", err
	}

	kind, err := filetype.Match(header)
	if err != nil {
		return "", err
	}

	return kind.MIME.Subtype, nil
}
