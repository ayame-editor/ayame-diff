package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

// postDiff marshals req, POSTs it to /api/diff on h, and (on 200) decodes the
// response. It returns the recorder so callers can also assert on status.
func postDiff(t *testing.T, h http.Handler, req diffRequest) (*httptest.ResponseRecorder, diffResponse) {
	t.Helper()
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/diff", bytes.NewReader(body)))
	var resp diffResponse
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response: %v (body=%s)", err, rec.Body.String())
		}
	}
	return rec, resp
}

// writeFile writes data to dir/name and returns the full path.
func writeFile(t *testing.T, dir, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// toShiftJIS encodes a UTF-8 string to Shift_JIS bytes.
func toShiftJIS(t *testing.T, s string) []byte {
	t.Helper()
	out, _, err := transform.Bytes(japanese.ShiftJIS.NewEncoder(), []byte(s))
	if err != nil {
		t.Fatalf("shift_jis encode: %v", err)
	}
	return out
}

// TestDiffSortedMode covers mode "sorted": a permutation compares equal, and
// the numeric/reverse request fields change the sort order (and thus where the
// inserted line lands).
func TestDiffSortedMode(t *testing.T) {
	t.Parallel()
	h := newTestServer(t)

	t.Run("permutation_is_zero_hunks", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		oldP := writeFile(t, dir, "old.txt", []byte("banana\napple\ncherry\n"))
		newP := writeFile(t, dir, "new.txt", []byte("cherry\nbanana\napple\n"))

		rec, resp := postDiff(t, h, diffRequest{Old: oldP, New: newP, Mode: "sorted"})
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
		}
		if resp.HunkCount != 0 || len(resp.Hunks) != 0 {
			t.Fatalf("permutation should be equal: hunk_count=%d hunks=%d", resp.HunkCount, len(resp.Hunks))
		}
		if resp.Added != 0 || resp.Deleted != 0 || resp.Modified != 0 {
			t.Fatalf("stats not all zero: +%d -%d ~%d", resp.Added, resp.Deleted, resp.Modified)
		}
	})

	t.Run("numeric_changes_insert_position", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		// The inserted value "9" lands after "2" numerically but at the end
		// lexically (where "9" > "10").
		oldP := writeFile(t, dir, "old.txt", []byte("2\n10\n"))
		newP := writeFile(t, dir, "new.txt", []byte("2\n9\n10\n"))

		// Lexical (numeric=false): sorted old=[10,2], new=[10,2,9]; "9" appended.
		_, lex := postDiff(t, h, diffRequest{Old: oldP, New: newP, Mode: "sorted"})
		if lex.HunkCount != 1 || len(lex.Hunks) != 1 {
			t.Fatalf("lexical: want 1 hunk, got count=%d len=%d", lex.HunkCount, len(lex.Hunks))
		}
		if hk := lex.Hunks[0]; hk.Kind != "insert" || hk.NewStart != 2 ||
			len(hk.New) != 1 || hk.New[0] != "9" {
			t.Fatalf("lexical insert not at end: %+v", hk)
		}

		// Numeric: sorted old=[2,10], new=[2,9,10]; "9" inserted in the middle.
		_, num := postDiff(t, h, diffRequest{Old: oldP, New: newP, Mode: "sorted", Numeric: true})
		if num.HunkCount != 1 || len(num.Hunks) != 1 {
			t.Fatalf("numeric: want 1 hunk, got count=%d len=%d", num.HunkCount, len(num.Hunks))
		}
		if hk := num.Hunks[0]; hk.Kind != "insert" || hk.NewStart != 1 ||
			len(hk.New) != 1 || hk.New[0] != "9" {
			t.Fatalf("numeric insert not in middle: %+v", hk)
		}
	})

	t.Run("reverse_moves_insert_to_front", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		oldP := writeFile(t, dir, "old.txt", []byte("1\n2\n3\n"))
		newP := writeFile(t, dir, "new.txt", []byte("1\n2\n3\n4\n"))

		// Ascending: "4" is appended (new_start=3).
		_, asc := postDiff(t, h, diffRequest{Old: oldP, New: newP, Mode: "sorted", Numeric: true})
		if asc.HunkCount != 1 || asc.Hunks[0].NewStart != 3 || asc.Hunks[0].New[0] != "4" {
			t.Fatalf("ascending insert not at end: %+v", asc.Hunks)
		}

		// Reverse: sorted old=[3,2,1], new=[4,3,2,1]; "4" is prepended.
		_, rev := postDiff(t, h, diffRequest{Old: oldP, New: newP, Mode: "sorted", Numeric: true, Reverse: true})
		if rev.HunkCount != 1 || rev.Hunks[0].Kind != "insert" ||
			rev.Hunks[0].NewStart != 0 || rev.Hunks[0].New[0] != "4" {
			t.Fatalf("reverse insert not at front: %+v", rev.Hunks)
		}
	})
}

