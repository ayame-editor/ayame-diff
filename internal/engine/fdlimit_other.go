//go:build !linux && !darwin

package engine

// ensureFileDescriptorBudget is a no-op where RLIMIT_NOFILE is not
// available (Windows has no comparable practical per-process limit).
func ensureFileDescriptorBudget(Config) error { return nil }
