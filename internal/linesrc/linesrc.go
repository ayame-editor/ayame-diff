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
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/hjosugi/ayame-diff/internal/encoding"
)

const (
	// keepBehind is how many already-served lines are retained below the read
	// cursor so the diff walk's small back-references are cheap cache hits.
	keepBehind = 4096
	// highWater is the buffer length that triggers a compaction back down to
	// keepBehind, amortizing the shift cost to O(1) per line.
	highWater = 2 * keepBehind
	// readerBufSize sizes the bufio.Reader. Lines longer than this still work;
	// universalLineReader accumulates bufio.ErrBufferFull fragments.
	readerBufSize = 64 * 1024
)

// universalLineReader recognizes LF, CRLF, and lone CR without Scanner's token
// limit. pending retains unread bytes when one buffered chunk contains several
// CR-delimited lines.
type universalLineReader struct {
	reader  *bufio.Reader
	pending []byte
	eof     bool
	// maxLine bounds one logical line; 0 disables the check. The sliding
	// window bounds how many lines are resident, but nothing bounded a single
	// line, so a file with no line breaks was fully resident (#137).
	maxLine int
}

func (u *universalLineReader) readLine() (string, string, bool, error) {
	for {
		if index := bytes.IndexAny(u.pending, "\r\n"); index >= 0 {
			// A CR at the buffer boundary needs one more byte to distinguish CRLF.
			if u.pending[index] == '\r' && index+1 == len(u.pending) && !u.eof {
				if err := u.fill(); err != nil {
					return "", "", false, err
				}
				continue
			}
			ending, width := "\n", 1
			if u.pending[index] == '\r' {
				ending = "\r"
				if index+1 < len(u.pending) && u.pending[index+1] == '\n' {
					ending, width = "\r\n", 2
				}
			}
			line := string(u.pending[:index])
			u.pending = u.pending[index+width:]
			return line, ending, true, nil
		}
		if u.eof {
			if len(u.pending) == 0 {
				return "", "", false, nil
			}
			line := string(u.pending)
			u.pending = nil
			return line, "", true, nil
		}
		if err := u.fill(); err != nil {
			return "", "", false, err
		}
	}
}

func (u *universalLineReader) fill() error {
	chunk, err := u.reader.ReadSlice('\n')
	u.appendPending(chunk)
	if limitErr := u.checkLineLength(); limitErr != nil {
		return limitErr
	}
	switch err {
	case nil, bufio.ErrBufferFull:
		return nil
	case io.EOF:
		u.eof = true
		return nil
	default:
		return err
	}
}

// appendPending appends chunk, capping the growth strategy at maxLine plus one
// read buffer. Plain append doubles, so accumulating a line up to a 64 MiB
// limit could reserve 128 MiB and peak near 256 MiB mid-copy — several times
// the budget the user asked for. Growth stays bounded by the limit instead, so
// peak stays close to it.
func (u *universalLineReader) appendPending(chunk []byte) {
	needed := len(u.pending) + len(chunk)
	if needed > cap(u.pending) && u.maxLine > 0 {
		target := 2 * needed
		if ceiling := u.maxLine + readerBufSize + 1; target > ceiling {
			target = ceiling
		}
		if target > cap(u.pending) {
			if target < needed {
				target = needed
			}
			grown := make([]byte, len(u.pending), target)
			copy(grown, u.pending)
			u.pending = grown
		}
	}
	u.pending = append(u.pending, chunk...)
}

// checkLineLength rejects a single line longer than maxLine. It measures the
// first line in pending rather than all of pending, which can legitimately hold
// several CR-delimited lines from one chunk.
func (u *universalLineReader) checkLineLength() error {
	if u.maxLine <= 0 {
		return nil
	}
	length := len(u.pending)
	if index := bytes.IndexAny(u.pending, "\r\n"); index >= 0 {
		length = index
	}
	if length > u.maxLine {
		return &LineTooLongError{Limit: u.maxLine}
	}
	return nil
}

