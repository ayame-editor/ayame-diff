package merge

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ayame-editor/ayame-diff/internal/linediff"
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
	choices := make(map[int][]Side)
	for i := range diff.Hunks {
		choices[i] = []Side{Right}
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
	if _, err := WriteText(old, new, diff, TextOptions{Output: input, OldPath: input, Choices: map[int][]Side{0: {Right}}}); err == nil {
		t.Fatal("unconfirmed overwrite succeeded")
	}
	got, _ := os.ReadFile(input)
	if string(got) != "untouched" {
		t.Fatalf("input changed after rejected save: %q", got)
	}
}

// TestWriteTextBothSideOrderIsCanonical is the #271 contract: adopting both
// contributions of a hunk concatenates left then right, and the result does not
// depend on the order the caller listed them in.
func TestWriteTextBothSideOrderIsCanonical(t *testing.T) {
	old := linediff.SplitTextLines("same\nold\ntail\n")
	new := linediff.SplitTextLines("same\nnew\ntail\n")
	diff := linediff.Diff(old, new, 10, 10)
	if len(diff.Hunks) != 1 {
		t.Fatalf("hunks=%d", len(diff.Hunks))
	}
	out := filepath.Join(t.TempDir(), "both.txt")
	result, err := WriteText(old, new, diff, TextOptions{
		Output:  out,
		Choices: map[int][]Side{0: {Right, Left}},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(out)
	if string(got) != "same\nold\nnew\ntail\n" {
		t.Fatalf("both merged=%q", got)
	}
	if result.Resolved != 1 || result.Unresolved != 0 {
		t.Fatalf("result=%+v", result)
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
	if result.ConflictsRemaining != 0 || len(result.ImplicitlyResolved) != 1 || result.ImplicitlyResolved[0] != 0 {
		t.Fatalf("implicit provenance missing: %+v", result)
	}
}

// TestWriteTextUnresolvedTarget selects the implicit target instead of always
// keeping the left side, and reports the hunks it decided implicitly so the
// saved file stays traceable (#272).
func TestWriteTextUnresolvedTarget(t *testing.T) {
	old := linediff.SplitTextLines("same\nleft\n")
	new := linediff.SplitTextLines("same\nright\n")
	diff := linediff.Diff(old, new, 10, 10)
	if len(diff.Hunks) != 1 {
		t.Fatalf("want one hunk, got %d", len(diff.Hunks))
	}
	cases := []struct {
		target string
		want   string
		remain int
	}{
		{UnresolvedRight, "same\nright\n", 0},
		{UnresolvedMarkers, "same\n<<<<<<< LEFT\nleft\n=======\nright\n>>>>>>> RIGHT\n", 1},
	}
	for _, tc := range cases {
		out := filepath.Join(t.TempDir(), "merged.txt")
		result, err := WriteText(old, new, diff, TextOptions{Output: out, AllowUnresolved: true, UnresolvedTarget: tc.target})
		if err != nil {
			t.Fatalf("%s: %v", tc.target, err)
		}
		got, _ := os.ReadFile(out)
		if string(got) != tc.want {
			t.Fatalf("%s: got=%q want=%q", tc.target, got, tc.want)
		}
		if result.Unresolved != 1 || len(result.ImplicitlyResolved) != 1 || result.ImplicitlyResolved[0] != 0 {
			t.Fatalf("%s: result=%+v", tc.target, result)
		}
		if result.ConflictsRemaining != tc.remain {
			t.Fatalf("%s: conflictsRemaining=%d want=%d", tc.target, result.ConflictsRemaining, tc.remain)
		}
	}
}

func TestWriteTextRejectsUnknownUnresolvedTarget(t *testing.T) {
	old := linediff.SplitTextLines("left\n")
	new := linediff.SplitTextLines("right\n")
	diff := linediff.Diff(old, new, 10, 10)
	_, err := WriteText(old, new, diff, TextOptions{Output: filepath.Join(t.TempDir(), "o.txt"), AllowUnresolved: true, UnresolvedTarget: "middle"})
	if err == nil {
		t.Fatal("unknown unresolved target accepted")
	}
}
