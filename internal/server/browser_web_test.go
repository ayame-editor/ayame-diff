package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestFilesAPIReportsMetadata covers the browser listing (#103). Opening a file
// from the list is a choice between same-named candidates, so the size and the
// modification time have to travel with each entry; both are strings for the
// same reason the watch API uses them (they exceed JavaScript's exact range).
func TestFilesAPIReportsMetadata(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "data.csv")
	if err := os.WriteFile(path, []byte("a,b\n1,2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	h := newTestServer(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/files?path="+url.QueryEscape(dir), nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body)
	}
	var response struct {
		Entries []struct {
			Name      string `json:"name"`
			Size      int64  `json:"size"`
			Modified  string `json:"modified"`
			Directory bool   `json:"directory"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, entry := range response.Entries {
		if entry.Name != "data.csv" {
			continue
		}
		found = true
		if entry.Directory {
			t.Error("a file was reported as a directory")
		}
		if entry.Size != info.Size() {
			t.Errorf("size=%d, want %d", entry.Size, info.Size())
		}
		if _, err := strconv.ParseInt(entry.Modified, 10, 64); err != nil {
			t.Errorf("modified %q is not an epoch-nanosecond string: %v", entry.Modified, err)
		}
	}
	if !found {
		t.Fatalf("data.csv missing from %s", rec.Body)
	}
}

// TestFileBrowserAssetsAreWired guards the browser half of #103: the tested pure
// helpers are loaded, the new controls exist, and the keyboard model keeps a
// single tab stop instead of one per entry.
func TestFileBrowserAssetsAreWired(t *testing.T) {
	t.Parallel()

	index := readWebAsset(t, "index.html")
	app := readWebAsset(t, "app.js")
	style := readWebAsset(t, "style.css")

	for _, want := range []string{`id="browserHome"`, `id="browserRoot"`, `id="browserRecent"`, `id="browserFilter"`} {
		if !strings.Contains(index, want) {
			t.Errorf("index.html missing %q", want)
		}
	}
	for _, want := range []string{
		"function renderBrowserEntries(",
		"function syncBrowserFocus(",
		"rememberPlace(readBrowserPlaces(), current)",
		"formatEpochNanos(item.Modified || item.modified)",
		`addEventListener("keydown"`,
		`await loadBrowser("~")`,
	} {
		if !strings.Contains(app, want) {
			t.Errorf("app.js missing %q", want)
		}
	}
	// Roving tabindex: exactly one entry is reachable by Tab.
	if !strings.Contains(app, "row.tabIndex = index === browserFocus ? 0 : -1;") {
		t.Error("the browser keeps one tab stop per entry instead of roving focus")
	}
	if !strings.Contains(style, ".browser-entry-meta") || !strings.Contains(style, ".browser-tools") {
		t.Error("style.css has no layout for the browser tools or entry metadata")
	}
}
