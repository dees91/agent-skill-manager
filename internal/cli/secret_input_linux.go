//go:build linux

package cli

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/sys/unix"
)

func readTTYSecret(file *os.File, stderr io.Writer) (string, error) {
	fd := int(file.Fd())
	state, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return "", fmt.Errorf("standard input is not a terminal; use --key-stdin")
	}
	old := *state
	state.Lflag &^= unix.ECHO
	if err := unix.IoctlSetTermios(fd, unix.TCSETS, state); err != nil {
		return "", fmt.Errorf("standard input is not a terminal; use --key-stdin")
	}
	restore := func() { _ = unix.IoctlSetTermios(fd, unix.TCSETS, &old) }
	defer restore()
	stop := watchInterrupt(restore)
	defer stop()
	fmt.Fprint(stderr, "TypeSafe API key: ")
	secret, err := readSecretLine(file)
	fmt.Fprintln(stderr)
	return secret, err
}
