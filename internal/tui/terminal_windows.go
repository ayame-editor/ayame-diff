//go:build windows

package tui

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

const (
	enableProcessedInput = 0x0001
	enableLineInput      = 0x0002
	enableEchoInput      = 0x0004
	enableWindowInput    = 0x0008
	enableMouseInput     = 0x0010
	enableInsertMode     = 0x0020
	enableQuickEditMode  = 0x0040
	enableExtendedFlags  = 0x0080

	keyEventType = 0x0001

	rightAltPressed  = 0x0001
	leftAltPressed   = 0x0002
	rightCtrlPressed = 0x0004
	leftCtrlPressed  = 0x0008
	shiftPressed     = 0x0010

	vkBack   = 0x08
	vkTab    = 0x09
	vkReturn = 0x0d
	vkEscape = 0x1b
	vkPrior  = 0x21
	vkNext   = 0x22
	vkEnd    = 0x23
	vkHome   = 0x24
	vkLeft   = 0x25
	vkUp     = 0x26
	vkRight  = 0x27
	vkDown   = 0x28
	vkDelete = 0x2e
	vkC      = 0x43
)

var (
	kernel32                       = syscall.NewLazyDLL("kernel32.dll")
	procGetConsoleMode             = kernel32.NewProc("GetConsoleMode")
	procSetConsoleMode             = kernel32.NewProc("SetConsoleMode")
	procReadConsoleInputW          = kernel32.NewProc("ReadConsoleInputW")
	procGetConsoleScreenBufferInfo = kernel32.NewProc("GetConsoleScreenBufferInfo")
	procSetConsoleCursorPosition   = kernel32.NewProc("SetConsoleCursorPosition")
	procFillConsoleOutputCharacter = kernel32.NewProc("FillConsoleOutputCharacterW")
	procFillConsoleOutputAttribute = kernel32.NewProc("FillConsoleOutputAttribute")
	procWriteConsoleW              = kernel32.NewProc("WriteConsoleW")
	procGetConsoleCursorInfo       = kernel32.NewProc("GetConsoleCursorInfo")
	procSetConsoleCursorInfo       = kernel32.NewProc("SetConsoleCursorInfo")
)

type windowsTerminal struct {
	inHandle      syscall.Handle
	outHandle     syscall.Handle
	oldInputMode  uint32
	oldOutputMode uint32
	oldCursorInfo consoleCursorInfo
	repeatKey     Key
	repeatCount   uint16
	pendingHigh   uint16
	queuedKey     *Key
	closed        bool
}

type coord struct {
	X int16
	Y int16
}

type smallRect struct {
	Left   int16
	Top    int16
	Right  int16
	Bottom int16
}

type consoleScreenBufferInfo struct {
	Size              coord
	CursorPosition    coord
	Attributes        uint16
	Window            smallRect
	MaximumWindowSize coord
}

type consoleCursorInfo struct {
	Size    uint32
	Visible int32
}

type inputRecord struct {
	EventType uint16
	_         uint16
	Event     [16]byte
}

type keyEventRecord struct {
	KeyDown         int32
	RepeatCount     uint16
	VirtualKeyCode  uint16
	VirtualScanCode uint16
	UnicodeChar     uint16
	ControlKeyState uint32
}

// Compile-time ABI checks for the Win32 console structures used above.
var _ [20 - int(unsafe.Sizeof(inputRecord{}))]byte
var _ [int(unsafe.Sizeof(inputRecord{})) - 20]byte
var _ [16 - int(unsafe.Sizeof(keyEventRecord{}))]byte
var _ [int(unsafe.Sizeof(keyEventRecord{})) - 16]byte
var _ [22 - int(unsafe.Sizeof(consoleScreenBufferInfo{}))]byte
var _ [int(unsafe.Sizeof(consoleScreenBufferInfo{})) - 22]byte
var _ [8 - int(unsafe.Sizeof(consoleCursorInfo{}))]byte
var _ [int(unsafe.Sizeof(consoleCursorInfo{})) - 8]byte

