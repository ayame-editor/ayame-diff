package threeway

import (
	"reflect"
	"testing"

	"github.com/ayame-editor/ayame-diff/internal/linediff"
)

func TestCompareClassifiesIndependentSameAndConflict(t *testing.T) {
	base := linediff.SplitLines("a\nb\nc\nd\n")
	left := linediff.SplitLines("A\nb\nC-left\nd\nleft-tail\n")
	right := linediff.SplitLines("a\nB\nC-right\nd\nleft-tail\n")
	result, err := Compare(base, left, right, linediff.Options{Window: 32})
	if err != nil {
		t.Fatal(err)
	}
	if result.LeftOnly != 1 || result.RightOnly != 1 || result.Same != 1 || result.Conflicts != 1 {
		t.Fatalf("result=%+v events=%+v", result, result.Events)
	}
}

func TestMergeLinesAutomaticAndResolvedConflict(t *testing.T) {
	base := linediff.SplitLines("one\ntwo\nthree\n")
	left := linediff.SplitLines("ONE\nleft\nthree\n")
	right := linediff.SplitLines("one\nright\nTHREE\n")
	result, err := Compare(base, left, right, linediff.Options{Window: 32})
	if err != nil {
		t.Fatal(err)
	}
	choices := map[int]string{}
	for _, event := range result.Events {
		if event.Kind == Conflict {
			choices[event.ID] = "right"
		}
	}
	merged, unresolved, err := MergeLines(base, result, choices, false)
	if err != nil || unresolved != 0 {
		t.Fatalf("unresolved=%d err=%v", unresolved, err)
	}
	if !reflect.DeepEqual(merged, []string{"ONE", "right", "THREE"}) {
		t.Fatalf("merged=%q events=%+v", merged, result.Events)
	}
}

// TestMergeLinesAdoptsBothSides is the #271 contract for three-way text: a
// conflict choice may name more than one side, concatenated base→left→right
// regardless of the order the choice lists them.
func TestMergeLinesAdoptsBothSides(t *testing.T) {
	base := linediff.SplitLines("base\n")
	result, err := Compare(base, linediff.SplitLines("left\n"), linediff.SplitLines("right\n"), linediff.Options{Window: 8})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Events) != 1 || result.Events[0].Kind != Conflict {
		t.Fatalf("events=%+v", result.Events)
	}
	choices := map[int]string{result.Events[0].ID: "right,left"}
	merged, unresolved, err := MergeLines(base, result, choices, false)
	if err != nil || unresolved != 0 {
		t.Fatalf("unresolved=%d err=%v", unresolved, err)
	}
	if !reflect.DeepEqual(merged, []string{"left", "right"}) {
		t.Fatalf("both merged=%q", merged)
	}
	choices[result.Events[0].ID] = "base,right,left"
	merged, _, err = MergeLines(base, result, choices, false)
	if err != nil || !reflect.DeepEqual(merged, []string{"base", "left", "right"}) {
		t.Fatalf("all-three merged=%q err=%v", merged, err)
	}
}

func TestMergeLinesRejectsOrMarksUnresolved(t *testing.T) {
	base := linediff.SplitLines("base\n")
	result, err := Compare(base, linediff.SplitLines("left\n"), linediff.SplitLines("right\n"), linediff.Options{Window: 8})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := MergeLines(base, result, nil, false); err == nil {
		t.Fatal("unresolved merge succeeded")
	}
	merged, count, err := MergeLines(base, result, nil, true)
	if err != nil || count != 1 || merged[0] != "<<<<<<< LEFT" {
		t.Fatalf("merged=%q count=%d err=%v", merged, count, err)
	}
}

// TestMergeLinesTargetSelectsImplicitResolution covers #272: rather than always
// leaving markers, an unresolved three-way conflict can be sent to left, right,
// or base, and the marker count distinguishes a saved clean merge from one that
// still holds conflict markers.
func TestMergeLinesTargetSelectsImplicitResolution(t *testing.T) {
	base := linediff.SplitLines("base\n")
	result, err := Compare(base, linediff.SplitLines("left\n"), linediff.SplitLines("right\n"), linediff.Options{Window: 8})
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string][]string{
		UnresolvedLeft:  {"left"},
		UnresolvedRight: {"right"},
		UnresolvedBase:  {"base"},
	}
	for target, want := range cases {
		merged, unresolved, markers, err := MergeLinesTarget(base, result, nil, true, target)
		if err != nil || unresolved != 1 || markers != 0 {
			t.Fatalf("%s: merged=%q unresolved=%d markers=%d err=%v", target, merged, unresolved, markers, err)
		}
		if !reflect.DeepEqual(merged, want) {
			t.Fatalf("%s: merged=%q want=%q", target, merged, want)
		}
	}
	merged, unresolved, markers, err := MergeLinesTarget(base, result, nil, true, UnresolvedMarkers)
	if err != nil || unresolved != 1 || markers != 1 || merged[0] != "<<<<<<< LEFT" {
		t.Fatalf("markers: merged=%q unresolved=%d markers=%d err=%v", merged, unresolved, markers, err)
	}
	if _, _, _, err := MergeLinesTarget(base, result, nil, true, "middle"); err == nil {
		t.Fatal("unknown target accepted")
	}
}

