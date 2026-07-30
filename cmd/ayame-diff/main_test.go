package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/hjosugi/ayame-diff/internal/engine"
	"github.com/hjosugi/ayame-diff/internal/project"
)

func TestParseFlagsDefaultsToAllKeys(t *testing.T) {
	t.Parallel()
	opts, err := parseFlags([]string{"--left", "a.tsv", "--right", "b.tsv", "--out", "d.tsv"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.ShowVersion {
		t.Fatal("showVersion = true")
	}
	cfg := opts.Engine
	if len(cfg.KeyNames) != 0 || len(cfg.KeyIndexes) != 0 || len(cfg.ExcludeKeyNames) != 0 || len(cfg.ExcludeKeyIndexes) != 0 {
		t.Fatalf("unexpected key options: %#v", cfg)
	}
}

func TestRunExitCodesAndStreams(t *testing.T) {
	tests := []struct {
		name, stdout, stderr string
		args                 []string
		code                 int
	}{
		{name: "version", args: []string{"--version"}, code: 0, stdout: "ayame-diff"},
		{name: "top-level help", args: []string{"--help"}, code: 0, stdout: "open a comparison in the browser"},
		{name: "csv help", args: []string{"csv", "--help"}, code: 0, stdout: "compares huge CSV/TSV"},
		{name: "text help", args: []string{"text", "--help"}, code: 0, stdout: "Line-level diff"},
		{name: "sorted help", args: []string{"sorted", "--help"}, code: 0, stdout: "Sort both text files"},
		{name: "dir help", args: []string{"dir", "--help"}, code: 0, stdout: "Recursively compare"},
		{name: "bin help", args: []string{"bin", "--help"}, code: 0, stdout: "Byte-level"},
		{name: "3way help", args: []string{"3way", "--help"}, code: 0, stdout: "common base"},
		{name: "serve help", args: []string{"serve", "--help"}, code: 0, stdout: "local web UI"},
		{name: "gui help", args: []string{"gui", "--help"}, code: 0, stdout: "open it in your browser"},
		{name: "update help", args: []string{"update", "--help"}, code: 0, stdout: "latest release"},
		{name: "remove help", args: []string{"remove", "--help"}, code: 0, stdout: "Uninstall"},
		{name: "shell install help", args: []string{"shell-install", "--help"}, code: 0, stdout: "file-manager"},
		{name: "shell uninstall help", args: []string{"shell-uninstall", "--help"}, code: 0, stdout: "file-manager"},
		{name: "shell select help", args: []string{"shell-select", "--help"}, code: 0, stdout: "Windows Explorer"},
		{name: "no arguments", code: 0, stdout: "ayame-diff gui a.txt b.txt"},
		{name: "parse error", args: []string{"--not-a-real-flag"}, code: 2, stderr: "flag provided but not defined"},
		{name: "text missing paths", args: []string{"text", "only-one"}, code: 2, stderr: "needs exactly two paths"},
		{name: "removed interactive mode", args: []string{"--interactive"}, code: 2, stderr: "interactive setup UI was removed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := run(tt.args, &stdout, &stderr); code != tt.code {
				t.Fatalf("code = %d, want %d; stdout=%q stderr=%q", code, tt.code, stdout.String(), stderr.String())
			}
			if tt.stdout != "" && !strings.Contains(stdout.String(), tt.stdout) {
				t.Errorf("stdout %q does not contain %q", stdout.String(), tt.stdout)
			}
			if tt.stdout == "" && stdout.Len() != 0 {
				t.Errorf("stdout = %q, want empty", stdout.String())
			}
			if tt.stderr != "" && !strings.Contains(stderr.String(), tt.stderr) {
				t.Errorf("stderr %q does not contain %q", stderr.String(), tt.stderr)
			}
			if tt.stderr == "" && stderr.Len() != 0 {
				t.Errorf("stderr = %q, want empty", stderr.String())
			}
		})
	}
}

func TestRootUsageListsEverySubcommandOnItsOwnLine(t *testing.T) {
	t.Parallel()
	for command := range subcommandRunners {
		if !strings.Contains(rootUsage, "\n  "+command+" ") {
			t.Errorf("root usage does not list %q on its own line", command)
		}
	}
	for _, example := range []string{
		"ayame-diff gui a.txt b.txt",
		"ayame-diff text a.txt b.txt",
		"ayame-diff dir old-dir new-dir",
	} {
		if !strings.Contains(rootUsage, example) {
			t.Errorf("root usage missing example %q", example)
		}
	}
}

