package tty

import (
	"os"
	"strings"
	"unicode"
)

func Open() (*TTY, error) {
	return open()
}

func (tty *TTY) Buffered() bool {
	return tty.buffered()
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

func (tty *TTY) readString(isPassword bool) (string, error) {
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
		case 8, 127:
			if len(rs) > 0 {
				rs = rs[:len(rs)-1]
				tty.Output().WriteString("\b \b")
			}
		default:
			if unicode.IsPrint(r) {
				rs = append(rs, r)
				if isPassword {
					tty.Output().WriteString("*")
				} else {
					tty.Output().WriteString(string(r))
				}
			}
		}
	}
	return string(rs), nil
}

func (tty *TTY) ReadString() (string, error) {
	defer tty.Output().WriteString("\n")
	return tty.readString(false)
}

func (tty *TTY) ReadPassword() (string, error) {
	defer tty.Output().WriteString("\n")
	return tty.readString(true)
}

func (tty *TTY) ReadPasswordClear() (string, error) {
	s, err := tty.readString(true)
	tty.Output().WriteString(
		strings.Repeat("\b", len(s)) +
			strings.Repeat(" ", len(s)) +
			strings.Repeat("\b", len(s)))
	return s, err
}