// FileLines serves the lines of a file to linediff.Lines with bounded memory.
// It is not safe for concurrent use.
type FileLines struct {
	path     string
	gzipped  bool
	encoding string // concrete encoding name (as resolved by the encoding pkg)
	hasBOM   bool   // a leading UTF-8 BOM was present (and stripped) on open
	maxLine  int    // per-line cap carried so reset() rebuilds the same reader
	count    uint64

	// Streaming state. reader is positioned just past line index next-1.
	file   *os.File
	gz     *gzip.Reader
	reader *universalLineReader
	eof    bool

	// Sliding window: buf holds line indexes [bufStart, next). Lines below
	// bufStart have been dropped; lines at or above next are unread.
	buf      []string
	endings  []string
	bufStart uint64
	next     uint64
}

// Open reads path with automatic encoding detection. Equivalent to
// OpenEncoding(path, encoding.Auto).
func Open(path string) (*FileLines, error) {
	return OpenEncoding(path, encoding.Auto)
}

// OpenEncoding reads path (gzip-decompressed if it ends in ".gz"), decoding it
// to UTF-8 from the given encoding hint (a concrete name forces it; "auto"/""
// detects from a sample). It pre-counts the lines in one pass and returns a
// FileLines positioned to stream from the start. Close releases the file handle.
func OpenEncoding(path, encHint string) (*FileLines, error) {
	return OpenEncodingLimit(path, encHint, DefaultMaxLineBytes)
}

// OpenEncodingLimit is OpenEncoding with an explicit per-line byte cap; 0
// disables the check. The cap is enforced during the pre-count pass, so an
// input that cannot be compared within bounded memory fails at open time rather
// than part-way through a comparison (#137).
func OpenEncodingLimit(path, encHint string, maxLine int) (*FileLines, error) {
	gzipped := strings.HasSuffix(strings.ToLower(path), ".gz")
	enc, err := detectEncoding(path, gzipped, encHint)
	if err != nil {
		return nil, err
	}
	count, err := countLines(path, gzipped, enc, maxLine)
	if err != nil {
		return nil, err
	}
	f := &FileLines{path: path, gzipped: gzipped, encoding: enc, maxLine: maxLine, count: count}
	if err := f.reset(); err != nil {
		return nil, err
	}
	return f, nil
}

// Encoding reports the concrete encoding the file was decoded from.
func (f *FileLines) Encoding() string { return f.encoding }

// HasBOM reports whether the file began with a UTF-8 byte-order mark, which
// Open strips from the first line. Merge writers use it to re-emit the mark so
// a round-trip preserves it (the UTF-16 codecs carry their own BOM, so this
// stays false for them). (#159)
func (f *FileLines) HasBOM() bool { return f.hasBOM }

// ErrIsDirectory and ErrBinaryContent report inputs the text/line path cannot
// meaningfully compare, so callers can steer users to the folder (dir) or
// binary (bin) modes instead of surfacing a raw "is a directory" syscall error
// or a mojibake "diff" of decoded binary (#166).
var (
	ErrIsDirectory   = errors.New("this path is a folder; use folder (dir) mode to compare directories")
	ErrBinaryContent = errors.New("this file looks binary; use binary (bin) mode to compare non-text files")
)

// DefaultMaxLineBytes bounds one logical line. The sliding window bounds how
// many lines stay resident, but a file with no line breaks is a single line, so
// without this the "streaming" reader would hold the whole file. A var so the
// CLI and server can raise it. (#137)
var DefaultMaxLineBytes = 64 << 20 // 64 MiB

// LineTooLongError reports a line past the limit. It names a way forward
// rather than just refusing: such a file is usually minified data, a database
// dump, or something that is not really line-oriented at all.
type LineTooLongError struct{ Limit int }

func (e *LineTooLongError) Error() string {
	return fmt.Sprintf("this file has a single line longer than %d bytes, so comparing it line by line would hold the whole file in memory; "+
		"raise --max-line-bytes to allow it, or use binary (bin) mode if the file is not line-oriented", e.Limit)
}

// detectSample bounds how many bytes are read to detect the encoding.
const detectSample = 8 * 1024

// detectEncoding reads a decompressed sample from path and resolves its
// encoding, honoring an explicit hint. It rejects directories and binary
// content up front (#166) so the caller never streams a folder or decodes
// non-text bytes into a garbled diff.
func detectEncoding(path string, gzipped bool, hint string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	if info, statErr := file.Stat(); statErr == nil && info.IsDir() {
		return "", ErrIsDirectory
	}
	var r io.Reader = file
	if gzipped {
		gz, err := gzip.NewReader(file)
		if err != nil {
			return "", err
		}
		defer gz.Close()
		r = gz
	}
	sample := make([]byte, detectSample)
	n, err := io.ReadFull(r, sample)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return "", err
	}
	enc := encoding.Detect(sample[:n], hint)
	if looksBinary(sample[:n], enc) {
		return "", ErrBinaryContent
	}
	return enc, nil
}

