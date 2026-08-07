package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ayame-editor/ayame-diff/internal/engine"
	"github.com/ayame-editor/ayame-diff/internal/pathutil"
)

type csvRequest struct {
	Old                 string                   `json:"old"`
	New                 string                   `json:"new"`
	HasHeader           bool                     `json:"hasHeader"`
	AlignColumnsByName  bool                     `json:"alignColumnsByName"`
	KeyMode             string                   `json:"keyMode"`
	KeyNames            []string                 `json:"keyNames"`
	KeyIndexes          []int                    `json:"keyIndexes"`
	ExcludeKeyNames     []string                 `json:"excludeKeyNames"`
	ExcludeKeyIndexes   []int                    `json:"excludeKeyIndexes"`
	IndexBase           int                      `json:"indexBase"`
	LeftFormat          string                   `json:"leftFormat"`
	RightFormat         string                   `json:"rightFormat"`
	LeftDelimiter       string                   `json:"leftDelimiter"`
	RightDelimiter      string                   `json:"rightDelimiter"`
	LeftParser          string                   `json:"leftParser"`
	RightParser         string                   `json:"rightParser"`
	LazyQuotes          bool                     `json:"lazyQuotes"`
	TrimLeadingSpace    bool                     `json:"trimLeadingSpace"`
	IgnoreCase          bool                     `json:"ignoreCase"`
	Whitespace          string                   `json:"whitespace"`
	LineFilters         []string                 `json:"lineFilters"`
	IgnoreColumnNames   []string                 `json:"ignoreColumnNames"`
	IgnoreColumnIndexes []int                    `json:"ignoreColumnIndexes"`
	Tolerance           *float64                 `json:"tolerance"`
	ColumnTolerances    []engine.ColumnTolerance `json:"columnTolerances"`
	Partitions          int                      `json:"partitions"`
	ParseWorkers        int                      `json:"parseWorkers"`
	Workers             int                      `json:"workers"`
	Memory              string                   `json:"memory"`
	PartitionBuffer     string                   `json:"partitionBuffer"`
	MergeFanIn          int                      `json:"mergeFanIn"`
	MaxRecordBytes      string                   `json:"maxRecordBytes"`
	TempDir             string                   `json:"tempDir"`
	KeepTemp            bool                     `json:"keepTemp"`
	MaxRows             int                      `json:"maxRows"`
	Output              string                   `json:"output"`
	OutputFormat        string                   `json:"outputFormat"`
	OutputHeader        bool                     `json:"outputHeader"`
	ProjectPath         string                   `json:"projectPath"`
}

func requestFromConfig(cfg engine.Config) csvRequest {
	var tolerance *float64
	if cfg.ToleranceSet {
		value := cfg.Tolerance
		tolerance = &value
	}
	keyMode := "all"
	if len(cfg.KeyNames)+len(cfg.KeyIndexes) > 0 {
		keyMode = "include"
	}
	if len(cfg.ExcludeKeyNames)+len(cfg.ExcludeKeyIndexes) > 0 {
		keyMode = "exclude"
	}
	return csvRequest{
		Old: cfg.LeftPath, New: cfg.RightPath, HasHeader: cfg.HasHeader, AlignColumnsByName: cfg.AlignColumnsByName, KeyMode: keyMode,
		KeyNames: cfg.KeyNames, KeyIndexes: cfg.KeyIndexes, ExcludeKeyNames: cfg.ExcludeKeyNames, ExcludeKeyIndexes: cfg.ExcludeKeyIndexes, IndexBase: cfg.IndexBase,
		LeftFormat: cfg.LeftFormat, RightFormat: cfg.RightFormat, LeftDelimiter: cfg.LeftDelimiter, RightDelimiter: cfg.RightDelimiter,
		LeftParser: cfg.LeftParser, RightParser: cfg.RightParser, LazyQuotes: cfg.LazyQuotes, TrimLeadingSpace: cfg.TrimLeadingSpace,
		IgnoreCase: cfg.IgnoreCase, Whitespace: cfg.IgnoreWhitespace, LineFilters: cfg.LineFilters,
		IgnoreColumnNames: cfg.IgnoreColumnNames, IgnoreColumnIndexes: cfg.IgnoreColumnIndexes, Tolerance: tolerance, ColumnTolerances: cfg.ColumnTolerances,
		Partitions: cfg.Partitions, ParseWorkers: cfg.ParseWorkers, Workers: cfg.Workers, Memory: cfg.MemoryText, PartitionBuffer: cfg.PartitionBufferText,
		MergeFanIn: cfg.MergeFanIn, MaxRecordBytes: cfg.MaxRecordText, TempDir: cfg.TempDir, KeepTemp: cfg.KeepTemp,
		Output: cfg.OutputPath, OutputFormat: cfg.OutputFormat, OutputHeader: cfg.OutputHeader,
	}
}

