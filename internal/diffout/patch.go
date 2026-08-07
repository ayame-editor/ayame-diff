package diffout

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/ayame-editor/ayame-diff/internal/linediff"
)

type lineEndingSource interface {
	LineEnding(uint64) string
}

func lineEndingAt(lines linediff.Lines, index uint64) string {
	if source, ok := lines.(lineEndingSource); ok {
		return source.LineEnding(index)
	}
	// Legacy/custom Lines implementations carry no terminator metadata. Keep
	// their historical behavior and treat every materialized line as LF-ended.
	return "\n"
}

func writePatchLine(w io.Writer, prefix string, lines linediff.Lines, index uint64) {
	fmt.Fprint(w, prefix, lineAt(lines, index))
	ending := lineEndingAt(lines, index)
	if ending == "" {
		fmt.Fprintln(w)
		fmt.Fprintln(w, `\ No newline at end of file`)
		return
	}
	fmt.Fprint(w, ending)
}

// ValidatePatchable rejects decoded binary input before an HTTP or CLI caller
// commits response headers/output bytes.
func ValidatePatchable(old, new linediff.Lines) error {
	for _, source := range []struct {
		name  string
		lines linediff.Lines
	}{{"old", old}, {"new", new}} {
		for i := uint64(0); i < source.lines.Count(); i++ {
			line, _ := source.lines.Line(i)
			if strings.IndexByte(line, 0) >= 0 {
				return fmt.Errorf("%s input appears binary (NUL byte at line %d); patch output supports text only", source.name, i+1)
			}
		}
	}
	return nil
}

type patchGroup struct {
	hunks                              []linediff.Hunk
	oldStart, oldEnd, newStart, newEnd uint64
}

func patchContext(opts Options) uint64 {
	if !opts.ContextSet {
		return 3
	}
	if opts.Context < 0 {
		return 0
	}
	return uint64(opts.Context)
}

func makePatchGroups(res linediff.Result, context uint64) ([]patchGroup, error) {
	if res.OmittedHunks != 0 {
		return nil, fmt.Errorf("patch output requires every hunk; increase --max-hunks or omit the limit")
	}
	if len(res.Hunks) == 0 {
		return nil, nil
	}
	groups := make([]patchGroup, 0, len(res.Hunks))
	for start := 0; start < len(res.Hunks); {
		end := start + 1
		last := res.Hunks[start]
		for end < len(res.Hunks) {
			next := res.Hunks[end]
			oldEnd := last.OldStart + last.OldLen
			newEnd := last.NewStart + last.NewLen
			oldGap, newGap := uint64(0), uint64(0)
			if next.OldStart > oldEnd {
				oldGap = next.OldStart - oldEnd
			}
			if next.NewStart > newEnd {
				newGap = next.NewStart - newEnd
			}
			if oldGap > 2*context || newGap > 2*context {
				break
			}
			last = next
			end++
		}
		first := res.Hunks[start]
		oldStart := first.OldStart
		newStart := first.NewStart
		if oldStart > context {
			oldStart -= context
		} else {
			oldStart = 0
		}
		if newStart > context {
			newStart -= context
		} else {
			newStart = 0
		}
		oldEnd := expandedPatchEnd(last.OldStart+last.OldLen, context, res.OldLines)
		newEnd := expandedPatchEnd(last.NewStart+last.NewLen, context, res.NewLines)
		groups = append(groups, patchGroup{
			hunks: res.Hunks[start:end], oldStart: oldStart, oldEnd: oldEnd,
			newStart: newStart, newEnd: newEnd,
		})
		start = end
	}
	return groups, nil
}

func expandedPatchEnd(end, context, total uint64) uint64 {
	if end >= total || context >= total-end {
		return total
	}
	return end + context
}

func patchLabel(label, fallback string) string {
	if label == "" {
		label = fallback
	}
	return strings.NewReplacer("\n", "_", "\r", "_", "\t", "_").Replace(label)
}

