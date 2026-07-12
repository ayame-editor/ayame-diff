package atomicfile

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteCreatesAndReplaces(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "nested", "result.txt")
	for _, content := range []string{"first", "second"} {
		if err := Write(path, Options{}, func(writer io.Writer) error {
			_, err := io.WriteString(writer, content)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(path)
		if err != nil || string(data) != content {
			t.Fatalf("data=%q err=%v", data, err)
		}
	}
}

func TestWriteFailurePreservesDestinationAndCleansStage(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "result.txt")
	if err := os.WriteFile(path, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	wantErr := errors.New("write failed")
	err := Write(path, Options{Pattern: ".failure-*"}, func(writer io.Writer) error {
		_, _ = io.WriteString(writer, "partial")
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("err=%v", err)
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil || string(data) != "original" {
		t.Fatalf("data=%q err=%v", data, readErr)
	}
	matches, globErr := filepath.Glob(filepath.Join(dir, ".failure-*"))
	if globErr != nil || len(matches) != 0 {
		t.Fatalf("staged files=%v err=%v", matches, globErr)
	}
}

func TestWriteRejectsEmptyPath(t *testing.T) {
	t.Parallel()
	if err := Write("", Options{}, func(io.Writer) error { return nil }); err == nil {
		t.Fatal("expected empty-path error")
	}
}
