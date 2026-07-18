// Package threeway combines BASE→LEFT and BASE→RIGHT diffs and identifies
// independent edits, identical edits, and true conflicts.
package threeway

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"slices"
	"sort"

	"github.com/hjosugi/ayame-diff/internal/atomicfile"
	"github.com/hjosugi/ayame-diff/internal/encoding"
	"github.com/hjosugi/ayame-diff/internal/linediff"
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

// Optional capabilities a base source may expose so a merge round-trips the
// input's byte-level conventions instead of normalizing them.
type (
	lineEndings   interface{ LineEnding(uint64) string }
	encodedSource interface{ Encoding() string }
	bomSource     interface{ HasBOM() bool }
)

// MergeProfile captures the base file's character encoding, a leading UTF-8
// BOM, the line terminator, and whether the final line is newline-terminated,
// so WriteMerged reproduces them rather than forcing BOM-less UTF-8 with LF and
// a trailing newline (#159). Sources without this metadata (e.g. in-memory
// SplitLines) yield the historical UTF-8/LF/trailing-newline defaults.
type MergeProfile struct {
	Encoding     string // concrete encoding name; "" or "utf-8" needs no re-encoding
	BOM          bool   // the base began with a UTF-8 BOM
	LineEnding   string // terminator between lines ("\n", "\r\n", or "\r")
	FinalNewline bool   // whether to terminate the final line
}

// ProfileOf derives the output conventions from base. It reads the terminators
// of the last and then the first base line, so for a streaming FileLines it
// leaves the reader rewound to the start — call it immediately before
// MergeLines, which streams forward from line 0.
func ProfileOf(base linediff.Lines) MergeProfile {
	profile := MergeProfile{LineEnding: "\n", FinalNewline: true}
	if enc, ok := base.(encodedSource); ok {
		profile.Encoding = enc.Encoding()
	}
	if b, ok := base.(bomSource); ok {
		profile.BOM = b.HasBOM()
	}
	endings, ok := base.(lineEndings)
	if !ok {
		return profile
	}
	count := base.Count()
	if count == 0 {
		return profile
	}
	// Read the last terminator first (streams to the end), then the first
	// (rewinds to the start) so the caller streams forward from line 0. A final
	// line without a terminator reports "" and suppresses the trailing newline;
	// only the last line can lack one, so the first line always carries the
	// document's separator unless the file is a single unterminated line.
	profile.FinalNewline = endings.LineEnding(count-1) != ""
	if sep := endings.LineEnding(0); sep != "" {
		profile.LineEnding = sep
	}
	return profile
}

// flushOnlyWriter hides an underlying io.Closer so a transform.Writer's Close
// flushes the encoder's final bytes (e.g. ISO-2022-JP's return-to-ASCII escape)
// without also closing the atomic temp file, which atomicfile.Write owns.
type flushOnlyWriter struct{ w io.Writer }

func (f flushOnlyWriter) Write(p []byte) (int, error) { return f.w.Write(p) }

// WriteMerged atomically writes the merged lines to a new path, restoring the
// base file's encoding, BOM, line terminator, and final-newline state from
// profile so the output round-trips instead of being normalized to BOM-less
// UTF-8/LF with a forced trailing newline (#159).
func WriteMerged(path string, lines []string, profile MergeProfile) error {
	separator := profile.LineEnding
	if separator == "" {
		separator = "\n"
	}
	return atomicfile.Write(path, atomicfile.Options{Pattern: ".ayame-three-way-*.tmp"}, func(destination io.Writer) error {
		// A UTF-8 BOM is written raw; the UTF-16 encoders emit their own.
		if profile.BOM && isUTF8(profile.Encoding) {
			if _, err := destination.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
				return err
			}
		}
		encoded := encoding.Encoder(flushOnlyWriter{destination}, profile.Encoding)
		writer := bufio.NewWriterSize(encoded, 256*1024)
		for i, line := range lines {
			if _, err := writer.WriteString(line); err != nil {
				return err
			}
			if i < len(lines)-1 || profile.FinalNewline {
				if _, err := writer.WriteString(separator); err != nil {
					return err
				}
			}
		}
		if err := writer.Flush(); err != nil {
			return err
		}
		if closer, ok := encoded.(io.Closer); ok {
			return closer.Close()
		}
		return nil
	})
}

// isUTF8 reports whether name selects UTF-8 output, for which a BOM must be
// written explicitly (the codec does not add one).
func isUTF8(name string) bool { return name == "" || name == encoding.UTF8 }
