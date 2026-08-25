package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ayame-editor/ayame-diff/internal/textfile"
)

func newFileServer(t *testing.T) *Server {
	t.Helper()
	srv, err := NewWithOptions(Options{Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	return srv
}

func call(t *testing.T, srv *Server, route string, body any) *httptest.ResponseRecorder {
	t.Helper()
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, route, strings.NewReader(string(encoded)))
	request.Header.Set(tokenHeader, srv.Token())
	recorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recorder, request)
	return recorder
}

func decode[T any](t *testing.T, recorder *httptest.ResponseRecorder) T {
	t.Helper()
	var out T
	if err := json.NewDecoder(recorder.Body).Decode(&out); err != nil {
		t.Fatalf("%s: %v", recorder.Body.String(), err)
	}
	return out
}

// An editable pane is only safe if saving reproduces the file it opened. These
// are the conventions a naive write would silently normalize away (#255, #159).
func TestEditableSaveRoundTripsFileConventions(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		original []byte
		hint     string
		edit     func([]string) []string
		want     []byte
	}{
		{
			name:     "utf-8 bom and crlf",
			original: append([]byte{0xEF, 0xBB, 0xBF}, []byte("alpha\r\nbeta\r\n")...),
			edit:     func(lines []string) []string { return []string{lines[0], "BETA"} },
			want:     append([]byte{0xEF, 0xBB, 0xBF}, []byte("alpha\r\nBETA\r\n")...),
		},
		{
			name:     "no final newline",
			original: []byte("alpha\nbeta"),
			edit:     func(lines []string) []string { return []string{"ALPHA", lines[1]} },
			want:     []byte("ALPHA\nbeta"),
		},
		{
			name:     "shift_jis stays shift_jis",
			original: []byte{0x93, 0xFA, 0x96, 0x7B, 0x8C, 0xEA, 0x0A}, // 日本語\n
			hint:     "shift_jis",
			edit:     func(lines []string) []string { return []string{lines[0] + "!"} },
			want:     []byte{0x93, 0xFA, 0x96, 0x7B, 0x8C, 0xEA, 0x21, 0x0A},
		},
		{
			name:     "old mac line endings",
			original: []byte("alpha\rbeta\r"),
			edit:     func(lines []string) []string { return []string{lines[0], "BETA"} },
			want:     []byte("alpha\rBETA\r"),
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			srv := newFileServer(t)
			path := filepath.Join(t.TempDir(), "input.txt")
			if err := os.WriteFile(path, test.original, 0o644); err != nil {
				t.Fatal(err)
			}

			read := decode[fileReadResponse](t, call(t, srv, "/api/file/read", fileReadRequest{Path: path, Encoding: test.hint}))
			if read.ReadOnly {
				t.Fatal("a writable file was reported read-only")
			}
			saved := call(t, srv, "/api/file/save", fileSaveRequest{
				Path:      path,
				Lines:     test.edit(read.Lines),
				Profile:   read.Profile,
				Expect:    &read.Stamp,
				Overwrite: true,
			})
			if saved.Code != http.StatusOK {
				t.Fatalf("save returned %d: %s", saved.Code, saved.Body.String())
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != string(test.want) {
				t.Fatalf("file is %q, want %q", got, test.want)
			}
		})
	}
}

