package engine

import (
	"bufio"
	"compress/gzip"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

type parserKind uint8

const (
	parserSimple parserKind = iota
	parserRFC4180
)

type inputSpec struct {
	Path       string
	Delimiter  byte
	Parser     parserKind
	Compressed bool
	Label      string
}
type inspectedInput struct {
	Header      []string
	ColumnCount int
	DataOffset  int64
}
type schema struct {
	Header                        []string
	ColumnCount                   int
	LeftMap, RightMap, KeyIndexes []int
	KeyIsFullRow                  bool
}

func resolveInputSpec(path, formatText, delimiterText, parserText, label string) (inputSpec, error) {
	if path == "-" {
		return inputSpec{}, fmt.Errorf("%s input from stdin is not supported because inputs must be scanned more than once", label)
	}
	compressed := strings.HasSuffix(strings.ToLower(path), ".gz")
	format := formatText
	if format == "auto" {
		base := strings.ToLower(path)
		if compressed {
			base = strings.TrimSuffix(base, ".gz")
		}
		switch filepath.Ext(base) {
		case ".tsv", ".tab":
			format = "tsv"
		case ".csv":
			format = "csv"
		default:
			detected, err := sniffFormat(path)
			if err != nil {
				return inputSpec{}, fmt.Errorf("detect %s format: %w", label, err)
			}
			format = detected
		}
	}
	var delimiter byte
	var err error
	if delimiterText != "" {
		delimiter, err = parseDelimiter(delimiterText)
		if err != nil {
			return inputSpec{}, fmt.Errorf("%s delimiter: %w", label, err)
		}
	} else if format == "tsv" {
		delimiter = '\t'
	} else {
		delimiter = ','
	}
	var parser parserKind
	switch parserText {
	case "simple":
		parser = parserSimple
	case "rfc4180":
		parser = parserRFC4180
	case "auto":
		if delimiter == '\t' {
			parser = parserSimple
		} else {
			parser = parserRFC4180
		}
	default:
		return inputSpec{}, fmt.Errorf("invalid %s parser %q", label, parserText)
	}
	return inputSpec{path, delimiter, parser, compressed, label}, nil
}
func parseDelimiter(text string) (byte, error) {
	switch strings.ToLower(text) {
	case "tab", `\t`:
		return '\t', nil
	case "comma":
		return ',', nil
	case "pipe":
		return '|', nil
	}
	if len(text) != 1 || text[0] >= 0x80 {
		return 0, fmt.Errorf("must be comma, tab, \\t, pipe, or one ASCII character")
	}
	if text[0] == '\r' || text[0] == '\n' || text[0] == 0 || text[0] == '"' {
		return 0, fmt.Errorf("unsupported delimiter %q", text)
	}
	return text[0], nil
}
func sniffFormat(path string) (string, error) {
	r, err := openInput(path)
	if err != nil {
		return "", err
	}
	defer r.Close()
	br := bufio.NewReaderSize(r, 256*1024)
	line, err := br.ReadBytes('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	if len(line) == 0 {
		return "", fmt.Errorf("empty input")
	}
	if countByteOutsideSimpleQuotes(line, '\t') > countByteOutsideSimpleQuotes(line, ',') {
		return "tsv", nil
	}
	return "csv", nil
}
func countByteOutsideSimpleQuotes(line []byte, target byte) int {
	count := 0
	quoted := false
	for i := 0; i < len(line); i++ {
		switch line[i] {
		case '"':
			if quoted && i+1 < len(line) && line[i+1] == '"' {
				i++
				continue
			}
			quoted = !quoted
		default:
			if line[i] == target && !quoted {
				count++
			}
		}
	}
	return count
}
func inspectInput(spec inputSpec, hasHeader, lazyQuotes, trimLeadingSpace bool) (inspectedInput, error) {
	if spec.Parser == parserSimple {
		return inspectSimple(spec, hasHeader)
	}
	return inspectRFC4180(spec, hasHeader, lazyQuotes, trimLeadingSpace)
}
func inspectSimple(spec inputSpec, hasHeader bool) (inspectedInput, error) {
	r, err := openInput(spec.Path)
	if err != nil {
		return inspectedInput{}, err
	}
	defer r.Close()
	br := bufio.NewReaderSize(r, 256*1024)
	bomLen, err := skipBOM(br)
	if err != nil {
		return inspectedInput{}, err
	}
	line, err := br.ReadBytes('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return inspectedInput{}, err
	}
	if len(line) == 0 {
		return inspectedInput{}, fmt.Errorf("%s input is empty", spec.Label)
	}
	rawLen := len(line)
	line = trimLineEnding(line)
	fields := splitSimpleLine(line, spec.Delimiter, nil)
	if len(fields) == 0 {
		return inspectedInput{}, fmt.Errorf("%s input has no columns", spec.Label)
	}
	result := inspectedInput{ColumnCount: len(fields)}
	if hasHeader {
		result.Header = byteFieldsToStrings(fields)
		stripBOM(result.Header)
		if !spec.Compressed {
			result.DataOffset = int64(bomLen + rawLen)
		}
	} else {
		result.Header = syntheticHeader(len(fields))
		if !spec.Compressed {
			// Point past the BOM so the parallel range reader never
			// includes it in the first record's key.
			result.DataOffset = int64(bomLen)
		}
	}
	return result, nil
}
func inspectRFC4180(spec inputSpec, hasHeader, lazyQuotes, trimLeadingSpace bool) (inspectedInput, error) {
	r, err := openInput(spec.Path)
	if err != nil {
		return inspectedInput{}, err
	}
	defer r.Close()
	buffered := bufio.NewReaderSize(r, 4*1024*1024)
	if _, err := skipBOM(buffered); err != nil {
		return inspectedInput{}, err
	}
	reader := csv.NewReader(buffered)
	reader.Comma = rune(spec.Delimiter)
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = lazyQuotes
	reader.TrimLeadingSpace = trimLeadingSpace
	record, err := reader.Read()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return inspectedInput{}, fmt.Errorf("%s input is empty", spec.Label)
		}
		return inspectedInput{}, fmt.Errorf("read first %s record: %w", spec.Label, err)
	}
	if len(record) == 0 {
		return inspectedInput{}, fmt.Errorf("%s input has no columns", spec.Label)
	}
	result := inspectedInput{ColumnCount: len(record)}
	if hasHeader {
		result.Header = append([]string(nil), record...)
		stripBOM(result.Header)
	} else {
		result.Header = syntheticHeader(len(record))
	}
	return result, nil
}
func buildSchema(left, right inspectedInput, cfg Config) (schema, error) {
	if len(cfg.KeyNames)+len(cfg.KeyIndexes) > 0 && len(cfg.ExcludeKeyNames)+len(cfg.ExcludeKeyIndexes) > 0 {
		return schema{}, fmt.Errorf("include and exclude key options cannot be combined")
	}
	if left.ColumnCount != right.ColumnCount {
		return schema{}, fmt.Errorf("column count differs: left=%d right=%d", left.ColumnCount, right.ColumnCount)
	}
	n := left.ColumnCount
	leftMap := identityMap(n)
	rightMap := identityMap(n)
	if cfg.HasHeader {
		if err := validateUniqueHeader(left.Header, "left"); err != nil {
			return schema{}, err
		}
		if err := validateUniqueHeader(right.Header, "right"); err != nil {
			return schema{}, err
		}
		if cfg.AlignColumnsByName {
			rightByName := make(map[string]int, n)
			for i, name := range right.Header {
				rightByName[name] = i
			}
			for i, name := range left.Header {
				j, ok := rightByName[name]
				if !ok {
					return schema{}, fmt.Errorf("right header is missing column %q", name)
				}
				rightMap[i] = j
			}
		} else if !reflect.DeepEqual(left.Header, right.Header) {
			return schema{}, fmt.Errorf("headers differ; enable --align-columns-by-name or make the headers identical")
		}
	}
	nameToIndex := make(map[string]int, n)
	for i, name := range left.Header {
		nameToIndex[name] = i
	}
	includeMode := len(cfg.KeyNames)+len(cfg.KeyIndexes) > 0
	keys := make([]int, 0, n)
	if includeMode {
		seen := map[int]struct{}{}
		for _, name := range cfg.KeyNames {
			i, ok := nameToIndex[name]
			if !ok {
				return schema{}, fmt.Errorf("key header %q not found in left header", name)
			}
			if _, ok := seen[i]; !ok {
				seen[i] = struct{}{}
				keys = append(keys, i)
			}
		}
		for _, i := range cfg.KeyIndexes {
			if i < 0 || i >= n {
				return schema{}, fmt.Errorf("key index %d is outside 0..%d", i, n-1)
			}
			if _, ok := seen[i]; !ok {
				seen[i] = struct{}{}
				keys = append(keys, i)
			}
		}
	} else {
		excluded := map[int]struct{}{}
		for _, name := range cfg.ExcludeKeyNames {
			i, ok := nameToIndex[name]
			if !ok {
				return schema{}, fmt.Errorf("excluded key header %q not found in left header", name)
			}
			excluded[i] = struct{}{}
		}
		for _, i := range cfg.ExcludeKeyIndexes {
			if i < 0 || i >= n {
				return schema{}, fmt.Errorf("excluded key index %d is outside 0..%d", i, n-1)
			}
			excluded[i] = struct{}{}
		}
		for i := 0; i < n; i++ {
			if _, skip := excluded[i]; !skip {
				keys = append(keys, i)
			}
		}
	}
	if len(keys) == 0 {
		return schema{}, fmt.Errorf("no key columns remain after applying key selection")
	}
	keyIsFullRow := len(keys) == n
	if keyIsFullRow {
		for i, keyIndex := range keys {
			if keyIndex != i {
				keyIsFullRow = false
				break
			}
		}
	}
	return schema{
		Header:       append([]string(nil), left.Header...),
		ColumnCount:  n,
		LeftMap:      leftMap,
		RightMap:     rightMap,
		KeyIndexes:   keys,
		KeyIsFullRow: keyIsFullRow,
	}, nil
}
func validateUniqueHeader(header []string, side string) error {
	seen := map[string]int{}
	for i, name := range header {
		if prev, ok := seen[name]; ok {
			return fmt.Errorf("%s header contains duplicate column %q at indexes %d and %d", side, name, prev, i)
		}
		seen[name] = i
	}
	return nil
}
func identityMap(n int) []int {
	r := make([]int, n)
	for i := range r {
		r[i] = i
	}
	return r
}
func syntheticHeader(n int) []string {
	r := make([]string, n)
	for i := range r {
		r[i] = fmt.Sprintf("column_%d", i)
	}
	return r
}
func stripBOM(header []string) {
	if len(header) > 0 {
		header[0] = strings.TrimPrefix(header[0], "\uFEFF")
	}
}

