package server

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"

	"github.com/ayame-editor/ayame-diff/internal/engine"
)

// Failures reach the browser as a stable code beside the English message (#94).
// The message stays for API callers and diagnostics; the GUI shows its own
// localized sentence with a remedy, which a raw syscall or encoding/json string
// can never give it. Codes are part of the API: keep them stable, add rather
// than rename.
const (
	CodeFileNotFound     = "file_not_found"
	CodePermissionDenied = "permission_denied"
	CodeInvalidPath      = "invalid_path"
	CodeInvalidRequest   = "invalid_request"
	CodeOverwriteRefused = "overwrite_refused"
	CodeUnsupportedInput = "unsupported_input"
	CodeTimeout          = "timeout"
	CodeBusy             = "busy"
	CodeUnauthorized     = "unauthorized"
	CodeInternal         = "internal"
)

// errorBody is what every failing API response carries.
type errorBody struct {
	Error string `json:"error"`
	Code  string `json:"code,omitempty"`
	// Path and Side name the input a message would otherwise have to be parsed
	// for, so the GUI can say which side failed without splitting strings.
	Path string `json:"path,omitempty"`
	Side string `json:"side,omitempty"`
}

// sideError marks which compared input failed. It keeps the "left: " / "right: "
// message prefix the API has always returned while letting the classifier
// report the side as data.
type sideError struct {
	side string
	err  error
}

func (e *sideError) Error() string { return e.side + ": " + e.err.Error() }
func (e *sideError) Unwrap() error { return e.err }

func leftError(err error) error  { return &sideError{side: "left", err: err} }
func rightError(err error) error { return &sideError{side: "right", err: err} }

// classifyError maps a failure to its API code and HTTP status. The fallback
// applies to errors this server has no specific answer for.
func classifyError(err error, fallback int) (string, int) {
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return CodeTimeout, http.StatusRequestTimeout
	case errors.Is(err, fs.ErrNotExist):
		return CodeFileNotFound, http.StatusNotFound
	case errors.Is(err, fs.ErrPermission):
		return CodePermissionDenied, http.StatusForbidden
	case errors.Is(err, engine.ErrUnresolvedRows):
		return CodeUnsupportedInput, http.StatusBadRequest
	}
	switch fallback {
	case http.StatusBadRequest:
		return CodeInvalidRequest, fallback
	case http.StatusInternalServerError:
		return CodeInternal, fallback
	default:
		return "", fallback
	}
}

// errorDetail reports the input a failure names, so the GUI can point at it.
func errorDetail(err error) (path string, side string) {
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		path = pathErr.Path
	}
	var sided *sideError
	if errors.As(err, &sided) {
		side = sided.side
	}
	return path, side
}

func statusForError(err error, fallback int) int {
	_, status := classifyError(err, fallback)
	return status
}

func writeJSONError(w http.ResponseWriter, status int, body errorBody) {
	writeJSON(w, status, body)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	code := ""
	switch status {
	case http.StatusBadRequest:
		code = CodeInvalidRequest
	case http.StatusUnauthorized:
		code = CodeUnauthorized
	case http.StatusForbidden:
		code = CodePermissionDenied
	case http.StatusNotFound:
		code = CodeFileNotFound
	case http.StatusTooManyRequests:
		code = CodeBusy
	case http.StatusRequestTimeout:
		code = CodeTimeout
	case http.StatusInternalServerError:
		code = CodeInternal
	}
	writeJSONError(w, status, errorBody{Error: msg, Code: code})
}

// writeCodedError answers a failure the handler already understands, so the
// code says more than the status alone could.
func writeCodedError(w http.ResponseWriter, status int, code, msg string) {
	writeJSONError(w, status, errorBody{Error: msg, Code: code})
}

func writeClassifiedError(w http.ResponseWriter, err error, fallback int) {
	code, status := classifyError(err, fallback)
	path, side := errorDetail(err)
	writeJSONError(w, status, errorBody{Error: err.Error(), Code: code, Path: path, Side: side})
}

// invalidRequestError answers a malformed request without echoing the decoder's
// own words at the user; the detail stays in the message for API callers.
func invalidRequestError(w http.ResponseWriter, msg string, err error) {
	if msg == "" {
		msg = fmt.Sprintf("invalid JSON: %v", err)
	}
	writeCodedError(w, http.StatusBadRequest, CodeInvalidRequest, msg)
}
