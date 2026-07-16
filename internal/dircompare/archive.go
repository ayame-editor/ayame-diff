package dircompare

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// source is a comparable tree of files with content, backed by either a
// directory on disk or a size-bounded archive held in memory.
type source interface {
	// files returns rel-slash-path -> size for the regular files.
	files() (map[string]fileMeta, error)
	// open returns the content of one file.
	open(rel string) (io.ReadCloser, error)
}

// dirSource walks a directory on disk (content is streamed, not buffered).
type dirSource struct {
	root string
	opts Options
}

func (d dirSource) files() (map[string]fileMeta, error) { return walk(d.root, d.opts) }
func (d dirSource) open(rel string) (io.ReadCloser, error) {
	return os.Open(filepath.Join(d.root, filepath.FromSlash(rel)))
}

// archiveSource holds an archive's selected entries in memory (uncompressed).
// loadArchive enforces per-entry and aggregate limits before returning one.
type archiveSource struct {
	content map[string]archiveEntry
}
type archiveEntry struct {
	data    []byte
	modTime time.Time
}

func (a archiveSource) files() (map[string]fileMeta, error) {
	m := make(map[string]fileMeta, len(a.content))
	for k, v := range a.content {
		m[k] = fileMeta{Size: int64(len(v.data)), ModTime: v.modTime}
	}
	return m, nil
}
func (a archiveSource) open(rel string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(a.content[rel].data)), nil
}

// IsArchive reports whether path names a supported archive.
func IsArchive(path string) bool {
	p := strings.ToLower(path)
	return strings.HasSuffix(p, ".zip") || strings.HasSuffix(p, ".tar") ||
		strings.HasSuffix(p, ".tar.gz") || strings.HasSuffix(p, ".tgz")
}

func makeSource(path string, opts Options) (source, error) {
	if IsArchive(path) {
		return loadArchive(path, opts)
	}
	return dirSource{root: path, opts: opts}, nil
}

// loadArchive reads a .zip / .tar / .tar.gz / .tgz into an archiveSource,
// honoring filters and strict per-entry and aggregate expansion limits.
func loadArchive(path string, opts Options) (source, error) {
	content := map[string]archiveEntry{}
	var expandedBytes int64
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
			if (!opts.IncludeHidden && hiddenPath(rel)) || excluded(rel, filepath.Base(rel), opts.Excludes) || (len(opts.Includes) > 0 && !matched(rel, filepath.Base(rel), opts.Includes)) {
				continue
			}
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			declaredSize := int64(f.UncompressedSize64)
			if f.UncompressedSize64 > math.MaxInt64 {
				rc.Close()
				return nil, archiveLimitError(rel, "entry", math.MaxInt64, opts)
			}
			if opts.Filter != nil && !opts.Filter.Match(rel, declaredSize, f.Modified) {
				rc.Close()
				continue
			}
			b, err := readArchiveEntry(rc, rel, declaredSize, &expandedBytes, opts)
			closeErr := rc.Close()
			if err != nil {
				return nil, err
			}
			if closeErr != nil {
				return nil, closeErr
			}
			content[rel] = archiveEntry{data: b, modTime: f.Modified}
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
		if (!opts.IncludeHidden && hiddenPath(rel)) || excluded(rel, filepath.Base(rel), opts.Excludes) || (len(opts.Includes) > 0 && !matched(rel, filepath.Base(rel), opts.Includes)) {
			continue
		}
		if opts.Filter != nil && !opts.Filter.Match(rel, h.Size, h.ModTime) {
			continue
		}
		b, err := readArchiveEntry(tr, rel, h.Size, &expandedBytes, opts)
		if err != nil {
			return nil, err
		}
		content[rel] = archiveEntry{data: b, modTime: h.ModTime}
	}
	return archiveSource{content}, nil
}

func archiveLimits(opts Options) (entry, total int64) {
	entry = opts.MaxArchiveEntryBytes
	if entry <= 0 {
		entry = DefaultMaxArchiveEntryBytes
	}
	total = opts.MaxArchiveBytes
	if total <= 0 {
		total = DefaultMaxArchiveBytes
	}
	return entry, total
}

