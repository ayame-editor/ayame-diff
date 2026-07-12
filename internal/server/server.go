// Package server hosts a small local web UI for ayame-diff: it serves an
// embedded single-page frontend and a JSON diff API over localhost. It is the
// GUI foundation (hjosugi/ayame-diff#10); the diff view lives in the embedded
// web assets (#11). Dependency-zero: net/http, embed, and the diff packages.
package server

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hjosugi/ayame-diff/internal/diffout"
	"github.com/hjosugi/ayame-diff/internal/dircompare"
	"github.com/hjosugi/ayame-diff/internal/engine"
	"github.com/hjosugi/ayame-diff/internal/linediff"
	"github.com/hjosugi/ayame-diff/internal/linesort"
	"github.com/hjosugi/ayame-diff/internal/linesrc"
	"github.com/hjosugi/ayame-diff/internal/merge"
	"github.com/hjosugi/ayame-diff/internal/project"
	"github.com/hjosugi/ayame-diff/internal/threeway"
)

//go:embed web
var webFS embed.FS

// Server serves the UI and diff API.
type Server struct {
	version string
	mux     *http.ServeMux
	dropMu  sync.Mutex
	drops   map[string]string
}

// New returns a Server. version is reported by /api/health.
func New(version string) (*Server, error) {
	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		return nil, err
	}
	s := &Server{version: version, mux: http.NewServeMux(), drops: make(map[string]string)}
	s.mux.Handle("/", http.FileServer(http.FS(sub)))
	s.mux.HandleFunc("/api/health", s.handleHealth)
	s.mux.HandleFunc("/api/diff", s.handleDiff)
	s.mux.HandleFunc("/api/patch", s.handlePatch)
	s.mux.HandleFunc("/api/merge/text", s.handleTextMerge)
	s.mux.HandleFunc("/api/three-way/text", s.handleThreeWayText)
	s.mux.HandleFunc("/api/merge/three-way/text", s.handleThreeWayTextMerge)
	s.mux.HandleFunc("/api/csv/inspect", s.handleCSVInspect)
	s.mux.HandleFunc("/api/csv/diff", s.handleCSVDiff)
	s.mux.HandleFunc("/api/csv/export", s.handleCSVExport)
	s.mux.HandleFunc("/api/merge/csv", s.handleCSVMerge)
	s.mux.HandleFunc("/api/three-way/csv", s.handleThreeWayCSV)
	s.mux.HandleFunc("/api/merge/three-way/csv", s.handleThreeWayCSVMerge)
	s.mux.HandleFunc("/api/files", s.handleFiles)
	s.mux.HandleFunc("/api/path-info", s.handlePathInfo)
	s.mux.HandleFunc("/api/drop", s.handleDrop)
	s.mux.HandleFunc("/api/project/save", s.handleProjectSave)
	s.mux.HandleFunc("/api/project/load", s.handleProjectLoad)
	s.mux.HandleFunc("/api/dir/diff", s.handleDirDiff)
	return s, nil
}

// handleDrop streams browser-dropped files to a private local cache. Browsers
// intentionally hide native absolute paths, so the GUI cannot otherwise pass
// a dropped File to the existing path-based comparison engines.
func (s *Server) handleDrop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "use POST")
		return
	}
	session := r.URL.Query().Get("session")
	relative := filepath.Clean(filepath.FromSlash(r.URL.Query().Get("relative")))
	if session == "" || strings.ContainsAny(session, `/\\`) || relative == "." || filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		writeError(w, http.StatusBadRequest, "safe session and relative path are required")
		return
	}
	root, err := s.dropRoot(session)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	target := filepath.Join(root, relative)
	if r.URL.Query().Get("directory") == "1" {
		err = os.MkdirAll(target, 0o700)
	} else {
		if err = os.MkdirAll(filepath.Dir(target), 0o700); err == nil {
			var file *os.File
			file, err = os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
			if err == nil {
				_, copyErr := io.Copy(file, r.Body)
				closeErr := file.Close()
				if copyErr != nil {
					err = copyErr
				} else if closeErr != nil {
					err = closeErr
				}
			}
		}
	}
	if err != nil {
		_ = os.Remove(target)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Path string `json:"path"`
	}{target})
}

