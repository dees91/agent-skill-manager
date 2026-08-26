package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dees91/agent-skill-manager/internal/advisor"
	"github.com/dees91/agent-skill-manager/internal/model"
	"github.com/dees91/agent-skill-manager/internal/paths"
	"github.com/dees91/agent-skill-manager/internal/state"
)

func TestRunAdvisorSearchJSONReturnsRankedPathFreeInventory(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	writeSearchSkill(t, filepath.Join(p.CodexUserSkills, "quality-review"), "Conducts multi-axis code review and quality review.")
	writeSearchSkill(t, filepath.Join(p.CodexUserSkills, "generic-docs"), "Looks up official documentation.")
	writeSearchSkill(t, filepath.Join(p.CodexSystemSkills, "system-docs"), "Records architecture decisions and documentation as ADRs.")
	disabledPath := filepath.Join(p.CodexDisabledDir, "decision-records")
	writeSearchSkill(t, disabledPath, "Records architecture decisions and documentation as ADRs.")
	saveState(t, p, state.Manifest{Disabled: []state.DisabledEntry{{
		Tool: model.ToolCodex, SkillName: "decision-records",
		OriginalPath: filepath.Join(p.CodexUserSkills, "decision-records"), DisabledPath: disabledPath,
		EntryType: model.EntryTypeDir, Source: model.SourceLocal, Group: model.GroupLocal,
	}}})
	beforeState := readFile(t, p.StateFile)
	var stdout, stderr strings.Builder

	code := RunWithPaths([]string{
		"advisor", "search", "--tool", "codex",
		"--query", "documentation ADR decision record code review quality review",
		"--limit", "10", "--json",
	}, &stdout, &stderr, p)

	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("advisor search code=%d stderr=%q", code, stderr.String())
	}
	var output advisorSearchOutput
	if err := json.Unmarshal([]byte(stdout.String()), &output); err != nil {
		t.Fatalf("decode advisor search JSON: %v\n%s", err, stdout.String())
	}
	if output.APIVersion != 1 || output.Tool != model.ToolCodex || len(output.Skills) != 3 {
		t.Fatalf("advisor search output = %#v", output)
	}
	firstTwo := map[string]bool{output.Skills[0].Name: true, output.Skills[1].Name: true}
	if !firstTwo["decision-records"] || !firstTwo["quality-review"] {
		t.Fatalf("advisor search order = %v", searchJSONNames(output.Skills))
	}
	states := map[string]string{}
	for _, skill := range output.Skills {
		states[skill.Name] = skill.Tools.Codex.State
	}
	if states["decision-records"] != "off" || states["quality-review"] != "on" {
		t.Fatalf("advisor search states = %#v", states)
	}
	if strings.Contains(stdout.String(), p.Home) || strings.Contains(stdout.String(), `"score"`) ||
		strings.Contains(stdout.String(), `"query"`) || strings.Contains(stdout.String(), "system-docs") {
		t.Fatalf("advisor search leaked private/internal data: %s", stdout.String())
	}
	if afterState := readFile(t, p.StateFile); string(afterState) != string(beforeState) {
		t.Fatalf("state changed during advisor search\nafter=%s\nbefore=%s", afterState, beforeState)
	}
}

func TestRunAdvisorSearchHumanOutputAndNoMatches(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	writeSearchSkill(t, filepath.Join(p.CodexUserSkills, "quality-review"), "Conducts quality review.")
	var stdout, stderr strings.Builder

	code := RunWithPaths([]string{"advisor", "search", "--tool", "codex", "--query", "quality review"}, &stdout, &stderr, p)

	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("advisor search code=%d stderr=%q", code, stderr.String())
	}
	for _, want := range []string{"Rank", "Skill", "State", "Group", "Source", "quality-review", "ON"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("human search output = %q, want %q", stdout.String(), want)
		}
	}

	stdout.Reset()
	code = RunWithPaths([]string{"advisor", "search", "--tool", "codex", "--query", "unmatched-phrase"}, &stdout, &stderr, p)
	if code != 0 || stdout.String() != "No advisor search matches.\n" || stderr.Len() != 0 {
		t.Fatalf("empty search code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestRunAdvisorSearchDefaultsToTwentyResults(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	for index := 0; index < advisor.DefaultSearchLimit+1; index++ {
		writeSearchSkill(t, filepath.Join(p.CodexUserSkills, fmt.Sprintf("helper-%02d", index)), "Common search helper.")
	}
	var stdout, stderr strings.Builder

	code := RunWithPaths([]string{"advisor", "search", "--tool", "codex", "--query", "common", "--json"}, &stdout, &stderr, p)

	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("advisor search code=%d stderr=%q", code, stderr.String())
	}
	var output advisorSearchOutput
	if err := json.Unmarshal([]byte(stdout.String()), &output); err != nil {
		t.Fatal(err)
	}
	if len(output.Skills) != advisor.DefaultSearchLimit || output.Skills[0].Name != "helper-00" || output.Skills[19].Name != "helper-19" {
		t.Fatalf("default-limited skills = %v", searchJSONNames(output.Skills))
	}
}

