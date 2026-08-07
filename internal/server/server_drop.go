package server

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ayame-editor/ayame-diff/internal/atomicfile"
)

// handleDrop streams browser-dropped files to a private local cache. Browsers
// intentionally hide native absolute paths, so the GUI cannot otherwise pass
// a dropped File to the existing path-based comparison engines.
func (s *Server) handleDrop(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
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
