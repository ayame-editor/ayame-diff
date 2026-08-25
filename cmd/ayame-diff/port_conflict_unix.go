//go:build !windows

package main

import "syscall"

// Only Windows reports a busy port under a second errno.
var extraPortConflictErrnos []syscall.Errno
