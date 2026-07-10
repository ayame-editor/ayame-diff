//go:build linux

package tui

import (
	"syscall"
	"time"
)

const ioctlReadTermios = syscall.TCGETS
const ioctlWriteTermios = syscall.TCSETS

func waitReadable(fd int, timeout time.Duration) (bool, error) {
	if fd < 0 || fd >= 1024 {
		return false, syscall.EINVAL
	}
	var readSet syscall.FdSet
	readSet.Bits[fd/64] |= int64(1) << uint(fd%64)
	tv := syscall.NsecToTimeval(timeout.Nanoseconds())
	n, err := syscall.Select(fd+1, &readSet, nil, nil, &tv)
	if err == syscall.EINTR {
		return false, nil
	}
	return n > 0, err
}
