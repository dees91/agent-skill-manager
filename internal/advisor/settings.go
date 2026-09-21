package advisor

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"

	"github.com/dees91/agent-skill-manager/internal/paths"
	"github.com/dees91/agent-skill-manager/internal/state"
)

const settingsVersion = 1

// ProviderMode is the saved recommendation provider preference.
type ProviderMode string

const (
	ProviderLocal    ProviderMode = "local"
	ProviderTypeSafe ProviderMode = "typesafe"
)

// ErrSettingsCorrupt is a path-free failure to read advisor-settings.json.
var ErrSettingsCorrupt = errors.New("advisor settings are unreadable")

// Settings is the on-disk advisor preference file.
type Settings struct {
	Version  int          `json:"version"`
	Provider ProviderMode `json:"provider"`
}

// DefaultSettings is first-run local mode.
func DefaultSettings() Settings {
	return Settings{Version: settingsVersion, Provider: ProviderLocal}
}

// ParseProviderMode accepts local or typesafe.
func ParseProviderMode(raw string) (ProviderMode, bool) {
	switch ProviderMode(strings.TrimSpace(raw)) {
	case ProviderLocal:
		return ProviderLocal, true
	case ProviderTypeSafe:
		return ProviderTypeSafe, true
	default:
		return "", false
	}
}

// SettingsStore persists advisor-settings.json with owner-only atomic writes.
type SettingsStore struct {
	paths paths.Paths
}

// NewSettingsStore creates a settings store for the provided paths.
func NewSettingsStore(p paths.Paths) SettingsStore {
	return SettingsStore{paths: p}
}

// Load returns defaults when the file is missing. Corrupt files stay path-free.
func (s SettingsStore) Load() (Settings, error) {
	if err := secureRegularFile(s.paths.AdvisorSettingsFile); err != nil {
		return Settings{}, ErrSettingsCorrupt
	}
	data, err := os.ReadFile(s.paths.AdvisorSettingsFile)
	if errors.Is(err, os.ErrNotExist) {
		return DefaultSettings(), nil
	}
	if err != nil {
		return Settings{}, ErrSettingsCorrupt
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	var decoded Settings
	if err := decoder.Decode(&decoded); err != nil {
		return Settings{}, ErrSettingsCorrupt
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return Settings{}, ErrSettingsCorrupt
	}
	if decoded.Version != settingsVersion {
		return Settings{}, ErrSettingsCorrupt
	}
	if _, ok := ParseProviderMode(string(decoded.Provider)); !ok {
		return Settings{}, ErrSettingsCorrupt
	}
	return decoded, nil
}

// Save overwrites advisor-settings.json without reading the previous file.
func (s SettingsStore) Save(contents Settings) error {
	if contents.Version != settingsVersion {
		return ErrSettingsCorrupt
	}
	if _, ok := ParseProviderMode(string(contents.Provider)); !ok {
		return ErrSettingsCorrupt
	}
	if err := state.New(s.paths).Secure(); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(s.paths.StateDir, "advisor-settings-*.json")
	if err != nil {
		return fmt.Errorf("create temporary advisor settings file: %w", err)
	}
	temporaryPath := temporary.Name()
	removeTemporary := true
	defer func() {
		if removeTemporary {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(privateFileMode); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("secure temporary advisor settings file: %w", err)
	}
	encoder := json.NewEncoder(temporary)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(contents); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("encode advisor settings: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary advisor settings file: %w", err)
	}
	if err := os.Rename(temporaryPath, s.paths.AdvisorSettingsFile); err != nil {
		return fmt.Errorf("replace advisor settings: %w", err)
	}
	if err := os.Chmod(s.paths.AdvisorSettingsFile, privateFileMode); err != nil {
		return fmt.Errorf("secure advisor settings: %w", err)
	}
	removeTemporary = false
	return nil
}

// ValidateAPIKey accepts a trimmed printable ASCII key of 8 to 512 bytes.
func ValidateAPIKey(raw string) error {
	key := strings.TrimSpace(raw)
	if len(key) < 8 || len(key) > 512 {
		return errors.New("API key must be 8 to 512 bytes")
	}
	for _, r := range key {
		if r < 32 || r > 126 || unicode.IsSpace(r) {
			return errors.New("API key must be printable ASCII")
		}
	}
	return nil
}
