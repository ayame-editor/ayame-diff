// Package threeway combines BASE→LEFT and BASE→RIGHT diffs and identifies
// independent edits, identical edits, and true conflicts.
package threeway

import (
	"context"
	"fmt"
	"io"
	"slices"
	"sort"

	"github.com/ayame-editor/ayame-diff/internal/linediff"
	"github.com/ayame-editor/ayame-diff/internal/mergechoice"
	"github.com/ayame-editor/ayame-diff/internal/textfile"
)

type Kind string

const (
	LeftOnly  Kind = "left_only"
	RightOnly Kind = "right_only"
	Same      Kind = "same_change"
	Conflict  Kind = "conflict"
	// Merged is CSV-only: within one key group LEFT and RIGHT edited different
	// base rows, so both edits apply and the group resolves without asking the
	// user to pick a side (#160).
	Merged Kind = "merged"
)

type Event struct {
	ID        int      `json:"id"`
	Kind      Kind     `json:"kind"`
	BaseStart uint64   `json:"base_start"`
	BaseLen   uint64   `json:"base_len"`
	Base      []string `json:"base"`
	Left      []string `json:"left"`
	Right     []string `json:"right"`
}

type Result struct {
	BaseLines uint64  `json:"base_lines"`
	Events    []Event `json:"events"`
	Conflicts int     `json:"conflicts"`
	LeftOnly  int     `json:"left_only"`
	RightOnly int     `json:"right_only"`
	Same      int     `json:"same_change"`
}

type edit struct {
	start, end uint64
	lines      []string
	side       byte
}

// Compare performs a three-way text comparison using the bounded-window
// two-way engine. Memory is proportional to changed regions, not file size.
func Compare(base, left, right linediff.Lines, options linediff.Options) (Result, error) {
	return CompareContext(context.Background(), base, left, right, options)
}

// CompareContext is Compare that aborts early when ctx is cancelled, so a
// server-side three-way diff of huge inputs stops on a client disconnect (#169).
func CompareContext(ctx context.Context, base, left, right linediff.Lines, options linediff.Options) (Result, error) {
	options.MaxHunks = int(^uint(0) >> 1)
	leftDiff, err := linediff.DiffWithContext(ctx, base, left, options)
	if err != nil {
		return Result{}, err
	}
	rightDiff, err := linediff.DiffWithContext(ctx, base, right, options)
	if err != nil {
		return Result{}, err
	}
	leftEdits := edits(base, left, leftDiff, 'L')
	rightEdits := edits(base, right, rightDiff, 'R')
	all := append(leftEdits, rightEdits...)
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].start != all[j].start {
			return all[i].start < all[j].start
		}
		if all[i].end != all[j].end {
			return all[i].end < all[j].end
		}
		return all[i].side < all[j].side
	})
	result := Result{BaseLines: base.Count()}
	for len(all) > 0 {
		cluster := []edit{all[0]}
		all = all[1:]
		start, end := cluster[0].start, cluster[0].end
		for len(all) > 0 && overlaps(start, end, all[0]) {
			cluster = append(cluster, all[0])
			if all[0].end > end {
				end = all[0].end
			}
			all = all[1:]
		}
		var le, re []edit
		for _, item := range cluster {
			if item.side == 'L' {
				le = append(le, item)
			} else {
				re = append(re, item)
			}
		}
		baseLines := lineRange(base, start, end-start)
		leftLines, rightLines := apply(base, start, end, le), apply(base, start, end, re)
		kind := Conflict
		switch {
		case len(le) == 0:
			kind = RightOnly
		case len(re) == 0:
			kind = LeftOnly
		case slices.Equal(leftLines, rightLines):
			kind = Same
		case slices.Equal(leftLines, baseLines):
			kind = RightOnly
		case slices.Equal(rightLines, baseLines):
			kind = LeftOnly
		}
		event := Event{ID: len(result.Events), Kind: kind, BaseStart: start, BaseLen: end - start, Base: baseLines, Left: leftLines, Right: rightLines}
		result.Events = append(result.Events, event)
		switch kind {
		case Conflict:
			result.Conflicts++
		case LeftOnly:
			result.LeftOnly++
		case RightOnly:
			result.RightOnly++
		case Same:
			result.Same++
		}
	}
	return result, nil
}

func edits(base, target linediff.Lines, result linediff.Result, side byte) []edit {
	items := make([]edit, 0, len(result.Hunks))
	for _, h := range result.Hunks {
		items = append(items, edit{start: h.OldStart, end: h.OldStart + h.OldLen, lines: lineRange(target, h.NewStart, h.NewLen), side: side})
	}
	return items
}

func overlaps(start, end uint64, item edit) bool {
	if start == end {
		return item.start == start
	}
	return item.start == start || item.start < end
}

