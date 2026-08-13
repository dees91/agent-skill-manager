package contextbudget

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dees91/agent-skill-manager/internal/model"
	"github.com/dees91/agent-skill-manager/internal/paths"
)

func TestAnalyzeFilesystemFallbackAndClaudeVisibilityRules(t *testing.T) {
	t.Setenv("ANTHROPIC_MODEL", "")
	t.Setenv("CLAUDE_CODE_MAX_CONTEXT_TOKENS", "")
	t.Setenv("SLASH_COMMAND_TOOL_CHAR_BUDGET", "")
	p := paths.ForHome(t.TempDir())
	claudePath := writeTestSkill(t, filepath.Join(p.ClaudeUserSkills, "alpha"), "Alpha helper", "")
	codexPath := writeTestSkill(t, filepath.Join(p.CodexUserSkills, "alpha"), "Alpha helper", "")
	writeTestSkill(t, filepath.Join(p.CodexSystemSkills, "system"), "System helper", "")
	hiddenPath := writeTestSkill(t, filepath.Join(p.ClaudeUserSkills, "manual-only"), "Manual helper", "disable-model-invocation: true\n")

	rows := []model.SkillRow{
		{Name: "alpha", Claude: testCell(model.ToolClaude, "alpha", claudePath, model.SkillStateOn), Codex: testCell(model.ToolCodex, "alpha", codexPath, model.SkillStateOn)},
		{Name: "manual-only", Claude: testCell(model.ToolClaude, "manual-only", hiddenPath, model.SkillStateOn)},
	}
	reports := New(p).Analyze(rows).Reports

	if reports.Claude.Current.SkillCount != 1 {
		t.Fatalf("Claude skill count = %d, want 1", reports.Claude.Current.SkillCount)
	}
	if reports.Codex.Current.SkillCount != 2 {
		t.Fatalf("Codex skill count = %d, want managed + system", reports.Codex.Current.SkillCount)
	}
	if reports.Claude.BudgetTokens != 2000 || reports.Claude.BudgetFraction != .01 {
		t.Fatalf("Claude budget = %#v", reports.Claude)
	}
	if reports.Codex.BudgetTokens != 2000 || reports.Codex.BudgetFraction != .02 {
		t.Fatalf("Codex fallback budget = %#v", reports.Codex)
	}
	if reports.Claude.Accuracy != AccuracyPartial || reports.Codex.Accuracy != AccuracyPartial {
		t.Fatalf("accuracy = Claude %q, Codex %q", reports.Claude.Accuracy, reports.Codex.Accuracy)
	}
}

func TestAnalyzeCodexMeasuresShorteningAndOmission(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	skillPath := writeTestSkill(t, filepath.Join(p.CodexUserSkills, "alpha"), "A long description", "")
	row := model.SkillRow{Name: "alpha", Codex: testCell(model.ToolCodex, "alpha", skillPath, model.SkillStateOn)}
	raw := skillsPrompt(
		"- alpha: A long description that exceeds the rendered catalog (file: "+skillPath+")",
		"- beta: Another global skill (file: /tmp/beta/SKILL.md)",
	)
	actual := skillsPrompt("- alpha: A long… (file: " + skillPath + ")")
	analyzer := &Analyzer{
		paths: p,
		runner: staticRunner{
			models: `{"models":[{"slug":"gpt-test","context_window":1000,"priority":1}]}`,
			actual: promptDocument(t, actual),
			raw:    promptDocument(t, raw),
		},
		timeout:  defaultDiagnosticTimeout,
		lookPath: func(string) (string, error) { return "/fake/codex", nil },
	}

	result := analyzer.Analyze([]model.SkillRow{row})
	report := result.Reports.Codex
	if report.Accuracy != AccuracyMeasured || report.Model != "gpt-test" || report.ContextWindowTokens != 1000 {
		t.Fatalf("report identity = %#v", report)
	}
	if report.Current.SkillCount != 2 || report.Current.ShortenedDescriptions != 1 || report.Current.OmittedSkills != 1 {
		t.Fatalf("measured usage = %#v", report.Current)
	}
	if report.Current.RequestedCharacters <= report.Current.RenderedCharacters {
		t.Fatalf("requested %d <= rendered %d", report.Current.RequestedCharacters, report.Current.RenderedCharacters)
	}
}

