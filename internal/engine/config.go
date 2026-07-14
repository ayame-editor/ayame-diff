package engine

import (
	"fmt"
	"strconv"
	"strings"
)

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
	MemoryBytes                                            int64
	PartitionBufferBytes                                   int
	MaxRecordBytes                                         int64
	TempDir, WorkDir                                       string
	KeepTemp, Progress                                     bool
	SummaryJSON                                            string
	DiffExitCode, OutputHeader                             bool
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

// Validate checks the configuration without modifying it. It is safe to
// call any number of times; normalization (index-base shifting and derived
// byte sizes) happens in resolved(), which returns a fresh copy.
func (c *Config) Validate() error {
	if c.LeftPath == "" || c.RightPath == "" || c.OutputPath == "" {
		return fmt.Errorf("--left, --right, and --out are required")
	}
	includeCount := len(c.KeyNames) + len(c.KeyIndexes)
	excludeCount := len(c.ExcludeKeyNames) + len(c.ExcludeKeyIndexes)
	if includeCount > 0 && excludeCount > 0 {
		return fmt.Errorf("include options (--key/--key-index) cannot be combined with exclude options (--exclude-key/--exclude-key-index)")
	}
	if !c.HasHeader && (len(c.KeyNames) > 0 || len(c.ExcludeKeyNames) > 0) {
		return fmt.Errorf("--key and --exclude-key require --header=true; use index options without a header")
	}
	if c.IndexBase != 0 && c.IndexBase != 1 {
		return fmt.Errorf("--index-base must be 0 or 1")
	}
	for _, index := range c.KeyIndexes {
		if index-c.IndexBase < 0 {
			return fmt.Errorf("key index becomes negative after applying --index-base")
		}
	}
	for _, index := range c.ExcludeKeyIndexes {
		if index-c.IndexBase < 0 {
			return fmt.Errorf("excluded key index becomes negative after applying --index-base")
		}
	}
	if c.Partitions < 2 || c.Partitions > 1024 || c.Partitions&(c.Partitions-1) != 0 {
		return fmt.Errorf("--partitions must be a power of two from 2 to 1024")
	}
	if c.ParseWorkers < 1 || c.Workers < 1 {
		return fmt.Errorf("--parse-workers and --workers must be at least 1")
	}
	if c.MergeFanIn < 2 || c.MergeFanIn > 256 {
		return fmt.Errorf("--merge-fan-in must be from 2 to 256")
	}
	memoryBytes, err := parseBytes(c.MemoryText)
	if err != nil {
		return fmt.Errorf("--memory: %w", err)
	}
	partitionBuffer, err := parseBytes(c.PartitionBufferText)
	if err != nil {
		return fmt.Errorf("--partition-buffer: %w", err)
	}
	if partitionBuffer < 4*1024 || partitionBuffer > 16*1024*1024 {
		return fmt.Errorf("--partition-buffer must be between 4KiB and 16MiB")
	}
	maxRecordBytes, err := parseBytes(c.MaxRecordText)
	if err != nil {
		return fmt.Errorf("--max-record-bytes: %w", err)
	}
	if maxRecordBytes < 1024 {
		return fmt.Errorf("--max-record-bytes must be at least 1KiB")
	}
	minimumMemory := int64(c.Workers) * 16 * 1024 * 1024
	if memoryBytes < minimumMemory {
		return fmt.Errorf("--memory must be at least %dMiB for %d workers", minimumMemory/(1024*1024), c.Workers)
	}
	if int64(c.Partitions)*partitionBuffer > memoryBytes/2 {
		return fmt.Errorf("partition buffers use too much memory; lower --partition-buffer or --partitions")
	}
	for name, value := range map[string]string{"--left-format": c.LeftFormat, "--right-format": c.RightFormat} {
		if value != "auto" && value != "csv" && value != "tsv" {
			return fmt.Errorf("%s must be auto, csv, or tsv", name)
		}
	}
	for name, value := range map[string]string{"--left-parser": c.LeftParser, "--right-parser": c.RightParser} {
		if value != "auto" && value != "simple" && value != "rfc4180" {
			return fmt.Errorf("%s must be auto, simple, or rfc4180", name)
		}
	}
	return nil
}

// resolved validates the configuration and returns a normalized copy:
// key indexes are rebased to zero using IndexBase (into fresh slices, so
// the caller's slices are never written through) and the derived byte
// sizes are populated. Calling resolved twice on the same source Config
// yields the same result; the source is never modified.
func (c Config) resolved() (Config, error) {
	if err := c.Validate(); err != nil {
		return Config{}, err
	}
	out := c
	if len(c.KeyIndexes) > 0 {
		out.KeyIndexes = make([]int, len(c.KeyIndexes))
		for i, index := range c.KeyIndexes {
			out.KeyIndexes[i] = index - c.IndexBase
		}
	}
	if len(c.ExcludeKeyIndexes) > 0 {
		out.ExcludeKeyIndexes = make([]int, len(c.ExcludeKeyIndexes))
		for i, index := range c.ExcludeKeyIndexes {
			out.ExcludeKeyIndexes[i] = index - c.IndexBase
		}
	}
	out.IndexBase = 0
	var err error
	if out.MemoryBytes, err = parseBytes(c.MemoryText); err != nil {
		return Config{}, fmt.Errorf("--memory: %w", err)
	}
	partitionBuffer, err := parseBytes(c.PartitionBufferText)
	if err != nil {
		return Config{}, fmt.Errorf("--partition-buffer: %w", err)
	}
	out.PartitionBufferBytes = int(partitionBuffer)
	if out.MaxRecordBytes, err = parseBytes(c.MaxRecordText); err != nil {
		return Config{}, fmt.Errorf("--max-record-bytes: %w", err)
	}
	return out, nil
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