func TestDispatchedSubcommandsMatchBilingualOverviews(t *testing.T) {
	t.Parallel()

	want := make(map[string]struct{}, len(subcommandRunners))
	for command := range subcommandRunners {
		want[command] = struct{}{}
	}

	rootBlock := rootUsage
	if _, after, ok := strings.Cut(rootBlock, "Subcommands:\n"); ok {
		rootBlock = after
	}
	if before, _, ok := strings.Cut(rootBlock, "\n\nRun '"); ok {
		rootBlock = before
	}
	assertCommandSet(t, "top-level help", commandsFromLines(rootBlock, `(?m)^  ([[:alnum:]-]+)(?:\s|$)`), want)

	for _, overview := range []struct {
		name, path, heading string
	}{
		{name: "English README", path: "../../README.md", heading: `(?m)^## Subcommands\s*$`},
		{name: "English compatibility README", path: "../../README.en.md", heading: `(?m)^## Subcommands\s*$`},
		{name: "Japanese README", path: "../../README.ja.md", heading: `(?m)^## サブコマンド\s*$`},
		{name: "English docs home", path: "../../docs/index.md", heading: `(?m)^## Subcommands at a glance\s*$`},
		{name: "Japanese docs home", path: "../../docs/ja/index.md", heading: `(?m)^## サブコマンド一覧\s*$`},
		{name: "English usage", path: "../../docs/usage.md", heading: `(?m)^## Command overview\s*$`},
		{name: "Japanese usage", path: "../../docs/usage.ja.md", heading: `(?m)^## コマンド一覧\s*$`},
	} {
		content := readTestFile(t, overview.path)
		block := firstFencedBlockAfter(t, overview.name, content, overview.heading)
		assertCommandSet(t, overview.name, commandsFromLines(block, `(?m)^ayame-diff\s+([[:alnum:]-]+)(?:\s|$)`), want)
	}
}

