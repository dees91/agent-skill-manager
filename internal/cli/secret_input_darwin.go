//go:build darwin

package cli

import (
	"fmt"
	"io"
	"os"
	"os/signal"

	"golang.org/x/sys/unix"
)

func readTTYSecret(file *os.File, stderr io.Writer) (string, error) {
	fd := int(file.Fd())
	if _, err := unix.IoctlGetTermios(fd, unix.TIOCGETA); err != nil {
		return "", fmt.Errorf("standard input is not a terminal; use --key-stdin")
	}
	state, err := unix.IoctlGetTermios(fd, unix.TIOCGETA)
	if err != nil {
		return "", fmt.Errorf("standard input is not a terminal; use --key-stdin")
	}
	old := *state
	state.Lflag &^= unix.ECHO
	if err := unix.IoctlSetTermios(fd, unix.TIOCSETA, state); err != nil {
		return "", fmt.Errorf("standard input is not a terminal; use --key-stdin")
	}
	restore := func() { _ = unix.IoctlSetTermios(fd, unix.TIOCSETA, &old) }
	defer restore()
	ctx, stop := signal.NotifyContext(backgroundContext(), os.Interrupt)
	defer stop()
	go func() {
		<-ctx.Done()
		restore()
	}()
	fmt.Fprint(stderr, "TypeSafe API key: ")
	secret, err := readSecretLine(file)
	fmt.Fprintln(stderr)
	return secret, err
}
