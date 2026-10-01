package main

import (
	"bytes"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ayame-editor/ayame-diff/internal/server"
)

func writeToolFile(t *testing.T, path, content string) string {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// toolTestDeps returns deps whose serve returns immediately, standing in for a
// browser tab that has just closed, and capturing the URL the browser got.
func toolTestDeps(opened *string) guiCommandDeps {
	return guiCommandDeps{
		newHandler: newServerHandler,
		listen:     net.Listen,
		serve:      func(net.Listener, http.Handler, <-chan struct{}) error { return nil },
		openBrowser: func(target string) error {
			if opened != nil {
				*opened = target
			}
			return nil
		},
	}
}

func TestRunDifftoolTerminalUsesLabels(t *testing.T) {
	dir := t.TempDir()
	left := writeToolFile(t, filepath.Join(dir, "left.txt"), "same\nold\n")
	right := writeToolFile(t, filepath.Join(dir, "right.txt"), "same\nnew\n")

	var stdout, stderr bytes.Buffer
	code := runDifftool([]string{
		"--label", "HEAD~1:foo.txt", "--label", "HEAD:foo.txt", left, right,
	}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "HEAD~1:foo.txt vs HEAD:foo.txt") {
		t.Fatalf("labels missing from difftool banner: %q", stderr.String())
	}
}

func TestRunTextLabelAppearsInUnifiedHeader(t *testing.T) {
	dir := t.TempDir()
	left := writeToolFile(t, filepath.Join(dir, "left.txt"), "same\nold\n")
	right := writeToolFile(t, filepath.Join(dir, "right.txt"), "same\nnew\n")

	var stdout, stderr bytes.Buffer
	code := runText([]string{"--label", "LEFT:logical", "--label", "RIGHT:logical", "--format", "unified", left, right}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "LEFT:logical") || !strings.Contains(stdout.String(), "RIGHT:logical") {
		t.Fatalf("labels missing from unified header:\n%s", stdout.String())
	}
}

func TestRunDifftoolRejectsTooManyLabelsAndBadArity(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := runDifftool([]string{"-L", "a", "-L", "b", "-L", "c", "x", "y"}, &stdout, &stderr); code != exitUsage {
		t.Fatalf("too many labels code=%d stderr=%q", code, stderr.String())
	}
	stderr.Reset()
	if code := runDifftool([]string{"only-one"}, &stdout, &stderr); code != exitUsage {
		t.Fatalf("arity code=%d stderr=%q", code, stderr.String())
	}
}

func TestRunDifftoolGUIBlocksAndCarriesLabels(t *testing.T) {
	dir := t.TempDir()
	left := writeToolFile(t, filepath.Join(dir, "left.txt"), "a\n")
	right := writeToolFile(t, filepath.Join(dir, "right.txt"), "b\n")

	var opened string
	code := runDifftoolWithDeps(
		[]string{"--wait", "-L", "OLD:side", "-L", "NEW:side", left, right},
		&bytes.Buffer{}, &bytes.Buffer{}, toolTestDeps(&opened),
	)
	if code != exitOK {
		t.Fatalf("code=%d", code)
	}
	parsed, err := url.Parse(opened)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	if query.Get("old") != left || query.Get("new") != right || query.Get("mode") != "text" || query.Get("autorun") != "1" {
		t.Fatalf("url=%q", opened)
	}
	if query.Get("oldLabel") != "OLD:side" || query.Get("newLabel") != "NEW:side" {
		t.Fatalf("labels missing from url %q", opened)
	}
}

func TestRunMergetoolTerminalResolvedAndUnresolved(t *testing.T) {
	dir := t.TempDir()
	base := writeToolFile(t, filepath.Join(dir, "base.txt"), "one\ntwo\nthree\n")
	local := writeToolFile(t, filepath.Join(dir, "local.txt"), "ONE\ntwo\nthree\n")
	remote := writeToolFile(t, filepath.Join(dir, "remote.txt"), "one\ntwo\nTHREE\n")

	// Independent edits: clean automatic merge, exit 0.
	merged := filepath.Join(dir, "merged.txt")
	var stdout, stderr bytes.Buffer
	code := runMergetool([]string{"--output", merged, base, local, remote}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("clean merge code=%d stderr=%q", code, stderr.String())
	}
	data, err := os.ReadFile(merged)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "ONE\ntwo\nTHREE\n" {
		t.Fatalf("merged=%q", data)
	}

	// Same base region changed differently: unresolved markers, exit 1.
	conflictLocal := writeToolFile(t, filepath.Join(dir, "conflict-local.txt"), "LOCAL\n")
	conflictRemote := writeToolFile(t, filepath.Join(dir, "conflict-remote.txt"), "REMOTE\n")
	conflictBase := writeToolFile(t, filepath.Join(dir, "conflict-base.txt"), "BASE\n")
	conflictMerged := filepath.Join(dir, "conflict-merged.txt")
	stdout.Reset()
	stderr.Reset()
	code = runMergetool([]string{"--output", conflictMerged, conflictBase, conflictLocal, conflictRemote}, &stdout, &stderr)
	if code != exitDiff {
		t.Fatalf("conflict merge code=%d want %d stderr=%q", code, exitDiff, stderr.String())
	}
	data, err = os.ReadFile(conflictMerged)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "<<<<<<< LEFT") {
		t.Fatalf("conflict merge did not write markers: %q", data)
	}
}

