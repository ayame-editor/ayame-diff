package engine

import (
	"bufio"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
)

// FilterPreview is the live "how many rows does this filter keep?" report the
// GUI shows before running a comparison (#129). Counts come from one streaming
// pass per side; no row is retained.
type FilterPreview struct {
	LeftTotal      uint64   `json:"left_total"`
	LeftMatched    uint64   `json:"left_matched"`
	RightTotal     uint64   `json:"right_total"`
	RightMatched   uint64   `json:"right_matched"`
	ColumnCount    int      `json:"column_count"`
	KeptColumns    int      `json:"kept_columns"`
	IgnoredColumns []string `json:"ignored_columns,omitempty"`
	Scanned        uint64   `json:"scanned"`
	Truncated      bool     `json:"truncated"`
}

// PreviewFilter counts rows kept by a Config's RowFilter/ColumnFilter without
// running a comparison. It scans at most limit rows per side so a preview on a
// huge input stays bounded; Truncated reports that the cap was reached.
func PreviewFilter(ctx context.Context, cfg Config, limit uint64) (FilterPreview, error) {
	if cfg.LeftPath == "" || cfg.RightPath == "" {
		return FilterPreview{}, fmt.Errorf("left and right input paths are required")
	}
	if limit == 0 {
		limit = 100000
	}
	resolved, err := cfg.resolve()
	if err != nil {
		return FilterPreview{}, err
	}
	leftSpec, err := resolveInputSpec(resolved.LeftPath, resolved.LeftFormat, resolved.LeftDelimiter, resolved.LeftParser, "left")
	if err != nil {
		return FilterPreview{}, err
	}
	rightSpec, err := resolveInputSpec(resolved.RightPath, resolved.RightFormat, resolved.RightDelimiter, resolved.RightParser, "right")
	if err != nil {
		return FilterPreview{}, err
	}
	leftInfo, err := inspectInput(leftSpec, resolved.HasHeader, resolved.LazyQuotes, resolved.TrimLeadingSpace)
	if err != nil {
		return FilterPreview{}, err
	}
	rightInfo, err := inspectInput(rightSpec, resolved.HasHeader, resolved.LazyQuotes, resolved.TrimLeadingSpace)
	if err != nil {
		return FilterPreview{}, err
	}
	// Key selection is irrelevant to a filter preview and a half-filled form
	// would otherwise fail here; the column filter and row filter are kept.
	schemaCfg := resolved.Config
	schemaCfg.KeyNames, schemaCfg.KeyIndexes, schemaCfg.ExcludeKeyNames, schemaCfg.ExcludeKeyIndexes = nil, nil, nil, nil
	schema, err := buildSchema(leftInfo, rightInfo, schemaCfg)
	if err != nil {
		return FilterPreview{}, err
	}
	leftTotal, leftMatched, leftTruncated, err := countFilterRows(ctx, leftSpec, leftInfo, resolved, schema.LeftMap, schema.Comparison.rowFilter, limit)
	if err != nil {
		return FilterPreview{}, fmt.Errorf("left input: %w", err)
	}
	rightTotal, rightMatched, rightTruncated, err := countFilterRows(ctx, rightSpec, rightInfo, resolved, schema.RightMap, schema.Comparison.rowFilter, limit)
	if err != nil {
		return FilterPreview{}, fmt.Errorf("right input: %w", err)
	}
	preview := FilterPreview{
		LeftTotal:    leftTotal,
		LeftMatched:  leftMatched,
		RightTotal:   rightTotal,
		RightMatched: rightMatched,
		ColumnCount:  schema.ColumnCount,
		Scanned:      leftTotal + rightTotal,
		Truncated:    leftTruncated || rightTruncated,
	}
	for index, name := range schema.Header {
		if schema.Comparison.ignoreColumns[index] {
			preview.IgnoredColumns = append(preview.IgnoredColumns, name)
		} else {
			preview.KeptColumns++
		}
	}
	return preview, nil
}

