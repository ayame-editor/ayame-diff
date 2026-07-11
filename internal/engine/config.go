package engine

import (
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// ColumnTolerance applies an absolute numeric tolerance to one CSV column.
// Use either Name (header mode) or Index (with ByIndex=true).
type ColumnTolerance struct {
	Name    string  `json:"name,omitempty"`
	Index   int     `json:"index,omitempty"`
	ByIndex bool    `json:"by_index,omitempty"`
	Value   float64 `json:"value"`
}

type Config struct {
	LeftPath, RightPath, OutputPath                        string
	KeyNames                                               []string
	KeyIndexes                                             []int
	ExcludeKeyNames                                        []string
	ExcludeKeyIndexes                                      []int
	IndexBase                                              int
	HasHeader, AlignColumnsByName                          bool
	LeftFormat, RightFormat, LeftDelimiter, RightDelimiter string
	LeftParser, RightParser                                string
	LazyQuotes, TrimLeadingSpace                           bool
	Partitions, ParseWorkers, Workers                      int
	MemoryText, PartitionBufferText                        string
	MergeFanIn                                             int
	MaxRecordText                                          string
	TempDir, WorkDir                                       string
	KeepTemp, Progress                                     bool
	OutputHeader                                           bool
	IgnoreCase, IgnoreEOL, IgnoreTrailingEOL               bool
	IgnoreWhitespace                                       string
	LineFilters                                            []string
	IgnoreColumnNames                                      []string
	IgnoreColumnIndexes                                    []int
	Tolerance                                              float64
	ToleranceSet                                           bool
	ColumnTolerances                                       []ColumnTolerance
	Log                                                    io.Writer
	OnProgress                                             func(ProgressEvent)
}

// Resource limits are exported so CLI and GUI validation can share the
// engine's exact constraints instead of duplicating magic numbers.
const (
	MinPartitions           = 2
	MaxPartitions           = 1024
	MinMergeFanIn           = 2
	MaxMergeFanIn           = 256
	MinPartitionBuffer      = 4 * 1024
	MaxPartitionBuffer      = 16 * 1024 * 1024
	MinRecordBytes          = 1024
	MinMemoryBytesPerWorker = 16 * 1024 * 1024
)

// resolvedConfig is the validated, normalized form used only inside engine.
// Config remains a caller-owned description and is never mutated by resolve.
type resolvedConfig struct {
	Config
	MemoryBytes          int64
	PartitionBufferBytes int
	MaxRecordBytes       int64
	Comparison           comparisonConfig
}

type Summary struct {
	LeftRows     uint64 `json:"left_rows"`
	RightRows    uint64 `json:"right_rows"`
	EqualRows    uint64 `json:"equal_rows"`
	LeftOnly     uint64 `json:"left_only"`
	RightOnly    uint64 `json:"right_only"`
	ChangedLeft  uint64 `json:"changed_left"`
	ChangedRight uint64 `json:"changed_right"`
	DiffRows     uint64 `json:"diff_rows"`
	Partitions   int    `json:"partitions"`
	Workers      int    `json:"workers"`
	Elapsed      string `json:"elapsed"`
}

// Validate reports whether c can be resolved. It is idempotent and does not
// mutate c or any slices owned by the caller.
func (c Config) Validate() error {
	_, err := c.resolve()
	return err
}

func (c Config) resolve() (resolvedConfig, error) {
	r := resolvedConfig{Config: c}
	r.KeyNames = append([]string(nil), c.KeyNames...)
	r.KeyIndexes = append([]int(nil), c.KeyIndexes...)
	r.ExcludeKeyNames = append([]string(nil), c.ExcludeKeyNames...)
	r.ExcludeKeyIndexes = append([]int(nil), c.ExcludeKeyIndexes...)
	r.LineFilters = append([]string(nil), c.LineFilters...)
	r.IgnoreColumnNames = append([]string(nil), c.IgnoreColumnNames...)
	r.IgnoreColumnIndexes = append([]int(nil), c.IgnoreColumnIndexes...)
	r.ColumnTolerances = append([]ColumnTolerance(nil), c.ColumnTolerances...)

	if c.LeftPath == "" || c.RightPath == "" || c.OutputPath == "" {
		return resolvedConfig{}, fmt.Errorf("--left, --right, and --out are required")
	}
	if err := validateKeySelection(c); err != nil {
		return resolvedConfig{}, err
	}
	if !c.HasHeader && (len(c.KeyNames) > 0 || len(c.ExcludeKeyNames) > 0) {
		return resolvedConfig{}, fmt.Errorf("--key and --exclude-key require --header=true; use index options without a header")
	}
	if c.IndexBase != 0 && c.IndexBase != 1 {
		return resolvedConfig{}, fmt.Errorf("--index-base must be 0 or 1")
	}
	for i := range r.KeyIndexes {
		r.KeyIndexes[i] -= c.IndexBase
		if r.KeyIndexes[i] < 0 {
			return resolvedConfig{}, fmt.Errorf("key index becomes negative after applying --index-base")
		}
	}
	for i := range r.ExcludeKeyIndexes {
		r.ExcludeKeyIndexes[i] -= c.IndexBase
		if r.ExcludeKeyIndexes[i] < 0 {
			return resolvedConfig{}, fmt.Errorf("excluded key index becomes negative after applying --index-base")
		}
	}
	for i := range r.IgnoreColumnIndexes {
		r.IgnoreColumnIndexes[i] -= c.IndexBase
		if r.IgnoreColumnIndexes[i] < 0 {
			return resolvedConfig{}, fmt.Errorf("ignored column index becomes negative after applying --index-base")
		}
	}
	for i := range r.ColumnTolerances {
		if r.ColumnTolerances[i].ByIndex {
			r.ColumnTolerances[i].Index -= c.IndexBase
			if r.ColumnTolerances[i].Index < 0 {
				return resolvedConfig{}, fmt.Errorf("tolerance column index becomes negative after applying --index-base")
			}
		}
	}
	r.IndexBase = 0
	if c.IgnoreWhitespace == "" {
		r.IgnoreWhitespace = "none"
	}
	if r.IgnoreWhitespace != "none" && r.IgnoreWhitespace != "change" && r.IgnoreWhitespace != "all" {
		return resolvedConfig{}, fmt.Errorf("--ignore-whitespace must be none, change, or all")
	}
	if math.IsNaN(c.Tolerance) || math.IsInf(c.Tolerance, 0) || c.Tolerance < 0 {
		return resolvedConfig{}, fmt.Errorf("--tolerance must be a finite non-negative number")
	}
	for _, tolerance := range r.ColumnTolerances {
		if math.IsNaN(tolerance.Value) || math.IsInf(tolerance.Value, 0) || tolerance.Value < 0 {
			return resolvedConfig{}, fmt.Errorf("column tolerance must be a finite non-negative number")
		}
		if !tolerance.ByIndex && strings.TrimSpace(tolerance.Name) == "" {
			return resolvedConfig{}, fmt.Errorf("column tolerance name must not be empty")
		}
	}
	for _, pattern := range r.LineFilters {
		if _, err := regexp.Compile(pattern); err != nil {
			return resolvedConfig{}, fmt.Errorf("invalid line filter %q: %w", pattern, err)
		}
	}
	if err := ValidatePartitions(c.Partitions); err != nil {
		return resolvedConfig{}, err
	}
	if c.ParseWorkers < 1 || c.Workers < 1 {
		return resolvedConfig{}, fmt.Errorf("--parse-workers and --workers must be at least 1")
	}
	if err := ValidateMergeFanIn(c.MergeFanIn); err != nil {
		return resolvedConfig{}, err
	}
	var err error
	r.MemoryBytes, err = parseBytes(c.MemoryText)
	if err != nil {
		return resolvedConfig{}, fmt.Errorf("--memory: %w", err)
	}
	partitionBuffer, err := parseBytes(c.PartitionBufferText)
	if err != nil {
		return resolvedConfig{}, fmt.Errorf("--partition-buffer: %w", err)
	}
	if partitionBuffer < MinPartitionBuffer || partitionBuffer > MaxPartitionBuffer {
		return resolvedConfig{}, fmt.Errorf("--partition-buffer must be between 4KiB and 16MiB")
	}
	r.PartitionBufferBytes = int(partitionBuffer)
	r.MaxRecordBytes, err = parseBytes(c.MaxRecordText)
	if err != nil {
		return resolvedConfig{}, fmt.Errorf("--max-record-bytes: %w", err)
	}
	if r.MaxRecordBytes < MinRecordBytes {
		return resolvedConfig{}, fmt.Errorf("--max-record-bytes must be at least 1KiB")
	}
	minimumMemory := int64(c.Workers) * MinMemoryBytesPerWorker
	if r.MemoryBytes < minimumMemory {
		return resolvedConfig{}, fmt.Errorf("--memory must be at least %dMiB for %d workers", minimumMemory/(1024*1024), c.Workers)
	}
	if int64(c.Partitions)*int64(r.PartitionBufferBytes) > r.MemoryBytes/2 {
		return resolvedConfig{}, fmt.Errorf("partition buffers use too much memory; lower --partition-buffer or --partitions")
	}
	for name, value := range map[string]string{"--left-format": c.LeftFormat, "--right-format": c.RightFormat} {
		if value != "auto" && value != "csv" && value != "tsv" {
			return resolvedConfig{}, fmt.Errorf("%s must be auto, csv, or tsv", name)
		}
	}
	for name, value := range map[string]string{"--left-parser": c.LeftParser, "--right-parser": c.RightParser} {
		if value != "auto" && value != "simple" && value != "rfc4180" {
			return resolvedConfig{}, fmt.Errorf("%s must be auto, simple, or rfc4180", name)
		}
	}
	return r, nil
}

func validateKeySelection(c Config) error {
	includeCount := len(c.KeyNames) + len(c.KeyIndexes)
	excludeCount := len(c.ExcludeKeyNames) + len(c.ExcludeKeyIndexes)
	if includeCount > 0 && excludeCount > 0 {
		return fmt.Errorf("include and exclude key options cannot be combined")
	}
	return nil
}

// ValidatePartitions validates the hash partition count shared by all clients.
func ValidatePartitions(partitions int) error {
	if partitions < MinPartitions || partitions > MaxPartitions || partitions&(partitions-1) != 0 {
		return fmt.Errorf("--partitions must be a power of two from %d to %d", MinPartitions, MaxPartitions)
	}
	return nil
}

// ValidateMergeFanIn validates the external-sort merge fan-in.
func ValidateMergeFanIn(fanIn int) error {
	if fanIn < MinMergeFanIn || fanIn > MaxMergeFanIn {
		return fmt.Errorf("--merge-fan-in must be from %d to %d", MinMergeFanIn, MaxMergeFanIn)
	}
	return nil
}

func parseBytes(text string) (int64, error) {
	s := strings.TrimSpace(text)
	if s == "" {
		return 0, fmt.Errorf("empty size")
	}
	upper := strings.ToUpper(s)
	units := []struct {
		suffix string
		mult   int64
	}{{"TIB", 1 << 40}, {"TB", 1_000_000_000_000}, {"GIB", 1 << 30}, {"GB", 1_000_000_000}, {"MIB", 1 << 20}, {"MB", 1_000_000}, {"KIB", 1 << 10}, {"KB", 1_000}, {"B", 1}}
	mult, number := int64(1), upper
	for _, unit := range units {
		if strings.HasSuffix(upper, unit.suffix) {
			mult = unit.mult
			number = strings.TrimSpace(upper[:len(upper)-len(unit.suffix)])
			break
		}
	}
	value, err := strconv.ParseFloat(number, 64)
	if err != nil || value < 0 {
		return 0, fmt.Errorf("invalid size %q", text)
	}
	result := value * float64(mult)
	if result > float64(^uint64(0)>>1) {
		return 0, fmt.Errorf("size is too large")
	}
	return int64(result), nil
}

// ParseByteSize parses the same size syntax accepted by --memory,
// --partition-buffer, and --max-record-bytes.
func ParseByteSize(text string) (int64, error) {
	return parseBytes(text)
}
