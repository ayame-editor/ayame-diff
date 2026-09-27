package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"net/url"
	"sort"
	"sync"

	"github.com/ayame-editor/ayame-diff/internal/pathutil"
)

// difftool and mergetool exist so a version-control system or an IDE can invoke
// ayame-diff as the comparison or merge program (ADR 0004). ayame-diff still
// only sees explicit paths; it does not read a repository. Git supplies
// temporary files and logical names that mean nothing on disk, so these
// commands add labels, git's positional order, a blocking GUI lifetime, and an
// exit code that says whether a merge actually resolved.

// toolFlags are the options shared by difftool and mergetool.
type toolFlags struct {
	labels      repeatedFlag[string]
	gui         bool
	wait        bool
	noOpen      bool
	addr        string
	allowRemote bool
}

func (f *toolFlags) register(fs *flag.FlagSet) {
	f.labels = stringFlags()
	fs.Var(&f.labels, "label", "pane display name, in argument order; repeatable")
	fs.Var(&f.labels, "L", "alias for --label")
	fs.BoolVar(&f.gui, "gui", false, "open the comparison in the browser GUI")
	fs.BoolVar(&f.wait, "wait", false, "block until the GUI tab closes (implies --gui)")
	fs.BoolVar(&f.noOpen, "no-open", false, "with --gui, serve but do not launch a browser")
	fs.StringVar(&f.addr, "addr", "127.0.0.1:0", "GUI listen address; port 0 picks a free port")
	fs.BoolVar(&f.allowRemote, "allow-remote", false, "allow a non-loopback GUI listen address (unsafe)")
}

// browser reports whether this invocation should open the GUI. A blocking GUI
// is the point of --wait, so it implies --gui rather than erroring.
func (f toolFlags) browser() bool { return f.gui || f.wait }

// label returns the display name for the positional argument at i, or "".
func (f toolFlags) label(i int) string {
	if i < len(f.labels.values) {
		return f.labels.values[i]
	}
	return ""
}

// sideName is the logical label at i when one was given, else the path.
func (f toolFlags) sideName(i int, path string) string {
	if label := f.label(i); label != "" {
		return label
	}
	return path
}

// labelArgs renders the labels as --label flags for a delegated runText call.
func (f toolFlags) labelArgs() []string {
	out := make([]string, 0, len(f.labels.values)*2)
	for _, label := range f.labels.values {
		out = append(out, "--label", label)
	}
	return out
}

func (f toolFlags) validateLabelCount(stderr io.Writer, command string, max int) bool {
	if len(f.labels.values) > max {
		fmt.Fprintf(stderr, "error: %s accepts at most %d --label values\n", command, max)
		return false
	}
	return true
}

// runDifftool implements: ayame-diff difftool [--gui|--wait] [--label NAME]... LEFT RIGHT
func runDifftool(args []string, stdout, stderr io.Writer) int {
	return runDifftoolWithDeps(args, stdout, stderr, defaultToolGUIDeps())
}

func runDifftoolWithDeps(args []string, stdout, stderr io.Writer, deps guiCommandDeps) int {
	fs := flag.NewFlagSet("ayame-diff difftool", flag.ContinueOnError)
	fs.SetOutput(flagOutput(args, stdout, stderr))
	var tool toolFlags
	tool.register(fs)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), `ayame-diff difftool [--gui|--wait] [--label NAME]... LEFT RIGHT

Compare two files as a version-control difftool. With --gui (or --wait) the
browser GUI opens and this command blocks until its tab closes, so a calling
`+"`git difftool`"+` waits correctly. --label names each side logically, e.g.
-L 'HEAD~1:foo.txt' -L ':foo.txt', instead of the temporary paths Git passes.`)
		fmt.Fprintln(fs.Output(), "\nOptions:")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return reportFlagError(err, stderr)
	}
	if fs.NArg() != 2 {
		fmt.Fprintln(stderr, "error: difftool needs exactly two paths: LEFT RIGHT")
		return exitUsage
	}
	if !tool.validateLabelCount(stderr, "difftool", 2) {
		return exitUsage
	}
	left, right := fs.Arg(0), fs.Arg(1)
	if tool.browser() {
		return serveGUISession(guiSessionRequest{
			addr: tool.addr, noOpen: tool.noOpen, allowRemote: tool.allowRemote,
			launchURL: func(baseURL, token string) string {
				return toolLaunchURL(baseURL, token, toolLaunch{
					mode: "text", left: left, right: right,
					leftLabel: tool.label(0), rightLabel: tool.label(1),
				})
			},
		}, deps, stderr)
	}
	if len(tool.labels.values) > 0 {
		fmt.Fprintf(stderr, "difftool: %s vs %s\n", tool.sideName(0, left), tool.sideName(1, right))
	}
	return runText(append(tool.labelArgs(), left, right), stdout, stderr)
}

// runMergetool implements:
// ayame-diff mergetool [--gui|--wait] --output MERGED [--label NAME]... BASE LOCAL REMOTE
func runMergetool(args []string, stdout, stderr io.Writer) int {
	return runMergetoolWithDeps(args, stdout, stderr, defaultToolGUIDeps())
}

