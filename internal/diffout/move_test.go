package diffout

import (
	"bytes"
	"strings"
	"testing"

	"github.com/hjosugi/ayame-diff/internal/linediff"
)

func TestMovedBlocksRenderInTextAndJSON(t *testing.T) {
	t.Parallel()
	old := linediff.SplitLines("top\nmove-a\nmove-b\nstay-a\nstay-b\nstay-c\nbottom\n")
	new := linediff.SplitLines("top\nstay-a\nstay-b\nstay-c\nmove-a\nmove-b\nbottom\n")
	res := linediff.Diff(old, new, 100, 128)
	linediff.DetectMoves(old, new, &res, linediff.MoveOptions{MinLines: 2})
	var text, summary bytes.Buffer
	if err := Write(&text, &summary, old, new, res, Options{Format: Unified}); err != nil {
		t.Fatal(err)
	}
	if strings.Count(text.String(), "MOVED #1") != 2 || !strings.Contains(summary.String(), "1 moved block(s) / 2 line(s)") {
		t.Fatalf("text=%q summary=%q", text.String(), summary.String())
	}
	var jsonOut bytes.Buffer
	if err := Write(&jsonOut, &bytes.Buffer{}, old, new, res, Options{Format: JSON}); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{`"move_id": 1`, `"move_peer":`, `"moved_blocks": 1`, `"moved_lines": 2`} {
		if !strings.Contains(jsonOut.String(), field) {
			t.Errorf("JSON missing %s: %s", field, jsonOut.String())
		}
	}
}

func TestSkippedMoveDetectionRendersInSummaryAndJSON(t *testing.T) {
	t.Parallel()
	res := linediff.Result{HunkCount: 2, OmittedHunks: 1, Modified: 2, MoveDetectionSkipped: true}
	var output, summary bytes.Buffer
	if err := Write(&bytes.Buffer{}, &summary, linediff.StringLines{}, linediff.StringLines{}, res, Options{Format: Summary}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(summary.String(), "move detection skipped because hunks were omitted") {
		t.Fatalf("summary = %q", summary.String())
	}
	if err := Write(&output, &bytes.Buffer{}, linediff.StringLines{}, linediff.StringLines{}, res, Options{Format: JSON}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `"move_detection_skipped": true`) {
		t.Fatalf("JSON = %s", output.String())
	}
}
