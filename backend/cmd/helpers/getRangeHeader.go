package helpers

import "strconv"

func ParseRange(rangeHeader string, size int64, chunkSize int64) (int64, int64) {
	start, _ := strconv.ParseInt(rangeHeader[6:], 10, 64)
	end := start + chunkSize - 1
	if end >= size {
		end = size - 1
	}

	return start, end
}
