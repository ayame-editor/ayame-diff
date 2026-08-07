package diffout

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ayame-editor/ayame-diff/internal/linediff"
	"github.com/ayame-editor/ayame-diff/internal/textwidth"
)

// run renders old/new with opts and returns the stdout (w) and stderr
// (summaryW) buffers as strings.
func run(t *testing.T, oldText, newText string, opts Options) (out, summary string) {
	t.Helper()
	old := linediff.SplitLines(oldText)
	nw := linediff.SplitLines(newText)
	res := linediff.Diff(old, nw, 200, 128)
	var w, sw bytes.Buffer
	if err := Write(&w, &sw, old, nw, res, opts); err != nil {
		t.Fatalf("Write: %v", err)
	}
	return w.String(), sw.String()
}

func TestUnified(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		oldText     string
		newText     string
		wantOut     string
		wantSummary string
	}{
		{
			name:    "replace",
			oldText: "a\nb\nc\n",
			newText: "a\nX\nc\n",
			// One 1:1 Replace hunk at line 2.
			wantOut:     "@@ -2,1 +2,1 Replace @@\n-b\n+X\n",
			wantSummary: "1 hunk(s), 0 added, 0 deleted, 1 modified\n",
		},
		{
			name:        "insert",
			oldText:     "a\nb\n",
			newText:     "a\nx\ny\nb\n",
			wantOut:     "@@ -2,0 +2,2 Insert @@\n+x\n+y\n",
			wantSummary: "1 hunk(s), 2 added, 0 deleted, 0 modified\n",
		},
		{
			name:        "delete",
			oldText:     "a\nx\ny\nb\n",
			newText:     "a\nb\n",
			wantOut:     "@@ -2,2 +2,0 Delete @@\n-x\n-y\n",
			wantSummary: "1 hunk(s), 0 added, 2 deleted, 0 modified\n",
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			out, summary := run(t, c.oldText, c.newText, Options{Format: Unified})
			if out != c.wantOut {
				t.Errorf("out =\n%q\nwant\n%q", out, c.wantOut)
			}
			if summary != c.wantSummary {
				t.Errorf("summary = %q, want %q", summary, c.wantSummary)
			}
		})
	}
}

func TestUnifiedDefaultsZeroOptions(t *testing.T) {
	t.Parallel()
	// A zero Options must render a full unified diff (Format defaults to Unified,
	// MaxLines to 200) — i.e. behave like the explicit case above.
	out, summary := run(t, "a\nb\nc\n", "a\nX\nc\n", Options{})
	if out != "@@ -2,1 +2,1 Replace @@\n-b\n+X\n" {
		t.Errorf("out = %q", out)
	}
	if summary != "1 hunk(s), 0 added, 0 deleted, 1 modified\n" {
		t.Errorf("summary = %q", summary)
	}
}

func TestMaxLinesTruncation(t *testing.T) {
	t.Parallel()
	// A 5-line deletion capped at 2 shows 2 lines plus a "more line(s)" note.
	out, summary := run(t, "l1\nl2\nl3\nl4\nl5\n", "", Options{Format: Unified, MaxLines: 2})
	want := "@@ -1,5 +1,0 Delete @@\n-l1\n-l2\n-... 3 more line(s)\n"
	if out != want {
		t.Errorf("out =\n%q\nwant\n%q", out, want)
	}
	if summary != "1 hunk(s), 0 added, 5 deleted, 0 modified\n" {
		t.Errorf("summary = %q", summary)
	}
}

