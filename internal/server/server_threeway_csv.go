package server

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/hjosugi/ayame-diff/internal/pathutil"
	"github.com/hjosugi/ayame-diff/internal/threeway"
)

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
		return threeway.CSVResult{}, fmt.Errorf("base, left, and right paths are required")
	}
	if req.KeyMode == "include" && len(req.KeyNames)+len(req.KeyIndexes) == 0 {
		return threeway.CSVResult{}, fmt.Errorf("select at least one key column")
	}
	cfg := csvConfig(req.csvRequest, filepath.Join(os.TempDir(), "three-way-unused.tsv"))
	return threeway.CompareCSV(r.Context(), req.Base, req.Old, req.New, cfg)
}

func (s *Server) handleThreeWayCSV(w http.ResponseWriter, r *http.Request) {
	req, ok := decodePostJSON[threeWayCSVRequest](w, r, "")
	if !ok {
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
	req, ok := decodePostJSON[threeWayCSVRequest](w, r, "")
	if !ok {
		return
	}
	if strings.TrimSpace(req.Output) == "" {
		writeError(w, http.StatusBadRequest, "output path is required")
		return
	}
	if aliases := pathutil.Equal(req.Output, req.Base) || pathutil.Equal(req.Output, req.Old) || pathutil.Equal(req.Output, req.New); aliases && (!req.Overwrite || !req.ConfirmOverwrite) {
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