func TestBilingualUsageHasDedicatedPreviouslyMissingSections(t *testing.T) {
	t.Parallel()
	for _, path := range []string{"../../docs/usage.md", "../../docs/usage.ja.md"} {
		content := readTestFile(t, path)
		for _, command := range []string{"bin", "update", "remove", "shell-select"} {
			pattern := `(?m)^## ` + regexp.QuoteMeta("`"+command+"`") + `(?:\s|—)`
			if !regexp.MustCompile(pattern).MatchString(content) {
				t.Errorf("%s has no dedicated %q section", path, command)
			}
		}
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

func firstFencedBlockAfter(t *testing.T, name, content, headingPattern string) string {
	t.Helper()
	location := regexp.MustCompile(headingPattern).FindStringIndex(content)
	if location == nil {
		t.Fatalf("%s: overview heading not found", name)
	}
	block := regexp.MustCompile("(?s)```(?:text)?\\n(.*?)\\n```").FindStringSubmatch(content[location[1]:])
	if block == nil {
		t.Fatalf("%s: overview command block not found", name)
	}
	return block[1]
}

func commandsFromLines(content, pattern string) map[string]struct{} {
	commands := make(map[string]struct{})
	for _, match := range regexp.MustCompile(pattern).FindAllStringSubmatch(content, -1) {
		commands[match[1]] = struct{}{}
	}
	return commands
}

func assertCommandSet(t *testing.T, name string, got, want map[string]struct{}) {
	t.Helper()
	for command := range want {
		if _, ok := got[command]; !ok {
			t.Errorf("%s is missing dispatched subcommand %q", name, command)
		}
	}
	for command := range got {
		if _, ok := want[command]; !ok {
			t.Errorf("%s documents non-dispatched subcommand %q", name, command)
		}
	}
}

func TestQuickLaunchArgs(t *testing.T) {
	tests := []struct {
		args  []string
		paths []string
		gui   bool
		ok    bool
	}{
		{[]string{"old.txt", "new.txt"}, []string{"old.txt", "new.txt"}, false, true},
		{[]string{"--gui", "old.txt", "new.txt"}, []string{"old.txt", "new.txt"}, true, true},
		{[]string{"old.txt", "--gui"}, []string{"old.txt"}, true, true},
		{[]string{"--gui", "--", "-old.txt", "-new.txt"}, []string{"-old.txt", "-new.txt"}, true, true},
		{[]string{"text", "old.txt", "new.txt"}, nil, false, false},
		{[]string{"--left", "old.csv"}, nil, false, false},
	}
	for _, tt := range tests {
		paths, gui, ok := quickLaunchArgs(tt.args)
		if !reflect.DeepEqual(paths, tt.paths) || gui != tt.gui || ok != tt.ok {
			t.Errorf("quickLaunchArgs(%q) = %q, %v, %v", tt.args, paths, gui, ok)
		}
	}
}

func TestGUIQuickLaunchURL(t *testing.T) {
	oldDir, newDir := t.TempDir(), t.TempDir()
	got := guiLaunchURL("http://127.0.0.1:1/", []string{oldDir, newDir}, "test-token")
	if !strings.Contains(got, "autorun=1") || !strings.Contains(got, "mode=dir") || !strings.Contains(got, "old=") || !strings.Contains(got, "new=") {
		t.Fatalf("url=%s", got)
	}
}

func TestRunCSVMapsCancellationAndDiffExitCodes(t *testing.T) {
	original := runEngine
	t.Cleanup(func() { runEngine = original })
	args := []string{"--left", "left.csv", "--right", "right.csv", "--out", "diff.tsv"}

	runEngine = func(context.Context, engine.Config) (engine.Summary, error) {
		return engine.Summary{}, context.Canceled
	}
	var stdout, stderr bytes.Buffer
	if code := run(args, &stdout, &stderr); code != 130 {
		t.Fatalf("canceled code = %d, want 130", code)
	}
	if !strings.Contains(stderr.String(), "context canceled") {
		t.Fatalf("canceled stderr = %q", stderr.String())
	}

	runEngine = func(context.Context, engine.Config) (engine.Summary, error) {
		return engine.Summary{DiffRows: 1}, nil
	}
	stdout.Reset()
	stderr.Reset()
	if code := run(append(args, "--diff-exit-code"), &stdout, &stderr); code != 1 {
		t.Fatalf("diff code = %d, want 1", code)
	}
}

func TestParseFlagsExclusionOptions(t *testing.T) {
	t.Parallel()
	opts, err := parseFlags([]string{
		"--left", "a.csv", "--right", "b.csv", "--out", "d.tsv",
		"--exclude-key", "updated_at", "--exclude-key", "checksum",
		"--exclude-key-index", "4", "--index-base", "1",
	})
	if err != nil {
		t.Fatal(err)
	}
	cfg := opts.Engine
	if !reflect.DeepEqual(cfg.ExcludeKeyNames, []string{"updated_at", "checksum"}) {
		t.Fatalf("exclude names = %#v", cfg.ExcludeKeyNames)
	}
	if !reflect.DeepEqual(cfg.ExcludeKeyIndexes, []int{4}) {
		t.Fatalf("exclude indexes = %#v", cfg.ExcludeKeyIndexes)
	}
}

func TestParseFlagsKeepsCLIOptionsOutsideEngineConfig(t *testing.T) {
	t.Parallel()
	opts, err := parseFlags([]string{
		"--left", "a.tsv", "--right", "b.tsv", "--out", "d.tsv",
		"--summary-json", "summary.json", "--diff-exit-code",
	})
	if err != nil {
		t.Fatal(err)
	}
	if opts.SummaryJSON != "summary.json" || !opts.DiffExitCode {
		t.Fatalf("CLI options = %#v", opts)
	}
}

func TestParseFlagsCSVComparisonOptions(t *testing.T) {
	t.Parallel()
	opts, err := parseFlags([]string{
		"--left", "a.csv", "--right", "b.csv", "--out", "d.tsv",
		"--ignore-case", "--ignore-space-change", "--ignore-eol", "--ignore-trailing-eol",
		"--filter-line", `time=\d+`, "--ignore-column", "updated", "--ignore-column-index", "3",
		"--tolerance", "0.001", "--column-tolerance", "price=0.01", "--column-tolerance-index", "4=0.1",
	})
	if err != nil {
		t.Fatal(err)
	}
	cfg := opts.Engine
	if !cfg.IgnoreCase || cfg.IgnoreWhitespace != "change" || !cfg.IgnoreEOL || !cfg.IgnoreTrailingEOL ||
		!cfg.ToleranceSet || cfg.Tolerance != 0.001 || len(cfg.ColumnTolerances) != 2 ||
		!reflect.DeepEqual(cfg.IgnoreColumnNames, []string{"updated"}) || !reflect.DeepEqual(cfg.IgnoreColumnIndexes, []int{3}) {
		t.Fatalf("config=%+v", cfg)
	}
}

func TestParseFlagsCellDiffJSON(t *testing.T) {
	t.Parallel()
	opts, err := parseFlags([]string{"--left", "a.csv", "--right", "b.csv", "--out", "diff.jsonl", "--json"})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.JSON || !opts.Engine.CellDiff || opts.Engine.OutputFormat != "jsonl" {
		t.Fatalf("options=%+v", opts)
	}
}

func TestCLIHelpUsesLeftRightTerminology(t *testing.T) {
	t.Parallel()
	if strings.Contains(rootUsage, "OLD") || strings.Contains(rootUsage, "NEW") {
		t.Fatalf("root usage still exposes legacy side labels:\n%s", rootUsage)
	}
	if !strings.Contains(rootUsage, "ayame-diff LEFT RIGHT") {
		t.Fatal("root usage does not identify positional inputs as LEFT and RIGHT")
	}

	commands := []struct {
		name string
		run  func([]string, io.Writer, io.Writer) int
	}{
		{"text", runText},
		{"sorted", runSorted},
		{"dir", runDir},
		{"bin", runBin},
		{"gui", runGUI},
	}
	for _, command := range commands {
		command := command
		t.Run(command.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := command.run([]string{"--help"}, &stdout, &stderr); code != exitOK {
				t.Fatalf("help exit code = %d, stderr = %q", code, stderr.String())
			}
			output := stdout.String() + stderr.String()
			if strings.Contains(output, "OLD") || strings.Contains(output, "NEW") {
				t.Fatalf("help still exposes legacy side labels:\n%s", output)
			}
			if !strings.Contains(output, "LEFT") || !strings.Contains(output, "RIGHT") {
				t.Fatalf("help does not identify both sides:\n%s", output)
			}
		})
	}

	var points syncFlag
	if err := points.Set("invalid"); err == nil || !strings.Contains(err.Error(), "LEFT:RIGHT") {
		t.Fatalf("sync validation error = %v", err)
	}
}

func TestRunCSVLoadsAndSavesProject(t *testing.T) {
	original := runEngine
	t.Cleanup(func() { runEngine = original })
	dir := t.TempDir()
	projectPath := filepath.Join(dir, "daily.ayamediff.json")
	cfg := validCLIConfig(filepath.Join(dir, "old.csv"), filepath.Join(dir, "new.csv"), filepath.Join(dir, "diff.tsv"))
	cfg.KeyNames = []string{"id"}
	if err := project.Save(projectPath, project.Project{Mode: "csv", CSV: cfg}); err != nil {
		t.Fatal(err)
	}
	var captured engine.Config
	runEngine = func(_ context.Context, got engine.Config) (engine.Summary, error) {
		captured = got
		return engine.Summary{}, nil
	}
	var stdout, stderr bytes.Buffer
	if code := runCSV([]string{"--project", projectPath}, &stdout, &stderr); code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	if captured.LeftPath != cfg.LeftPath || !reflect.DeepEqual(captured.KeyNames, []string{"id"}) {
		t.Fatalf("captured=%+v", captured)
	}

	saved := filepath.Join(dir, "saved.ayamediff.json")
	args := []string{"--left", cfg.LeftPath, "--right", cfg.RightPath, "--out", cfg.OutputPath, "--partitions", "2", "--workers", "1", "--parse-workers", "1", "--memory", "64MiB", "--partition-buffer", "4KiB", "--merge-fan-in", "2", "--max-record-bytes", "1MiB", "--save-project", saved}
	stderr.Reset()
	if code := runCSV(args, &stdout, &stderr); code != 0 {
		t.Fatalf("save code=%d stderr=%q", code, stderr.String())
	}
	if _, err := project.Load(saved); err != nil {
		t.Fatal(err)
	}
}

func validCLIConfig(left, right, out string) engine.Config {
	return engine.Config{LeftPath: left, RightPath: right, OutputPath: out, HasHeader: true, AlignColumnsByName: true,
		LeftFormat: "auto", RightFormat: "auto", LeftParser: "auto", RightParser: "auto", Partitions: 2,
		ParseWorkers: 1, Workers: 1, MemoryText: "64MiB", PartitionBufferText: "4KiB", MergeFanIn: 2, MaxRecordText: "1MiB"}
}
