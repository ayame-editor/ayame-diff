package linediff

import (
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

// SplitLines splits s into lines the same way the reference implementation's
// document reader does: on "\n", with a trailing "\r" trimmed from each line
// (CRLF) and a single trailing newline not producing an extra empty line.
// An empty string yields zero lines.
func SplitLines(s string) StringLines {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, "\n")
	if n := len(parts); n > 0 && parts[n-1] == "" {
		parts = parts[:n-1]
	}
	for i := range parts {
		parts[i] = strings.TrimSuffix(parts[i], "\r")
	}
	return StringLines(parts)
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
	MaxHunks   int
	Window     uint64
	IgnoreCase bool
	Whitespace Whitespace
}

// Diff computes the line diff of old vs new. At most maxHunks hunks are stored
// in the result; any beyond that are still counted (HunkCount / OmittedHunks /
// stats). window bounds how far the resync scan looks ahead when it hits a
// difference. It is Diff­With with no ignore options.
func Diff(old, new Lines, maxHunks int, window uint64) Result {
	return DiffWith(old, new, Options{MaxHunks: maxHunks, Window: window})
}

// DiffWith is Diff with comparison options. When any ignore option is set, the
// comparison runs over a normalized view of each line while positions (and thus
// the output, rendered from the caller's originals) are unchanged.
func DiffWith(old, new Lines, opts Options) Result {
	window := opts.Window
	if window < 1 {
		window = 1
	}
	maxHunks := opts.MaxHunks
	if norm := normalizer(opts); norm != nil {
		old = normLines{Lines: old, norm: norm}
		new = normLines{Lines: new, norm: norm}
	}
	oldTotal := old.Count()
	newTotal := new.Count()
	res := Result{OldLines: oldTotal, NewLines: newTotal}

	var i, j uint64
	for i < oldTotal || j < newTotal {
		if i < oldTotal && j < newTotal {
			ol, _ := old.Line(i)
			nl, _ := new.Line(j)
			if ol == nl {
				i++
				j++
				continue
			}
		}

		h := nextDiffHunk(old, new, i, j, window)
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

func nextDiffHunk(old, new Lines, i, j, window uint64) Hunk {
	oldTotal := old.Count()
	newTotal := new.Count()
	if i >= oldTotal {
		return Hunk{Kind: Insert, OldStart: i, OldLen: 0, NewStart: j, NewLen: newTotal - j}
	}
	if j >= newTotal {
		return Hunk{Kind: Delete, OldStart: i, OldLen: oldTotal - i, NewStart: j, NewLen: 0}
	}

	// Anchor lines: the resync scans below read ahead in each source.
	oldLine, _ := old.Line(i)
	newLine, _ := new.Line(j)
	// Insert: the old anchor reappears in new within the window -> the skipped
	// new lines were inserted. Delete: the new anchor reappears in old -> the
	// skipped old lines were deleted.
	rj, insOK := findLine(new, oldLine, j+1, clampEnd(j+1, window, newTotal))
	li, delOK := findLine(old, newLine, i+1, clampEnd(i+1, window, oldTotal))

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

func findLine(lines Lines, target string, start, end uint64) (uint64, bool) {
	for n := start; n < end; n++ {
		if s, ok := lines.Line(n); ok && s == target {
			return n, true
		}
	}
	return 0, false
}

func insertHunk(oldStart, newStart, newLen uint64) Hunk {
	return Hunk{Kind: Insert, OldStart: oldStart, OldLen: 0, NewStart: newStart, NewLen: newLen}
}

func deleteHunk(oldStart, newStart, oldLen uint64) Hunk {
	return Hunk{Kind: Delete, OldStart: oldStart, OldLen: oldLen, NewStart: newStart, NewLen: 0}
}

// normLines is a Lines whose Line returns a normalized form (for comparison);
// its Count and line positions are those of the underlying source.
type normLines struct {
	Lines
	norm func(string) string
}

func (n normLines) Line(i uint64) (string, bool) {
	s, ok := n.Lines.Line(i)
	if !ok {
		return "", false
	}
	return n.norm(s), true
}

// normalizer returns the comparison-normalization function for opts, or nil
// when no ignore option is set (the fast path — no wrapping).
func normalizer(o Options) func(string) string {
	if !o.IgnoreCase && o.Whitespace == WSKeep {
		return nil
	}
	return func(s string) string {
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
