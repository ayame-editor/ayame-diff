// Package merge composes selected diff hunks into new, atomically written files.
package merge

import (
	"bufio"
	"fmt"
	"io"

	"github.com/ayame-editor/ayame-diff/internal/atomicfile"
	"github.com/ayame-editor/ayame-diff/internal/linediff"
	"github.com/ayame-editor/ayame-diff/internal/pathutil"
)

// Side selects which document supplies one differing region.
type Side string

const (
	Left  Side = "left"
	Right Side = "right"
)

// Unresolved targets select what an undecided hunk becomes when the caller
// permits saving with unresolved hunks (#272). The zero value keeps the
// historical left side; markers leaves standard conflict markers instead.
const (
	UnresolvedLeft    = "left"
	UnresolvedRight   = "right"
	UnresolvedMarkers = "markers"
)

type lineEndings interface{ LineEnding(uint64) string }

// TextOptions controls safe output behavior.
type TextOptions struct {
	Output  string
	OldPath string
	NewPath string
	// Choices maps a hunk index to the contributions it adopts. An empty slice
	// is unresolved; more than one entry concatenates them in canonical order
	// (left before right) so adopting "both" is order-independent (#271).
	Choices          map[int][]Side
	AllowUnresolved  bool
	UnresolvedTarget string // "left", "right", or "markers"; "" means left
	Overwrite        bool
	ConfirmOverwrite bool
}

// orderedSides returns the recognized sides in canonical concatenation order
// (left before right), dropping duplicates and values that are not Left or
// Right.
func orderedSides(sides []Side) []Side {
	var hasLeft, hasRight bool
	for _, side := range sides {
		switch side {
		case Left:
			hasLeft = true
		case Right:
			hasRight = true
		}
	}
	var ordered []Side
	if hasLeft {
		ordered = append(ordered, Left)
	}
	if hasRight {
		ordered = append(ordered, Right)
	}
	return ordered
}

// TextResult reports the result without retaining any input lines.
type TextResult struct {
	Output     string `json:"output"`
	Resolved   int    `json:"resolved"`
	Unresolved int    `json:"unresolved"`
	// ImplicitlyResolved lists the hunk indexes that had no explicit choice and
	// were written using the unresolved target. A saved file can therefore be
	// traced back to the decisions that were never made (#272).
	ImplicitlyResolved []int `json:"implicitlyResolved,omitempty"`
	// ConflictsRemaining counts conflict markers actually left in the output. It
	// is zero for a left/right target even when Unresolved is nonzero, because
	// those hunks were decided implicitly rather than left for a later pass. This
	// keeps "saved" distinct from "no conflicts remain" (#272).
	ConflictsRemaining int `json:"conflictsRemaining"`
}

