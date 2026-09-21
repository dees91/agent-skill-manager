package gui

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/dees91/agent-skill-manager/internal/advisor"
	"github.com/dees91/agent-skill-manager/internal/credentials"
	"github.com/dees91/agent-skill-manager/internal/typesafe"
)

type advisorVerifier interface {
	Verify(ctx context.Context) (typesafe.VerifyResult, error)
}

// AdvisorSettingsView is a path-free, secret-free provider configuration.
type AdvisorSettingsView struct {
	Mode            string `json:"mode"`
	SettingsWarning string `json:"settingsWarning,omitempty"`
	EnvironmentKey  bool   `json:"environmentKey"`
	StoredKey       string `json:"storedKey"`
	CredentialStore string `json:"credentialStore"`
	Model           string `json:"model"`
}

// AdvisorConnectionCheck is the result of an explicit connectivity test.
type AdvisorConnectionCheck struct {
	OK           bool   `json:"ok"`
	Reason       string `json:"reason,omitempty"`
	KeySource    string `json:"keySource"`
	InputTokens  int    `json:"inputTokens"`
	OutputTokens int    `json:"outputTokens"`
}

// GetAdvisorSettings reports provider mode and key presence without reading the secret.
func (s *Service) GetAdvisorSettings() (AdvisorSettingsView, error) {
	view := AdvisorSettingsView{
		Mode:            string(advisor.ProviderLocal),
		StoredKey:       credentials.StoredAbsent,
		CredentialStore: s.credentialStore().Kind(),
		Model:           typesafe.Model,
	}
	settings, err := s.settingsStore().Load()
	if errors.Is(err, advisor.ErrSettingsCorrupt) {
		view.SettingsWarning = err.Error()
	} else if err != nil {
		return AdvisorSettingsView{}, err
	} else {
		view.Mode = string(settings.Provider)
	}
	presence, err := credentials.Resolver{LookupEnv: s.envLookup(), Store: s.credentialStore()}.Presence(context.Background())
	if err != nil {
		return AdvisorSettingsView{}, err
	}
	view.EnvironmentKey = presence.Environment
	view.StoredKey = presence.Stored
	return view, nil
}

// SaveAdvisorProvider records local or typesafe consent.
func (s *Service) SaveAdvisorProvider(mode string) (AdvisorSettingsView, error) {
	parsed, ok := advisor.ParseProviderMode(mode)
	if !ok {
		return AdvisorSettingsView{}, errors.New("invalid provider")
	}
	s.advisorMu.Lock()
	err := s.settingsStore().Save(advisor.Settings{Version: 1, Provider: parsed})
	s.advisorMu.Unlock()
	if err != nil {
		return AdvisorSettingsView{}, err
	}
	return s.GetAdvisorSettings()
}

// SetAdvisorKey stores a validated API key and does not retain it.
func (s *Service) SetAdvisorKey(key string) (AdvisorSettingsView, error) {
	if err := advisor.ValidateAPIKey(key); err != nil {
		return AdvisorSettingsView{}, err
	}
	s.advisorMu.Lock()
	err := s.credentialStore().Set(context.Background(), credentials.AccountTypeSafe, strings.TrimSpace(key))
	s.advisorMu.Unlock()
	if err != nil {
		return AdvisorSettingsView{}, err
	}
	return s.GetAdvisorSettings()
}

// RemoveAdvisorKey deletes the stored key and returns the provider to local.
func (s *Service) RemoveAdvisorKey() (AdvisorSettingsView, error) {
	s.advisorMu.Lock()
	err := s.credentialStore().Delete(context.Background(), credentials.AccountTypeSafe)
	if err != nil && !errors.Is(err, credentials.ErrNotFound) {
		s.advisorMu.Unlock()
		return AdvisorSettingsView{}, err
	}
	err = s.settingsStore().Save(advisor.Settings{Version: 1, Provider: advisor.ProviderLocal})
	s.advisorMu.Unlock()
	if err != nil {
		return AdvisorSettingsView{}, err
	}
	return s.GetAdvisorSettings()
}

// CheckAdvisorConnection verifies the resolved key with a cheap TypeSafe ping.
func (s *Service) CheckAdvisorConnection() (AdvisorConnectionCheck, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resolved, err := credentials.Resolver{LookupEnv: s.envLookup(), Store: s.credentialStore()}.Resolve(ctx)
	if err != nil {
		reason := advisor.FallbackStoreDenied
		if errors.Is(err, credentials.ErrUnavailable) {
			reason = advisor.FallbackStoreUnavailable
		}
		return AdvisorConnectionCheck{OK: false, Reason: reason}, nil
	}
	if resolved.Source == credentials.SourceNone {
		return AdvisorConnectionCheck{OK: false, Reason: advisor.FallbackMissingKey}, nil
	}
	result, err := s.verifierFactory()(typesafe.Key(resolved.Secret)).Verify(ctx)
	if err != nil {
		reason := typesafe.ReasonOf(err)
		if reason == "" {
			reason = typesafe.ReasonProviderError
		}
		return AdvisorConnectionCheck{OK: false, Reason: reason, KeySource: resolved.Source}, nil
	}
	return AdvisorConnectionCheck{
		OK:           true,
		KeySource:    resolved.Source,
		InputTokens:  result.Usage.InputTokens,
		OutputTokens: result.Usage.OutputTokens,
	}, nil
}

func (s *Service) settingsStore() advisor.SettingsStore {
	return s.advisorSettings
}

func (s *Service) credentialStore() credentials.Store {
	if s.credentials == nil {
		s.credentials = credentials.System()
	}
	return s.credentials
}

func (s *Service) envLookup() func(string) (string, bool) {
	if s.lookupEnv == nil {
		return os.LookupEnv
	}
	return s.lookupEnv
}

func (s *Service) verifierFactory() func(typesafe.Key) advisorVerifier {
	if s.newVerifier == nil {
		return func(key typesafe.Key) advisorVerifier { return typesafe.New(key) }
	}
	return s.newVerifier
}
