package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunThreeWayTextJSONAndMerge(t *testing.T) {
	dir := t.TempDir()
	base, left, right, output := filepath.Join(dir, "base.txt"), filepath.Join(dir, "left.txt"), filepath.Join(dir, "right.txt"), filepath.Join(dir, "merged.txt")
	for path, value := range map[string]string{base: "base\ntail\n", left: "left\ntail\n", right: "right\ntail\n"} {
		if err := os.WriteFile(path, []byte(value), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var stdout, stderr bytes.Buffer
	if code := runThreeWay([]string{"text", "--json", base, left, right}, &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), `"conflicts": 1`) {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := runThreeWay([]string{"text", "--choice", "0=right", "--output", output, base, left, right}, &stdout, &stderr); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	data, _ := os.ReadFile(output)
	if string(data) != "right\ntail\n" {
		t.Fatalf("merged=%q", data)
	}
}

func TestRunThreeWayTextMergeExitCode(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "base.txt")
	left := filepath.Join(dir, "left.txt")
	right := filepath.Join(dir, "right.txt")
	unresolvedOutput := filepath.Join(dir, "unresolved.txt")
	resolvedOutput := filepath.Join(dir, "resolved.txt")
	for path, value := range map[string]string{
		base:  "base\ntail\n",
		left:  "left\ntail\n",
		right: "right\ntail\n",
	} {
		if err := os.WriteFile(path, []byte(value), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	var stdout, stderr bytes.Buffer
	code := runThreeWay([]string{
		"text", "--allow-conflicts", "--merge-exit-code",
		"--output", unresolvedOutput, base, left, right,
	}, &stdout, &stderr)
	if code != exitDiff {
		t.Fatalf("unresolved code=%d, want %d; stderr=%s", code, exitDiff, stderr.String())
	}
	data, err := os.ReadFile(unresolvedOutput)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "<<<<<<< LEFT") {
		t.Fatalf("unresolved merge did not write conflict markers: %q", data)
	}

	stdout.Reset()
	stderr.Reset()
	code = runThreeWay([]string{
		"text", "--choice", "0=right", "--merge-exit-code",
		"--output", resolvedOutput, base, left, right,
	}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("resolved code=%d, want %d; stderr=%s", code, exitOK, stderr.String())
	}
	data, err = os.ReadFile(resolvedOutput)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "right\ntail\n" {
		t.Fatalf("resolved merge=%q", data)
	}

	stdout.Reset()
	stderr.Reset()
	code = runThreeWay([]string{
		"text", "--merge-exit-code", base, left, right,
	}, &stdout, &stderr)
	if code != exitUsage || !strings.Contains(stderr.String(), "--merge-exit-code requires --output") {
		t.Fatalf("missing output code=%d stderr=%q", code, stderr.String())
	}
}

func TestRunThreeWayCSV(t *testing.T) {
	dir := t.TempDir()
	base, left, right, output := filepath.Join(dir, "base.csv"), filepath.Join(dir, "left.csv"), filepath.Join(dir, "right.csv"), filepath.Join(dir, "merged.csv")
	for path, value := range map[string]string{base: "id,v\n1,b\n", left: "id,v\n1,l\n", right: "id,v\n1,r\n"} {
		if err := os.WriteFile(path, []byte(value), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var stdout, stderr bytes.Buffer
	if code := runThreeWay([]string{"csv", "--base", base, "--left", left, "--right", right, "--key", "id", "--json"}, &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), `"conflicts": 1`) {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	var id string
	marker := `"id": "`
	if pos := strings.Index(stdout.String(), marker); pos >= 0 {
		rest := stdout.String()[pos+len(marker):]
		id = rest[:strings.Index(rest, `"`)]
	}
	if id == "" {
		t.Fatalf("no event id: %s", stdout.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := runThreeWay([]string{"csv", "--base", base, "--left", left, "--right", right, "--key", "id", "--choice", id + "=left", "--output", output}, &stdout, &stderr); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	data, _ := os.ReadFile(output)
	if !strings.Contains(string(data), "1,l") {
		t.Fatalf("merged=%s", data)
	}
}
