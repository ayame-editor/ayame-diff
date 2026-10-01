package e2e

import "testing"

// The benchmarks below run the four large-input shapes from #279 end to end.
// They default to a size CI can finish in a few minutes; set AYAME_BENCH_LARGE=1
// for the sizes named in the issue (10M-row CSV, ~1 GiB text, 100k-file tree,
// 100 MiB single line). Each size also takes an explicit override:
//
//	AYAME_BENCH_CSV_ROWS, AYAME_BENCH_TEXT_BYTES,
//	AYAME_BENCH_FOLDER_FILES, AYAME_BENCH_LINE_BYTES
//
// Run them with:
//
//	go test -run '^$' -bench . -benchmem ./internal/e2e
func BenchmarkEndToEndCSV(b *testing.B) {
	rows := scaleInt(b, "AYAME_BENCH_CSV_ROWS", ciCSVRows, 10_000_000)
	c := newCSVCase(b, rows, 1000)
	b.ReportAllocs()
	b.SetBytes(fileBytes(b, c.left, c.right))
	b.ReportMetric(float64(rows), "rows/op")
	b.ResetTimer()
	for range b.N {
		if summary, _ := c.run(b); summary.DiffRows == 0 {
			b.Fatal("csv benchmark produced no differences")
		}
	}
}

func BenchmarkEndToEndText(b *testing.B) {
	target := scaleInt(b, "AYAME_BENCH_TEXT_BYTES", ciTextBytes, 1<<30)
	c := newTextCase(b, target, 1000)
	b.ReportAllocs()
	b.SetBytes(fileBytes(b, c.left, c.right))
	b.ResetTimer()
	for range b.N {
		if res, _, err := c.run(b, 0, 512); err != nil || res.HunkCount == 0 {
			b.Fatalf("text benchmark: hunks=%d err=%v", res.HunkCount, err)
		}
	}
}

func BenchmarkEndToEndFolderTree(b *testing.B) {
	files := scaleInt(b, "AYAME_BENCH_FOLDER_FILES", ciFolderFiles, 100_000)
	c := newFolderCase(b, files, 10)
	b.ReportAllocs()
	b.ReportMetric(float64(files), "files/op")
	b.ResetTimer()
	for range b.N {
		if res, _, err := c.run(b, 0); err != nil || res.Changed == 0 {
			b.Fatalf("folder benchmark: changed=%d err=%v", res.Changed, err)
		}
	}
}

func BenchmarkEndToEndSingleLongLine(b *testing.B) {
	size := scaleInt(b, "AYAME_BENCH_LINE_BYTES", ciLineBytes, 100<<20)
	c := newLongLineCase(b, size)
	b.ReportAllocs()
	b.SetBytes(fileBytes(b, c.left, c.right))
	b.ResetTimer()
	for range b.N {
		// maxLine = size models a user who raised --max-line-bytes to compare
		// this line; the default cap refuses it explicitly (see the
		// degradation test).
		if res, _, err := c.run(b, size); err != nil || res.HunkCount != 1 {
			b.Fatalf("long-line benchmark: hunks=%d err=%v", res.HunkCount, err)
		}
	}
}
