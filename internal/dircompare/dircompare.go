// Package dircompare recursively compares two directory trees and reports which
// files were added, removed, or changed between them (WinMerge-style folder
// comparison — hjosugi/ayame-diff#52). It compares file content, not just
// timestamps, so it is reliable across copies and checkouts.
package dircompare

import (
	"context"
	"errors"
	"io/fs"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	// DefaultMaxArchiveEntryBytes bounds one uncompressed archive member.
	DefaultMaxArchiveEntryBytes int64 = 64 << 20
	// DefaultMaxArchiveBytes bounds all selected uncompressed members in one
	// archive. CompareAny can open two archives, so its worst-case retention is
	// twice this value plus small comparison buffers.
	DefaultMaxArchiveBytes int64 = 256 << 20
)

// ErrArchiveLimit identifies an archive rejected before its expanded content
// could exceed a configured memory limit.
var ErrArchiveLimit = errors.New("archive extraction limit exceeded")

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
	// Excludes are slash-separated glob patterns tested against each file's
	// relative path and base name. In addition to path.Match syntax, a complete
	// ** path segment matches zero or more directories. A matching directory is
	// not descended.
	Excludes      []string
	Includes      []string
	IncludeHidden bool
	Quick         bool
	Workers       int
	// MaxArchiveEntryBytes and MaxArchiveBytes cap uncompressed archive data
	// retained in memory (0 selects the safe defaults above).
	MaxArchiveEntryBytes int64
	MaxArchiveBytes      int64
}

// Result holds every file entry (sorted by path, including Same) plus counts.
type Result struct {
	Entries                       []Entry
	Added, Removed, Changed, Same int
}

// Compare walks oldDir and newDir and classifies every file by content. For
// directories or archives interchangeably, use CompareAny.
func Compare(oldDir, newDir string, opts Options) (*Result, error) {
	return CompareContext(context.Background(), oldDir, newDir, opts)
}

// CompareContext is Compare that aborts the content comparison early when ctx is
// cancelled (#169).
func CompareContext(ctx context.Context, oldDir, newDir string, opts Options) (*Result, error) {
	return compareSources(
		ctx,
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
		if globMatch(pattern, rel) {
			return true
		}
		if globMatch(pattern, base) {
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
		if globMatch(pat, rel) {
			return true
		}
		if globMatch(pat, base) {
			return true
		}
	}
	return false
}

// globMatch extends path.Match with the conventional doublestar rule: a path
// segment consisting only of ** matches zero or more complete path segments.
// Malformed patterns are treated as non-matches, matching the prior behavior.
func globMatch(pattern, name string) bool {
	if !strings.Contains(pattern, "**") {
		ok, _ := path.Match(pattern, name)
		return ok
	}
	patternParts := strings.Split(pattern, "/")
	nameParts := strings.Split(name, "/")
	type state struct{ pattern, name int }
	memo := make(map[state]bool)
	seen := make(map[state]bool)
	var match func(int, int) bool
	match = func(patternIndex, nameIndex int) bool {
		key := state{patternIndex, nameIndex}
		if seen[key] {
			return memo[key]
		}
		seen[key] = true
		matched := false
		defer func() { memo[key] = matched }()
		if patternIndex == len(patternParts) {
			matched = nameIndex == len(nameParts)
			return matched
		}
		if patternParts[patternIndex] == "**" {
			for patternIndex+1 < len(patternParts) && patternParts[patternIndex+1] == "**" {
				patternIndex++
			}
			if patternIndex+1 == len(patternParts) {
				matched = true
				return true
			}
			for next := nameIndex; next <= len(nameParts); next++ {
				if match(patternIndex+1, next) {
					matched = true
					return true
				}
			}
			return false
		}
		if nameIndex == len(nameParts) {
			return false
		}
		segmentMatch, err := path.Match(patternParts[patternIndex], nameParts[nameIndex])
		matched = err == nil && segmentMatch && match(patternIndex+1, nameIndex+1)
		return matched
	}
	return match(0, 0)
}
