//go:build darwin || linux

package cli

import (
	"os"
	"testing"
)

func TestWatchSignalsStopDoesNotInterrupt(t *testing.T) {
	var restored, reraised bool
	stop := watchSignals(make(chan os.Signal, 1), func() { restored = true }, func() { reraised = true })
	stop()
	if restored || reraised {
		t.Fatalf("restored=%v reraised=%v after a normal return", restored, reraised)
	}
}

func TestWatchSignalsRestoresBeforeReraise(t *testing.T) {
	signals := make(chan os.Signal, 1)
	var calls []string
	stop := watchSignals(signals, func() { calls = append(calls, "restore") }, func() { calls = append(calls, "reraise") })
	signals <- os.Interrupt
	stop()
	if len(calls) != 2 || calls[0] != "restore" || calls[1] != "reraise" {
		t.Fatalf("calls = %v", calls)
	}
}
