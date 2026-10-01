package engine

import "testing"

// TestBuildVerdictClassifiesEquivalence pins the #116 verdict: a key-based
// comparison is row-order independent, so a no-difference result must say
// whether the only difference left is presentation (column order, row order)
// rather than making the reader interpret the counts.
func TestBuildVerdictClassifiesEquivalence(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		summary        Summary
		inspection     InputInspection
		sameRowOrder   bool
		wantStatus     VerdictStatus
		wantEqual      bool
		wantDiffs      uint64
		wantColumnOnly bool
		wantRowOnly    bool
		wantAlignable  bool
	}{
		{
			name:         "identical",
			inspection:   InputInspection{HasHeader: true, ColumnsReordered: false, ColumnSetMatches: true},
			sameRowOrder: true,
			wantStatus:   VerdictEqual, wantEqual: true,
		},
		{
			name:         "real differences",
			summary:      Summary{LeftOnly: 2, RightOnly: 1, ChangedLeft: 3, ChangedRight: 3},
			inspection:   InputInspection{HasHeader: true, ColumnSetMatches: true},
			sameRowOrder: true,
			wantStatus:   VerdictDifferences, wantEqual: false, wantDiffs: 6,
		},
		{
			name:         "column order differs only",
			inspection:   InputInspection{HasHeader: true, ColumnsReordered: true, ColumnSetMatches: true},
			sameRowOrder: true,
			wantStatus:   VerdictEqualColumns, wantEqual: true, wantColumnOnly: true, wantAlignable: true,
		},
		{
			name:         "row order differs only",
			inspection:   InputInspection{HasHeader: true, ColumnsReordered: false, ColumnSetMatches: true},
			sameRowOrder: false,
			wantStatus:   VerdictEqualRows, wantEqual: true, wantRowOnly: true,
		},
		{
			name:         "column and row order differ",
			inspection:   InputInspection{HasHeader: true, ColumnsReordered: true, ColumnSetMatches: true},
			sameRowOrder: false,
			wantStatus:   VerdictEqualPresentation, wantEqual: true, wantColumnOnly: true, wantRowOnly: true, wantAlignable: true,
		},
		{
			name:         "changed pair counts once",
			summary:      Summary{ChangedLeft: 1, ChangedRight: 1, DiffRows: 2},
			inspection:   InputInspection{HasHeader: true, ColumnSetMatches: true},
			sameRowOrder: true,
			wantStatus:   VerdictDifferences, wantDiffs: 1,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := BuildVerdict(tt.summary, tt.inspection, tt.sameRowOrder)
			if got.Status != tt.wantStatus || got.SubstantivelyEqual != tt.wantEqual || got.Differences != tt.wantDiffs ||
				got.ColumnOrderOnly != tt.wantColumnOnly || got.RowOrderOnly != tt.wantRowOnly || got.AlignableColumns != tt.wantAlignable {
				t.Fatalf("BuildVerdict() = %+v; want status=%q equal=%v diffs=%d columnOnly=%v rowOnly=%v alignable=%v",
					got, tt.wantStatus, tt.wantEqual, tt.wantDiffs, tt.wantColumnOnly, tt.wantRowOnly, tt.wantAlignable)
			}
		})
	}
}

func TestReorderVerdictOffersAlignment(t *testing.T) {
	t.Parallel()
	got := ReorderVerdict(InputInspection{HasHeader: true, ColumnsReordered: true, ColumnSetMatches: true})
	if got.Status != VerdictColumnsReordered || got.SubstantivelyEqual || !got.AlignableColumns {
		t.Fatalf("ReorderVerdict() = %+v", got)
	}
	if !ColumnsAlignable(InputInspection{ColumnsReordered: true, ColumnSetMatches: true}) {
		t.Fatal("ColumnsAlignable should accept a same-set reorder")
	}
	if ColumnsAlignable(InputInspection{ColumnsReordered: true, ColumnSetMatches: false}) {
		t.Fatal("ColumnsAlignable must reject a renamed or mismatched column set")
	}
}

func TestSameHeaderSet(t *testing.T) {
	t.Parallel()
	if !sameHeaderSet([]string{"id", "name", "value"}, []string{"value", "id", "name"}) {
		t.Fatal("reordered same set not detected")
	}
	if sameHeaderSet([]string{"id", "name"}, []string{"id", "title"}) {
		t.Fatal("different names treated as the same set")
	}
	if sameHeaderSet([]string{"id"}, []string{"id", "name"}) {
		t.Fatal("different lengths treated as the same set")
	}
}
