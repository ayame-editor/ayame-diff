package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ayame-editor/ayame-diff/internal/diffout"
)

func TestSubcommandDispatch(t *testing.T) {
	t.Parallel()
	cases := []struct {
		args []string
		want string
	}{
		{nil, ""},
		{[]string{}, ""},
		{[]string{"text", "a", "b"}, "text"},
		{[]string{"sorted", "a", "b"}, "sorted"},
		{[]string{"dir", "a", "b"}, "dir"},
		{[]string{"bin", "a", "b"}, "bin"},
		{[]string{"serve"}, "serve"},
		{[]string{"gui"}, "gui"},
		{[]string{"update", "--check"}, "update"},
		{[]string{"remove"}, "remove"},
		{[]string{"csv", "--left", "a"}, "csv"},
		{[]string{"--left", "a.csv"}, ""}, // bare flags => CSV back-compat
		{[]string{"--version"}, ""},       // handled before dispatch
		{[]string{"unknown", "x"}, ""},    // not a subcommand => CSV
	}
	for _, c := range cases {
		if got := subcommand(c.args); got != c.want {
			t.Fatalf("subcommand(%v) = %q, want %q", c.args, got, c.want)
		}
	}
}

func TestRunTextUnifiedPatch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "old.txt")
	newPath := filepath.Join(dir, "new.txt")
	if err := os.WriteFile(oldPath, []byte("a\r\nold"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newPath, []byte("a\r\nnew"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := runText([]string{"--format", "unified", "-U", "1", oldPath, newPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d, stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "--- "+oldPath+"\t") || !strings.Contains(stdout.String(), "@@ -1,2 +1,2 @@") {
		t.Fatalf("patch output:\n%s", stdout.String())
	}
	if strings.Count(stdout.String(), `\ No newline at end of file`) != 2 {
		t.Fatalf("missing newline markers:\n%s", stdout.String())
	}
}

func TestRunSortedRejectsPatchOutput(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	if code := runSorted([]string{"--format", "unified", "old", "new"}, &stdout, &stderr); code != 2 {
		t.Fatalf("code = %d", code)
	}
	if !strings.Contains(stderr.String(), "require text mode") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunTextDetectMoves(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "old.txt")
	newPath := filepath.Join(dir, "new.txt")
	oldText := "top\nmove-a\nmove-b\nstay-a\nstay-b\nstay-c\nbottom\n"
	newText := "top\nstay-a\nstay-b\nstay-c\nmove-a\nmove-b\nbottom\n"
	if err := os.WriteFile(oldPath, []byte(oldText), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newPath, []byte(newText), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := runText([]string{"--detect-moves", oldPath, newPath}, &stdout, &stderr); code != 0 {
		t.Fatalf("code = %d stderr=%q", code, stderr.String())
	}
	if strings.Count(stdout.String(), "MOVED #1") != 2 || !strings.Contains(stderr.String(), "1 moved block(s) / 2 line(s)") {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestDiffFlagsFormat(t *testing.T) {
	t.Parallel()
	cases := []struct {
		d    diffFlags
		want diffout.Format
	}{
		{diffFlags{}, diffout.Unified},
		{diffFlags{side: true}, diffout.SideBySide},
		{diffFlags{summary: true}, diffout.Summary},
		{diffFlags{json: true}, diffout.JSON},
		{diffFlags{json: true, side: true}, diffout.JSON}, // json wins
		{diffFlags{summary: true, side: true}, diffout.Summary},
	}
	for _, c := range cases {
		if got := c.d.format(); got != c.want {
			t.Fatalf("format(%+v) = %v, want %v", c.d, got, c.want)
		}
	}
}

func TestPatchFormatFlags(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		d    diffFlags
		want diffout.Format
		ctx  int
	}{
		{name: "normal alias", d: diffFlags{normal: true}, want: diffout.Normal},
		{name: "normal format", d: diffFlags{patchFormat: "normal"}, want: diffout.Normal},
		{name: "context", d: diffFlags{patchFormat: "context", contextLines: 5}, want: diffout.PatchContext, ctx: 5},
		{name: "unified U0", d: diffFlags{unifiedContext: optionalInt{value: 0, set: true}}, want: diffout.PatchUnified},
		{name: "context C2", d: diffFlags{contextContext: optionalInt{value: 2, set: true}}, want: diffout.PatchContext, ctx: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ctx, patch, err := tc.d.outputFormat()
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want || ctx != tc.ctx || !patch {
				t.Fatalf("format/context/patch = %v/%d/%v, want %v/%d/true", got, ctx, patch, tc.want, tc.ctx)
			}
		})
	}
}

func TestSyncFlagUsesOneBasedLines(t *testing.T) {
	t.Parallel()
	var points syncFlag
	if err := points.Set("100:120"); err != nil {
		t.Fatal(err)
	}
	if len(points) != 1 || points[0].Old != 99 || points[0].New != 119 || points.String() != "100:120" {
		t.Fatalf("points = %+v (%s)", points, points.String())
	}
	for _, invalid := range []string{"0:1", "1:0", "a:2", "1", "1:2:3"} {
		if err := points.Set(invalid); err == nil {
			t.Fatalf("accepted invalid sync %q", invalid)
		}
	}
}

func TestRunTextIgnoreEOLAndLineFilter(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	oldPath, newPath := filepath.Join(dir, "old.txt"), filepath.Join(dir, "new.txt")
	if err := os.WriteFile(oldPath, []byte("timestamp=1 ok\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newPath, []byte("timestamp=2 ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := runText([]string{"--summary", "--ignore-eol", "--filter-line", `timestamp=\d+`, oldPath, newPath}, &stdout, &stderr)
	if code != 0 || !strings.Contains(stderr.String(), "0 hunk") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestDiffFlagsRejectConflictingWhitespaceAliases(t *testing.T) {
	t.Parallel()
	if _, err := (diffFlags{ignoreAllSpace: true, ignoreSpaceChange: true}).comparisonOptions(); err == nil {
		t.Fatal("conflicting whitespace aliases succeeded")
	}
}
