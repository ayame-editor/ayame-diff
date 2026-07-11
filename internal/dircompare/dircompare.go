// Package dircompare recursively compares two directory trees and reports which
// files were added, removed, or changed between them (WinMerge-style folder
// comparison — hjosugi/ayame-diff#52). It compares file content, not just
// timestamps, so it is reliable across copies and checkouts.
package dircompare

import (
	"bufio"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
)

// Status is a file's state between the two trees.
type Status uint8

const (
	Same    Status = iota // present in both, identical content
	Added                 // only in the new tree
	Removed               // only in the old tree
	Changed               // present in both, content differs
)

func (s Status) String() string {
	switch s {
	case Added:
		return "added"
	case Removed:
		return "removed"
	case Changed:
		return "changed"
	default:
		return "same"
	}
}

// Entry is one file's comparison result. Path is relative and slash-separated.
type Entry struct {
	Path    string
	Status  Status
	OldSize int64
	NewSize int64
}

// Options tunes the comparison.
type Options struct {
	// Excludes are glob patterns (path.Match syntax) tested against each file's
	// relative path and base name; a match skips the file, and a matching
	// directory is not descended.
	Excludes []string
}

// Result holds every file entry (sorted by path, including Same) plus counts.
type Result struct {
	Entries                       []Entry
	Added, Removed, Changed, Same int
}

// Compare walks oldDir and newDir and classifies every file.
func Compare(oldDir, newDir string, opts Options) (*Result, error) {
	oldFiles, err := walk(oldDir, opts.Excludes)
	if err != nil {
		return nil, err
	}
	newFiles, err := walk(newDir, opts.Excludes)
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
				equal, err = filesEqual(filepath.Join(oldDir, filepath.FromSlash(rel)), filepath.Join(newDir, filepath.FromSlash(rel)))
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

// walk returns a map of relative slash-path -> size for the regular files under
// root, honoring the exclude patterns.
func walk(root string, excludes []string) (map[string]int64, error) {
	files := make(map[string]int64)
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(root, p)
		if relErr != nil {
			return relErr
		}
		if rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if excluded(rel, d.Name(), excludes) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		info, infoErr := d.Info()
		if infoErr != nil {
			return infoErr
		}
		if info.Mode().IsRegular() {
			files[rel] = info.Size()
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

func excluded(rel, base string, patterns []string) bool {
	for _, pat := range patterns {
		if ok, _ := path.Match(pat, rel); ok {
			return true
		}
		if ok, _ := path.Match(pat, base); ok {
			return true
		}
	}
	return false
}

// filesEqual streams both files and reports whether their bytes are identical.
// It short-circuits on the first difference.
func filesEqual(a, b string) (bool, error) {
	fa, err := os.Open(a)
	if err != nil {
		return false, err
	}
	defer fa.Close()
	fb, err := os.Open(b)
	if err != nil {
		return false, err
	}
	defer fb.Close()

	const bufSize = 64 * 1024
	ra := bufio.NewReaderSize(fa, bufSize)
	rb := bufio.NewReaderSize(fb, bufSize)
	bufA := make([]byte, bufSize)
	bufB := make([]byte, bufSize)
	for {
		na, errA := io.ReadFull(ra, bufA)
		nb, errB := io.ReadFull(rb, bufB)
		if na != nb {
			return false, nil
		}
		if !equalBytes(bufA[:na], bufB[:nb]) {
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

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
