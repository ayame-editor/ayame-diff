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

type lineEndings interface{ LineEnding(uint64) string }

// TextOptions controls safe output behavior.
type TextOptions struct {
	Output           string
	OldPath          string
	NewPath          string
	Choices          map[int]Side
	AllowUnresolved  bool
	Overwrite        bool
	ConfirmOverwrite bool
}

// TextResult reports the result without retaining any input lines.
type TextResult struct {
	Output     string `json:"output"`
	Resolved   int    `json:"resolved"`
	Unresolved int    `json:"unresolved"`
}

// WriteText streams unchanged and selected line ranges to a temporary sibling
// and renames it only after a complete flush. Unresolved hunks retain the left
// side when the caller explicitly permits saving them.
func WriteText(old, new linediff.Lines, diff linediff.Result, opts TextOptions) (result TextResult, resultErr error) {
	if opts.Output == "" {
		return result, fmt.Errorf("output path is required")
	}
	for index := range diff.Hunks {
		choice := opts.Choices[index]
		if choice == Left || choice == Right {
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
		var oldCursor uint64
		for index, hunk := range diff.Hunks {
			if hunk.OldStart < oldCursor {
				return fmt.Errorf("merge hunks overlap at %d", index)
			}
			if err := writeRange(old, oldCursor, hunk.OldStart-oldCursor); err != nil {
				return err
			}
			if opts.Choices[index] == Right {
				if err := writeRange(new, hunk.NewStart, hunk.NewLen); err != nil {
					return err
				}
			} else if err := writeRange(old, hunk.OldStart, hunk.OldLen); err != nil {
				return err
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