func (s *Server) dropRoot(session string) (string, error) {
	s.dropMu.Lock()
	defer s.dropMu.Unlock()
	if root := s.drops[session]; root != "" {
		return root, nil
	}
	base, err := os.UserCacheDir()
	if err != nil {
		base = os.TempDir()
	}
	base = filepath.Join(base, "ayame-diff", "drops")
	if err := os.MkdirAll(base, 0o700); err != nil {
		return "", err
	}
	// Clear stale browser drop sessions opportunistically.
	if entries, readErr := os.ReadDir(base); readErr == nil {
		for _, entry := range entries {
			if info, infoErr := entry.Info(); infoErr == nil && time.Since(info.ModTime()) > 24*time.Hour {
				_ = os.RemoveAll(filepath.Join(base, entry.Name()))
			}
		}
	}
	root, err := os.MkdirTemp(base, "session-")
	if err == nil {
		s.drops[session] = root
	}
	return root, err
}

func (s *Server) handlePathInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "use GET")
		return
	}
	path := strings.TrimSpace(r.URL.Query().Get("path"))
	if path == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	info, err := os.Stat(absolute)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Path      string `json:"path"`
		Directory bool   `json:"directory"`
	}{absolute, info.IsDir()})
}

type dirRequest struct {
	Old                  string   `json:"old"`
	New                  string   `json:"new"`
	Includes             []string `json:"includes"`
	Excludes             []string `json:"excludes"`
	Hidden               bool     `json:"hidden"`
	Quick                bool     `json:"quick"`
	Workers              int      `json:"workers"`
	MaxArchiveEntryBytes string   `json:"maxArchiveEntryBytes"`
	MaxArchiveBytes      string   `json:"maxArchiveBytes"`
}
type dirEntryResponse struct {
	Path     string `json:"path"`
	Status   string `json:"status"`
	OldSize  int64  `json:"old_size"`
	NewSize  int64  `json:"new_size"`
	OldMTime string `json:"old_mtime"`
	NewMTime string `json:"new_mtime"`
}