func TestRunMergetoolOrderAndOutputValidation(t *testing.T) {
	dir := t.TempDir()
	base := writeToolFile(t, filepath.Join(dir, "base.txt"), "one\ntwo\nthree\n")
	local := writeToolFile(t, filepath.Join(dir, "local.txt"), "ONE\ntwo\nthree\n")
	remote := writeToolFile(t, filepath.Join(dir, "remote.txt"), "one\ntwo\nTHREE\n")
	merged := filepath.Join(dir, "merged.txt")

	// Meld's LOCAL BASE REMOTE order must map to the same merge.
	var stdout, stderr bytes.Buffer
	if code := runMergetool([]string{"--order", "local-base-remote", "--output", merged, local, base, remote}, &stdout, &stderr); code != exitOK {
		t.Fatalf("order code=%d stderr=%q", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := runMergetool([]string{base, local, remote}, &stdout, &stderr); code != exitUsage ||
		!strings.Contains(stderr.String(), "requires --output") {
		t.Fatalf("missing output code=%d stderr=%q", code, stderr.String())
	}

	stderr.Reset()
	if code := runMergetool([]string{"--order", "sideways", "--output", merged, base, local, remote}, &stdout, &stderr); code != exitUsage ||
		!strings.Contains(stderr.String(), "--order must be") {
		t.Fatalf("bad order code=%d stderr=%q", code, stderr.String())
	}
}

func TestRunMergetoolGUIBlocksAndCarriesRequest(t *testing.T) {
	dir := t.TempDir()
	base := writeToolFile(t, filepath.Join(dir, "base.txt"), "BASE\n")
	local := writeToolFile(t, filepath.Join(dir, "local.txt"), "LOCAL\n")
	remote := writeToolFile(t, filepath.Join(dir, "remote.txt"), "REMOTE\n")
	output := filepath.Join(dir, "merged.txt")

	// The fake session saves nothing, so the run is treated as aborted.
	var opened string
	code := runMergetoolWithDeps(
		[]string{"--gui", "--label", "B", "--label", "L", "--label", "R", "--output", output, base, local, remote},
		&bytes.Buffer{}, &bytes.Buffer{}, toolTestDeps(&opened),
	)
	if code != exitInterrupt {
		t.Fatalf("aborted GUI code=%d want %d", code, exitInterrupt)
	}
	parsed, err := url.Parse(opened)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	if query.Get("mode") != "threeway" || query.Get("base") != base ||
		query.Get("output") != output || query.Get("baseLabel") != "B" ||
		query.Get("oldLabel") != "L" || query.Get("newLabel") != "R" {
		t.Fatalf("url=%q", opened)
	}
}

// The outcome callback must travel from the GUI session into the exit code, so
// a real save is what makes mergetool return 0.
func TestRunMergetoolGUIUsesReportedOutcome(t *testing.T) {
	dir := t.TempDir()
	base := writeToolFile(t, filepath.Join(dir, "base.txt"), "BASE\n")
	local := writeToolFile(t, filepath.Join(dir, "local.txt"), "LOCAL\n")
	remote := writeToolFile(t, filepath.Join(dir, "remote.txt"), "REMOTE\n")

	for _, tt := range []struct {
		name       string
		unresolved int
		want       int
	}{
		{name: "resolved", unresolved: 0, want: exitOK},
		{name: "markers", unresolved: 1, want: exitDiff},
	} {
		t.Run(tt.name, func(t *testing.T) {
			output := filepath.Join(dir, tt.name+".txt")
			deps := guiCommandDeps{
				newHandler: func(_ string, _ net.Addr, _ bool, lifecycle server.LifecycleOptions) (http.Handler, string, error) {
					if lifecycle.MergeOutcome == nil {
						t.Fatal("mergetool did not pass a merge-outcome callback")
					}
					lifecycle.MergeOutcome(output, tt.unresolved)
					return http.NotFoundHandler(), "token", nil
				},
				listen:      net.Listen,
				serve:       func(net.Listener, http.Handler, <-chan struct{}) error { return nil },
				openBrowser: func(string) error { return nil },
			}
			code := runMergetoolWithDeps(
				[]string{"--gui", "--output", output, base, local, remote},
				&bytes.Buffer{}, &bytes.Buffer{}, deps,
			)
			if code != tt.want {
				t.Fatalf("code=%d want %d", code, tt.want)
			}
		})
	}
}

func TestMergeToolExitCode(t *testing.T) {
	dir := t.TempDir()
	expected := filepath.Join(dir, "merged.txt")
	tests := []struct {
		name       string
		saved      string
		unresolved int
		want       int
	}{
		{name: "resolved save", saved: expected, unresolved: 0, want: exitOK},
		{name: "saved with markers", saved: expected, unresolved: 2, want: exitDiff},
		{name: "saved elsewhere", saved: filepath.Join(dir, "other.txt"), unresolved: 0, want: exitDiff},
		{name: "aborted", saved: "", unresolved: -1, want: exitInterrupt},
	}
	for _, tt := range tests {
		var stderr bytes.Buffer
		if got := mergeToolExitCode(tt.saved, tt.unresolved, expected, &stderr); got != tt.want {
			t.Errorf("%s: mergeToolExitCode = %d, want %d (stderr=%q)", tt.name, got, tt.want, stderr.String())
		}
	}
}

func TestToolLaunchURL(t *testing.T) {
	got := toolLaunchURL("http://127.0.0.1:1/", "tok", toolLaunch{
		mode: "threeway", base: "b.txt", left: "l.txt", right: "r.txt", output: "m.txt",
		baseLabel: "B", leftLabel: "L", rightLabel: "R",
	})
	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	for key, want := range map[string]string{
		"token": "tok", "mode": "threeway", "base": "b.txt", "old": "l.txt", "new": "r.txt",
		"output": "m.txt", "autorun": "1", "baseLabel": "B", "oldLabel": "L", "newLabel": "R",
	} {
		if query.Get(key) != want {
			t.Errorf("query[%s] = %q, want %q", key, query.Get(key), want)
		}
	}
}

// Ensure the new commands are reachable through the dispatcher and print help
// that keeps the LEFT/RIGHT vocabulary.
func TestExternalToolHelp(t *testing.T) {
	for _, command := range []string{"difftool", "mergetool"} {
		var stdout, stderr bytes.Buffer
		if code := run([]string{command, "--help"}, &stdout, &stderr); code != exitOK {
			t.Fatalf("%s help code=%d stderr=%q", command, code, stderr.String())
		}
		output := stdout.String() + stderr.String()
		if strings.Contains(output, "OLD") || strings.Contains(output, "NEW") {
			t.Fatalf("%s help exposes legacy labels:\n%s", command, output)
		}
		want := []string{"LEFT", "RIGHT"}
		if command == "mergetool" {
			want = []string{"BASE", "LOCAL", "REMOTE"}
		}
		for _, term := range want {
			if !strings.Contains(output, term) {
				t.Fatalf("%s help does not name %s:\n%s", command, term, output)
			}
		}
	}
}

// The GUI launcher must still be created with the same lifecycle as `gui`; a
// mutual import check is unnecessary, but guard the deps wiring here.
func TestDefaultToolGUIDepsUsesServerHandler(t *testing.T) {
	deps := defaultToolGUIDeps()
	if deps.newHandler == nil || deps.listen == nil || deps.serve == nil || deps.openBrowser == nil {
		t.Fatalf("incomplete tool deps: %#v", deps)
	}
}