// TestMergeLinesWithOriginsTracksEachSide pins the #257 contract: every output
// line names its source, and the key identifies the input line rather than the
// output position.
func TestMergeLinesWithOriginsTracksEachSide(t *testing.T) {
	base := linediff.SplitLines("a\nb\nc\n")
	left := linediff.SplitLines("a\nB\nc\n")
	right := linediff.SplitLines("a\nb\nC\n")
	result, err := Compare(base, left, right, linediff.Options{Window: 32})
	if err != nil {
		t.Fatal(err)
	}
	merged, unresolved, err := MergeLinesWithOrigins(base, result, nil, false)
	if err != nil || unresolved != 0 {
		t.Fatalf("unresolved=%d err=%v", unresolved, err)
	}
	want := []MergedLine{
		{Text: "a", Origin: OriginBase, Key: "b:0"},
		{Text: "B", Origin: OriginLeft, Key: "e0:left:0"},
		{Text: "C", Origin: OriginRight, Key: "e1:right:0"},
	}
	if !reflect.DeepEqual(merged, want) {
		t.Fatalf("merged=%+v", merged)
	}
	if got := CountOrigins(merged); got != (Provenance{Base: 1, Left: 1, Right: 1, Total: 3}) {
		t.Fatalf("provenance=%+v", got)
	}
}

func TestMergeLinesWithOriginsConflictChoiceKeepsStableKeys(t *testing.T) {
	base := linediff.SplitLines("base\n")
	left := linediff.SplitLines("left\n")
	right := linediff.SplitLines("right\n")
	result, err := Compare(base, left, right, linediff.Options{Window: 8})
	if err != nil {
		t.Fatal(err)
	}
	id := result.Events[0].ID
	for side, want := range map[string]MergedLine{
		"left":  {Text: "left", Origin: OriginLeft, Key: "e0:left:0"},
		"right": {Text: "right", Origin: OriginRight, Key: "e0:right:0"},
		"base":  {Text: "base", Origin: OriginBase, Key: "e0:base:0"},
	} {
		merged, _, err := MergeLinesWithOrigins(base, result, map[int]string{id: side}, false)
		if err != nil || len(merged) != 1 || merged[0] != want {
			t.Fatalf("choice %s merged=%+v err=%v", side, merged, err)
		}
	}
}

func TestMergeLinesWithOriginsUnresolvedMarkersAreAttributable(t *testing.T) {
	base := linediff.SplitLines("base\n")
	result, err := Compare(base, linediff.SplitLines("left\n"), linediff.SplitLines("right\n"), linediff.Options{Window: 8})
	if err != nil {
		t.Fatal(err)
	}
	merged, unresolved, err := MergeLinesWithOrigins(base, result, nil, true)
	if err != nil || unresolved != 1 {
		t.Fatalf("unresolved=%d err=%v", unresolved, err)
	}
	got := CountOrigins(merged)
	want := Provenance{Base: 3, Left: 2, Right: 2, Total: 7}
	if got != want {
		t.Fatalf("provenance=%+v want=%+v lines=%+v", got, want, merged)
	}
	for _, line := range merged {
		if line.Origin != OriginLeft && line.Origin != OriginRight && line.Origin != OriginBase {
			t.Fatalf("unresolved line without a side: %+v", line)
		}
		if line.Key == "" {
			t.Fatalf("unresolved line without a stable key: %+v", line)
		}
	}
}

func TestApplyManualMarksTypedLines(t *testing.T) {
	base := linediff.SplitLines("a\nb\n")
	left := linediff.SplitLines("a\nB\n")
	right := linediff.SplitLines("a\nb\n")
	result, err := Compare(base, left, right, linediff.Options{Window: 8})
	if err != nil {
		t.Fatal(err)
	}
	merged, _, err := MergeLinesWithOrigins(base, result, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	applied := ApplyManual(merged, map[string]string{"e0:left:0": "typed"})
	if applied[1].Text != "typed" || applied[1].Origin != OriginManual {
		t.Fatalf("manual line=%+v", applied[1])
	}
	// The original slice must not be mutated: a caller may reuse the preview.
	if merged[1].Text != "B" || merged[1].Origin != OriginLeft {
		t.Fatalf("ApplyManual mutated its input: %+v", merged[1])
	}
	if got := CountOrigins(applied); got.Manual != 1 || got.Left != 0 || got.Base != 1 {
		t.Fatalf("provenance=%+v", got)
	}
}
