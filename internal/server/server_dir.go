package server

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ayame-editor/ayame-diff/internal/dircompare"
	"github.com/ayame-editor/ayame-diff/internal/engine"
)

func (s *Server) handlePathInfo(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
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
	req, ok := decodePostJSON[dirRequest](w, r, "left and right directory paths are required")
	if !ok {
		return
	}
	if req.Old == "" || req.New == "" {
		writeError(w, http.StatusBadRequest, "left and right directory paths are required")
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
	req, ok := decodePostJSON[dirRequest](w, r, "left and right directory paths are required")
	if !ok {
		return
	}
	if req.Old == "" || req.New == "" {
		writeError(w, http.StatusBadRequest, "left and right directory paths are required")
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
