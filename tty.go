package tty

import (
	"os"
)

func New() (*TTY, error) {
	return open()
}

func (tty *TTY) ReadRune() (rune, error) {
	return tty.readRune()
}

func (tty *TTY) Close() error {
	return tty.close()
}

func (tty *TTY) Size() (int, int, error) {
	return tty.size()
}

func (tty *TTY) Input() *os.File {
	return tty.input()
}

func (tty *TTY) Output() *os.File {
	return tty.output()
}
