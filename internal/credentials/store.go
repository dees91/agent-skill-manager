package credentials

import (
	"context"
	"errors"
)

const (
	ServiceName     = "Skill Manager"
	AccountTypeSafe = "typesafe-api-key"
	EnvTypeSafeKey  = "TYPESAFE_API_KEY"
	keychainLabel   = "Skill Manager TypeSafe API key"
)

var (
	ErrNotFound    = errors.New("credential not found")
	ErrUnavailable = errors.New("credential store unavailable")
	ErrDenied      = errors.New("credential store denied")
)

// Store is a narrow secret store. Exists is attributes-only and must not prompt.
type Store interface {
	Kind() string
	Exists(ctx context.Context, account string) (bool, error)
	Get(ctx context.Context, account string) (string, error)
	Set(ctx context.Context, account, secret string) error
	Delete(ctx context.Context, account string) error
}

// System returns the platform credential store.
func System() Store {
	return systemStore()
}
