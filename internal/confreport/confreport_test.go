package confreport

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/ayame-editor/ayame-diff/internal/linediff"
)

func buildFixture(t *testing.T, oldText, newText string, mutate func(*BuildOptions)) (*Report, linediff.Result, linediff.Lines, linediff.Lines) {
	t.Helper()
	old := linediff.SplitLines(oldText)
	new := linediff.SplitLines(newText)
	res := linediff.Diff(old, new, 200, 128)
	opts := BuildOptions{
		Version:     "9.9.9",
		GeneratedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		Mode:        "text",
		Inputs: []Input{
			{Label: "left", Path: "old.txt", Exists: true, Size: 10, SHA256: "1111"},
			{Label: "right", Path: "new.txt", Exists: true, Size: 11, SHA256: "2222"},
		},
		Conditions: Conditions{Mode: "text", Window: 128, MaxHunks: 200, MaxLines: 200, Whitespace: "none"},
		Old:        old,
		New:        new,
		Result:     res,
		Reproduce:  Reproduce{StateVersion: 1},
	}
	if mutate != nil {
		mutate(&opts)
	}
	return Build(opts), res, old, new
}

func TestBuildReportsContents(t *testing.T) {
	t.Parallel()
	report, result, _, _ := buildFixture(t,
		"one\nkeep\nSECRET_OLD_LINE\nkeep\nthree\n",
		"one\nkeep\nSECRET_NEW_LINE\nkeep\nTHREE\n", nil)
	if len(result.Hunks) != 2 {
		t.Fatalf("fixture wants 2 hunks, got %d", len(result.Hunks))
	}

	if report.Generator != Generator || report.Version != "9.9.9" {
		t.Errorf("generator/version = %q/%q", report.Generator, report.Version)
	}
	if report.GeneratedAt != "2026-01-02T03:04:05Z" {
		t.Errorf("generated_at = %q", report.GeneratedAt)
	}
	if len(report.Inputs) != 2 || report.Inputs[0].Path != "old.txt" || report.Inputs[0].SHA256 != "1111" {
		t.Errorf("inputs = %#v", report.Inputs)
	}
	if report.Conditions.Window != 128 || report.Conditions.MaxHunks != 200 {
		t.Errorf("conditions = %#v", report.Conditions)
	}
	if report.Totals.Modified+report.Totals.Added+report.Totals.Deleted == 0 {
		t.Errorf("totals carry no differences: %#v", report.Totals)
	}
	if report.Review.Available != true || report.Review.UnreadCount != 2 {
		t.Errorf("review = %#v", report.Review)
	}
}

func TestBuildConfirmationIsUnavailableWithoutIssue288(t *testing.T) {
	t.Parallel()
	report, _, _, _ := buildFixture(t, "a\n", "b\n", nil)
	if report.Confirmation.Available {
		t.Fatal("confirmation must be unavailable when the caller passes no confirmed hunks")
	}
	if report.Confirmation.Note != ConfirmationUnavailableNote {
		t.Errorf("note = %q", report.Confirmation.Note)
	}
	if report.Confirmation.ConfirmedCount != 0 || report.Confirmation.UnconfirmedCount != 1 {
		t.Errorf("every retained hunk should stay unconfirmed: %#v", report.Confirmation)
	}
}

func TestBuildListsIgnoredHunksAndShrinksTotals(t *testing.T) {
	t.Parallel()
	report, result, _, _ := buildFixture(t,
		"a\nkeep\nb\n",
		"A\nkeep\nB\n",
		func(opts *BuildOptions) {
			opts.Conditions.IgnoredHunkIndexes = []int{1}
		})
	if len(result.Hunks) < 2 {
		t.Skip("fixture did not produce two hunks")
	}
	if len(report.IgnoredHunks) != 1 {
		t.Fatalf("ignored hunks = %#v", report.IgnoredHunks)
	}
	if report.IgnoredHunks[0].Reason != ReasonIgnored {
		t.Errorf("reason = %q", report.IgnoredHunks[0].Reason)
	}
	if report.IgnoredHunks[0].Index != 1 {
		t.Errorf("index = %d", report.IgnoredHunks[0].Index)
	}
	if report.Totals.IgnoredHunks != 1 || report.Totals.Hunks != len(result.Hunks)-1 {
		t.Errorf("totals = %#v", report.Totals)
	}
	if report.Review.UnreadCount != len(result.Hunks)-1 {
		t.Errorf("unread count should exclude the ignored hunk: %#v", report.Review)
	}
}

func TestBuildReportsConfirmedHunksWhenSupplied(t *testing.T) {
	t.Parallel()
	confirmed := []int{0}
	report, result, _, _ := buildFixture(t, "a\nkeep\nb\n", "A\nkeep\nB\n", func(opts *BuildOptions) {
		opts.ConfirmedHunks = &confirmed
	})
	if !report.Confirmation.Available {
		t.Fatal("confirmation should be available when confirmed hunks are supplied")
	}
	if report.Confirmation.ConfirmedCount != 1 || len(report.Confirmation.Confirmed) != 1 || report.Confirmation.Confirmed[0] != 0 {
		t.Errorf("confirmed = %#v", report.Confirmation)
	}
	if report.Confirmation.UnconfirmedCount != len(result.Hunks)-1 {
		t.Errorf("unconfirmed = %#v", report.Confirmation)
	}
}

