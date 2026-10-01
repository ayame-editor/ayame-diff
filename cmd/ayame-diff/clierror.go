package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/ayame-editor/ayame-diff/internal/climsg"
)

// debugEnv keeps the raw failure text available behind the localized message.
// It is undocumented in --help on purpose: it is for bug reports, not daily
// use (#94).
const debugEnv = "AYAME_DIFF_DEBUG"

func messageLanguage() climsg.Language { return climsg.DetectEnv() }

func debugEnabled() bool { return os.Getenv(debugEnv) != "" }

// reportError prints err as a localized problem and a one-line remedy. The raw
// error text is appended only when AYAME_DIFF_DEBUG is set, so a syscall or
// strconv string never becomes the user-facing top line (#94).
func reportError(stderr io.Writer, err error) {
	message := climsg.Explain(messageLanguage(), err)
	fmt.Fprintln(stderr, "error:", message.Problem)
	if message.Remedy != "" {
		fmt.Fprintln(stderr, "  hint:", message.Remedy)
	}
	if debugEnabled() {
		fmt.Fprintf(stderr, "  (debug: %v)\n", err)
	}
}

// parseFlagsOrExit parses a command's flags while capturing the flag package's
// own output. On --help the captured usage is forwarded to stdout. Otherwise
// the explained message is printed first — never the flag package's raw value
// line — the usage follows, and the raw output is kept only under
// AYAME_DIFF_DEBUG. The bool reports whether parsing stopped the command.
func parseFlagsOrExit(fs *flag.FlagSet, args []string, stdout, stderr io.Writer) (int, bool) {
	var captured bytes.Buffer
	fs.SetOutput(&captured)
	if err := fs.Parse(args); err != nil {
		return finishFlagParse(err, &captured, stdout, stderr), true
	}
	return exitOK, false
}

// parseDiffFlags is parseFlagsOrExit plus the two-positional-path check shared
// by the text and sorted commands.
func parseDiffFlags(fs *flag.FlagSet, args []string, stdout, stderr io.Writer) (int, bool) {
	var captured bytes.Buffer
	fs.SetOutput(&captured)
	if err := fs.Parse(args); err != nil {
		return finishFlagParse(err, &captured, stdout, stderr), true
	}
	if fs.NArg() != 2 {
		reportError(stderr, fmt.Errorf("%s needs exactly two paths: LEFT RIGHT", fs.Name()))
		return exitUsage, true
	}
	return exitOK, false
}

// parseCSVFlags parses the CSV mode's flags, which return options on success.
func parseCSVFlags(args []string, stdout, stderr io.Writer) (cliOptions, int, bool) {
	var captured bytes.Buffer
	opts, err := parseFlags(args, &captured)
	if err == nil {
		return opts, exitOK, false
	}
	return opts, finishFlagParse(err, &captured, stdout, stderr), true
}

// finishFlagParse turns a failed parse into an exit code: help is success and
// every other failure is a usage error.
func finishFlagParse(err error, captured *bytes.Buffer, stdout, stderr io.Writer) int {
	if errors.Is(err, flag.ErrHelp) {
		_, _ = io.Copy(stdout, captured)
		return exitOK
	}
	reportError(stderr, err)
	if debugEnabled() {
		_, _ = io.Copy(stderr, captured)
	} else {
		writeAfterFirstLine(stderr, captured.Bytes())
	}
	return exitUsage
}

// writeAfterFirstLine forwards captured output minus its first line, which is
// the flag package's raw error. The usage text that follows is still useful.
func writeAfterFirstLine(w io.Writer, captured []byte) {
	if index := bytes.IndexByte(captured, '\n'); index >= 0 {
		_, _ = w.Write(captured[index+1:])
	}
}
