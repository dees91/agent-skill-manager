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
	musePath := writeTestSkill(t, filepath.Join(p.MuseUserSkills, "alpha"), "Alpha helper", "")
	grokPath := writeTestSkill(t, filepath.Join(p.GrokUserSkills, "alpha"), "Alpha helper", "")
	writeTestSkill(t, filepath.Join(p.CodexSystemSkills, "system"), "System helper", "")
	hiddenPath := writeTestSkill(t, filepath.Join(p.ClaudeUserSkills, "manual-only"), "Manual helper", "disable-model-invocation: true\n")

	rows := []model.SkillRow{
		{Name: "alpha", Claude: testCell(model.ToolClaude, "alpha", claudePath, model.SkillStateOn), Codex: testCell(model.ToolCodex, "alpha", codexPath, model.SkillStateOn), Muse: testCell(model.ToolMuse, "alpha", musePath, model.SkillStateOn), Grok: testCell(model.ToolGrok, "alpha", grokPath, model.SkillStateOn)},
		{Name: "manual-only", Claude: testCell(model.ToolClaude, "manual-only", hiddenPath, model.SkillStateOn)},
	}
	reports := New(p).Estimate(rows).Reports

	if reports.Claude.Current.SkillCount != 1 {
		t.Fatalf("Claude skill count = %d, want 1", reports.Claude.Current.SkillCount)
	}
	if reports.Codex.Current.SkillCount != 2 {
		t.Fatalf("Codex skill count = %d, want managed + system", reports.Codex.Current.SkillCount)
	}
	if reports.Muse.Current.SkillCount != 1 {
		t.Fatalf("Muse skill count = %d, want 1", reports.Muse.Current.SkillCount)
	}
	if reports.Grok.Current.SkillCount != 1 {
		t.Fatalf("Grok skill count = %d, want 1", reports.Grok.Current.SkillCount)
	}
	if reports.Claude.BudgetTokens != 2000 || reports.Claude.BudgetFraction != .01 {
		t.Fatalf("Claude budget = %#v", reports.Claude)
	}
	if reports.Codex.BudgetTokens != 2000 || reports.Codex.BudgetFraction != .02 {
		t.Fatalf("Codex fallback budget = %#v", reports.Codex)
	}
	if reports.Muse.BudgetTokens != 2000 || reports.Muse.BudgetFraction != .01 {
		t.Fatalf("Muse fallback budget = %#v", reports.Muse)
	}
	if reports.Grok.BudgetTokens != 2000 || reports.Grok.BudgetFraction != .01 {
		t.Fatalf("Grok fallback budget = %#v", reports.Grok)
	}
	if reports.Claude.Accuracy != AccuracyPartial || reports.Codex.Accuracy != AccuracyPartial {
		t.Fatalf("accuracy = Claude %q, Codex %q", reports.Claude.Accuracy, reports.Codex.Accuracy)
	}
	if reports.Muse.Accuracy != AccuracyEstimated || !reports.Muse.ContextWindowAssumed {
		t.Fatalf("Muse report = %#v, want labeled estimate", reports.Muse)
	}
	if reports.Grok.Accuracy != AccuracyEstimated || !reports.Grok.ContextWindowAssumed {
		t.Fatalf("Grok report = %#v, want labeled estimate", reports.Grok)
	}
}

