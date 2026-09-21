package credentials

import (
	"context"
	"errors"
	"strings"
)

const (
	SourceEnvironment = "environment"
	SourceStored      = "stored"
	SourceNone        = "none"
	StoredPresent     = "present"
	StoredAbsent      = "absent"
	StoredUnavailable = "unavailable"
	StoredDenied      = "denied"
)

// Resolver reads the environment first, then the credential store.
type Resolver struct {
	LookupEnv func(string) (string, bool)
	Store     Store
}

// Resolved is a secret lookup that never writes.
type Resolved struct {
	Secret string
	Source string
}

// Presence reports whether a key exists without reading the secret.
type Presence struct {
	Environment bool
	Stored      string
}

// Resolve prefers a trimmed non-blank environment value and never writes.
func (r Resolver) Resolve(ctx context.Context) (Resolved, error) {
	if secret, ok := r.environmentSecret(); ok {
		return Resolved{Secret: secret, Source: SourceEnvironment}, nil
	}
	if r.Store == nil {
		return Resolved{Source: SourceNone}, nil
	}
	secret, err := r.Store.Get(ctx, AccountTypeSafe)
	if errors.Is(err, ErrNotFound) {
		return Resolved{Source: SourceNone}, nil
	}
	if err != nil {
		return Resolved{}, err
	}
	return Resolved{Secret: secret, Source: SourceStored}, nil
}

// Presence uses Exists only and never reads the secret.
func (r Resolver) Presence(ctx context.Context) (Presence, error) {
	presence := Presence{Stored: StoredAbsent}
	_, presence.Environment = r.environmentSecret()
	if r.Store == nil {
		return presence, nil
	}
	exists, err := r.Store.Exists(ctx, AccountTypeSafe)
	if errors.Is(err, ErrUnavailable) {
		presence.Stored = StoredUnavailable
		return presence, nil
	}
	if errors.Is(err, ErrDenied) {
		presence.Stored = StoredDenied
		return presence, nil
	}
	if err != nil {
		return Presence{}, err
	}
	if exists {
		presence.Stored = StoredPresent
	}
	return presence, nil
}

func (r Resolver) environmentSecret() (string, bool) {
	if r.LookupEnv == nil {
		return "", false
	}
	raw, ok := r.LookupEnv(EnvTypeSafeKey)
	if !ok {
		return "", false
	}
	secret := strings.TrimSpace(raw)
	if secret == "" {
		return "", false
	}
	return secret, true
}
