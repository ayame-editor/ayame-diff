package server

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ayame-editor/ayame-diff/internal/textfile"
)

// Editable panes (#255) need the whole file, not the hunk slices the diff
// response carries, so the browser can hold a buffer and compare what the user
// is typing. maxEditableBytes bounds that: a file this size is already past the
// point where editing it in a browser pane is the right tool, and refusing is
// better than loading it into a tab that then stalls.
const maxEditableBytes = 8 << 20

type fileReadRequest struct {
	Path     string `json:"path"`
	Encoding string `json:"encoding,omitempty"`
}

type fileStamp struct {
	// Strings for the same reason the watch API uses them: nanosecond times and
	// large sizes exceed JavaScript's exact integer range, and the browser only
	// hands these back untouched.
	Size     string `json:"size"`
	Modified string `json:"modified"`
}

type fileReadResponse struct {
	Path    string           `json:"path"`
	Lines   []string         `json:"lines"`
	Profile textfile.Profile `json:"profile"`
	Stamp   fileStamp        `json:"stamp"`
	// ReadOnly reports that the file cannot be written back, so the GUI can say
	// so in the pane header instead of failing at save time.
	ReadOnly bool `json:"readOnly"`
}

type fileSaveRequest struct {
	Path    string           `json:"path"`
	Lines   []string         `json:"lines"`
	Profile textfile.Profile `json:"profile"`
	// Expect is the stamp the browser read. A file that moved on since then is
	// refused rather than silently overwritten; the GUI offers reload or force.
	Expect *fileStamp `json:"expect,omitempty"`
	// Overwrite must be set. Saving a pane replaces a file the user opened for
	// comparison, so the intent is explicit rather than implied by the request.
	Overwrite bool `json:"overwrite"`
	Force     bool `json:"force,omitempty"`
}

func (s *Server) handleFileRead(w http.ResponseWriter, r *http.Request) {
	request, ok := decodePostJSON[fileReadRequest](w, r, "")
	if !ok {
		return
	}
	path, ok := editablePath(w, request.Path)
	if !ok {
		return
	}
	info, err := os.Stat(path)
	if err != nil {
		writeClassifiedError(w, err, http.StatusInternalServerError)
		return
	}
	if info.IsDir() {
		writeCodedError(w, http.StatusBadRequest, CodeUnsupportedInput, "a folder cannot be opened for editing")
		return
	}
	if info.Size() > maxEditableBytes {
		writeCodedError(w, http.StatusRequestEntityTooLarge, CodeUnsupportedInput,
			fmt.Sprintf("%s is %d bytes, over the %d byte editing limit", path, info.Size(), int64(maxEditableBytes)))
		return
	}
	lines, profile, err := textfile.ReadAll(path, request.Encoding, maxEditableBytes)
	if err != nil {
		if strings.Contains(err.Error(), "editing limit") {
			writeCodedError(w, http.StatusRequestEntityTooLarge, CodeUnsupportedInput, err.Error())
			return
		}
		writeClassifiedError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, fileReadResponse{
		Path:     path,
		Lines:    lines,
		Profile:  profile,
		Stamp:    stampOf(info),
		ReadOnly: !writable(path, info),
	})
}

func (s *Server) handleFileSave(w http.ResponseWriter, r *http.Request) {
	request, ok := decodePostJSON[fileSaveRequest](w, r, "")
	if !ok {
		return
	}
	path, ok := editablePath(w, request.Path)
	if !ok {
		return
	}
	if !request.Overwrite {
		writeCodedError(w, http.StatusBadRequest, CodeOverwriteRefused,
			"saving a pane replaces the compared file and requires overwrite")
		return
	}
	if len(request.Lines) == 0 {
		writeCodedError(w, http.StatusBadRequest, CodeInvalidRequest, "a save needs the pane's lines")
		return
	}
	info, err := os.Stat(path)
	if err != nil {
		writeClassifiedError(w, err, http.StatusInternalServerError)
		return
	}
	if info.IsDir() {
		writeCodedError(w, http.StatusBadRequest, CodeUnsupportedInput, "a folder cannot be saved over")
		return
	}
	// A file that changed under the editor is the one case where writing does
	// real damage: it would discard whatever the other writer did. Refuse, and
	// let the GUI decide with the user (#251 already watches for this).
	if !request.Force && request.Expect != nil {
		if current := stampOf(info); current != *request.Expect {
			writeCodedError(w, http.StatusConflict, CodeStaleWrite,
				"the file changed on disk since it was opened; reload it or save again to overwrite")
			return
		}
	}
	if err := textfile.Write(path, request.Lines, request.Profile); err != nil {
		writeClassifiedError(w, err, http.StatusInternalServerError)
		return
	}
	saved, err := os.Stat(path)
	if err != nil {
		writeClassifiedError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"path": path, "stamp": stampOf(saved)})
}

// editablePath resolves and validates the path both endpoints take.
func editablePath(w http.ResponseWriter, raw string) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		writeCodedError(w, http.StatusBadRequest, CodeInvalidPath, "a file path is required")
		return "", false
	}
	absolute, err := filepath.Abs(trimmed)
	if err != nil {
		writeCodedError(w, http.StatusBadRequest, CodeInvalidPath, err.Error())
		return "", false
	}
	return filepath.Clean(absolute), true
}

func stampOf(info os.FileInfo) fileStamp {
	return fileStamp{
		Size:     strconv.FormatInt(info.Size(), 10),
		Modified: strconv.FormatInt(info.ModTime().UnixNano(), 10),
	}
}

// writable reports whether the file can be replaced. The mode bits answer this
// on every platform ayame-diff ships to; a read-only file is reported to the
// pane header rather than discovered when a save fails.
func writable(path string, info os.FileInfo) bool {
	if info.Mode().Perm()&0o200 == 0 {
		return false
	}
	file, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return false
	}
	_ = file.Close()
	return true
}
