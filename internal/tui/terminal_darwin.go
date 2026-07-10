//go:build darwin

package tui

import (
	"syscall"
	"time"
)

const ioctlReadTermios = syscall.TIOCGETA
const ioctlWriteTermios = syscall.TIOCSETA

func waitReadable(fd int, timeout time.Duration) (bool, error) {
	if fd < 0 || fd >= 1024 {
		return false, syscall.EINVAL
	}
	var readSet syscall.FdSet
	readSet.Bits[fd/32] |= int32(1) << uint(fd%32)
	tv := syscall.NsecToTimeval(timeout.Nanoseconds())
	err := syscall.Select(fd+1, &readSet, nil, nil, &tv)
	if err == syscall.EINTR {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	mask := int32(1) << uint(fd%32)
	return readSet.Bits[fd/32]&mask != 0, nil
}