func (s *Server) handleDirDiff(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "use POST")
		return
	}
	var req dirRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Old == "" || req.New == "" {
		writeError(w, http.StatusBadRequest, "old and new directory paths are required")
		return
	}
	entryLimit, totalLimit, err := parseArchiveLimits(req.MaxArchiveEntryBytes, req.MaxArchiveBytes)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := dircompare.CompareAny(req.Old, req.New, dircompare.Options{
		Includes: req.Includes, Excludes: req.Excludes, IncludeHidden: req.Hidden,
		Quick: req.Quick, Workers: req.Workers,
		MaxArchiveEntryBytes: entryLimit, MaxArchiveBytes: totalLimit,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	entries := make([]dirEntryResponse, len(result.Entries))
	for i, entry := range result.Entries {
		entries[i] = dirEntryResponse{entry.Path, entry.Status.String(), entry.OldSize, entry.NewSize, formatTime(entry.OldModTime), formatTime(entry.NewModTime)}
	}
	writeJSON(w, http.StatusOK, struct {
		Added   int                `json:"added"`
		Removed int                `json:"removed"`
		Changed int                `json:"changed"`
		Same    int                `json:"same"`
		Entries []dirEntryResponse `json:"entries"`
	}{result.Added, result.Removed, result.Changed, result.Same, entries})
}

func parseArchiveLimits(entryText, totalText string) (int64, int64, error) {
	entryLimit := dircompare.DefaultMaxArchiveEntryBytes
	totalLimit := dircompare.DefaultMaxArchiveBytes
	for _, item := range []struct {
		name string
		text string
		out  *int64
	}{
		{"maxArchiveEntryBytes", strings.TrimSpace(entryText), &entryLimit},
		{"maxArchiveBytes", strings.TrimSpace(totalText), &totalLimit},
	} {
		if item.text == "" {
			continue
		}
		value, err := engine.ParseByteSize(item.text)
		if err != nil {
			return 0, 0, fmt.Errorf("%s: %w", item.name, err)
		}
		if value < 1 {
			return 0, 0, fmt.Errorf("%s must be at least 1 byte", item.name)
		}
		*item.out = value
	}
	if entryLimit > totalLimit {
		return 0, 0, fmt.Errorf("maxArchiveEntryBytes cannot exceed maxArchiveBytes")
	}
	return entryLimit, totalLimit, nil
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

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
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "use POST")
		return csvRequest{}, false
	}
	var req csvRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return csvRequest{}, false
	}
	if req.Old == "" || req.New == "" {
		writeError(w, http.StatusBadRequest, "both 'old' and 'new' paths are required")
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
		writeError(w, http.StatusBadRequest, err.Error())
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
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	summary, err := engine.Run(r.Context(), cfg)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, r.Context().Err()) {
			status = 499
		}
		writeError(w, status, err.Error())
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
	mergeIDs := make(map[string]struct{})
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
		if _, seen := mergeIDs[difference.ID]; !seen {
			mergeIDs[difference.ID] = struct{}{}
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
	summary, err := engine.Run(r.Context(), cfg)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
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
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "use POST")
		return
	}
	var req csvMergeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Old == "" || req.New == "" || strings.TrimSpace(req.Output) == "" {
		writeError(w, http.StatusBadRequest, "old, new, and output paths are required")
		return
	}
	if !validateCSVKeys(w, req.csvRequest) {
		return
	}
	overwriteInput := pathsEqual(req.Output, req.Old) || pathsEqual(req.Output, req.New)
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
	summary, err := engine.Run(r.Context(), cfg)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if overwriteInput {
		if runtime.GOOS == "windows" {
			if err := os.Remove(req.Output); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
		}
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

func pathsEqual(a, b string) bool {
	aa, errA := filepath.Abs(a)
	bb, errB := filepath.Abs(b)
	return errA == nil && errB == nil && filepath.Clean(aa) == filepath.Clean(bb)
}

func (s *Server) handleProjectSave(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeCSVRequest(w, r)
	if !ok {
		return
	}
	if req.ProjectPath == "" || req.Output == "" {
		writeError(w, http.StatusBadRequest, "project and output paths are required")
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
	if err := project.Save(req.ProjectPath, project.Project{Mode: "csv", CSV: cfg, Report: project.Report{CellDiff: true, OutputFormat: cfg.OutputFormat}}); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"path": req.ProjectPath})
}

func (s *Server) handleProjectLoad(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "use POST")
		return
	}
	var body struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Path == "" {
		writeError(w, http.StatusBadRequest, "project path is required")
		return
	}
	loaded, err := project.Load(body.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	req := requestFromConfig(loaded.CSV)
	req.ProjectPath = body.Path
	writeJSON(w, http.StatusOK, req)
}

type fileEntry struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Directory bool   `json:"directory"`
	Size      int64  `json:"size,omitempty"`
}

func (s *Server) handleFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "use GET")
		return
	}
	path := strings.TrimSpace(r.URL.Query().Get("path"))
	if path == "" {
		path, _ = os.Getwd()
	}
	if path == "~" || strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	result := struct {
		Path    string      `json:"path"`
		Parent  string      `json:"parent"`
		Entries []fileEntry `json:"entries"`
	}{Path: abs, Parent: filepath.Dir(abs)}
	for _, entry := range entries {
		if len(result.Entries) == 2000 {
			break
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		result.Entries = append(result.Entries, fileEntry{Name: entry.Name(), Path: filepath.Join(abs, entry.Name()), Directory: entry.IsDir(), Size: info.Size()})
	}
	writeJSON(w, http.StatusOK, result)
}

