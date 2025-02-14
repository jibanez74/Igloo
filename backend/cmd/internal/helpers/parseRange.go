package helpers

import (
	"fmt"
	"strconv"
	"strings"
)

type HttpRange struct {
	Start, End int64
}

func ParseRange(rangeHeader string, size int64) ([]HttpRange, error) {
	if !strings.HasPrefix(rangeHeader, "bytes=") {
		return nil, fmt.Errorf("invalid range format")
	}

	ranges := strings.Split(strings.TrimPrefix(rangeHeader, "bytes="), ",")
	parsedRanges := make([]HttpRange, 0, len(ranges))

	for _, r := range ranges {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}

		i := strings.Index(r, "-")
		if i < 0 {
			return nil, fmt.Errorf("invalid range format")
		}

		start, end := strings.TrimSpace(r[:i]), strings.TrimSpace(r[i+1:])

		var startByte, endByte int64

		if start == "" {
			n, err := strconv.ParseInt(end, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid range format")
			}

			if n > size {
				n = size
			}

			startByte = size - n
			endByte = size - 1
		} else {
			n, err := strconv.ParseInt(start, 10, 64)
			if err != nil || n >= size {
				return nil, fmt.Errorf("invalid range format")
			}

			startByte = n
			if end == "" {
				endByte = size - 1
			} else {
				n, err := strconv.ParseInt(end, 10, 64)
				if err != nil || n >= size {
					endByte = size - 1
				} else {
					endByte = n
				}
			}
		}

		if startByte > endByte {
			return nil, fmt.Errorf("invalid range format")
		}

		parsedRanges = append(parsedRanges, HttpRange{startByte, endByte})
	}

	return parsedRanges, nil
}
