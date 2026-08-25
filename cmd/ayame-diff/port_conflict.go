package main

import (
	"errors"
	"syscall"
)

// isPortConflict reports whether err means the address is already taken.
// Windows answers a busy port with WSAEADDRINUSE, a different errno from the
// POSIX EADDRINUSE that syscall exposes there, so the per-platform files add
// the codes this one cannot name portably.
func isPortConflict(err error) bool {
	if errors.Is(err, syscall.EADDRINUSE) {
		return true
	}
	for _, errno := range extraPortConflictErrnos {
		if errors.Is(err, errno) {
			return true
		}
	}
	return false
}
