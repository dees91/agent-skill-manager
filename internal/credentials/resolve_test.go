package credentials

import (
	"context"
	"errors"
	"testing"
)

func TestResolveEnvironmentWinsWithoutTouchingStore(t *testing.T) {
	store := &Memory{Secrets: map[string]string{AccountTypeSafe: "stored-secret"}}
	resolver := Resolver{
		LookupEnv: func(key string) (string, bool) {
			if key != EnvTypeSafeKey {
				t.Fatalf("lookup %q", key)
			}
			return "  env-secret  ", true
		},
		Store: store,
	}
	resolved, err := resolver.Resolve(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Source != SourceEnvironment || resolved.Secret != "env-secret" {
		t.Fatalf("resolved = %#v", resolved)
	}
	if store.Calls != 0 {
		t.Fatalf("store calls = %d", store.Calls)
	}
}

func TestResolveBlankEnvironmentFallsThrough(t *testing.T) {
	store := &Memory{Secrets: map[string]string{AccountTypeSafe: "stored-secret"}}
	resolver := Resolver{
		LookupEnv: func(string) (string, bool) { return "  ", true },
		Store:     store,
	}
	resolved, err := resolver.Resolve(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Source != SourceStored || resolved.Secret != "stored-secret" {
		t.Fatalf("resolved = %#v", resolved)
	}
	if store.Calls != 1 {
		t.Fatalf("store calls = %d", store.Calls)
	}
}

func TestResolveMissingStoredIsNone(t *testing.T) {
	resolver := Resolver{Store: &Memory{}}
	resolved, err := resolver.Resolve(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Source != SourceNone || resolved.Secret != "" {
		t.Fatalf("resolved = %#v", resolved)
	}
}

func TestResolveDeniedAndUnavailablePropagate(t *testing.T) {
	for _, want := range []error{ErrDenied, ErrUnavailable} {
		resolver := Resolver{Store: &Memory{Fail: want}}
		_, err := resolver.Resolve(context.Background())
		if !errors.Is(err, want) {
			t.Fatalf("err = %v want %v", err, want)
		}
	}
}

func TestPresenceUsesExistsOnly(t *testing.T) {
	store := &Memory{Secrets: map[string]string{AccountTypeSafe: "stored-secret"}}
	resolver := Resolver{
		LookupEnv: func(string) (string, bool) { return "env", true },
		Store:     store,
	}
	presence, err := resolver.Presence(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !presence.Environment || presence.Stored != StoredPresent {
		t.Fatalf("presence = %#v", presence)
	}
	if store.Calls != 1 {
		t.Fatalf("store calls = %d", store.Calls)
	}
	if _, ok := store.Secrets[AccountTypeSafe]; !ok {
		t.Fatal("secret was consumed")
	}
}

func TestPresenceMapsStoreErrors(t *testing.T) {
	unavailable, err := Resolver{Store: &Memory{Fail: ErrUnavailable}}.Presence(context.Background())
	if err != nil || unavailable.Stored != StoredUnavailable {
		t.Fatalf("unavailable = %#v err=%v", unavailable, err)
	}
	denied, err := Resolver{Store: &Memory{Fail: ErrDenied}}.Presence(context.Background())
	if err != nil || denied.Stored != StoredDenied {
		t.Fatalf("denied = %#v err=%v", denied, err)
	}
	absent, err := Resolver{Store: &Memory{}}.Presence(context.Background())
	if err != nil || absent.Stored != StoredAbsent || absent.Environment {
		t.Fatalf("absent = %#v err=%v", absent, err)
	}
}
