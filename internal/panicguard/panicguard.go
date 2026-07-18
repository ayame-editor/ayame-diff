// Package panicguard converts a panic into an ordinary error so a bug in one
// comparison cannot take the whole process down (#137).
//
// A panic is a bug, not a supported control-flow path, so nothing here tries to
// make the failed work succeed. The goal is narrower and specific to a tool that
// must not die: a panic inside one request, one partition worker, or one file
// comparison should fail *that unit* with a diagnosable error while the CLI
// exits with its documented runtime-failure code and the local GUI server keeps
// serving. The captured stack is preserved on the error so the bug is still
// reportable rather than silently swallowed.
//
// Guard belongs at goroutine boundaries in particular: net/http recovers a
// panic on the goroutine running a handler, but a panic on a worker goroutine
// the handler spawned is unrecoverable from the handler and kills the process.
//
// Runtime conditions that bypass recover — out-of-memory, concurrent map
// writes, stack exhaustion — remain fatal by design; this package does not and
// cannot mask them.
package panicguard

import (
	"fmt"
	"runtime/debug"
)

// Error is the error a recovered panic becomes. It keeps the panic value and
// the stack captured at recovery so the failure stays diagnosable.
type Error struct {
	Value any
	Stack []byte
}

func (e *Error) Error() string { return fmt.Sprintf("internal error: %v", e.Value) }

// Unwrap exposes a panic value that was itself an error, so callers can still
// match sentinels with errors.Is/As.
func (e *Error) Unwrap() error {
	if err, ok := e.Value.(error); ok {
		return err
	}
	return nil
}

// Guard recovers a panic in the calling goroutine and stores it through err.
// Use it as the first deferred call of any function that owns a goroutine or a
// unit of work whose failure must not be fatal:
//
//	func work() (err error) {
//		defer panicguard.Guard(&err)
//		return mayPanic()
//	}
//
// An error already assigned to *err wins: a function that failed normally and
// then panicked while unwinding keeps the more meaningful first failure.
func Guard(err *error) {
	value := recover()
	if value == nil {
		return
	}
	if err == nil || *err != nil {
		return
	}
	*err = &Error{Value: value, Stack: debug.Stack()}
}

// Call runs fn, returning a *Error instead of panicking. It suits call sites
// that have no error return of their own to guard, such as a bare goroutine.
func Call(fn func()) (err error) {
	defer Guard(&err)
	fn()
	return nil
}
