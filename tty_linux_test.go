//go:build linux
// +build linux

package tty

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime/debug"
	"testing"

	"golang.org/x/sys/unix"
)

func descriptorsForPath(t *testing.T, path string) int {
	t.Helper()
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Fatal(err)
	}
	var count int
	for _, entry := range entries {
		target, err := os.Readlink(filepath.Join("/proc/self/fd", entry.Name()))
		if err == nil && target == path {
			count++
		}
	}
	return count
}

func TestOpenDeviceFailureClosesDescriptors(t *testing.T) {
	// Failed opens must close promptly, without waiting for os.File finalizers.
	previousGCPercent := debug.SetGCPercent(-1)
	t.Cleanup(func() { debug.SetGCPercent(previousGCPercent) })

	for _, tc := range []struct {
		name      string
		directory bool
		wantErr   error
	}{
		{"output open", true, unix.EISDIR},
		{"termios read", false, unix.ENOTTY},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := t.TempDir()
			if !tc.directory {
				path = filepath.Join(path, "not-a-terminal")
				if err := os.WriteFile(path, nil, 0600); err != nil {
					t.Fatal(err)
				}
			}
			for i := 0; i < 3; i++ {
				device, err := OpenDevice(path)
				if device != nil || !errors.Is(err, tc.wantErr) {
					t.Fatalf("OpenDevice(%q) = %v, %v; want nil, %v", path, device, err, tc.wantErr)
				}
			}
			if got := descriptorsForPath(t, path); got != 0 {
				t.Errorf("failed OpenDevice calls retained %d descriptors for %q", got, path)
			}
		})
	}
}

func TestOpenDevicePTYLifecycle(t *testing.T) {
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer master.Close()
	if err := unix.IoctlSetPointerInt(int(master.Fd()), unix.TIOCSPTLCK, 0); err != nil {
		t.Fatal(err)
	}
	number, err := unix.IoctlGetInt(int(master.Fd()), unix.TIOCGPTN)
	if err != nil {
		t.Fatal(err)
	}
	path := fmt.Sprintf("/dev/pts/%d", number)
	before, err := unix.IoctlGetTermios(int(master.Fd()), ioctlReadTermios)
	if err != nil {
		t.Fatal(err)
	}

	device, err := OpenDevice(path)
	if err != nil {
		t.Fatal(err)
	}
	defer device.Close()
	input, output := device.Input(), device.Output()
	inputFD, outputFD := input.Fd(), output.Fd()
	if got := descriptorsForPath(t, path); got != 2 {
		t.Fatalf("OpenDevice owns %d descriptors, want 2", got)
	}
	if _, err := master.Write([]byte("é")); err != nil {
		t.Fatal(err)
	}
	if r, n, err := device.ReadRune(); r != 'é' || n != 2 || err != nil {
		t.Errorf("ReadRune() = %q, %d, %v; want 'é', 2, nil", r, n, err)
	}
	if err := device.Close(); err != nil {
		t.Fatal(err)
	}
	after, err := unix.IoctlGetTermios(int(master.Fd()), ioctlReadTermios)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Errorf("Close did not restore terminal settings: got %+v, want %+v", after, before)
	}
	if got := descriptorsForPath(t, path); got != 0 {
		t.Errorf("Close retained %d terminal descriptors", got)
	}
	if _, err := input.Read(make([]byte, 1)); !errors.Is(err, os.ErrClosed) {
		t.Errorf("retained input Read() = %v, want os.ErrClosed", err)
	}
	if _, err := output.Write([]byte("x")); !errors.Is(err, os.ErrClosed) {
		t.Errorf("retained output Write() = %v, want os.ErrClosed", err)
	}

	// A repeated Close must not close a descriptor now owned by another file.
	replacement, err := os.CreateTemp(t.TempDir(), "replacement")
	if err != nil {
		t.Fatal(err)
	}
	defer replacement.Close()
	if replacement.Fd() != inputFD && replacement.Fd() != outputFD {
		t.Fatalf("replacement did not reuse a terminal descriptor")
	}
	if err := device.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := replacement.Write([]byte("still open")); err != nil {
		t.Errorf("reused descriptor was closed: %v", err)
	}
}
