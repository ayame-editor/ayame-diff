// Package hexdiff compares two files byte-for-byte and reports the differing
// regions, for a binary/hex diff mode (WinMerge binary compare —
// hjosugi/ayame-diff#57). It streams both files, so it stays memory-bounded on
// large inputs; only the (capped) differing regions are held.
package hexdiff

import (
	"bufio"
	"io"
	"os"
)

// Region is a run of differing bytes at Offset. Old/New hold the bytes from
// each file (either may be shorter when one file ends first).
type Region struct {
	Offset int64
	Old    []byte
	New    []byte
}

// Result summarizes a binary comparison.
type Result struct {
	Regions        []Region
	OldSize        int64
	NewSize        int64
	Equal          bool
	TotalDiffBytes int64
	Truncated      bool // more regions existed than MaxRegions
}

// Options tunes the comparison.
type Options struct {
	// MaxRegions caps how many differing regions are kept (0 => 256).
	MaxRegions int
	// MaxRegionBytes caps the bytes retained on either side of one region
	// (0 => 32). Dense differences are split into bounded regions, so retained
	// data never grows with the input file size.
	MaxRegionBytes int
	// Coalesce merges differing runs separated by fewer than this many equal
	// bytes into one region, so a scattered edit reads as one block (0 => 16).
	Coalesce int
}

// Compare reads both files and returns their differing regions.
func Compare(oldPath, newPath string, opts Options) (*Result, error) {
	if opts.MaxRegions <= 0 {
		opts.MaxRegions = 256
	}
	if opts.MaxRegionBytes <= 0 {
		opts.MaxRegionBytes = 32
	}
	if opts.Coalesce <= 0 {
		opts.Coalesce = 16
	}
	fa, err := os.Open(oldPath)
	if err != nil {
		return nil, err
	}
	defer fa.Close()
	fb, err := os.Open(newPath)
	if err != nil {
		return nil, err
	}
	defer fb.Close()
	ra := bufio.NewReaderSize(fa, 64*1024)
	rb := bufio.NewReaderSize(fb, 64*1024)

	res := &Result{}
	var offset int64
	var cur *Region // region being accumulated
	var gap int     // equal bytes seen since the current region's last diff
	for {
		ba, ea := ra.ReadByte()
		bb, eb := rb.ReadByte()
		aEnd := ea == io.EOF
		bEnd := eb == io.EOF
		if aEnd && bEnd {
			break
		}
		if ea != nil && !aEnd {
			return nil, ea
		}
		if eb != nil && !bEnd {
			return nil, eb
		}

		differ := aEnd || bEnd || ba != bb
		if differ {
			res.TotalDiffBytes++
			if cur != nil && (gap > opts.Coalesce || regionBytes(cur) >= opts.MaxRegionBytes) {
				res.finish(cur)
				cur = nil
			}
			if cur == nil {
				if len(res.Regions) >= opts.MaxRegions {
					res.Truncated = true
				} else {
					cur = &Region{Offset: offset}
				}
			}
			if cur != nil && !aEnd {
				cur.Old = append(cur.Old, ba)
			}
			if cur != nil && !bEnd {
				cur.New = append(cur.New, bb)
			}
			gap = 0
		} else if cur != nil {
			if regionBytes(cur) >= opts.MaxRegionBytes {
				res.finish(cur)
				cur = nil
				gap = 0
			} else {
				// equal byte inside/after a region: extend both sides so the region
				// stays contiguous until Coalesce equal bytes end it.
				cur.Old = append(cur.Old, ba)
				cur.New = append(cur.New, bb)
				gap++
			}
		}

		if !aEnd {
			res.OldSize++
		}
		if !bEnd {
			res.NewSize++
		}
		offset++
	}
	if cur != nil {
		res.finish(cur)
	}
	res.Equal = res.TotalDiffBytes == 0 && res.OldSize == res.NewSize
	return res, nil
}

func regionBytes(reg *Region) int {
	return max(len(reg.Old), len(reg.New))
}

// finish trims trailing equal bytes (from Coalesce look-ahead) off a region and
// appends it.
func (r *Result) finish(reg *Region) {
	// Trailing equal bytes were appended to both sides equally; trim the common
	// equal suffix so the region ends on the last real difference.
	for len(reg.Old) > 0 && len(reg.New) > 0 &&
		reg.Old[len(reg.Old)-1] == reg.New[len(reg.New)-1] {
		reg.Old = reg.Old[:len(reg.Old)-1]
		reg.New = reg.New[:len(reg.New)-1]
	}
	r.Regions = append(r.Regions, *reg)
}
