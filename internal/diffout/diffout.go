// Package diffout renders a [linediff.Result] to text in four formats:
// unified (default), side-by-side, JSON, and a one-line summary. It is a thin
// presentation layer kept separate from the pure diff core so that core stays
// I/O- and format-agnostic (see hjosugi/ayame-diff#6, ADR 0002).
//
// Behavior is ported from ayame-editor's crates/ayame-cli/src/diff.rs
// (print_diff_result). The one deliberate improvement over the reference is
// display-width-correct (CJK-aware) truncation and padding in side-by-side
// mode via internal/textwidth, replacing the reference's char-count layout.
package diffout

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/hjosugi/ayame-diff/internal/linediff"
	"github.com/hjosugi/ayame-diff/internal/textwidth"
	"github.com/hjosugi/ayame-diff/internal/worddiff"
)

// Format selects the rendering of a diff result.
type Format uint8

const (
	// Unified is the default GNU-diff-like output: a header per hunk, old lines
	// prefixed "-", new lines prefixed "+".
	Unified Format = iota
	// SideBySide lays old and new lines in two aligned columns.
	SideBySide
	// JSON emits the machine-readable result (no summary).
	JSON
	// Summary emits only the one-line stats summary.
	Summary
	// Normal emits a classic (GNU diff, no-flags) patch: <n>c<n> / <n>a<n> /
	// <n>d<n> headers with "< " old and "> " new lines.
	Normal
)

// Options tunes rendering. Zero values fall back to the reference defaults
// inside [Write], so a zero Options renders a full unified diff.
type Options struct {
	Format   Format
	MaxLines uint64 // per-hunk line cap; 0 means default 200
	Width    int    // side-by-side total width; <=0 means default 160
	// Word, in Unified format, highlights the changed words within a Replace
	// hunk using git-style [-removed-] / {+added+} markers instead of plain
	// -/+ lines. Ignored for the other formats.
	Word bool
}

const (
	defaultMaxLines uint64 = 200
	defaultWidth    int    = 160
	minWidth        int    = 60
	minColumn       int    = 20
	maxColumn       int    = 120
)

// Write renders res to w (unified/side-by-side hunks or JSON) and the summary
// line to summaryW, matching the reference. Which streams are used depends on
// opts.Format: JSON writes only w, Summary writes only summaryW, and the two
// hunk formats write hunks to w followed by the summary to summaryW (mirroring
// the reference's stdout/stderr split).
func Write(w io.Writer, summaryW io.Writer, old, new linediff.Lines, res linediff.Result, opts Options) error {
	// A per-hunk cap of 0 would print nothing but the "more" line, which is
	// useless; treat 0 as "unset" and use the reference default instead.
	maxLines := opts.MaxLines
	if maxLines == 0 {
		maxLines = defaultMaxLines
	}
	width := opts.Width
	if width <= 0 {
		width = defaultWidth
	}

	switch opts.Format {
	case JSON:
		return writeJSON(w, res)
	case Summary:
		return writeSummary(summaryW, res)
	case Normal:
		if err := writeNormal(w, old, new, res, maxLines); err != nil {
			return err
		}
		return writeSummary(summaryW, res)
	case SideBySide:
		if err := writeSideBySide(w, old, new, res, maxLines, width); err != nil {
			return err
		}
		return writeSummary(summaryW, res)
	default: // Unified
		if err := writeUnified(w, old, new, res, maxLines, opts.Word); err != nil {
			return err
		}
		return writeSummary(summaryW, res)
	}
}

// jsonHunk / jsonResult are dedicated marshaling shapes: they fix the exact
// snake_case field names and order the reference (and any GUI consumer) expects,
// independent of the internal linediff struct layout.
type jsonHunk struct {
	Kind     string `json:"kind"` // lowercase insert/delete/replace
	OldStart uint64 `json:"old_start"`
	OldLen   uint64 `json:"old_len"`
	NewStart uint64 `json:"new_start"`
	NewLen   uint64 `json:"new_len"`
}

type jsonResult struct {
	OldLines     uint64     `json:"old_lines"`
	NewLines     uint64     `json:"new_lines"`
	Hunks        []jsonHunk `json:"hunks"`
	HunkCount    uint64     `json:"hunk_count"`
	OmittedHunks uint64     `json:"omitted_hunks"`
	Added        uint64     `json:"added"`
	Deleted      uint64     `json:"deleted"`
	Modified     uint64     `json:"modified"`
}

func writeJSON(w io.Writer, res linediff.Result) error {
	// Non-nil zero-length slice so an empty diff marshals to "[]", not "null".
	hunks := make([]jsonHunk, len(res.Hunks))
	for i, h := range res.Hunks {
		hunks[i] = jsonHunk{
			Kind:     h.Kind.String(),
			OldStart: h.OldStart,
			OldLen:   h.OldLen,
			NewStart: h.NewStart,
			NewLen:   h.NewLen,
		}
	}
	data, err := json.MarshalIndent(jsonResult{
		OldLines:     res.OldLines,
		NewLines:     res.NewLines,
		Hunks:        hunks,
		HunkCount:    res.HunkCount,
		OmittedHunks: res.OmittedHunks,
		Added:        res.Added,
		Deleted:      res.Deleted,
		Modified:     res.Modified,
	}, "", "  ")
	if err != nil {
		return err
	}
	// Trailing newline matches the reference's println-style emission (and the
	// repo's own summary-json output in cmd/ayame-diff).
	data = append(data, '\n')
	_, err = w.Write(data)
	return err
}

