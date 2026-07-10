// Package worddiff computes a token-level ("word") diff between two short
// strings using a classic longest-common-subsequence (LCS) dynamic program.
// It is meant for refining a single changed line into intra-line highlights:
// given the old and new text of a replaced line, it returns the runs of text
// that stayed the same and the runs that changed, on each side.
//
// Ported from ayame-editor's web/src/search.ts inlineWordDiff (see
// hjosugi/ayame-diff#8). The LCS DP is O(m*n) in time and memory, so callers
// must keep inputs small; the MaxChars/MaxTokens guards below enforce that.
package worddiff

import (
	"regexp"
	"unicode/utf8"
)

// Segment is a run of contiguous text on one side of the diff. Changed is true
// when the run has no counterpart on the other side (an insertion or deletion),
// false when it is common to both sides.
type Segment struct {
	Text    string
	Changed bool
}

// Result holds the segmented old and new text. Concatenating the Text of Old
// reproduces oldText; concatenating New reproduces newText. Common (unchanged)
// segments appear in both Old and New in the same order.
type Result struct {
	Old []Segment
	New []Segment
}

const (
	// MaxChars bounds the combined input size. Beyond this the O(m*n) DP is
	// not worth running for an inline highlight, so Diff bails out.
	MaxChars = 2000
	// MaxTokens bounds the combined token count, the real driver of the DP's
	// m*n cost.
	MaxTokens = 260
)

// tokenRE splits text into runs of (1) whitespace, (2) Unicode word characters
// (letters, numbers, or underscore), or (3) any other characters. Splitting on
// these boundaries keeps whole words and whole whitespace gaps as single tokens
// so the diff aligns on word boundaries rather than individual runes. Compiled
// once here to match the reference's single shared RegExp.
var tokenRE = regexp.MustCompile(`(\s+|[\p{L}\p{N}_]+|[^\s\p{L}\p{N}_]+)`)

// Tokenize splits s into diff tokens. An empty string yields no tokens.
func Tokenize(s string) []string {
	return tokenRE.FindAllString(s, -1)
}

// Diff returns the word-level diff of oldText vs newText, or (nil, false) when
// they are equal or exceed the guard limits.
//
// The size guard uses utf8.RuneCountInString rather than len (byte length).
// The reference implementation in TypeScript compares against String.length,
// which counts UTF-16 code units; counting runes here is the closest
// Unicode-correct analogue and keeps the effective limit consistent regardless
// of how many bytes a character encodes to.
func Diff(oldText, newText string) (*Result, bool) {
	if oldText == newText {
		return nil, false
	}
	if utf8.RuneCountInString(oldText)+utf8.RuneCountInString(newText) > MaxChars {
		return nil, false
	}

	oldTokens := Tokenize(oldText)
	newTokens := Tokenize(newText)
	if len(oldTokens)+len(newTokens) > MaxTokens {
		return nil, false
	}

	m, n := len(oldTokens), len(newTokens)

	// dp[i][j] = length of the LCS of oldTokens[i:] and newTokens[j:]. Filled
	// backward so the forward walk below can read the "best from here" value.
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := m - 1; i >= 0; i-- {
		for j := n - 1; j >= 0; j-- {
			if oldTokens[i] == newTokens[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}

	res := &Result{}
	i, j := 0, 0
	for i < m || j < n {
		switch {
		case i < m && j < n && oldTokens[i] == newTokens[j]:
			// Common token: emit unchanged on both sides.
			res.Old = pushDiffPart(res.Old, oldTokens[i], false)
			res.New = pushDiffPart(res.New, newTokens[j], false)
			i++
			j++
		case j >= n || (i < m && dp[i+1][j] >= dp[i][j+1]):
			// Advancing in old loses no more of the LCS than advancing in new
			// (ties favor consuming old): this old token was deleted.
			res.Old = pushDiffPart(res.Old, oldTokens[i], true)
			i++
		default:
			// Otherwise this new token was inserted.
			res.New = pushDiffPart(res.New, newTokens[j], true)
			j++
		}
	}
	return res, true
}

// pushDiffPart appends text to parts, merging it into the last segment when the
// Changed flag matches so adjacent runs of the same kind form one segment.
// Empty text is never pushed. Mirrors the reference's pushDiffPart.
func pushDiffPart(parts []Segment, text string, changed bool) []Segment {
	if text == "" {
		return parts
	}
	if n := len(parts); n > 0 && parts[n-1].Changed == changed {
		parts[n-1].Text += text
		return parts
	}
	return append(parts, Segment{Text: text, Changed: changed})
}