func Open() (Terminal, error) {
	inHandle := syscall.Handle(os.Stdin.Fd())
	outHandle := syscall.Handle(os.Stdout.Fd())
	oldInputMode, err := getConsoleMode(inHandle)
	if err != nil {
		return nil, fmt.Errorf("interactive mode requires a Windows console on stdin: %w", err)
	}
	oldOutputMode, err := getConsoleMode(outHandle)
	if err != nil {
		return nil, fmt.Errorf("interactive mode requires a Windows console on stdout: %w", err)
	}
	cursorInfo, err := getConsoleCursorInfo(outHandle)
	if err != nil {
		return nil, fmt.Errorf("read Windows console cursor state: %w", err)
	}

	inputMode := oldInputMode
	inputMode |= enableExtendedFlags | enableWindowInput
	inputMode &^= enableProcessedInput | enableLineInput | enableEchoInput | enableMouseInput | enableQuickEditMode | enableInsertMode
	if err := setConsoleMode(inHandle, inputMode); err != nil {
		return nil, fmt.Errorf("enable Windows console raw input: %w", err)
	}

	hidden := cursorInfo
	hidden.Visible = 0
	if err := setConsoleCursorInfo(outHandle, hidden); err != nil {
		_ = setConsoleMode(inHandle, oldInputMode)
		return nil, fmt.Errorf("hide Windows console cursor: %w", err)
	}

	return &windowsTerminal{
		inHandle:      inHandle,
		outHandle:     outHandle,
		oldInputMode:  oldInputMode,
		oldOutputMode: oldOutputMode,
		oldCursorInfo: cursorInfo,
	}, nil
}

func (t *windowsTerminal) Close() error {
	if t.closed {
		return nil
	}
	t.closed = true
	clearErr := t.clearViewport()
	inputErr := setConsoleMode(t.inHandle, t.oldInputMode)
	outputErr := setConsoleMode(t.outHandle, t.oldOutputMode)
	cursorErr := setConsoleCursorInfo(t.outHandle, t.oldCursorInfo)
	switch {
	case clearErr != nil:
		return clearErr
	case inputErr != nil:
		return inputErr
	case outputErr != nil:
		return outputErr
	default:
		return cursorErr
	}
}

func (t *windowsTerminal) Size() (int, int) {
	info, err := getConsoleScreenBufferInfo(t.outHandle)
	if err != nil {
		return 80, 24
	}
	width := int(info.Window.Right-info.Window.Left) + 1
	height := int(info.Window.Bottom-info.Window.Top) + 1
	if width <= 0 || height <= 0 {
		return 80, 24
	}
	return width, height
}

func (t *windowsTerminal) Draw(screen string) error {
	width, height := t.Size()
	screen = NormalizeScreen(screen, width, height)
	if err := t.clearViewport(); err != nil {
		return err
	}
	screen = strings.ReplaceAll(screen, "\n", "\r\n")
	return writeConsole(t.outHandle, screen)
}

func (t *windowsTerminal) ReadKey() (Key, error) {
	if t.queuedKey != nil {
		key := *t.queuedKey
		t.queuedKey = nil
		return key, nil
	}
	if t.repeatCount > 0 {
		t.repeatCount--
		return t.repeatKey, nil
	}

	for {
		var record inputRecord
		var count uint32
		r1, _, callErr := procReadConsoleInputW.Call(
			uintptr(t.inHandle),
			uintptr(unsafe.Pointer(&record)),
			1,
			uintptr(unsafe.Pointer(&count)),
		)
		if r1 == 0 {
			return Key{}, windowsCallError(callErr)
		}
		if count == 0 || record.EventType != keyEventType {
			continue
		}
		event := *(*keyEventRecord)(unsafe.Pointer(&record.Event[0]))
		if event.KeyDown == 0 {
			continue
		}
		key, ok := t.decodeWindowsKey(event)
		if !ok {
			continue
		}
		if event.RepeatCount > 1 {
			t.repeatKey = key
			t.repeatCount = event.RepeatCount - 1
		}
		return key, nil
	}
}

