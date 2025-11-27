package server

import (
	"fmt"
	"strconv"
)

func parseString[T any](s string) (T, error) {
	var zero T

	switch any(zero).(type) {
	case string:
		return any(s).(T), nil

	case int, int64:
		if v, err := strconv.ParseInt(s, 10, 64); err != nil {
			return zero, err
		} else {
			return any(v).(T), nil
		}

	default:
		return zero, fmt.Errorf("unsupported type %T", zero)
	}
}
