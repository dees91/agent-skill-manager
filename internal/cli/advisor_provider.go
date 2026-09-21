package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/dees91/agent-skill-manager/internal/advisor"
	"github.com/dees91/agent-skill-manager/internal/credentials"
	"github.com/dees91/agent-skill-manager/internal/typesafe"
)

func (a App) runAdvisorProvider(stdout, stderr io.Writer, args []string) int {
	if len(args) == 0 {
		return advisorUsageError(stderr, false, fmt.Errorf("expected advisor provider <status|use|set-key|remove|check>"))
	}
	jsonOutput := containsArgument(args, "--json")
	switch args[0] {
	case "status":
		return a.runProviderStatus(stdout, stderr, args[1:], jsonOutput)
	case "use":
		return a.runProviderUse(stdout, stderr, args[1:], jsonOutput)
	case "set-key":
		return a.runProviderSetKey(stdout, stderr, args[1:], jsonOutput)
	case "remove":
		return a.runProviderRemove(stdout, stderr, args[1:], jsonOutput)
	case "check":
		return a.runProviderCheck(stdout, stderr, args[1:], jsonOutput)
	default:
		return advisorUsageError(stderr, jsonOutput, fmt.Errorf("unknown advisor provider command %q", args[0]))
	}
}

func (a App) runProviderStatus(stdout, stderr io.Writer, args []string, jsonOutput bool) int {
	if err := parseProviderFlags(args, jsonOutput); err != nil {
		return advisorUsageError(stderr, jsonOutput, err)
	}
	settings, err := advisor.NewSettingsStore(a.paths).Load()
	source := "default"
	warning := ""
	mode := advisor.ProviderLocal
	if errors.Is(err, advisor.ErrSettingsCorrupt) {
		warning = err.Error()
	} else if err != nil {
		return advisorCommandError(stderr, jsonOutput, "PROVIDER_FAILED", err, "")
	} else {
		mode = settings.Provider
		if _, statErr := os.Stat(a.paths.AdvisorSettingsFile); statErr == nil {
			source = "settings"
		}
	}
	presence, err := credentials.Resolver{LookupEnv: a.lookupEnv, Store: a.credentials}.Presence(context.Background())
	if err != nil {
		return advisorCommandError(stderr, jsonOutput, "PROVIDER_FAILED", err, "")
	}
	output := map[string]any{
		"apiVersion": advisor.APIVersion,
		"provider": map[string]any{
			"mode":   mode,
			"source": source,
		},
		"key": map[string]any{
			"environment": presence.Environment,
			"stored":      presence.Stored,
		},
		"credentialStore": a.credentialKind(),
		"model":           typesafe.Model,
	}
	if warning != "" {
		output["provider"].(map[string]any)["warning"] = warning
	}
	if jsonOutput {
		return writeAdvisorJSON(stdout, stderr, output)
	}
	fmt.Fprintf(stdout, "Provider: %s (%s)\n", mode, source)
	if warning != "" {
		fmt.Fprintf(stdout, "Warning: %s\n", warning)
	}
	fmt.Fprintf(stdout, "Key environment: %v\n", presence.Environment)
	fmt.Fprintf(stdout, "Key stored: %s\n", presence.Stored)
	fmt.Fprintf(stdout, "Credential store: %s\n", a.credentialKind())
	fmt.Fprintf(stdout, "Model: %s\n", typesafe.Model)
	return 0
}

func (a App) runProviderUse(stdout, stderr io.Writer, args []string, jsonOutput bool) int {
	if len(args) == 0 {
		return advisorUsageError(stderr, jsonOutput, fmt.Errorf("advisor provider use requires local or typesafe"))
	}
	mode, ok := advisor.ParseProviderMode(args[0])
	if !ok {
		return advisorUsageError(stderr, jsonOutput, fmt.Errorf("invalid provider %q", args[0]))
	}
	if err := parseProviderFlags(args[1:], jsonOutput); err != nil {
		return advisorUsageError(stderr, jsonOutput, err)
	}
	if err := advisor.NewSettingsStore(a.paths).Save(advisor.Settings{Version: 1, Provider: mode}); err != nil {
		return advisorCommandError(stderr, jsonOutput, "PROVIDER_FAILED", err, "")
	}
	hint := ""
	if mode == advisor.ProviderTypeSafe {
		presence, _ := credentials.Resolver{LookupEnv: a.lookupEnv, Store: a.credentials}.Presence(context.Background())
		if !presence.Environment && presence.Stored != credentials.StoredPresent {
			hint = "No TypeSafe API key is present. Use advisor provider set-key."
		}
	}
	if jsonOutput {
		payload := map[string]any{"apiVersion": advisor.APIVersion, "provider": map[string]any{"mode": mode}}
		if hint != "" {
			payload["hint"] = hint
		}
		return writeAdvisorJSON(stdout, stderr, payload)
	}
	fmt.Fprintf(stdout, "Provider set to %s.\n", mode)
	if hint != "" {
		fmt.Fprintln(stdout, hint)
	}
	return 0
}

