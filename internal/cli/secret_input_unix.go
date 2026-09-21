//go:build darwin || linux

package cli

import (
	"os"
	"os/signal"
	"syscall"
)

// watchInterrupt restores the terminal and re-raises SIGINT when Ctrl-C
// arrives at the hidden prompt, so the process exits instead of waiting for
// Enter. The returned stop function must run before a normal return.
func watchInterrupt(restore func()) func() {
	return watchSignals(make(chan os.Signal, 1), restore, func() {
		signal.Reset(os.Interrupt)
		_ = syscall.Kill(os.Getpid(), syscall.SIGINT)
	})
}

func watchSignals(signals chan os.Signal, restore, reraise func()) func() {
	signal.Notify(signals, os.Interrupt)
	done := make(chan struct{})
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		select {
		case <-signals:
			restore()
			reraise()
		case <-done:
		}
	}()
	return func() {
		signal.Stop(signals)
		close(done)
		<-finished
	}
}
