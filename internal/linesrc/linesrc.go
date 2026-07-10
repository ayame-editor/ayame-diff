// Package linesrc streams the lines of a text file (plain or gzip) to the
// linediff.Lines interface with bounded memory.
//
// The whole file is never held in memory as one slice of lines. Instead Open
// pre-counts the lines in a single pass, then Line serves them from a sliding
// window over a forward-only reader: requesting an index beyond the window
// reads forward, dropping lines that fall far enough behind. The diff walk
// accesses lines forward with a bounded look-ahead (and only a little
// back-reference), so this stays cheap while keeping resident memory to a small
// multiple of keepBehind lines rather than the whole file.
//
// Contract: access is expected to be forward-only with at most keepBehind lines
// of back-reference. A request for an index below the retained window still
// returns the correct value, but pays for a rewind-and-rescan from the start.
package linesrc

import (
	"bufio"
	"compress/gzip"
	"io"
	"os"
	"strings"
)

const (
	// keepBehind is how many already-served lines are retained below the read
	// cursor so the diff walk's small back-references are cheap cache hits.
	keepBehind = 4096
	// highWater is the buffer length that triggers a compaction back down to
	// keepBehind, amortizing the shift cost to O(1) per line.
	highWater = 2 * keepBehind
	// readerBufSize sizes the bufio.Reader. Lines longer than this still work
	// because ReadString accumulates across fills (unlike bufio.Scanner).
	readerBufSize = 64 * 1024
)

// FileLines serves the lines of a file to linediff.Lines with bounded memory.
// It is not safe for concurrent use.
type FileLines struct {
	path    string
	gzipped bool
	count   uint64

	// Streaming state. reader is positioned just past line index next-1.
	file   *os.File
	gz     *gzip.Reader
	reader *bufio.Reader
	eof    bool

	// Sliding window: buf holds line indexes [bufStart, next). Lines below
	// bufStart have been dropped; lines at or above next are unread.
	buf      []string
	bufStart uint64
	next     uint64
}

// Open reads path (gzip-decompressed if it ends in ".gz"), pre-counts its lines
// in one pass, and returns a FileLines positioned to stream from the start.
// Close releases the underlying file handle.
func Open(path string) (*FileLines, error) {
	gzipped := strings.HasSuffix(strings.ToLower(path), ".gz")
	count, err := countLines(path, gzipped)
	if err != nil {
		return nil, err
	}
	f := &FileLines{path: path, gzipped: gzipped, count: count}
	if err := f.reset(); err != nil {
		return nil, err
	}
	return f, nil
}

// Count returns the pre-counted number of lines.
func (f *FileLines) Count() uint64 { return f.count }

// Line returns the line at index i and true, or "", false if i is out of range.
func (f *FileLines) Line(i uint64) (string, bool) {
	if i >= f.count {
		return "", false
	}
	// Back-reference below the retained window breaks the forward-only
	// contract; recover correctness by rewinding and re-scanning from the top.
	if i < f.bufStart {
		if err := f.reset(); err != nil {
			return "", false
		}
	}
	if i >= f.next {
		f.fill(i)
	}
	if i < f.bufStart || i >= f.next {
		return "", false // unreachable unless the pre-count disagrees with the stream
	}
	return f.buf[i-f.bufStart], true
}

// Close releases the underlying file (and gzip) handles.
func (f *FileLines) Close() error { return f.closeStream() }

// fill reads forward until the window covers target (which the caller has
// already bounded by count), compacting the window as it grows.
func (f *FileLines) fill(target uint64) {
	for f.next <= target {
		line, ok := f.readLine()
		if !ok {
			break
		}
		f.buf = append(f.buf, line)
		f.next++
		if uint64(len(f.buf)) > highWater {
			f.compact()
		}
	}
}