// TestDiffEncoding writes Shift_JIS files and verifies both "auto" detection and
// an explicit "shift_jis" hint decode the hunk text back to UTF-8 Japanese.
func TestDiffEncoding(t *testing.T) {
	t.Parallel()
	h := newTestServer(t)

	// A shared first line drives Shift_JIS auto-detection; only the last line
	// differs, so the replace hunk carries decoded Japanese text.
	const oldUTF8 = "日本語のテスト\nABC 123\n漢字とかな\n"
	const newUTF8 = "日本語のテスト\nABC 123\nかなと漢字\n"

	dir := t.TempDir()
	oldP := writeFile(t, dir, "old.txt", toShiftJIS(t, oldUTF8))
	newP := writeFile(t, dir, "new.txt", toShiftJIS(t, newUTF8))

	for _, enc := range []string{"auto", "shift_jis"} {
		enc := enc
		t.Run(enc, func(t *testing.T) {
			t.Parallel()
			rec, resp := postDiff(t, h, diffRequest{Old: oldP, New: newP, Mode: "text", Encoding: enc})
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
			}
			if resp.OldLines != 3 || resp.NewLines != 3 {
				t.Fatalf("lines = %d/%d", resp.OldLines, resp.NewLines)
			}
			if len(resp.Hunks) != 1 {
				t.Fatalf("want 1 hunk, got %d: %+v", len(resp.Hunks), resp.Hunks)
			}
			hk := resp.Hunks[0]
			if hk.Kind != "replace" || len(hk.Old) != 1 || len(hk.New) != 1 {
				t.Fatalf("unexpected hunk: %+v", hk)
			}
			// The decoded UTF-8 Japanese must round-trip through the API.
			if hk.Old[0] != "漢字とかな" || hk.New[0] != "かなと漢字" {
				t.Fatalf("decoded text wrong: old=%q new=%q", hk.Old[0], hk.New[0])
			}
		})
	}
}

// TestDiffIgnoreOptions verifies that case-only and whitespace-only differences
// collapse to zero hunks once the corresponding request option is set, while the
// same inputs differ without it.
func TestDiffIgnoreOptions(t *testing.T) {
	t.Parallel()
	h := newTestServer(t)

	cases := []struct {
		name    string
		old     string
		new     string
		req     diffRequest // Old/New filled in below
		baseReq diffRequest // control: same files, no ignore option
	}{
		{
			name:    "ignore_case",
			old:     "Apple\nBanana\n",
			new:     "apple\nBANANA\n",
			req:     diffRequest{Mode: "text", IgnoreCase: true},
			baseReq: diffRequest{Mode: "text"},
		},
		{
			name:    "whitespace_all",
			old:     "a b c\n",
			new:     "abc\n",
			req:     diffRequest{Mode: "text", Whitespace: "all"},
			baseReq: diffRequest{Mode: "text"},
		},
		{
			name:    "whitespace_change",
			old:     "  a   b  \n",
			new:     "a b\n",
			req:     diffRequest{Mode: "text", Whitespace: "change"},
			baseReq: diffRequest{Mode: "text"},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			oldP := writeFile(t, dir, "old.txt", []byte(c.old))
			newP := writeFile(t, dir, "new.txt", []byte(c.new))

			// Control: without the ignore option the inputs differ.
			base := c.baseReq
			base.Old, base.New = oldP, newP
			if _, resp := postDiff(t, h, base); resp.HunkCount == 0 {
				t.Fatalf("control expected differences, got 0 hunks")
			}

			// With the ignore option the difference collapses away.
			req := c.req
			req.Old, req.New = oldP, newP
			rec, resp := postDiff(t, h, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
			}
			if resp.HunkCount != 0 || len(resp.Hunks) != 0 {
				t.Fatalf("expected 0 hunks with %s, got count=%d len=%d",
					c.name, resp.HunkCount, len(resp.Hunks))
			}
		})
	}
}

func TestDiffAPIEOLAndRegexFilters(t *testing.T) {
	s := newTestServer(t)
	request := func(body diffRequest) diffResponse {
		data, _ := json.Marshal(body)
		rec := httptest.NewRecorder()
		s.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/diff", bytes.NewReader(data)))
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		var result diffResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	base := diffRequest{Inline: true, OldText: "time=1\r\nvalue\r\n", NewText: "time=2\nvalue\n", MaxHunks: 10}
	if got := request(base); got.Modified != 2 {
		t.Fatalf("base=%+v", got)
	}
	base.IgnoreEOL = true
	base.LineFilters = []string{`time=\d+`}
	if got := request(base); got.HunkCount != 0 {
		t.Fatalf("filtered=%+v", got)
	}

	bad, _ := json.Marshal(diffRequest{Inline: true, OldText: "a", NewText: "b", LineFilters: []string{"["}})
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/diff", bytes.NewReader(bad)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid regex status=%d", rec.Code)
	}
}

