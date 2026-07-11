//go:build linux || darwin || freebsd || openbsd || netbsd || dragonfly

package engine

import "syscall"

func openFileLimits() (soft, hard uint64, supported bool, err error) {
	var limit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &limit); err != nil {
		return 0, 0, true, err
	}
	return limit.Cur, limit.Max, true, nil
}

func setOpenFileSoftLimit(soft uint64) error {
	var limit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &limit); err != nil {
		return err
	}
	limit.Cur = soft
	return syscall.Setrlimit(syscall.RLIMIT_NOFILE, &limit)
}