func TestAnalyzeCodexMeasuresShorteningAndOmission(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	if err := os.MkdirAll(filepath.Join(p.Home, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p.Home, ".codex", "models_cache.json"), []byte(`{"models":[{"slug":"gpt-test","context_window":1000,"priority":1}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
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
			actual: promptDocument(t, actual),
			raw:    promptDocument(t, raw),
		},
		timeout:  defaultDiagnosticTimeout,
		lookPath: func(string) (string, error) { return "/fake/codex", nil },
	}

	result := analyzer.Measure([]model.SkillRow{row})
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
		Reports: Reports{Claude: base, Codex: base, Muse: base, Grok: base},
		contributions: map[CellKey]contribution{
			{Tool: model.ToolClaude, SkillName: "on"}: {Characters: 40, Included: true},
			{Tool: model.ToolCodex, SkillName: "off"}: {Characters: 80, Included: false},
			{Tool: model.ToolMuse, SkillName: "on"}:   {Characters: 40, Included: true},
			{Tool: model.ToolGrok, SkillName: "on"}:   {Characters: 40, Included: true},
		},
	}

	projected := result.Project(map[CellKey]model.OperationKind{
		{Tool: model.ToolClaude, SkillName: "on"}: model.OperationDisable,
		{Tool: model.ToolCodex, SkillName: "off"}: model.OperationEnable,
		{Tool: model.ToolMuse, SkillName: "on"}:   model.OperationDisable,
		{Tool: model.ToolGrok, SkillName: "on"}:   model.OperationDisable,
	})

	if projected.Claude.Projected.RequestedCharacters != 160 || projected.Claude.Projected.SkillCount != 1 || projected.Claude.Projected.UsedPercent != 40 {
		t.Fatalf("Claude projection = %#v", projected.Claude.Projected)
	}
	if projected.Codex.Projected.RequestedCharacters != 280 || projected.Codex.Projected.SkillCount != 3 || projected.Codex.Projected.UsedPercent != 70 {
		t.Fatalf("Codex projection = %#v", projected.Codex.Projected)
	}
	if projected.Muse.Projected.RequestedCharacters != 160 || projected.Muse.Projected.SkillCount != 1 || projected.Muse.Projected.UsedPercent != 40 {
		t.Fatalf("Muse projection = %#v", projected.Muse.Projected)
	}
	if projected.Grok.Projected.RequestedCharacters != 160 || projected.Grok.Projected.SkillCount != 1 || projected.Grok.Projected.UsedPercent != 40 {
		t.Fatalf("Grok projection = %#v", projected.Grok.Projected)
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
	got := environmentWithHome([]string{"PATH=/bin", "HOME=/real", "CODEX_HOME=/real/codex", "CLAUDE_CONFIG_DIR=/real/claude", "ANTHROPIC_API_KEY=secret", "HTTPS_PROXY=https://proxy.invalid", "OTHER=value"}, "/isolated")
	joined := strings.Join(got, "\n")
	if strings.Contains(joined, "HOME=/real") || strings.Contains(joined, "CODEX_HOME=") || strings.Contains(joined, "CLAUDE_CONFIG_DIR=") || strings.Contains(joined, "ANTHROPIC_API_KEY=") || strings.Contains(joined, "HTTPS_PROXY=") || !strings.Contains(joined, "HOME=/isolated") {
		t.Fatalf("environment = %#v", got)
	}
}

func TestEstimateNeverRunsProviderCommands(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	runner := &countingRunner{}
	analyzer := &Analyzer{paths: p, runner: runner, timeout: defaultDiagnosticTimeout, lookPath: func(string) (string, error) { return "/fake/provider", nil }}

	analyzer.Estimate(nil)
	if runner.calls != 0 {
		t.Fatalf("provider calls = %d, want 0", runner.calls)
	}
}

func TestAllowedDiagnosticsAreAnExplicitReadOnlyAllowlist(t *testing.T) {
	allowed := [][]string{
		{"debug", "prompt-input"},
		{"debug", "prompt-input", "-c", "model_context_window=100000000"},
		{"plugin", "list", "--json"},
	}
	for _, args := range allowed {
		if !allowedDiagnostic(args) {
			t.Fatalf("expected diagnostic to be allowed: %v", args)
		}
	}
	for _, args := range [][]string{{"debug", "models"}, {"plugin", "install", "demo"}, {"exec", "echo"}} {
		if allowedDiagnostic(args) {
			t.Fatalf("unexpected diagnostic allowed: %v", args)
		}
	}
}

type staticRunner struct {
	actual []byte
	raw    []byte
}

func (runner staticRunner) Run(_ context.Context, _ string, _ string, args ...string) ([]byte, error) {
	joined := strings.Join(args, " ")
	if strings.Contains(joined, "model_context_window=") {
		return runner.raw, nil
	}
	return runner.actual, nil
}

type countingRunner struct {
	calls int
}

func (runner *countingRunner) Run(_ context.Context, _ string, _ string, _ ...string) ([]byte, error) {
	runner.calls++
	return nil, nil
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