func readArchiveEntry(r io.Reader, name string, declaredSize int64, expandedBytes *int64, opts Options) ([]byte, error) {
	entryLimit, totalLimit := archiveLimits(opts)
	if declaredSize < 0 || declaredSize > entryLimit {
		return nil, archiveLimitError(name, "entry", declaredSize, opts)
	}
	remaining := totalLimit - *expandedBytes
	if remaining < 0 || declaredSize > remaining {
		return nil, archiveLimitError(name, "total", *expandedBytes+declaredSize, opts)
	}
	readLimit := min(entryLimit, remaining)
	limitWithSentinel := readLimit
	if limitWithSentinel < math.MaxInt64 {
		limitWithSentinel++
	}
	data, err := io.ReadAll(io.LimitReader(r, limitWithSentinel))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > entryLimit {
		return nil, archiveLimitError(name, "entry", int64(len(data)), opts)
	}
	if int64(len(data)) > remaining {
		return nil, archiveLimitError(name, "total", *expandedBytes+int64(len(data)), opts)
	}
	*expandedBytes += int64(len(data))
	return data, nil
}

func archiveLimitError(name, kind string, size int64, opts Options) error {
	entryLimit, totalLimit := archiveLimits(opts)
	limit := entryLimit
	if kind == "total" {
		limit = totalLimit
	}
	if kind == "total" {
		return fmt.Errorf("%w: archive would expand to %d bytes after %q (total limit %d bytes)", ErrArchiveLimit, size, name, limit)
	}
	return fmt.Errorf("%w: entry %q expands to %d bytes (entry limit %d bytes)", ErrArchiveLimit, name, size, limit)
}

// CompareAny compares two paths, each a directory or a supported archive
// (.zip / .tar / .tar.gz / .tgz), so archives compare like folders. Archive
// selected contents are held in memory within Options' expansion limits.
func CompareAny(oldPath, newPath string, opts Options) (*Result, error) {
	oldSrc, err := makeSource(oldPath, opts)
	if err != nil {
		return nil, err
	}
	newSrc, err := makeSource(newPath, opts)
	if err != nil {
		return nil, err
	}
	return compareSources(oldSrc, newSrc, opts)
}

// Preview summarizes the paths selected by folder filters before content
// comparison starts. Sample is a sorted union capped by the caller.
type Preview struct {
	OldCount   int      `json:"old_count"`
	NewCount   int      `json:"new_count"`
	UnionCount int      `json:"union_count"`
	Sample     []string `json:"sample"`
}

// PreviewAny applies source selection and metadata filters without opening file
// contents. It is used by the GUI filter editor for quick feedback.
func PreviewAny(oldPath, newPath string, opts Options, maxSample int) (Preview, error) {
	oldSrc, err := makeSource(oldPath, opts)
	if err != nil {
		return Preview{}, err
	}
	newSrc, err := makeSource(newPath, opts)
	if err != nil {
		return Preview{}, err
	}
	oldFiles, err := oldSrc.files()
	if err != nil {
		return Preview{}, err
	}
	newFiles, err := newSrc.files()
	if err != nil {
		return Preview{}, err
	}
	union := make(map[string]struct{}, len(oldFiles)+len(newFiles))
	for name := range oldFiles {
		union[name] = struct{}{}
	}
	for name := range newFiles {
		union[name] = struct{}{}
	}
	names := make([]string, 0, len(union))
	for name := range union {
		names = append(names, name)
	}
	sort.Strings(names)
	if maxSample < 0 {
		maxSample = 0
	}
	if len(names) > maxSample {
		names = names[:maxSample]
	}
	return Preview{OldCount: len(oldFiles), NewCount: len(newFiles), UnionCount: len(union), Sample: names}, nil
}

