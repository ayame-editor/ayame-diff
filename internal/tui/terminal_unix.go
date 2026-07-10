//go:build linux || darwin

package tui

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"
	"unsafe"
)

type unixTerminal struct {
	in       *os.File
	out      *os.File
	inFD     int
	outFD    int
	oldState syscall.Termios
	pending  []byte
	closed   bool
}

func Open() (Terminal, error) {
	inFD := int(os.Stdin.Fd())
	outFD := int(os.Stdout.Fd())
	oldState, err := getTermios(inFD)
	if err != nil {
		return nil, fmt.Errorf("interactive mode requires a terminal on stdin: %w", err)
	}
	if _, err := getWindowSize(outFD); err != nil {
		return nil, fmt.Errorf("interactive mode requires a terminal on stdout: %w", err)
	}

	raw := oldState
	raw.Iflag &^= syscall.IGNBRK | syscall.BRKINT | syscall.PARMRK | syscall.ISTRIP | syscall.INLCR | syscall.IGNCR | syscall.ICRNL | syscall.IXON
	raw.Oflag &^= syscall.OPOST
	raw.Lflag &^= syscall.ECHO | syscall.ECHONL | syscall.ICANON | syscall.ISIG | syscall.IEXTEN
	raw.Cflag &^= syscall.CSIZE | syscall.PARENB
	raw.Cflag |= syscall.CS8
	raw.Cc[syscall.VMIN] = 1
	raw.Cc[syscall.VTIME] = 0
	if err := setTermios(inFD, raw); err != nil {
		return nil, fmt.Errorf("enable raw terminal mode: %w", err)
	}

	t := &unixTerminal{
		in:       os.Stdin,
		out:      os.Stdout,
		inFD:     inFD,
		outFD:    outFD,
		oldState: oldState,
	}
	if _, err := io.WriteString(t.out, "\x1b[?1049h\x1b[2J\x1b[H\x1b[?25l"); err != nil {
		_ = setTermios(inFD, oldState)
		return nil, err
	}
	return t, nil
}

func (t *unixTerminal) Close() error {
	if t.closed {
		return nil
	}
	t.closed = true
	_, writeErr := io.WriteString(t.out, "\x1b[0m\x1b[?25h\x1b[2J\x1b[H\x1b[?1049l")
	restoreErr := setTermios(t.inFD, t.oldState)
	if writeErr != nil {
		return writeErr
	}
	return restoreErr
}

func (t *unixTerminal) Size() (int, int) {
	ws, err := getWindowSize(t.outFD)
	if err != nil || ws.Col == 0 || ws.Row == 0 {
		return 80, 24
	}
	return int(ws.Col), int(ws.Row)
}

func (t *unixTerminal) Draw(screen string) error {
	width, height := t.Size()
	screen = NormalizeScreen(screen, width, height)
	screen = strings.ReplaceAll(screen, "\n", "\r\n")
	_, err := io.WriteString(t.out, "\x1b[H\x1b[2J"+screen)
	return err
}

func (t *unixTerminal) ReadKey() (Key, error) {
	b, err := t.readByte()
	if err != nil {
		return Key{}, err
	}
	switch b {
	case 0x03:
		return Key{Kind: KeyInterrupt, Ctrl: true}, nil
	case '\r', '\n':
		return Key{Kind: KeyEnter}, nil
	case '\t':
		return Key{Kind: KeyTab}, nil
	case 0x7f, 0x08:
		return Key{Kind: KeyBackspace}, nil
	case 0x1b:
		return t.readEscape()
	}
	if b > 0 && b < 0x20 {
		return Key{Kind: KeyRune, Rune: rune('a' + b - 1), Ctrl: true}, nil
	}
	return t.decodeRuneStartingWith(b, false)
}

func (t *unixTerminal) readEscape() (Key, error) {
	next, ok, err := t.readByteIfReady(45 * time.Millisecond)
	if err != nil {
		return Key{}, err
	}
	if !ok {
		return Key{Kind: KeyEscape}, nil
	}
	switch next {
	case '[':
		return t.readCSI()
	case 'O':
		b, err := t.readByte()
		if err != nil {
			return Key{}, err
		}
		switch b {
		case 'A':
			return Key{Kind: KeyUp}, nil
		case 'B':
			return Key{Kind: KeyDown}, nil
		case 'C':
			return Key{Kind: KeyRight}, nil
		case 'D':
			return Key{Kind: KeyLeft}, nil
		case 'H':
			return Key{Kind: KeyHome}, nil
		case 'F':
			return Key{Kind: KeyEnd}, nil
		}
		return Key{Kind: KeyUnknown}, nil
	default:
		key, err := t.decodeRuneStartingWith(next, true)
		if err != nil {
			return Key{}, err
		}
		key.Alt = true
		return key, nil
	}
}

