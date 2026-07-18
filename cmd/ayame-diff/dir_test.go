package main

import (
	"bytes"
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunDirTSVAndDiffExitCode(t *testing.T) {
	t.Parallel()
	oldDir, newDir := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(oldDir, "a.txt"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(newDir, "a.txt"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := runDir([]string{"--tsv", "--include", "*.txt", "--workers", "2", "--diff-exit-code", oldDir, newDir}, &stdout, &stderr)
	if code != 1 || !strings.Contains(stdout.String(), "status\tpath\told_size") || !strings.Contains(stdout.String(), "changed\ta.txt") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestRunDirWritesHTMLAndCSVReports(t *testing.T) {
	t.Parallel()
	oldDir, newDir := t.TempDir(), t.TempDir()
	for _, fixture := range []struct {
		dir, name, value string
	}{
		{oldDir, "changed.txt", "old"}, {newDir, "changed.txt", "new"},
		{oldDir, "same,quoted.txt", "same"}, {newDir, "same,quoted.txt", "same"},
	} {
		if err := os.WriteFile(filepath.Join(fixture.dir, fixture.name), []byte(fixture.value), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	htmlPath := filepath.Join(t.TempDir(), "folder.html")
	var stdout, stderr bytes.Buffer
	if code := runDir([]string{"--html", htmlPath, oldDir, newDir}, &stdout, &stderr); code != 0 {
		t.Fatalf("HTML code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	htmlData, err := os.ReadFile(htmlPath)
	if err != nil {
		t.Fatal(err)
	}
	if stdout.Len() != 0 || !strings.Contains(string(htmlData), "folder report") || !strings.Contains(string(htmlData), "changed.txt") || strings.Contains(string(htmlData), "same,quoted.txt") {
		t.Fatalf("HTML stdout=%q report=%q", stdout.String(), htmlData)
	}

	csvPath := filepath.Join(t.TempDir(), "folder.csv")
	stdout.Reset()
	stderr.Reset()
	if code := runDir([]string{"--csv", csvPath, "--all", oldDir, newDir}, &stdout, &stderr); code != 0 {
		t.Fatalf("CSV code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	f, err := os.Open(csvPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.Close() })
	records, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 3 || records[2][1] != "same,quoted.txt" || !strings.Contains(stderr.String(), "wrote "+csvPath) {
		t.Fatalf("records=%#v stderr=%q", records, stderr.String())
	}
}

func TestRunDirRejectsCombinedReportFormats(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	if code := runDir([]string{"--html", "report.html", "--csv", "report.csv", "old", "new"}, &stdout, &stderr); code != 2 || !strings.Contains(stderr.String(), "cannot be combined") {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
}

func TestRunDirValidatesArchiveLimits(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{
		{"--max-archive-entry-bytes", "0", "old", "new"},
		{"--max-archive-entry-bytes", "2MiB", "--max-archive-bytes", "1MiB", "old", "new"},
		{"--max-archive-bytes", "invalid", "old", "new"},
	} {
		var stdout, stderr bytes.Buffer
		if code := runDir(args, &stdout, &stderr); code != 2 || !strings.Contains(stderr.String(), "max-archive") {
			t.Fatalf("args=%v code=%d stderr=%q", args, code, stderr.String())
		}
	}
}

func TestRunDirAdvancedFiltersAndCompareBy(t *testing.T) {
	t.Parallel()
	oldDir, newDir := t.TempDir(), t.TempDir()
	for _, fixture := range []struct{ dir, name, value string }{
		{oldDir, "keep.log", strings.Repeat("a", 2048)}, {newDir, "keep.log", strings.Repeat("b", 2048)},
		{oldDir, "small.log", "a"}, {newDir, "small.log", "b"},
		{oldDir, "skip.txt", strings.Repeat("a", 2048)}, {newDir, "skip.txt", strings.Repeat("b", 2048)},
	} {
		if err := os.WriteFile(filepath.Join(fixture.dir, fixture.name), []byte(fixture.value), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var stdout, stderr bytes.Buffer
	code := runDir([]string{"--filter", `size > 1KiB and name =~ '\.log$'`, "--compare-by", "size", "--all", oldDir, newDir}, &stdout, &stderr)
	if code != 0 || !strings.Contains(stdout.String(), "= keep.log") || strings.Contains(stdout.String(), "small.log") || strings.Contains(stdout.String(), "skip.txt") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := runDir([]string{"--compare-by", "hash", oldDir, newDir}, &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), "~ keep.log") {
		t.Fatalf("hash code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestRunDirLoadsEmbeddedDirectoryProject(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	oldDir, newDir := filepath.Join(root, "old"), filepath.Join(root, "new")
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{oldDir, newDir} {
		if err := os.WriteFile(filepath.Join(dir, "app.log"), []byte("same"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, ".git", "config"), []byte("different"+dir), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	projectPath := filepath.Join(root, "compare.ayamediff.json")
	project := `{"version":1,"mode":"dir","directory":{"old":"old","new":"new","filter_sets":["vcs"],"compare_by":"contents"}}`
	if err := os.WriteFile(projectPath, []byte(project), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := runDir([]string{"--filter-file", projectPath, "--all"}, &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), "= app.log") || strings.Contains(stdout.String(), ".git") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}

	stdout.Reset()
	if code := runDir([]string{"--list-filter-sets"}, &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), "development") {
		t.Fatalf("sets code=%d stdout=%q", code, stdout.String())
	}
}
