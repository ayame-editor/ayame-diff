package merge

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hjosugi/ayame-diff/internal/linediff"
)

func TestWriteTextChoicesAndOriginalsUnchanged(t *testing.T) {
	dir := t.TempDir()
	oldPath, newPath, out := filepath.Join(dir, "old.txt"), filepath.Join(dir, "new.txt"), filepath.Join(dir, "merged.txt")
	oldText, newText := "same\r\nold\r\ntail", "same\r\nnew\r\ninsert\r\ntail"
	if err := os.WriteFile(oldPath, []byte(oldText), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newPath, []byte(newText), 0o644); err != nil {
		t.Fatal(err)
	}
	old, new := linediff.SplitTextLines(oldText), linediff.SplitTextLines(newText)
	diff := linediff.Diff(old, new, 100, 10)
	choices := make(map[int]Side)
	for i := range diff.Hunks {
		choices[i] = Right
	}
	result, err := WriteText(old, new, diff, TextOptions{Output: out, OldPath: oldPath, NewPath: newPath, Choices: choices})
	if err != nil {
		t.Fatal(err)
	}
	if result.Unresolved != 0 || result.Resolved != len(diff.Hunks) {
		t.Fatalf("result=%+v", result)
	}
	merged, err := os.ReadFile(out)
	if err != nil || string(merged) != newText {
		t.Fatalf("merged=%q err=%v", merged, err)
	}
	for path, want := range map[string]string{oldPath: oldText, newPath: newText} {
		got, readErr := os.ReadFile(path)
		if readErr != nil || string(got) != want {
			t.Fatalf("input %s changed: %q err=%v", path, got, readErr)
		}
	}
}

func TestWriteTextRequiresResolutionAndOverwriteConfirmation(t *testing.T) {
	old := linediff.SplitTextLines("old\n")
	new := linediff.SplitTextLines("new\n")
	diff := linediff.Diff(old, new, 10, 10)
	dir := t.TempDir()
	input := filepath.Join(dir, "old.txt")
	if err := os.WriteFile(input, []byte("untouched"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteText(old, new, diff, TextOptions{Output: filepath.Join(dir, "out.txt")}); err == nil {
		t.Fatal("unresolved save succeeded")
	}
	if _, err := WriteText(old, new, diff, TextOptions{Output: input, OldPath: input, Choices: map[int]Side{0: Right}}); err == nil {
		t.Fatal("unconfirmed overwrite succeeded")
	}
	got, _ := os.ReadFile(input)
	if string(got) != "untouched" {
		t.Fatalf("input changed after rejected save: %q", got)
	}
}

func TestWriteTextUnresolvedDefaultsLeftWhenAllowed(t *testing.T) {
	old := linediff.SplitTextLines("left\n")
	new := linediff.SplitTextLines("right\n")
	diff := linediff.Diff(old, new, 10, 10)
	out := filepath.Join(t.TempDir(), "merged.txt")
	result, err := WriteText(old, new, diff, TextOptions{Output: out, AllowUnresolved: true})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(out)
	if string(got) != "left\n" || result.Unresolved != 1 {
		t.Fatalf("got=%q result=%+v", got, result)
	}
}
