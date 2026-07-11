package dircompare

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func makeZip(t *testing.T, files map[string]string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "a.zip")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(content))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return p
}

func makeTarGz(t *testing.T, files map[string]string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "a.tar.gz")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(content)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		tw.Write([]byte(content))
	}
	tw.Close()
	gz.Close()
	return p
}

func TestCompareArchives(t *testing.T) {
	t.Parallel()
	oldZip := makeZip(t, map[string]string{
		"same.txt":    "hello\n",
		"changed.csv": "a,b\n1,2\n",
		"onlyold.txt": "x\n",
	})
	newTar := makeTarGz(t, map[string]string{
		"same.txt":    "hello\n",
		"changed.csv": "a,b\n1,3\n",
		"onlynew.txt": "y\n",
	})

	// zip vs tar.gz, compared like folders.
	res, err := CompareAny(oldZip, newTar, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Same != 1 || res.Changed != 1 || res.Removed != 1 || res.Added != 1 {
		t.Fatalf("counts: same=%d changed=%d removed=%d added=%d", res.Same, res.Changed, res.Removed, res.Added)
	}
	if statusOf(res, "same.txt") != Same || statusOf(res, "changed.csv") != Changed ||
		statusOf(res, "onlyold.txt") != Removed || statusOf(res, "onlynew.txt") != Added {
		t.Fatalf("statuses = %+v", res.Entries)
	}
}

func TestCompareArchiveVsDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	write(t, dir, "a.txt", "one\n")
	write(t, dir, "sub/b.txt", "two\n")
	z := makeZip(t, map[string]string{"a.txt": "one\n", "sub/b.txt": "changed\n"})

	res, err := CompareAny(dir, z, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if statusOf(res, "a.txt") != Same || statusOf(res, "sub/b.txt") != Changed {
		t.Fatalf("archive-vs-dir statuses = %+v", res.Entries)
	}
}

func TestIsArchive(t *testing.T) {
	t.Parallel()
	for _, p := range []string{"x.zip", "X.ZIP", "a.tar", "a.tar.gz", "a.tgz"} {
		if !IsArchive(p) {
			t.Fatalf("%s should be an archive", p)
		}
	}
	for _, p := range []string{"dir", "a.txt", "a.gz"} {
		if IsArchive(p) {
			t.Fatalf("%s should not be an archive", p)
		}
	}
}