func (a App) runProviderSetKey(stdout, stderr io.Writer, args []string, jsonOutput bool) int {
	keyStdin := false
	filtered := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "--key-stdin" {
			if keyStdin {
				return advisorUsageError(stderr, jsonOutput, fmt.Errorf("--key-stdin may be provided only once"))
			}
			keyStdin = true
			continue
		}
		filtered = append(filtered, arg)
	}
	if err := parseProviderFlags(filtered, jsonOutput); err != nil {
		return advisorUsageError(stderr, jsonOutput, err)
	}
	secret, err := a.readSecret(a.stdin, stderr, keyStdin)
	if err != nil {
		return advisorCommandError(stderr, jsonOutput, "PROVIDER_FAILED", err, "")
	}
	if err := advisor.ValidateAPIKey(secret); err != nil {
		return advisorCommandError(stderr, jsonOutput, "PROVIDER_FAILED", err, "")
	}
	if err := a.credentials.Set(context.Background(), credentials.AccountTypeSafe, strings.TrimSpace(secret)); err != nil {
		return advisorCommandError(stderr, jsonOutput, "PROVIDER_FAILED", redactStoreError(err), "")
	}
	envPresent := environmentKeyPresent(a.lookupEnv)
	if jsonOutput {
		return writeAdvisorJSON(stdout, stderr, map[string]any{"apiVersion": advisor.APIVersion, "stored": true, "environmentKeyPresent": envPresent})
	}
	fmt.Fprintln(stdout, "TypeSafe API key stored.")
	if envPresent {
		fmt.Fprintln(stdout, "TYPESAFE_API_KEY is set and will take precedence.")
	}
	return 0
}

func (a App) runProviderRemove(stdout, stderr io.Writer, args []string, jsonOutput bool) int {
	if err := parseProviderFlags(args, jsonOutput); err != nil {
		return advisorUsageError(stderr, jsonOutput, err)
	}
	if err := a.credentials.Delete(context.Background(), credentials.AccountTypeSafe); err != nil && !errors.Is(err, credentials.ErrNotFound) {
		return advisorCommandError(stderr, jsonOutput, "PROVIDER_FAILED", redactStoreError(err), "")
	}
	if err := advisor.NewSettingsStore(a.paths).Save(advisor.Settings{Version: 1, Provider: advisor.ProviderLocal}); err != nil {
		return advisorCommandError(stderr, jsonOutput, "PROVIDER_FAILED", err, "")
	}
	envPresent := environmentKeyPresent(a.lookupEnv)
	if jsonOutput {
		return writeAdvisorJSON(stdout, stderr, map[string]any{
			"apiVersion":            advisor.APIVersion,
			"removed":               true,
			"provider":              map[string]any{"mode": advisor.ProviderLocal},
			"environmentKeyPresent": envPresent,
		})
	}
	fmt.Fprintln(stdout, "Stored TypeSafe API key removed. Provider set to local.")
	return 0
}

func (a App) runProviderCheck(stdout, stderr io.Writer, args []string, jsonOutput bool) int {
	if err := parseProviderFlags(args, jsonOutput); err != nil {
		return advisorUsageError(stderr, jsonOutput, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resolved, err := credentials.Resolver{LookupEnv: a.lookupEnv, Store: a.credentials}.Resolve(ctx)
	if err != nil {
		return advisorCommandError(stderr, jsonOutput, "PROVIDER_CHECK_FAILED", errors.New(fallbackStoreReason(err)), "")
	}
	if resolved.Source == credentials.SourceNone {
		return advisorCommandError(stderr, jsonOutput, "PROVIDER_CHECK_FAILED", errors.New(advisor.FallbackMissingKey), "")
	}
	result, err := a.newVerifier(typesafe.Key(resolved.Secret)).Verify(ctx)
	if err != nil {
		reason := typesafe.ReasonOf(err)
		if reason == "" {
			reason = typesafe.ReasonProviderError
		}
		return advisorCommandError(stderr, jsonOutput, "PROVIDER_CHECK_FAILED", errors.New(reason), "")
	}
	output := map[string]any{
		"apiVersion": advisor.APIVersion,
		"ok":         true,
		"keySource":  resolved.Source,
		"model":      result.Model,
		"usage":      result.Usage,
	}
	if jsonOutput {
		return writeAdvisorJSON(stdout, stderr, output)
	}
	fmt.Fprintf(stdout, "ok keySource=%s model=%s\n", resolved.Source, result.Model)
	return 0
}

func parseProviderFlags(args []string, _ bool) error {
	jsonCount := 0
	for _, arg := range args {
		if arg == "--json" {
			jsonCount++
			continue
		}
		return fmt.Errorf("unknown advisor provider argument %q", arg)
	}
	if jsonCount > 1 {
		return fmt.Errorf("--json may be provided only once")
	}
	return nil
}

func (a App) credentialKind() string {
	if a.credentials == nil {
		return "unavailable"
	}
	return a.credentials.Kind()
}

func environmentKeyPresent(lookup func(string) (string, bool)) bool {
	if lookup == nil {
		return false
	}
	raw, ok := lookup(credentials.EnvTypeSafeKey)
	return ok && strings.TrimSpace(raw) != ""
}

func redactStoreError(err error) error {
	if errors.Is(err, credentials.ErrDenied) {
		return credentials.ErrDenied
	}
	if errors.Is(err, credentials.ErrUnavailable) {
		return credentials.ErrUnavailable
	}
	return err
}

func fallbackStoreReason(err error) string {
	if errors.Is(err, credentials.ErrDenied) {
		return advisor.FallbackStoreDenied
	}
	if errors.Is(err, credentials.ErrUnavailable) {
		return advisor.FallbackStoreUnavailable
	}
	return typesafe.ReasonProviderError
}
