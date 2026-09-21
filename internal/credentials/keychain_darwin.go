//go:build darwin && cgo

package credentials

import (
	"context"

	"github.com/keybase/go-keychain"
)

func systemStore() Store {
	return darwinStore{}
}

type darwinStore struct{}

func (darwinStore) Kind() string { return "keychain" }

func (darwinStore) Exists(ctx context.Context, account string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	query := genericQuery(account)
	query.SetMatchLimit(keychain.MatchLimitOne)
	query.SetReturnAttributes(true)
	results, err := keychain.QueryItem(query)
	if err != nil {
		return false, mapKeychainError(err)
	}
	return len(results) > 0, nil
}

func (darwinStore) Get(ctx context.Context, account string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	query := genericQuery(account)
	query.SetMatchLimit(keychain.MatchLimitOne)
	query.SetReturnData(true)
	results, err := keychain.QueryItem(query)
	if err != nil {
		return "", mapKeychainError(err)
	}
	if len(results) == 0 {
		return "", ErrNotFound
	}
	return string(results[0].Data), nil
}

func (darwinStore) Set(ctx context.Context, account, secret string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	item := genericQuery(account)
	item.SetLabel(keychainLabel)
	item.SetData([]byte(secret))
	item.SetSynchronizable(keychain.SynchronizableNo)
	item.SetAccessible(keychain.AccessibleWhenUnlocked)
	if err := keychain.AddItem(item); err != keychain.ErrorDuplicateItem {
		return mapKeychainError(err)
	}
	// Replace only the secret in place, so a failed update keeps the
	// previous key instead of leaving the user with none.
	update := keychain.NewItem()
	update.SetLabel(keychainLabel)
	update.SetData([]byte(secret))
	return mapKeychainError(keychain.UpdateItem(genericQuery(account), update))
}

func (darwinStore) Delete(ctx context.Context, account string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	item := genericQuery(account)
	if err := keychain.DeleteItem(item); err != nil {
		return mapKeychainError(err)
	}
	return nil
}

func genericQuery(account string) keychain.Item {
	item := keychain.NewItem()
	item.SetSecClass(keychain.SecClassGenericPassword)
	item.SetService(ServiceName)
	item.SetAccount(account)
	return item
}

func mapKeychainError(err error) error {
	if err == nil {
		return nil
	}
	if err == keychain.ErrorItemNotFound {
		return ErrNotFound
	}
	switch err {
	case keychain.ErrorUserCanceled, keychain.ErrorAuthFailed, keychain.ErrorInteractionNotAllowed, keychain.ErrorNotAvailable, keychain.ErrorNoAccessForItem:
		return ErrDenied
	default:
		return ErrDenied
	}
}
