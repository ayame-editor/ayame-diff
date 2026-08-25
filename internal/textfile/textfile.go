// Package textfile reads and writes whole text files while preserving the
// byte-level conventions of the original: character encoding, a leading UTF-8
// BOM, the line terminator, and whether the last line is terminated.
//
// It exists because more than one feature needs this. Three-way merge output
// needed it first (#159); the GUI's editable panes need exactly the same
// guarantee when they save what a user typed (#255). Normalizing to BOM-less
// UTF-8 with LF and a forced trailing newline would silently rewrite files the
// user only meant to edit one line of.
package textfile

import (
	"bufio"
	"fmt"
	"io"

	"github.com/ayame-editor/ayame-diff/internal/atomicfile"
	"github.com/ayame-editor/ayame-diff/internal/encoding"
	"github.com/ayame-editor/ayame-diff/internal/linediff"
	"github.com/ayame-editor/ayame-diff/internal/linesrc"
)

// Optional capabilities a source may expose so a rewrite round-trips the
// input's conventions instead of normalizing them.
type (
	lineEndings   interface{ LineEnding(uint64) string }
	encodedSource interface{ Encoding() string }
	bomSource     interface{ HasBOM() bool }
)

// Profile captures what a rewrite has to reproduce. Sources without this
// metadata (for example in-memory SplitLines) yield UTF-8 / LF / terminated.
type Profile struct {
	Encoding     string `json:"encoding"`     // concrete encoding name; "" or "utf-8" needs no re-encoding
	BOM          bool   `json:"bom"`          // the file began with a UTF-8 BOM
	LineEnding   string `json:"lineEnding"`   // terminator between lines ("\n", "\r\n", or "\r")
	FinalNewline bool   `json:"finalNewline"` // whether the final line is terminated
}

// ProfileOf derives the conventions from source. It reads the terminators of
// the last and then the first line, so for a streaming source it leaves the
// reader rewound to the start — call it immediately before streaming forward
// from line 0.
func ProfileOf(source linediff.Lines) Profile {
	profile := Profile{LineEnding: "\n", FinalNewline: true}
	if enc, ok := source.(encodedSource); ok {
		profile.Encoding = enc.Encoding()
	}
	if b, ok := source.(bomSource); ok {
		profile.BOM = b.HasBOM()
	}
	endings, ok := source.(lineEndings)
	if !ok {
		return profile
	}
	count := source.Count()
	if count == 0 {
		return profile
	}
	// A final line without a terminator reports "" and suppresses the trailing
	// newline; only the last line can lack one, so the first line carries the
	// document's separator unless the file is a single unterminated line.
	profile.FinalNewline = endings.LineEnding(count-1) != ""
	if separator := endings.LineEnding(0); separator != "" {
		profile.LineEnding = separator
	}
	return profile
}

// ReadAll decodes a whole file into lines, refusing anything over maxBytes so a
// caller that holds the result in memory — the GUI editor does — cannot be made
// to load an arbitrarily large file. It returns the profile a later Write needs.
func ReadAll(path, encodingHint string, maxBytes int64) ([]string, Profile, error) {
	source, err := linesrc.OpenEncoding(path, encodingHint)
	if err != nil {
		return nil, Profile{}, err
	}
	defer source.Close()

	profile := ProfileOf(source)
	count := source.Count()
	lines := make([]string, 0, count)
	var size int64
	for index := uint64(0); index < count; index++ {
		line, ok := source.Line(index)
		if !ok {
			return nil, Profile{}, fmt.Errorf("%s: line %d disappeared while reading", path, index)
		}
		size += int64(len(line)) + 1
		if size > maxBytes {
			return nil, Profile{}, fmt.Errorf("%s is larger than the %d byte editing limit", path, maxBytes)
		}
		lines = append(lines, line)
	}
	return lines, profile, nil
}

// flushOnlyWriter hides an underlying io.Closer so a transform.Writer's Close
// flushes the encoder's final bytes (for example ISO-2022-JP's return-to-ASCII
// escape) without also closing the atomic temp file, which atomicfile owns.
type flushOnlyWriter struct{ w io.Writer }

func (f flushOnlyWriter) Write(p []byte) (int, error) { return f.w.Write(p) }

// Write atomically replaces path with lines, restoring profile so the result
// round-trips instead of being normalized.
func Write(path string, lines []string, profile Profile) error {
	separator := profile.LineEnding
	if separator == "" {
		separator = "\n"
	}
	return atomicfile.Write(path, atomicfile.Options{Pattern: ".ayame-text-*.tmp"}, func(destination io.Writer) error {
		// A UTF-8 BOM is written raw; the UTF-16 encoders emit their own.
		if profile.BOM && IsUTF8(profile.Encoding) {
			if _, err := destination.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
				return err
			}
		}
		encoded := encoding.Encoder(flushOnlyWriter{destination}, profile.Encoding)
		writer := bufio.NewWriterSize(encoded, 256*1024)
		for i, line := range lines {
			if _, err := writer.WriteString(line); err != nil {
				return err
			}
			if i < len(lines)-1 || profile.FinalNewline {
				if _, err := writer.WriteString(separator); err != nil {
					return err
				}
			}
		}
		if err := writer.Flush(); err != nil {
			return err
		}
		if closer, ok := encoded.(io.Closer); ok {
			return closer.Close()
		}
		return nil
	})
}

// IsUTF8 reports whether name selects UTF-8 output, for which a BOM must be
// written explicitly (the codec does not add one).
func IsUTF8(name string) bool { return name == "" || name == encoding.UTF8 }