func apply(base linediff.Lines, start, end uint64, changes []edit) []string {
	if len(changes) == 0 {
		return lineRange(base, start, end-start)
	}
	var result []string
	cursor := start
	for _, change := range changes {
		result = append(result, lineRange(base, cursor, change.start-cursor)...)
		result = append(result, change.lines...)
		cursor = change.end
	}
	return append(result, lineRange(base, cursor, end-cursor)...)
}

func lineRange(source linediff.Lines, start, length uint64) []string {
	lines := make([]string, 0, length)
	for index := start; index < start+length; index++ {
		if line, ok := source.Line(index); ok {
			lines = append(lines, line)
		}
	}
	return lines
}

// Origin names the document one line of a merge result came from. Manual marks
// a line the user typed into the result; no input supplied it (#257).
type Origin string

const (
	OriginBase   Origin = "base"
	OriginLeft   Origin = "left"
	OriginRight  Origin = "right"
	OriginManual Origin = "manual"
)

// Provenance counts a merge result's lines by origin.
type Provenance struct {
	Base   int `json:"base"`
	Left   int `json:"left"`
	Right  int `json:"right"`
	Manual int `json:"manual"`
	Total  int `json:"total"`
}

// MergedLine is one line of a merge result together with its provenance. Key is
// stable across conflict choices — it names the input line, not the output
// position — so a manual override can be carried from a preview to the save
// that follows it (#257).
type MergedLine struct {
	Text   string `json:"text"`
	Origin Origin `json:"origin"`
	Key    string `json:"key"`
}

// MergeLines selects automatic non-conflicts plus explicit conflict choices. A
// conflict choice is a comma-joined list of sides ("left", "right", "base", or a
// combination such as "left,right"), concatenated in the canonical
// base→left→right order. Missing conflict choices are emitted with standard
// conflict markers when allowUnresolved is true; otherwise an error is returned.
func MergeLines(base linediff.Lines, result Result, choices map[int]string, allowUnresolved bool) ([]string, int, error) {
	lines, unresolved, _, err := MergeLinesTarget(base, result, choices, allowUnresolved, UnresolvedMarkers)
	return lines, unresolved, err
}

// Resolution targets for an undecided three-way conflict. The zero value
// and UnresolvedMarkers both leave standard markers, so the CLI and existing
// callers keep their behavior (#272).
const (
	UnresolvedLeft    = "left"
	UnresolvedRight   = "right"
	UnresolvedBase    = "base"
	UnresolvedMarkers = "markers"
)

// MergeLinesTarget is MergeLines with a selectable implicit-resolution target.
// A left/right/base target resolves every undecided conflict to that side; the
// markers target leaves standard LEFT/BASE/RIGHT markers. It returns the number
// of conflicted events left undecided and the number of marker blocks written.
func MergeLinesTarget(base linediff.Lines, result Result, choices map[int]string, allowUnresolved bool, target string) ([]string, int, int, error) {
	merged, unresolved, markers, err := MergeLinesTargetOrigins(base, result, choices, allowUnresolved, target)
	if err != nil {
		return nil, unresolved, markers, err
	}
	lines := make([]string, len(merged))
	for i, line := range merged {
		lines[i] = line.Text
	}
	return lines, unresolved, markers, nil
}

// MergeLinesWithOrigins is MergeLines that also reports where each output line
// came from, so a GUI can show per-line provenance and count what was adopted
// versus typed (#257). Every output line carries one origin; the key of a line
// stays the same when only a conflict choice changes.
func MergeLinesWithOrigins(base linediff.Lines, result Result, choices map[int]string, allowUnresolved bool) ([]MergedLine, int, error) {
	merged, unresolved, _, err := MergeLinesTargetOrigins(base, result, choices, allowUnresolved, UnresolvedMarkers)
	return merged, unresolved, err
}