// Handler exposes the routes for http.ListenAndServe (and tests).
func (s *Server) Handler() http.Handler { return s.mux }

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": s.version})
}

// diffRequest is the POST body for /api/diff.
type diffRequest struct {
	Old               string               `json:"old"`
	New               string               `json:"new"`
	Mode              string               `json:"mode"` // "text" (default) or "sorted"
	Encoding          string               `json:"encoding"`
	Window            uint64               `json:"window"`
	MaxHunks          int                  `json:"maxHunks"`
	MaxLines          uint64               `json:"maxLines"`
	Numeric           bool                 `json:"numeric"`
	Reverse           bool                 `json:"reverse"`
	IgnoreCase        bool                 `json:"ignoreCase"`
	Whitespace        string               `json:"whitespace"` // none | change | all
	IgnoreEOL         bool                 `json:"ignoreEOL"`
	IgnoreTrailingEOL bool                 `json:"ignoreTrailingEOL"`
	LineFilters       []string             `json:"lineFilters,omitempty"`
	PatchFormat       string               `json:"patchFormat,omitempty"`
	Context           *int                 `json:"context,omitempty"`
	DetectMoves       bool                 `json:"detectMoves,omitempty"`
	MoveMinLines      uint64               `json:"moveMinLines,omitempty"`
	SyncPoints        []linediff.SyncPoint `json:"syncPoints,omitempty"`
	IgnoredHunks      []int                `json:"ignoredHunks,omitempty"`
	// Inline compares OldText/NewText directly instead of the Old/New paths —
	// "scratch" comparison of pasted text (#55).
	Inline  bool   `json:"inline"`
	OldText string `json:"oldText"`
	NewText string `json:"newText"`
}

// hunkOut is a hunk with its (capped) line text, ready for the frontend.
type hunkOut struct {
	Kind     string   `json:"kind"`
	OldStart uint64   `json:"old_start"`
	OldLen   uint64   `json:"old_len"`
	NewStart uint64   `json:"new_start"`
	NewLen   uint64   `json:"new_len"`
	Old      []string `json:"old"`
	New      []string `json:"new"`
	MoveID   uint64   `json:"move_id,omitempty"`
	MovePeer *uint64  `json:"move_peer,omitempty"`
}

type diffResponse struct {
	OldLines     uint64    `json:"old_lines"`
	NewLines     uint64    `json:"new_lines"`
	Hunks        []hunkOut `json:"hunks"`
	HunkCount    uint64    `json:"hunk_count"`
	OmittedHunks uint64    `json:"omitted_hunks"`
	Added        uint64    `json:"added"`
	Deleted      uint64    `json:"deleted"`
	Modified     uint64    `json:"modified"`
	MovedBlocks  uint64    `json:"moved_blocks,omitempty"`
	MovedLines   uint64    `json:"moved_lines,omitempty"`
	IgnoredHunks uint64    `json:"ignored_hunks,omitempty"`
}

func (s *Server) handleDiff(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "use POST")
		return
	}
	var req diffRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if !req.Inline && (req.Old == "" || req.New == "") {
		writeError(w, http.StatusBadRequest, "both 'old' and 'new' paths are required")
		return
	}
	window := req.Window
	if window == 0 {
		window = 128
	}
	maxHunks := req.MaxHunks
	if maxHunks <= 0 {
		maxHunks = 200
	}
	maxLines := req.MaxLines
	if maxLines == 0 {
		maxLines = 200
	}

	oldLines, newLines, closeLines, err := openRequestLines(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	defer closeLines()
	if err := linediff.ValidateSyncPoints(req.SyncPoints, oldLines.Count(), newLines.Count()); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	options, err := requestDiffOptions(req, maxHunks, window)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	res := linediff.DiffWith(oldLines, newLines, options)
	if req.DetectMoves {
		linediff.DetectMoves(oldLines, newLines, &res, linediff.MoveOptions{
			MinLines: req.MoveMinLines, MaxCandidates: 10_000,
		})
	}
	linediff.IgnoreHunks(&res, req.IgnoredHunks)
	writeJSON(w, http.StatusOK, buildResponse(oldLines, newLines, res, maxLines))
}

