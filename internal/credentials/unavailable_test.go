//go:build !darwin || !cgo

package credentials

import "testing"

func TestSystemKindUnavailable(t *testing.T) {
	if got := System().Kind(); got != "unavailable" {
		t.Fatalf("Kind() = %q", got)
	}
}
