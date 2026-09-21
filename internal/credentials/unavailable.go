//go:build !darwin || !cgo

package credentials

import "context"

func systemStore() Store {
	return unavailableStore{}
}

type unavailableStore struct{}

func (unavailableStore) Kind() string { return "unavailable" }

func (unavailableStore) Exists(context.Context, string) (bool, error) {
	return false, ErrUnavailable
}

func (unavailableStore) Get(context.Context, string) (string, error) {
	return "", ErrUnavailable
}

func (unavailableStore) Set(context.Context, string, string) error {
	return ErrUnavailable
}

func (unavailableStore) Delete(context.Context, string) error {
	return ErrUnavailable
}
