package server

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/hjosugi/ayame-diff/internal/dircompare"
	"github.com/hjosugi/ayame-diff/internal/project"
)

func (s *Server) handleProjectSave(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
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
			writeError(w, http.StatusBadRequest, "directory project, left, and right paths are required")
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
		writeError(w, http.StatusBadRequest, "both left and right paths are required")
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
	body, ok := decodePostJSON[struct {
		Path string `json:"path"`
	}](w, r, "project path is required")
	if !ok {
		return
	}
	if body.Path == "" {
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
	if !requireMethod(w, r, http.MethodGet) {
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