// compareSources classifies every file across the two sources by content.
func compareSources(oldSrc, newSrc source, opts Options) (*Result, error) {
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

	method, err := ParseCompareMethod(string(opts.CompareBy))
	if err != nil {
		return nil, err
	}
	if opts.CompareBy == "" && opts.Quick {
		method = CompareQuick
	}
	res := &Result{}
	for _, rel := range rels {
		oldMeta, inOld := oldFiles[rel]
		newMeta, inNew := newFiles[rel]
		e := Entry{Path: rel, OldSize: -1, NewSize: -1}
		switch {
		case inOld && !inNew:
			e.Status, e.OldSize, e.OldModTime = Removed, oldMeta.Size, oldMeta.ModTime
		case !inOld && inNew:
			e.Status, e.NewSize, e.NewModTime = Added, newMeta.Size, newMeta.ModTime
		default:
			e.OldSize, e.NewSize, e.OldModTime, e.NewModTime = oldMeta.Size, newMeta.Size, oldMeta.ModTime, newMeta.ModTime
			switch method {
			case CompareSize:
				if oldMeta.Size == newMeta.Size {
					e.Status = Same
				} else {
					e.Status = Changed
				}
			case CompareDate:
				if !oldMeta.ModTime.IsZero() && oldMeta.ModTime.Equal(newMeta.ModTime) {
					e.Status = Same
				} else {
					e.Status = Changed
				}
			case CompareQuick:
				if oldMeta.Size == newMeta.Size && !oldMeta.ModTime.IsZero() && oldMeta.ModTime.Equal(newMeta.ModTime) {
					e.Status = Same
				} else {
					e.Status = Changed
				}
			default:
				e.Status = Changed
			}
		}
		res.Entries = append(res.Entries, e)
	}
	jobs := make(chan int)
	results := make(chan struct {
		index int
		equal bool
		err   error
	})
	var wg sync.WaitGroup
	for range workerCount(opts) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobs {
				var equal bool
				var err error
				if method == CompareHash {
					equal, err = hashEqual(oldSrc, newSrc, res.Entries[index].Path)
				} else {
					equal, err = contentEqual(oldSrc, newSrc, res.Entries[index].Path)
				}
				results <- struct {
					index int
					equal bool
					err   error
				}{index, equal, err}
			}
		}()
	}
	go func() {
		for index := range res.Entries {
			e := res.Entries[index]
			comparableSize := e.OldSize == e.NewSize || strings.HasSuffix(strings.ToLower(e.Path), ".gz")
			needsContent := method == CompareContents || method == CompareHash || (method == CompareQuick && e.Status != Same)
			if needsContent && e.OldSize >= 0 && e.NewSize >= 0 && comparableSize {
				jobs <- index
			}
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()
	var compareErr error
	for result := range results {
		if result.err != nil {
			if compareErr == nil {
				compareErr = result.err
			}
			continue
		}
		if result.equal {
			res.Entries[result.index].Status = Same
		}
	}
	if compareErr != nil {
		return nil, compareErr
	}
	for _, e := range res.Entries {
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
	}
	return res, nil
}

func hashEqual(oldSrc, newSrc source, rel string) (bool, error) {
	a, err := openCompared(oldSrc, rel)
	if err != nil {
		return false, err
	}
	defer a.Close()
	b, err := openCompared(newSrc, rel)
	if err != nil {
		return false, err
	}
	defer b.Close()
	left, right := sha256.New(), sha256.New()
	if _, err := io.Copy(left, a); err != nil {
		return false, err
	}
	if _, err := io.Copy(right, b); err != nil {
		return false, err
	}
	return bytes.Equal(left.Sum(nil), right.Sum(nil)), nil
}

func openCompared(src source, rel string) (io.ReadCloser, error) {
	raw, err := src.open(rel)
	if err != nil {
		return nil, err
	}
	if !strings.HasSuffix(strings.ToLower(rel), ".gz") {
		return raw, nil
	}
	gz, err := gzip.NewReader(raw)
	if err != nil {
		raw.Close()
		return nil, err
	}
	return &combinedReadCloser{Reader: gz, close: func() error {
		gzErr := gz.Close()
		rawErr := raw.Close()
		if gzErr != nil {
			return gzErr
		}
		return rawErr
	}}, nil
}

type combinedReadCloser struct {
	io.Reader
	close func() error
}

func (c *combinedReadCloser) Close() error { return c.close() }

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
	var ar, br io.Reader = a, b
	var ag, bg *gzip.Reader
	if strings.HasSuffix(strings.ToLower(rel), ".gz") {
		ag, err = gzip.NewReader(a)
		if err != nil {
			return false, err
		}
		defer ag.Close()
		ar = ag
		bg, err = gzip.NewReader(b)
		if err != nil {
			return false, err
		}
		defer bg.Close()
		br = bg
	}
	return readersEqual(ar, br)
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
