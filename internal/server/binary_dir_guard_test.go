package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDiffGuidesBinaryAndDirectoryInput covers #166: /api/diff no longer returns
// a mojibake 200 for binary input or a cryptic "is a directory" for a folder —
// it returns a 400 that names the right mode (bin / dir).
func TestDiffGuidesBinaryAndDirectoryInput(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	binPath := filepath.Join(dir, "a.bin")
	binary := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x0d, 'I', 'H', 'D', 'R', 0xde, 0xad}
	if err := os.WriteFile(binPath, binary, 0o644); err != nil {
		t.Fatal(err)
	}
	txtPath := filepath.Join(dir, "b.txt")
	if err := os.WriteFile(txtPath, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := newTestServer(t)

	post := func(old, new string) (int, string) {
		body, _ := json.Marshal(map[string]any{"old": old, "new": new, "mode": "text"})
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/diff", bytes.NewReader(body)))
		var payload struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &payload)
		return rec.Code, payload.Error
	}

	// Binary input is guided to bin mode, not decoded into a garbled diff.
	if code, msg := post(binPath, txtPath); code != http.StatusBadRequest || !strings.Contains(msg, "bin") {
		t.Fatalf("binary input: code=%d msg=%q, want 400 naming bin mode", code, msg)
	}
	// A directory is guided to dir mode, not a raw "is a directory".
	if code, msg := post(dir, txtPath); code != http.StatusBadRequest || !strings.Contains(msg, "dir") || strings.Contains(msg, "is a directory") {
		t.Fatalf("directory input: code=%d msg=%q, want 400 naming dir mode without raw syscall text", code, msg)
	}
}
