package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// useEnglish pins the locale so message assertions are deterministic. It also
// clears AYAME_DIFF_DEBUG so the raw wording stays hidden.
func useEnglish(t *testing.T) {
	t.Helper()
	t.Setenv("LC_ALL", "en_US.UTF-8")
	t.Setenv("LC_MESSAGES", "")
	t.Setenv("LANG", "en_US.UTF-8")
	t.Setenv(debugEnv, "")
}

func TestCLIExplainsMissingFileInJapanese(t *testing.T) {
	useEnglish(t)
	t.Setenv("LC_ALL", "ja_JP.UTF-8")
	var stdout, stderr bytes.Buffer

	if code := runText([]string{"/no/such/old.txt", "/no/such/new.txt"}, &stdout, &stderr); code != exitError {
		t.Fatalf("code = %d, want %d", code, exitError)
	}
	got := stderr.String()
	if !strings.Contains(got, "ファイルが見つかりません") {
		t.Fatalf("stderr = %q, want a Japanese not-found message", got)
	}
	if !strings.Contains(got, "パスを確認") {
		t.Fatalf("stderr = %q, want a remedy", got)
	}
	assertNoRawWording(t, got)
}

func TestCLIExplainsMissingFileInEnglish(t *testing.T) {
	useEnglish(t)
	var stdout, stderr bytes.Buffer

	if code := runText([]string{"/no/such/old.txt", "/no/such/new.txt"}, &stdout, &stderr); code != exitError {
		t.Fatalf("code = %d, want %d", code, exitError)
	}
	got := stderr.String()
	if !strings.Contains(got, "The file was not found.") {
		t.Fatalf("stderr = %q, want an English not-found message", got)
	}
	if !strings.Contains(got, "--gui") {
		t.Fatalf("stderr = %q, want the GUI hint", got)
	}
	assertNoRawWording(t, got)
}

func TestCLIExplainsBadIntegerWithoutStrconv(t *testing.T) {
	useEnglish(t)
	var stdout, stderr bytes.Buffer

	code := runCSV([]string{"--left", "a", "--right", "b", "--out", "c", "--key-index", "x"}, &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("code = %d, want %d", code, exitUsage)
	}
	got := stderr.String()
	if !strings.Contains(got, "--key-index") || !strings.Contains(got, "whole number") {
		t.Fatalf("stderr = %q, want the flag and accepted form", got)
	}
	assertNoRawWording(t, got)
}

func TestCLIDebugKeepsRawWording(t *testing.T) {
	useEnglish(t)
	t.Setenv(debugEnv, "1")
	var stdout, stderr bytes.Buffer

	if code := runCSV([]string{"--left", "a", "--right", "b", "--out", "c", "--key-index", "x"}, &stdout, &stderr); code != exitUsage {
		t.Fatalf("code = %d, want %d", code, exitUsage)
	}
	if got := stderr.String(); !strings.Contains(got, "strconv") {
		t.Fatalf("stderr = %q, want the raw strconv text under %s", got, debugEnv)
	}
}

func TestCLIExplainsMalformedProjectJSON(t *testing.T) {
	useEnglish(t)
	path := filepath.Join(t.TempDir(), "broken.ayamediff.json")
	if err := os.WriteFile(path, []byte("{bad"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	if code := runCSV([]string{"--project", path}, &stdout, &stderr); code != exitError {
		t.Fatalf("code = %d, want %d", code, exitError)
	}
	got := stderr.String()
	if !strings.Contains(got, "request could not be read") {
		t.Fatalf("stderr = %q, want the malformed-JSON explanation", got)
	}
	assertNoRawWording(t, got)
}

func TestCLIExplainsOutputIsInput(t *testing.T) {
	useEnglish(t)
	var stdout, stderr bytes.Buffer

	code := runThreeWay([]string{"text", "--output", "same.txt", "same.txt", "right.txt", "base.txt"}, &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("code = %d, want %d", code, exitUsage)
	}
	got := stderr.String()
	if !strings.Contains(got, "output is one of the compared inputs") {
		t.Fatalf("stderr = %q, want the overwrite explanation", got)
	}
	if strings.Contains(got, "merge output must differ") {
		t.Fatalf("stderr = %q, still uses internal wording", got)
	}
}

// assertNoRawWording pins the issue's completion condition for the CLI: a
// mapped failure must not show strconv or syscall wording.
func assertNoRawWording(t *testing.T, got string) {
	t.Helper()
	for _, banned := range []string{"strconv", "no such file", "invalid syntax", "syscall"} {
		if strings.Contains(got, banned) {
			t.Errorf("stderr %q leaks raw wording %q", got, banned)
		}
	}
}
