package linediff

import "strings"

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

// Diff computes the line diff of old vs new. At most maxHunks hunks are stored
// in the result; any beyond that are still counted (HunkCount / OmittedHunks /
// stats). window bounds how far the resync scan looks ahead when it hits a
// difference: a larger window recovers cleaner insert/delete hunks across
// bigger gaps, at the cost of a longer scan. window is clamped to at least 1.
func Diff(old, new Lines, maxHunks int, window uint64) Result {
	if window < 1 {
		window = 1
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
