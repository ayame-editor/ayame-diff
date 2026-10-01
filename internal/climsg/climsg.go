// Package climsg turns CLI failures into a short, localized explanation and a
// one-line remedy (#94). A raw syscall, strconv, or encoding/json string never
// becomes the user-facing top line; it stays available behind AYAME_DIFF_DEBUG.
//
// The package is pure and standard-library only, so it can be tested without a
// terminal or a running command.
package climsg

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"strconv"
	"strings"
)

// Language identifies the language of a user-facing message.
type Language int

const (
	// English is the default language.
	English Language = iota
	// Japanese is selected by a Japanese locale.
	Japanese
)

// String reports the language as a short tag: "en" or "ja".
func (l Language) String() string {
	if l == Japanese {
		return "ja"
	}
	return "en"
}

// localeVars are the environment variables that select the message language,
// consulted in this order.
var localeVars = []string{"LC_ALL", "LC_MESSAGES", "LANG"}

// Detect picks the message language from locale environment variables, in the
// order LC_ALL, LC_MESSAGES, LANG. The first non-empty value decides: one that
// begins with "ja" selects Japanese, and anything else selects English. An
// unset locale selects English.
func Detect(getenv func(string) string) Language {
	for _, name := range localeVars {
		value := strings.ToLower(strings.TrimSpace(getenv(name)))
		if value == "" {
			continue
		}
		if strings.HasPrefix(value, "ja") {
			return Japanese
		}
		return English
	}
	return English
}

// DetectEnv is Detect over os.Getenv.
func DetectEnv() Language { return Detect(os.Getenv) }

// Message is one user-facing failure: what went wrong, and what to do next.
// Remedy is empty when the failure has no obvious next step.
type Message struct {
	Problem string
	Remedy  string
}

// ErrOutputIsInput marks a write whose output path is also one of the inputs.
// The CLI maps it to a localized explanation instead of internal wording.
var ErrOutputIsInput = errors.New("output path is also an input")

// ValueKind describes the form a malformed command-line value should have had.
type ValueKind int

const (
	// ValueOther is a value this package does not classify further.
	ValueOther ValueKind = iota
	// ValueInteger wants a whole number.
	ValueInteger
	// ValueNumber wants a finite decimal number.
	ValueNumber
)

// ValueError marks a command-line value that could not be parsed. Err keeps the
// original parse failure so debug output can still show it.
type ValueError struct {
	Flag  string
	Value string
	Kind  ValueKind
	Err   error
}

func (e *ValueError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return "invalid value " + strconv.Quote(e.Value)
}

// Unwrap exposes the original parse failure.
func (e *ValueError) Unwrap() error { return e.Err }

// Explain returns the user-facing message for err in lang. An error this
// package does not recognize keeps its own text and carries no remedy, so an
// unfamiliar failure is still reported rather than replaced by a wrong guess.
func Explain(lang Language, err error) Message {
	if err == nil {
		return Message{}
	}
	var valueErr *ValueError
	if errors.As(err, &valueErr) {
		return valueMessage(lang, valueErr.Flag, valueErr.Value, valueErr.Kind, "")
	}
	if flag, value, inner, ok := splitFlagError(err); ok {
		return flagValueMessage(lang, flag, value, inner)
	}
	if errors.Is(err, ErrOutputIsInput) {
		return Message{
			Problem: pick(lang, "The output is one of the compared inputs.", "出力先が比較対象のファイルと同じです。"),
			Remedy:  pick(lang, "Choose an output path different from every input.", "入力とは別の出力先を指定してください。"),
		}
	}
	if isJSONError(err) {
		return Message{
			Problem: pick(lang, "The request could not be read.", "リクエストを解釈できませんでした。"),
			Remedy:  pick(lang, "Make sure the JSON is well formed, then try again.", "JSONの書式を確認してから、もう一度実行してください。"),
		}
	}
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, os.ErrNotExist) {
		return Message{
			Problem: pick(lang, "The file was not found.", "ファイルが見つかりません。"),
			Remedy: pick(lang,
				"Check the path. In the GUI, pick the file again with the … button (--gui).",
				"パスを確認してください。GUIでは「…」ボタンでファイルを選び直せます（--gui）。"),
		}
	}
	if errors.Is(err, fs.ErrPermission) || errors.Is(err, os.ErrPermission) {
		return Message{
			Problem: pick(lang, "Access was denied.", "アクセスが拒否されました。"),
			Remedy: pick(lang,
				"Check the file's permissions, or choose a file you can read.",
				"ファイルの権限を確認するか、読み取れるファイルを指定してください。"),
		}
	}
	return Message{Problem: err.Error()}
}

func pick(lang Language, english, japanese string) string {
	if lang == Japanese {
		return japanese
	}
	return english
}

