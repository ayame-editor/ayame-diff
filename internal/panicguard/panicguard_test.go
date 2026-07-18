package panicguard

import (
	"errors"
	"strings"
	"sync"
	"testing"
)

func TestGuardConvertsPanicToError(t *testing.T) {
	t.Parallel()
	work := func() (err error) {
		defer Guard(&err)
		panic("boom")
	}
	err := work()
	if err == nil {
		t.Fatal("panic did not become an error")
	}
	var guarded *Error
	if !errors.As(err, &guarded) {
		t.Fatalf("err=%T want *panicguard.Error", err)
	}
	if guarded.Value != "boom" {
		t.Errorf("value=%v want boom", guarded.Value)
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("message=%q must name the panic value", err.Error())
	}
	// The stack is what keeps a recovered bug reportable rather than swallowed.
	if !strings.Contains(string(guarded.Stack), "panicguard") {
		t.Errorf("stack does not reach the panic site: %s", guarded.Stack)
	}
}

func TestGuardLeavesSuccessAlone(t *testing.T) {
	t.Parallel()
	work := func() (err error) {
		defer Guard(&err)
		return nil
	}
	if err := work(); err != nil {
		t.Fatalf("err=%v want nil", err)
	}
}

func TestGuardKeepsTheFirstFailure(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("original failure")
	work := func() (err error) {
		defer Guard(&err)
		defer panic("panic while unwinding")
		return sentinel
	}
	err := work()
	if !errors.Is(err, sentinel) {
		t.Fatalf("err=%v want the original failure to survive", err)
	}
}

func TestGuardUnwrapsAnErrorPanicValue(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("panicked with an error")
	work := func() (err error) {
		defer Guard(&err)
		panic(sentinel)
	}
	if err := work(); !errors.Is(err, sentinel) {
		t.Fatalf("err=%v want errors.Is to match the panic value", err)
	}
}

func TestCallGuardsABareGoroutine(t *testing.T) {
	t.Parallel()
	// The case the package exists for: without a guard this panic would take
	// the whole process down rather than failing one unit of work.
	var wg sync.WaitGroup
	var err error
	wg.Add(1)
	go func() {
		defer wg.Done()
		err = Call(func() { panic("worker exploded") })
	}()
	wg.Wait()
	if err == nil || !strings.Contains(err.Error(), "worker exploded") {
		t.Fatalf("err=%v want the worker panic reported as an error", err)
	}
}

func TestCallReportsNoErrorWithoutAPanic(t *testing.T) {
	t.Parallel()
	ran := false
	if err := Call(func() { ran = true }); err != nil {
		t.Fatalf("err=%v want nil", err)
	}
	if !ran {
		t.Error("Call did not run fn")
	}
}
