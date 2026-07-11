package diffout

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/hjosugi/ayame-diff/internal/linediff"
)

func renderPatch(t *testing.T, oldText, newText string, format Format, context int, oldLabel, newLabel string) []byte {
	t.Helper()
	old := linediff.SplitTextLines(oldText)
	new := linediff.SplitTextLines(newText)
	res := linediff.Diff(old, new, 1<<20, 128)
	var output bytes.Buffer
	if err := Write(&output, &bytes.Buffer{}, old, new, res, Options{
		Format: format, Context: context, ContextSet: true,
		OldLabel: oldLabel, NewLabel: newLabel,
	}); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func TestUnifiedPatchSyntax(t *testing.T) {
	t.Parallel()
	got := string(renderPatch(t, "a\nb\nc\nd\n", "a\nX\nc\nd\ne\n", PatchUnified, 3, "a/file.txt", "b/file.txt"))
	want := "--- a/file.txt\n+++ b/file.txt\n@@ -1,4 +1,5 @@\n a\n-b\n+X\n c\n d\n+e\n"
	if got != want {
		t.Fatalf("unified patch:\n%q\nwant:\n%q", got, want)
	}
}

func TestContextPatchSyntax(t *testing.T) {
	t.Parallel()
	got := string(renderPatch(t, "a\nb\nc\nd\n", "a\nX\nc\nd\ne\n", PatchContext, 3, "old.txt", "new.txt"))
	want := "*** old.txt\n--- new.txt\n***************\n*** 1,4 ****\n  a\n! b\n  c\n  d\n--- 1,5 ----\n  a\n! X\n  c\n  d\n+ e\n"
	if got != want {
		t.Fatalf("context patch:\n%q\nwant:\n%q", got, want)
	}
}

func TestPatchMarksMissingFinalNewlineAndRejectsBinary(t *testing.T) {
	t.Parallel()
	patch := string(renderPatch(t, "a\nold", "a\nnew", PatchUnified, 1, "old", "new"))
	if got := strings.Count(patch, `\ No newline at end of file`); got != 2 {
		t.Fatalf("newline markers = %d, patch:\n%s", got, patch)
	}
	old := linediff.SplitTextLines("a\x00b\n")
	new := linediff.SplitTextLines("a\x00c\n")
	res := linediff.Diff(old, new, 100, 128)
	if err := Write(&bytes.Buffer{}, &bytes.Buffer{}, old, new, res, Options{Format: PatchUnified}); err == nil || !strings.Contains(err.Error(), "appears binary") {
		t.Fatalf("binary patch error = %v", err)
	}
}

func TestPatchFormatsApplyWithGNUAndGit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("external compatibility runs on Ubuntu; Windows patch.exe and Git rewrite line endings")
	}
	patchCommand, err := exec.LookPath("patch")
	if err != nil {
		t.Skip("GNU patch is not installed")
	}
	gitCommand, gitErr := exec.LookPath("git")
	fixtures := []struct {
		name, old, new string
	}{
		{name: "lf", old: "a\nb\nc\n", new: "a\nB\nc\nd\n"},
		{name: "crlf", old: "a\r\nb\r\nc\r\n", new: "a\r\nB\r\nc\r\nd\r\n"},
		{name: "no-final-newline", old: "a\nold", new: "a\nnew"},
	}
	formats := []struct {
		name   string
		format Format
	}{{"normal", Normal}, {"context", PatchContext}, {"unified", PatchUnified}}
	for _, fixture := range fixtures {
		for _, format := range formats {
			t.Run(fixture.name+"/gnu-"+format.name, func(t *testing.T) {
				dir := t.TempDir()
				target := filepath.Join(dir, "target.txt")
				patchPath := filepath.Join(dir, "change.patch")
				if err := os.WriteFile(target, []byte(fixture.old), 0o644); err != nil {
					t.Fatal(err)
				}
				data := renderPatch(t, fixture.old, fixture.new, format.format, 3, "old.txt", "new.txt")
				if err := os.WriteFile(patchPath, data, 0o644); err != nil {
					t.Fatal(err)
				}
				if output, err := exec.Command(patchCommand, target, patchPath).CombinedOutput(); err != nil {
					t.Fatalf("patch failed: %v\n%s\npatch:\n%s", err, output, data)
				}
				got, err := os.ReadFile(target)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(got, []byte(fixture.new)) {
					t.Fatalf("applied bytes = %q, want %q", got, fixture.new)
				}
			})
		}
		if gitErr == nil {
			t.Run(fixture.name+"/git-unified", func(t *testing.T) {
				dir := t.TempDir()
				target := filepath.Join(dir, "sample.txt")
				patchPath := filepath.Join(dir, "change.patch")
				if output, err := exec.Command(gitCommand, "-C", dir, "init", "-q").CombinedOutput(); err != nil {
					t.Fatalf("git init: %v: %s", err, output)
				}
				if err := os.WriteFile(target, []byte(fixture.old), 0o644); err != nil {
					t.Fatal(err)
				}
				data := renderPatch(t, fixture.old, fixture.new, PatchUnified, 3, "a/sample.txt", "b/sample.txt")
				if err := os.WriteFile(patchPath, data, 0o644); err != nil {
					t.Fatal(err)
				}
				if output, err := exec.Command(gitCommand, "-C", dir, "apply", "--whitespace=nowarn", patchPath).CombinedOutput(); err != nil {
					t.Fatalf("git apply failed: %v\n%s\npatch:\n%s", err, output, data)
				}
				got, err := os.ReadFile(target)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(got, []byte(fixture.new)) {
					t.Fatalf("git-applied bytes = %q, want %q", got, fixture.new)
				}
			})
		}
	}
}