// TestDiffMaxHunks checks that hunk_count keeps counting every hunk while only
// maxHunks are stored and the remainder is reported as omitted_hunks.
func TestDiffMaxHunks(t *testing.T) {
	t.Parallel()
	h := newTestServer(t)

	dir := t.TempDir()
	// 10 lines that all differ and never resync -> 10 independent 1:1 replaces.
	var oldB, newB bytes.Buffer
	for i := 0; i < 10; i++ {
		oldB.WriteString("old-")
		oldB.WriteByte(byte('0' + i))
		oldB.WriteByte('\n')
		newB.WriteString("new-")
		newB.WriteByte(byte('0' + i))
		newB.WriteByte('\n')
	}
	oldP := writeFile(t, dir, "old.txt", oldB.Bytes())
	newP := writeFile(t, dir, "new.txt", newB.Bytes())

	rec, resp := postDiff(t, h, diffRequest{Old: oldP, New: newP, Mode: "text", MaxHunks: 3})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if resp.HunkCount != 10 {
		t.Fatalf("hunk_count = %d, want 10", resp.HunkCount)
	}
	if len(resp.Hunks) != 3 {
		t.Fatalf("stored hunks = %d, want 3 (capped by maxHunks)", len(resp.Hunks))
	}
	if resp.OmittedHunks != 7 {
		t.Fatalf("omitted_hunks = %d, want 7", resp.OmittedHunks)
	}
}

// TestDiffMaxLines checks that each stored hunk's old/new line arrays are capped
// by maxLines while the hunk's *_len metadata still reports the full extent.
func TestDiffMaxLines(t *testing.T) {
	t.Parallel()
	h := newTestServer(t)

	dir := t.TempDir()

	t.Run("caps_new_on_insert", func(t *testing.T) {
		t.Parallel()
		var newB bytes.Buffer
		newB.WriteString("keep\n")
		for i := 0; i < 10; i++ {
			newB.WriteString("ins-")
			newB.WriteByte(byte('0' + i))
			newB.WriteByte('\n')
		}
		oldP := writeFile(t, dir, "ins-old.txt", []byte("keep\n"))
		newP := writeFile(t, dir, "ins-new.txt", newB.Bytes())

		_, resp := postDiff(t, h, diffRequest{Old: oldP, New: newP, Mode: "text", MaxLines: 4})
		if len(resp.Hunks) != 1 {
			t.Fatalf("want 1 hunk, got %d: %+v", len(resp.Hunks), resp.Hunks)
		}
		hk := resp.Hunks[0]
		if hk.Kind != "insert" || hk.NewLen != 10 {
			t.Fatalf("unexpected hunk metadata: %+v", hk)
		}
		if len(hk.New) != 4 {
			t.Fatalf("new lines = %d, want 4 (capped by maxLines)", len(hk.New))
		}
		if len(hk.Old) != 0 {
			t.Fatalf("old lines = %d, want 0", len(hk.Old))
		}
	})

	t.Run("caps_old_on_delete", func(t *testing.T) {
		t.Parallel()
		var oldB bytes.Buffer
		oldB.WriteString("keep\n")
		for i := 0; i < 10; i++ {
			oldB.WriteString("del-")
			oldB.WriteByte(byte('0' + i))
			oldB.WriteByte('\n')
		}
		oldP := writeFile(t, dir, "del-old.txt", oldB.Bytes())
		newP := writeFile(t, dir, "del-new.txt", []byte("keep\n"))

		_, resp := postDiff(t, h, diffRequest{Old: oldP, New: newP, Mode: "text", MaxLines: 4})
		if len(resp.Hunks) != 1 {
			t.Fatalf("want 1 hunk, got %d: %+v", len(resp.Hunks), resp.Hunks)
		}
		hk := resp.Hunks[0]
		if hk.Kind != "delete" || hk.OldLen != 10 {
			t.Fatalf("unexpected hunk metadata: %+v", hk)
		}
		if len(hk.Old) != 4 {
			t.Fatalf("old lines = %d, want 4 (capped by maxLines)", len(hk.Old))
		}
		if len(hk.New) != 0 {
			t.Fatalf("new lines = %d, want 0", len(hk.New))
		}
	})
}