func (t *windowsTerminal) decodeWindowsKey(event keyEventRecord) (Key, bool) {
	ctrl := event.ControlKeyState&(leftCtrlPressed|rightCtrlPressed) != 0
	alt := event.ControlKeyState&(leftAltPressed|rightAltPressed) != 0
	shift := event.ControlKeyState&shiftPressed != 0
	altGr := event.ControlKeyState&rightAltPressed != 0 && event.ControlKeyState&leftCtrlPressed != 0

	switch event.VirtualKeyCode {
	case vkBack:
		return Key{Kind: KeyBackspace, Ctrl: ctrl, Alt: alt, Shift: shift}, true
	case vkTab:
		if shift {
			return Key{Kind: KeyBackTab, Shift: true}, true
		}
		return Key{Kind: KeyTab, Ctrl: ctrl, Alt: alt}, true
	case vkReturn:
		return Key{Kind: KeyEnter, Ctrl: ctrl, Alt: alt, Shift: shift}, true
	case vkEscape:
		return Key{Kind: KeyEscape}, true
	case vkPrior:
		return Key{Kind: KeyPageUp, Ctrl: ctrl, Alt: alt, Shift: shift}, true
	case vkNext:
		return Key{Kind: KeyPageDown, Ctrl: ctrl, Alt: alt, Shift: shift}, true
	case vkEnd:
		return Key{Kind: KeyEnd, Ctrl: ctrl, Alt: alt, Shift: shift}, true
	case vkHome:
		return Key{Kind: KeyHome, Ctrl: ctrl, Alt: alt, Shift: shift}, true
	case vkLeft:
		return Key{Kind: KeyLeft, Ctrl: ctrl, Alt: alt, Shift: shift}, true
	case vkUp:
		return Key{Kind: KeyUp, Ctrl: ctrl, Alt: alt, Shift: shift}, true
	case vkRight:
		return Key{Kind: KeyRight, Ctrl: ctrl, Alt: alt, Shift: shift}, true
	case vkDown:
		return Key{Kind: KeyDown, Ctrl: ctrl, Alt: alt, Shift: shift}, true
	case vkDelete:
		return Key{Kind: KeyDelete, Ctrl: ctrl, Alt: alt, Shift: shift}, true
	}

	if event.VirtualKeyCode == vkC && ctrl && !altGr {
		return Key{Kind: KeyInterrupt, Ctrl: true}, true
	}
	if event.UnicodeChar == 0 {
		return Key{}, false
	}
	if event.UnicodeChar == 0x0003 {
		return Key{Kind: KeyInterrupt, Ctrl: true}, true
	}

	unit := event.UnicodeChar
	if unit >= 0xd800 && unit <= 0xdbff {
		t.pendingHigh = unit
		return Key{}, false
	}
	if unit >= 0xdc00 && unit <= 0xdfff && t.pendingHigh != 0 {
		r := utf16.DecodeRune(rune(t.pendingHigh), rune(unit))
		t.pendingHigh = 0
		return Key{Kind: KeyRune, Rune: r, Shift: shift}, true
	}
	if t.pendingHigh != 0 {
		replacement := Key{Kind: KeyRune, Rune: '\ufffd'}
		current := Key{Kind: KeyRune, Rune: rune(unit), Ctrl: ctrl, Alt: alt, Shift: shift}
		if altGr {
			current.Ctrl = false
			current.Alt = false
		}
		t.pendingHigh = 0
		t.queuedKey = &current
		return replacement, true
	}
	key := Key{Kind: KeyRune, Rune: rune(unit), Ctrl: ctrl, Alt: alt, Shift: shift}
	if altGr {
		key.Ctrl = false
		key.Alt = false
	}
	return key, true
}

func (t *windowsTerminal) clearViewport() error {
	info, err := getConsoleScreenBufferInfo(t.outHandle)
	if err != nil {
		return err
	}
	width := int(info.Window.Right-info.Window.Left) + 1
	if width <= 0 {
		return nil
	}
	for row := int(info.Window.Top); row <= int(info.Window.Bottom); row++ {
		start := coord{X: info.Window.Left, Y: int16(row)}
		if err := fillConsoleCharacter(t.outHandle, ' ', uint32(width), start); err != nil {
			return err
		}
		if err := fillConsoleAttribute(t.outHandle, info.Attributes, uint32(width), start); err != nil {
			return err
		}
	}
	return setConsoleCursorPosition(t.outHandle, coord{X: info.Window.Left, Y: info.Window.Top})
}

