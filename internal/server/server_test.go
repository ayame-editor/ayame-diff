package server

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
	return authorizedHandler(s)
}

// authorizedHandler wraps a server so every test request carries its API token,
// keeping each test focused on the behavior it is about. The token requirement
// itself is covered against the bare handler in auth_test.go (#108).
func authorizedHandler(s *Server) http.Handler {
	handler := s.Handler()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Set(tokenHeader, s.Token())
		handler.ServeHTTP(w, r)
	})
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

func TestHealthRejectsNonGETMethods(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	newTestServer(t).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/health", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

// TestSecurityHeaders checks every response carries the hardening headers (#146).
func TestSecurityHeaders(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	newTestServer(t).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	for header, want := range map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
	} {
		if got := rec.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
	if csp := rec.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "default-src 'self'") || !strings.Contains(csp, "frame-ancestors 'none'") {
		t.Errorf("Content-Security-Policy = %q", csp)
	}
}

// TestCSRFOriginGate verifies the Origin check (#145): a cross-origin
// state-changing request is rejected before it can touch the filesystem, while
// same-origin requests, requests with no Origin (curl / the native GUI), and
// safe GET reads are all allowed through. httptest requests default to Host
// "example.com".
func TestCSRFOriginGate(t *testing.T) {
	t.Parallel()
	h := newTestServer(t)
	post := func(origin string) int {
		req := httptest.NewRequest(http.MethodPost, "/api/diff", strings.NewReader(`{}`))
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	if code := post("https://evil.example.com"); code != http.StatusForbidden {
		t.Errorf("cross-origin POST = %d, want 403", code)
	}
	// Same-origin and Origin-less requests clear the gate (they then fail body
	// validation with 400, which is fine — the point is they are not 403).
	if code := post("http://example.com"); code == http.StatusForbidden {
		t.Errorf("same-origin POST was rejected as cross-origin")
	}
	if code := post(""); code == http.StatusForbidden {
		t.Errorf("Origin-less POST was rejected as cross-origin")
	}
	// A loopback Origin is accepted even if it doesn't match the test Host.
	if code := post("http://127.0.0.1:8080"); code == http.StatusForbidden {
		t.Errorf("loopback Origin was rejected as cross-origin")
	}
	// GET is exempt: cross-origin reads can't be read back cross-origin anyway.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("GET with foreign Origin = %d, want 200", rec.Code)
	}
}

func TestStatusForError(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name     string
		err      error
		fallback int
		want     int
	}{
		{"missing", os.ErrNotExist, http.StatusInternalServerError, http.StatusNotFound},
		{"permission", os.ErrPermission, http.StatusInternalServerError, http.StatusForbidden},
		{"cancelled", context.Canceled, http.StatusInternalServerError, http.StatusRequestTimeout},
		{"deadline", context.DeadlineExceeded, http.StatusInternalServerError, http.StatusRequestTimeout},
		{"unresolved merge", engine.ErrUnresolvedRows, http.StatusInternalServerError, http.StatusBadRequest},
		{"internal", errors.New("disk failed"), http.StatusInternalServerError, http.StatusInternalServerError},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := statusForError(test.err, test.fallback); got != test.want {
				t.Fatalf("statusForError(%v) = %d, want %d", test.err, got, test.want)
			}
		})
	}
}

