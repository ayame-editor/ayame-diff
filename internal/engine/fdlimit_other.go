//go:build !linux && !darwin && !freebsd && !openbsd && !netbsd && !dragonfly

package engine

func openFileLimits() (soft, hard uint64, supported bool, err error) {
	return 0, 0, false, nil
}

func setOpenFileSoftLimit(uint64) error { return nil }