func TestRunAdvisorSearchUsesStructuredArgumentErrors(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing tool", args: []string{"advisor", "search", "--query", "search", "--json"}, want: "--tool"},
		{name: "missing query", args: []string{"advisor", "search", "--tool", "codex", "--json"}, want: "--query"},
		{name: "duplicate query", args: []string{"advisor", "search", "--tool", "codex", "--query", "one", "--query", "two", "--json"}, want: "one --query"},
		{name: "empty query", args: []string{"advisor", "search", "--tool", "codex", "--query", " ", "--json"}, want: "non-empty"},
		{name: "invalid limit", args: []string{"advisor", "search", "--tool", "codex", "--query", "search", "--limit", "51", "--json"}, want: "limit"},
		{name: "non-numeric limit", args: []string{"advisor", "search", "--tool", "codex", "--query", "search", "--limit", "many", "--json"}, want: "limit"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr strings.Builder
			code := RunWithPaths(test.args, &stdout, &stderr, p)
			if code == 0 || stdout.Len() != 0 {
				t.Fatalf("code=%d stdout=%q", code, stdout.String())
			}
			var output advisorErrorOutput
			if err := json.Unmarshal([]byte(stderr.String()), &output); err != nil {
				t.Fatalf("decode error JSON: %v\n%s", err, stderr.String())
			}
			if output.APIVersion != 1 || output.Error.Code != "INVALID_ARGUMENT" || !strings.Contains(output.Error.Message, test.want) {
				t.Fatalf("error output = %#v", output)
			}
		})
	}
}

func TestRunAdvisorSearchRedactsJSONScanErrors(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	if err := os.MkdirAll(p.StateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.StateFile, []byte("not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr strings.Builder

	code := RunWithPaths([]string{"advisor", "search", "--tool", "codex", "--query", "search", "--json"}, &stdout, &stderr, p)

	if code == 0 || stdout.Len() != 0 {
		t.Fatalf("code=%d stdout=%q", code, stdout.String())
	}
	var output advisorErrorOutput
	if err := json.Unmarshal([]byte(stderr.String()), &output); err != nil {
		t.Fatalf("decode error JSON: %v\n%s", err, stderr.String())
	}
	if output.Error.Code != "SEARCH_FAILED" || output.Error.Message != "advisor search could not read the local skill inventory" {
		t.Fatalf("error output = %#v", output)
	}
	if strings.Contains(stderr.String(), p.Home) || strings.Contains(stderr.String(), "state.json") {
		t.Fatalf("search error leaked local path: %s", stderr.String())
	}
}

func TestRunAdvisorStatusAdvertisesRankedSearchCapability(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	var stdout, stderr strings.Builder

	code := RunWithPaths([]string{"advisor", "status", "--tool", "codex", "--json"}, &stdout, &stderr, p)

	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("advisor status code=%d stderr=%q", code, stderr.String())
	}
	var output struct {
		APIVersion   int      `json:"apiVersion"`
		Capabilities []string `json:"capabilities"`
	}
	if err := json.Unmarshal([]byte(stdout.String()), &output); err != nil {
		t.Fatal(err)
	}
	if output.APIVersion != 1 || len(output.Capabilities) != 1 || output.Capabilities[0] != advisor.CapabilityRankedSearch {
		t.Fatalf("advisor status = %#v", output)
	}
}

func TestRunLegacyListQueriesRemainSubstringORAndAlphabetical(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	writeSearchSkill(t, filepath.Join(p.CodexUserSkills, "zeta-quality"), "Quality review helper.")
	writeSearchSkill(t, filepath.Join(p.CodexUserSkills, "alpha-documentation"), "Documentation helper.")
	writeSearchSkill(t, filepath.Join(p.CodexUserSkills, "middle-unrelated"), "Release helper.")
	var stdout, stderr strings.Builder

	code := RunWithPaths([]string{"list", "--json", "--query", "documentation", "--query", "quality review"}, &stdout, &stderr, p)

	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("legacy list code=%d stderr=%q", code, stderr.String())
	}
	var output listJSONOutput
	if err := json.Unmarshal([]byte(stdout.String()), &output); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(searchJSONNames(output.Skills), ","); got != "alpha-documentation,zeta-quality" {
		t.Fatalf("legacy list order = %q", got)
	}
}

func writeSearchSkill(t *testing.T, dir, description string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir skill: %v", err)
	}
	contents := fmt.Sprintf("---\ndescription: %q\n---\n# Skill\n", description)
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(contents), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}
}

func searchJSONNames(skills []listJSONSkill) []string {
	names := make([]string, 0, len(skills))
	for _, skill := range skills {
		names = append(names, skill.Name)
	}
	return names
}
