// Package server hosts a small local web UI for ayame-diff: it serves an
// embedded single-page frontend and a JSON diff API over localhost. It is the
// GUI foundation (hjosugi/ayame-diff#10); the diff view lives in the embedded
// web assets (#11). Dependency-zero: net/http, embed, and the diff packages.
package server

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hjosugi/ayame-diff/internal/atomicfile"
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

// Server-side resource caps bound what an (unauthenticated, #108) client can
// demand, so a single request cannot exhaust the host (#170).
const (
	// serverMaxMemoryText caps the resident-memory budget a CSV request may ask
	// for; larger values are clamped down — the engine just spills more, so the
	// result is unchanged.
	serverMaxMemoryText = "8GiB"
	// serverMaxArchiveEntryBytes / serverMaxArchiveBytes cap the uncompressed
	// archive expansion a dir request may request, so a client cannot raise the
	// zip-bomb guard (#70) back toward unbounded.
	serverMaxArchiveEntryBytes int64 = 2 << 30 // 2 GiB
	serverMaxArchiveBytes      int64 = 8 << 30 // 8 GiB
)

// Browser drops are staged on disk before the path-based comparison engines
// open them. Bound both one request and the aggregate for one browser session
// so a bad or accidental drop cannot grow one cache tree without limit (#109).
// Vars let regression tests exercise the boundaries with tiny payloads.
var (
	maxDropUploadBytes  int64 = 2 << 30 // 2 GiB per file
	maxDropSessionBytes int64 = 8 << 30 // 8 GiB per browser session
)

// maxConcurrentComparisons bounds how many expensive comparisons run at once, so
// N parallel requests cannot multiply memory / temp-dir / goroutine use without
// limit (#170). A var so tests can shrink it before New.
var maxConcurrentComparisons = max(2, runtime.NumCPU())

// dropSessionTTL is how long an unowned (previous-run) drop directory may sit
// before opportunistic cleanup removes it. Live sessions are excluded from
// cleanup regardless of age (#168). A var so tests can shrink it.
var dropSessionTTL = 24 * time.Hour

type dropSession struct {
	root string

	// Uploads within one browser session are serialized so aggregate accounting
	// remains exact. Different sessions can still stream concurrently.
	mu   sync.Mutex
	used int64
}

// Server serves the UI and diff API.
type Server struct {
	version      string
	token        string
	allowedHosts map[string]bool
	mux          *http.ServeMux
	dropMu       sync.Mutex
	drops        map[string]*dropSession
	compareSem   chan struct{} // bounds concurrent expensive comparisons (#170)
	watchSem     chan struct{} // bounds authenticated long-poll file watches (#251)
}

// Options configures a Server.
type Options struct {
	// Version is reported by /api/health.
	Version string
	// AllowedHosts is the exact set of Host header values the server answers
	// to, normally the loopback names for the bound port. It is the defense
	// against DNS rebinding: a page on any website can make the browser resolve
	// its own hostname to 127.0.0.1 and reach this server, but the request
	// still carries that site's name in Host, not ours (#108).
	//
	// Empty accepts any Host. That is only appropriate for a deliberately
	// remote listener, whose reachable names the process cannot enumerate;
	// there the token is the defense.
	AllowedHosts []string
}

// New returns a Server that accepts any Host. Prefer NewWithOptions, which can
// pin the Host header; this form suits tests and callers whose listener a
// browser cannot reach.
func New(version string) (*Server, error) {
	return NewWithOptions(Options{Version: version})
}

