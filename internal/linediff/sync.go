package linediff

import (
	"context"
	"fmt"
)

// SyncPoint forces the old/new lines at these 0-based indexes to correspond.
// Diff is computed independently before, at, and after every point.
type SyncPoint struct {
	Old uint64 `json:"old"`
	New uint64 `json:"new"`
}

func ValidateSyncPoints(points []SyncPoint, oldLines, newLines uint64) error {
	var previous SyncPoint
	for i, point := range points {
		if point.Old >= oldLines || point.New >= newLines {
			return fmt.Errorf("sync point %d (%d:%d) is outside document bounds", i+1, point.Old+1, point.New+1)
		}
		if i > 0 && (point.Old <= previous.Old || point.New <= previous.New) {
			return fmt.Errorf("sync points must be strictly increasing on both sides")
		}
		previous = point
	}
	return nil
}

type rangeLines struct {
	Lines
	start, end uint64
}

func (r rangeLines) Count() uint64 { return r.end - r.start }
func (r rangeLines) Line(index uint64) (string, bool) {
	if index >= r.Count() {
		return "", false
	}
	return r.Lines.Line(r.start + index)
}

func (r rangeLines) LineEnding(index uint64) string {
	if index >= r.Count() {
		return ""
	}
	if endings, ok := r.Lines.(lineEndings); ok {
		return endings.LineEnding(r.start + index)
	}
	return ""
}

func diffWithSyncPoints(ctx context.Context, old, new Lines, opts Options) (Result, error) {
	result := Result{OldLines: old.Count(), NewLines: new.Count()}
	points := opts.SyncPoints
	opts.SyncPoints = nil
	oldStart, newStart := uint64(0), uint64(0)
	for _, point := range points {
		segment, err := diffWithOptions(ctx,
			rangeLines{Lines: old, start: oldStart, end: point.Old},
			rangeLines{Lines: new, start: newStart, end: point.New}, opts)
		if err != nil {
			return result, err
		}
		mergeSyncSegment(&result, segment, oldStart, newStart, opts.MaxHunks)
		// Diff the forced pair alone: differing anchor text remains a Replace,
		// but the resync search cannot cross this user-specified boundary.
		anchor, err := diffWithOptions(ctx,
			rangeLines{Lines: old, start: point.Old, end: point.Old + 1},
			rangeLines{Lines: new, start: point.New, end: point.New + 1}, opts)
		if err != nil {
			return result, err
		}
		mergeSyncSegment(&result, anchor, point.Old, point.New, opts.MaxHunks)
		oldStart, newStart = point.Old+1, point.New+1
	}
	tail, err := diffWithOptions(ctx,
		rangeLines{Lines: old, start: oldStart, end: old.Count()},
		rangeLines{Lines: new, start: newStart, end: new.Count()}, opts)
	if err != nil {
		return result, err
	}
	mergeSyncSegment(&result, tail, oldStart, newStart, opts.MaxHunks)
	result.OmittedHunks = result.HunkCount - uint64(len(result.Hunks))
	return result, nil
}

func mergeSyncSegment(dst *Result, segment Result, oldOffset, newOffset uint64, maxHunks int) {
	dst.HunkCount += segment.HunkCount
	dst.Added += segment.Added
	dst.Deleted += segment.Deleted
	dst.Modified += segment.Modified
	for _, hunk := range segment.Hunks {
		if len(dst.Hunks) >= maxHunks {
			break
		}
		hunk.OldStart += oldOffset
		hunk.NewStart += newOffset
		dst.Hunks = append(dst.Hunks, hunk)
	}
}

// IgnoreHunks removes selected stored hunks from output/statistics. A moved
// pair is ignored together so no dangling move annotation remains.
func IgnoreHunks(result *Result, indexes []int) {
	if result == nil || len(indexes) == 0 {
		return
	}
	ignored := make(map[int]bool, len(indexes))
	moveIDs := make(map[uint64]bool)
	for _, index := range indexes {
		if index >= 0 && index < len(result.Hunks) {
			ignored[index] = true
			if id := result.Hunks[index].MoveID; id != 0 {
				moveIDs[id] = true
			}
		}
	}
	for i, hunk := range result.Hunks {
		if moveIDs[hunk.MoveID] && hunk.MoveID != 0 {
			ignored[i] = true
		}
	}
	kept := result.Hunks[:0]
	seenMoves := make(map[uint64]bool)
	for i, hunk := range result.Hunks {
		if !ignored[i] {
			kept = append(kept, hunk)
			continue
		}
		result.IgnoredHunks++
		if result.HunkCount > 0 {
			result.HunkCount--
		}
		if hunk.MoveID != 0 {
			if !seenMoves[hunk.MoveID] {
				seenMoves[hunk.MoveID] = true
				if result.MovedBlocks > 0 {
					result.MovedBlocks--
				}
				if result.MovedLines >= max(hunk.OldLen, hunk.NewLen) {
					result.MovedLines -= max(hunk.OldLen, hunk.NewLen)
				}
			}
			continue
		}
		subtractStats(result, hunk)
	}
	result.Hunks = kept
}

func subtractStats(result *Result, hunk Hunk) {
	switch hunk.Kind {
	case Insert:
		result.Added -= min(result.Added, hunk.NewLen)
	case Delete:
		result.Deleted -= min(result.Deleted, hunk.OldLen)
	case Replace:
		both := min(hunk.OldLen, hunk.NewLen)
		result.Modified -= min(result.Modified, both)
		result.Deleted -= min(result.Deleted, hunk.OldLen-both)
		result.Added -= min(result.Added, hunk.NewLen-both)
	}
}
