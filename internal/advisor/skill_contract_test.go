package advisor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestFirstPartySkillContract(t *testing.T) {
	skillDir := filepath.Join("..", "..", "skills", "skill-advisor")
	skillContents, err := os.ReadFile(filepath.Join(skillDir, "SKILL.md"))
	if err != nil {
		t.Fatalf("read first-party SKILL.md: %v", err)
	}
	text := string(skillContents)
	requiredInstructions := []string{
		"name: skill-advisor",
		"non-trivial",
		"skill-manager advisor search",
		"--query",
		"--limit 20",
		"ranked_search_v1",
		"Do not fall back",
		"Already active",
		"Needs activation",
		"state: \"on\"",
		"state: \"off\"",
		"skill-manager advisor activate",
		"skill-manager advisor cleanup",
		"Clean up before finishing",
		"before sending the final response",
		"every normal exit path",
		"not returned by the current invocation",
		"apiVersion",
	}
	for _, required := range requiredInstructions {
		if !strings.Contains(text, required) {
			t.Fatalf("SKILL.md is missing %q", required)
		}
	}
	if strings.Contains(text, "/Users/") || strings.Contains(text, "TODO") {
		t.Fatal("SKILL.md contains a private path or placeholder")
	}
	for _, superseded := range []string{"Do not disable skills automatically when the task ends", "Run cleanup only when the user requests it"} {
		if strings.Contains(text, superseded) {
			t.Fatalf("SKILL.md retains superseded cleanup guidance %q", superseded)
		}
	}
	for _, superseded := range []string{"skill-manager list --json \\", "Repeat `--query`", "Omitting `--available-for`"} {
		if strings.Contains(text, superseded) {
			t.Fatalf("SKILL.md retains superseded search guidance %q", superseded)
		}
	}

	metadataContents, err := os.ReadFile(filepath.Join(skillDir, "agents", "openai.yaml"))
	if err != nil {
		t.Fatalf("read first-party openai.yaml: %v", err)
	}
	var metadata struct {
		Interface struct {
			DisplayName      string `yaml:"display_name"`
			ShortDescription string `yaml:"short_description"`
			DefaultPrompt    string `yaml:"default_prompt"`
		} `yaml:"interface"`
	}
	if err := yaml.Unmarshal(metadataContents, &metadata); err != nil {
		t.Fatalf("decode openai.yaml: %v", err)
	}
	if metadata.Interface.DisplayName != "Skill Advisor" || len(metadata.Interface.ShortDescription) < 25 || len(metadata.Interface.ShortDescription) > 64 || !strings.Contains(metadata.Interface.DefaultPrompt, "$skill-advisor") {
		t.Fatalf("openai.yaml interface = %#v", metadata.Interface)
	}
}
