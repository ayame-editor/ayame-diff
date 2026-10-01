package engine

import (
	"bufio"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
)

// SameRowOrder reports whether left and right present their data rows in the
// same order after columns are mapped and comparison normalization is applied.
//
// A key-based comparison is row-order independent, so a "no differences"
// result cannot by itself say whether the inputs were also written in the same
// order. This is a streaming, single-pass check that stops at the first
// mismatch, so a client can distinguish "identical" from "same data, different
// row order" (#116) without re-extracting anything.
func SameRowOrder(cfg Config) (bool, error) {
	prepared, err := prepareComparisonInputs(cfg)
	if err != nil {
		return false, err
	}
	left, err := openOrderedRows(prepared.leftSpec, prepared.leftInfo, cfg.HasHeader, prepared.schema.LeftMap, prepared.resolved.Comparison, cfg.LazyQuotes, cfg.TrimLeadingSpace)
	if err != nil {
		return false, err
	}
	defer left.close()
	right, err := openOrderedRows(prepared.rightSpec, prepared.rightInfo, cfg.HasHeader, prepared.schema.RightMap, prepared.resolved.Comparison, cfg.LazyQuotes, cfg.TrimLeadingSpace)
	if err != nil {
		return false, err
	}
	defer right.close()

	for {
		leftFields, leftErr := left.next()
		rightFields, rightErr := right.next()
		leftDone, rightDone := errors.Is(leftErr, io.EOF), errors.Is(rightErr, io.EOF)
		if leftDone && rightDone {
			return true, nil
		}
		if leftDone || rightDone {
			return false, nil
		}
		if leftErr != nil {
			return false, leftErr
		}
		if rightErr != nil {
			return false, rightErr
		}
		if !rowsEqualInOrder(leftFields, rightFields, prepared.resolved.Comparison) {
			return false, nil
		}
	}
}

// rowsEqualInOrder compares one pair of already-mapped rows with the same rules
// the comparison uses, so an ignored or tolerance-equal column does not create
// a phantom row-order difference.
func rowsEqualInOrder(left, right []string, comparison comparisonConfig) bool {
	if len(left) != len(right) {
		return false
	}
	if comparison.hasTolerance() {
		return comparison.equivalentPrepared(comparison.prepare(left), comparison.prepare(right))
	}
	for i := range left {
		if comparison.enabled && comparison.ignoreColumns[i] {
			continue
		}
		leftValue, rightValue := left[i], right[i]
		if comparison.enabled {
			leftValue, rightValue = comparison.normalize(leftValue), comparison.normalize(rightValue)
		}
		if leftValue != rightValue {
			return false
		}
	}
	return true
}

// orderedRowReader yields each input record mapped into left-column order. It
// reuses one slice for the mapped row, so a caller compares the two rows before
// reading further.
type orderedRowReader struct {
	raw        func() ([]string, error)
	closeFn    func() error
	mapping    []int
	reorder    []string
	comparison comparisonConfig
}

func (r *orderedRowReader) next() ([]string, error) {
	fields, err := r.raw()
	if err != nil {
		return nil, err
	}
	if len(fields) != len(r.mapping) {
		return nil, fmt.Errorf("record has %d columns; expected %d", len(fields), len(r.mapping))
	}
	if cap(r.reorder) < len(r.mapping) {
		r.reorder = make([]string, len(r.mapping))
	}
	r.reorder = r.reorder[:len(r.mapping)]
	for i, j := range r.mapping {
		r.reorder[i] = fields[j]
	}
	return r.reorder, nil
}

func (r *orderedRowReader) close() error {
	if r.closeFn == nil {
		return nil
	}
	return r.closeFn()
}

func openOrderedRows(spec inputSpec, info inspectedInput, hasHeader bool, mapping []int, comparison comparisonConfig, lazyQuotes, trimLeadingSpace bool) (*orderedRowReader, error) {
	reader := &orderedRowReader{mapping: mapping, comparison: comparison}
	if spec.Parser != parserSimple {
		return openOrderedRFC4180(reader, spec, info, hasHeader, lazyQuotes, trimLeadingSpace)
	}
	r, err := openInput(spec.Path)
	if err != nil {
		return nil, err
	}
	reader.closeFn = r.Close
	br := bufio.NewReaderSize(r, ioBufferBytes)
	if hasHeader {
		if _, _, _, err := readPhysicalLine(br, nil); err != nil && !errors.Is(err, io.EOF) {
			_ = r.Close()
			return nil, fmt.Errorf("read %s header: %w", spec.Label, err)
		}
	} else if info.DataOffset > 0 {
		if _, err := br.Discard(int(info.DataOffset)); err != nil {
			_ = r.Close()
			return nil, fmt.Errorf("skip %s BOM: %w", spec.Label, err)
		}
	}
	var scratch []byte
	var fields [][]byte
	done := false
	reader.raw = func() ([]string, error) {
		if done {
			return nil, io.EOF
		}
		for {
			line, _, owned, readErr := readPhysicalLine(br, scratch)
			if owned {
				scratch = line[:0]
			}
			line = trimLineEnding(line)
			if len(line) == 0 {
				if errors.Is(readErr, io.EOF) {
					done = true
					return nil, io.EOF
				}
				if readErr != nil {
					return nil, readErr
				}
				continue
			}
			if readErr != nil && !errors.Is(readErr, io.EOF) {
				return nil, readErr
			}
			fields = splitSimpleLine(line, spec.Delimiter, fields)
			row := byteFieldsToStrings(fields)
			if errors.Is(readErr, io.EOF) {
				done = true
			}
			return row, nil
		}
	}
	return reader, nil
}

func openOrderedRFC4180(reader *orderedRowReader, spec inputSpec, info inspectedInput, hasHeader, lazyQuotes, trimLeadingSpace bool) (*orderedRowReader, error) {
	r, err := openInput(spec.Path)
	if err != nil {
		return nil, err
	}
	reader.closeFn = r.Close
	br := bufio.NewReaderSize(r, ioBufferBytes)
	if !hasHeader && info.DataOffset > 0 {
		if _, err := io.CopyN(io.Discard, br, info.DataOffset); err != nil {
			_ = r.Close()
			return nil, fmt.Errorf("skip %s BOM: %w", spec.Label, err)
		}
	}
	cr := csv.NewReader(br)
	cr.Comma = rune(spec.Delimiter)
	cr.FieldsPerRecord = -1
	cr.LazyQuotes = lazyQuotes
	cr.TrimLeadingSpace = trimLeadingSpace
	if hasHeader {
		if _, err := cr.Read(); err != nil {
			_ = r.Close()
			return nil, fmt.Errorf("read %s header: %w", spec.Label, err)
		}
	}
	reader.raw = func() ([]string, error) {
		record, err := cr.Read()
		if err != nil {
			return nil, err
		}
		return record, nil
	}
	return reader, nil
}
