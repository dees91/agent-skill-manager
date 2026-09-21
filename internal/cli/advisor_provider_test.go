package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dees91/agent-skill-manager/internal/advisor"
	"github.com/dees91/agent-skill-manager/internal/credentials"
	"github.com/dees91/agent-skill-manager/internal/paths"
	"github.com/dees91/agent-skill-manager/internal/typesafe"
)

const sentinelKey = "sk-sentinel-test-key"

func TestAdvisorProviderStatusUsesExistsOnly(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	store := &credentials.Memory{Secrets: map[string]string{credentials.AccountTypeSafe: sentinelKey}}
	app := newApp(p)
	app.credentials = store
	app.lookupEnv = func(string) (string, bool) { return "", false }
	var stdout, stderr strings.Builder
	code := app.Run([]string{"advisor", "provider", "status", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if store.Calls != 1 {
		t.Fatalf("calls = %d, want Exists only", store.Calls)
	}
	if strings.Contains(stdout.String(), sentinelKey) {
		t.Fatalf("leaked key: %s", stdout.String())
	}
}

func TestAdvisorProviderUseAndSetKey(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	store := &credentials.Memory{}
	app := newApp(p)
	app.credentials = store
	app.lookupEnv = func(string) (string, bool) { return "", false }
	app.stdin = strings.NewReader(sentinelKey + "\n")
	var stdout, stderr strings.Builder
	if code := app.Run([]string{"advisor", "provider", "use", "typesafe", "--json"}, &stdout, &stderr); code != 0 {
		t.Fatalf("use code=%d stderr=%s", code, stderr.String())
	}
	info, err := os.Stat(p.AdvisorSettingsFile)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("settings mode err=%v info=%v", err, info)
	}
	stdout.Reset()
	stderr.Reset()
	if code := app.Run([]string{"advisor", "provider", "set-key", "--key-stdin", "--json"}, &stdout, &stderr); code != 0 {
		t.Fatalf("set-key code=%d stderr=%s", code, stderr.String())
	}
	if store.Secrets[credentials.AccountTypeSafe] != sentinelKey {
		t.Fatalf("stored = %q", store.Secrets[credentials.AccountTypeSafe])
	}
	if strings.Contains(stdout.String()+stderr.String(), sentinelKey) {
		t.Fatalf("leaked key")
	}
}

func TestAdvisorProviderSetKeyNonTTY(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	app := newApp(p)
	app.credentials = &credentials.Memory{}
	app.stdin = strings.NewReader(sentinelKey)
	var stdout, stderr strings.Builder
	code := app.Run([]string{"advisor", "provider", "set-key", "--json"}, &stdout, &stderr)
	if code != 1 || !strings.Contains(stderr.String(), "--key-stdin") {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
}

func TestAdvisorProviderRemoveAndCheck(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	store := &credentials.Memory{Secrets: map[string]string{credentials.AccountTypeSafe: sentinelKey}}
	app := newApp(p)
	app.credentials = store
	app.lookupEnv = func(string) (string, bool) { return "", false }
	app.newVerifier = func(typesafe.Key) providerVerifier {
		return verifierStub{result: typesafe.VerifyResult{Model: typesafe.Model, Usage: typesafe.Usage{InputTokens: 4}}}
	}
	var stdout, stderr strings.Builder
	if code := app.Run([]string{"advisor", "provider", "check", "--json"}, &stdout, &stderr); code != 0 {
		t.Fatalf("check code=%d stderr=%s", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := app.Run([]string{"advisor", "provider", "remove", "--json"}, &stdout, &stderr); code != 0 {
		t.Fatalf("remove code=%d stderr=%s", code, stderr.String())
	}
	if _, ok := store.Secrets[credentials.AccountTypeSafe]; ok {
		t.Fatal("key still stored")
	}
	settings, err := advisor.NewSettingsStore(p).Load()
	if err != nil || settings.Provider != advisor.ProviderLocal {
		t.Fatalf("settings = %#v err=%v", settings, err)
	}
	if strings.Contains(stdout.String(), sentinelKey) {
		t.Fatal("leaked key")
	}
	filepath.Walk(p.Home, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		data, _ := os.ReadFile(path)
		if bytes.Contains(data, []byte(sentinelKey)) {
			t.Errorf("key in %s", path)
		}
		return nil
	})
}

func TestAdvisorProviderRemoveResetsWhenStoreUnavailable(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	if err := advisor.NewSettingsStore(p).Save(advisor.Settings{Version: 1, Provider: advisor.ProviderTypeSafe}); err != nil {
		t.Fatal(err)
	}
	app := newApp(p)
	app.credentials = &credentials.Memory{Fail: credentials.ErrUnavailable}
	var stdout, stderr strings.Builder
	if code := app.Run([]string{"advisor", "provider", "remove", "--json"}, &stdout, &stderr); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"removed": false`) {
		t.Fatalf("stdout = %s", stdout.String())
	}
	settings, err := advisor.NewSettingsStore(p).Load()
	if err != nil || settings.Provider != advisor.ProviderLocal {
		t.Fatalf("settings = %#v err=%v", settings, err)
	}
}

func TestAdvisorProviderRemoveDeniedStillResetsProvider(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	if err := advisor.NewSettingsStore(p).Save(advisor.Settings{Version: 1, Provider: advisor.ProviderTypeSafe}); err != nil {
		t.Fatal(err)
	}
	app := newApp(p)
	app.credentials = &credentials.Memory{Fail: credentials.ErrDenied}
	var stdout, stderr strings.Builder
	if code := app.Run([]string{"advisor", "provider", "remove", "--json"}, &stdout, &stderr); code != 1 {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	settings, err := advisor.NewSettingsStore(p).Load()
	if err != nil || settings.Provider != advisor.ProviderLocal {
		t.Fatalf("settings = %#v err=%v", settings, err)
	}
}

func TestAdvisorProviderDenied(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	app := newApp(p)
	app.credentials = &credentials.Memory{Fail: credentials.ErrDenied}
	app.stdin = strings.NewReader(sentinelKey + "\n")
	var stdout, stderr strings.Builder
	code := app.Run([]string{"advisor", "provider", "set-key", "--key-stdin", "--json"}, &stdout, &stderr)
	if code != 1 || strings.Contains(stderr.String(), sentinelKey) {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
}

type verifierStub struct {
	result typesafe.VerifyResult
	err    error
}

func (v verifierStub) Verify(context.Context) (typesafe.VerifyResult, error) {
	return v.result, v.err
}
