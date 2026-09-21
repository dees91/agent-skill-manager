//go:build darwin && cgo

package credentials

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"testing"
)

func TestSystemKindKeychain(t *testing.T) {
	if got := System().Kind(); got != "keychain" {
		t.Fatalf("Kind() = %q", got)
	}
}

func TestDarwinKeychainRoundTrip(t *testing.T) {
	if os.Getenv("SKILL_MANAGER_KEYCHAIN_TEST") != "1" {
		t.Skip("set SKILL_MANAGER_KEYCHAIN_TEST=1 to exercise the macOS keychain")
	}
	account := fmt.Sprintf("typesafe-api-key-test-%d", os.Getpid())
	store := System()
	ctx := context.Background()
	t.Cleanup(func() {
		_ = store.Delete(ctx, account)
	})

	secret := "synthetic-test-secret-not-a-real-key"
	if err := store.Set(ctx, account, secret); err != nil {
		t.Fatalf("set: %v", err)
	}
	exists, err := store.Exists(ctx, account)
	if err != nil || !exists {
		t.Fatalf("exists = %v err=%v", exists, err)
	}
	got, err := store.Get(ctx, account)
	if err != nil || got != secret {
		t.Fatalf("get = %q err=%v", got, err)
	}
	if err := store.Delete(ctx, account); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := store.Get(ctx, account); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get after delete err = %v", err)
	}

	cmd := exec.Command("security", "find-generic-password", "-s", ServiceName, "-a", account)
	if err := cmd.Run(); err == nil {
		t.Fatalf("security still found %s", account)
	}
}
