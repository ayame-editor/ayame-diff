// Package merge composes selected diff hunks into new, atomically written files.
package merge

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/hjosugi/ayame-diff/internal/linediff"
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
	aliasesInput := samePath(opts.Output, opts.OldPath) || samePath(opts.Output, opts.NewPath)
	if aliasesInput && (!opts.Overwrite || !opts.ConfirmOverwrite) {
		return result, fmt.Errorf("overwriting an input requires overwrite and explicit confirmation")
	}
	dir := filepath.Dir(opts.Output)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return result, err
	}
	temp, err := os.CreateTemp(dir, ".ayame-diff-merge-*.tmp")
	if err != nil {
		return result, err
	}
	tempPath := temp.Name()
	open := true
	defer func() {
		if open {
			_ = temp.Close()
		}
		if resultErr != nil {
			_ = os.Remove(tempPath)
		}
	}()
	w := bufio.NewWriterSize(temp, 256*1024)
	writeRange := func(source linediff.Lines, start, length uint64) error {
		endings, preservesEOL := source.(lineEndings)
		for i := start; i < start+length; i++ {
			line, ok := source.Line(i)
			if !ok {
				return fmt.Errorf("line %d is unavailable", i+1)
			}
			if _, err := w.WriteString(line); err != nil {
				return err
			}
			ending := "\n"
			if preservesEOL {
				ending = endings.LineEnding(i)
			}
			if _, err := w.WriteString(ending); err != nil {
				return err
			}
		}
		return nil
	}
	var oldCursor uint64
	for index, hunk := range diff.Hunks {
		if hunk.OldStart < oldCursor {
			return result, fmt.Errorf("merge hunks overlap at %d", index)
		}
		if err := writeRange(old, oldCursor, hunk.OldStart-oldCursor); err != nil {
			return result, err
		}
		if opts.Choices[index] == Right {
			if err := writeRange(new, hunk.NewStart, hunk.NewLen); err != nil {
				return result, err
			}
		} else if err := writeRange(old, hunk.OldStart, hunk.OldLen); err != nil {
			return result, err
		}
		oldCursor = hunk.OldStart + hunk.OldLen
	}
	if err := writeRange(old, oldCursor, old.Count()-oldCursor); err != nil {
		return result, err
	}
	if err := w.Flush(); err != nil {
		return result, err
	}
	if err := temp.Sync(); err != nil {
		return result, err
	}
	if err := temp.Close(); err != nil {
		return result, err
	}
	open = false
	if err := os.Chmod(tempPath, 0o644); err != nil {
		return result, err
	}
	if aliasesInput && opts.Overwrite && runtime.GOOS == "windows" {
		// os.Rename atomically replaces on Unix. Windows portable builds may need
		// the destination removed first; the default new-output path remains atomic.
		if err := os.Remove(opts.Output); err != nil {
			return result, err
		}
	}
	if err := os.Rename(tempPath, opts.Output); err != nil {
		if runtime.GOOS != "windows" {
			return result, err
		}
		if removeErr := os.Remove(opts.Output); removeErr != nil && !os.IsNotExist(removeErr) {
			return result, removeErr
		}
		if renameErr := os.Rename(tempPath, opts.Output); renameErr != nil {
			return result, renameErr
		}
	}
	result.Output = opts.Output
	return result, nil
}

func samePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	aa, errA := filepath.Abs(a)
	bb, errB := filepath.Abs(b)
	return errA == nil && errB == nil && filepath.Clean(aa) == filepath.Clean(bb)
}