// valueMessage formats a malformed value for a known flag and kind. inner is
// the parser's own explanation, used as a remedy when it is safe to show.
func valueMessage(lang Language, flag, value string, kind ValueKind, inner string) Message {
	switch kind {
	case ValueInteger:
		return integerValueMessage(lang, flag)
	case ValueNumber:
		return numberValueMessage(lang, flag)
	default:
		return genericValueMessage(lang, flag, value, inner)
	}
}

func integerValueMessage(lang Language, flag string) Message {
	name := flagLabel(flag)
	if lang == Japanese {
		return Message{
			Problem: name + "には整数を指定してください。",
			Remedy:  "例: " + flagExample(flag, "2") + " のように、整数を入力してください。",
		}
	}
	return Message{
		Problem: name + " is not a whole number.",
		Remedy:  "Give a whole number, for example " + flagExample(flag, "2") + ".",
	}
}

func numberValueMessage(lang Language, flag string) Message {
	name := flagLabel(flag)
	if lang == Japanese {
		return Message{
			Problem: name + "には数値を指定してください。",
			Remedy:  "例: " + flagExample(flag, "0.01") + " のように、有限の非負の数値を入力してください。",
		}
	}
	return Message{
		Problem: name + " is not a valid number.",
		Remedy:  "Give a finite non-negative number, for example " + flagExample(flag, "0.01") + ".",
	}
}

func genericValueMessage(lang Language, flag, value, inner string) Message {
	name := flagLabel(flag)
	quoted := strconv.Quote(value)
	if lang == Japanese {
		message := Message{
			Problem: name + " の値 " + quoted + " が正しくありません。",
			Remedy:  "受け付けられる書式を --help で確認してください。",
		}
		if usableRemedy(inner) {
			message.Remedy = inner
		}
		return message
	}
	message := Message{
		Problem: "The value " + quoted + " for " + name + " is not valid.",
		Remedy:  "See --help for the accepted form.",
	}
	if usableRemedy(inner) {
		message.Remedy = inner
	}
	return message
}

// flagValueMessage classifies the message the flag package wrapped around a
// malformed value. The flag name is available here, so the explanation can name
// it and the accepted form without repeating strconv's wording.
func flagValueMessage(lang Language, flag, value, inner string) Message {
	switch {
	case strings.Contains(inner, "invalid integer"), strings.Contains(inner, "strconv"):
		return integerValueMessage(lang, flag)
	case strings.Contains(inner, "tolerance"), strings.Contains(inner, "non-negative number"):
		return numberValueMessage(lang, flag)
	default:
		return genericValueMessage(lang, flag, value, inner)
	}
}

func flagLabel(flag string) string {
	if flag == "" {
		return "the value"
	}
	return "--" + flag
}

func flagExample(flag, value string) string {
	if flag == "" {
		return value
	}
	return "--" + flag + " " + value
}

// usableRemedy reports whether a parser's own explanation is safe to show; a
// message that carries strconv wording or only a bare parse failure is not.
func usableRemedy(inner string) bool {
	inner = strings.TrimSpace(inner)
	if inner == "" || strings.Contains(inner, "strconv") {
		return false
	}
	switch inner {
	case "parse error", "invalid syntax":
		return false
	}
	return true
}

// isJSONError reports whether err is an encoding/json parse failure, whether
// direct or wrapped by the caller (for example "decode project: ...").
func isJSONError(err error) bool {
	var syntax *json.SyntaxError
	var typeMismatch *json.UnmarshalTypeError
	return errors.As(err, &syntax) || errors.As(err, &typeMismatch)
}

// splitFlagError recognizes the flag package's error for a value it could not
// parse: invalid value "x" for flag -name: <inner>. It returns the flag name,
// the offending value, and the inner message. ok is false for any other error.
func splitFlagError(err error) (flag, value, inner string, ok bool) {
	const prefix = "invalid value "
	text := err.Error()
	if !strings.HasPrefix(text, prefix) {
		return "", "", "", false
	}
	rest := text[len(prefix):]
	if !strings.HasPrefix(rest, `"`) {
		return "", "", "", false
	}
	end := closingQuote(rest)
	if end < 0 {
		return "", "", "", false
	}
	if unquoted, unquoteErr := strconv.Unquote(rest[:end+1]); unquoteErr == nil {
		value = unquoted
	} else {
		value = rest[:end+1]
	}
	tail := rest[end+1:]
	const mid = " for flag -"
	if !strings.HasPrefix(tail, mid) {
		return "", "", "", false
	}
	tail = tail[len(mid):]
	colon := strings.Index(tail, ":")
	if colon < 0 {
		return "", "", "", false
	}
	return tail[:colon], value, strings.TrimSpace(tail[colon+1:]), true
}

// closingQuote returns the index of the quote that ends the Go-quoted string
// beginning at s[0], or -1 if there is none.
func closingQuote(s string) int {
	for i := 1; i < len(s); i++ {
		if s[i] == '\\' {
			i++
			continue
		}
		if s[i] == '"' {
			return i
		}
	}
	return -1
}
