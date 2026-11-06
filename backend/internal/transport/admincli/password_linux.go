package admincli

import (
	"context"
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

var ErrTerminal = errors.New("cannot read hidden passwords from the terminal")

// ReadPasswords requires a controlling terminal. It never accepts a pipe as input.
func ReadPasswords(ctx context.Context) ([]byte, []byte, error) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return nil, nil, ErrTerminal
	}
	defer tty.Close()
	fd := int(tty.Fd())
	state, err := term.GetState(fd)
	if err != nil {
		return nil, nil, ErrTerminal
	}
	defer term.Restore(fd, state)
	attrs, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return nil, nil, ErrTerminal
	}
	attrs.Lflag &^= unix.ECHO
	attrs.Lflag |= unix.ICANON | unix.ISIG
	attrs.Iflag |= unix.ICRNL
	if err := unix.IoctlSetTermios(fd, unix.TCSETS, attrs); err != nil {
		return nil, nil, ErrTerminal
	}
	if err := unix.SetNonblock(fd, true); err != nil {
		return nil, nil, ErrTerminal
	}
	defer unix.SetNonblock(fd, false)
	read := func(prompt string) ([]byte, error) {
		if _, err := fmt.Fprint(tty, prompt); err != nil {
			return nil, ErrTerminal
		}
		for {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			fds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
			_, err := unix.Poll(fds, 100)
			if errors.Is(err, unix.EINTR) {
				continue
			}
			if err != nil {
				return nil, ErrTerminal
			}
			if fds[0].Revents&(unix.POLLERR|unix.POLLNVAL|unix.POLLHUP) != 0 {
				return nil, ErrTerminal
			}
			if fds[0].Revents&unix.POLLIN != 0 {
				value, err := term.ReadPassword(fd)
				fmt.Fprintln(tty)
				if err != nil {
					clear(value)
					return nil, ErrTerminal
				}
				return value, nil
			}
		}
	}
	first, err := read("Password (15-128 characters after NFC): ")
	if err != nil {
		return nil, nil, err
	}
	second, err := read("Confirm password: ")
	if err != nil {
		clear(first)
		return nil, nil, err
	}
	return first, second, nil
}
