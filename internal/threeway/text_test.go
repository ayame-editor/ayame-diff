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