func (s *Server) handlePatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "use POST")
		return
	}
	var req diffRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if !req.Inline && (req.Old == "" || req.New == "") {
		writeError(w, http.StatusBadRequest, "both 'old' and 'new' paths are required")
		return
	}
	if req.Mode == "sorted" {
		writeError(w, http.StatusBadRequest, "patch export requires text mode; sorted output cannot be applied to the original file")
		return
	}
	format := diffout.PatchUnified
	switch req.PatchFormat {
	case "", "unified":
	case "context":
		format = diffout.PatchContext
	case "normal":
		format = diffout.Normal
	default:
		writeError(w, http.StatusBadRequest, "patchFormat must be normal, context, or unified")
		return
	}
	contextLines := 3
	if req.Context != nil {
		contextLines = *req.Context
	}
	if contextLines < 0 {
		writeError(w, http.StatusBadRequest, "context must be non-negative")
		return
	}
	oldLines, newLines, closeLines, err := openRequestLines(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	defer closeLines()
	if err := linediff.ValidateSyncPoints(req.SyncPoints, oldLines.Count(), newLines.Count()); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := diffout.ValidatePatchable(oldLines, newLines); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	window := req.Window
	if window == 0 {
		window = 128
	}
	options, err := requestDiffOptions(req, math.MaxInt, window)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	res := linediff.DiffWith(oldLines, newLines, options)
	if req.DetectMoves {
		linediff.DetectMoves(oldLines, newLines, &res, linediff.MoveOptions{
			MinLines: req.MoveMinLines, MaxCandidates: 10_000,
		})
	}
	linediff.IgnoreHunks(&res, req.IgnoredHunks)
	oldLabel, newLabel := req.Old, req.New
	if req.Inline {
		oldLabel, newLabel = "old.txt", "new.txt"
	}
	w.Header().Set("Content-Type", "text/x-diff; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="ayame.patch"`)
	w.Header().Set("X-Ayame-Ignored-Hunks", fmt.Sprint(res.IgnoredHunks))
	_ = diffout.Write(w, io.Discard, oldLines, newLines, res, diffout.Options{
		Format: format, Context: contextLines, ContextSet: true,
		OldLabel: oldLabel, NewLabel: newLabel,
		OldTime: pathModTime(req.Old), NewTime: pathModTime(req.New),
	})
}

type textMergeRequest struct {
	diffRequest
	Output           string            `json:"output"`
	Choices          map[string]string `json:"choices"`
	DefaultChoice    string            `json:"defaultChoice"`
	AllowUnresolved  bool              `json:"allowUnresolved"`
	Overwrite        bool              `json:"overwrite"`
	ConfirmOverwrite bool              `json:"confirmOverwrite"`
}

func (s *Server) handleTextMerge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "use POST")
		return
	}
	var req textMergeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Mode == "sorted" {
		writeError(w, http.StatusBadRequest, "sorted comparisons cannot be merged back to the original order")
		return
	}
	if !req.Inline && (req.Old == "" || req.New == "") {
		writeError(w, http.StatusBadRequest, "both 'old' and 'new' paths are required")
		return
	}
	oldLines, newLines, closeLines, err := openRequestLines(req.diffRequest)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	defer closeLines()
	window := req.Window
	if window == 0 {
		window = 128
	}
	options, err := requestDiffOptions(req.diffRequest, math.MaxInt, window)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	result := linediff.DiffWith(oldLines, newLines, options)
	choices := make(map[int]merge.Side, len(req.Choices))
	for key, value := range req.Choices {
		index, parseErr := strconv.Atoi(key)
		if parseErr != nil || index < 0 || index >= len(result.Hunks) || (value != string(merge.Left) && value != string(merge.Right)) {
			writeError(w, http.StatusBadRequest, "invalid merge choice "+key+"="+value)
			return
		}
		choices[index] = merge.Side(value)
	}
	if req.DefaultChoice != "" {
		if req.DefaultChoice != string(merge.Left) && req.DefaultChoice != string(merge.Right) {
			writeError(w, http.StatusBadRequest, "defaultChoice must be left or right")
			return
		}
		for index := range result.Hunks {
			if _, set := choices[index]; !set {
				choices[index] = merge.Side(req.DefaultChoice)
			}
		}
	}
	merged, err := merge.WriteText(oldLines, newLines, result, merge.TextOptions{
		Output: req.Output, OldPath: req.Old, NewPath: req.New, Choices: choices,
		AllowUnresolved: req.AllowUnresolved, Overwrite: req.Overwrite, ConfirmOverwrite: req.ConfirmOverwrite,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, merged)
}

