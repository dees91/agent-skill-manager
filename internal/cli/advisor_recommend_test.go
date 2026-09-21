package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dees91/agent-skill-manager/internal/advisor"
	"github.com/dees91/agent-skill-manager/internal/credentials"
	"github.com/dees91/agent-skill-manager/internal/paths"
	"github.com/dees91/agent-skill-manager/internal/typesafe"
)

func TestAdvisorRecommendArgumentErrorsOmitBrief(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	app := newApp(p)
	app.stdin = strings.NewReader("secret-brief-text")
	var stdout, stderr strings.Builder
	code := app.Run([]string{"advisor", "recommend", "--tool", "codex", "--json"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("code = %d", code)
	}
	if strings.Contains(stderr.String(), "secret-brief-text") {
		t.Fatalf("leaked brief: %s", stderr.String())
	}
}

func TestAdvisorRecommendLocalDefault(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	writeSearchSkill(t, filepath.Join(p.CodexUserSkills, "video-encode"), "Encodes video files with ffmpeg.")
	app := newApp(p)
	app.stdin = strings.NewReader("Encode a local video file.")
	app.newProvider = func(typesafe.Key) advisor.Provider { t.Fatal("provider"); return nil }
	var stdout, stderr strings.Builder
	code := app.Run([]string{"advisor", "recommend", "--tool", "codex", "--query", "video encode ffmpeg", "--task-stdin", "--json"}, &stdout, &stderr)
	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("code=%d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	var output advisorRecommendOutput
	if err := json.Unmarshal([]byte(stdout.String()), &output); err != nil {
		t.Fatal(err)
	}
	if output.Outcome != advisor.OutcomeLocalCandidates || output.UsedProvider != "local" || len(output.Candidates) == 0 {
		t.Fatalf("output = %#v", output)
	}
}

func TestAdvisorRecommendTypesafeFlagUsesProvider(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	writeSearchSkill(t, filepath.Join(p.CodexUserSkills, "video-encode"), "Encodes video files with ffmpeg.")
	app := newApp(p)
	app.stdin = strings.NewReader("Encode a local video file with ffmpeg.")
	app.newProvider = func(typesafe.Key) advisor.Provider {
		return recommendStub{handler: func(req typesafe.Request) (typesafe.Response, error) {
			return noulOK(req), nil
		}}
	}
	app.lookupEnv = func(string) (string, bool) { return "env-key-value", true }
	var stdout, stderr strings.Builder
	code := app.Run([]string{"advisor", "recommend", "--tool", "codex", "--query", "video encode ffmpeg", "--task-stdin", "--provider", "typesafe", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	var output advisorRecommendOutput
	if err := json.Unmarshal([]byte(stdout.String()), &output); err != nil {
		t.Fatal(err)
	}
	if output.Outcome != advisor.OutcomeRecommended || output.UsedProvider != "typesafe" {
		t.Fatalf("output = %#v", output)
	}
}

func TestAdvisorRecommendSavedTypesafeUsesEnvWithoutStore(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	writeSearchSkill(t, filepath.Join(p.CodexUserSkills, "video-encode"), "Encodes video files with ffmpeg.")
	if err := advisor.NewSettingsStore(p).Save(advisor.Settings{Version: 1, Provider: advisor.ProviderTypeSafe}); err != nil {
		t.Fatal(err)
	}
	store := &credentials.Memory{}
	app := newApp(p)
	app.stdin = strings.NewReader("Encode a local video file with ffmpeg.")
	app.credentials = store
	app.lookupEnv = func(key string) (string, bool) {
		if key == credentials.EnvTypeSafeKey {
			return "env-key-value", true
		}
		return "", false
	}
	app.newProvider = func(typesafe.Key) advisor.Provider {
		return recommendStub{handler: func(req typesafe.Request) (typesafe.Response, error) { return noulOK(req), nil }}
	}
	var stdout, stderr strings.Builder
	code := app.Run([]string{"advisor", "recommend", "--tool", "codex", "--query", "video encode ffmpeg", "--task-stdin", "--json"}, &stdout, &stderr)
	if code != 0 || store.Calls != 0 {
		t.Fatalf("code=%d calls=%d stderr=%s", code, store.Calls, stderr.String())
	}
}

func TestAdvisorRecommendMissingKeyAndLocalSettings(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	writeSearchSkill(t, filepath.Join(p.CodexUserSkills, "video-encode"), "Encodes video files with ffmpeg.")
	if err := advisor.NewSettingsStore(p).Save(advisor.Settings{Version: 1, Provider: advisor.ProviderTypeSafe}); err != nil {
		t.Fatal(err)
	}
	app := newApp(p)
	app.stdin = strings.NewReader("Encode a local video file with ffmpeg.")
	app.credentials = &credentials.Memory{}
	app.lookupEnv = func(string) (string, bool) { return "", false }
	var stdout, stderr strings.Builder
	code := app.Run([]string{"advisor", "recommend", "--tool", "codex", "--query", "video encode ffmpeg", "--task-stdin", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	var output advisorRecommendOutput
	if err := json.Unmarshal([]byte(stdout.String()), &output); err != nil {
		t.Fatal(err)
	}
	if output.FallbackReason != advisor.FallbackMissingKey {
		t.Fatalf("output = %#v", output)
	}
}

func TestAdvisorRecommendDoesNotTouchStateFiles(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	writeSearchSkill(t, filepath.Join(p.CodexUserSkills, "video-encode"), "Encodes video files with ffmpeg.")
	app := newApp(p)
	app.stdin = strings.NewReader("Encode a local video file.")
	var stdout, stderr strings.Builder
	if code := app.Run([]string{"advisor", "recommend", "--tool", "codex", "--query", "video encode ffmpeg", "--task-stdin", "--json"}, &stdout, &stderr); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if _, err := os.Stat(p.StateFile); !os.IsNotExist(err) {
		t.Fatalf("state.json err = %v", err)
	}
	if _, err := os.Stat(p.AdvisorFile); !os.IsNotExist(err) {
		t.Fatalf("advisor file err = %v", err)
	}
	combined := stdout.String() + stderr.String()
	if strings.Contains(combined, p.Home) || strings.Contains(combined, "sk-sentinel") {
		t.Fatalf("leaked path or key: %s", combined)
	}
}

type recommendStub struct {
	handler func(typesafe.Request) (typesafe.Response, error)
}

func (s recommendStub) Evaluate(_ context.Context, request typesafe.Request) (typesafe.Response, error) {
	return s.handler(request)
}

func noulOK(req typesafe.Request) typesafe.Response {
	answers := map[string]typesafe.Answer{}
	for id := range req.Questions {
		value := 0.9
		copied := value
		answers[id] = typesafe.Answer{Type: "noul", Noul: &copied}
	}
	return typesafe.Response{Model: typesafe.Model, Answers: answers, Usage: typesafe.Usage{InputTokens: 3, OutputTokens: 1}}
}
