package helpers

import (
	"errors"
	"strconv"
	"strings"
)

func SplitSliceBySlash(str string) ([]int32, error) {
	if str == "" {
		return nil, errors.New("got empty string iSplitSliceBySlash function")
	}

	parts := strings.Split(str, "/")

	val1, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, err
	}

	val2, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, err
	}
	list := []int32{
		int32(val1),
		int32(val2),
	}

	return list, nil
}