// skipBOM consumes a UTF-8 byte order mark at the reader's current position
// and reports how many bytes were skipped (0 or 3). Without this, a BOM on a
// headerless input would be encoded into the first row's key and produce a
// spurious diff against an otherwise identical BOM-less file.
func skipBOM(reader *bufio.Reader) (int, error) {
	prefix, err := reader.Peek(3)
	if len(prefix) == 3 && prefix[0] == 0xEF && prefix[1] == 0xBB && prefix[2] == 0xBF {
		if _, err := reader.Discard(3); err != nil {
			return 0, err
		}
		return 3, nil
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return 0, err
	}
	return 0, nil
}
func splitSimpleLine(line []byte, d byte, dst [][]byte) [][]byte {
	dst = dst[:0]
	start := 0
	for i, b := range line {
		if b == d {
			dst = append(dst, line[start:i])
			start = i + 1
		}
	}
	return append(dst, line[start:])
}
func byteFieldsToStrings(fields [][]byte) []string {
	r := make([]string, len(fields))
	for i, f := range fields {
		r[i] = string(f)
	}
	return r
}
func trimLineEnding(line []byte) []byte {
	if len(line) > 0 && line[len(line)-1] == '\n' {
		line = line[:len(line)-1]
	}
	if len(line) > 0 && line[len(line)-1] == '\r' {
		line = line[:len(line)-1]
	}
	return line
}

type combinedReadCloser struct {
	reader  io.Reader
	closers []io.Closer
}

func (c *combinedReadCloser) Read(p []byte) (int, error) { return c.reader.Read(p) }
func (c *combinedReadCloser) Close() error {
	var result error
	for _, cl := range c.closers {
		if err := cl.Close(); err != nil && result == nil {
			result = err
		}
	}
	return result
}
func openInput(path string) (io.ReadCloser, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	if !strings.HasSuffix(strings.ToLower(path), ".gz") {
		return f, nil
	}
	gz, err := gzip.NewReader(f)
	if err != nil {
		f.Close()
		return nil, err
	}
	return &combinedReadCloser{gz, []io.Closer{gz, f}}, nil
}
