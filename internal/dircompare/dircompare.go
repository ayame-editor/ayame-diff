// Package dircompare recursively compares two directory trees and reports which
// files were added, removed, or changed between them (WinMerge-style folder
// comparison — hjosugi/ayame-diff#52). It compares file content, not just
// timestamps, so it is reliable across copies and checkouts.
package dircompare

import (
	"io/fs"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type fileMeta struct {
	Size    int64
	ModTime time.Time
}

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
	Path       string
	Status     Status
	OldSize    int64
	NewSize    int64
	OldModTime time.Time
	NewModTime time.Time
}

// Options tunes the comparison.
type Options struct {
	// Excludes are glob patterns (path.Match syntax) tested against each file's
	// relative path and base name; a match skips the file, and a matching
	// directory is not descended.
	Excludes      []string
	Includes      []string
	IncludeHidden bool
	Quick         bool
	Workers       int
}

// Result holds every file entry (sorted by path, including Same) plus counts.
type Result struct {
	Entries                       []Entry
	Added, Removed, Changed, Same int
}

// Compare walks oldDir and newDir and classifies every file by content. For
// directories or archives interchangeably, use CompareAny.
func Compare(oldDir, newDir string, opts Options) (*Result, error) {
	return compareSources(
		dirSource{root: oldDir, opts: opts},
		dirSource{root: newDir, opts: opts},
		opts,
	)
}

// walk returns a map of relative slash-path -> size for the regular files under
// root, honoring the exclude patterns.
func walk(root string, opts Options) (map[string]fileMeta, error) {
	files := make(map[string]fileMeta)
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
		if !opts.IncludeHidden && hiddenPath(rel) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if excluded(rel, d.Name(), opts.Excludes) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() || d.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		if len(opts.Includes) > 0 && !matched(rel, d.Name(), opts.Includes) {
			return nil
		}
		info, infoErr := d.Info()
		if infoErr != nil {
			return infoErr
		}
		if info.Mode().IsRegular() {
			files[rel] = fileMeta{Size: info.Size(), ModTime: info.ModTime()}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

func matched(rel, base string, patterns []string) bool {
	for _, pattern := range patterns {
		if ok, _ := path.Match(pattern, rel); ok {
			return true
		}
		if ok, _ := path.Match(pattern, base); ok {
			return true
		}
	}
	return false
}

func hiddenPath(rel string) bool {
	for _, part := range strings.Split(rel, "/") {
		if strings.HasPrefix(part, ".") {
			return true
		}
	}
	return false
}

func workerCount(opts Options) int {
	workers := opts.Workers
	if workers <= 0 {
		workers = min(runtime.NumCPU(), 8)
	}
	return max(1, min(workers, 64))
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
