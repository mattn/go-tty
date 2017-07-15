// +build !windows
// +build !plan9

package tty

import (
	"bufio"
	"os"
	"syscall"
	"unsafe"
)

type TTY struct {
	in      *os.File
	bin     *bufio.Reader
	out     *os.File
	termios syscall.Termios
}

func open() (*TTY, error) {
	tty := new(TTY)

	in, err := os.Open("/dev/tty")
	if err != nil {
		return nil, err
	}
	tty.in = in
	tty.bin = bufio.NewReader(in)

	out, err := os.OpenFile("/dev/tty", syscall.O_WRONLY, 0)
	if err != nil {
		return nil, err
	}
	tty.out = out

	if _, _, err := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(tty.in.Fd()), ioctlReadTermios, uintptr(unsafe.Pointer(&tty.termios)), 0, 0, 0); err != 0 {
		return nil, err
	}
	newios := tty.termios
	newios.Iflag &^= syscall.ISTRIP | syscall.INLCR | syscall.ICRNL | syscall.IGNCR | syscall.IXON | syscall.IXOFF
	newios.Lflag &^= syscall.ECHO | syscall.ICANON | syscall.ISIG
	if _, _, err := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(tty.in.Fd()), ioctlWriteTermios, uintptr(unsafe.Pointer(&newios)), 0, 0, 0); err != 0 {
		return nil, err
	}
	return tty, nil
}

func (tty *TTY) inbuf() bool {
	return tty.bin.Buffered() > 0
}

func (tty *TTY) readRune() (rune, error) {
	r, _, err := tty.bin.ReadRune()
	return r, err
}

func (tty *TTY) close() error {
	_, _, err := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(tty.in.Fd()), ioctlWriteTermios, uintptr(unsafe.Pointer(&tty.termios)), 0, 0, 0)
	return err
}

func (tty *TTY) size() (int, int, error) {
	var dim [4]uint16
	if _, _, err := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(tty.out.Fd()), uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(&dim)), 0, 0, 0); err != 0 {
		return -1, -1, err
	}
	return int(dim[1]), int(dim[0]), nil
}

func (tty *TTY) input() *os.File {
	return tty.in
}

func (tty *TTY) output() *os.File {
	return tty.out
}
