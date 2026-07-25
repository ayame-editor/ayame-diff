package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// File watches are authenticated long polls rather than EventSource streams.
// The UI's API credential deliberately travels in X-Ayame-Token, which the
// native EventSource API cannot set; fetch can, preserving the security model
// from #108 while a request waits for an external save (#251).
const (
	maxWatchPaths              = 3
	maxConcurrentWatchRequests = 16
)

// Variables let focused tests exercise change and timeout paths quickly.
var (
	watchPollInterval   = 250 * time.Millisecond
	watchRequestTimeout = 20 * time.Second
)

type watchPathState struct {
	Path      string `json:"path"`
	Exists    bool   `json:"exists"`
	Directory bool   `json:"directory,omitempty"`
	// Strings are intentional. Nanosecond timestamps and very large file sizes
	// exceed JavaScript's exact integer range; the browser only round-trips
	// these opaque values, so encoding them as numbers would create false
	// change notifications.
	Size     string `json:"size,omitempty"`
	Modified string `json:"modified,omitempty"`
	Mode     string `json:"mode,omitempty"`
}

type watchRequest struct {
	Paths    []string         `json:"paths"`
	Baseline []watchPathState `json:"baseline,omitempty"`
}

type watchResponse struct {
	Changed  []string         `json:"changed"`
	Snapshot []watchPathState `json:"snapshot"`
}

func (s *Server) handleWatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "use POST")
		return
	}
	select {
	case s.watchSem <- struct{}{}:
		defer func() { <-s.watchSem }()
	default:
		w.Header().Set("Retry-After", "1")
		writeError(w, http.StatusTooManyRequests, "too many active file watches; please retry shortly")
		return
	}

	var req watchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	paths, err := normalizeWatchPaths(req.Paths)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	snapshot, err := snapshotWatchPaths(paths)
	if err != nil {
		writeClassifiedError(w, err, http.StatusBadRequest)
		return
	}
	if len(req.Baseline) == 0 {
		writeJSON(w, http.StatusOK, watchResponse{Changed: []string{}, Snapshot: snapshot})
		return
	}
	if !watchBaselineMatchesPaths(req.Baseline, snapshot) {
		writeError(w, http.StatusBadRequest, "baseline must describe the requested paths in order")
		return
	}
	if changed := changedWatchPaths(req.Baseline, snapshot); len(changed) > 0 {
		writeJSON(w, http.StatusOK, watchResponse{Changed: changed, Snapshot: snapshot})
		return
	}

	ticker := time.NewTicker(watchPollInterval)
	defer ticker.Stop()
	timeout := time.NewTimer(watchRequestTimeout)
	defer timeout.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-timeout.C:
			writeJSON(w, http.StatusOK, watchResponse{Changed: []string{}, Snapshot: snapshot})
			return
		case <-ticker.C:
			snapshot, err = snapshotWatchPaths(paths)
			if err != nil {
				writeClassifiedError(w, err, http.StatusBadRequest)
				return
			}
			if changed := changedWatchPaths(req.Baseline, snapshot); len(changed) > 0 {
				writeJSON(w, http.StatusOK, watchResponse{Changed: changed, Snapshot: snapshot})
				return
			}
		}
	}
}

func normalizeWatchPaths(raw []string) ([]string, error) {
	if len(raw) == 0 {
		return nil, errors.New("at least one path is required")
	}
	if len(raw) > maxWatchPaths {
		return nil, fmt.Errorf("at most %d paths may be watched", maxWatchPaths)
	}
	paths := make([]string, 0, len(raw))
	seen := make(map[string]bool, len(raw))
	for _, value := range raw {
		value = strings.TrimSpace(value)
		if value == "" {
			return nil, errors.New("watch paths must not be empty")
		}
		absolute, err := filepath.Abs(value)
		if err != nil {
			return nil, fmt.Errorf("resolve watch path: %w", err)
		}
		absolute = filepath.Clean(absolute)
		if !seen[absolute] {
			seen[absolute] = true
			paths = append(paths, absolute)
		}
	}
	return paths, nil
}

func snapshotWatchPaths(paths []string) ([]watchPathState, error) {
	states := make([]watchPathState, 0, len(paths))
	for _, path := range paths {
		info, err := os.Stat(path)
		if errors.Is(err, os.ErrNotExist) {
			states = append(states, watchPathState{Path: path})
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("watch %s: %w", path, err)
		}
		if info.IsDir() {
			return nil, fmt.Errorf("%s is a directory; directory watching is disabled to avoid unbounded tree scans", path)
		}
		states = append(states, watchPathState{
			Path: path, Exists: true,
			Size:     strconv.FormatInt(info.Size(), 10),
			Modified: info.ModTime().UTC().Format(time.RFC3339Nano),
			Mode:     info.Mode().String(),
		})
	}
	return states, nil
}

func watchBaselineMatchesPaths(baseline, current []watchPathState) bool {
	if len(baseline) != len(current) {
		return false
	}
	for i := range current {
		if baseline[i].Path != current[i].Path {
			return false
		}
	}
	return true
}

func changedWatchPaths(baseline, current []watchPathState) []string {
	changed := make([]string, 0, len(current))
	for i := range current {
		if baseline[i] != current[i] {
			changed = append(changed, current[i].Path)
		}
	}
	return changed
}
