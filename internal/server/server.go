// Package server hosts a small local web UI for ayame-diff: it serves an
// embedded single-page frontend and a JSON diff API over localhost. It is the
// GUI foundation (hjosugi/ayame-diff#10); the diff view lives in the embedded
// web assets (#11). Dependency-zero: net/http, embed, and the diff packages.
package server

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"math"
	"net/http"
	"os"
	"time"

	"github.com/hjosugi/ayame-diff/internal/diffout"
	"github.com/hjosugi/ayame-diff/internal/linediff"
	"github.com/hjosugi/ayame-diff/internal/linesort"
	"github.com/hjosugi/ayame-diff/internal/linesrc"
)

//go:embed web
var webFS embed.FS

// Server serves the UI and diff API.
type Server struct {
	version string
	mux     *http.ServeMux
}

// New returns a Server. version is reported by /api/health.
func New(version string) (*Server, error) {
	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		return nil, err
	}
	s := &Server{version: version, mux: http.NewServeMux()}
	s.mux.Handle("/", http.FileServer(http.FS(sub)))
	s.mux.HandleFunc("/api/health", s.handleHealth)
	s.mux.HandleFunc("/api/diff", s.handleDiff)
	s.mux.HandleFunc("/api/patch", s.handlePatch)
	return s, nil
}

// Handler exposes the routes for http.ListenAndServe (and tests).
func (s *Server) Handler() http.Handler { return s.mux }

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": s.version})
}

// diffRequest is the POST body for /api/diff.
type diffRequest struct {
	Old         string `json:"old"`
	New         string `json:"new"`
	Mode        string `json:"mode"` // "text" (default) or "sorted"
	Encoding    string `json:"encoding"`
	Window      uint64 `json:"window"`
	MaxHunks    int    `json:"maxHunks"`
	MaxLines    uint64 `json:"maxLines"`
	Numeric     bool   `json:"numeric"`
	Reverse     bool   `json:"reverse"`
	IgnoreCase  bool   `json:"ignoreCase"`
	Whitespace  string `json:"whitespace"` // none | change | all
	PatchFormat string `json:"patchFormat,omitempty"`
	Context     *int   `json:"context,omitempty"`
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

	res := linediff.DiffWith(oldLines, newLines, linediff.Options{
		MaxHunks:   maxHunks,
		Window:     window,
		IgnoreCase: req.IgnoreCase,
		Whitespace: whitespaceMode(req.Whitespace),
	})
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
	if err := diffout.ValidatePatchable(oldLines, newLines); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	window := req.Window
	if window == 0 {
		window = 128
	}
	res := linediff.DiffWith(oldLines, newLines, linediff.Options{
		MaxHunks: math.MaxInt, Window: window, IgnoreCase: req.IgnoreCase,
		Whitespace: whitespaceMode(req.Whitespace),
	})
	oldLabel, newLabel := req.Old, req.New
	if req.Inline {
		oldLabel, newLabel = "old.txt", "new.txt"
	}
	w.Header().Set("Content-Type", "text/x-diff; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="ayame.patch"`)
	_ = diffout.Write(w, io.Discard, oldLines, newLines, res, diffout.Options{
		Format: format, Context: contextLines, ContextSet: true,
		OldLabel: oldLabel, NewLabel: newLabel,
		OldTime: pathModTime(req.Old), NewTime: pathModTime(req.New),
	})
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