func TestBuildDoesNotEmbedContentByDefault(t *testing.T) {
	t.Parallel()
	report, _, old, new := buildFixture(t,
		"SECRET_OLD_LINE\nkeep\n",
		"SECRET_NEW_LINE\nkeep\n", nil)
	_ = new
	if report.ContentIncluded || report.Content != nil {
		t.Fatal("content must not be included by default")
	}
	if !containsNote(report.Notes, NoContentNote) {
		t.Errorf("notes do not explain the default: %#v", report.Notes)
	}

	var jsonOut bytes.Buffer
	if err := report.WriteJSON(&jsonOut); err != nil {
		t.Fatal(err)
	}
	var markdown bytes.Buffer
	if err := report.WriteMarkdown(&markdown); err != nil {
		t.Fatal(err)
	}
	var htmlOut bytes.Buffer
	if err := report.WriteHTML(&htmlOut); err != nil {
		t.Fatal(err)
	}
	for name, out := range map[string]string{
		"json": jsonOut.String(), "markdown": markdown.String(), "html": htmlOut.String(),
	} {
		if strings.Contains(out, "SECRET_OLD_LINE") || strings.Contains(out, "SECRET_NEW_LINE") {
			t.Errorf("%s leaked compared content without opt-in:\n%s", name, out)
		}
	}
	_ = old
}

func TestBuildEmbedsContentOnlyWhenOptedIn(t *testing.T) {
	t.Parallel()
	report, _, _, _ := buildFixture(t, "left <script>\n", "right <b>\n", func(opts *BuildOptions) {
		opts.IncludeContent = true
	})
	if !report.ContentIncluded || report.Content == nil || len(report.Content.Hunks) == 0 {
		t.Fatal("content should be present when opted in")
	}
	var htmlOut bytes.Buffer
	if err := report.WriteHTML(&htmlOut); err != nil {
		t.Fatal(err)
	}
	out := htmlOut.String()
	if !strings.Contains(out, "&lt;script&gt;") {
		t.Error("embedded content must be HTML-escaped")
	}
	if strings.Contains(out, "<script>") {
		t.Error("raw markup from the compared files leaked into the HTML report")
	}
}

func TestBuildCountsWhitespaceOnlyHunks(t *testing.T) {
	t.Parallel()
	report, _, _, _ := buildFixture(t,
		"alpha\nkeep\n",
		"alpha   \nkeep\n", nil)
	if report.Totals.WhitespaceOnly != 1 {
		t.Fatalf("whitespace_only_hunks = %d, report=%#v", report.Totals.WhitespaceOnly, report.Totals)
	}
}

func TestEncodeFragmentMatchesURLStateSerialization(t *testing.T) {
	t.Parallel()
	state := json.RawMessage(`{"v":1,"mode":"text","paths":{"old":"a","new":"b"},"controls":{"ignoreCase":true}}`)
	fragment, err := EncodeFragment(state)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(fragment, "compare=") {
		t.Fatalf("fragment = %q", fragment)
	}
	encoded := strings.TrimPrefix(fragment, "compare=")
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("fragment is not base64url: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["mode"] != "text" {
		t.Errorf("decoded = %#v", decoded)
	}
}

func TestEncodeFragmentRejectsMalformedState(t *testing.T) {
	t.Parallel()
	for name, state := range map[string]json.RawMessage{
		"not-json":      json.RawMessage(`nope`),
		"wrong-version": json.RawMessage(`{"v":2,"mode":"text","paths":{},"controls":{}}`),
		"missing-paths": json.RawMessage(`{"v":1,"mode":"text","controls":{}}`),
	} {
		if _, err := EncodeFragment(state); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestWriteMarkdownIsPrintable(t *testing.T) {
	t.Parallel()
	report, _, _, _ := buildFixture(t, "a\n", "b\n", nil)
	var out bytes.Buffer
	if err := report.WriteMarkdown(&out); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{
		"# ayame-diff confirmation report", "## Inputs", "## Comparison conditions",
		"## Difference totals", "## Ignored hunks", "## Confirmation", "## Reproduce this comparison",
		"| Side | Path |",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("markdown report missing %q", want)
		}
	}
}

func TestWriteHTMLIsSelfContainedAndPrintable(t *testing.T) {
	t.Parallel()
	report, _, _, _ := buildFixture(t, "a\n", "b\n", nil)
	var out bytes.Buffer
	if err := report.WriteHTML(&out); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{"<!doctype html>", "<style>", "@media print", "</body></html>"} {
		if !strings.Contains(text, want) {
			t.Errorf("html report missing %q", want)
		}
	}
}

func containsNote(notes []string, want string) bool {
	for _, note := range notes {
		if note == want {
			return true
		}
	}
	return false
}
