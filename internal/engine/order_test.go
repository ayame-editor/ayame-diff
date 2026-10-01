package engine

import (
	"context"
	"path/filepath"
	"testing"
)

// TestSameRowOrderReportsOrder is the #116 row-order check: a no-difference
// key-based comparison cannot tell identical inputs from the same rows in a
// different order, so SameRowOrder reads both once and says which it is.
func TestSameRowOrderReportsOrder(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		left, right string
		hasHeader   bool
		align       bool
		ignoreCase  bool
		want        bool
	}{
		{
			name: "identical order",
			left: "id,name,value\n1,a,10\n2,b,20\n", right: "id,name,value\n1,a,10\n2,b,20\n",
			hasHeader: true, align: true, want: true,
		},
		{
			name: "rows reordered",
			left: "id,name,value\n1,a,10\n2,b,20\n", right: "id,name,value\n2,b,20\n1,a,10\n",
			hasHeader: true, align: true, want: false,
		},
		{
			name: "columns reordered, rows in order",
			left: "id,name,value\n1,a,10\n2,b,20\n", right: "value,id,name\n10,1,a\n20,2,b\n",
			hasHeader: true, align: true, want: true,
		},
		{
			name: "columns and rows reordered",
			left: "id,name,value\n1,a,10\n2,b,20\n", right: "value,id,name\n20,2,b\n10,1,a\n",
			hasHeader: true, align: true, want: false,
		},
		{
			name: "headerless same order",
			left: "1\ta\n2\tb\n", right: "1\ta\n2\tb\n",
			hasHeader: false, want: true,
		},
		{
			name: "headerless reordered",
			left: "1\ta\n2\tb\n", right: "2\tb\n1\ta\n",
			hasHeader: false, want: false,
		},
		{
			name: "ignore case keeps order",
			left: "id,name\n1,ALPHA\n2,beta\n", right: "id,name\n1,alpha\n2,BETA\n",
			hasHeader: true, align: true, ignoreCase: true, want: true,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			extension := ".csv"
			if !tt.hasHeader {
				extension = ".tsv"
			}
			leftPath, rightPath := filepath.Join(dir, "left"+extension), filepath.Join(dir, "right"+extension)
			mustWriteFile(t, leftPath, tt.left)
			mustWriteFile(t, rightPath, tt.right)
			cfg := testConfig(leftPath, rightPath, filepath.Join(dir, "out.tsv"))
			cfg.HasHeader, cfg.AlignColumnsByName = tt.hasHeader, tt.align
			cfg.IgnoreCase = tt.ignoreCase
			cfg.ParseWorkers = 1
			got, err := SameRowOrder(cfg)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("SameRowOrder() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestSameRowOrderAgreesWithRunOrderIndependence guards the premise: Run
// reports no differences for the same rows in a different order, and
// SameRowOrder is what distinguishes that case from an identical input.
func TestSameRowOrderAgreesWithRunOrderIndependence(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	leftPath, rightPath := filepath.Join(dir, "left.csv"), filepath.Join(dir, "right.csv")
	mustWriteFile(t, leftPath, "id,name,value\n1,a,10\n2,b,20\n")
	mustWriteFile(t, rightPath, "id,name,value\n2,b,20\n1,a,10\n")
	cfg := testConfig(leftPath, rightPath, filepath.Join(dir, "out.tsv"))
	summary, err := Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if summary.DiffRows != 0 || summary.EqualRows != 2 {
		t.Fatalf("summary = %+v; the row-reorder premise does not hold", summary)
	}
	if same, err := SameRowOrder(cfg); err != nil || same {
		t.Fatalf("SameRowOrder() = %v, %v; want false, nil", same, err)
	}
}