func runMergetoolWithDeps(args []string, stdout, stderr io.Writer, deps guiCommandDeps) int {
	fs := flag.NewFlagSet("ayame-diff mergetool", flag.ContinueOnError)
	fs.SetOutput(flagOutput(args, stdout, stderr))
	var tool toolFlags
	tool.register(fs)
	var output, order string
	fs.StringVar(&output, "output", "", "write the merge result here (Git's $MERGED)")
	fs.StringVar(&output, "merged", "", "alias for --output")
	fs.StringVar(&order, "order", "base-local-remote", "positional order: base-local-remote (P4Merge/Git) or local-base-remote (Meld)")
	choices := conflictChoices{}
	fs.Var(choices, "choice", "resolve conflict EVENT=left|right|base; repeatable")
	var allowConflicts bool
	fs.BoolVar(&allowConflicts, "allow-conflicts", true, "write standard conflict markers when conflicts remain")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), `ayame-diff mergetool [--gui|--wait] --output MERGED [--label NAME]... BASE LOCAL REMOTE

Merge two edits of a common base as a version-control mergetool. The exit code
is 0 only when MERGED is written with no unresolved conflicts, and 1 when
conflict markers remain or the GUI session ended without a clean save; a caller
therefore never mistakes "saved" for "resolved". --order accepts Git/P4Merge's
BASE LOCAL REMOTE (the default) or Meld's LOCAL BASE REMOTE. --label names each
side logically.`)
		fmt.Fprintln(fs.Output(), "\nOptions:")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return reportFlagError(err, stderr)
	}
	if fs.NArg() != 3 {
		fmt.Fprintln(stderr, "error: mergetool needs three paths: BASE LOCAL REMOTE")
		return exitUsage
	}
	if !tool.validateLabelCount(stderr, "mergetool", 3) {
		return exitUsage
	}
	if output == "" {
		fmt.Fprintln(stderr, "error: mergetool requires --output MERGED")
		return exitUsage
	}
	var base, local, remote string
	switch order {
	case "base-local-remote":
		base, local, remote = fs.Arg(0), fs.Arg(1), fs.Arg(2)
	case "local-base-remote":
		base, local, remote = fs.Arg(1), fs.Arg(0), fs.Arg(2)
	default:
		fmt.Fprintln(stderr, "error: --order must be base-local-remote or local-base-remote")
		return exitUsage
	}

	if tool.browser() {
		var (
			mu              sync.Mutex
			savedOutput     string
			savedUnresolved = -1
		)
		code := serveGUISession(guiSessionRequest{
			addr: tool.addr, noOpen: tool.noOpen, allowRemote: tool.allowRemote,
			mergeOutcome: func(saved string, unresolved int) {
				mu.Lock()
				savedOutput, savedUnresolved = saved, unresolved
				mu.Unlock()
			},
			launchURL: func(baseURL, token string) string {
				return toolLaunchURL(baseURL, token, toolLaunch{
					mode: "threeway", base: base, left: local, right: remote, output: output,
					baseLabel: tool.label(0), leftLabel: tool.label(1), rightLabel: tool.label(2),
				})
			},
		}, deps, stderr)
		if code != exitOK {
			return code
		}
		mu.Lock()
		saved, unresolved := savedOutput, savedUnresolved
		mu.Unlock()
		return mergeToolExitCode(saved, unresolved, output, stderr)
	}

	threeArgs := []string{"text", "--output", output, "--merge-exit-code"}
	if allowConflicts {
		threeArgs = append(threeArgs, "--allow-conflicts")
	}
	ids := make([]int, 0, len(choices))
	for id := range choices {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	for _, id := range ids {
		threeArgs = append(threeArgs, "--choice", fmt.Sprintf("%d=%s", id, choices[id]))
	}
	threeArgs = append(threeArgs, base, local, remote)
	return runThreeWay(threeArgs, stdout, stderr)
}

// mergeToolExitCode turns one GUI session's observed merge outcome into the
// process exit code. A negative unresolved count means nothing was saved, so
// the session was aborted and the path must stay unresolved (130, the same
// "explicitly cancelled" code as declining `remove`). A save to a different
// path or one with unresolved conflicts is a nonzero failure. Only a save to
// the requested path with no unresolved conflicts returns 0.
func mergeToolExitCode(saved string, unresolved int, expected string, stderr io.Writer) int {
	switch {
	case unresolved < 0:
		fmt.Fprintln(stderr, "merge was not saved; leaving the path unresolved")
		return exitInterrupt
	case !pathutil.Equal(saved, expected):
		fmt.Fprintf(stderr, "merge was saved to %s, not %s; leaving the path unresolved\n", saved, expected)
		return exitDiff
	case unresolved > 0:
		fmt.Fprintf(stderr, "merge was saved with %d unresolved conflict(s)\n", unresolved)
		return exitDiff
	default:
		return exitOK
	}
}

// defaultToolGUIDeps wires the real browser session for difftool/mergetool.
func defaultToolGUIDeps() guiCommandDeps {
	return guiCommandDeps{
		newHandler:  newServerHandler,
		listen:      net.Listen,
		serve:       serveUntilShutdown,
		openBrowser: openBrowser,
	}
}

// toolLaunch describes the GUI comparison a difftool/mergetool invocation opens.
type toolLaunch struct {
	mode                             string // "text" or "threeway"
	base, left, right, output        string
	baseLabel, leftLabel, rightLabel string
}

// toolLaunchURL builds the launch URL for a tool session. Unlike the
// file-manager launcher it can name a BASE, prefill the merge output, and carry
// the logical labels so a temporary path is not what the panes show.
func toolLaunchURL(baseURL, token string, spec toolLaunch) string {
	query := url.Values{}
	if token != "" {
		query.Set("token", token)
	}
	if spec.base != "" {
		query.Set("base", spec.base)
	}
	query.Set("old", spec.left)
	query.Set("new", spec.right)
	query.Set("mode", spec.mode)
	query.Set("autorun", "1")
	if spec.output != "" {
		query.Set("output", spec.output)
	}
	if spec.baseLabel != "" {
		query.Set("baseLabel", spec.baseLabel)
	}
	if spec.leftLabel != "" {
		query.Set("oldLabel", spec.leftLabel)
	}
	if spec.rightLabel != "" {
		query.Set("newLabel", spec.rightLabel)
	}
	return baseURL + "?" + query.Encode()
}
