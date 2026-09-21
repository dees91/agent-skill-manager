package gui

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

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
	if _, err := service.SaveAdvisorProvider("typesafe"); err != nil {
		t.Fatal(err)
	}
	view, err := service.SetAdvisorKey(guiSentinelKey)
	if err != nil {
		t.Fatal(err)
	}
	if view.StoredKey != credentials.StoredPresent || store.Secrets[credentials.AccountTypeSafe] != guiSentinelKey {
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

type guiVerifier struct {
	result typesafe.VerifyResult
}

func (g guiVerifier) Verify(context.Context) (typesafe.VerifyResult, error) {
	return g.result, nil
}
