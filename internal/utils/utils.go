package utils

type Comparable interface {
	string | int | float32 | float64
}

func Any[T Comparable](haystack []T, needle T) bool {
	for _, value := range haystack {
		if value == needle {
			return true
		}
	}

	return false
}

func Map[T any, R any](list []T, f func(value T) R) []R {
	rv := []R{}
	for _, value := range list {
		rv = append(rv, f(value))
	}

	return rv
}
