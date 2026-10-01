package engine

import "slices"

// VerdictStatus is the one-line conclusion of a CSV comparison: whether the two
// inputs are substantively the same data, and if so, which presentational
// difference (column order, row order) accounts for the inputs not being
// byte-identical (#116).
type VerdictStatus string

const (
	// VerdictEqual means the data agree and the inputs line up as presented.
	VerdictEqual VerdictStatus = "equal"
	// VerdictEqualColumns means the data agree and only the column order differs.
	VerdictEqualColumns VerdictStatus = "equal_columns_reordered"
	// VerdictEqualRows means the data agree and only the row order differs.
	VerdictEqualRows VerdictStatus = "equal_row_order"
	// VerdictEqualPresentation means the data agree but both column and row
	// order differ.
	VerdictEqualPresentation VerdictStatus = "equal_presentation"
	// VerdictDifferences means there are real data differences to resolve.
	VerdictDifferences VerdictStatus = "differences"
	// VerdictColumnsReordered means the comparison was not run because the
	// columns are reordered and name alignment is off; aligning them would let
	// the comparison proceed.
	VerdictColumnsReordered VerdictStatus = "columns_reordered"
)

// EquivalenceVerdict separates real data differences from presentational ones
// (column order, row order) in a completed comparison. It is derived from the
// comparison Summary plus the input inspection, so building it needs no
// re-extraction and no re-read of the data.
type EquivalenceVerdict struct {
	Status VerdictStatus `json:"status"`
	// SubstantivelyEqual is true when no real data difference was found; only
	// presentation may differ.
	SubstantivelyEqual bool `json:"substantively_equal"`
	// Differences is the number of distinct data differences a user must
	// resolve: left-only rows, right-only rows, and changed row pairs.
	Differences uint64 `json:"differences"`
	// ColumnsReordered is true when both inputs have headers but the columns
	// are declared in a different order (or with different names).
	ColumnsReordered bool `json:"columns_reordered"`
	// ColumnSetMatches is true when the two headers name the same set of
	// columns, even if their order differs.
	ColumnSetMatches bool `json:"column_set_matches"`
	// ColumnOrderOnly is true when the only presentational difference is the
	// order of identically named columns.
	ColumnOrderOnly bool `json:"column_order_only"`
	// RowOrderOnly is true when the only presentational difference is the order
	// of rows.
	RowOrderOnly bool `json:"row_order_only"`
	// AlignableColumns is true when enabling column-name alignment would let
	// the comparison run (same column set, different order).
	AlignableColumns bool `json:"alignable_columns"`
}

// realDifferenceCount is the number of distinct differences a comparison
// reports. A changed key pair is emitted once and counts as one difference,
// which is why the two changed sides are folded with max rather than summed.
func realDifferenceCount(summary Summary) uint64 {
	changed := summary.ChangedLeft
	if summary.ChangedRight > changed {
		changed = summary.ChangedRight
	}
	return summary.LeftOnly + summary.RightOnly + changed
}

// sameHeaderSet reports whether the two headers name exactly the same columns,
// regardless of order.
func sameHeaderSet(left, right []string) bool {
	if len(left) != len(right) || len(left) == 0 {
		return false
	}
	leftSorted := append([]string(nil), left...)
	rightSorted := append([]string(nil), right...)
	slices.Sort(leftSorted)
	slices.Sort(rightSorted)
	return slices.Equal(leftSorted, rightSorted)
}

// ColumnsAlignable reports whether the two headers name the same columns in a
// different order, so turning on name alignment would let the comparison run
// without re-extraction.
func ColumnsAlignable(inspection InputInspection) bool {
	return inspection.ColumnSetMatches && inspection.ColumnsReordered
}

// BuildVerdict classifies a completed comparison. sameRowOrder reports whether
// the rows appeared in the same order after column mapping and normalization;
// a caller that did not observe row order passes true so no claim is made.
func BuildVerdict(summary Summary, inspection InputInspection, sameRowOrder bool) EquivalenceVerdict {
	differences := realDifferenceCount(summary)
	alignable := ColumnsAlignable(inspection)
	verdict := EquivalenceVerdict{
		Differences:      differences,
		ColumnsReordered: inspection.ColumnsReordered,
		ColumnSetMatches: inspection.ColumnSetMatches,
		AlignableColumns: alignable,
	}
	if differences > 0 {
		verdict.Status = VerdictDifferences
		return verdict
	}
	verdict.SubstantivelyEqual = true
	verdict.ColumnOrderOnly = alignable
	verdict.RowOrderOnly = !sameRowOrder
	switch {
	case alignable && !sameRowOrder:
		verdict.Status = VerdictEqualPresentation
	case alignable:
		verdict.Status = VerdictEqualColumns
	case !sameRowOrder:
		verdict.Status = VerdictEqualRows
	default:
		verdict.Status = VerdictEqual
	}
	return verdict
}

// ReorderVerdict is the verdict for a comparison the engine declined to run
// because the columns are reordered and name alignment is off. No data has been
// compared yet; the client can enable alignment and run again in one action.
func ReorderVerdict(inspection InputInspection) EquivalenceVerdict {
	return EquivalenceVerdict{
		Status:           VerdictColumnsReordered,
		ColumnsReordered: inspection.ColumnsReordered,
		ColumnSetMatches: inspection.ColumnSetMatches,
		AlignableColumns: ColumnsAlignable(inspection),
	}
}