func writeUnified(w io.Writer, old, new linediff.Lines, res linediff.Result, maxLines uint64, word bool) error {
	bw := bufio.NewWriter(w)
	for _, h := range res.Hunks {
		writeHeader(bw, h)

		if word && h.Kind == linediff.Replace {
			writeWordReplace(bw, old, new, h, maxLines)
			continue
		}

		shownOld := min(h.OldLen, maxLines)
		for n := h.OldStart; n < h.OldStart+shownOld; n++ {
			fmt.Fprintf(bw, "-%s\n", lineAt(old, n))
		}
		if h.OldLen > shownOld {
			fmt.Fprintf(bw, "-... %d more line(s)\n", h.OldLen-shownOld)
		}

		shownNew := min(h.NewLen, maxLines)
		for n := h.NewStart; n < h.NewStart+shownNew; n++ {
			fmt.Fprintf(bw, "+%s\n", lineAt(new, n))
		}
		if h.NewLen > shownNew {
			fmt.Fprintf(bw, "+... %d more line(s)\n", h.NewLen-shownNew)
		}
	}
	return bw.Flush()
}

// writeNormal renders the result as a classic GNU-diff (no-flags) patch. The
// current hunk model (1:1 replaces, insert/delete runs) maps directly onto the
// c/a/d commands, so no context-line assembly is needed.
func writeNormal(w io.Writer, old, new linediff.Lines, res linediff.Result, maxLines uint64) error {
	bw := bufio.NewWriter(w)
	for _, h := range res.Hunks {
		switch h.Kind {
		case linediff.Delete:
			fmt.Fprintf(bw, "%sd%d\n", normalRange(h.OldStart, h.OldLen), h.NewStart)
			writeNormalLines(bw, old, "< ", h.OldStart, h.OldLen, maxLines)
		case linediff.Insert:
			fmt.Fprintf(bw, "%da%s\n", h.OldStart, normalRange(h.NewStart, h.NewLen))
			writeNormalLines(bw, new, "> ", h.NewStart, h.NewLen, maxLines)
		default: // Replace
			fmt.Fprintf(bw, "%sc%s\n", normalRange(h.OldStart, h.OldLen), normalRange(h.NewStart, h.NewLen))
			writeNormalLines(bw, old, "< ", h.OldStart, h.OldLen, maxLines)
			fmt.Fprintln(bw, "---")
			writeNormalLines(bw, new, "> ", h.NewStart, h.NewLen, maxLines)
		}
	}
	return bw.Flush()
}

// normalRange renders a 1-based line range: "S" for a single line, else "S1,S2".
func normalRange(start0, count uint64) string {
	s := start0 + 1
	if count <= 1 {
		return strconv.FormatUint(s, 10)
	}
	return fmt.Sprintf("%d,%d", s, start0+count)
}

func writeNormalLines(bw *bufio.Writer, lines linediff.Lines, prefix string, start, count, maxLines uint64) {
	shown := min(count, maxLines)
	for n := start; n < start+shown; n++ {
		fmt.Fprintf(bw, "%s%s\n", prefix, lineAt(lines, n))
	}
	if count > shown {
		fmt.Fprintf(bw, "%s... %d more line(s)\n", prefix, count-shown)
	}
}

// writeWordReplace renders a Replace hunk with intra-line word markers. It pairs
// old line k with new line k and highlights the differing words with git-style
// [-removed-] / {+added+} markers; lines whose word diff is skipped (identical
// after trimming, or too large — see worddiff limits) fall back to plain -/+.
// The current diff algorithm always emits 1:1 replaces, but the leftover
// branches keep this correct if that ever changes.
func writeWordReplace(bw *bufio.Writer, old, new linediff.Lines, h linediff.Hunk, maxLines uint64) {
	pairs := min(h.OldLen, h.NewLen)
	shown := min(pairs, maxLines)
	for k := uint64(0); k < shown; k++ {
		ol := lineAt(old, h.OldStart+k)
		nl := lineAt(new, h.NewStart+k)
		if wd, ok := worddiff.Diff(ol, nl); ok {
			fmt.Fprintln(bw, renderWordLine('-', wd.Old, "[-", "-]"))
			fmt.Fprintln(bw, renderWordLine('+', wd.New, "{+", "+}"))
		} else {
			fmt.Fprintf(bw, "-%s\n+%s\n", ol, nl)
		}
	}
	if pairs > shown {
		fmt.Fprintf(bw, "... %d more changed line(s)\n", pairs-shown)
	}
	writeExtra(bw, old, '-', h.OldStart+pairs, h.OldLen-pairs, maxLines)
	writeExtra(bw, new, '+', h.NewStart+pairs, h.NewLen-pairs, maxLines)
}

