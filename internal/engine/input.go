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
	"slices"
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

const utf8BOM = "\xef\xbb\xbf"

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
			result.DataOffset = int64(rawLen)
		}
	} else {
		result.Header = syntheticHeader(len(fields))
		if strings.HasPrefix(string(line), utf8BOM) {
			result.DataOffset = int64(len(utf8BOM))
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
	reader := csv.NewReader(bufio.NewReaderSize(r, ioBufferBytes))
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
		if len(record) > 0 && strings.HasPrefix(record[0], "\uFEFF") {
			result.DataOffset = int64(len(utf8BOM))
		}
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
	leftMap, rightMap, err := alignHeaders(left.Header, right.Header, n, cfg.HasHeader, cfg.AlignColumnsByName)
	if err != nil {
		return schema{}, err
	}
	includeMode := len(cfg.KeyNames)+len(cfg.KeyIndexes) > 0
	var keys []int
	if includeMode {
		keys, err = resolveIncludeKeys(left.Header, n, cfg.KeyNames, cfg.KeyIndexes)
	} else {
		keys, err = resolveExcludeKeys(left.Header, n, cfg.ExcludeKeyNames, cfg.ExcludeKeyIndexes)
	}
	if err != nil {
		return schema{}, err
	}
	if len(keys) == 0 {
		return schema{}, fmt.Errorf("no key columns remain after applying key selection")
	}
	return schema{
		Header:       append([]string(nil), left.Header...),
		ColumnCount:  n,
		LeftMap:      leftMap,
		RightMap:     rightMap,
		KeyIndexes:   keys,
		KeyIsFullRow: isIdentityKey(keys, n),
	}, nil
}

func alignHeaders(left, right []string, columnCount int, hasHeader, alignByName bool) ([]int, []int, error) {
	leftMap := identityMap(columnCount)
	rightMap := identityMap(columnCount)
	if !hasHeader {
		return leftMap, rightMap, nil
	}
	if err := validateUniqueHeader(left, "left"); err != nil {
		return nil, nil, err
	}
	if err := validateUniqueHeader(right, "right"); err != nil {
		return nil, nil, err
	}
	if !alignByName {
		if !slices.Equal(left, right) {
			return nil, nil, fmt.Errorf("headers differ; enable column-name alignment or make the headers identical")
		}
		return leftMap, rightMap, nil
	}
	rightByName := make(map[string]int, columnCount)
	for i, name := range right {
		rightByName[name] = i
	}
	for i, name := range left {
		j, ok := rightByName[name]
		if !ok {
			return nil, nil, fmt.Errorf("right header is missing column %q", name)
		}
		rightMap[i] = j
	}
	return leftMap, rightMap, nil
}

func resolveIncludeKeys(header []string, columnCount int, names []string, indexes []int) ([]int, error) {
	nameToIndex := indexHeaders(header)
	seen := make(map[int]struct{}, len(names)+len(indexes))
	keys := make([]int, 0, len(names)+len(indexes))
	for _, name := range names {
		i, ok := nameToIndex[name]
		if !ok {
			return nil, fmt.Errorf("key header %q not found in left header", name)
		}
		keys = appendUniqueIndex(keys, seen, i)
	}
	for _, i := range indexes {
		if i < 0 || i >= columnCount {
			return nil, fmt.Errorf("key index %d is outside 0..%d", i, columnCount-1)
		}
		keys = appendUniqueIndex(keys, seen, i)
	}
	return keys, nil
}

func resolveExcludeKeys(header []string, columnCount int, names []string, indexes []int) ([]int, error) {
	nameToIndex := indexHeaders(header)
	excluded := make(map[int]struct{}, len(names)+len(indexes))
	for _, name := range names {
		i, ok := nameToIndex[name]
		if !ok {
			return nil, fmt.Errorf("excluded key header %q not found in left header", name)
		}
		excluded[i] = struct{}{}
	}
	for _, i := range indexes {
		if i < 0 || i >= columnCount {
			return nil, fmt.Errorf("excluded key index %d is outside 0..%d", i, columnCount-1)
		}
		excluded[i] = struct{}{}
	}
	keys := make([]int, 0, columnCount-len(excluded))
	for i := 0; i < columnCount; i++ {
		if _, skip := excluded[i]; !skip {
			keys = append(keys, i)
		}
	}
	return keys, nil
}

func indexHeaders(header []string) map[string]int {
	result := make(map[string]int, len(header))
	for i, name := range header {
		result[name] = i
	}
	return result
}

func appendUniqueIndex(indexes []int, seen map[int]struct{}, index int) []int {
	if _, ok := seen[index]; ok {
		return indexes
	}
	seen[index] = struct{}{}
	return append(indexes, index)
}

func isIdentityKey(keys []int, columnCount int) bool {
	if len(keys) != columnCount {
		return false
	}
	for i, key := range keys {
		if key != i {
			return false
		}
	}
	return true
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
