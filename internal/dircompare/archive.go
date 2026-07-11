package dircompare

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// source is a comparable tree of files with content, backed by either a
// directory on disk or an archive read into memory.
type source interface {
	// files returns rel-slash-path -> size for the regular files.
	files() (map[string]int64, error)
	// open returns the content of one file.
	open(rel string) (io.ReadCloser, error)
}

// dirSource walks a directory on disk (content is streamed, not buffered).
type dirSource struct {
	root     string
	excludes []string
}

func (d dirSource) files() (map[string]int64, error) { return walk(d.root, d.excludes) }
func (d dirSource) open(rel string) (io.ReadCloser, error) {
	return os.Open(filepath.Join(d.root, filepath.FromSlash(rel)))
}

// archiveSource holds an archive's entries in memory (uncompressed).
type archiveSource struct {
	content map[string][]byte
}

func (a archiveSource) files() (map[string]int64, error) {
	m := make(map[string]int64, len(a.content))
	for k, v := range a.content {
		m[k] = int64(len(v))
	}
	return m, nil
}
func (a archiveSource) open(rel string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(a.content[rel])), nil
}

// IsArchive reports whether path names a supported archive.
func IsArchive(path string) bool {
	p := strings.ToLower(path)
	return strings.HasSuffix(p, ".zip") || strings.HasSuffix(p, ".tar") ||
		strings.HasSuffix(p, ".tar.gz") || strings.HasSuffix(p, ".tgz")
}

func makeSource(path string, excludes []string) (source, error) {
	if IsArchive(path) {
		return loadArchive(path, excludes)
	}
	return dirSource{root: path, excludes: excludes}, nil
}

// loadArchive reads a .zip / .tar / .tar.gz / .tgz into an archiveSource,
// honoring the exclude patterns. Contents are held in memory.
func loadArchive(path string, excludes []string) (source, error) {
	content := map[string][]byte{}
	lower := strings.ToLower(path)

	if strings.HasSuffix(lower, ".zip") {
		zr, err := zip.OpenReader(path)
		if err != nil {
			return nil, err
		}
		defer zr.Close()
		for _, f := range zr.File {
			if f.FileInfo().IsDir() {
				continue
			}
			rel := filepath.ToSlash(f.Name)
			if excluded(rel, filepath.Base(rel), excludes) {
				continue
			}
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			b, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return nil, err
			}
			content[rel] = b
		}
		return archiveSource{content}, nil
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var r io.Reader = f
	if strings.HasSuffix(lower, ".gz") || strings.HasSuffix(lower, ".tgz") {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		r = gz
	}
	tr := tar.NewReader(r)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if h.Typeflag != tar.TypeReg {
			continue
		}
		rel := filepath.ToSlash(h.Name)
		if excluded(rel, filepath.Base(rel), excludes) {
			continue
		}
		b, err := io.ReadAll(tr)
		if err != nil {
			return nil, err
		}
		content[rel] = b
	}
	return archiveSource{content}, nil
}

// CompareAny compares two paths, each a directory or a supported archive
// (.zip / .tar / .tar.gz / .tgz), so archives compare like folders. Archive
// contents are read into memory.
func CompareAny(oldPath, newPath string, opts Options) (*Result, error) {
	oldSrc, err := makeSource(oldPath, opts.Excludes)
	if err != nil {
		return nil, err
	}
	newSrc, err := makeSource(newPath, opts.Excludes)
	if err != nil {
		return nil, err
	}
	return compareSources(oldSrc, newSrc)
}

// compareSources classifies every file across the two sources by content.
func compareSources(oldSrc, newSrc source) (*Result, error) {
	oldFiles, err := oldSrc.files()
	if err != nil {
		return nil, err
	}
	newFiles, err := newSrc.files()
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool, len(oldFiles)+len(newFiles))
	var rels []string
	for r := range oldFiles {
		if !seen[r] {
			seen[r] = true
			rels = append(rels, r)
		}
	}
	for r := range newFiles {
		if !seen[r] {
			seen[r] = true
			rels = append(rels, r)
		}
	}
	sort.Strings(rels)

	res := &Result{}
	for _, rel := range rels {
		oldSize, inOld := oldFiles[rel]
		newSize, inNew := newFiles[rel]
		e := Entry{Path: rel, OldSize: -1, NewSize: -1}
		switch {
		case inOld && !inNew:
			e.Status, e.OldSize = Removed, oldSize
		case !inOld && inNew:
			e.Status, e.NewSize = Added, newSize
		default:
			e.OldSize, e.NewSize = oldSize, newSize
			equal := oldSize == newSize
			if equal {
				equal, err = contentEqual(oldSrc, newSrc, rel)
				if err != nil {
					return nil, err
				}
			}
			if equal {
				e.Status = Same
			} else {
				e.Status = Changed
			}
		}
		switch e.Status {
		case Added:
			res.Added++
		case Removed:
			res.Removed++
		case Changed:
			res.Changed++
		default:
			res.Same++
		}
		res.Entries = append(res.Entries, e)
	}
	return res, nil
}

// contentEqual streams both sources' copies of rel and compares their bytes.
func contentEqual(oldSrc, newSrc source, rel string) (bool, error) {
	a, err := oldSrc.open(rel)
	if err != nil {
		return false, err
	}
	defer a.Close()
	b, err := newSrc.open(rel)
	if err != nil {
		return false, err
	}
	defer b.Close()
	return readersEqual(a, b)
}

// readersEqual reports whether two readers yield identical bytes, short-
// circuiting on the first difference.
func readersEqual(a, b io.Reader) (bool, error) {
	const bufSize = 64 * 1024
	ra := bufio.NewReaderSize(a, bufSize)
	rb := bufio.NewReaderSize(b, bufSize)
	bufA := make([]byte, bufSize)
	bufB := make([]byte, bufSize)
	for {
		na, errA := io.ReadFull(ra, bufA)
		nb, errB := io.ReadFull(rb, bufB)
		if na != nb || !bytes.Equal(bufA[:na], bufB[:nb]) {
			return false, nil
		}
		aDone := errA == io.EOF || errA == io.ErrUnexpectedEOF
		bDone := errB == io.EOF || errB == io.ErrUnexpectedEOF
		if aDone || bDone {
			if aDone != bDone {
				return false, nil
			}
			return true, nil
		}
		if errA != nil {
			return false, errA
		}
		if errB != nil {
			return false, errB
		}
	}
}