type threeWayTextRequest struct {
	diffRequest
	Base             string            `json:"base"`
	Output           string            `json:"output,omitempty"`
	Choices          map[string]string `json:"choices,omitempty"`
	AllowUnresolved  bool              `json:"allowUnresolved,omitempty"`
	Overwrite        bool              `json:"overwrite,omitempty"`
	ConfirmOverwrite bool              `json:"confirmOverwrite,omitempty"`
}

func openThreeWayText(req threeWayTextRequest) (linediff.Lines, linediff.Lines, linediff.Lines, func(), error) {
	base, closeBase, err := openMode(req.Base, "text", req.Encoding, false, false)
	if err != nil {
		return nil, nil, nil, func() {}, fmt.Errorf("base: %w", err)
	}
	left, closeLeft, err := openMode(req.Old, "text", req.Encoding, false, false)
	if err != nil {
		closeBase()
		return nil, nil, nil, func() {}, fmt.Errorf("left: %w", err)
	}
	right, closeRight, err := openMode(req.New, "text", req.Encoding, false, false)
	if err != nil {
		closeLeft()
		closeBase()
		return nil, nil, nil, func() {}, fmt.Errorf("right: %w", err)
	}
	return base, left, right, func() { closeRight(); closeLeft(); closeBase() }, nil
}

func threeWayTextResult(req threeWayTextRequest) (linediff.Lines, threeway.Result, func(), error) {
	if req.Base == "" || req.Old == "" || req.New == "" {
		return nil, threeway.Result{}, func() {}, fmt.Errorf("base, old/left, and new/right paths are required")
	}
	base, left, right, closeLines, err := openThreeWayText(req)
	if err != nil {
		return nil, threeway.Result{}, func() {}, err
	}
	window := req.Window
	if window == 0 {
		window = 128
	}
	options, err := requestDiffOptions(req.diffRequest, math.MaxInt, window)
	if err != nil {
		closeLines()
		return nil, threeway.Result{}, func() {}, err
	}
	return base, threeway.Compare(base, left, right, options), closeLines, nil
}

