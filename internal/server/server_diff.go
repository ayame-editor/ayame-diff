package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/hjosugi/ayame-diff/internal/diffout"
	"github.com/hjosugi/ayame-diff/internal/linediff"
	"github.com/hjosugi/ayame-diff/internal/linesort"
	"github.com/hjosugi/ayame-diff/internal/linesrc"
)

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
	if !requireMethod(w, r, http.MethodPost) {
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
	if !requireMethod(w, r, http.MethodPost) {
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
		return fmt.Errorf("both left and right paths are required")
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
		return nil, nil, func() {}, fmt.Errorf("left: %w", err)
	}
	newLines, closeNew, err := openSide(req.New, req.NewAbsent)
	if err != nil {
		closeOld()
		return nil, nil, func() {}, fmt.Errorf("right: %w", err)
	}
	return oldLines, newLines, func() { closeNew(); closeOld() }, nil
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
		Whitespace: linediff.ParseWhitespace(req.Whitespace), IgnoreEOL: req.IgnoreEOL,
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