type csvCellChange struct {
	Index int    `json:"index"`
	Name  string `json:"name"`
	Old   string `json:"old"`
	New   string `json:"new"`
}
type csvDifference struct {
	ID             string          `json:"id"`
	Kind           string          `json:"kind"`
	Old            []string        `json:"old,omitempty"`
	New            []string        `json:"new,omitempty"`
	ChangedColumns []csvCellChange `json:"changed_columns,omitempty"`
}
type csvResponse struct {
	Header          []string               `json:"header"`
	Inspection      engine.InputInspection `json:"inspection"`
	Summary         engine.Summary         `json:"summary"`
	Differences     []csvDifference        `json:"differences"`
	Truncated       bool                   `json:"truncated"`
	DifferenceCount int                    `json:"difference_count"`
}

// clampMemoryBudget lowers an over-limit resident-memory request to the server
// cap (#170). Using a smaller budget only makes the engine spill more, so the
// comparison result is unchanged; malformed values pass through to be rejected
// by engine.Validate with a clear error.
func clampMemoryBudget(text string) string {
	limit, err := engine.ParseByteSize(serverMaxMemoryText)
	if err != nil {
		return text
	}
	value, err := engine.ParseByteSize(strings.TrimSpace(text))
	if err != nil || value <= limit {
		return text
	}
	return serverMaxMemoryText
}

func csvConfig(req csvRequest, output string) engine.Config {
	workers := min(runtime.NumCPU(), 8)
	if req.ParseWorkers <= 0 {
		req.ParseWorkers = workers
	}
	if req.Workers <= 0 {
		req.Workers = workers
	}
	if req.Partitions == 0 {
		req.Partitions = 8
	}
	if req.MergeFanIn == 0 {
		req.MergeFanIn = 8
	}
	if req.Memory == "" {
		req.Memory = "512MiB"
	}
	req.Memory = clampMemoryBudget(req.Memory)
	if req.PartitionBuffer == "" {
		req.PartitionBuffer = "64KiB"
	}
	if req.MaxRecordBytes == "" {
		req.MaxRecordBytes = "256MiB"
	}
	if req.LeftFormat == "" {
		req.LeftFormat = "auto"
	}
	if req.RightFormat == "" {
		req.RightFormat = "auto"
	}
	if req.LeftParser == "" {
		req.LeftParser = "auto"
	}
	if req.RightParser == "" {
		req.RightParser = "auto"
	}
	if req.Whitespace == "" {
		req.Whitespace = "none"
	}
	cfg := engine.Config{
		LeftPath: req.Old, RightPath: req.New, OutputPath: output,
		KeyNames: req.KeyNames, KeyIndexes: req.KeyIndexes, ExcludeKeyNames: req.ExcludeKeyNames, ExcludeKeyIndexes: req.ExcludeKeyIndexes,
		IndexBase: req.IndexBase, HasHeader: req.HasHeader, AlignColumnsByName: req.AlignColumnsByName,
		LeftFormat: req.LeftFormat, RightFormat: req.RightFormat, LeftDelimiter: req.LeftDelimiter, RightDelimiter: req.RightDelimiter,
		LeftParser: req.LeftParser, RightParser: req.RightParser, LazyQuotes: req.LazyQuotes, TrimLeadingSpace: req.TrimLeadingSpace,
		IgnoreCase: req.IgnoreCase, IgnoreWhitespace: req.Whitespace, LineFilters: req.LineFilters,
		IgnoreColumnNames: req.IgnoreColumnNames, IgnoreColumnIndexes: req.IgnoreColumnIndexes,
		ColumnTolerances: req.ColumnTolerances, Partitions: req.Partitions, ParseWorkers: req.ParseWorkers, Workers: req.Workers,
		MemoryText: req.Memory, PartitionBufferText: req.PartitionBuffer, MergeFanIn: req.MergeFanIn, MaxRecordText: req.MaxRecordBytes,
		TempDir: req.TempDir, KeepTemp: req.KeepTemp, OutputHeader: false, CellDiff: true, OutputFormat: "jsonl",
	}
	if req.Tolerance != nil {
		cfg.Tolerance, cfg.ToleranceSet = *req.Tolerance, true
	}
	return cfg
}