func (s *Server) handleThreeWayText(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "use POST")
		return
	}
	var req threeWayTextRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	_, result, closeLines, err := threeWayTextResult(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	defer closeLines()
	maxLines := req.MaxLines
	if maxLines == 0 {
		maxLines = 200
	}
	for i := range result.Events {
		if uint64(len(result.Events[i].Base)) > maxLines {
			result.Events[i].Base = result.Events[i].Base[:maxLines]
		}
		if uint64(len(result.Events[i].Left)) > maxLines {
			result.Events[i].Left = result.Events[i].Left[:maxLines]
		}
		if uint64(len(result.Events[i].Right)) > maxLines {
			result.Events[i].Right = result.Events[i].Right[:maxLines]
		}
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleThreeWayTextMerge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "use POST")
		return
	}
	var req threeWayTextRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if aliases := pathsEqual(req.Output, req.Base) || pathsEqual(req.Output, req.Old) || pathsEqual(req.Output, req.New); aliases && (!req.Overwrite || !req.ConfirmOverwrite) {
		writeError(w, http.StatusBadRequest, "overwriting an input requires overwrite and explicit confirmation")
		return
	}
	base, result, closeLines, err := threeWayTextResult(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	defer closeLines()
	choices := make(map[int]string, len(req.Choices))
	for idText, side := range req.Choices {
		id, parseErr := strconv.Atoi(idText)
		if parseErr != nil || id < 0 || (side != "left" && side != "right" && side != "base") {
			writeError(w, http.StatusBadRequest, "invalid conflict choice")
			return
		}
		choices[id] = side
	}
	lines, unresolved, err := threeway.MergeLines(base, result, choices, req.AllowUnresolved)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := threeway.WriteMerged(req.Output, lines); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"output": req.Output, "conflicts": result.Conflicts, "unresolved": unresolved})
}

type threeWayCSVRequest struct {
	csvRequest
	Base             string            `json:"base"`
	Choices          map[string]string `json:"choices,omitempty"`
	AllowUnresolved  bool              `json:"allowUnresolved,omitempty"`
	Overwrite        bool              `json:"overwrite,omitempty"`
	ConfirmOverwrite bool              `json:"confirmOverwrite,omitempty"`
}

func compareThreeWayCSV(r *http.Request, req threeWayCSVRequest) (threeway.CSVResult, error) {
	if req.Base == "" || req.Old == "" || req.New == "" {
		return threeway.CSVResult{}, fmt.Errorf("base, old/left, and new/right paths are required")
	}
	if req.KeyMode == "include" && len(req.KeyNames)+len(req.KeyIndexes) == 0 {
		return threeway.CSVResult{}, fmt.Errorf("select at least one key column")
	}
	cfg := csvConfig(req.csvRequest, filepath.Join(os.TempDir(), "three-way-unused.tsv"))
	return threeway.CompareCSV(r.Context(), req.Base, req.Old, req.New, cfg)
}

func (s *Server) handleThreeWayCSV(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "use POST")
		return
	}
	var req threeWayCSVRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	result, err := compareThreeWayCSV(r, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleThreeWayCSVMerge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "use POST")
		return
	}
	var req threeWayCSVRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if strings.TrimSpace(req.Output) == "" {
		writeError(w, http.StatusBadRequest, "output path is required")
		return
	}
	if aliases := pathsEqual(req.Output, req.Base) || pathsEqual(req.Output, req.Old) || pathsEqual(req.Output, req.New); aliases && (!req.Overwrite || !req.ConfirmOverwrite) {
		writeError(w, http.StatusBadRequest, "overwriting an input requires overwrite and explicit confirmation")
		return
	}
	result, err := compareThreeWayCSV(r, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	unresolved, err := threeway.WriteCSVMerge(req.Base, req.Output, result, req.Choices, req.AllowUnresolved)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"output": req.Output, "conflicts": result.Conflicts, "unresolved": unresolved})
}

