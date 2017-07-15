// +build windows

package tty

import (
	"os"
	"syscall"
	"unsafe"

	"github.com/mattn/go-isatty"
)

const (
	rightAltPressed  = 1
	leftAltPressed   = 2
	rightCtrlPressed = 4
	leftCtrlPressed  = 8
	ctrlPressed      = rightCtrlPressed | leftCtrlPressed
	altPressed       = rightAltPressed | leftAltPressed
)

const (
	enableProcessedInput = 0x1
	enableLineInput      = 0x2
	enableEchoInput      = 0x4
	enableWindowInput    = 0x8
	enableMouseInput     = 0x10
	enableInsertMode     = 0x20
	enableQuickEditMode  = 0x40
	enableExtendedFlag   = 0x80

	keyEvent              = 0x1
	mouseEvent            = 0x2
	windowBufferSizeEvent = 0x4
)

var kernel32 = syscall.NewLazyDLL("kernel32.dll")

var (
	procAllocConsole                = kernel32.NewProc("AllocConsole")
	procSetStdHandle                = kernel32.NewProc("SetStdHandle")
	procGetStdHandle                = kernel32.NewProc("GetStdHandle")
	procSetConsoleScreenBufferSize  = kernel32.NewProc("SetConsoleScreenBufferSize")
	procCreateConsoleScreenBuffer   = kernel32.NewProc("CreateConsoleScreenBuffer")
	procGetConsoleScreenBufferInfo  = kernel32.NewProc("GetConsoleScreenBufferInfo")
	procWriteConsoleOutputCharacter = kernel32.NewProc("WriteConsoleOutputCharacterW")
	procWriteConsoleOutputAttribute = kernel32.NewProc("WriteConsoleOutputAttribute")
	procGetConsoleCursorInfo        = kernel32.NewProc("GetConsoleCursorInfo")
	procSetConsoleCursorInfo        = kernel32.NewProc("SetConsoleCursorInfo")
	procSetConsoleCursorPosition    = kernel32.NewProc("SetConsoleCursorPosition")
	procReadConsoleInput            = kernel32.NewProc("ReadConsoleInputW")
	procGetConsoleMode              = kernel32.NewProc("GetConsoleMode")
	procSetConsoleMode              = kernel32.NewProc("SetConsoleMode")
	procFillConsoleOutputCharacter  = kernel32.NewProc("FillConsoleOutputCharacterW")
	procFillConsoleOutputAttribute  = kernel32.NewProc("FillConsoleOutputAttribute")
	procScrollConsoleScreenBuffer   = kernel32.NewProc("ScrollConsoleScreenBufferW")
)

type wchar uint16
type short int16
type dword uint32
type word uint16

type coord struct {
	x short
	y short
}

type smallRect struct {
	left   short
	top    short
	right  short
	bottom short
}

type consoleScreenBufferInfo struct {
	size              coord
	cursorPosition    coord
	attributes        word
	window            smallRect
	maximumWindowSize coord
}

type consoleCursorInfo struct {
	size    dword
	visible int32
}

type inputRecord struct {
	eventType word
	_         [2]byte
	event     [16]byte
}

type keyEventRecord struct {
	keyDown         int32
	repeatCount     word
	virtualKeyCode  word
	virtualScanCode word
	unicodeChar     wchar
	controlKeyState dword
}

type windowBufferSizeRecord struct {
	size coord
}

type mouseEventRecord struct {
	mousePos        coord
	buttonState     dword
	controlKeyState dword
	eventFlags      dword
}

type charInfo struct {
	unicodeChar wchar
	attributes  word
}

type TTY struct {
	in  *os.File
	out *os.File
	st  uint32
	rs  []rune
}

func readConsoleInput(fd uintptr, record *inputRecord) (err error) {
	var w uint32
	r1, _, err := procReadConsoleInput.Call(fd, uintptr(unsafe.Pointer(record)), 1, uintptr(unsafe.Pointer(&w)))
	if r1 == 0 {
		return err
	}
	return nil
}

func open() (*TTY, error) {
	tty := new(TTY)
	if false && isatty.IsTerminal(os.Stdin.Fd()) {
		tty.in = os.Stdin
	} else {
		conin, err := os.Open("CONIN$")
		if err != nil {
			return nil, err
		}
		tty.in = conin
	}

	if isatty.IsTerminal(os.Stdout.Fd()) {
		tty.out = os.Stdout
	} else {
		procAllocConsole.Call()
		out, err := syscall.Open("CONOUT$", syscall.O_RDWR, 0)
		if err != nil {
			return nil, err
		}

		tty.out = os.NewFile(uintptr(out), "/dev/tty")
	}

	h := tty.in.Fd()
	var st uint32
	r1, _, err := procGetConsoleMode.Call(h, uintptr(unsafe.Pointer(&st)))
	if r1 == 0 {
		return nil, err
	}
	tty.st = st

	st &^= enableEchoInput
	st &^= enableInsertMode
	st &^= enableLineInput
	st &^= enableMouseInput
	st &^= enableWindowInput
	st &^= enableExtendedFlag
	st &^= enableQuickEditMode
	st |= enableProcessedInput

	// ignore error
	procSetConsoleMode.Call(h, uintptr(st))

	return tty, nil
}

func (tty *TTY) inbuf() bool {
	return len(tty.rs) > 0
}

func (tty *TTY) readRune() (rune, error) {
	if len(tty.rs) > 0 {
		r := tty.rs[0]
		tty.rs = tty.rs[1:]
		return r, nil
	}
	var ir inputRecord
	err := readConsoleInput(tty.in.Fd(), &ir)
	if err != nil {
		return 0, err
	}

	if ir.eventType == keyEvent {
		kr := (*keyEventRecord)(unsafe.Pointer(&ir.event))
		if kr.keyDown != 0 {
			if kr.unicodeChar > 0 {
				return rune(kr.unicodeChar), nil
			}
			switch kr.virtualKeyCode {
			case 0x25: // left
				tty.rs = []rune{0x5b, 0x44}
				return rune(0x1b), nil
			case 0x26: // up
				tty.rs = []rune{0x5b, 0x41}
				return rune(0x1b), nil
			case 0x27: // right
				tty.rs = []rune{0x5b, 0x43}
				return rune(0x1b), nil
			case 0x28: // down
				tty.rs = []rune{0x5b, 0x42}
				return rune(0x1b), nil
			case 0x2e: // delete
				tty.rs = []rune{0x5b, 0x33, 0x7e}
				return rune(0x1b), nil
			}
			return 0, nil
		}
	}
	return 0, nil
}

func (tty *TTY) close() error {
	procSetConsoleMode.Call(tty.in.Fd(), uintptr(tty.st))
	return nil
}

func (tty *TTY) size() (int, int, error) {
	var csbi consoleScreenBufferInfo
	r1, _, err := procGetConsoleScreenBufferInfo.Call(tty.out.Fd(), uintptr(unsafe.Pointer(&csbi)))
	if r1 == 0 {
		return 0, 0, err
	}
	return int(csbi.window.right - csbi.window.left), int(csbi.window.bottom - csbi.window.top), nil
}

func (tty *TTY) input() *os.File {
	return tty.in
}

func (tty *TTY) output() *os.File {
	return tty.out
}
