package linediff

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// Lines is a random-access, count-known source of text lines. Implementations
// back it however they like — an in-memory slice ([StringLines]) or a batched,
// window-caching reader over a huge file. The diff walk accesses lines almost
// monotonically (forward with a bounded look-ahead), so a forward-caching
// implementation stays cheap.
type Lines interface {
	// Count returns the number of lines.
	Count() uint64
	// Line returns the line at index i and true, or "", false if i is out of
	// range.
	Line(i uint64) (string, bool)
}

// StringLines adapts an in-memory slice of lines to [Lines].
type StringLines []string

// Count implements [Lines].
func (s StringLines) Count() uint64 { return uint64(len(s)) }

// Line implements [Lines].
func (s StringLines) Line(i uint64) (string, bool) {
	if i >= uint64(len(s)) {
		return "", false
	}
	return s[i], true
}

// TextLines is an in-memory line source that also preserves each original line
// terminator for patch output. Comparison still uses the normalized text.
type TextLines struct {
	lines   StringLines
	endings []string
}

func (t *TextLines) Count() uint64 { return t.lines.Count() }
func (t *TextLines) Line(i uint64) (string, bool) {
	return t.lines.Line(i)
}
func (t *TextLines) LineEnding(i uint64) string {
	if i >= uint64(len(t.endings)) {
		return ""
	}
	return t.endings[i]
}

// SplitTextLines splits LF, CRLF, and lone-CR text while preserving each exact
// terminator. A final terminator does not create an extra empty line.
func SplitTextLines(s string) *TextLines {
	result := &TextLines{}
	for len(s) > 0 {
		i := strings.IndexAny(s, "\r\n")
		if i < 0 {
			result.lines = append(result.lines, s)
			result.endings = append(result.endings, "")
			break
		}
		ending, width := s[i:i+1], 1
		if ending == "\r" && i+1 < len(s) && s[i+1] == '\n' {
			ending, width = "\r\n", 2
		}
		result.lines = append(result.lines, s[:i])
		result.endings = append(result.endings, ending)
		s = s[i+width:]
	}
	return result
}

// SplitLines splits LF, CRLF, and lone-CR text into normalized lines. A final
// terminator does not produce an extra empty line; an empty string has no lines.
func SplitLines(s string) StringLines {
	return SplitTextLines(s).lines
}

// Whitespace selects how whitespace is treated when comparing lines.
type Whitespace uint8

const (
	// WSKeep compares whitespace exactly (default).
	WSKeep Whitespace = iota
	// WSChange collapses each run of whitespace to a single space and trims
	// the ends, so indentation and spacing changes are ignored (WinMerge's
	// "ignore change").
	WSChange
	// WSAll removes all whitespace before comparing (WinMerge's "ignore all").
	WSAll
)

// Options tunes the diff. MaxHunks and Window are always used; the Ignore*
// fields normalize the text used for *comparison* only — output still shows the
// original lines.
type Options struct {
	MaxHunks          int
	Window            uint64
	IgnoreCase        bool
	Whitespace        Whitespace
	IgnoreEOL         bool
	IgnoreTrailingEOL bool
	LineFilters       []*regexp.Regexp
	SyncPoints        []SyncPoint
}

// Diff computes the line diff of old vs new. At most maxHunks hunks are stored
// in the result; any beyond that are still counted (HunkCount / OmittedHunks /
// stats). window bounds how far the resync scan looks ahead when it hits a
// difference. It is Diff­With with no ignore options.
func Diff(old, new Lines, maxHunks int, window uint64) Result {
	return diffWithOptions(old, new, Options{MaxHunks: maxHunks, Window: window})
}

// DiffWith is Diff with comparison options. When any ignore option is set, the
// comparison runs over a normalized view of each line while positions (and thus
// the output, rendered from the caller's originals) are unchanged. Invalid sync
// points are returned to the caller instead of being silently ignored.
func DiffWith(old, new Lines, opts Options) (Result, error) {
	if err := ValidateSyncPoints(opts.SyncPoints, old.Count(), new.Count()); err != nil {
		return Result{}, err
	}
	if len(opts.SyncPoints) > 0 {
		return diffWithSyncPoints(old, new, opts), nil
	}
	return diffWithOptions(old, new, opts), nil
}

func diffWithOptions(old, new Lines, opts Options) Result {
	window := opts.Window
	if window < 1 {
		window = 1
	}
	maxHunks := opts.MaxHunks
	comparison := lineComparator{old: old, new: new, norm: normalizer(opts), opts: opts}
	oldTotal := old.Count()
	newTotal := new.Count()
	res := Result{OldLines: oldTotal, NewLines: newTotal}

	var i, j uint64
	for i < oldTotal || j < newTotal {
		if i < oldTotal && j < newTotal {
			if comparison.equal(i, j) {
				i++
				j++
				continue
			}
		}

		h := nextDiffHunk(comparison, i, j, window)
		applyStats(&res, h)
		res.HunkCount++
		if len(res.Hunks) < maxHunks {
			res.Hunks = append(res.Hunks, h)
		} else {
			res.OmittedHunks++
		}
		i += h.OldLen
		j += h.NewLen
	}
	return res
}