func pathModTime(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

func openRequestLines(req diffRequest) (linediff.Lines, linediff.Lines, func(), error) {
	if req.Inline {
		return inlineLines(req.OldText, req.Mode, req.Numeric, req.Reverse),
			inlineLines(req.NewText, req.Mode, req.Numeric, req.Reverse), func() {}, nil
	}
	oldLines, closeOld, err := openMode(req.Old, req.Mode, req.Encoding, req.Numeric, req.Reverse)
	if err != nil {
		return nil, nil, func() {}, fmt.Errorf("old: %w", err)
	}
	newLines, closeNew, err := openMode(req.New, req.Mode, req.Encoding, req.Numeric, req.Reverse)
	if err != nil {
		closeOld()
		return nil, nil, func() {}, fmt.Errorf("new: %w", err)
	}
	return oldLines, newLines, func() { closeNew(); closeOld() }, nil
}

// whitespaceMode maps the request's whitespace option to the linediff enum.
func whitespaceMode(s string) linediff.Whitespace {
	switch s {
	case "change":
		return linediff.WSChange
	case "all":
		return linediff.WSAll
	default:
		return linediff.WSKeep
	}
}

func requestDiffOptions(req diffRequest, maxHunks int, window uint64) (linediff.Options, error) {
	if req.Whitespace != "" && req.Whitespace != "none" && req.Whitespace != "change" && req.Whitespace != "all" {
		return linediff.Options{}, fmt.Errorf("whitespace must be none, change, or all")
	}
	filters, err := linediff.CompileLineFilters(req.LineFilters)
	if err != nil {
		return linediff.Options{}, err
	}
	return linediff.Options{
		MaxHunks: maxHunks, Window: window, IgnoreCase: req.IgnoreCase,
		Whitespace: whitespaceMode(req.Whitespace), IgnoreEOL: req.IgnoreEOL,
		IgnoreTrailingEOL: req.IgnoreTrailingEOL, LineFilters: filters,
		SyncPoints: req.SyncPoints,
	}, nil
}

// inlineLines builds a linediff.Lines from in-memory text (scratch comparison),
// sorting it when the sorted mode is selected.
func inlineLines(text, mode string, numeric, reverse bool) linediff.Lines {
	lines := linediff.SplitTextLines(text)
	if mode == "sorted" {
		values := make([]string, 0, lines.Count())
		for i := uint64(0); i < lines.Count(); i++ {
			line, _ := lines.Line(i)
			values = append(values, line)
		}
		return linesort.SortLines(values, numeric, reverse)
	}
	return lines
}

// openMode builds a linediff.Lines for path in the given mode, decoded from
// encHint ("auto" to detect). It returns a close func (a no-op for sorted,
// whose lines are already in memory).
func openMode(path, mode, encHint string, numeric, reverse bool) (linediff.Lines, func(), error) {
	if mode == "sorted" {
		lines, err := linesort.Sorted(path, numeric, reverse, encHint)
		if err != nil {
			return nil, func() {}, err
		}
		return lines, func() {}, nil
	}
	f, err := linesrc.OpenEncoding(path, encHint)
	if err != nil {
		return nil, func() {}, err
	}
	return f, func() { f.Close() }, nil
}

func buildResponse(old, new linediff.Lines, res linediff.Result, maxLines uint64) diffResponse {
	hunks := make([]hunkOut, len(res.Hunks))
	for i, h := range res.Hunks {
		hunks[i] = hunkOut{
			Kind:     h.Kind.String(),
			OldStart: h.OldStart,
			OldLen:   h.OldLen,
			NewStart: h.NewStart,
			NewLen:   h.NewLen,
			Old:      sliceLines(old, h.OldStart, h.OldLen, maxLines),
			New:      sliceLines(new, h.NewStart, h.NewLen, maxLines),
			MoveID:   h.MoveID,
		}
		if h.MoveID != 0 {
			peer := h.MovePeer
			hunks[i].MovePeer = &peer
		}
	}
	return diffResponse{
		OldLines:     res.OldLines,
		NewLines:     res.NewLines,
		Hunks:        hunks,
		HunkCount:    res.HunkCount,
		OmittedHunks: res.OmittedHunks,
		Added:        res.Added,
		Deleted:      res.Deleted,
		Modified:     res.Modified,
		MovedBlocks:  res.MovedBlocks,
		MovedLines:   res.MovedLines,
		IgnoredHunks: res.IgnoredHunks,
	}
}

// sliceLines returns up to maxLines line strings starting at start.
func sliceLines(lines linediff.Lines, start, count, maxLines uint64) []string {
	if count > maxLines {
		count = maxLines
	}
	out := make([]string, 0, count)
	for i := start; i < start+count; i++ {
		s, _ := lines.Line(i)
		out = append(out, s)
	}
	return out
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