func countFilterRows(ctx context.Context, spec inputSpec, info inspectedInput, cfg resolvedConfig, mapping []int, filter *compiledFilter, limit uint64) (uint64, uint64, bool, error) {
	if spec.Parser == parserSimple {
		return countSimpleFilterRows(ctx, spec, info, cfg, mapping, filter, limit)
	}
	return countRFC4180FilterRows(ctx, spec, info, cfg, mapping, filter, limit)
}

func countSimpleFilterRows(ctx context.Context, spec inputSpec, info inspectedInput, cfg resolvedConfig, mapping []int, filter *compiledFilter, limit uint64) (uint64, uint64, bool, error) {
	r, err := openInput(spec.Path)
	if err != nil {
		return 0, 0, false, err
	}
	defer r.Close()
	reader := bufio.NewReaderSize(r, ioBufferBytes)
	if !cfg.HasHeader && info.DataOffset > 0 {
		if _, err := reader.Discard(int(info.DataOffset)); err != nil {
			return 0, 0, false, err
		}
	}
	var lineScratch []byte
	var fields [][]byte
	next := func() ([][]byte, error) {
		for {
			line, raw, owned, e := readPhysicalLine(reader, lineScratch)
			if owned {
				lineScratch = line[:0]
			}
			if errors.Is(e, io.EOF) && raw == 0 {
				return nil, io.EOF
			}
			if e != nil && !errors.Is(e, io.EOF) {
				return nil, e
			}
			trailingEOF := errors.Is(e, io.EOF)
			line = trimLineEnding(line)
			if len(line) == 0 {
				if trailingEOF {
					return nil, io.EOF
				}
				continue
			}
			fields = splitSimpleLine(line, spec.Delimiter, fields)
			return fields, nil
		}
	}
	return scanFilterRows(ctx, next, info.ColumnCount, spec.Label, cfg.HasHeader, mapping, filter, limit)
}

func countRFC4180FilterRows(ctx context.Context, spec inputSpec, info inspectedInput, cfg resolvedConfig, mapping []int, filter *compiledFilter, limit uint64) (uint64, uint64, bool, error) {
	r, err := openInput(spec.Path)
	if err != nil {
		return 0, 0, false, err
	}
	defer r.Close()
	if !cfg.HasHeader && info.DataOffset > 0 {
		if _, err := io.CopyN(io.Discard, r, info.DataOffset); err != nil {
			return 0, 0, false, fmt.Errorf("skip %s BOM: %w", spec.Label, err)
		}
	}
	reader := csv.NewReader(bufio.NewReaderSize(r, ioBufferBytes))
	reader.Comma = rune(spec.Delimiter)
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = cfg.LazyQuotes
	reader.TrimLeadingSpace = cfg.TrimLeadingSpace
	next := func() ([]string, error) {
		record, err := reader.Read()
		if err != nil {
			return nil, err
		}
		return record, nil
	}
	return scanFilterRows(ctx, next, info.ColumnCount, spec.Label, cfg.HasHeader, mapping, filter, limit)
}

func scanFilterRows[T fieldBytes](ctx context.Context, next func() ([]T, error), columnCount int, label string, hasHeader bool, mapping []int, filter *compiledFilter, limit uint64) (uint64, uint64, bool, error) {
	if hasHeader {
		if _, err := next(); err != nil {
			if errors.Is(err, io.EOF) {
				return 0, 0, false, nil
			}
			return 0, 0, false, err
		}
	}
	var total, matched uint64
	for {
		if err := ctx.Err(); err != nil {
			return total, matched, false, err
		}
		if total >= limit {
			if _, err := next(); errors.Is(err, io.EOF) {
				return total, matched, false, nil
			} else if err != nil {
				return total, matched, false, err
			}
			return total, matched, true, nil
		}
		fields, err := next()
		if errors.Is(err, io.EOF) {
			return total, matched, false, nil
		}
		if err != nil {
			return total, matched, false, err
		}
		if len(fields) != columnCount {
			return total, matched, false, fmt.Errorf("%s record %d has %d columns; expected %d", label, total+1, len(fields), columnCount)
		}
		total++
		if filter == nil || matchFilterFields(filter, fields, mapping) {
			matched++
		}
	}
}
