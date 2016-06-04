package tty

import (
	"os"
	"unicode"
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

func (tty *TTY) ReadPassword() (string, error) {
	rs := []rune{}
loop:
	for {
		r, err := tty.readRune()
		if err != nil {
			return "", err
		}
		switch r {
		case 13:
			break loop
		case 8:
			if len(rs) > 0 {
				rs = rs[:len(rs)-1]
				tty.Output().WriteString("\b \b")
			}
		default:
			if unicode.IsPrint(r) {
				rs = append(rs, r)
				tty.Output().WriteString("*")
			}
		}
	}
	tty.Output().WriteString("\n")
	return string(rs), nil
}
