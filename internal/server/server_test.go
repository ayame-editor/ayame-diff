package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/hjosugi/ayame-diff/internal/engine"
	"github.com/hjosugi/ayame-diff/internal/linediff"
)

func newTestServer(t *testing.T) http.Handler {
	t.Helper()
	s, err := New("test")
	if err != nil {
		t.Fatal(err)
	}
	return s.Handler()
}

func TestHealth(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	newTestServer(t).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "ok" || body["version"] != "test" {
		t.Fatalf("health = %v", body)
	}
}

func TestIndexServed(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	newTestServer(t).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	for _, marker := range []string{"ayame-diff", `id="diffNav"`, `id="minimap"`, `id="nextDiff"`} {
		if !strings.Contains(rec.Body.String(), marker) {
			t.Fatalf("index.html missing %s", marker)
		}
	}
}

func TestCSVInspectDiffAndFileBrowser(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	left, right := filepath.Join(dir, "left.csv"), filepath.Join(dir, "right.csv")
	if err := os.WriteFile(left, []byte("id,name,value\n1,old,10\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(right, []byte("id,name,value\n1,new,11\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := newTestServer(t)
	request := csvRequest{Old: left, New: right, HasHeader: true, AlignColumnsByName: true, KeyMode: "include", KeyNames: []string{"id"}, MaxRows: 20}
	body, _ := json.Marshal(request)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/csv/inspect", bytes.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("inspect status=%d body=%s", rec.Code, rec.Body.String())
	}
	var inspection engine.InputInspection
	if err := json.Unmarshal(rec.Body.Bytes(), &inspection); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(inspection.Header, []string{"id", "name", "value"}) {
		t.Fatalf("inspection=%+v", inspection)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/csv/diff", bytes.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("diff status=%d body=%s", rec.Code, rec.Body.String())
	}
	var result csvResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Differences) != 1 || len(result.Differences[0].ChangedColumns) != 2 || result.Summary.DiffRows != 2 {
		t.Fatalf("result=%+v", result)
	}

	request.Output, request.OutputFormat, request.OutputHeader = filepath.Join(dir, "export.tsv"), "tsv", true
	body, _ = json.Marshal(request)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/csv/export", bytes.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("export status=%d body=%s", rec.Code, rec.Body.String())
	}
	exported, err := os.ReadFile(request.Output)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(exported), "_changed_cols") || !strings.Contains(string(exported), "name,value") {
		t.Fatalf("export=%s", exported)
	}
	request.ProjectPath = filepath.Join(dir, "daily.ayamediff.json")
	body, _ = json.Marshal(request)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/project/save", bytes.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("project save status=%d body=%s", rec.Code, rec.Body.String())
	}
	loadBody, _ := json.Marshal(map[string]string{"path": request.ProjectPath})
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/project/load", bytes.NewReader(loadBody)))
	if rec.Code != http.StatusOK {
		t.Fatalf("project load status=%d body=%s", rec.Code, rec.Body.String())
	}
	var loaded csvRequest
	if err := json.Unmarshal(rec.Body.Bytes(), &loaded); err != nil {
		t.Fatal(err)
	}
	if loaded.Old != left || loaded.New != right || loaded.KeyMode != "include" || loaded.ProjectPath != request.ProjectPath {
		t.Fatalf("loaded=%+v", loaded)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/files?path="+url.QueryEscape(dir), nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "left.csv") {
		t.Fatalf("files status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestDirectoryDiffAPI(t *testing.T) {
	t.Parallel()
	oldDir, newDir := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(oldDir, "a.txt"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(newDir, "a.txt"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(newDir, "b.txt"), []byte("added"), 0o644); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(dirRequest{Old: oldDir, New: newDir, Includes: []string{"*.txt"}, Workers: 2})
	rec := httptest.NewRecorder()
	newTestServer(t).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/dir/diff", bytes.NewReader(body)))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"changed":1`) || !strings.Contains(rec.Body.String(), `"added":1`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestDiffAPI(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	oldP := filepath.Join(dir, "old.txt")
	newP := filepath.Join(dir, "new.txt")
	os.WriteFile(oldP, []byte("apple\nbanana\ncherry\n"), 0o644)
	os.WriteFile(newP, []byte("apple\nblueberry\ncherry\ndate\n"), 0o644)

	reqBody, _ := json.Marshal(diffRequest{Old: oldP, New: newP, Mode: "text"})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/diff", bytes.NewReader(reqBody))
	newTestServer(t).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp diffResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.OldLines != 3 || resp.NewLines != 4 {
		t.Fatalf("lines = %d/%d", resp.OldLines, resp.NewLines)
	}
	if resp.Modified != 1 || resp.Added != 1 {
		t.Fatalf("stats: modified=%d added=%d", resp.Modified, resp.Added)
	}
	// The replace hunk must carry its line text for the frontend.
	if len(resp.Hunks) == 0 || resp.Hunks[0].Kind != "replace" ||
		len(resp.Hunks[0].Old) != 1 || resp.Hunks[0].Old[0] != "banana" ||
		resp.Hunks[0].New[0] != "blueberry" {
		t.Fatalf("first hunk = %+v", resp.Hunks)
	}
}

func TestDiffAPIDetectsMovedBlocks(t *testing.T) {
	t.Parallel()
	body, _ := json.Marshal(diffRequest{
		Inline: true, DetectMoves: true, MoveMinLines: 2,
		OldText: "top\nmove-a\nmove-b\nstay-a\nstay-b\nstay-c\nbottom\n",
		NewText: "top\nstay-a\nstay-b\nstay-c\nmove-a\nmove-b\nbottom\n",
	})
	rec := httptest.NewRecorder()
	newTestServer(t).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/diff", bytes.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var response diffResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.MovedBlocks != 1 || response.MovedLines != 2 {
		t.Fatalf("moved = %d/%d hunks=%+v", response.MovedBlocks, response.MovedLines, response.Hunks)
	}
}

func TestDiffAPISyncPointsAndIgnoredPatchAudit(t *testing.T) {
	t.Parallel()
	h := newTestServer(t)
	syncBody, _ := json.Marshal(diffRequest{
		Inline: true, Window: 2,
		OldText:    "start\na\nb\nc\nANCHOR\ntail\n",
		NewText:    "start\nx1\nx2\nx3\nx4\nx5\na\nb\nc\nANCHOR\ntail\n",
		SyncPoints: []linediff.SyncPoint{{Old: 1, New: 6}},
	})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/diff", bytes.NewReader(syncBody)))
	if rec.Code != http.StatusOK {
		t.Fatalf("sync status = %d body=%s", rec.Code, rec.Body.String())
	}
	var response diffResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Added != 5 || response.Modified != 0 {
		t.Fatalf("sync response = %+v", response)
	}

	ignoredBody, _ := json.Marshal(diffRequest{
		Inline: true, OldText: "a\nold\n", NewText: "a\nnew\n",
		PatchFormat: "unified", IgnoredHunks: []int{0},
	})
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/patch", bytes.NewReader(ignoredBody)))
	if rec.Code != http.StatusOK {
		t.Fatalf("patch status = %d body=%s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("X-Ayame-Ignored-Hunks") != "1" || rec.Body.Len() != 0 {
		t.Fatalf("ignored audit=%q patch=%q", rec.Header().Get("X-Ayame-Ignored-Hunks"), rec.Body.String())
	}
}

func TestDiffAPIErrors(t *testing.T) {
	t.Parallel()
	h := newTestServer(t)

	// GET is rejected.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/diff", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET status = %d", rec.Code)
	}

	// Missing paths.
	rec = httptest.NewRecorder()
	body, _ := json.Marshal(diffRequest{Old: "", New: ""})
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/diff", bytes.NewReader(body)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing-paths status = %d", rec.Code)
	}

	// Nonexistent file.
	rec = httptest.NewRecorder()
	body, _ = json.Marshal(diffRequest{Old: "/no/such/a", New: "/no/such/b"})
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/diff", bytes.NewReader(body)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing-file status = %d", rec.Code)
	}
}

func TestPatchAPI(t *testing.T) {
	t.Parallel()
	h := newTestServer(t)
	contextLines := 1
	for _, format := range []string{"normal", "context", "unified"} {
		t.Run(format, func(t *testing.T) {
			body, _ := json.Marshal(diffRequest{
				Inline: true, OldText: "a\r\nold", NewText: "a\r\nnew",
				PatchFormat: format, Context: &contextLines,
			})
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/patch", bytes.NewReader(body)))
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
			}
			if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/x-diff") {
				t.Fatalf("content type = %q", got)
			}
			if !strings.Contains(rec.Body.String(), `\ No newline at end of file`) {
				t.Fatalf("missing final-newline marker in:\n%s", rec.Body.String())
			}
		})
	}
}

func TestPatchAPIRejectsInvalidFormatAndBinary(t *testing.T) {
	t.Parallel()
	h := newTestServer(t)
	for _, reqBody := range []diffRequest{
		{Inline: true, OldText: "a", NewText: "b", PatchFormat: "invalid"},
		{Inline: true, OldText: "a\x00b", NewText: "a\x00c", PatchFormat: "unified"},
	} {
		body, _ := json.Marshal(reqBody)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/patch", bytes.NewReader(body)))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
		}
	}
}