// Overwriting a file someone else just wrote is the one failure that destroys
// work, so a stale save is refused until the user decides.
func TestSaveRefusesAFileThatChangedUnderTheEditor(t *testing.T) {
	t.Parallel()

	srv := newFileServer(t)
	path := filepath.Join(t.TempDir(), "input.txt")
	if err := os.WriteFile(path, []byte("alpha\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	read := decode[fileReadResponse](t, call(t, srv, "/api/file/read", fileReadRequest{Path: path}))

	if err := os.WriteFile(path, []byte("written by someone else\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stale := call(t, srv, "/api/file/save", fileSaveRequest{
		Path: path, Lines: []string{"mine"}, Profile: read.Profile, Expect: &read.Stamp, Overwrite: true,
	})
	if stale.Code != http.StatusConflict {
		t.Fatalf("a stale save returned %d, want %d", stale.Code, http.StatusConflict)
	}
	if code := decode[errorBody](t, stale).Code; code != CodeStaleWrite {
		t.Errorf("code = %q, want %q", code, CodeStaleWrite)
	}
	if content, _ := os.ReadFile(path); string(content) != "written by someone else\n" {
		t.Fatalf("the other writer's content was overwritten: %q", content)
	}

	forced := call(t, srv, "/api/file/save", fileSaveRequest{
		Path: path, Lines: []string{"mine"}, Profile: read.Profile, Expect: &read.Stamp, Overwrite: true, Force: true,
	})
	if forced.Code != http.StatusOK {
		t.Fatalf("a forced save returned %d: %s", forced.Code, forced.Body.String())
	}
	if content, _ := os.ReadFile(path); string(content) != "mine\n" {
		t.Fatalf("the forced save wrote %q", content)
	}
}

func TestSaveRequiresAnExplicitOverwrite(t *testing.T) {
	t.Parallel()

	srv := newFileServer(t)
	path := filepath.Join(t.TempDir(), "input.txt")
	if err := os.WriteFile(path, []byte("alpha\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	recorder := call(t, srv, "/api/file/save", fileSaveRequest{Path: path, Lines: []string{"beta"}, Profile: textfile.Profile{LineEnding: "\n", FinalNewline: true}})
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	if code := decode[errorBody](t, recorder).Code; code != CodeOverwriteRefused {
		t.Errorf("code = %q, want %q", code, CodeOverwriteRefused)
	}
	if content, _ := os.ReadFile(path); string(content) != "alpha\n" {
		t.Fatalf("the file was written without an overwrite: %q", content)
	}
}

func TestReadRefusesWhatCannotBeEdited(t *testing.T) {
	t.Parallel()

	srv := newFileServer(t)
	dir := t.TempDir()

	folder := call(t, srv, "/api/file/read", fileReadRequest{Path: dir})
	if folder.Code != http.StatusBadRequest {
		t.Errorf("a folder returned %d, want %d", folder.Code, http.StatusBadRequest)
	}

	empty := call(t, srv, "/api/file/read", fileReadRequest{Path: "  "})
	if empty.Code != http.StatusBadRequest {
		t.Errorf("an empty path returned %d, want %d", empty.Code, http.StatusBadRequest)
	}
	if code := decode[errorBody](t, empty).Code; code != CodeInvalidPath {
		t.Errorf("code = %q, want %q", code, CodeInvalidPath)
	}

	missing := call(t, srv, "/api/file/read", fileReadRequest{Path: filepath.Join(dir, "missing.txt")})
	if missing.Code != http.StatusNotFound {
		t.Errorf("a missing file returned %d, want %d", missing.Code, http.StatusNotFound)
	}

	big := filepath.Join(dir, "big.txt")
	if err := os.WriteFile(big, make([]byte, maxEditableBytes+1), 0o644); err != nil {
		t.Fatal(err)
	}
	oversized := call(t, srv, "/api/file/read", fileReadRequest{Path: big})
	if oversized.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("an oversized file returned %d, want %d", oversized.Code, http.StatusRequestEntityTooLarge)
	}
	if code := decode[errorBody](t, oversized).Code; code != CodeUnsupportedInput {
		t.Errorf("code = %q, want %q", code, CodeUnsupportedInput)
	}
}

// A read-only file must be reported as such when it is opened, not discovered
// when a save fails after the user has already typed into it.
func TestReadReportsAReadOnlyFile(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root ignores the write bit")
	}
	t.Parallel()

	srv := newFileServer(t)
	path := filepath.Join(t.TempDir(), "locked.txt")
	if err := os.WriteFile(path, []byte("alpha\n"), 0o444); err != nil {
		t.Fatal(err)
	}
	read := decode[fileReadResponse](t, call(t, srv, "/api/file/read", fileReadRequest{Path: path}))
	if !read.ReadOnly {
		t.Fatal("a read-only file was reported as editable")
	}
}