// mergeLinesOrigins composes the merge result line by line, recording each
// line's origin, and resolves undecided conflicts according to target. It
// returns the lines, the number of undecided conflicts, and the number of
// marker blocks written.
func MergeLinesTargetOrigins(base linediff.Lines, result Result, choices map[int]string, allowUnresolved bool, target string) ([]MergedLine, int, int, error) {
	if target == "" {
		target = UnresolvedMarkers
	}
	switch target {
	case UnresolvedLeft, UnresolvedRight, UnresolvedBase, UnresolvedMarkers:
	default:
		return nil, 0, 0, fmt.Errorf("unresolvedTarget must be left, right, base, or markers")
	}
	var output []MergedLine
	var cursor uint64
	unresolved, markers := 0, 0
	appendBase := func(start, length uint64) {
		for i := start; i < start+length; i++ {
			line, ok := base.Line(i)
			if !ok {
				continue
			}
			output = append(output, MergedLine{Text: line, Origin: OriginBase, Key: fmt.Sprintf("b:%d", i)})
		}
	}
	appendSide := func(eventID int, origin Origin, lines []string) {
		for i, line := range lines {
			output = append(output, MergedLine{Text: line, Origin: origin, Key: fmt.Sprintf("e%d:%s:%d", eventID, origin, i)})
		}
	}
	for _, event := range result.Events {
		appendBase(cursor, event.BaseStart-cursor)
		switch event.Kind {
		case LeftOnly, Same:
			appendSide(event.ID, OriginLeft, event.Left)
		case RightOnly:
			appendSide(event.ID, OriginRight, event.Right)
		case Conflict:
			// A conflict may adopt more than one contribution ("both"): the
			// recognized sides are concatenated in the canonical base→left→right
			// order regardless of the order the caller listed them (#271). The
			// older "both" token from #277 is folded into the left,right pair so
			// both spellings keep working.
			choice := choices[event.ID]
			if choice == "both" {
				choice = "left,right"
			}
			adopted := mergechoice.Parse(choice, "base", "left", "right")
			if len(adopted) > 0 {
				for _, side := range adopted {
					switch side {
					case "base":
						appendSide(event.ID, OriginBase, event.Base)
					case "left":
						appendSide(event.ID, OriginLeft, event.Left)
					case "right":
						appendSide(event.ID, OriginRight, event.Right)
					}
				}
				break
			}
			unresolved++
			if !allowUnresolved {
				return nil, unresolved, markers, fmt.Errorf("%d three-way conflicts are unresolved", unresolved)
			}
			switch target {
			case UnresolvedLeft:
				appendSide(event.ID, OriginLeft, event.Left)
			case UnresolvedRight:
				appendSide(event.ID, OriginRight, event.Right)
			case UnresolvedBase:
				appendSide(event.ID, OriginBase, event.Base)
			default:
				// Marker lines carry the origin of the section they open or
				// close, so an unresolved block is still attributable (#257).
				markers++
				marker := 0
				appendMarker := func(origin Origin, text string) {
					output = append(output, MergedLine{Text: text, Origin: origin, Key: fmt.Sprintf("e%d:unresolved:%d", event.ID, marker)})
					marker++
				}
				appendMarker(OriginLeft, "<<<<<<< LEFT")
				for _, line := range event.Left {
					appendMarker(OriginLeft, line)
				}
				appendMarker(OriginBase, "||||||| BASE")
				for _, line := range event.Base {
					appendMarker(OriginBase, line)
				}
				appendMarker(OriginBase, "=======")
				for _, line := range event.Right {
					appendMarker(OriginRight, line)
				}
				appendMarker(OriginRight, ">>>>>>> RIGHT")
			}
		}
		cursor = event.BaseStart + event.BaseLen
	}
	appendBase(cursor, base.Count()-cursor)
	return output, unresolved, markers, nil
}

// ApplyManual replaces the text of every line whose key appears in overrides
// and marks it manual. A caller that lets the user type into the merge result
// passes the typed lines here, so the saved report and the preview agree that
// they were not adopted from any side (#257).
func ApplyManual(lines []MergedLine, overrides map[string]string) []MergedLine {
	if len(overrides) == 0 {
		return lines
	}
	applied := make([]MergedLine, len(lines))
	copy(applied, lines)
	for i := range applied {
		if text, ok := overrides[applied[i].Key]; ok {
			applied[i].Text = text
			applied[i].Origin = OriginManual
		}
	}
	return applied
}

// CountOrigins tallies a merge result by origin for a save report (#257).
func CountOrigins(lines []MergedLine) Provenance {
	provenance := Provenance{Total: len(lines)}
	for _, line := range lines {
		switch line.Origin {
		case OriginLeft:
			provenance.Left++
		case OriginRight:
			provenance.Right++
		case OriginManual:
			provenance.Manual++
		case OriginBase:
			provenance.Base++
		}
	}
	return provenance
}

// The byte-level conventions of an input — encoding, BOM, terminator, final
// newline — are preserved by internal/textfile, which the GUI's editable panes
// share (#159, #255).
type MergeProfile = textfile.Profile

// ProfileOf derives the output conventions from base. Call it immediately
// before MergeLines, which streams forward from line 0.
func ProfileOf(base linediff.Lines) MergeProfile { return textfile.ProfileOf(base) }

// WriteMerged atomically writes the merged lines, restoring base's conventions.
func WriteMerged(path string, lines []string, profile MergeProfile) error {
	return textfile.Write(path, lines, profile)
}

// flushOnlyWriter hides an underlying io.Closer so a transform.Writer's Close
// flushes the encoder's final bytes without closing the atomic temp file.
type flushOnlyWriter struct{ w io.Writer }

func (f flushOnlyWriter) Write(p []byte) (int, error) { return f.w.Write(p) }

// isUTF8 reports whether name selects UTF-8 output, for which a BOM must be
// written explicitly (the codec does not add one).
func isUTF8(name string) bool { return textfile.IsUTF8(name) }
