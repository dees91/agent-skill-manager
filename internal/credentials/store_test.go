package credentials

import (
	"context"
	"errors"
	"testing"
)

func TestMemoryRoundTrip(t *testing.T) {
	store := &Memory{}
	ctx := context.Background()
	if err := store.Set(ctx, AccountTypeSafe, "secret-one"); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get(ctx, AccountTypeSafe)
	if err != nil || got != "secret-one" {
		t.Fatalf("get = %q err=%v", got, err)
	}
	exists, err := store.Exists(ctx, AccountTypeSafe)
	if err != nil || !exists {
		t.Fatalf("exists = %v err=%v", exists, err)
	}
	if err := store.Delete(ctx, AccountTypeSafe); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(ctx, AccountTypeSafe); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted get err = %v", err)
	}
}

func TestMemoryFailPropagates(t *testing.T) {
	store := &Memory{Fail: ErrDenied}
	if _, err := store.Exists(context.Background(), AccountTypeSafe); !errors.Is(err, ErrDenied) {
		t.Fatalf("exists err = %v", err)
	}
	if _, err := store.Get(context.Background(), AccountTypeSafe); !errors.Is(err, ErrDenied) {
		t.Fatalf("get err = %v", err)
	}
}
