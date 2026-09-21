package advisor

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dees91/agent-skill-manager/internal/paths"
)

func TestLoadMissingSettingsReturnsDefault(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	got, err := NewSettingsStore(p).Load()
	if err != nil {
		t.Fatal(err)
	}
	if got != DefaultSettings() || got.Provider != ProviderLocal {
		t.Fatalf("got = %#v", got)
	}
}

func TestSettingsRoundTripOwnerOnlyAndCleansTemp(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	store := NewSettingsStore(p)
	if err := store.Save(Settings{Version: 1, Provider: ProviderTypeSafe}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(p.AdvisorSettingsFile)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o", info.Mode().Perm())
	}
	got, err := store.Load()
	if err != nil || got.Provider != ProviderTypeSafe {
		t.Fatalf("load = %#v err=%v", got, err)
	}
	matches, err := filepath.Glob(filepath.Join(p.StateDir, "advisor-settings-*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temp leftovers = %v", matches)
	}
}

func TestLoadRejectsCorruptSettingsWithoutHomePath(t *testing.T) {
	cases := map[string]string{
		"unknown-field": `{"version":1,"provider":"local","extra":true}`,
		"trailing":      `{"version":1,"provider":"local"}{"x":1}`,
		"version":       `{"version":2,"provider":"local"}`,
		"provider":      `{"version":1,"provider":"cloud"}`,
		"truncated":     `{"version":1,`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			p := paths.ForHome(t.TempDir())
			if err := os.MkdirAll(p.StateDir, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p.AdvisorSettingsFile, []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := NewSettingsStore(p).Load()
			if !errors.Is(err, ErrSettingsCorrupt) {
				t.Fatalf("err = %v", err)
			}
			if strings.Contains(err.Error(), p.Home) {
				t.Fatalf("leaked home: %v", err)
			}
		})
	}
}

func TestLoadRejectsSymlink(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	if err := os.MkdirAll(p.StateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(p.StateDir, "other.json")
	if err := os.WriteFile(target, []byte(`{"version":1,"provider":"local"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, p.AdvisorSettingsFile); err != nil {
		t.Fatal(err)
	}
	_, err := NewSettingsStore(p).Load()
	if !errors.Is(err, ErrSettingsCorrupt) {
		t.Fatalf("err = %v", err)
	}
	if strings.Contains(err.Error(), p.Home) {
		t.Fatalf("leaked home: %v", err)
	}
}

func TestSaveOverCorruptFile(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	if err := os.MkdirAll(p.StateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.AdvisorSettingsFile, []byte(`{`), 0o600); err != nil {
		t.Fatal(err)
	}
	store := NewSettingsStore(p)
	if err := store.Save(Settings{Version: 1, Provider: ProviderLocal}); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load()
	if err != nil || got.Provider != ProviderLocal {
		t.Fatalf("load = %#v err=%v", got, err)
	}
}

func TestValidateAPIKeyBounds(t *testing.T) {
	if err := ValidateAPIKey("short"); err == nil {
		t.Fatal("expected short key error")
	}
	if err := ValidateAPIKey("validkey"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateAPIKey("bad key\n"); err == nil {
		t.Fatal("expected control character error")
	}
}

func TestSettingsJSONOmitsUnknownShape(t *testing.T) {
	encoded, err := json.Marshal(DefaultSettings())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"provider":"local"`) {
		t.Fatalf("encoded = %s", encoded)
	}
}