func nextDiffHunk(comparison lineComparator, i, j, window uint64) Hunk {
	old, new := comparison.old, comparison.new
	oldTotal := old.Count()
	newTotal := new.Count()
	if i >= oldTotal {
		return Hunk{Kind: Insert, OldStart: i, OldLen: 0, NewStart: j, NewLen: newTotal - j}
	}
	if j >= newTotal {
		return Hunk{Kind: Delete, OldStart: i, OldLen: oldTotal - i, NewStart: j, NewLen: 0}
	}

	// Anchor lines: the resync scans below read ahead in each source.
	// Insert: the old anchor reappears in new within the window -> the skipped
	// new lines were inserted. Delete: the new anchor reappears in old -> the
	// skipped old lines were deleted.
	rj, insOK := comparison.findNew(i, j+1, clampEnd(j+1, window, newTotal))
	li, delOK := comparison.findOld(j, i+1, clampEnd(i+1, window, oldTotal))

	switch {
	case insOK && delOK:
		// Prefer the smaller edit; ties favor insert, matching the reference.
		if rj-j <= li-i {
			return insertHunk(i, j, rj-j)
		}
		return deleteHunk(i, j, li-i)
	case insOK:
		return insertHunk(i, j, rj-j)
	case delOK:
		return deleteHunk(i, j, li-i)
	default:
		// No resync within the window: treat it as a 1:1 replacement and move
		// on. Consecutive replaced lines become consecutive 1:1 hunks.
		return Hunk{Kind: Replace, OldStart: i, OldLen: 1, NewStart: j, NewLen: 1}
	}
}

// clampEnd returns min(start+window, total) without risking uint64 overflow.
// Callers guarantee start <= total.
func clampEnd(start, window, total uint64) uint64 {
	if window > total-start {
		return total
	}
	return start + window
}

func insertHunk(oldStart, newStart, newLen uint64) Hunk {
	return Hunk{Kind: Insert, OldStart: oldStart, OldLen: 0, NewStart: newStart, NewLen: newLen}
}

func deleteHunk(oldStart, newStart, oldLen uint64) Hunk {
	return Hunk{Kind: Delete, OldStart: oldStart, OldLen: oldLen, NewStart: newStart, NewLen: 0}
}

// lineEndings is implemented by line sources that preserve original EOLs.
// Keeping this optional leaves synthetic StringLines backwards compatible.
type lineEndings interface {
	LineEnding(uint64) string
}

type lineComparator struct {
	old, new Lines
	norm     func(string) string
	opts     Options
}

func (c lineComparator) equal(oldIndex, newIndex uint64) bool {
	oldText, oldOK := c.old.Line(oldIndex)
	newText, newOK := c.new.Line(newIndex)
	if !oldOK || !newOK {
		return false
	}
	if c.norm != nil {
		oldText, newText = c.norm(oldText), c.norm(newText)
	}
	if oldText != newText {
		return false
	}
	if c.opts.IgnoreEOL {
		return true
	}
	oldEndings, oldHasEOL := c.old.(lineEndings)
	newEndings, newHasEOL := c.new.(lineEndings)
	if !oldHasEOL || !newHasEOL {
		return true
	}
	oldEOL, newEOL := oldEndings.LineEnding(oldIndex), newEndings.LineEnding(newIndex)
	if oldEOL == newEOL {
		return true
	}
	// When lines are appended to a file without a final terminator, its former
	// last line necessarily gains one. The unchanged text is still the same
	// line; only the following lines are new. Handle the inverse deletion too.
	if oldEOL == "" && oldIndex+1 == c.old.Count() && newIndex+1 < c.new.Count() {
		return true
	}
	if newEOL == "" && newIndex+1 == c.new.Count() && oldIndex+1 < c.old.Count() {
		return true
	}
	return c.opts.IgnoreTrailingEOL && oldIndex+1 == c.old.Count() && newIndex+1 == c.new.Count() &&
		(oldEOL == "" || newEOL == "")
}

func (c lineComparator) findNew(oldIndex, start, end uint64) (uint64, bool) {
	for index := start; index < end; index++ {
		if c.equal(oldIndex, index) {
			return index, true
		}
	}
	return 0, false
}

func (c lineComparator) findOld(newIndex, start, end uint64) (uint64, bool) {
	for index := start; index < end; index++ {
		if c.equal(index, newIndex) {
			return index, true
		}
	}
	return 0, false
}

// normalizer returns the comparison-normalization function for opts, or nil
// when no ignore option is set (the fast path — no wrapping).
func normalizer(o Options) func(string) string {
	if !o.IgnoreCase && o.Whitespace == WSKeep && len(o.LineFilters) == 0 {
		return nil
	}
	return func(s string) string {
		for _, filter := range o.LineFilters {
			s = filter.ReplaceAllString(s, "")
		}
		switch o.Whitespace {
		case WSAll:
			s = removeSpace(s)
		case WSChange:
			s = collapseSpace(s)
		}
		if o.IgnoreCase {
			s = strings.ToLower(s)
		}
		return s
	}
}

// CompileLineFilters validates repeatable --filter-line patterns once, before
// the diff walk. Matching portions are removed only from the comparison view.
func CompileLineFilters(patterns []string) ([]*regexp.Regexp, error) {
	filters := make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		filter, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid line filter %q: %w", pattern, err)
		}
		filters = append(filters, filter)
	}
	return filters, nil
}

// collapseSpace trims leading/trailing whitespace and collapses each internal
// run of whitespace to a single space.
func collapseSpace(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	inRun, started := false, false
	for _, r := range s {
		if unicode.IsSpace(r) {
			inRun = true
			continue
		}
		if inRun && started {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
		started, inRun = true, false
	}
	return b.String()
}

// removeSpace drops all whitespace.
func removeSpace(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if !unicode.IsSpace(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func applyStats(res *Result, h Hunk) {
	switch h.Kind {
	case Insert:
		res.Added += h.NewLen
	case Delete:
		res.Deleted += h.OldLen
	case Replace:
		both := h.OldLen
		if h.NewLen < both {
			both = h.NewLen
		}
		res.Modified += both
		res.Deleted += h.OldLen - both
		res.Added += h.NewLen - both
	}
}
