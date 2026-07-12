package hexdiff

import (
	"os"
	"path/filepath"
	"testing"
)

func tmp(t *testing.T, b []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestIdentical(t *testing.T) {
	t.Parallel()
	a := tmp(t, []byte{1, 2, 3, 4, 5})
	b := tmp(t, []byte{1, 2, 3, 4, 5})
	res, err := Compare(a, b, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Equal || len(res.Regions) != 0 || res.TotalDiffBytes != 0 {
		t.Fatalf("identical: %+v", res)
	}
	if res.OldSize != 5 || res.NewSize != 5 {
		t.Fatalf("sizes %d/%d", res.OldSize, res.NewSize)
	}
}

func TestSingleByteDiff(t *testing.T) {
	t.Parallel()
	a := tmp(t, []byte{0x41, 0x42, 0x43})
	b := tmp(t, []byte{0x41, 0x58, 0x43})
	res, err := Compare(a, b, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Equal || len(res.Regions) != 1 || res.TotalDiffBytes != 1 {
		t.Fatalf("single diff: %+v", res)
	}
	r := res.Regions[0]
	if r.Offset != 1 || len(r.Old) != 1 || r.Old[0] != 0x42 || r.New[0] != 0x58 {
		t.Fatalf("region = %+v", r)
	}
}

func TestSizeMismatchTail(t *testing.T) {
	t.Parallel()
	a := tmp(t, []byte{1, 2, 3})
	b := tmp(t, []byte{1, 2, 3, 9, 9})
	res, err := Compare(a, b, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if res.OldSize != 3 || res.NewSize != 5 || len(res.Regions) != 1 {
		t.Fatalf("size mismatch: %+v", res)
	}
	r := res.Regions[0]
	if r.Offset != 3 || len(r.Old) != 0 || len(r.New) != 2 {
		t.Fatalf("tail region = %+v", r)
	}
}

func TestCoalesce(t *testing.T) {
	t.Parallel()
	// Two diffs 2 bytes apart coalesce (default 16); far apart do not.
	a := tmp(t, []byte{0, 0, 0, 0, 0, 0, 0, 0})
	b := tmp(t, []byte{1, 0, 1, 0, 0, 0, 0, 0})
	res, err := Compare(a, b, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Regions) != 1 {
		t.Fatalf("near diffs should coalesce: %d regions", len(res.Regions))
	}
	if res.Regions[0].Offset != 0 || len(res.Regions[0].Old) != 3 {
		t.Fatalf("coalesced region = %+v", res.Regions[0])
	}

	far := tmp(t, []byte{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1})
	res2, err := Compare(a2(t, 20), far, Options{Coalesce: 4})
	if err != nil {
		t.Fatal(err)
	}
	if len(res2.Regions) != 2 {
		t.Fatalf("far diffs should not coalesce: %d regions", len(res2.Regions))
	}
}

func TestDenseDiffRetainsBoundedBytes(t *testing.T) {
	t.Parallel()
	const size = 1 << 20
	a := tmp(t, make([]byte, size))
	bBytes := make([]byte, size)
	for i := range bBytes {
		bBytes[i] = 0xff
	}
	b := tmp(t, bBytes)

	res, err := Compare(a, b, Options{MaxRegions: 4, MaxRegionBytes: 64})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Truncated || len(res.Regions) != 4 {
		t.Fatalf("truncation = %v, regions = %d", res.Truncated, len(res.Regions))
	}
	retained := 0
	for i, region := range res.Regions {
		if len(region.Old) > 64 || len(region.New) > 64 {
			t.Fatalf("region %d retained %d/%d bytes", i, len(region.Old), len(region.New))
		}
		retained += len(region.Old) + len(region.New)
	}
	if retained > 4*64*2 {
		t.Fatalf("retained %d bytes, want at most %d", retained, 4*64*2)
	}
	if res.TotalDiffBytes != size || res.OldSize != size || res.NewSize != size {
		t.Fatalf("summary = %+v", res)
	}
}

func a2(t *testing.T, n int) string {
	t.Helper()
	return tmp(t, make([]byte, n))
}