// compact drops the oldest lines, keeping the most recent keepBehind, and
// shifts the survivors to the front so the backing array's capacity is reused
// (a plain reslice would pin the dropped strings and grow memory unbounded).
func (f *FileLines) compact() {
	if uint64(len(f.buf)) <= keepBehind {
		return
	}
	drop := len(f.buf) - keepBehind
	n := copy(f.buf, f.buf[drop:])
	for k := n; k < len(f.buf); k++ {
		f.buf[k] = "" // release the pinned tail duplicates for GC
	}
	f.buf = f.buf[:n]
	f.bufStart += uint64(drop)
}

// readLine returns the next line in the same sequence linediff.SplitLines would
// produce, or ok=false at end of input. A mid-stream read error stops the
// stream; the whole file was already validated readable by the pre-count pass,
// and the Lines interface has no error channel to surface it.
func (f *FileLines) readLine() (string, bool) {
	if f.eof {
		return "", false
	}
	chunk, err := f.reader.ReadString('\n')
	if err != nil {
		f.eof = true
	}
	// A trailing empty chunk (input ended exactly on a newline, or was empty)
	// is not a line, matching SplitLines dropping the final empty field.
	if len(chunk) == 0 {
		return "", false
	}
	return normalizeLine(chunk), true
}

// skipUTF8BOM discards a leading UTF-8 byte-order mark (EF BB BF) if present, so
// the first line is not prefixed with a stray BOM. It must be applied
// identically in the count and streaming passes so the two agree (a file that is
// only a BOM must count as zero lines in both).
func skipUTF8BOM(br *bufio.Reader) error {
	prefix, err := br.Peek(3)
	if err != nil && err != io.EOF && err != bufio.ErrBufferFull {
		return err
	}
	if len(prefix) == 3 && prefix[0] == 0xEF && prefix[1] == 0xBB && prefix[2] == 0xBF {
		_, _ = br.Discard(3)
	}
	return nil
}

// normalizeLine strips the line's trailing "\n" and then a single trailing
// "\r", exactly as linediff.SplitLines trims each field.
func normalizeLine(chunk string) string {
	chunk = strings.TrimSuffix(chunk, "\n")
	chunk = strings.TrimSuffix(chunk, "\r")
	return chunk
}

// countLines counts lines using the same emit rule as readLine, in one pass.
func countLines(path string, gzipped bool) (uint64, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	var r io.Reader = file
	if gzipped {
		gz, err := gzip.NewReader(file)
		if err != nil {
			return 0, err
		}
		defer gz.Close()
		r = gz
	}
	br := bufio.NewReaderSize(r, readerBufSize)
	if err := skipUTF8BOM(br); err != nil {
		return 0, err
	}

	var count uint64
	for {
		chunk, err := br.ReadString('\n')
		if len(chunk) > 0 {
			count++
		}
		if err != nil {
			if err == io.EOF {
				return count, nil
			}
			return 0, err
		}
	}
}

// reset closes any open stream and re-opens a fresh reader at the start.
func (f *FileLines) reset() error {
	f.closeStream()

	file, err := os.Open(f.path)
	if err != nil {
		return err
	}
	var r io.Reader = file
	var gz *gzip.Reader
	if f.gzipped {
		gz, err = gzip.NewReader(file)
		if err != nil {
			file.Close()
			return err
		}
		r = gz
	}
	f.file = file
	f.gz = gz
	f.reader = bufio.NewReaderSize(r, readerBufSize)
	if err := skipUTF8BOM(f.reader); err != nil {
		file.Close()
		return err
	}
	f.buf = nil
	f.bufStart = 0
	f.next = 0
	f.eof = false
	return nil
}

// closeStream releases the current file/gzip handles; safe to call repeatedly.
func (f *FileLines) closeStream() error {
	var firstErr error
	if f.gz != nil {
		if err := f.gz.Close(); err != nil {
			firstErr = err
		}
		f.gz = nil
	}
	if f.file != nil {
		if err := f.file.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		f.file = nil
	}
	f.reader = nil
	return firstErr
}
