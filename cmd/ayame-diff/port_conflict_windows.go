package main

import "syscall"

// wsaEAddrInUse is WSAEADDRINUSE (10048), what Winsock returns for a port that
// another process already listens on. syscall.EADDRINUSE on Windows is a
// separate value that never reaches a caller of net.Listen.
const wsaEAddrInUse = syscall.Errno(10048)

var extraPortConflictErrnos = []syscall.Errno{wsaEAddrInUse}