// NewWithOptions returns a Server with a freshly generated API token. Every
// /api route except the health probe requires that token in an X-Ayame-Token
// header, so a website the user happens to be visiting cannot drive the local
// GUI's read and write endpoints (#108).
func NewWithOptions(opts Options) (*Server, error) {
	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		return nil, err
	}
	token, err := newAPIToken()
	if err != nil {
		return nil, err
	}
	allowed := make(map[string]bool, len(opts.AllowedHosts))
	for _, host := range opts.AllowedHosts {
		allowed[strings.ToLower(host)] = true
	}
	s := &Server{
		version: opts.Version, token: token, allowedHosts: allowed,
		mux: http.NewServeMux(), drops: make(map[string]*dropSession),
		compareSem: make(chan struct{}, maxConcurrentComparisons),
		watchSem:   make(chan struct{}, maxConcurrentWatchRequests),
	}
	s.mux.Handle("/", http.FileServer(http.FS(sub)))
	s.mux.HandleFunc("/api/health", s.handleHealth)
	// Compute-heavy comparison / merge / export handlers run under the
	// concurrency gate (#170); cheap metadata handlers (health, files,
	// path-info, drop, project, csv/inspect) do not.
	s.mux.HandleFunc("/api/diff", s.limited(s.handleDiff))
	s.mux.HandleFunc("/api/patch", s.limited(s.handlePatch))
	s.mux.HandleFunc("/api/merge/text", s.limited(s.handleTextMerge))
	s.mux.HandleFunc("/api/three-way/text", s.limited(s.handleThreeWayText))
	s.mux.HandleFunc("/api/merge/three-way/text", s.limited(s.handleThreeWayTextMerge))
	s.mux.HandleFunc("/api/csv/inspect", s.handleCSVInspect)
	s.mux.HandleFunc("/api/csv/diff", s.limited(s.handleCSVDiff))
	s.mux.HandleFunc("/api/csv/export", s.limited(s.handleCSVExport))
	s.mux.HandleFunc("/api/merge/csv", s.limited(s.handleCSVMerge))
	s.mux.HandleFunc("/api/three-way/csv", s.limited(s.handleThreeWayCSV))
	s.mux.HandleFunc("/api/merge/three-way/csv", s.limited(s.handleThreeWayCSVMerge))
	s.mux.HandleFunc("/api/dir/diff", s.limited(s.handleDirDiff))
	s.mux.HandleFunc("/api/files", s.handleFiles)
	s.mux.HandleFunc("/api/path-info", s.handlePathInfo)
	s.mux.HandleFunc("/api/watch", s.handleWatch)
	s.mux.HandleFunc("/api/drop", s.handleDrop)
	s.mux.HandleFunc("/api/project/save", s.handleProjectSave)
	s.mux.HandleFunc("/api/project/load", s.handleProjectLoad)
	s.mux.HandleFunc("/api/dir/preview", s.limited(s.handleDirPreview))
	return s, nil
}

// limited caps concurrency on expensive comparison handlers, returning 429 when
// maxConcurrentComparisons are already running so parallel requests cannot
// multiply resource use without bound (#170).
func (s *Server) limited(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w = longOperationResponseWriter(w)
		select {
		case s.compareSem <- struct{}{}:
			defer func() { <-s.compareSem }()
			h(w, r)
		default:
			w.Header().Set("Retry-After", "1")
			writeError(w, http.StatusTooManyRequests, "too many concurrent comparisons; please retry shortly")
		}
	}
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
	extendDropDeadlines(w)
	drop, err := s.dropState(session)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	drop.mu.Lock()
	defer drop.mu.Unlock()

	target := filepath.Join(drop.root, relative)
	if r.URL.Query().Get("directory") == "1" {
		err = os.MkdirAll(target, 0o700)
	} else {
		oldSize, sizeErr := regularFileSize(target)
		if sizeErr != nil {
			writeError(w, http.StatusInternalServerError, sizeErr.Error())
			return
		}
		baseUsed := drop.used - oldSize
		sessionAvailable := maxDropSessionBytes - baseUsed
		if sessionAvailable <= 0 {
			writeDropLimitError(w, maxDropSessionBytes, true)
			return
		}
		allowed := min(maxDropUploadBytes, sessionAvailable)
		if r.ContentLength > allowed {
			writeDropLimitError(w, limitForDropError(sessionAvailable), sessionAvailable < maxDropUploadBytes)
			return
		}
		if err = os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, allowed)
		var copied int64
		err = atomicfile.Write(target, atomicfile.Options{
			Pattern: ".ayame-drop-*.tmp",
			Mode:    0o600,
		}, func(destination io.Writer) error {
			var copyErr error
			copied, copyErr = io.Copy(destination, r.Body)
			return copyErr
		})
		if err == nil {
			drop.used = baseUsed + copied
		} else {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				writeDropLimitError(w, limitForDropError(sessionAvailable), sessionAvailable < maxDropUploadBytes)
				return
			}
		}
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Path string `json:"path"`
	}{target})
}

func regularFileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if !info.Mode().IsRegular() {
		return 0, fmt.Errorf("%s is not a regular file", path)
	}
	return info.Size(), nil
}

