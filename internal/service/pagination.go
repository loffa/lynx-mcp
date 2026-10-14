package service

import "fmt"

const (
	defaultListLimit = 100
	maxListLimit     = 10_000
)

func normalizePagination(limit, offset int) (int, error) {
	if limit < 0 || limit > maxListLimit {
		return 0, fmt.Errorf("limit must be between 1 and 10 000")
	}

	if offset < 0 {
		return 0, fmt.Errorf("offset cannot be negative")
	}

	if limit == 0 {
		limit = defaultListLimit
	}

	return limit, nil
}

func pageBounds(total, limit, offset int) (int, int) {
	start := min(offset, total)
	end := start + min(limit, total-start)

	return start, end
}