func getConsoleMode(handle syscall.Handle) (uint32, error) {
	var mode uint32
	r1, _, callErr := procGetConsoleMode.Call(uintptr(handle), uintptr(unsafe.Pointer(&mode)))
	if r1 == 0 {
		return 0, windowsCallError(callErr)
	}
	return mode, nil
}

func setConsoleMode(handle syscall.Handle, mode uint32) error {
	r1, _, callErr := procSetConsoleMode.Call(uintptr(handle), uintptr(mode))
	if r1 == 0 {
		return windowsCallError(callErr)
	}
	return nil
}

func getConsoleScreenBufferInfo(handle syscall.Handle) (consoleScreenBufferInfo, error) {
	var info consoleScreenBufferInfo
	r1, _, callErr := procGetConsoleScreenBufferInfo.Call(uintptr(handle), uintptr(unsafe.Pointer(&info)))
	if r1 == 0 {
		return consoleScreenBufferInfo{}, windowsCallError(callErr)
	}
	return info, nil
}

func setConsoleCursorPosition(handle syscall.Handle, position coord) error {
	packed := uintptr(uint32(uint16(position.X)) | uint32(uint16(position.Y))<<16)
	r1, _, callErr := procSetConsoleCursorPosition.Call(uintptr(handle), packed)
	if r1 == 0 {
		return windowsCallError(callErr)
	}
	return nil
}

func fillConsoleCharacter(handle syscall.Handle, char uint16, length uint32, position coord) error {
	var written uint32
	packed := uintptr(uint32(uint16(position.X)) | uint32(uint16(position.Y))<<16)
	r1, _, callErr := procFillConsoleOutputCharacter.Call(
		uintptr(handle), uintptr(char), uintptr(length), packed, uintptr(unsafe.Pointer(&written)),
	)
	if r1 == 0 {
		return windowsCallError(callErr)
	}
	return nil
}

func fillConsoleAttribute(handle syscall.Handle, attribute uint16, length uint32, position coord) error {
	var written uint32
	packed := uintptr(uint32(uint16(position.X)) | uint32(uint16(position.Y))<<16)
	r1, _, callErr := procFillConsoleOutputAttribute.Call(
		uintptr(handle), uintptr(attribute), uintptr(length), packed, uintptr(unsafe.Pointer(&written)),
	)
	if r1 == 0 {
		return windowsCallError(callErr)
	}
	return nil
}

func getConsoleCursorInfo(handle syscall.Handle) (consoleCursorInfo, error) {
	var info consoleCursorInfo
	r1, _, callErr := procGetConsoleCursorInfo.Call(uintptr(handle), uintptr(unsafe.Pointer(&info)))
	if r1 == 0 {
		return consoleCursorInfo{}, windowsCallError(callErr)
	}
	return info, nil
}

func setConsoleCursorInfo(handle syscall.Handle, info consoleCursorInfo) error {
	r1, _, callErr := procSetConsoleCursorInfo.Call(uintptr(handle), uintptr(unsafe.Pointer(&info)))
	if r1 == 0 {
		return windowsCallError(callErr)
	}
	return nil
}

func writeConsole(handle syscall.Handle, text string) error {
	units := utf16.Encode([]rune(text))
	for len(units) > 0 {
		chunkSize := len(units)
		if chunkSize > 16*1024 {
			chunkSize = 16 * 1024
		}
		if chunkSize < len(units) && chunkSize > 0 && units[chunkSize-1] >= 0xd800 && units[chunkSize-1] <= 0xdbff {
			chunkSize--
		}
		var written uint32
		r1, _, callErr := procWriteConsoleW.Call(
			uintptr(handle),
			uintptr(unsafe.Pointer(&units[0])),
			uintptr(chunkSize),
			uintptr(unsafe.Pointer(&written)),
			0,
		)
		if r1 == 0 {
			return windowsCallError(callErr)
		}
		if written == 0 {
			return syscall.EIO
		}
		units = units[int(written):]
	}
	return nil
}

func windowsCallError(err error) error {
	if err == nil || err == syscall.Errno(0) {
		return syscall.EINVAL
	}
	return err
}
