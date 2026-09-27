package server

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"strconv"

	"github.com/ayame-editor/ayame-diff/internal/linediff"
	"github.com/ayame-editor/ayame-diff/internal/merge"
	"github.com/ayame-editor/ayame-diff/internal/pathutil"
	"github.com/ayame-editor/ayame-diff/internal/threeway"
)

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
	if !requireMethod(w, r, http.MethodPost) {
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
	Manual           map[string]string `json:"manual,omitempty"`
	AllowUnresolved  bool              `json:"allowUnresolved,omitempty"`
	Overwrite        bool              `json:"overwrite,omitempty"`
	ConfirmOverwrite bool              `json:"confirmOverwrite,omitempty"`
}

// threeWayTextChoices validates the per-conflict side choices shared by the
// three-way preview and save handlers (#257).
func threeWayTextChoices(raw map[string]string) (map[int]string, error) {
	choices := make(map[int]string, len(raw))
	for idText, side := range raw {
		id, parseErr := strconv.Atoi(idText)
		if parseErr != nil || id < 0 || (side != "left" && side != "right" && side != "base") {
			return nil, fmt.Errorf("invalid conflict choice")
		}
		choices[id] = side
	}
	return choices, nil
}

func openThreeWayText(req threeWayTextRequest) (linediff.Lines, linediff.Lines, linediff.Lines, func(), error) {
	base, closeBase, err := openMode(req.Base, "text", req.Encoding, false, false)
	if err != nil {
		return nil, nil, nil, func() {}, fmt.Errorf("base: %w", err)
	}
	left, closeLeft, err := openMode(req.Old, "text", req.Encoding, false, false)
	if err != nil {
		closeBase()
		return nil, nil, nil, func() {}, leftError(err)
	}
	right, closeRight, err := openMode(req.New, "text", req.Encoding, false, false)
	if err != nil {
		closeLeft()
		closeBase()
		return nil, nil, nil, func() {}, rightError(err)
	}
	return base, left, right, func() { closeRight(); closeLeft(); closeBase() }, nil
}

func threeWayTextResult(ctx context.Context, req threeWayTextRequest) (linediff.Lines, threeway.Result, func(), error) {
	if req.Base == "" || req.Old == "" || req.New == "" {
		return nil, threeway.Result{}, func() {}, fmt.Errorf("base, left, and right paths are required")
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
	if !requireMethod(w, r, http.MethodPost) {
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
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	var req threeWayTextRequest
	if err := decodeDiffJSON(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if aliases := pathutil.Equal(req.Output, req.Base) || pathutil.Equal(req.Output, req.Old) || pathutil.Equal(req.Output, req.New); aliases && (!req.Overwrite || !req.ConfirmOverwrite) {
		writeCodedError(w, http.StatusBadRequest, CodeOverwriteRefused,
			"overwriting an input requires overwrite and explicit confirmation")
		return
	}
	base, result, closeLines, err := threeWayTextResult(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	defer closeLines()
	choices, err := threeWayTextChoices(req.Choices)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	// Capture the base file's encoding/BOM/EOL before MergeLines streams it, so
	// the written merge round-trips them instead of BOM-less UTF-8/LF (#159).
	profile := threeway.ProfileOf(base)
	merged, unresolved, err := threeway.MergeLinesWithOrigins(base, result, choices, req.AllowUnresolved)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	merged = threeway.ApplyManual(merged, req.Manual)
	lines := make([]string, len(merged))
	for i, line := range merged {
		lines[i] = line.Text
	}
	if err := threeway.WriteMerged(req.Output, lines, profile); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"output": req.Output, "conflicts": result.Conflicts, "unresolved": unresolved,
		"provenance": threeway.CountOrigins(merged),
	})
}

// handleThreeWayTextPreview resolves the current choices without writing, so
// the GUI can show the merge result line by line before saving. It always
// renders unresolved conflicts with markers, because the point of a preview is
// to see the whole result (#257).
func (s *Server) handleThreeWayTextPreview(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	var req threeWayTextRequest
	if err := decodeDiffJSON(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	base, result, closeLines, err := threeWayTextResult(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	defer closeLines()
	choices, err := threeWayTextChoices(req.Choices)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	merged, unresolved, err := threeway.MergeLinesWithOrigins(base, result, choices, true)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	merged = threeway.ApplyManual(merged, req.Manual)
	writeJSON(w, http.StatusOK, map[string]any{
		"lines": merged, "conflicts": result.Conflicts, "unresolved": unresolved,
		"provenance": threeway.CountOrigins(merged),
	})
}
