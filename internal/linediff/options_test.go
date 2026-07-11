package linediff

import "testing"

func TestDiffWithEOLControls(t *testing.T) {
	lf := SplitTextLines("one\ntwo\n")
	crlf := SplitTextLines("one\r\ntwo\r\n")
	if got := DiffWith(lf, crlf, Options{MaxHunks: 10, Window: 8}); got.Modified != 2 {
		t.Fatalf("EOL-sensitive diff = %+v", got)
	}
	if got := DiffWith(lf, crlf, Options{MaxHunks: 10, Window: 8, IgnoreEOL: true}); got.HunkCount != 0 {
		t.Fatalf("ignore EOL diff = %+v", got)
	}
	missing := SplitTextLines("one\ntwo")
	if got := DiffWith(lf, missing, Options{MaxHunks: 10, Window: 8, IgnoreTrailingEOL: true}); got.HunkCount != 0 {
		t.Fatalf("ignore trailing EOL diff = %+v", got)
	}
	if got := DiffWith(crlf, missing, Options{MaxHunks: 10, Window: 8, IgnoreTrailingEOL: true}); got.Modified != 1 {
		t.Fatalf("non-trailing EOL must remain = %+v", got)
	}
	if got := DiffWith(SplitTextLines("one\r\n"), SplitTextLines("one\n"), Options{MaxHunks: 10, Window: 8, IgnoreTrailingEOL: true}); got.Modified != 1 {
		t.Fatalf("trailing CRLF/LF is not a missing-EOL difference = %+v", got)
	}
}

func TestDiffWithLineFiltersKeepsOriginalPositions(t *testing.T) {
	filters, err := CompileLineFilters([]string{`timestamp=\S+`, `request_id=\d+`})
	if err != nil {
		t.Fatal(err)
	}
	old := StringLines{"timestamp=10:00 request_id=1 ok", "real=old"}
	new := StringLines{"timestamp=10:01 request_id=2 ok", "real=new"}
	got := DiffWith(old, new, Options{MaxHunks: 10, Window: 8, LineFilters: filters})
	if got.Modified != 1 || len(got.Hunks) != 1 || got.Hunks[0].OldStart != 1 {
		t.Fatalf("filtered diff = %+v", got)
	}
	if _, err := CompileLineFilters([]string{"["}); err == nil {
		t.Fatal("invalid regex succeeded")
	}
}

func TestDiffWithIgnoreCase(t *testing.T) {
	t.Parallel()
	old := SplitLines("Hello\nWorld\n")
	new := SplitLines("hello\nWORLD\n")

	// Without ignore-case: two replaces.
	if r := Diff(old, new, 200, 128); r.HunkCount != 2 {
		t.Fatalf("case-sensitive hunks = %d, want 2", r.HunkCount)
	}
	// With ignore-case: no differences.
	r := DiffWith(old, new, Options{MaxHunks: 200, Window: 128, IgnoreCase: true})
	if r.HunkCount != 0 {
		t.Fatalf("ignore-case hunks = %d, want 0 (%+v)", r.HunkCount, r.Hunks)
	}
}

func TestDiffWithWhitespace(t *testing.T) {
	t.Parallel()
	old := SplitLines("a\tb\n  indent\n")
	new := SplitLines("a    b\nindent\n")

	if r := Diff(old, new, 200, 128); r.HunkCount == 0 {
		t.Fatal("exact compare should see whitespace differences")
	}
	// WSChange: collapsed/trimmed whitespace makes them equal.
	if r := DiffWith(old, new, Options{MaxHunks: 200, Window: 128, Whitespace: WSChange}); r.HunkCount != 0 {
		t.Fatalf("WSChange hunks = %d, want 0 (%+v)", r.HunkCount, r.Hunks)
	}
	// WSAll: "a\tb" vs "a b c" differ only if non-space differs.
	oldA := SplitLines("a b c\n")
	newA := SplitLines("abc\n")
	if r := DiffWith(oldA, newA, Options{MaxHunks: 200, Window: 128, Whitespace: WSAll}); r.HunkCount != 0 {
		t.Fatalf("WSAll hunks = %d, want 0", r.HunkCount)
	}
}

func TestDiffWithPreservesOriginalPositions(t *testing.T) {
	t.Parallel()
	// A real change survives ignore-case, and positions/output still reference
	// the originals.
	old := SplitLines("Alpha\nBeta\n")
	new := SplitLines("alpha\nGamma\n")
	r := DiffWith(old, new, Options{MaxHunks: 200, Window: 128, IgnoreCase: true})
	if r.HunkCount != 1 {
		t.Fatalf("hunks = %d, want 1", r.HunkCount)
	}
	h := r.Hunks[0]
	if h.Kind != Replace || h.OldStart != 1 || h.NewStart != 1 {
		t.Fatalf("hunk = %+v", h)
	}
	// The originals are unchanged (comparison normalization does not mutate them).
	if s, _ := old.Line(0); s != "Alpha" {
		t.Fatalf("original mutated: %q", s)
	}
}

func TestCollapseAndRemoveSpace(t *testing.T) {
	t.Parallel()
	if got := collapseSpace("  a\t\t b  c  "); got != "a b c" {
		t.Fatalf("collapseSpace = %q", got)
	}
	if got := removeSpace(" a b\tc\n"); got != "abc" {
		t.Fatalf("removeSpace = %q", got)
	}
}
