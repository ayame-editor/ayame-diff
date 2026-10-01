package climsg

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"testing"
)

func TestDetectUsesLocaleOrder(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		env  map[string]string
		want Language
	}{
		{"unset is english", map[string]string{}, English},
		{"ja_JP selects japanese", map[string]string{"LANG": "ja_JP.UTF-8"}, Japanese},
		{"bare ja selects japanese", map[string]string{"LANG": "ja"}, Japanese},
		{"case insensitive", map[string]string{"LANG": "JA_JP"}, Japanese},
		{"non-ja selects english", map[string]string{"LANG": "en_US.UTF-8"}, English},
		{"LC_ALL wins over LANG", map[string]string{"LC_ALL": "en_US.UTF-8", "LANG": "ja_JP.UTF-8"}, English},
		{"LC_MESSAGES used when LC_ALL empty", map[string]string{"LC_ALL": "", "LC_MESSAGES": "ja_JP.UTF-8", "LANG": "en_US.UTF-8"}, Japanese},
		{"empty values fall through", map[string]string{"LC_ALL": "", "LC_MESSAGES": "", "LANG": "ja_JP.UTF-8"}, Japanese},
		{"C locale is english", map[string]string{"LC_ALL": "C", "LANG": "ja_JP.UTF-8"}, English},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Detect(func(name string) string { return tt.env[name] })
			if got != tt.want {
				t.Fatalf("Detect(%v) = %v, want %v", tt.env, got, tt.want)
			}
		})
	}
}

func TestExplainFileNotFound(t *testing.T) {
	t.Parallel()
	err := &fs.PathError{Op: "open", Path: "missing.txt", Err: fs.ErrNotExist}
	for _, lang := range []Language{English, Japanese} {
		message := Explain(lang, err)
		if message.Problem == "" || message.Remedy == "" {
			t.Fatalf("%s: message = %+v, want problem and remedy", lang, message)
		}
		assertNoRawWording(t, message)
	}
	if got := Explain(English, err).Problem; got != "The file was not found." {
		t.Fatalf("english problem = %q", got)
	}
	if got := Explain(Japanese, err).Problem; !strings.Contains(got, "ファイル") {
		t.Fatalf("japanese problem = %q", got)
	}
}

func TestExplainPermissionDenied(t *testing.T) {
	t.Parallel()
	err := &fs.PathError{Op: "open", Path: "secret.txt", Err: fs.ErrPermission}
	for _, lang := range []Language{English, Japanese} {
		message := Explain(lang, err)
		if message.Problem == "" || message.Remedy == "" {
			t.Fatalf("%s: message = %+v, want problem and remedy", lang, message)
		}
		assertNoRawWording(t, message)
	}
}

func TestExplainValueErrorNamesFlagAndForm(t *testing.T) {
	t.Parallel()
	err := &ValueError{Flag: "key-index", Value: "x", Kind: ValueInteger, Err: errors.New("strconv.Atoi: parsing \"x\": invalid syntax")}
	for _, lang := range []Language{English, Japanese} {
		message := Explain(lang, err)
		if !strings.Contains(message.Problem, "--key-index") {
			t.Fatalf("%s: problem %q does not name the flag", lang, message.Problem)
		}
		if message.Remedy == "" {
			t.Fatalf("%s: missing remedy", lang)
		}
		assertNoRawWording(t, message)
	}
}

func TestExplainFlagWrappedValueError(t *testing.T) {
	t.Parallel()
	// The flag package formats a Set error with %v, dropping its type, so the
	// wrapper is all the classifier sees.
	err := errors.New(`invalid value "x" for flag -key-index: invalid integer "x": strconv.Atoi: parsing "x": invalid syntax`)
	for _, lang := range []Language{English, Japanese} {
		message := Explain(lang, err)
		if !strings.Contains(message.Problem, "--key-index") {
			t.Fatalf("%s: problem %q does not name the flag", lang, message.Problem)
		}
		assertNoRawWording(t, message)
	}
}

func TestExplainFlagWrappedNumber(t *testing.T) {
	t.Parallel()
	err := errors.New(`invalid value "abc" for flag -tolerance: tolerance must be a finite non-negative number`)
	message := Explain(English, err)
	if !strings.Contains(message.Problem, "--tolerance") {
		t.Fatalf("problem %q does not name the flag", message.Problem)
	}
	assertNoRawWording(t, message)
}

func TestExplainMalformedJSON(t *testing.T) {
	t.Parallel()
	var raw json.RawMessage
	parseErr := json.Unmarshal([]byte("{bad"), &raw)
	err := fmt.Errorf("decode project: %w", parseErr)
	if !isJSONError(err) {
		t.Fatalf("wrapped json error not recognized: %v", err)
	}
	for _, lang := range []Language{English, Japanese} {
		message := Explain(lang, err)
		if message.Problem == "" || message.Remedy == "" {
			t.Fatalf("%s: message = %+v", lang, message)
		}
		assertNoRawWording(t, message)
	}
	if !strings.Contains(Explain(English, err).Problem, "request") {
		t.Fatalf("json problem = %q", Explain(English, err).Problem)
	}
}

func TestExplainOutputIsInput(t *testing.T) {
	t.Parallel()
	for _, lang := range []Language{English, Japanese} {
		message := Explain(lang, ErrOutputIsInput)
		if message.Problem == "" || message.Remedy == "" {
			t.Fatalf("%s: message = %+v", lang, message)
		}
		assertNoRawWording(t, message)
	}
}

func TestExplainUnknownKeepsOwnText(t *testing.T) {
	t.Parallel()
	message := Explain(English, errors.New("something unusual"))
	if message.Problem != "something unusual" || message.Remedy != "" {
		t.Fatalf("message = %+v", message)
	}
}

// assertNoRawWording pins the issue's completion condition: a mapped message
// must not leak strconv or syscall wording.
func assertNoRawWording(t *testing.T, message Message) {
	t.Helper()
	combined := message.Problem + " " + message.Remedy
	for _, banned := range []string{"strconv", "no such file", "invalid syntax", "syscall", "Atoi", "ParseInt"} {
		if strings.Contains(combined, banned) {
			t.Errorf("message %q leaks raw wording %q", combined, banned)
		}
	}
}