func (t *unixTerminal) readCSI() (Key, error) {
	var sequence strings.Builder
	for sequence.Len() < 24 {
		b, err := t.readByte()
		if err != nil {
			return Key{}, err
		}
		sequence.WriteByte(b)
		if (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || b == '~' {
			break
		}
	}
	seq := sequence.String()
	if seq == "Z" {
		return Key{Kind: KeyBackTab, Shift: true}, nil
	}
	final := seq[len(seq)-1]
	ctrl, alt, shift := decodeCSIModifiers(seq)
	switch final {
	case 'A':
		return Key{Kind: KeyUp, Ctrl: ctrl, Alt: alt, Shift: shift}, nil
	case 'B':
		return Key{Kind: KeyDown, Ctrl: ctrl, Alt: alt, Shift: shift}, nil
	case 'C':
		return Key{Kind: KeyRight, Ctrl: ctrl, Alt: alt, Shift: shift}, nil
	case 'D':
		return Key{Kind: KeyLeft, Ctrl: ctrl, Alt: alt, Shift: shift}, nil
	case 'H':
		return Key{Kind: KeyHome, Ctrl: ctrl, Alt: alt, Shift: shift}, nil
	case 'F':
		return Key{Kind: KeyEnd, Ctrl: ctrl, Alt: alt, Shift: shift}, nil
	case '~':
		number := seq[:len(seq)-1]
		if semicolon := strings.IndexByte(number, ';'); semicolon >= 0 {
			number = number[:semicolon]
		}
		switch number {
		case "1", "7":
			return Key{Kind: KeyHome, Ctrl: ctrl, Alt: alt, Shift: shift}, nil
		case "3":
			return Key{Kind: KeyDelete, Ctrl: ctrl, Alt: alt, Shift: shift}, nil
		case "4", "8":
			return Key{Kind: KeyEnd, Ctrl: ctrl, Alt: alt, Shift: shift}, nil
		case "5":
			return Key{Kind: KeyPageUp, Ctrl: ctrl, Alt: alt, Shift: shift}, nil
		case "6":
			return Key{Kind: KeyPageDown, Ctrl: ctrl, Alt: alt, Shift: shift}, nil
		}
	}
	return Key{Kind: KeyUnknown}, nil
}

func decodeCSIModifiers(seq string) (ctrl, alt, shift bool) {
	semicolon := strings.LastIndexByte(seq, ';')
	if semicolon < 0 || semicolon+1 >= len(seq) {
		return false, false, false
	}
	value := 0
	for i := semicolon + 1; i < len(seq); i++ {
		if seq[i] < '0' || seq[i] > '9' {
			break
		}
		value = value*10 + int(seq[i]-'0')
	}
	if value < 2 {
		return false, false, false
	}
	mask := value - 1
	return mask&4 != 0, mask&2 != 0, mask&1 != 0
}

func (t *unixTerminal) decodeRuneStartingWith(first byte, alt bool) (Key, error) {
	if first < utf8.RuneSelf {
		return Key{Kind: KeyRune, Rune: rune(first), Alt: alt}, nil
	}
	expected := utf8.RuneLen(rune(first))
	// RuneLen cannot determine a length from a leading byte. Use the prefix.
	switch {
	case first&0xe0 == 0xc0:
		expected = 2
	case first&0xf0 == 0xe0:
		expected = 3
	case first&0xf8 == 0xf0:
		expected = 4
	default:
		return Key{Kind: KeyRune, Rune: utf8.RuneError, Alt: alt}, nil
	}
	bytes := make([]byte, expected)
	bytes[0] = first
	for i := 1; i < expected; i++ {
		b, err := t.readByte()
		if err != nil {
			return Key{}, err
		}
		bytes[i] = b
	}
	r, _ := utf8.DecodeRune(bytes)
	return Key{Kind: KeyRune, Rune: r, Alt: alt}, nil
}

func (t *unixTerminal) readByte() (byte, error) {
	if len(t.pending) > 0 {
		b := t.pending[0]
		t.pending = t.pending[1:]
		return b, nil
	}
	var one [1]byte
	for {
		n, err := t.in.Read(one[:])
		if n == 1 {
			return one[0], nil
		}
		if err != nil {
			return 0, err
		}
		if n == 0 {
			return 0, io.ErrNoProgress
		}
	}
}

func (t *unixTerminal) readByteIfReady(timeout time.Duration) (byte, bool, error) {
	if len(t.pending) > 0 {
		b := t.pending[0]
		t.pending = t.pending[1:]
		return b, true, nil
	}
	ready, err := waitReadable(t.inFD, timeout)
	if err != nil {
		return 0, false, err
	}
	if !ready {
		return 0, false, nil
	}
	b, err := t.readByte()
	return b, true, err
}

type windowSize struct {
	Row    uint16
	Col    uint16
	Xpixel uint16
	Ypixel uint16
}

func getTermios(fd int) (syscall.Termios, error) {
	var state syscall.Termios
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), uintptr(ioctlReadTermios), uintptr(unsafe.Pointer(&state)))
	if errno != 0 {
		return syscall.Termios{}, errno
	}
	return state, nil
}

func setTermios(fd int, state syscall.Termios) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), uintptr(ioctlWriteTermios), uintptr(unsafe.Pointer(&state)))
	if errno != 0 {
		return errno
	}
	return nil
}

func getWindowSize(fd int) (windowSize, error) {
	var size windowSize
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(&size)))
	if errno != 0 {
		return windowSize{}, errno
	}
	return size, nil
}

func isTemporaryReadError(err error) bool {
	return errors.Is(err, syscall.EINTR) || errors.Is(err, syscall.EAGAIN)
}