// writeExtra prints up to maxLines tagged lines starting at start, plus a
// "more line(s)" note when capped. count may be zero (nothing is printed).
func writeExtra(bw *bufio.Writer, lines linediff.Lines, tag byte, start, count, maxLines uint64) {
	shown := min(count, maxLines)
	for n := start; n < start+shown; n++ {
		fmt.Fprintf(bw, "%c%s\n", tag, lineAt(lines, n))
	}
	if count > shown {
		fmt.Fprintf(bw, "%c... %d more line(s)\n", tag, count-shown)
	}
}

func renderWordLine(prefix byte, segs []worddiff.Segment, openMark, closeMark string) string {
	var b strings.Builder
	b.WriteByte(prefix)
	for _, s := range segs {
		if s.Changed {
			b.WriteString(openMark)
			b.WriteString(s.Text)
			b.WriteString(closeMark)
		} else {
			b.WriteString(s.Text)
		}
	}
	return b.String()
}

func writeSideBySide(w io.Writer, old, new linediff.Lines, res linediff.Result, maxLines uint64, width int) error {
	if width < minWidth {
		width = minWidth
	}
	// Split the width between two columns after reserving 7 cells for the two
	// tags, two spaces, and the " | " separator; clamp to a readable range.
	column := (width - 7) / 2
	if column < minColumn {
		column = minColumn
	} else if column > maxColumn {
		column = maxColumn
	}

	bw := bufio.NewWriter(w)
	for _, h := range res.Hunks {
		writeHeader(bw, h)

		paired := max(h.OldLen, h.NewLen)
		shown := min(paired, maxLines)
		for offset := uint64(0); offset < shown; offset++ {
			oldPresent := offset < h.OldLen
			newPresent := offset < h.NewLen

			leftTag, rightTag := byte(' '), byte(' ')
			var leftText, rightText string
			if oldPresent {
				leftTag = '-'
				leftText = lineAt(old, h.OldStart+offset)
			}
			if newPresent {
				rightTag = '+'
				rightText = lineAt(new, h.NewStart+offset)
			}
			// Left column is padded to full width so the separator aligns;
			// right column only needs truncation (nothing follows it).
			fmt.Fprintf(bw, "%c %s | %c %s\n",
				leftTag, textwidth.PadRight(textwidth.Truncate(leftText, column), column),
				rightTag, textwidth.Truncate(rightText, column))
		}
		if paired > shown {
			fmt.Fprintf(bw, "... %d more paired line(s)\n", paired-shown)
		}
	}
	return bw.Flush()
}

func writeSummary(w io.Writer, res linediff.Result) error {
	_, err := fmt.Fprintln(w, summaryLine(res))
	return err
}

func writeHeader(bw *bufio.Writer, h linediff.Hunk) {
	// Starts are 0-based internally but 1-based in the header, like GNU diff.
	fmt.Fprintf(bw, "@@ -%d,%d +%d,%d %s @@\n",
		h.OldStart+1, h.OldLen, h.NewStart+1, h.NewLen, kindTitle(h.Kind))
}

// summaryLine reports the totals from the full diff (which stay accurate even
// when hunks were truncated), with a hint appended when hunks were omitted.
func summaryLine(res linediff.Result) string {
	s := fmt.Sprintf("%s hunk(s), %s added, %s deleted, %s modified",
		group(res.HunkCount), group(res.Added), group(res.Deleted), group(res.Modified))
	if res.OmittedHunks > 0 {
		s += " (output truncated; raise --max-hunks)"
	}
	return s
}

// lineAt fetches a line, defaulting to "" for an out-of-range index so a
// malformed hunk can never panic the renderer.
func lineAt(l linediff.Lines, i uint64) string {
	s, ok := l.Line(i)
	if !ok {
		return ""
	}
	return s
}

// kindTitle is the capitalized hunk-kind name used in headers ("Insert" etc.),
// distinct from linediff.Kind.String()'s lowercase JSON form.
func kindTitle(k linediff.Kind) string {
	switch k {
	case linediff.Insert:
		return "Insert"
	case linediff.Delete:
		return "Delete"
	case linediff.Replace:
		return "Replace"
	default:
		return "Unknown"
	}
}

// group formats n in base 10 with comma thousands separators (e.g. 1234 ->
// "1,234"), so large line counts in the summary stay readable.
func group(n uint64) string {
	s := strconv.FormatUint(n, 10)
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	// Emit the leading 1-3 digit group, then a comma + 3 digits repeatedly.
	head := len(s) % 3
	if head == 0 {
		head = 3
	}
	b.WriteString(s[:head])
	for i := head; i < len(s); i += 3 {
		b.WriteByte(',')
		b.WriteString(s[i : i+3])
	}
	return b.String()
}
