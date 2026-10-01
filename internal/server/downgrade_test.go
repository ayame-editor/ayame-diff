package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestDiffResponseDowngradesWhitespaceOnlyDifferences covers #269 on the display
// path: a whitespace-only pair stays visible as a downgraded hunk but is not a
// difference for the counts.
func TestDiffResponseDowngradesWhitespaceOnlyDifferences(t *testing.T) {
	t.Parallel()
	_, resp := postDiff(t, newTestServer(t), diffRequest{
		Inline: true, Mode: "text", Whitespace: "change",
		OldText: "same\nalpha\nkeep\n", NewText: "same\nalpha \nkeep\n",
	})
	if resp.HunkCount != 0 || resp.Modified != 0 || resp.Added != 0 || resp.Deleted != 0 {
		t.Fatalf("dismissed difference leaked into the counts: %+v", resp)
	}
	if resp.DowngradedHunks != 1 || len(resp.Hunks) != 1 || !resp.Hunks[0].Downgraded {
		t.Fatalf("downgraded response = %+v hunks=%+v", resp, resp.Hunks)
	}
	if resp.Hunks[0].Old[0] != "alpha" || resp.Hunks[0].New[0] != "alpha " {
		t.Fatalf("downgraded hunk lost its original text: %+v", resp.Hunks[0])
	}
}

// TestDiffResponseDowngradesCaseOnlyDifferences mirrors the whitespace case for
// ignore-case, which the issue asks to consider.
func TestDiffResponseDowngradesCaseOnlyDifferences(t *testing.T) {
	t.Parallel()
	_, resp := postDiff(t, newTestServer(t), diffRequest{
		Inline: true, Mode: "text", IgnoreCase: true,
		OldText: "Hello\n", NewText: "hello\n",
	})
	if resp.HunkCount != 0 || resp.DowngradedHunks != 1 || len(resp.Hunks) != 1 || !resp.Hunks[0].Downgraded {
		t.Fatalf("case-only response = %+v hunks=%+v", resp, resp.Hunks)
	}
}

// TestDiffResponseKeepsRealAndDowngradedSeparate checks the two counts do not
// bleed into each other when a comparison has both kinds of difference.
func TestDiffResponseKeepsRealAndDowngradedSeparate(t *testing.T) {
	t.Parallel()
	_, resp := postDiff(t, newTestServer(t), diffRequest{
		Inline: true, Mode: "text", Whitespace: "all",
		OldText: "a b\nreal old\n", NewText: "ab\nreal new\n",
	})
	if resp.HunkCount != 1 || resp.Modified != 1 || resp.DowngradedHunks != 1 || len(resp.Hunks) != 2 {
		t.Fatalf("mixed response = %+v hunks=%+v", resp, resp.Hunks)
	}
	if !resp.Hunks[0].Downgraded || resp.Hunks[1].Downgraded {
		t.Fatalf("hunk order/flags = %+v", resp.Hunks)
	}
}

// TestDiffResponseWithoutIgnoreOptionsHasNoDowngradedHunks guards that the new
// state only appears when something was actually ignored.
func TestDiffResponseWithoutIgnoreOptionsHasNoDowngradedHunks(t *testing.T) {
	t.Parallel()
	_, resp := postDiff(t, newTestServer(t), diffRequest{
		Inline: true, Mode: "text",
		OldText: "alpha\n", NewText: "alpha \n",
	})
	if resp.DowngradedHunks != 0 || resp.HunkCount != 1 {
		t.Fatalf("exact response = %+v", resp)
	}
	for _, hunk := range resp.Hunks {
		if hunk.Downgraded {
			t.Fatalf("exact comparison marked a hunk downgraded: %+v", hunk)
		}
	}
}

// TestPatchExportIgnoresDowngradedDifferences guards that the third state stays
// a display affordance: an applyable patch must not carry a dismissed change.
func TestPatchExportIgnoresDowngradedDifferences(t *testing.T) {
	t.Parallel()
	body, err := json.Marshal(diffRequest{
		Inline: true, Mode: "text", Whitespace: "change",
		OldText: "a\n", NewText: "a \n", PatchFormat: "unified",
	})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	newTestServer(t).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/patch", bytes.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("patch status = %d body=%s", rec.Code, rec.Body.String())
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("patch contains a dismissed whitespace difference: %q", rec.Body.String())
	}
}