func TestProjectPendingUpdatesEachToolIndependently(t *testing.T) {
	base := ToolReport{BudgetCharacters: 400, Current: Usage{SkillCount: 2, RequestedCharacters: 200, RenderedCharacters: 200}}
	finalizeUsage(&base.Current, base.BudgetCharacters)
	base.Projected = base.Current
	result := Result{
		Reports: Reports{Claude: base, Codex: base},
		contributions: map[CellKey]contribution{
			{Tool: model.ToolClaude, SkillName: "on"}: {Characters: 40, Included: true},
			{Tool: model.ToolCodex, SkillName: "off"}: {Characters: 80, Included: false},
		},
	}

	projected := result.Project(map[CellKey]model.OperationKind{
		{Tool: model.ToolClaude, SkillName: "on"}: model.OperationDisable,
		{Tool: model.ToolCodex, SkillName: "off"}: model.OperationEnable,
	})

	if projected.Claude.Projected.RequestedCharacters != 160 || projected.Claude.Projected.SkillCount != 1 || projected.Claude.Projected.UsedPercent != 40 {
		t.Fatalf("Claude projection = %#v", projected.Claude.Projected)
	}
	if projected.Codex.Projected.RequestedCharacters != 280 || projected.Codex.Projected.SkillCount != 3 || projected.Codex.Projected.UsedPercent != 70 {
		t.Fatalf("Codex projection = %#v", projected.Codex.Projected)
	}
}

func TestClaudeBudgetHonorsSettingsAndFixedEnvironment(t *testing.T) {
	t.Setenv("SLASH_COMMAND_TOOL_CHAR_BUDGET", "")
	fraction := .025
	settings := claudeSettings{SkillListingBudgetFraction: &fraction}
	characters, gotFraction, label := claudeBudget(settings, 100000)
	if characters != 10000 || gotFraction != fraction || label != "2.5% of model context" {
		t.Fatalf("fraction budget = %d, %v, %q", characters, gotFraction, label)
	}

	t.Setenv("SLASH_COMMAND_TOOL_CHAR_BUDGET", "1234")
	characters, gotFraction, label = claudeBudget(claudeSettings{}, 100000)
	if characters != 1234 || gotFraction != 0 || label != "fixed character budget" {
		t.Fatalf("fixed budget = %d, %v, %q", characters, gotFraction, label)
	}
}

func TestEnvironmentWithHomeReplacesInheritedHome(t *testing.T) {
	got := environmentWithHome([]string{"PATH=/bin", "HOME=/real", "CODEX_HOME=/real/codex", "CLAUDE_CONFIG_DIR=/real/claude", "OTHER=value"}, "/isolated")
	joined := strings.Join(got, "\n")
	if strings.Contains(joined, "HOME=/real") || strings.Contains(joined, "CODEX_HOME=") || strings.Contains(joined, "CLAUDE_CONFIG_DIR=") || !strings.Contains(joined, "HOME=/isolated") {
		t.Fatalf("environment = %#v", got)
	}
}

type staticRunner struct {
	models string
	actual []byte
	raw    []byte
}

func (runner staticRunner) Run(_ context.Context, _ string, _ string, args ...string) ([]byte, error) {
	joined := strings.Join(args, " ")
	if joined == "debug models" {
		return []byte(runner.models), nil
	}
	if strings.Contains(joined, "model_context_window=") {
		return runner.raw, nil
	}
	return runner.actual, nil
}

func skillsPrompt(entries ...string) string {
	return "<skills_instructions>\n## Skills\n### Available skills\n" + strings.Join(entries, "\n") + "\n</skills_instructions>"
}

func promptDocument(t *testing.T, prompt string) []byte {
	t.Helper()
	data, err := json.Marshal([]map[string]any{{"content": []map[string]string{{"text": prompt}}}})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func writeTestSkill(t *testing.T, directory, description, extra string) string {
	t.Helper()
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	contents := "---\nname: " + filepath.Base(directory) + "\ndescription: " + description + "\n" + extra + "---\n"
	path := filepath.Join(directory, "SKILL.md")
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func testCell(tool model.Tool, name, skillFile string, state model.SkillState) *model.ToolSkill {
	return &model.ToolSkill{
		Tool:          tool,
		Name:          name,
		DisplayName:   name,
		Description:   "Description for " + name,
		State:         state,
		ActivePath:    filepath.Dir(skillFile),
		SkillFilePath: skillFile,
	}
}