func TestSideBySideCJK(t *testing.T) {
	t.Parallel()
	// Width 60 -> column = (60-7)/2 = 26. A short CJK left line needs no
	// truncation; PadRight fills to 26 display cells so " | " aligns.
	old := linediff.SplitLines("日本語\n")
	nw := linediff.SplitLines("hello\n")
	res := linediff.Diff(old, nw, 200, 128)
	var w, sw bytes.Buffer
	if err := Write(&w, &sw, old, nw, res, Options{Format: SideBySide, Width: 60}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	// "日本語" is 6 display cells, so 20 spaces pad it to the 26-cell column.
	want := "@@ -1,1 +1,1 Replace @@\n" +
		"- 日本語" + strings.Repeat(" ", 20) + " | + hello\n"
	if w.String() != want {
		t.Errorf("out =\n%q\nwant\n%q", w.String(), want)
	}
	if sw.String() != "1 hunk(s), 0 added, 0 deleted, 1 modified\n" {
		t.Errorf("summary = %q", sw.String())
	}
}

func TestSideBySideTruncationAndWidth(t *testing.T) {
	t.Parallel()
	// A wide CJK line must be truncated to exactly the column display width
	// (not char count), proving textwidth is used. Width 60 -> column 26.
	long := strings.Repeat("あ", 40) // 80 display cells
	old := linediff.SplitLines(long + "\n")
	nw := linediff.SplitLines("x\n")
	res := linediff.Diff(old, nw, 200, 128)
	var w, sw bytes.Buffer
	if err := Write(&w, &sw, old, nw, res, Options{Format: SideBySide, Width: 60}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	lines := strings.Split(strings.TrimRight(w.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines: %q", len(lines), w.String())
	}
	// lines[1] = "- " + leftCol(26 cells) + " | + x"
	body := lines[1]
	const prefix = "- "
	const sep = " | "
	if !strings.HasPrefix(body, prefix) {
		t.Fatalf("missing left tag: %q", body)
	}
	idx := strings.Index(body, sep)
	if idx < 0 {
		t.Fatalf("missing separator: %q", body)
	}
	leftCol := body[len(prefix):idx]
	if got := textwidth.DisplayWidth(leftCol); got != 26 {
		t.Errorf("left column display width = %d, want 26 (%q)", got, leftCol)
	}
	// The column is padded to full width, so the ellipsis is the last
	// non-space content, not the literal suffix.
	if !strings.HasSuffix(strings.TrimRight(leftCol, " "), "...") {
		t.Errorf("truncated column should end with ellipsis: %q", leftCol)
	}
	if right := body[idx+len(sep):]; right != "+ x" {
		t.Errorf("right side = %q, want %q", right, "+ x")
	}
	_ = sw
}

func TestSideBySideUnevenAndMorePaired(t *testing.T) {
	t.Parallel()
	// A pure insert (OldLen 0, NewLen 3) capped at 2 lines: absent old side gets
	// a blank tag and an all-space column, and a "more paired line(s)" note
	// closes the hunk.
	old := linediff.SplitLines("")
	nw := linediff.SplitLines("x\ny\nz\n")
	res := linediff.Diff(old, nw, 200, 128)
	var w, sw bytes.Buffer
	if err := Write(&w, &sw, old, nw, res, Options{Format: SideBySide, Width: 60, MaxLines: 2}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	blank := strings.Repeat(" ", 26)
	want := "@@ -1,0 +1,3 Insert @@\n" +
		"  " + blank + " | + x\n" +
		"  " + blank + " | + y\n" +
		"... 1 more paired line(s)\n"
	if w.String() != want {
		t.Errorf("out =\n%q\nwant\n%q", w.String(), want)
	}
	if sw.String() != "1 hunk(s), 3 added, 0 deleted, 0 modified\n" {
		t.Errorf("summary = %q", sw.String())
	}
}

func TestJSONSchema(t *testing.T) {
	t.Parallel()
	out, summary := run(t, "a\nb\nc\n", "a\nX\nc\n", Options{Format: JSON})
	want := `{
  "old_lines": 3,
  "new_lines": 3,
  "hunks": [
    {
      "kind": "replace",
      "old_start": 1,
      "old_len": 1,
      "new_start": 1,
      "new_len": 1
    }
  ],
  "hunk_count": 1,
  "omitted_hunks": 0,
  "added": 0,
  "deleted": 0,
  "modified": 1
}
`
	if out != want {
		t.Errorf("json =\n%q\nwant\n%q", out, want)
	}
	// JSON mode must not touch the summary stream.
	if summary != "" {
		t.Errorf("summary = %q, want empty", summary)
	}
}

func TestJSONEmptyHunks(t *testing.T) {
	t.Parallel()
	// Equal documents: hunks must serialize as [] (not null), and field order
	// must be exactly old_lines, new_lines, hunks, ...
	out, _ := run(t, "a\n", "a\n", Options{Format: JSON})
	if !strings.Contains(out, `"hunks": []`) {
		t.Errorf("empty hunks not rendered as []: %q", out)
	}
	// Field order sanity: old_lines precedes hunks precedes modified.
	oi := strings.Index(out, `"old_lines"`)
	hi := strings.Index(out, `"hunks"`)
	mi := strings.Index(out, `"modified"`)
	if !(oi >= 0 && oi < hi && hi < mi) {
		t.Errorf("unexpected field order in %q", out)
	}
}

func TestSummaryFormat(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		res  linediff.Result
		want string
	}{
		{
			name: "plain",
			res:  linediff.Result{HunkCount: 1, Added: 0, Deleted: 0, Modified: 1},
			want: "1 hunk(s), 0 added, 0 deleted, 1 modified\n",
		},
		{
			name: "omitted suffix",
			res:  linediff.Result{HunkCount: 3, OmittedHunks: 1, Modified: 3},
			want: "3 hunk(s), 0 added, 0 deleted, 3 modified (output truncated; raise --max-hunks)\n",
		},
		{
			name: "thousands separators",
			res:  linediff.Result{HunkCount: 1234, Added: 1000000, Deleted: 12, Modified: 999999},
			want: "1,234 hunk(s), 1,000,000 added, 12 deleted, 999,999 modified\n",
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			var w, sw bytes.Buffer
			// Summary mode ignores the line sources; pass empty ones.
			empty := linediff.SplitLines("")
			if err := Write(&w, &sw, empty, empty, c.res, Options{Format: Summary}); err != nil {
				t.Fatalf("Write: %v", err)
			}
			if w.Len() != 0 {
				t.Errorf("summary mode wrote to w: %q", w.String())
			}
			if sw.String() != c.want {
				t.Errorf("summary = %q, want %q", sw.String(), c.want)
			}
		})
	}
}

func TestOmittedSuffixEndToEnd(t *testing.T) {
	t.Parallel()
	// Drive the omitted path through a real capped diff so the suffix is
	// exercised from Result stats, not a hand-built struct.
	old := linediff.SplitLines("1\na\n2\nb\n3\nc\n")
	nw := linediff.SplitLines("1\nA\n2\nB\n3\nC\n")
	res := linediff.Diff(old, nw, 2, 128) // maxHunks 2 -> 1 omitted of 3
	if res.OmittedHunks == 0 {
		t.Fatal("expected omitted hunks in fixture")
	}
	var w, sw bytes.Buffer
	if err := Write(&w, &sw, old, nw, res, Options{Format: Summary}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if !strings.HasSuffix(strings.TrimRight(sw.String(), "\n"), "(output truncated; raise --max-hunks)") {
		t.Errorf("summary = %q, want omitted suffix", sw.String())
	}
}

func TestGroup(t *testing.T) {
	t.Parallel()
	cases := []struct {
		n    uint64
		want string
	}{
		{0, "0"},
		{7, "7"},
		{100, "100"},
		{1000, "1,000"},
		{1234, "1,234"},
		{12345, "12,345"},
		{999999, "999,999"},
		{1000000, "1,000,000"},
		{1234567890, "1,234,567,890"},
	}
	for _, c := range cases {
		if got := group(c.n); got != c.want {
			t.Errorf("group(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}

func TestKindTitle(t *testing.T) {
	t.Parallel()
	if kindTitle(linediff.Insert) != "Insert" ||
		kindTitle(linediff.Delete) != "Delete" ||
		kindTitle(linediff.Replace) != "Replace" {
		t.Fatalf("kindTitle mismatch: %s %s %s",
			kindTitle(linediff.Insert), kindTitle(linediff.Delete), kindTitle(linediff.Replace))
	}
}