// looksBinary reports whether sample is non-text. A NUL byte is the classic
// signal — text never contains one — but UTF-16 legitimately encodes ASCII with
// NUL bytes, so a detected UTF-16 encoding is exempt.
func looksBinary(sample []byte, enc string) bool {
	if enc == encoding.UTF16LE || enc == encoding.UTF16BE {
		return false
	}
	return bytes.IndexByte(sample, 0) >= 0
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

// LineEnding reports the original decoded terminator for line i: "\n",
// "\r\n", or "" for a final line without a newline. Patch renderers use this
// optional metadata; ordinary comparisons continue to see normalized text.
func (f *FileLines) LineEnding(i uint64) string {
	if i >= f.count {
		return ""
	}
	if i < f.bufStart {
		if err := f.reset(); err != nil {
			return ""
		}
	}
	if i >= f.next {
		f.fill(i)
	}
	if i < f.bufStart || i >= f.next {
		return ""
	}
	return f.endings[i-f.bufStart]
}

// Close releases the underlying file (and gzip) handles.
func (f *FileLines) Close() error { return f.closeStream() }

// fill reads forward until the window covers target (which the caller has
// already bounded by count), compacting the window as it grows.
func (f *FileLines) fill(target uint64) {
	for f.next <= target {
		line, ending, ok := f.readLine()
		if !ok {
			break
		}
		f.buf = append(f.buf, line)
		f.endings = append(f.endings, ending)
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
	copy(f.endings, f.endings[drop:])
	for k := n; k < len(f.buf); k++ {
		f.buf[k] = "" // release the pinned tail duplicates for GC
		f.endings[k] = ""
	}
	f.buf = f.buf[:n]
	f.endings = f.endings[:n]
	f.bufStart += uint64(drop)
}

// readLine returns the next line in the same sequence linediff.SplitLines would
// produce, or ok=false at end of input. A mid-stream read error stops the
// stream; the whole file was already validated readable by the pre-count pass,
// and the Lines interface has no error channel to surface it.
func (f *FileLines) readLine() (string, string, bool) {
	if f.eof {
		return "", "", false
	}
	line, ending, ok, err := f.reader.readLine()
	if err != nil {
		f.eof = true
	}
	if !ok {
		return "", "", false
	}
	return line, ending, true
}

// skipUTF8BOM discards a leading UTF-8 byte-order mark (EF BB BF) if present, so
// the first line is not prefixed with a stray BOM, and reports whether one was
// discarded. It must be applied identically in the count and streaming passes so
// the two agree (a file that is only a BOM must count as zero lines in both).
func skipUTF8BOM(br *bufio.Reader) (bool, error) {
	prefix, err := br.Peek(3)
	if err != nil && err != io.EOF && err != bufio.ErrBufferFull {
		return false, err
	}
	if len(prefix) == 3 && prefix[0] == 0xEF && prefix[1] == 0xBB && prefix[2] == 0xBF {
		_, _ = br.Discard(3)
		return true, nil
	}
	return false, nil
}

// countLines counts lines using the same emit rule as readLine, in one pass,
// over the decoded (UTF-8) stream.
func countLines(path string, gzipped bool, enc string, maxLine int) (uint64, error) {
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
	br := bufio.NewReaderSize(encoding.Decoder(r, enc), readerBufSize)
	if _, err := skipUTF8BOM(br); err != nil {
		return 0, err
	}

	reader := &universalLineReader{reader: br, maxLine: maxLine}
	var count uint64
	for {
		_, _, ok, err := reader.readLine()
		if err != nil {
			return 0, err
		}
		if !ok {
			return count, nil
		}
		count++
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
	reader := bufio.NewReaderSize(encoding.Decoder(r, f.encoding), readerBufSize)
	hadBOM, err := skipUTF8BOM(reader)
	if err != nil {
		file.Close()
		return err
	}
	f.hasBOM = hadBOM
	f.reader = &universalLineReader{reader: reader, maxLine: f.maxLine}
	f.buf = nil
	f.endings = nil
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
