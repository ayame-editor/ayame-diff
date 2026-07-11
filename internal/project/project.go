package project

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/hjosugi/ayame-diff/internal/engine"
)

const Version = 1

// Project is the portable, versioned on-disk comparison format.
type Project struct {
	Version int           `json:"version"`
	Mode    string        `json:"mode"`
	CSV     engine.Config `json:"csv"`
	Report  Report        `json:"report,omitempty"`
}

type Report struct {
	CellDiff     bool   `json:"cell_diff,omitempty"`
	OutputFormat string `json:"output_format,omitempty"`
	HTML         string `json:"html,omitempty"`
}

func Save(path string, p Project) error {
	if p.Version == 0 {
		p.Version = Version
	}
	if p.Version != Version {
		return fmt.Errorf("unsupported project version %d", p.Version)
	}
	if p.Mode == "" {
		p.Mode = "csv"
	}
	portable := p
	base, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return err
	}
	portable.CSV.LeftPath = relative(base, portable.CSV.LeftPath)
	portable.CSV.RightPath = relative(base, portable.CSV.RightPath)
	portable.CSV.OutputPath = relative(base, portable.CSV.OutputPath)
	portable.CSV.TempDir = relative(base, portable.CSV.TempDir)
	portable.CSV.WorkDir = relative(base, portable.CSV.WorkDir)
	data, err := json.MarshalIndent(portable, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".ayamediff-*.tmp")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if _, err := temp.Write(append(data, '\n')); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tempPath, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tempPath, path); err == nil {
		return nil
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Rename(tempPath, path)
}

func Load(path string) (Project, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Project{}, err
	}
	var p Project
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&p); err != nil {
		return Project{}, fmt.Errorf("decode project: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return Project{}, fmt.Errorf("decode project: trailing JSON content")
	}
	if p.Version != Version {
		return Project{}, fmt.Errorf("unsupported project version %d", p.Version)
	}
	if p.Mode != "csv" {
		return Project{}, fmt.Errorf("unsupported project mode %q", p.Mode)
	}
	base, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return Project{}, err
	}
	p.CSV.LeftPath = absolute(base, p.CSV.LeftPath)
	p.CSV.RightPath = absolute(base, p.CSV.RightPath)
	p.CSV.OutputPath = absolute(base, p.CSV.OutputPath)
	p.CSV.TempDir = absolute(base, p.CSV.TempDir)
	p.CSV.WorkDir = absolute(base, p.CSV.WorkDir)
	return p, nil
}

func relative(base, path string) string {
	if path == "" || !filepath.IsAbs(path) {
		return path
	}
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return path
	}
	return rel
}
func absolute(base, path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Clean(filepath.Join(base, path))
}
