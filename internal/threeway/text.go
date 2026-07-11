// Package threeway combines BASE→LEFT and BASE→RIGHT diffs and identifies
// independent edits, identical edits, and true conflicts.
package threeway

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"

	"github.com/hjosugi/ayame-diff/internal/linediff"
)

type Kind string

const (
	LeftOnly  Kind = "left_only"
	RightOnly Kind = "right_only"
	Same      Kind = "same_change"
	Conflict  Kind = "conflict"
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
func Compare(base, left, right linediff.Lines, options linediff.Options) Result {
	options.MaxHunks = int(^uint(0) >> 1)
	leftEdits := edits(base, left, linediff.DiffWith(base, left, options), 'L')
	rightEdits := edits(base, right, linediff.DiffWith(base, right, options), 'R')
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
	return result
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

// MergeLines selects automatic non-conflicts plus explicit conflict choices.
// Missing conflict choices are emitted with standard conflict markers when
// allowUnresolved is true; otherwise an error is returned.
func MergeLines(base linediff.Lines, result Result, choices map[int]string, allowUnresolved bool) ([]string, int, error) {
	var output []string
	var cursor uint64
	unresolved := 0
	for _, event := range result.Events {
		output = append(output, lineRange(base, cursor, event.BaseStart-cursor)...)
		var selected []string
		switch event.Kind {
		case LeftOnly:
			selected = event.Left
		case RightOnly:
			selected = event.Right
		case Same:
			selected = event.Left
		case Conflict:
			switch choices[event.ID] {
			case "left":
				selected = event.Left
			case "right":
				selected = event.Right
			case "base":
				selected = event.Base
			default:
				unresolved++
				if !allowUnresolved {
					return nil, unresolved, fmt.Errorf("%d three-way conflicts are unresolved", unresolved)
				}
				selected = append(selected, "<<<<<<< LEFT")
				selected = append(selected, event.Left...)
				selected = append(selected, "||||||| BASE")
				selected = append(selected, event.Base...)
				selected = append(selected, "=======")
				selected = append(selected, event.Right...)
				selected = append(selected, ">>>>>>> RIGHT")
			}
		}
		output = append(output, selected...)
		cursor = event.BaseStart + event.BaseLen
	}
	output = append(output, lineRange(base, cursor, base.Count()-cursor)...)
	return output, unresolved, nil
}

// WriteMerged atomically writes UTF-8/LF output to a new path.
func WriteMerged(path string, lines []string) (resultErr error) {
	if path == "" {
		return fmt.Errorf("output path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".ayame-three-way-*.tmp")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer func() {
		_ = temp.Close()
		if resultErr != nil {
			_ = os.Remove(tempPath)
		}
	}()
	w := bufio.NewWriterSize(temp, 256*1024)
	for _, line := range lines {
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}
	if err := w.Flush(); err != nil {
		return err
	}
	if err := temp.Sync(); err != nil {
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tempPath, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tempPath, path); err == nil {
		return nil
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Rename(tempPath, path)
}