// WriteText streams unchanged and selected line ranges to a temporary sibling
// and renames it only after a complete flush. Unresolved hunks follow
// opts.UnresolvedTarget when the caller explicitly permits saving them.
func WriteText(old, new linediff.Lines, diff linediff.Result, opts TextOptions) (result TextResult, resultErr error) {
	if opts.Output == "" {
		return result, fmt.Errorf("output path is required")
	}
	target := opts.UnresolvedTarget
	if target == "" {
		target = UnresolvedLeft
	}
	if target != UnresolvedLeft && target != UnresolvedRight && target != UnresolvedMarkers {
		return result, fmt.Errorf("unresolvedTarget must be left, right, or markers")
	}
	for index := range diff.Hunks {
		if len(orderedSides(opts.Choices[index])) > 0 {
			result.Resolved++
		} else {
			result.Unresolved++
		}
	}
	if result.Unresolved > 0 && !opts.AllowUnresolved {
		return result, fmt.Errorf("%d merge hunks are unresolved", result.Unresolved)
	}
	aliasesInput := pathutil.Equal(opts.Output, opts.OldPath) || pathutil.Equal(opts.Output, opts.NewPath)
	if aliasesInput && (!opts.Overwrite || !opts.ConfirmOverwrite) {
		return result, fmt.Errorf("overwriting an input requires overwrite and explicit confirmation")
	}
	err := atomicfile.Write(opts.Output, atomicfile.Options{Pattern: ".ayame-diff-merge-*.tmp"}, func(destination io.Writer) error {
		writer := bufio.NewWriterSize(destination, 256*1024)
		writeRange := func(source linediff.Lines, start, length uint64) error {
			endings, preservesEOL := source.(lineEndings)
			for i := start; i < start+length; i++ {
				line, ok := source.Line(i)
				if !ok {
					return fmt.Errorf("line %d is unavailable", i+1)
				}
				if _, err := writer.WriteString(line); err != nil {
					return err
				}
				ending := "\n"
				if preservesEOL {
					ending = endings.LineEnding(i)
				}
				if _, err := writer.WriteString(ending); err != nil {
					return err
				}
			}
			return nil
		}
		// markerEnding picks the line terminator for a marker written around a
		// hunk: the source's own terminator where it has one, so a CRLF file
		// keeps CRLF instead of gaining a stray LF.
		markerEnding := func(hunk linediff.Hunk) string {
			for _, candidate := range []struct {
				source linediff.Lines
				start  uint64
				length uint64
			}{{old, hunk.OldStart, hunk.OldLen}, {new, hunk.NewStart, hunk.NewLen}} {
				if candidate.length == 0 {
					continue
				}
				if endings, ok := candidate.source.(lineEndings); ok {
					if ending := endings.LineEnding(candidate.start); ending != "" {
						return ending
					}
				}
			}
			return "\n"
		}
		writeUnresolved := func(hunk linediff.Hunk) error {
			if target == UnresolvedRight {
				return writeRange(new, hunk.NewStart, hunk.NewLen)
			}
			if target != UnresolvedMarkers {
				return writeRange(old, hunk.OldStart, hunk.OldLen)
			}
			ending := markerEnding(hunk)
			markers := []string{"<<<<<<< LEFT", "=======", ">>>>>>> RIGHT"}
			if _, err := writer.WriteString(markers[0] + ending); err != nil {
				return err
			}
			if err := writeRange(old, hunk.OldStart, hunk.OldLen); err != nil {
				return err
			}
			if _, err := writer.WriteString(markers[1] + ending); err != nil {
				return err
			}
			if err := writeRange(new, hunk.NewStart, hunk.NewLen); err != nil {
				return err
			}
			_, err := writer.WriteString(markers[2] + ending)
			return err
		}
		var oldCursor uint64
		for index, hunk := range diff.Hunks {
			if hunk.OldStart < oldCursor {
				return fmt.Errorf("merge hunks overlap at %d", index)
			}
			if err := writeRange(old, oldCursor, hunk.OldStart-oldCursor); err != nil {
				return err
			}
			// An unresolved hunk (no adopted side) follows the unresolved
			// target; adopted sides are concatenated in canonical order.
			adopted := orderedSides(opts.Choices[index])
			if len(adopted) == 0 {
				result.ImplicitlyResolved = append(result.ImplicitlyResolved, index)
				if err := writeUnresolved(hunk); err != nil {
					return err
				}
				if target == UnresolvedMarkers {
					result.ConflictsRemaining++
				}
			} else {
				for _, side := range adopted {
					if side == Right {
						if err := writeRange(new, hunk.NewStart, hunk.NewLen); err != nil {
							return err
						}
						continue
					}
					if err := writeRange(old, hunk.OldStart, hunk.OldLen); err != nil {
						return err
					}
				}
			}
			oldCursor = hunk.OldStart + hunk.OldLen
		}
		if err := writeRange(old, oldCursor, old.Count()-oldCursor); err != nil {
			return err
		}
		return writer.Flush()
	})
	if err != nil {
		return result, err
	}
	result.Output = opts.Output
	return result, nil
}
