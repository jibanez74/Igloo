package helpers

import (
	"errors"
	"strconv"
	"strings"
)

func ParseRange(rangeHdr string, fileSize int64) (int64, int64, error) {
	if !strings.HasPrefix(rangeHdr, "bytes=") {
		return 0, 0, errors.New("invalid range")
	}

	parts := strings.Split(strings.TrimPrefix(rangeHdr, "bytes="), "-")
	if len(parts) != 2 {
		return 0, 0, errors.New("invalid range format")
	}

	start, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || start < 0 {
		return 0, 0, errors.New("invalid range start")
	}

	var end int64

	if parts[1] == "" {
		end = fileSize - 1
	} else {
		end, err = strconv.ParseInt(parts[1], 10, 64)
		if err != nil || end < start {
			return 0, 0, errors.New("invalid range end")
		}
	}

	return start, end, nil
}
