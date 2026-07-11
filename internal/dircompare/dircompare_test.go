package dircompare

import (
	"os"
	"path/filepath"
	"testing"
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
