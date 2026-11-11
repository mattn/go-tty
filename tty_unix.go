//go:build !windows && !plan9
// +build !windows,!plan9

package tty

import (
	"bufio"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

type TTY struct {
	in      *os.File
	bin     *bufio.Reader
	out     *os.File
	termios unix.Termios
	ss      chan os.Signal
}

func open(path string) (*TTY, error) {
	tty := new(TTY)

	in, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	tty.in = in
	tty.bin = bufio.NewReader(in)

	out, err := os.OpenFile(path, unix.O_WRONLY, 0)
	if err != nil {
		in.Close()
		return nil, err
	}
	tty.out = out

	termios, err := unix.IoctlGetTermios(int(tty.in.Fd()), ioctlReadTermios)
	if err != nil {
		out.Close()
		in.Close()
		return nil, err
	}
	tty.termios = *termios

	termios.Iflag &^= unix.ISTRIP | unix.INLCR | unix.ICRNL | unix.IGNCR | unix.IXOFF
	termios.Lflag &^= unix.ECHO | unix.ICANON /*| unix.ISIG*/
	termios.Cc[unix.VMIN] = 1
	termios.Cc[unix.VTIME] = 0
	if err := unix.IoctlSetTermios(int(tty.in.Fd()), ioctlWriteTermios, termios); err != nil {
		out.Close()
		in.Close()
		return nil, err
	}

	tty.ss = make(chan os.Signal, 1)

	return tty, nil
}

func (tty *TTY) buffered() bool {
	return tty.bin.Buffered() > 0
}

func (tty *TTY) readRune() (rune, int, error) {
	r, size, err := tty.bin.ReadRune()
	return r, size, err
}

func (tty *TTY) close() error {
	if tty.out == nil || tty.in == nil {
		return nil
	}

	signal.Stop(tty.ss)
	close(tty.ss)
	err1 := unix.IoctlSetTermios(int(tty.in.Fd()), ioctlWriteTermios, &tty.termios)
	err2 := tty.out.Close()
	err3 := tty.in.Close()

	tty.out = nil
	tty.in = nil
	if err1 != nil {
		return err1
	}
	if err2 != nil {
		return err2
	}
	return err3
}

func (tty *TTY) size() (int, int, error) {
	x, y, _, _, err := tty.sizePixel()
	return x, y, err
}

func (tty *TTY) sizePixel() (int, int, int, int, error) {
	ws, err := unix.IoctlGetWinsize(int(tty.out.Fd()), unix.TIOCGWINSZ)
	if err != nil {
		return -1, -1, -1, -1, err
	}
	xpixel, ypixel := int(ws.Xpixel), int(ws.Ypixel)
	if xpixel == 0 || ypixel == 0 {
		if xp, yp := tty.queryPixelSize(); xp > 0 && yp > 0 {
			xpixel, ypixel = xp, yp
		}
	}
	return int(ws.Col), int(ws.Row), xpixel, ypixel, nil
}

// queryPixelSize sends the xterm "report window size in pixels" sequence
// (\x1b[14t) and parses the response (\x1b[4;height;widtht).
func (tty *TTY) queryPixelSize() (xpixel, ypixel int) {
	fd := int(tty.in.Fd())

	// Temporarily set VMIN=0, VTIME=1 (100ms timeout) so the read
	// returns promptly if the terminal does not respond.
	termios, err := unix.IoctlGetTermios(fd, ioctlReadTermios)
	if err != nil {
		return 0, 0
	}
	backup := *termios
	termios.Cc[unix.VMIN] = 0
	termios.Cc[unix.VTIME] = 1
	if err := unix.IoctlSetTermios(fd, ioctlWriteTermios, termios); err != nil {
		return 0, 0
	}
	defer unix.IoctlSetTermios(fd, ioctlWriteTermios, &backup)

	tty.out.WriteString("\x1b[14t")

	// Read response byte by byte until 't' or timeout.
	var buf [1]byte
	var resp []byte
	for {
		n, _ := tty.in.Read(buf[:])
		if n == 0 {
			break
		}
		resp = append(resp, buf[0])
		if buf[0] == 't' {
			break
		}
	}

	// Parse \x1b[4;height;widtht
	s := string(resp)
	idx := strings.Index(s, "[4;")
	if idx < 0 {
		return 0, 0
	}
	s = s[idx+3:]
	tidx := strings.IndexByte(s, 't')
	if tidx < 0 {
		return 0, 0
	}
	parts := strings.SplitN(s[:tidx], ";", 2)
	if len(parts) != 2 {
		return 0, 0
	}
	h, err1 := strconv.Atoi(parts[0])
	w, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil || h <= 0 || w <= 0 {
		return 0, 0
	}
	return w, h
}

func (tty *TTY) input() *os.File {
	return tty.in
}

func (tty *TTY) output() *os.File {
	return tty.out
}

func (tty *TTY) raw() (func() error, error) {
	termios, err := unix.IoctlGetTermios(int(tty.in.Fd()), ioctlReadTermios)
	if err != nil {
		return nil, err
	}
	backup := *termios

	termios.Iflag &^= unix.IGNBRK | unix.BRKINT | unix.PARMRK | unix.ISTRIP | unix.INLCR | unix.IGNCR | unix.ICRNL | unix.IXON
	termios.Oflag &^= unix.OPOST
	termios.Lflag &^= unix.ECHO | unix.ECHONL | unix.ICANON | unix.ISIG | unix.IEXTEN
	termios.Cflag &^= unix.CSIZE | unix.PARENB
	termios.Cflag |= unix.CS8
	termios.Cc[unix.VMIN] = 1
	termios.Cc[unix.VTIME] = 0
	if err = unix.IoctlSetTermios(int(tty.in.Fd()), ioctlWriteTermios, termios); err != nil {
		return nil, err
	}
	if err = syscall.SetNonblock(int(tty.in.Fd()), true); err != nil {
		return nil, err
	}

	return func() error {
		if err := unix.IoctlSetTermios(int(tty.in.Fd()), ioctlWriteTermios, &backup); err != nil {
			return err
		}
		return nil
	}, nil
}

func (tty *TTY) sigwinch() <-chan WINSIZE {
	signal.Notify(tty.ss, unix.SIGWINCH)

	ws := make(chan WINSIZE)
	go func() {
		defer close(ws)
		for sig := range tty.ss {
			if sig != unix.SIGWINCH {
				continue
			}

			w, h, err := tty.size()
			if err != nil {
				continue
			}
			// send but do not block for it
			select {
			case ws <- WINSIZE{W: w, H: h}:
			default:
			}

		}
	}()
	return ws
}
