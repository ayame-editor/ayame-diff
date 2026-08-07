package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/ayame-editor/ayame-diff/internal/engine"
)

// TestRunGuardedMapsAPanicToTheRuntimeExitCode is the #137 regression. An
// unrecovered panic exits 2 via the Go runtime, which collides with exitUsage
// (#113) and leaves a script unable to tell a crash from a bad flag. It must
// report the documented runtime-failure code instead.
func TestRunGuardedMapsAPanicToTheRuntimeExitCode(t *testing.T) {
	original := runEngine
	t.Cleanup(func() { runEngine = original })
	runEngine = func(context.Context, engine.Config) (engine.Summary, error) {
		panic("comparison exploded")
	}
	var stdout, stderr bytes.Buffer
	code := runGuarded([]string{"csv", "--left", "a.csv", "--right", "b.csv", "--out", "o.tsv", "--key", "id"}, &stdout, &stderr)
	if code != exitError {
		t.Fatalf("code=%d want %d (runtime error), distinct from exitUsage=%d", code, exitError, exitUsage)
	}
	message := stderr.String()
	if !strings.Contains(message, "comparison exploded") {
		t.Errorf("stderr=%q must name the panic value", message)
	}
	if !strings.Contains(message, "internal error") {
		t.Errorf("stderr=%q must label the failure as internal", message)
	}
	// A crash the user cannot report is worse than a noisy one.
	if !strings.Contains(message, "report") || !strings.Contains(message, "goroutine") {
		t.Errorf("stderr=%q must ask for a report and include the trace", message)
	}
}

// TestRunGuardedLeavesNormalExitCodesAlone keeps the guard from disturbing the
// existing taxonomy: a usage error must still be 2, not the runtime code.
func TestRunGuardedLeavesNormalExitCodesAlone(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	if code := runGuarded([]string{"text", "only-one"}, &stdout, &stderr); code != exitUsage {
		t.Fatalf("code=%d want %d (usage)", code, exitUsage)
	}
	stdout.Reset()
	stderr.Reset()
	if code := runGuarded([]string{"--version"}, &stdout, &stderr); code != exitOK {
		t.Fatalf("code=%d want %d (ok)", code, exitOK)
	}
}
