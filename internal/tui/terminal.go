package tui

import (
	"errors"
	"fmt"
	"strings"
)

// ErrInterrupted is returned when Ctrl+C is pressed in the interactive UI.
var ErrInterrupted = errors.New("interactive setup interrupted")

// ErrCancelled is returned when the user cancels the interactive UI.
var ErrCancelled = errors.New("interactive setup cancelled")

type KeyKind uint8

const (
	KeyUnknown KeyKind = iota
	KeyRune
	KeyEnter
	KeyEscape
	KeyUp
	KeyDown
	KeyLeft
	KeyRight
	KeyHome
	KeyEnd
	KeyPageUp
	KeyPageDown
	KeyBackspace
	KeyDelete
	KeyTab
	KeyBackTab
	KeyInterrupt
)

type Key struct {
	Kind  KeyKind
	Rune  rune
	Ctrl  bool
	Alt   bool
	Shift bool
}

// Terminal is the small cross-platform terminal surface used by the wizard.
// The Windows implementation uses the native Unicode Console API. Unix-like
// systems use raw terminal input and ANSI screen control.
type Terminal interface {
	ReadKey() (Key, error)
	Size() (width, height int)
	Draw(screen string) error
	Close() error
}

func NormalizeScreen(text string, width, height int) string {
	if width < 20 {
		width = 20
	}
	if height < 5 {
		height = 5
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for i := range lines {
		lines[i] = TruncateDisplay(lines[i], width)
	}
	return strings.Join(lines, "\n")
}

func ReadRequiredKey(t Terminal) (Key, error) {
	key, err := t.ReadKey()
	if err != nil {
		return Key{}, err
	}
	if key.Kind == KeyInterrupt {
		return Key{}, ErrInterrupted
	}
	return key, nil
}

func MustSize(t Terminal) (int, int) {
	width, height := t.Size()
	if width <= 0 {
		width = 80
	}
	if height <= 0 {
		height = 24
	}
	return width, height
}

func DrawLines(t Terminal, lines ...string) error {
	width, height := MustSize(t)
	return t.Draw(NormalizeScreen(strings.Join(lines, "\n"), width, height))
}

func KeyString(k Key) string {
	if k.Kind == KeyRune {
		return fmt.Sprintf("%q", k.Rune)
	}
	return fmt.Sprintf("key(%d)", k.Kind)
}
