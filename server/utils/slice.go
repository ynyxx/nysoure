package utils

func RemoveDuplicate[T comparable](slice []T) []T {
	seen := make(map[T]struct{})
	var result []T
	for _, v := range slice {
		if _, ok := seen[v]; !ok {
			seen[v] = struct{}{}
			result = append(result, v)
		}
	}
	return result
}

// IntersectPreserveOrder returns items that appear in both slices, keeping the
// order and uniqueness of values in base.
func IntersectPreserveOrder[T comparable](base, other []T) []T {
	if len(base) == 0 || len(other) == 0 {
		return []T{}
	}
	allowed := make(map[T]struct{}, len(other))
	for _, v := range other {
		allowed[v] = struct{}{}
	}
	seen := make(map[T]struct{}, len(base))
	result := make([]T, 0)
	for _, v := range base {
		if _, ok := allowed[v]; !ok {
			continue
		}
		if _, dup := seen[v]; dup {
			continue
		}
		seen[v] = struct{}{}
		result = append(result, v)
	}
	return result
}
