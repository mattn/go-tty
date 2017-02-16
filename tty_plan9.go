package tty

import (
	"bufio"
	"os"
	"syscall"
)

type TTY struct {
	in  *os.File
	out *os.File
}

func open() (*TTY, error) {
	tty := new(TTY)

	in, err := os.Open("/dev/cons")
	if err != nil {
		return nil, err
	}
	tty.in = in

	out, err := os.OpenFile("/dev/cons", syscall.O_WRONLY, 0)
	if err != nil {
		return nil, err
	}
	tty.out = out

	return tty, nil
}

func (tty *TTY) readRune() (rune, error) {
	in := bufio.NewReader(tty.in)
	r, _, err := in.ReadRune()
	return r, err
}

func (tty *TTY) close() (err error) {
	if err2 := tty.in.Close(); err2 != nil {
		err = err2
	}
	if err2 := tty.out.Close(); err2 != nil {
		err = err2
	}
	return
}

func (tty *TTY) size() (int, int, error) {
	return 80, 24, nil
}

func (tty *TTY) input() *os.File {
	return tty.in
}

func (tty *TTY) output() *os.File {
	return tty.out
}