func decodeCSVRequest(w http.ResponseWriter, r *http.Request) (csvRequest, bool) {
	req, ok := decodePostJSON[csvRequest](w, r, "")
	if !ok {
		return csvRequest{}, false
	}
	if req.Old == "" || req.New == "" {
		writeError(w, http.StatusBadRequest, "both left and right paths are required")
		return csvRequest{}, false
	}
	return req, true
}

func validateCSVKeys(w http.ResponseWriter, req csvRequest) bool {
	if req.KeyMode == "include" && len(req.KeyNames)+len(req.KeyIndexes) == 0 {
		writeError(w, http.StatusBadRequest, "select at least one key column")
		return false
	}
	return true
}

func (s *Server) handleCSVInspect(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeCSVRequest(w, r)
	if !ok {
		return
	}
	inspection, err := engine.InspectInputs(csvConfig(req, "inspect.tmp"))
	if err != nil {
		writeClassifiedError(w, err, http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, inspection)
}

func (s *Server) handleCSVDiff(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeCSVRequest(w, r)
	if !ok {
		return
	}
	if !validateCSVKeys(w, req) {
		return
	}
	dir, err := os.MkdirTemp("", "ayame-diff-gui-")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer os.RemoveAll(dir)
	output := filepath.Join(dir, "diff.jsonl")
	cfg := csvConfig(req, output)
	inspection, err := engine.InspectInputs(cfg)
	if err != nil {
		writeClassifiedError(w, err, http.StatusBadRequest)
		return
	}
	summary, err := engine.Run(r.Context(), cfg)
	if err != nil {
		writeClassifiedError(w, err, http.StatusInternalServerError)
		return
	}
	file, err := os.Open(output)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer file.Close()
	maxRows := req.MaxRows
	if maxRows <= 0 {
		maxRows = 500
	}
	if maxRows > 5000 {
		maxRows = 5000
	}
	response := csvResponse{Header: inspection.Header, Inspection: inspection, Summary: summary}
	// Counting distinct differences with a set grew without bound: the loop
	// keeps draining the engine's output after maxRows, so a comparison of two
	// large, maximally divergent files sized the set by the input rather than by
	// anything the server controls — roughly 70MB per million differing rows,
	// with a transient doubling while the map rehashed (#156).
	//
	// A set is not needed. The engine sorts records within each key group and a
	// key group never spans partitions, so identical rows — the only source of a
	// repeated ID, which hashes kind, key, and row — are always emitted
	// consecutively. Comparing against the previous ID is therefore exact, in
	// constant memory. TestCSVDifferenceCountDedupesInConstantMemory pins that
	// emission order, since it is an assumption about the engine rather than
	// something this handler can see.
	previousID, havePrevious := "", false
	decoder := json.NewDecoder(file)
	for {
		var difference csvDifference
		err := decoder.Decode(&difference)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !havePrevious || difference.ID != previousID {
			previousID, havePrevious = difference.ID, true
			response.DifferenceCount++
		}
		if len(response.Differences) >= maxRows {
			response.Truncated = true
			continue
		}
		response.Differences = append(response.Differences, difference)
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleCSVExport(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeCSVRequest(w, r)
	if !ok {
		return
	}
	if !validateCSVKeys(w, req) {
		return
	}
	if strings.TrimSpace(req.Output) == "" {
		writeError(w, http.StatusBadRequest, "output path is required")
		return
	}
	cfg := csvConfig(req, req.Output)
	cfg.OutputFormat, cfg.OutputHeader = req.OutputFormat, req.OutputHeader
	if cfg.OutputFormat == "" {
		cfg.OutputFormat = "tsv"
	}
	if err := cfg.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	summary, err := engine.Run(r.Context(), cfg)
	if err != nil {
		writeClassifiedError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Output  string         `json:"output"`
		Summary engine.Summary `json:"summary"`
	}{Output: req.Output, Summary: summary})
}

type csvMergeRequest struct {
	csvRequest
	Choices          map[string]string `json:"choices"`
	DefaultChoice    string            `json:"defaultChoice"`
	AllowUnresolved  bool              `json:"allowUnresolved"`
	Overwrite        bool              `json:"overwrite"`
	ConfirmOverwrite bool              `json:"confirmOverwrite"`
}

func (s *Server) handleCSVMerge(w http.ResponseWriter, r *http.Request) {
	req, ok := decodePostJSON[csvMergeRequest](w, r, "")
	if !ok {
		return
	}
	if req.Old == "" || req.New == "" || strings.TrimSpace(req.Output) == "" {
		writeError(w, http.StatusBadRequest, "left, right, and output paths are required")
		return
	}
	if !validateCSVKeys(w, req.csvRequest) {
		return
	}
	overwriteInput := pathutil.Equal(req.Output, req.Old) || pathutil.Equal(req.Output, req.New)
	if overwriteInput && (!req.Overwrite || !req.ConfirmOverwrite) {
		writeError(w, http.StatusBadRequest, "overwriting an input requires overwrite and explicit confirmation")
		return
	}
	target := req.Output
	if overwriteInput {
		temp, err := os.CreateTemp(filepath.Dir(req.Output), ".ayame-diff-csv-merge-*"+filepath.Ext(req.Output))
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		target = temp.Name()
		_ = temp.Close()
		_ = os.Remove(target)
		defer os.Remove(target)
	}
	cfg := csvConfig(req.csvRequest, target)
	cfg.OutputFormat, cfg.OutputHeader = "tsv", req.HasHeader
	cfg.Reconcile, cfg.MergeChoices, cfg.MergeDefault = true, req.Choices, req.DefaultChoice
	cfg.AllowUnresolved = req.AllowUnresolved
	if strings.HasSuffix(strings.TrimSuffix(strings.ToLower(req.Output), ".gz"), ".csv") {
		cfg.OutputDelimiter = ','
	} else {
		cfg.OutputDelimiter = '\t'
	}
	if err := cfg.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	summary, err := engine.Run(r.Context(), cfg)
	if err != nil {
		writeClassifiedError(w, err, http.StatusInternalServerError)
		return
	}
	if overwriteInput {
		// os.Rename atomically replaces an existing destination on every
		// platform (Windows uses MoveFileEx with MOVEFILE_REPLACE_EXISTING), so
		// the staged merge output swaps into place in one step. The previous
		// Windows-only os.Remove(req.Output) before the rename opened a window
		// where a failing rename left the input gone with no staged copy. (#171)
		if err := os.Rename(target, req.Output); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, struct {
		Output  string         `json:"output"`
		Summary engine.Summary `json:"summary"`
	}{req.Output, summary})
}
