package dircompare

import (
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func write(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func BenchmarkCompareTenThousandFiles(b *testing.B) {
	content := make(map[string]archiveEntry, 10_000)
	for i := 0; i < 10_000; i++ {
		content[fmt.Sprintf("group/%05d.txt", i)] = archiveEntry{data: []byte("same\n")}
	}
	source := archiveSource{content: content}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, err := compareSources(source, source, Options{Workers: 8})
		if err != nil || result.Same != 10_000 {
			b.Fatal(err)
		}
	}
}

func TestFiltersHiddenSymlinksQuickAndGzip(t *testing.T) {
	t.Parallel()
	oldDir, newDir := t.TempDir(), t.TempDir()
	write(t, oldDir, ".hidden.txt", "old")
	write(t, newDir, ".hidden.txt", "new")
	write(t, oldDir, "keep.txt", "aaaa")
	write(t, newDir, "keep.txt", "bbbb")
	write(t, oldDir, "skip.csv", "old")
	write(t, newDir, "skip.csv", "new")
	_ = os.Symlink(filepath.Join(oldDir, "keep.txt"), filepath.Join(oldDir, "link.txt"))
	_ = os.Symlink(filepath.Join(newDir, "keep.txt"), filepath.Join(newDir, "link.txt"))
	stamp := time.Unix(1_700_000_000, 0)
	_ = os.Chtimes(filepath.Join(oldDir, "keep.txt"), stamp, stamp)
	_ = os.Chtimes(filepath.Join(newDir, "keep.txt"), stamp, stamp)
	quick, err := Compare(oldDir, newDir, Options{Includes: []string{"*.txt"}, Quick: true, Workers: 2})
	if err != nil {
		t.Fatal(err)
	}
	if statusOf(quick, "keep.txt") != Same || statusOf(quick, ".hidden.txt") != 255 || statusOf(quick, "link.txt") != 255 || statusOf(quick, "skip.csv") != 255 {
		t.Fatalf("quick=%+v", quick.Entries)
	}
	normal, err := Compare(oldDir, newDir, Options{Includes: []string{"*.txt"}, Workers: 2})
	if err != nil || statusOf(normal, "keep.txt") != Changed {
		t.Fatalf("normal=%+v err=%v", normal, err)
	}

	writeGzip := func(root, content string, level int) {
		file, err := os.Create(filepath.Join(root, "data.txt.gz"))
		if err != nil {
			t.Fatal(err)
		}
		writer, err := gzip.NewWriterLevel(file, level)
		if err != nil {
			t.Fatal(err)
		}
		writer.Header.ModTime = time.Unix(int64(level+10), 0)
		if _, err := writer.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
	}
	writeGzip(oldDir, "same decompressed\n", gzip.BestSpeed)
	writeGzip(newDir, "same decompressed\n", gzip.BestCompression)
	gz, err := Compare(oldDir, newDir, Options{Includes: []string{"*.gz"}})
	if err != nil || statusOf(gz, "data.txt.gz") != Same {
		t.Fatalf("gzip=%+v err=%v", gz, err)
	}
}

func statusOf(res *Result, path string) Status {
	for _, e := range res.Entries {
		if e.Path == path {
			return e.Status
		}
	}
	return 255
}

func TestCompare(t *testing.T) {
	t.Parallel()
	oldDir := t.TempDir()
	newDir := t.TempDir()

	write(t, oldDir, "same.txt", "hello\n")
	write(t, newDir, "same.txt", "hello\n")
	write(t, oldDir, "gone.txt", "bye\n")               // removed
	write(t, newDir, "brandnew.txt", "hi\n")            // added
	write(t, oldDir, "sub/changed.csv", "a,b\n1,2\n")   // changed (content)
	write(t, newDir, "sub/changed.csv", "a,b\n1,3\n")   // ^
	write(t, oldDir, "sizediff.txt", "short\n")         // changed (size)
	write(t, newDir, "sizediff.txt", "much longer!!\n") // ^
	write(t, oldDir, "skip.tmp", "x")                   // excluded
	write(t, newDir, "skip.tmp", "y")

	res, err := Compare(oldDir, newDir, Options{Excludes: []string{"*.tmp"}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Added != 1 || res.Removed != 1 || res.Changed != 2 || res.Same != 1 {
		t.Fatalf("counts: added=%d removed=%d changed=%d same=%d", res.Added, res.Removed, res.Changed, res.Same)
	}
	if statusOf(res, "same.txt") != Same {
		t.Fatal("same.txt should be Same")
	}
	if statusOf(res, "gone.txt") != Removed {
		t.Fatal("gone.txt should be Removed")
	}
	if statusOf(res, "brandnew.txt") != Added {
		t.Fatal("brandnew.txt should be Added")
	}
	if statusOf(res, "sub/changed.csv") != Changed {
		t.Fatal("sub/changed.csv should be Changed")
	}
	if statusOf(res, "sizediff.txt") != Changed {
		t.Fatal("sizediff.txt should be Changed")
	}
	if statusOf(res, "skip.tmp") != 255 {
		t.Fatal("skip.tmp should be excluded")
	}
}