func limitForDropError(sessionAvailable int64) int64 {
	return min(maxDropUploadBytes, sessionAvailable)
}

func writeDropLimitError(w http.ResponseWriter, limit int64, sessionLimit bool) {
	if sessionLimit {
		writeError(w, http.StatusRequestEntityTooLarge,
			fmt.Sprintf("drop session exceeds its total limit (%d bytes)", maxDropSessionBytes))
		return
	}
	writeError(w, http.StatusRequestEntityTooLarge,
		fmt.Sprintf("upload exceeds the per-file limit (%d bytes)", limit))
}

func (s *Server) dropRoot(session string) (string, error) {
	drop, err := s.dropState(session)
	if err != nil {
		return "", err
	}
	return drop.root, nil
}

func (s *Server) dropState(session string) (*dropSession, error) {
	s.dropMu.Lock()
	drop := s.drops[session]
	s.dropMu.Unlock()
	if drop != nil {
		return drop, nil
	}

	base, err := os.UserCacheDir()
	if err != nil {
		base = os.TempDir()
	}
	base = filepath.Join(base, "ayame-diff", "drops")
	if err := os.MkdirAll(base, 0o700); err != nil {
		return nil, err
	}

	s.dropMu.Lock()
	// A concurrent call for the same session may have registered a root while we
	// prepared the base directory; keep the first one.
	if existing := s.drops[session]; existing != nil {
		s.dropMu.Unlock()
		return existing, nil
	}
	root, err := os.MkdirTemp(base, "session-")
	if err != nil {
		s.dropMu.Unlock()
		return nil, err
	}
	drop = &dropSession{root: root}
	s.drops[session] = drop
	s.dropMu.Unlock()

	// Reclaim orphaned directories from previous runs, but never a live one, and
	// never while holding the lock (#168).
	s.cleanupStaleDrops(base)
	return drop, nil
}

// cleanupStaleDrops removes drop directories older than dropSessionTTL that no
// live session owns. The live-root set is snapshotted under the lock; the
// directory scan and each RemoveAll run without it, so a first drop never blocks
// other sessions behind filesystem work and an active session's directory (which
// handleDrop may be writing to) is never deleted (#168).
func (s *Server) cleanupStaleDrops(base string) {
	s.dropMu.Lock()
	live := make(map[string]struct{}, len(s.drops))
	for _, drop := range s.drops {
		live[drop.root] = struct{}{}
	}
	s.dropMu.Unlock()

	entries, err := os.ReadDir(base)
	if err != nil {
		return
	}
	for _, entry := range entries {
		full := filepath.Join(base, entry.Name())
		if _, active := live[full]; active {
			continue
		}
		if info, infoErr := entry.Info(); infoErr == nil && time.Since(info.ModTime()) > dropSessionTTL {
			_ = os.RemoveAll(full)
		}
	}
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
		writeClassifiedError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Path      string `json:"path"`
		Directory bool   `json:"directory"`
	}{absolute, info.IsDir()})
}