func TestMissingResourcesReturnNotFound(t *testing.T) {
	t.Parallel()
	h := newTestServer(t)
	missing := filepath.Join(t.TempDir(), "missing")
	for _, endpoint := range []string{"/api/path-info?path=", "/api/files?path="} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, endpoint+url.QueryEscape(missing), nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s status = %d body=%s", endpoint, rec.Code, rec.Body.String())
		}
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
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/path-info?path="+url.QueryEscape(dir), nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"directory":true`) {
		t.Fatalf("path-info status=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/drop?session=test&relative=dropped.txt", strings.NewReader("dropped content")))
	if rec.Code != http.StatusOK {
		t.Fatalf("drop status=%d body=%s", rec.Code, rec.Body.String())
	}
	var dropped struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &dropped); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(filepath.Dir(dropped.Path)) })
	if content, err := os.ReadFile(dropped.Path); err != nil || string(content) != "dropped content" {
		t.Fatalf("drop content=%q err=%v", content, err)
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

func TestDirectoryFilterPreviewAndProjectAPI(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	oldDir, newDir := filepath.Join(root, "old"), filepath.Join(root, "new")
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range []struct{ dir, name, value string }{
		{oldDir, "large.log", strings.Repeat("a", 2048)}, {newDir, "large.log", strings.Repeat("b", 2048)},
		{oldDir, "small.log", "a"}, {newDir, "small.log", "b"},
	} {
		if err := os.WriteFile(filepath.Join(fixture.dir, fixture.name), []byte(fixture.value), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	handler := newTestServer(t)
	req := dirRequest{Mode: "dir", Old: oldDir, New: newDir, Filter: "size > 1KiB", CompareBy: "size", Workers: 2}
	body, _ := json.Marshal(req)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/dir/preview", bytes.NewReader(body)))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"union_count":1`) || !strings.Contains(rec.Body.String(), "large.log") {
		t.Fatalf("preview status=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/dir/diff", bytes.NewReader(body)))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"same":1`) || strings.Contains(rec.Body.String(), "small.log") {
		t.Fatalf("diff status=%d body=%s", rec.Code, rec.Body.String())
	}

	projectPath := filepath.Join(root, "folder.ayamediff.json")
	req.ProjectPath = projectPath
	body, _ = json.Marshal(req)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/project/save", bytes.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("save status=%d body=%s", rec.Code, rec.Body.String())
	}
	loadBody, _ := json.Marshal(map[string]string{"path": projectPath})
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/project/load", bytes.NewReader(loadBody)))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"mode":"dir"`) || !strings.Contains(rec.Body.String(), `"compareBy":"size"`) || !strings.Contains(rec.Body.String(), `size \u003e 1KiB`) {
		t.Fatalf("load status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestDirectoryDiffAPIRejectsOversizedArchiveEntry(t *testing.T) {
	t.Parallel()
	archivePath := filepath.Join(t.TempDir(), "large.zip")
	archive, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(archive)
	entry, err := zw.Create("large.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write(bytes.Repeat([]byte("x"), 2048)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}

	req := dirRequest{
		Old: archivePath, New: t.TempDir(), Workers: 1,
		MaxArchiveEntryBytes: "1KiB", MaxArchiveBytes: "4KiB",
	}
	body, _ := json.Marshal(req)
	rec := httptest.NewRecorder()
	newTestServer(t).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/dir/diff", bytes.NewReader(body)))
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "archive extraction limit exceeded") {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestTextMergeAPIIsAtomicAndPreservesInputs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	oldPath, newPath, output := filepath.Join(dir, "old.txt"), filepath.Join(dir, "new.txt"), filepath.Join(dir, "merged.txt")
	if err := os.WriteFile(oldPath, []byte("same\nold\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newPath, []byte("same\nnew\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	req := textMergeRequest{diffRequest: diffRequest{Old: oldPath, New: newPath, Mode: "text", Window: 10}, Output: output, Choices: map[string]string{"0": "right"}}
	body, _ := json.Marshal(req)
	rec := httptest.NewRecorder()
	newTestServer(t).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/merge/text", bytes.NewReader(body)))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"unresolved":0`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	merged, err := os.ReadFile(output)
	if err != nil || string(merged) != "same\nnew\n" {
		t.Fatalf("merged=%q err=%v", merged, err)
	}
	old, _ := os.ReadFile(oldPath)
	newer, _ := os.ReadFile(newPath)
	if string(old) != "same\nold\n" || string(newer) != "same\nnew\n" {
		t.Fatalf("inputs changed: old=%q new=%q", old, newer)
	}

	req.Choices = nil
	req.Output = filepath.Join(dir, "rejected.txt")
	body, _ = json.Marshal(req)
	rec = httptest.NewRecorder()
	newTestServer(t).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/merge/text", bytes.NewReader(body)))
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "unresolved") {
		t.Fatalf("unresolved status=%d body=%s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(req.Output); !os.IsNotExist(err) {
		t.Fatalf("rejected output exists: %v", err)
	}
}

func TestCSVMergeAPIReconcilesRowsAndPreservesInputs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	left, right, output := filepath.Join(dir, "left.csv"), filepath.Join(dir, "right.csv"), filepath.Join(dir, "merged.csv")
	leftText, rightText := "id,name\n1,same\n2,left\n3,left-only\n", "id,name\n1,same\n2,right\n4,right-only\n"
	if err := os.WriteFile(left, []byte(leftText), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(right, []byte(rightText), 0o644); err != nil {
		t.Fatal(err)
	}
	h := newTestServer(t)
	request := csvRequest{Old: left, New: right, HasHeader: true, AlignColumnsByName: true, KeyMode: "include", KeyNames: []string{"id"}, MaxRows: 20}
	body, _ := json.Marshal(request)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/csv/diff", bytes.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("diff status=%d body=%s", rec.Code, rec.Body.String())
	}
	var compared csvResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &compared); err != nil {
		t.Fatal(err)
	}
	choices := make(map[string]string)
	for _, difference := range compared.Differences {
		if difference.ID == "" {
			t.Fatal("CSV difference has no merge ID")
		}
		choices[difference.ID] = "right"
	}
	request.Output = output
	mergeReq := csvMergeRequest{csvRequest: request, Choices: choices}
	body, _ = json.Marshal(mergeReq)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/merge/csv", bytes.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("merge status=%d body=%s", rec.Code, rec.Body.String())
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range []string{"1,same", "2,right", "4,right-only"} {
		if !strings.Contains(string(data), row) {
			t.Fatalf("merged missing %q: %s", row, data)
		}
	}
	if strings.Contains(string(data), "left-only") {
		t.Fatalf("merged retained rejected row: %s", data)
	}
	oldAfter, _ := os.ReadFile(left)
	newAfter, _ := os.ReadFile(right)
	if string(oldAfter) != leftText || string(newAfter) != rightText {
		t.Fatalf("inputs changed: old=%q new=%q", oldAfter, newAfter)
	}
}

func TestThreeWayTextCompareAndMergeAPI(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	base, left, right, output := filepath.Join(dir, "base.txt"), filepath.Join(dir, "left.txt"), filepath.Join(dir, "right.txt"), filepath.Join(dir, "merged.txt")
	for path, value := range map[string]string{base: "base\ntail\n", left: "left\ntail\n", right: "right\ntail\n"} {
		if err := os.WriteFile(path, []byte(value), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	h := newTestServer(t)
	req := threeWayTextRequest{diffRequest: diffRequest{Old: left, New: right, Window: 16}, Base: base}
	body, _ := json.Marshal(req)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/three-way/text", bytes.NewReader(body)))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"conflicts":1`) {
		t.Fatalf("compare status=%d body=%s", rec.Code, rec.Body.String())
	}
	req.Output, req.Choices = output, map[string]string{"0": "right"}
	body, _ = json.Marshal(req)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/merge/three-way/text", bytes.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("merge status=%d body=%s", rec.Code, rec.Body.String())
	}
	data, _ := os.ReadFile(output)
	if string(data) != "right\ntail\n" {
		t.Fatalf("merged=%q", data)
	}
}

func TestThreeWayCSVCompareAndMergeAPI(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	base, left, right, output := filepath.Join(dir, "base.csv"), filepath.Join(dir, "left.csv"), filepath.Join(dir, "right.csv"), filepath.Join(dir, "merged.csv")
	for path, value := range map[string]string{base: "id,v\n1,b\n", left: "id,v\n1,l\n", right: "id,v\n1,r\n"} {
		if err := os.WriteFile(path, []byte(value), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	h := newTestServer(t)
	csvReq := csvRequest{Old: left, New: right, HasHeader: true, AlignColumnsByName: true, KeyMode: "include", KeyNames: []string{"id"}}
	req := threeWayCSVRequest{csvRequest: csvReq, Base: base}
	body, _ := json.Marshal(req)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/three-way/csv", bytes.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("compare status=%d body=%s", rec.Code, rec.Body.String())
	}
	var compared struct {
		Events []struct {
			ID string `json:"id"`
		} `json:"events"`
		Conflicts int `json:"conflicts"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &compared); err != nil || compared.Conflicts != 1 || len(compared.Events) != 1 {
		t.Fatalf("compared=%+v err=%v", compared, err)
	}
	req.Output, req.Choices = output, map[string]string{compared.Events[0].ID: "left"}
	body, _ = json.Marshal(req)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/merge/three-way/csv", bytes.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("merge status=%d body=%s", rec.Code, rec.Body.String())
	}
	data, _ := os.ReadFile(output)
	if !strings.Contains(string(data), "1,l") {
		t.Fatalf("merged=%s", data)
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

func TestDiffAPIMarksMoveDetectionSkippedWhenHunksAreOmitted(t *testing.T) {
	t.Parallel()
	body, _ := json.Marshal(diffRequest{
		Inline: true, DetectMoves: true, MoveMinLines: 1, MaxHunks: 1,
		OldText: "a\nb\nc\n", NewText: "A\nB\nC\n",
	})
	recorder := httptest.NewRecorder()
	newTestServer(t).ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/diff", bytes.NewReader(body)))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	var response diffResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.OmittedHunks == 0 || !response.MoveDetectionSkipped || response.MovedBlocks != 0 {
		t.Fatalf("response = %+v", response)
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

func TestDiffAPIsRejectInvalidSyncPoints(t *testing.T) {
	t.Parallel()
	body, _ := json.Marshal(diffRequest{
		Inline:     true,
		OldText:    "a\nb\n",
		NewText:    "a\nc\n",
		SyncPoints: []linediff.SyncPoint{{Old: 2, New: 1}},
	})
	server := newTestServer(t)
	for _, path := range []string{"/api/diff", "/api/patch", "/api/merge/text"} {
		t.Run(path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			server.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body)))
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
			}
			if !strings.Contains(recorder.Body.String(), "outside document bounds") {
				t.Fatalf("error does not explain invalid sync point: %s", recorder.Body.String())
			}
		})
	}
}

func TestDiffAPIRejectsInvalidNumericFieldsWithoutGoDetails(t *testing.T) {
	t.Parallel()
	server := newTestServer(t)
	for _, field := range []string{"window", "maxHunks", "maxLines", "moveMinLines"} {
		for _, value := range []string{"-1", "0", "1.5", "18446744073709551616"} {
			t.Run(field+"="+value, func(t *testing.T) {
				body := []byte(`{"inline":true,"oldText":"a\n","newText":"b\n","` + field + `":` + value + `}`)
				recorder := httptest.NewRecorder()
				server.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/diff", bytes.NewReader(body)))
				if recorder.Code != http.StatusBadRequest {
					t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
				}
				message := recorder.Body.String()
				if !strings.Contains(message, field) {
					t.Fatalf("error does not identify %s: %s", field, message)
				}
				for _, leaked := range []string{"diffRequest", "uint64", "Go struct", "type int"} {
					if strings.Contains(message, leaked) {
						t.Fatalf("error leaks %q: %s", leaked, message)
					}
				}
			})
		}
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
	if rec.Code != http.StatusNotFound {
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
