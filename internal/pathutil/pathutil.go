// Package pathutil provides shared filesystem path comparisons.
package pathutil

import "path/filepath"

// Equal reports whether two non-empty paths resolve to the same cleaned
// absolute path. It compares path spellings and does not follow symlinks.
func Equal(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	absoluteA, errA := filepath.Abs(a)
	absoluteB, errB := filepath.Abs(b)
	return errA == nil && errB == nil && filepath.Clean(absoluteA) == filepath.Clean(absoluteB)
}
