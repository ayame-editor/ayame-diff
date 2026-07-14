package main

import (
	"bytes"
	"testing"
)

// TestExitCodeTaxonomy pins the #113 exit-code contract so scripts can tell a
// diff apart from a failure: runtime errors use 3 (not 1), usage errors 2, and
// a real diff under --diff-exit-code stays 1.
func TestExitCodeTaxonomy(t *testing.T) {
	t.Parallel()
	var out, err bytes.Buffer

	// Runtime error: a missing input file is neither a usage error nor a diff.
	if code := runText([]string{"/no/such/old.txt", "/no/such/new.txt"}, &out, &err); code != exitError {
		t.Fatalf("missing-file text diff: code=%d, want %d (runtime)\nstderr=%q", code, exitError, err.String())
	}

	// Runtime error: a missing directory fails at compare time, not as a diff.
	out.Reset()
	err.Reset()
	if code := runDir([]string{"/no/such/old", "/no/such/new"}, &out, &err); code != exitError {
		t.Fatalf("missing-dir compare: code=%d, want %d (runtime)\nstderr=%q", code, exitError, err.String())
	}

	// Runtime error: serve cannot bind an invalid address.
	out.Reset()
	err.Reset()
	if code := runServe([]string{"--addr", "127.0.0.1:99999999"}, &out, &err); code != exitError {
		t.Fatalf("serve bind failure: code=%d, want %d (runtime)\nstderr=%q", code, exitError, err.String())
	}

	// Usage error: wrong argument count stays 2, distinct from runtime and diff.
	out.Reset()
	err.Reset()
	if code := runText([]string{"only-one"}, &out, &err); code != exitUsage {
		t.Fatalf("bad arg count: code=%d, want %d (usage)", code, exitUsage)
	}
}
