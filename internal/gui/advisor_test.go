package gui

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/dees91/agent-skill-manager/internal/advisor"
	"github.com/dees91/agent-skill-manager/internal/credentials"
	"github.com/dees91/agent-skill-manager/internal/paths"
	"github.com/dees91/agent-skill-manager/internal/typesafe"
)

const guiSentinelKey = "sk-gui-sentinel-key"

func TestGetAdvisorSettingsDefault(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	service := New(p)
	service.credentials = &credentials.Memory{}
	service.lookupEnv = func(string) (string, bool) { return "", false }
	view, err := service.GetAdvisorSettings()
	if err != nil {
		t.Fatal(err)
	}
	if view.Mode != "local" || view.EnvironmentKey || view.StoredKey != credentials.StoredAbsent || view.Model != typesafe.Model {
		t.Fatalf("view = %#v", view)
	}
}

func TestGetSnapshotIgnoresDeniedStoreAndPanickingVerifier(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	store := &credentials.Memory{Fail: credentials.ErrDenied}
	service := New(p)
	service.credentials = store
	service.newVerifier = func(typesafe.Key) advisorVerifier { panic("verifier") }
	if _, err := service.GetSnapshot(false); err != nil {
		t.Fatal(err)
	}
	if store.Calls != 0 {
		t.Fatalf("store calls = %d", store.Calls)
	}
}

func TestAdvisorKeyLifecycle(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	store := &credentials.Memory{}
	service := New(p)
	service.credentials = store
	service.lookupEnv = func(string) (string, bool) { return "", false }
	saved, err := service.SaveAdvisorProvider("typesafe")
	if err != nil || saved.Mode != "typesafe" {
		t.Fatalf("save provider = %#v err=%v", saved, err)
	}
	view, err := service.SetAdvisorKey(guiSentinelKey)
	if err != nil {
		t.Fatal(err)
	}
	if view.Mode != "typesafe" || view.StoredKey != credentials.StoredPresent || store.Secrets[credentials.AccountTypeSafe] != guiSentinelKey {
		t.Fatalf("view = %#v secrets=%v", view, store.Secrets)
	}
	encoded, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), guiSentinelKey) {
		t.Fatalf("view JSON leaked key: %s", encoded)
	}
	removed, err := service.RemoveAdvisorKey()
	if err != nil {
		t.Fatal(err)
	}
	if removed.Mode != "local" || removed.StoredKey != credentials.StoredAbsent {
		t.Fatalf("removed = %#v", removed)
	}
}

func TestCheckAdvisorConnectionAndDeniedStore(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	service := New(p)
	service.credentials = &credentials.Memory{Secrets: map[string]string{credentials.AccountTypeSafe: guiSentinelKey}}
	service.lookupEnv = func(string) (string, bool) { return "", false }
	service.newVerifier = func(key typesafe.Key) advisorVerifier {
		if string(key) != guiSentinelKey {
			t.Fatalf("key = %q", key)
		}
		return guiVerifier{result: typesafe.VerifyResult{Usage: typesafe.Usage{InputTokens: 5, OutputTokens: 1}}}
	}
	check, err := service.CheckAdvisorConnection()
	if err != nil || !check.OK || check.InputTokens != 5 {
		t.Fatalf("check = %#v err=%v", check, err)
	}
	service.credentials = &credentials.Memory{Fail: credentials.ErrDenied}
	denied, err := service.SetAdvisorKey(guiSentinelKey)
	if err == nil || strings.Contains(err.Error(), guiSentinelKey) {
		t.Fatalf("denied = %#v err=%v", denied, err)
	}
}

func TestCheckAdvisorConnectionFailureReasons(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	service := New(p)
	service.lookupEnv = func(string) (string, bool) { return "", false }

	service.credentials = &credentials.Memory{}
	missing, err := service.CheckAdvisorConnection()
	if err != nil || missing.OK || missing.Reason != advisor.FallbackMissingKey {
		t.Fatalf("missing = %#v err=%v", missing, err)
	}

	service.credentials = &credentials.Memory{Fail: credentials.ErrDenied}
	denied, err := service.CheckAdvisorConnection()
	if err != nil || denied.OK || denied.Reason != advisor.FallbackStoreDenied {
		t.Fatalf("denied = %#v err=%v", denied, err)
	}

	service.credentials = &credentials.Memory{Secrets: map[string]string{credentials.AccountTypeSafe: guiSentinelKey}}
	service.newVerifier = func(typesafe.Key) advisorVerifier {
		return guiVerifier{err: &typesafe.Error{Reason: typesafe.ReasonInvalidKey, Status: 401}}
	}
	invalid, err := service.CheckAdvisorConnection()
	if err != nil || invalid.OK || invalid.Reason != typesafe.ReasonInvalidKey {
		t.Fatalf("invalid = %#v err=%v", invalid, err)
	}
}

func TestRemoveAdvisorKeyWhenAbsent(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	service := New(p)
	service.credentials = &credentials.Memory{}
	service.lookupEnv = func(string) (string, bool) { return "", false }
	view, err := service.RemoveAdvisorKey()
	if err != nil || view.Mode != "local" || view.StoredKey != credentials.StoredAbsent {
		t.Fatalf("remove absent = %#v err=%v", view, err)
	}
}

func TestRemoveAdvisorKeyDeniedStillResetsProvider(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	service := New(p)
	service.lookupEnv = func(string) (string, bool) { return "", false }
	service.credentials = &credentials.Memory{}
	if _, err := service.SaveAdvisorProvider("typesafe"); err != nil {
		t.Fatal(err)
	}
	service.credentials = &credentials.Memory{Fail: credentials.ErrDenied}
	if _, err := service.RemoveAdvisorKey(); !errors.Is(err, credentials.ErrDenied) {
		t.Fatalf("remove err = %v", err)
	}
	settings, err := advisor.NewSettingsStore(p).Load()
	if err != nil || settings.Provider != advisor.ProviderLocal {
		t.Fatalf("settings = %#v err=%v", settings, err)
	}
}

type guiVerifier struct {
	result typesafe.VerifyResult
	err    error
}

func (g guiVerifier) Verify(context.Context) (typesafe.VerifyResult, error) {
	return g.result, g.err
}
