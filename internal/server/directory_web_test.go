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

// TestDirectoryEntryOneSidedDiff covers opening an added or removed folder
// result (#104). The absent side is deliberately explicit: an ordinary typo or
// missing file must still fail instead of silently comparing against empty.
func TestDirectoryEntryOneSidedDiff(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "only.txt")
	if err := os.WriteFile(path, []byte("only line\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := newTestServer(t)
	for _, tc := range []struct {
		name        string
		body        map[string]any
		wantAdded   uint64
		wantDeleted uint64
	}{
		{
			name: "added",
			body: map[string]any{
				"old": filepath.Join(filepath.Dir(path), "missing-old.txt"),
				"new": path, "oldAbsent": true, "mode": "text",
			},
			wantAdded: 1,
		},
		{
			name: "removed",
			body: map[string]any{
				"old": path, "new": filepath.Join(filepath.Dir(path), "missing-new.txt"),
				"newAbsent": true, "mode": "text",
			},
			wantDeleted: 1,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body, err := json.Marshal(tc.body)
			if err != nil {
				t.Fatal(err)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/diff", bytes.NewReader(body)))
			if rec.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body)
			}
			var response diffResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if response.Added != tc.wantAdded || response.Deleted != tc.wantDeleted {
				t.Fatalf("added=%d deleted=%d, want %d/%d", response.Added, response.Deleted, tc.wantAdded, tc.wantDeleted)
			}
		})
	}

	for _, body := range []string{
		`{"old":"old.txt","new":"new.txt","oldAbsent":true,"newAbsent":true}`,
		`{"inline":true,"oldText":"","newText":"x","oldAbsent":true}`,
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/diff", strings.NewReader(body)))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("invalid absent-side request status=%d body=%s", rec.Code, rec.Body)
		}
	}
}

// TestDirectoryTreeAssetsAreWired ensures the tested pure helpers are the ones
// loaded by the page and that the browser path retains the accessibility and
// metadata-column affordances from #104.
func TestDirectoryTreeAssetsAreWired(t *testing.T) {
	t.Parallel()

	index := readWebAsset(t, "index.html")
	app := readWebAsset(t, "app.js")
	style := readWebAsset(t, "style.css")
	if !strings.Contains(index, `src="directory.js"`) {
		t.Fatal("index.html does not load directory.js")
	}
	if strings.Index(index, `src="directory.js"`) > strings.Index(index, `src="app.js"`) {
		t.Error("directory.js loads after app.js, which consumes it")
	}
	for _, want := range []string{
		"globalThis.AyameDirectory",
		"directoryEntryRequest(entry, body.old, body.new)",
		`row.tabIndex = -1`,
		`t("folderSize")`,
		`t("folderModified")`,
		`$("dirSearch").value`,
		`$("addSync").hidden = false`,
	} {
		if !strings.Contains(app, want) {
			t.Errorf("app.js missing %q", want)
		}
	}
	for _, want := range []string{".dir-header", ".dir-folder", ".dir-size", ".dir-stamp"} {
		if !strings.Contains(style, want) {
			t.Errorf("style.css missing %q", want)
		}
	}
}
