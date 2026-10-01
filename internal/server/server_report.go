package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ayame-editor/ayame-diff/internal/confreport"
	"github.com/ayame-editor/ayame-diff/internal/linediff"
)

// reportRequest is the POST body for /api/report. It carries the same
// comparison fields as /api/diff, plus the review state and the export
// options. ConfirmedHunks is a pointer so an absent field means "this build
// cannot report confirmation" rather than "nothing is confirmed".
type reportRequest struct {
	diffRequest
	Format          string          `json:"format"` // json | markdown | html
	IncludeContent  bool            `json:"includeContent"`
	ConfirmedHunks  *[]int          `json:"confirmedHunks"`
	ReadHunks       []int           `json:"readHunks"`
	ComparisonState json.RawMessage `json:"comparisonState"`
	ReproduceURL    string          `json:"reproduceURL"`
}

// handleReport builds the confirmation/verification report for a text or sorted
// comparison (#296). It recomputes the diff so the report reflects the same
// retained hunks the browser saw, then renders JSON, Markdown, or printable
// HTML. Content is never embedded unless includeContent is explicitly set.
func (s *Server) handleReport(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	var req reportRequest
	if err := decodeDiffJSON(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if err := validateDiffSources(req.diffRequest); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	format, err := parseReportFormat(req.Format)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	// The report is built from a line diff, so it covers the two text modes.
	// CSV, folder, and three-way comparisons would need their own summaries;
	// rejecting them here keeps a misleading report from being produced.
	switch req.Mode {
	case "", "text", "sorted":
	default:
		writeError(w, http.StatusBadRequest, "report export supports text and sorted comparisons only")
		return
	}
	if req.Mode == "sorted" && req.DetectMoves {
		// Move detection is a text-mode concept; ignore it rather than failing.
		req.DetectMoves = false
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

	oldLines, newLines, closeLines, err := openRequestLines(req.diffRequest)
	if err != nil {
		writeClassifiedError(w, err, http.StatusBadRequest)
		return
	}
	defer closeLines()
	if err := linediff.ValidateSyncPoints(req.SyncPoints, oldLines.Count(), newLines.Count()); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	options, err := requestDiffOptions(req.diffRequest, maxHunks, window)
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

	report := confreport.Build(confreport.BuildOptions{
		Version:        s.version,
		GeneratedAt:    time.Now().UTC(),
		Mode:           defaultTextMode(req.Mode),
		Inputs:         reportInputs(req, oldLines, newLines),
		Conditions:     reportConditions(req, window, maxHunks, maxLines),
		Old:            oldLines,
		New:            newLines,
		Result:         res,
		ConfirmedHunks: req.ConfirmedHunks,
		ReadHunks:      req.ReadHunks,
		Reproduce:      reportReproduce(req),
		IncludeContent: req.IncludeContent,
	})
	writeReport(w, format, report)
}

// parseReportFormat maps a request value to a renderer. An empty value defaults
// to JSON, the machine-readable form.
func parseReportFormat(value string) (string, error) {
	switch strings.ToLower(value) {
	case "", "json":
		return "json", nil
	case "markdown", "md":
		return "markdown", nil
	case "html":
		return "html", nil
	default:
		return "", fmt.Errorf("format must be json, markdown, or html")
	}
}

func writeReport(w http.ResponseWriter, format string, report *confreport.Report) {
	var contentType, filename string
	switch format {
	case "markdown":
		contentType, filename = "text/markdown; charset=utf-8", "ayame-report.md"
	case "html":
		contentType, filename = "text/html; charset=utf-8", "ayame-report.html"
	default:
		contentType, filename = "application/json; charset=utf-8", "ayame-report.json"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	var err error
	switch format {
	case "markdown":
		err = report.WriteMarkdown(w)
	case "html":
		err = report.WriteHTML(w)
	default:
		err = report.WriteJSON(w)
	}
	_ = err // the body may already be committed; nothing useful to send on failure
}

func defaultTextMode(mode string) string {
	if mode == "" {
		return "text"
	}
	return mode
}

func reportInputs(req reportRequest, oldLines, newLines linediff.Lines) []confreport.Input {
	return []confreport.Input{
		describeReportInput("left", req.Old, req.OldAbsent, req.Inline, req.OldText, reportEncoding(oldLines)),
		describeReportInput("right", req.New, req.NewAbsent, req.Inline, req.NewText, reportEncoding(newLines)),
	}
}

// describeReportInput gathers the metadata that lets a reader verify later that
// the same files were reviewed. The SHA-256 is over the bytes on disk (the
// compressed bytes for .gz inputs), which is what a later check re-reads.
func describeReportInput(label, path string, absent, inline bool, text, encoding string) confreport.Input {
	input := confreport.Input{Label: label, Path: path, Absent: absent, Inline: inline, Exists: true}
	if absent {
		input.Exists = false
		input.Path = ""
		return input
	}
	if inline {
		input.Path = ""
		data := []byte(text)
		input.Size = int64(len(data))
		input.SHA256 = sha256Hex(data)
		input.Encoding = "utf-8"
		return input
	}
	input.Encoding = encoding
	if absolute, err := filepath.Abs(path); err == nil {
		input.AbsPath = absolute
	}
	info, err := os.Stat(path)
	if err != nil {
		input.Exists = false
		return input
	}
	input.Size = info.Size()
	input.ModTime = info.ModTime().UTC().Format(time.RFC3339Nano)
	if sum, err := fileSHA256(path); err == nil {
		input.SHA256 = sum
	}
	return input
}

// reportEncoding reports the concrete encoding an opened path side decoded
// from. Sorted sources do not expose one, so the field stays empty rather than
// claiming a guess.
func reportEncoding(lines linediff.Lines) string {
	if reporter, ok := lines.(encodingReporter); ok {
		return reporter.Encoding()
	}
	return ""
}

func reportConditions(req reportRequest, window uint64, maxHunks int, maxLines uint64) confreport.Conditions {
	whitespace := req.Whitespace
	if whitespace == "" {
		whitespace = "none"
	}
	syncPoints := make([]confreport.SyncPoint, len(req.SyncPoints))
	for i, point := range req.SyncPoints {
		syncPoints[i] = confreport.SyncPoint{Old: point.Old, New: point.New}
	}
	return confreport.Conditions{
		Mode:               defaultTextMode(req.Mode),
		Encoding:           req.Encoding,
		Window:             window,
		MaxHunks:           maxHunks,
		MaxLines:           maxLines,
		IgnoreCase:         req.IgnoreCase,
		Whitespace:         whitespace,
		IgnoreEOL:          req.IgnoreEOL,
		IgnoreTrailingEOL:  req.IgnoreTrailingEOL,
		LineFilters:        req.LineFilters,
		SyncPoints:         syncPoints,
		DetectMoves:        req.DetectMoves,
		MoveMinLines:       req.MoveMinLines,
		Numeric:            req.Numeric,
		Reverse:            req.Reverse,
		IgnoredHunkIndexes: req.IgnoredHunks,
	}
}

// reportReproduce records the versioned URL state so the comparison can be
// replayed. A scratch comparison has no reproducible path state, and a malformed
// state is reported as a note rather than failing the whole export.
func reportReproduce(req reportRequest) confreport.Reproduce {
	reproduce := confreport.Reproduce{StateVersion: 1, URL: req.ReproduceURL}
	if len(req.ComparisonState) == 0 {
		reproduce.Note = "No comparison state was supplied, so this report cannot re-run the comparison from a URL. " +
			"Scratch (pasted-text) comparisons have no reproducible path state."
		return reproduce
	}
	fragment, err := confreport.EncodeFragment(req.ComparisonState)
	if err != nil {
		reproduce.Note = "The supplied comparison state is not usable for reproduction: " + err.Error() + "."
		return reproduce
	}
	reproduce.State = req.ComparisonState
	reproduce.Fragment = fragment
	return reproduce
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// fileSHA256 hashes a file's bytes without loading it whole.
func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