type dirRequest struct {
	Mode                 string   `json:"mode,omitempty"`
	ProjectPath          string   `json:"projectPath,omitempty"`
	Old                  string   `json:"old"`
	New                  string   `json:"new"`
	Includes             []string `json:"includes"`
	Excludes             []string `json:"excludes"`
	Hidden               bool     `json:"hidden"`
	Quick                bool     `json:"quick"`
	CompareBy            string   `json:"compareBy"`
	Filter               string   `json:"filter"`
	FilterFile           string   `json:"filterFile"`
	FilterSets           []string `json:"filterSets"`
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
	opts, err := directoryOptions(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := dircompare.CompareAnyContext(r.Context(), req.Old, req.New, opts)
	if err != nil {
		writeClassifiedError(w, err, http.StatusBadRequest)
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

func (s *Server) handleDirPreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "use POST")
		return
	}
	var req dirRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Old == "" || req.New == "" {
		writeError(w, http.StatusBadRequest, "old and new directory paths are required")
		return
	}
	opts, err := directoryOptions(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	preview, err := dircompare.PreviewAny(req.Old, req.New, opts, 100)
	if err != nil {
		writeClassifiedError(w, err, http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, preview)
}

func directoryOptions(req dirRequest) (dircompare.Options, error) {
	entryLimit, totalLimit, err := parseArchiveLimits(req.MaxArchiveEntryBytes, req.MaxArchiveBytes)
	if err != nil {
		return dircompare.Options{}, err
	}
	set, embedded, err := dircompare.ResolveFilterSets(req.FilterFile, req.FilterSets)
	if err != nil {
		return dircompare.Options{}, err
	}
	includes := append(append([]string(nil), req.Includes...), set.Includes...)
	excludes := append(append([]string(nil), req.Excludes...), set.Excludes...)
	expression := strings.TrimSpace(req.Filter)
	if set.Expression != "" {
		if expression == "" {
			expression = set.Expression
		} else {
			expression = "(" + expression + ") and (" + set.Expression + ")"
		}
	}
	filter, err := dircompare.ParseFilter(expression)
	if err != nil {
		return dircompare.Options{}, err
	}
	compareBy := req.CompareBy
	workers, hidden := req.Workers, req.Hidden
	if embedded != nil {
		if compareBy == "" {
			compareBy = embedded.CompareBy
		}
		if workers <= 0 {
			workers = embedded.Workers
		}
		hidden = hidden || embedded.Hidden
	}
	if req.Quick {
		if compareBy != "" && compareBy != "quick" {
			return dircompare.Options{}, fmt.Errorf("quick cannot be combined with compareBy %q", compareBy)
		}
		compareBy = "quick"
	}
	method, err := dircompare.ParseCompareMethod(compareBy)
	if err != nil {
		return dircompare.Options{}, err
	}
	return dircompare.Options{
		Includes: includes, Excludes: excludes, IncludeHidden: hidden, Filter: filter,
		CompareBy: method, Workers: workers, MaxArchiveEntryBytes: entryLimit, MaxArchiveBytes: totalLimit,
	}, nil
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
	// Clamp to the server's absolute maxima so a client cannot raise the
	// zip-bomb expansion guard (#70) back toward unbounded (#170). Clamping
	// (rather than rejecting) keeps legitimate requests working, just bounded.
	if entryLimit > serverMaxArchiveEntryBytes {
		entryLimit = serverMaxArchiveEntryBytes
	}
	if totalLimit > serverMaxArchiveBytes {
		totalLimit = serverMaxArchiveBytes
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

func pathsEqual(a, b string) bool {
	aa, errA := filepath.Abs(a)
	bb, errB := filepath.Abs(b)
	return errA == nil && errB == nil && filepath.Clean(aa) == filepath.Clean(bb)
}

func (s *Server) handleProjectSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "use POST")
		return
	}
	data, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var envelope struct {
		Mode string `json:"mode"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if envelope.Mode == "dir" {
		var req dirRequest
		if err := json.Unmarshal(data, &req); err != nil || req.ProjectPath == "" || req.Old == "" || req.New == "" {
			writeError(w, http.StatusBadRequest, "directory project, old, and new paths are required")
			return
		}
		opts, err := directoryOptions(req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		directory := &dircompare.DirectoryProject{
			Old: req.Old, New: req.New, Includes: opts.Includes, Excludes: opts.Excludes,
			CompareBy: string(opts.CompareBy), Hidden: opts.IncludeHidden, Workers: opts.Workers,
		}
		if opts.Filter != nil {
			directory.Filter = opts.Filter.Expression()
		}
		if err := project.Save(req.ProjectPath, project.Project{Mode: "dir", Directory: directory}); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"path": req.ProjectPath})
		return
	}
	var req csvRequest
	if err := json.Unmarshal(data, &req); err != nil || req.Old == "" || req.New == "" {
		writeError(w, http.StatusBadRequest, "both 'old' and 'new' paths are required")
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
		writeClassifiedError(w, err, http.StatusBadRequest)
		return
	}
	if loaded.Mode == "dir" {
		directory := loaded.Directory
		writeJSON(w, http.StatusOK, struct {
			Mode        string   `json:"mode"`
			ProjectPath string   `json:"projectPath"`
			Old         string   `json:"old"`
			New         string   `json:"new"`
			Includes    []string `json:"includes,omitempty"`
			Excludes    []string `json:"excludes,omitempty"`
			Filter      string   `json:"filter,omitempty"`
			FilterSets  []string `json:"filterSets,omitempty"`
			CompareBy   string   `json:"compareBy,omitempty"`
			Hidden      bool     `json:"hidden,omitempty"`
			Workers     int      `json:"workers,omitempty"`
		}{"dir", body.Path, directory.Old, directory.New, directory.Includes, directory.Excludes, directory.Filter, directory.FilterSets, directory.CompareBy, directory.Hidden, directory.Workers})
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
		writeClassifiedError(w, err, http.StatusBadRequest)
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
// contentSecurityPolicy locks the embedded single-page UI to its own origin:
// scripts and styles load only from us (runtime layout styles set via the
// CSSOM still work; inline <style>/style="" are covered by 'unsafe-inline'),
// images allow local data: URIs, and framing, objects, workers, foreign
// connections, and form posts are denied. Mirrors the sibling ayame-editor
// policy so the family stays consistent. (#146)
const contentSecurityPolicy = "default-src 'self'; base-uri 'none'; connect-src 'self'; font-src 'self'; form-action 'none'; frame-ancestors 'none'; img-src 'self' data:; object-src 'none'; script-src 'self'; style-src 'self' 'unsafe-inline'; worker-src 'none'"

// Handler returns the HTTP handler wrapped in the security middleware, so every
// route (the embedded UI and the JSON API) gets hardening headers and CSRF
// protection whether it is served by `serve` or `gui`. The recovery wrapper is
// outermost so a panic raised inside the security checks is caught too.
func (s *Server) Handler() http.Handler {
	return recovered(s.allowedHost(secure(s.authenticated(s.mux))))
}

// Token returns the API token this server requires. The command that starts the
// server puts it in the URL it opens or prints, which is how the browser comes
// to hold it.
func (s *Server) Token() string { return s.token }

// tokenHeader carries the API token. A header is deliberate: a page on another
// origin cannot set one without a CORS preflight, and this server answers no
// preflight, so the token requirement doubles as CSRF protection. A cookie
// would be attached automatically and would not.
const tokenHeader = "X-Ayame-Token"

// newAPIToken returns a fresh 256-bit token for one server run.
func newAPIToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate API token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// openPath reports whether a path is reachable without the token: the embedded
// UI itself, which holds no user data and cannot send headers for its own
// sub-resources, and the health probe, which reports only the version and is
// what the launching command polls for readiness.
func openPath(path string) bool {
	return path == "/api/health" || !strings.HasPrefix(path, "/api/")
}

// authenticated rejects API requests without the token (#108). Before this,
// any website the user visited could POST to the local server and have it read
// or overwrite arbitrary files, and could GET /api/files to enumerate the disk.
func (s *Server) authenticated(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if openPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		// Constant time so a wrong token cannot be discovered byte by byte.
		if subtle.ConstantTimeCompare([]byte(r.Header.Get(tokenHeader)), []byte(s.token)) != 1 {
			writeError(w, http.StatusUnauthorized, "missing or invalid API token; open the URL printed by ayame-diff, which carries it")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// allowedHost rejects a request whose Host header is not one this server was
// told to answer to. This is what stops DNS rebinding: the attacking page keeps
// its own hostname in Host even once that name resolves to 127.0.0.1 (#108).
func (s *Server) allowedHost(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(s.allowedHosts) > 0 && !s.allowedHosts[strings.ToLower(r.Host)] {
			writeError(w, http.StatusForbidden, "unexpected Host header; reach this server at the address it printed")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// recovered turns a panic in a handler into a 500 with the same JSON error
// shape as every other failure, instead of net/http silently closing the
// connection and leaving the GUI waiting on a request that never answers
// (#137). The process keeps serving; the stack goes to the server log so the
// bug stays reportable.
//
// The panic value is deliberately not sent to the client: it can carry
// filesystem paths or input fragments, and the user can do nothing with it. The
// log line is the diagnosable copy.
func recovered(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			value := recover()
			if value == nil {
				return
			}
			// A panic after the response started cannot be rewritten as a 500;
			// aborting tells net/http not to pretend the reply was complete.
			if value == http.ErrAbortHandler {
				panic(value)
			}
			log.Printf("panic serving %s %s: %v\n%s", r.Method, r.URL.Path, value, debug.Stack())
			writeError(w, http.StatusInternalServerError, "internal error: the comparison could not be completed")
		}()
		next.ServeHTTP(w, r)
	})
}

// maxJSONBodyBytes caps every JSON request body so a huge or slow POST cannot
// exhaust memory (#147). It is deliberately generous — real diff/merge requests
// carry paths, options, and merge choices (kilobytes), and even pasted "scratch"
// text is well under this — while still bounding worst-case allocation. File
// contents travel via /api/drop, which streams to disk and is exempt below. A
// var (not const) so tests can shrink it without allocating the full ceiling.
var maxJSONBodyBytes int64 = 64 << 20 // 64 MiB

// secure adds response-hardening headers to every reply (#146) and rejects
// cross-origin state-changing requests (#145). Without an Origin check a page
// on any other website can, while the user has this local server running, POST
// to e.g. /api/csv/export or /api/merge/* and write attacker-chosen files: the
// request targets 127.0.0.1 with a valid Host, `text/plain` dodges the CORS
// preflight, and the response being unreadable cross-origin does not stop the
// write side effect. Only an Origin check closes that hole.
func secure(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", contentSecurityPolicy)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		if !safeMethod(r.Method) && !sameOriginRequest(r) {
			writeError(w, http.StatusForbidden, "cross-origin request rejected")
			return
		}
		// Bound JSON request bodies so a crafted or slow POST can't exhaust
		// memory (#147). /api/drop streams uploads to disk in bounded memory and
		// carries file contents, so it keeps its own handling and is exempt.
		if !safeMethod(r.Method) && r.URL.Path != "/api/drop" {
			r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
		}
		next.ServeHTTP(w, r)
	})
}

// safeMethod reports whether the method only reads state. Cross-origin reads
// are already blocked from being *read back* by the same-origin policy, so the
// CSRF gate only needs to guard the state-changing verbs.
func safeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}

// sameOriginRequest is the CSRF gate. A browser always attaches an Origin on a
// cross-site request, so an absent Origin marks a non-browser client (curl, the
// native GUI shell) that a malicious page cannot drive — allow it. When present,
// the Origin's host:port must equal the request's own Host (true same-origin,
// which also holds when the user bound `--addr` to a LAN address), with
// loopback origins accepted as a fallback for proxy/port quirks on a local tool.
func sameOriginRequest(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return false
	}
	if strings.EqualFold(u.Host, r.Host) {
		return true
	}
	host := u.Hostname()
	if strings.EqualFold(host, "localhost") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return true
	}
	return false
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "use GET")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": s.version})
}

// diffRequest is the POST body for /api/diff.
type diffRequest struct {
	Old               string               `json:"old"`
	New               string               `json:"new"`
	Mode              string               `json:"mode"` // "text" (default) or "sorted"
	Encoding          string               `json:"encoding"`
	Window            uint64               `json:"window,omitempty"`
	MaxHunks          int                  `json:"maxHunks,omitempty"`
	MaxLines          uint64               `json:"maxLines,omitempty"`
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
	// A directory result can open an added or removed file even though the
	// nominal path on one side does not exist (#104). The path remains present
	// as its display/patch label; the explicit flag supplies an empty source.
	OldAbsent bool `json:"oldAbsent,omitempty"`
	NewAbsent bool `json:"newAbsent,omitempty"`
}

var positiveDiffNumberFields = []struct {
	name string
	max  uint64
}{
	{name: "window", max: ^uint64(0)},
	{name: "maxHunks", max: uint64(math.MaxInt)},
	{name: "maxLines", max: ^uint64(0)},
	{name: "moveMinLines", max: ^uint64(0)},
}

// decodeDiffJSON validates the positive integer controls before decoding any
// request that contains diffRequest. This keeps encoding/json's Go struct and
// type names out of user-facing validation errors.
func decodeDiffJSON(body io.Reader, destination any) error {
	data, err := io.ReadAll(body)
	if err != nil {
		return fmt.Errorf("read request body: %w", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return fmt.Errorf("malformed JSON")
	}
	for _, field := range positiveDiffNumberFields {
		raw, present := fields[field.name]
		if !present {
			continue
		}
		text := string(raw)
		if text == "" {
			return fmt.Errorf("%s must be an integer greater than or equal to 1", field.name)
		}
		for _, character := range text {
			if character < '0' || character > '9' {
				return fmt.Errorf("%s must be an integer greater than or equal to 1", field.name)
			}
		}
		value, err := strconv.ParseUint(text, 10, 64)
		if err != nil || value == 0 || value > field.max {
			return fmt.Errorf("%s must be an integer within the supported range (minimum 1)", field.name)
		}
	}
	if err := json.Unmarshal(data, destination); err != nil {
		var typeError *json.UnmarshalTypeError
		if errors.As(err, &typeError) {
			name := typeError.Field
			if dot := strings.LastIndexByte(name, '.'); dot >= 0 {
				name = name[dot+1:]
			}
			if name != "" {
				return fmt.Errorf("%s has an invalid value", name)
			}
		}
		return fmt.Errorf("malformed JSON")
	}
	return nil
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
	OldLines             uint64    `json:"old_lines"`
	NewLines             uint64    `json:"new_lines"`
	Hunks                []hunkOut `json:"hunks"`
	HunkCount            uint64    `json:"hunk_count"`
	OmittedHunks         uint64    `json:"omitted_hunks"`
	Added                uint64    `json:"added"`
	Deleted              uint64    `json:"deleted"`
	Modified             uint64    `json:"modified"`
	MovedBlocks          uint64    `json:"moved_blocks,omitempty"`
	MovedLines           uint64    `json:"moved_lines,omitempty"`
	MoveDetectionSkipped bool      `json:"move_detection_skipped,omitempty"`
	IgnoredHunks         uint64    `json:"ignored_hunks,omitempty"`
	// OldEncoding/NewEncoding report the concrete encoding each side was decoded
	// from (#130). Populated for file inputs — where `encoding: auto` may have
	// guessed shift_jis/euc-jp/utf-16 — so the UI can show what was detected and
	// flag a left/right mismatch. Empty for inline (scratch) text, which is
	// already UTF-8.
	OldEncoding string `json:"old_encoding,omitempty"`
	NewEncoding string `json:"new_encoding,omitempty"`
}

// encodingReporter is implemented by line sources that decoded from a detected
// or forced encoding (linesrc.FileLines). Inline and sorted sources do not.
type encodingReporter interface{ Encoding() string }

func (s *Server) handleDiff(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "use POST")
		return
	}
	var req diffRequest
	if err := decodeDiffJSON(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if err := validateDiffSources(req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
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
		writeClassifiedError(w, err, http.StatusBadRequest)
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
	res, err := linediff.DiffWithContext(r.Context(), oldLines, newLines, options)
	if err != nil {
		writeClassifiedError(w, err, http.StatusBadRequest)
		return
	}
	if req.DetectMoves {
		if _, err := linediff.DetectMovesContext(r.Context(), oldLines, newLines, &res, linediff.MoveOptions{
			MinLines: req.MoveMinLines, MaxCandidates: 10_000,
		}); err != nil {
			writeClassifiedError(w, err, http.StatusBadRequest)
			return
		}
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
	if err := decodeDiffJSON(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if err := validateDiffSources(req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
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
		writeClassifiedError(w, err, http.StatusBadRequest)
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
	res, err := linediff.DiffWithContext(r.Context(), oldLines, newLines, options)
	if err != nil {
		writeClassifiedError(w, err, http.StatusBadRequest)
		return
	}
	if req.DetectMoves {
		if _, err := linediff.DetectMovesContext(r.Context(), oldLines, newLines, &res, linediff.MoveOptions{
			MinLines: req.MoveMinLines, MaxCandidates: 10_000,
		}); err != nil {
			writeClassifiedError(w, err, http.StatusBadRequest)
			return
		}
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
	if err := decodeDiffJSON(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Mode == "sorted" {
		writeError(w, http.StatusBadRequest, "sorted comparisons cannot be merged back to the original order")
		return
	}
	if err := validateDiffSources(req.diffRequest); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	oldLines, newLines, closeLines, err := openRequestLines(req.diffRequest)
	if err != nil {
		writeClassifiedError(w, err, http.StatusBadRequest)
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
	result, err := linediff.DiffWithContext(r.Context(), oldLines, newLines, options)
	if err != nil {
		writeClassifiedError(w, err, http.StatusBadRequest)
		return
	}
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

func threeWayTextResult(ctx context.Context, req threeWayTextRequest) (linediff.Lines, threeway.Result, func(), error) {
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
	result, err := threeway.CompareContext(ctx, base, left, right, options)
	if err != nil {
		closeLines()
		return nil, threeway.Result{}, func() {}, err
	}
	return base, result, closeLines, nil
}

func (s *Server) handleThreeWayText(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "use POST")
		return
	}
	var req threeWayTextRequest
	if err := decodeDiffJSON(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	_, result, closeLines, err := threeWayTextResult(r.Context(), req)
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
	if err := decodeDiffJSON(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if aliases := pathsEqual(req.Output, req.Base) || pathsEqual(req.Output, req.Old) || pathsEqual(req.Output, req.New); aliases && (!req.Overwrite || !req.ConfirmOverwrite) {
		writeError(w, http.StatusBadRequest, "overwriting an input requires overwrite and explicit confirmation")
		return
	}
	base, result, closeLines, err := threeWayTextResult(r.Context(), req)
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
	// Capture the base file's encoding/BOM/EOL before MergeLines streams it, so
	// the written merge round-trips them instead of BOM-less UTF-8/LF (#159).
	profile := threeway.ProfileOf(base)
	lines, unresolved, err := threeway.MergeLines(base, result, choices, req.AllowUnresolved)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := threeway.WriteMerged(req.Output, lines, profile); err != nil {
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

func validateDiffSources(req diffRequest) error {
	if req.Inline {
		if req.OldAbsent || req.NewAbsent {
			return fmt.Errorf("absent sides are only valid for path comparisons")
		}
		return nil
	}
	if req.Old == "" || req.New == "" {
		return fmt.Errorf("both 'old' and 'new' paths are required")
	}
	if req.OldAbsent && req.NewAbsent {
		return fmt.Errorf("at least one comparison side must exist")
	}
	return nil
}

func openRequestLines(req diffRequest) (linediff.Lines, linediff.Lines, func(), error) {
	if req.Inline {
		return inlineLines(req.OldText, req.Mode, req.Numeric, req.Reverse),
			inlineLines(req.NewText, req.Mode, req.Numeric, req.Reverse), func() {}, nil
	}
	openSide := func(path string, absent bool) (linediff.Lines, func(), error) {
		if absent {
			return inlineLines("", req.Mode, req.Numeric, req.Reverse), func() {}, nil
		}
		return openMode(path, req.Mode, req.Encoding, req.Numeric, req.Reverse)
	}
	oldLines, closeOld, err := openSide(req.Old, req.OldAbsent)
	if err != nil {
		return nil, nil, func() {}, fmt.Errorf("old: %w", err)
	}
	newLines, closeNew, err := openSide(req.New, req.NewAbsent)
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
// encHint ("auto" to detect). It returns a close func the caller must run: a
// sorted comparison of a file larger than the sort budget spills to disk, and
// those files are released here (#137).
func openMode(path, mode, encHint string, numeric, reverse bool) (linediff.Lines, func(), error) {
	if mode == "sorted" {
		sorted, err := linesort.Sorted(path, numeric, reverse, encHint)
		if err != nil {
			return nil, func() {}, err
		}
		return sorted, func() { sorted.Close() }, nil
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
	resp := diffResponse{
		OldLines:             res.OldLines,
		NewLines:             res.NewLines,
		Hunks:                hunks,
		HunkCount:            res.HunkCount,
		OmittedHunks:         res.OmittedHunks,
		Added:                res.Added,
		Deleted:              res.Deleted,
		Modified:             res.Modified,
		MovedBlocks:          res.MovedBlocks,
		MovedLines:           res.MovedLines,
		MoveDetectionSkipped: res.MoveDetectionSkipped,
		IgnoredHunks:         res.IgnoredHunks,
	}
	if er, ok := old.(encodingReporter); ok {
		resp.OldEncoding = er.Encoding()
	}
	if er, ok := new.(encodingReporter); ok {
		resp.NewEncoding = er.Encoding()
	}
	return resp
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

func writeClassifiedError(w http.ResponseWriter, err error, fallback int) {
	writeError(w, statusForError(err, fallback), err.Error())
}

func statusForError(err error, fallback int) int {
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return http.StatusRequestTimeout
	case errors.Is(err, fs.ErrNotExist):
		return http.StatusNotFound
	case errors.Is(err, fs.ErrPermission):
		return http.StatusForbidden
	case errors.Is(err, engine.ErrUnresolvedRows):
		return http.StatusBadRequest
	default:
		return fallback
	}
}
