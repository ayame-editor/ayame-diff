// Package linediff computes a line-level diff between two line sources using a
// bounded resync window. It never materializes an O(n*m) LCS matrix, so it
// stays linear in the input and memory-bounded on very large files.
//
// This package is a pure core: it depends only on the [Lines] abstraction and
// knows nothing about files, encodings, or output formatting. Output rendering
// (unified / side-by-side / JSON / summary) and file/streaming line sources
// live in separate packages so this core stays easy to test and reuse (e.g.
// from the GUI).
//
// Ported from ayame-editor's crates/ayame-cli/src/diff.rs (see
// hjosugi/ayame-diff#5, ADR 0002).
package linediff

// Kind is the type of a diff hunk.
type Kind uint8

const (
	// Insert marks lines present only in the new input (OldLen == 0).
	Insert Kind = iota
	// Delete marks lines present only in the old input (NewLen == 0).
	Delete
	// Replace marks a run of old lines swapped for a run of new lines.
	Replace
)

// String returns the lowercase name of the kind, matching the reference
// implementation's JSON encoding.
func (k Kind) String() string {
	switch k {
	case Insert:
		return "insert"
	case Delete:
		return "delete"
	case Replace:
		return "replace"
	default:
		return "unknown"
	}
}

// Hunk is one contiguous region of difference. Starts are 0-based line indices;
// lengths are line counts. For Insert, OldLen is 0; for Delete, NewLen is 0.
type Hunk struct {
	Kind     Kind
	OldStart uint64
	OldLen   uint64
	NewStart uint64
	NewLen   uint64
	// MoveID pairs an exact Delete/Insert block detected by DetectMoves.
	// MovePeer is the corresponding 0-based line in the opposite document.
	MoveID   uint64
	MovePeer uint64
}

// Result is the outcome of a diff: the surviving hunks (capped at the caller's
// maxHunks) plus counts that always reflect the full diff even when hunks were
// truncated.
type Result struct {
	OldLines uint64
	NewLines uint64
	// Hunks holds at most maxHunks hunks, in order.
	Hunks []Hunk
	// HunkCount is the total number of hunks, including any omitted.
	HunkCount uint64
	// OmittedHunks is how many hunks were counted but not stored (HunkCount
	// minus len(Hunks)).
	OmittedHunks uint64
	Added        uint64
	Deleted      uint64
	Modified     uint64
	MovedBlocks  uint64
	MovedLines   uint64
	// MoveDetectionSkipped is true when move detection was requested but the
	// retained hunk set was truncated, so a complete answer was impossible.
	MoveDetectionSkipped bool
	IgnoredHunks         uint64
}