func writeFileHeader(w io.Writer, marker, label string, stamp time.Time) {
	fmt.Fprint(w, marker, label)
	if !stamp.IsZero() {
		fmt.Fprint(w, "\t", stamp.Format("2006-01-02 15:04:05.000000000 -0700"))
	}
	fmt.Fprintln(w)
}

func unifiedRange(start, count uint64) string {
	line := start
	if count > 0 {
		line++
	}
	if count == 1 {
		return strconv.FormatUint(line, 10)
	}
	return fmt.Sprintf("%d,%d", line, count)
}

func writeUnifiedPatch(w io.Writer, old, new linediff.Lines, res linediff.Result, opts Options) error {
	groups, err := makePatchGroups(res, patchContext(opts))
	if err != nil || len(groups) == 0 {
		return err
	}
	bw := bufio.NewWriter(w)
	writeFileHeader(bw, "--- ", patchLabel(opts.OldLabel, "old"), opts.OldTime)
	writeFileHeader(bw, "+++ ", patchLabel(opts.NewLabel, "new"), opts.NewTime)
	for _, group := range groups {
		fmt.Fprintf(bw, "@@ -%s +%s @@\n",
			unifiedRange(group.oldStart, group.oldEnd-group.oldStart),
			unifiedRange(group.newStart, group.newEnd-group.newStart))
		oi, ni := group.oldStart, group.newStart
		for _, h := range group.hunks {
			for oi < h.OldStart && ni < h.NewStart {
				writePatchLine(bw, " ", old, oi)
				oi++
				ni++
			}
			for n := h.OldStart; n < h.OldStart+h.OldLen; n++ {
				writePatchLine(bw, "-", old, n)
			}
			for n := h.NewStart; n < h.NewStart+h.NewLen; n++ {
				writePatchLine(bw, "+", new, n)
			}
			oi = h.OldStart + h.OldLen
			ni = h.NewStart + h.NewLen
		}
		for oi < group.oldEnd && ni < group.newEnd {
			writePatchLine(bw, " ", old, oi)
			oi++
			ni++
		}
	}
	return bw.Flush()
}

func contextRange(start, count uint64) string {
	if count == 0 {
		return "0"
	}
	first := start + 1
	if count == 1 {
		return strconv.FormatUint(first, 10)
	}
	return fmt.Sprintf("%d,%d", first, start+count)
}

func writeContextPatch(w io.Writer, old, new linediff.Lines, res linediff.Result, opts Options) error {
	groups, err := makePatchGroups(res, patchContext(opts))
	if err != nil || len(groups) == 0 {
		return err
	}
	bw := bufio.NewWriter(w)
	writeFileHeader(bw, "*** ", patchLabel(opts.OldLabel, "old"), opts.OldTime)
	writeFileHeader(bw, "--- ", patchLabel(opts.NewLabel, "new"), opts.NewTime)
	for _, group := range groups {
		fmt.Fprintln(bw, "***************")
		fmt.Fprintf(bw, "*** %s ****\n", contextRange(group.oldStart, group.oldEnd-group.oldStart))
		writeContextSection(bw, old, group, true)
		fmt.Fprintf(bw, "--- %s ----\n", contextRange(group.newStart, group.newEnd-group.newStart))
		writeContextSection(bw, new, group, false)
	}
	return bw.Flush()
}

func writeContextSection(w io.Writer, lines linediff.Lines, group patchGroup, old bool) {
	position, end := group.newStart, group.newEnd
	if old {
		position, end = group.oldStart, group.oldEnd
	}
	for _, h := range group.hunks {
		changeStart, changeLen, prefix := h.NewStart, h.NewLen, "+ "
		if old {
			changeStart, changeLen, prefix = h.OldStart, h.OldLen, "- "
		}
		for position < changeStart {
			writePatchLine(w, "  ", lines, position)
			position++
		}
		if h.Kind == linediff.Replace {
			prefix = "! "
		}
		for n := changeStart; n < changeStart+changeLen; n++ {
			writePatchLine(w, prefix, lines, n)
		}
		position = changeStart + changeLen
	}
	for position < end {
		writePatchLine(w, "  ", lines, position)
		position++
	}
}
