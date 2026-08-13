package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	domain "github.com/dees91/agent-skill-manager/internal/model"
	"github.com/dees91/agent-skill-manager/internal/paths"
	"github.com/dees91/agent-skill-manager/internal/state"
)

func TestNewGroupsRowsAndViewRendersColumns(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	mkdirSkill(t, filepath.Join(p.ClaudeUserSkills, "shared"))
	mkdirSkill(t, filepath.Join(p.CodexUserSkills, "shared"))
	mkdirSkill(t, filepath.Join(p.CodexSystemSkills, "imagegen"))

	model := New(p)

	if model.err != nil {
		t.Fatalf("New() err = %v", model.err)
	}
	if len(model.rows) != 1 {
		t.Fatalf("rows len = %d, want 1: %#v", len(model.rows), model.rows)
	}
	if model.rows[0].Name != "shared" || model.rows[0].Claude == nil || model.rows[0].Codex == nil {
		t.Fatalf("row = %#v, want grouped shared row", model.rows[0])
	}

	view := model.View()
	for _, want := range []string{"Skill", "Claude", "Codex", "Group", "shared", "local", "[ON]", "ON", "Group: all", "g group", "A all", "G filter"} {
		if !strings.Contains(view, want) {
			t.Fatalf("View() = %q, want %q", view, want)
		}
	}
	if strings.Contains(view, "imagegen") {
		t.Fatalf("View() = %q, did not expect read-only-only row by default", view)
	}
}

func TestMainTableShowsGroupNotSource(t *testing.T) {
	model := Model{
		rows: []domain.SkillRow{
			{
				Name:   "agp-9-upgrade",
				Source: domain.SourceSymlinkRepo,
				Group:  domain.GroupLabel("android/skills"),
				Claude: &domain.ToolSkill{
					State:  domain.SkillStateOn,
					Source: domain.SourceSymlinkRepo,
					Group:  domain.GroupLabel("android/skills"),
				},
			},
		},
		activeTool: domain.ToolClaude,
	}

	view := model.View()
	if !strings.Contains(view, "Group") || !strings.Contains(view, "android/skills") {
		t.Fatalf("View() = %q, want group column and label", view)
	}
	if !strings.Contains(view, "Group: all") {
		t.Fatalf("View() = %q, want group filter status", view)
	}
	if strings.Contains(view, "symlink repo") {
		t.Fatalf("View() = %q, did not expect source label in main table", view)
	}
}

func TestNewDefaultModelIgnoresReadOnlyScanErrors(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	mkdirSkill(t, filepath.Join(p.ClaudeUserSkills, "managed"))
	if err := os.MkdirAll(filepath.Dir(p.CodexSystemSkills), 0o755); err != nil {
		t.Fatalf("mkdir CodexSystemSkills parent: %v", err)
	}
	if err := os.WriteFile(p.CodexSystemSkills, []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("write CodexSystemSkills file: %v", err)
	}

	model := New(p)

	if model.err != nil {
		t.Fatalf("New() err = %v, want nil because read-only scan is not part of default model", model.err)
	}
	if len(model.rows) != 1 || model.rows[0].Name != "managed" {
		t.Fatalf("rows = %#v, want managed row", model.rows)
	}
}

func TestReadOnlyToggleShowsReadOnlyRows(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	mkdirSkill(t, filepath.Join(p.ClaudeUserSkills, "managed"))
	mkdirSkill(t, filepath.Join(p.CodexSystemSkills, "imagegen"))
	model := New(p)

	if rowNames(model.rows).String() != "managed" {
		t.Fatalf("default rows = %v, want only managed", rowNames(model.rows))
	}

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})

	if got := rowNames(model.rows).String(); got != "imagegen,managed" {
		t.Fatalf("rows after read-only toggle = %s, want imagegen,managed", got)
	}
	if model.rows[0].Codex == nil || model.rows[0].Codex.State != domain.SkillStateReadOnly {
		t.Fatalf("imagegen row = %#v, want codex read-only", model.rows[0])
	}

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})

	if got := rowNames(model.rows).String(); got != "managed" {
		t.Fatalf("rows after hiding read-only = %s, want managed", got)
	}
}

func TestReadOnlyToggleReportsReadOnlyScanErrors(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	mkdirSkill(t, filepath.Join(p.ClaudeUserSkills, "managed"))
	if err := os.MkdirAll(filepath.Dir(p.CodexSystemSkills), 0o755); err != nil {
		t.Fatalf("mkdir CodexSystemSkills parent: %v", err)
	}
	if err := os.WriteFile(p.CodexSystemSkills, []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("write CodexSystemSkills file: %v", err)
	}
	model := New(p)

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})

	if model.err == nil {
		t.Fatalf("model.err = nil, want read-only scan error")
	}
}

func TestTextFilterChangesVisibleRows(t *testing.T) {
	model := Model{
		allRows: []domain.SkillRow{
			{Name: "alpha", Description: "first"},
			{Name: "beta", Description: "second"},
		},
		activeTool: domain.ToolClaude,
	}
	model.rebuildRows()

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'B'}})
	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyEnter})

	if got := rowNames(model.rows).String(); got != "beta" {
		t.Fatalf("filtered rows = %s, want beta", got)
	}
	if model.editingFilter {
		t.Fatalf("editingFilter = true, want false after enter")
	}
}

func TestSourceFilterCyclesVisibleRows(t *testing.T) {
	model := Model{
		allRows: []domain.SkillRow{
			{
				Name:   "local-skill",
				Source: domain.SourceLocal,
				Claude: &domain.ToolSkill{
					Source: domain.SourceLocal,
				},
			},
			{
				Name:   "repo-skill",
				Source: domain.SourceSymlinkRepo,
				Codex: &domain.ToolSkill{
					Source: domain.SourceSymlinkRepo,
				},
			},
		},
		activeTool: domain.ToolClaude,
	}
	model.rebuildRows()

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})

	if model.sourceFilter != domain.SourceLocal {
		t.Fatalf("sourceFilter = %q, want local", model.sourceFilter)
	}
	if got := rowNames(model.rows).String(); got != "local-skill" {
		t.Fatalf("source-filtered rows = %s, want local-skill", got)
	}

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if model.sourceFilter != domain.SourceSymlinkRepo {
		t.Fatalf("sourceFilter = %q, want symlink repo", model.sourceFilter)
	}
	if got := rowNames(model.rows).String(); got != "repo-skill" {
		t.Fatalf("source-filtered rows = %s, want repo-skill", got)
	}
}

func TestGroupFilterCyclesVisibleRows(t *testing.T) {
	model := Model{
		allRows: []domain.SkillRow{
			{
				Name:  "android-skill",
				Group: domain.GroupLabel("android/skills"),
				Claude: &domain.ToolSkill{
					Group: domain.GroupLabel("android/skills"),
				},
			},
			{
				Name:  "local-skill",
				Group: domain.GroupLocal,
				Claude: &domain.ToolSkill{
					Group: domain.GroupLocal,
				},
			},
		},
		activeTool: domain.ToolClaude,
	}
	model.rebuildRows()

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})

	if model.groupFilter != domain.GroupLabel("android/skills") {
		t.Fatalf("groupFilter = %q, want android/skills", model.groupFilter)
	}
	if got := rowNames(model.rows).String(); got != "android-skill" {
		t.Fatalf("group-filtered rows = %s, want android-skill", got)
	}
	if !strings.Contains(model.filterStatusLine(), "Group: android/skills") {
		t.Fatalf("filterStatusLine() = %q, want active group", model.filterStatusLine())
	}

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	if model.groupFilter != domain.GroupLocal {
		t.Fatalf("groupFilter = %q, want local", model.groupFilter)
	}
	if got := rowNames(model.rows).String(); got != "local-skill" {
		t.Fatalf("group-filtered rows = %s, want local-skill", got)
	}

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	if model.groupFilter != allGroupsFilter {
		t.Fatalf("groupFilter = %q, want all", model.groupFilter)
	}
	if got := rowNames(model.rows).String(); got != "android-skill,local-skill" {
		t.Fatalf("group-filtered rows = %s, want all rows", got)
	}
}

func TestGroupFilterComposesWithTextFilter(t *testing.T) {
	model := Model{
		allRows: []domain.SkillRow{
			{Name: "alpha", Group: domain.GroupLabel("android/skills")},
			{Name: "beta-android", Group: domain.GroupLabel("android/skills")},
			{Name: "beta-local", Group: domain.GroupLocal},
		},
		activeTool: domain.ToolClaude,
		textFilter: "beta",
	}
	model.rebuildRows()

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})

	if model.groupFilter != domain.GroupLabel("android/skills") {
		t.Fatalf("groupFilter = %q, want android/skills", model.groupFilter)
	}
	if got := rowNames(model.rows).String(); got != "beta-android" {
		t.Fatalf("filtered rows = %s, want beta-android", got)
	}
}

func TestGroupFilterResetsWhenSourceFilterRemovesGroup(t *testing.T) {
	model := Model{
		allRows: []domain.SkillRow{
			{
				Name:   "local-skill",
				Source: domain.SourceLocal,
				Group:  domain.GroupLocal,
				Claude: &domain.ToolSkill{
					Source: domain.SourceLocal,
					Group:  domain.GroupLocal,
				},
			},
			{
				Name:   "repo-skill",
				Source: domain.SourceSymlinkRepo,
				Group:  domain.GroupLabel("android/skills"),
				Claude: &domain.ToolSkill{
					Source: domain.SourceSymlinkRepo,
					Group:  domain.GroupLabel("android/skills"),
				},
			},
		},
		activeTool:   domain.ToolClaude,
		groupFilter:  domain.GroupLabel("android/skills"),
		sourceFilter: allSourcesFilter,
	}
	model.rebuildRows()

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})

	if model.sourceFilter != domain.SourceLocal {
		t.Fatalf("sourceFilter = %q, want local", model.sourceFilter)
	}
	if model.groupFilter != allGroupsFilter {
		t.Fatalf("groupFilter = %q, want reset to all", model.groupFilter)
	}
	if got := rowNames(model.rows).String(); got != "local-skill" {
		t.Fatalf("filtered rows = %s, want local-skill", got)
	}
}

func TestGroupFilterUsesVisibleRowGroup(t *testing.T) {
	model := Model{
		allRows: []domain.SkillRow{
			{
				Name:  "mixed",
				Group: domain.GroupUnknown,
				Claude: &domain.ToolSkill{
					Group: domain.GroupLabel("android/skills"),
				},
				Codex: &domain.ToolSkill{
					Group: domain.GroupLocal,
				},
			},
			{
				Name:  "repo-skill",
				Group: domain.GroupLabel("android/skills"),
				Claude: &domain.ToolSkill{
					Group: domain.GroupLabel("android/skills"),
				},
			},
		},
		activeTool: domain.ToolClaude,
	}
	model.rebuildRows()

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})

	if model.groupFilter != domain.GroupLabel("android/skills") {
		t.Fatalf("groupFilter = %q, want android/skills", model.groupFilter)
	}
	if got := rowNames(model.rows).String(); got != "repo-skill" {
		t.Fatalf("group-filtered rows = %s, want only row with visible android/skills group", got)
	}
}

func TestGroupFilterChoicesRespectSourceFilter(t *testing.T) {
	model := Model{
		allRows: []domain.SkillRow{
			{
				Name:   "local-skill",
				Source: domain.SourceLocal,
				Group:  domain.GroupLocal,
			},
			{
				Name:   "repo-skill",
				Source: domain.SourceSymlinkRepo,
				Group:  domain.GroupLabel("android/skills"),
			},
		},
		activeTool:   domain.ToolClaude,
		sourceFilter: domain.SourceLocal,
	}
	model.rebuildRows()

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})

	if model.groupFilter != domain.GroupLocal {
		t.Fatalf("groupFilter = %q, want local", model.groupFilter)
	}
	if got := rowNames(model.rows).String(); got != "local-skill" {
		t.Fatalf("filtered rows = %s, want local-skill", got)
	}
}

func TestGroupFilterClampsCursor(t *testing.T) {
	model := Model{
		allRows: []domain.SkillRow{
			{Name: "android-skill", Group: domain.GroupLabel("android/skills")},
			{Name: "local-skill", Group: domain.GroupLocal},
		},
		activeTool: domain.ToolClaude,
		cursor:     1,
	}
	model.rebuildRows()

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})

	if model.cursor != 0 {
		t.Fatalf("cursor = %d, want clamped to 0", model.cursor)
	}
	if got := rowNames(model.rows).String(); got != "android-skill" {
		t.Fatalf("filtered rows = %s, want android-skill", got)
	}
}

func TestGroupFilterKeyIsTextInputWhileEditingFilter(t *testing.T) {
	model := Model{
		allRows: []domain.SkillRow{
			{Name: "alpha", Group: domain.GroupLocal},
		},
		activeTool: domain.ToolClaude,
	}
	model.rebuildRows()

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})

	if model.groupFilter != allGroupsFilter {
		t.Fatalf("groupFilter = %q, want unchanged all", model.groupFilter)
	}
	if model.textFilter != "G" {
		t.Fatalf("textFilter = %q, want G", model.textFilter)
	}
}

func TestReadOnlyCellsCannotBeToggled(t *testing.T) {
	model := Model{
		rows: []domain.SkillRow{
			{
				Name: "imagegen",
				Codex: &domain.ToolSkill{
					Tool:     domain.ToolCodex,
					Name:     "imagegen",
					State:    domain.SkillStateReadOnly,
					ReadOnly: true,
				},
			},
		},
		activeTool: domain.ToolCodex,
	}

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
	if len(model.pending) != 0 {
		t.Fatalf("pending after read-only Space = %#v, want empty", model.pending)
	}

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	if len(model.pending) != 0 {
		t.Fatalf("pending after read-only b = %#v, want empty", model.pending)
	}
}

func TestBothTogglesOnlyToggleableSideOnMixedReadOnlyRow(t *testing.T) {
	model := Model{
		rows: []domain.SkillRow{
			{
				Name: "mixed",
				Claude: &domain.ToolSkill{
					Tool:  domain.ToolClaude,
					Name:  "mixed",
					State: domain.SkillStateOn,
				},
				Codex: &domain.ToolSkill{
					Tool:     domain.ToolCodex,
					Name:     "mixed",
					State:    domain.SkillStateReadOnly,
					ReadOnly: true,
				},
			},
		},
		activeTool: domain.ToolClaude,
	}

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})

	if got := model.pending[pendingKey{tool: domain.ToolClaude, skillName: "mixed"}]; got != domain.OperationDisable {
		t.Fatalf("claude pending = %q, want disable", got)
	}
	if _, ok := model.pending[pendingKey{tool: domain.ToolCodex, skillName: "mixed"}]; ok {
		t.Fatalf("codex read-only pending exists: %#v", model.pending)
	}
}

func TestBatchToggleAllOnStagesDisables(t *testing.T) {
	model := Model{}
	rows := []domain.SkillRow{
		{
			Name:   "alpha",
			Claude: batchSkill(domain.SkillStateOn),
			Codex:  batchSkill(domain.SkillStateOn),
		},
	}

	result := model.batchTogglePending(rows, domain.Tools())

	if result.changed != 2 || result.removed != 0 {
		t.Fatalf("result = %#v, want 2 changed", result)
	}
	assertPendingKind(t, model.pending, domain.ToolClaude, "alpha", domain.OperationDisable)
	assertPendingKind(t, model.pending, domain.ToolCodex, "alpha", domain.OperationDisable)
}

func TestBatchToggleMixedScopeStagesEnablesOnlyForOffCells(t *testing.T) {
	model := Model{}
	rows := []domain.SkillRow{
		{
			Name:   "alpha",
			Claude: batchSkill(domain.SkillStateOn),
			Codex:  batchSkill(domain.SkillStateOff),
		},
	}

	result := model.batchTogglePending(rows, domain.Tools())

	if result.changed != 1 || result.removed != 0 {
		t.Fatalf("result = %#v, want 1 changed", result)
	}
	if _, ok := model.pending[pendingKey{tool: domain.ToolClaude, skillName: "alpha"}]; ok {
		t.Fatalf("claude pending exists for ON cell: %#v", model.pending)
	}
	assertPendingKind(t, model.pending, domain.ToolCodex, "alpha", domain.OperationEnable)
}

func TestBatchToggleSkipsReadOnlyMissingAndConflictCells(t *testing.T) {
	model := Model{}
	rows := []domain.SkillRow{
		{Name: "missing"},
		{Name: "readonly", Claude: &domain.ToolSkill{State: domain.SkillStateReadOnly, ReadOnly: true}},
		{Name: "conflict", Claude: &domain.ToolSkill{State: domain.SkillStateConflict, Conflict: &domain.Conflict{}}},
	}

	result := model.batchTogglePending(rows, []domain.Tool{domain.ToolClaude})

	if result.changed != 0 || result.skippedMissing != 1 || result.skippedReadOnly != 1 || result.skippedConflict != 1 {
		t.Fatalf("result = %#v, want missing/read-only/conflict skips", result)
	}
	if len(model.pending) != 0 {
		t.Fatalf("pending = %#v, want empty", model.pending)
	}
}

func TestBatchToggleOneSidedRowOnlyStagesExistingCell(t *testing.T) {
	model := Model{}
	rows := []domain.SkillRow{
		{Name: "one-sided", Claude: batchSkill(domain.SkillStateOn)},
	}

	result := model.batchTogglePending(rows, domain.Tools())

	if result.changed != 1 || result.skippedMissing != 1 {
		t.Fatalf("result = %#v, want one changed and one missing skip", result)
	}
	assertPendingKind(t, model.pending, domain.ToolClaude, "one-sided", domain.OperationDisable)
	if _, ok := model.pending[pendingKey{tool: domain.ToolCodex, skillName: "one-sided"}]; ok {
		t.Fatalf("codex pending exists for missing cell: %#v", model.pending)
	}
}

func TestBatchToggleAllApplicableWithSamePendingRemovesAll(t *testing.T) {
	model := Model{
		pending: map[pendingKey]domain.OperationKind{
			{tool: domain.ToolClaude, skillName: "alpha"}: domain.OperationDisable,
			{tool: domain.ToolClaude, skillName: "beta"}:  domain.OperationDisable,
		},
	}
	rows := []domain.SkillRow{
		{Name: "alpha", Claude: batchSkill(domain.SkillStateOn)},
		{Name: "beta", Claude: batchSkill(domain.SkillStateOn)},
	}

	result := model.batchTogglePending(rows, []domain.Tool{domain.ToolClaude})

	if result.removed != 2 || result.changed != 0 {
		t.Fatalf("result = %#v, want 2 removed", result)
	}
	if len(model.pending) != 0 {
		t.Fatalf("pending = %#v, want empty", model.pending)
	}
}

func TestBatchToggleReplacesExistingOppositePending(t *testing.T) {
	model := Model{
		pending: map[pendingKey]domain.OperationKind{
			{tool: domain.ToolClaude, skillName: "alpha"}: domain.OperationEnable,
		},
	}
	rows := []domain.SkillRow{
		{Name: "alpha", Claude: batchSkill(domain.SkillStateOn)},
		{Name: "beta", Claude: batchSkill(domain.SkillStateOn)},
	}

	result := model.batchTogglePending(rows, []domain.Tool{domain.ToolClaude})

	if result.changed != 2 || result.removed != 0 {
		t.Fatalf("result = %#v, want one replacement and one new pending", result)
	}
	assertPendingKind(t, model.pending, domain.ToolClaude, "alpha", domain.OperationDisable)
	assertPendingKind(t, model.pending, domain.ToolClaude, "beta", domain.OperationDisable)
}

func TestBatchTogglePendingDisableMakesCellEffectivelyOff(t *testing.T) {
	model := Model{
		pending: map[pendingKey]domain.OperationKind{
			{tool: domain.ToolClaude, skillName: "alpha"}: domain.OperationDisable,
		},
	}
	rows := []domain.SkillRow{
		{Name: "alpha", Claude: batchSkill(domain.SkillStateOn)},
		{Name: "beta", Claude: batchSkill(domain.SkillStateOn)},
	}

	result := model.batchTogglePending(rows, []domain.Tool{domain.ToolClaude})

	if result.changed != 0 || result.removed != 1 {
		t.Fatalf("result = %#v, want one pending cancellation", result)
	}
	if _, ok := model.pending[pendingKey{tool: domain.ToolClaude, skillName: "alpha"}]; ok {
		t.Fatalf("alpha pending exists, want cancellation: %#v", model.pending)
	}
	if _, ok := model.pending[pendingKey{tool: domain.ToolClaude, skillName: "beta"}]; ok {
		t.Fatalf("beta pending exists, want ON cell left unchanged: %#v", model.pending)
	}
}

func TestBatchTogglePendingEnableMakesCellEffectivelyOn(t *testing.T) {
	model := Model{
		pending: map[pendingKey]domain.OperationKind{
			{tool: domain.ToolClaude, skillName: "beta"}: domain.OperationEnable,
		},
	}
	rows := []domain.SkillRow{
		{Name: "alpha", Claude: batchSkill(domain.SkillStateOn)},
		{Name: "beta", Claude: batchSkill(domain.SkillStateOff)},
	}

	result := model.batchTogglePending(rows, []domain.Tool{domain.ToolClaude})

	if result.changed != 1 || result.removed != 1 {
		t.Fatalf("result = %#v, want one disable staged and one pending cancellation", result)
	}
	assertPendingKind(t, model.pending, domain.ToolClaude, "alpha", domain.OperationDisable)
	if _, ok := model.pending[pendingKey{tool: domain.ToolClaude, skillName: "beta"}]; ok {
		t.Fatalf("beta pending exists, want cancellation: %#v", model.pending)
	}
}

func TestGroupToggleUsesAllLoadedRowsForSelectedGroup(t *testing.T) {
	model := Model{
		allRows: []domain.SkillRow{
			{Name: "visible", Group: domain.GroupLabel("android/skills"), Claude: batchSkill(domain.SkillStateOn)},
			{Name: "hidden", Group: domain.GroupLabel("android/skills"), Codex: batchSkill(domain.SkillStateOn)},
			{Name: "other", Group: domain.GroupLocal, Claude: batchSkill(domain.SkillStateOn)},
		},
		rows: []domain.SkillRow{
			{Name: "visible", Group: domain.GroupLabel("android/skills"), Claude: batchSkill(domain.SkillStateOn)},
		},
		activeTool: domain.ToolClaude,
	}

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})

	assertPendingKind(t, model.pending, domain.ToolClaude, "visible", domain.OperationDisable)
	assertPendingKind(t, model.pending, domain.ToolCodex, "hidden", domain.OperationDisable)
	if _, ok := model.pending[pendingKey{tool: domain.ToolClaude, skillName: "other"}]; ok {
		t.Fatalf("other group pending exists: %#v", model.pending)
	}
	for _, want := range []string{"Group android/skills", "2 pending changes updated", "Skipped 2 missing"} {
		if !strings.Contains(model.message, want) {
			t.Fatalf("message = %q, want %q", model.message, want)
		}
	}
}

func TestGroupToggleNormalizesEmptyGroupToUnknown(t *testing.T) {
	model := Model{
		allRows: []domain.SkillRow{
			{Name: "selected", Group: "", Claude: batchSkill(domain.SkillStateOn)},
			{Name: "unknown", Group: domain.GroupUnknown, Claude: batchSkill(domain.SkillStateOn)},
		},
		rows: []domain.SkillRow{
			{Name: "selected", Group: "", Claude: batchSkill(domain.SkillStateOn)},
		},
		activeTool: domain.ToolClaude,
	}

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})

	assertPendingKind(t, model.pending, domain.ToolClaude, "selected", domain.OperationDisable)
	assertPendingKind(t, model.pending, domain.ToolClaude, "unknown", domain.OperationDisable)
	if !strings.Contains(model.message, "Group unknown") {
		t.Fatalf("message = %q, want normalized unknown group", model.message)
	}
}

func TestGroupToggleWithNoSelectedRowShowsMessage(t *testing.T) {
	model := Model{activeTool: domain.ToolClaude}

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})

	if len(model.pending) != 0 {
		t.Fatalf("pending = %#v, want empty", model.pending)
	}
	if model.message != "No selected skill." {
		t.Fatalf("message = %q, want no selected skill", model.message)
	}
}

func TestGroupToggleRepeatedPressRemovesPendingChanges(t *testing.T) {
	model := Model{
		allRows: []domain.SkillRow{
			{Name: "alpha", Group: domain.GroupLocal, Claude: batchSkill(domain.SkillStateOn)},
			{Name: "beta", Group: domain.GroupLocal, Claude: batchSkill(domain.SkillStateOn)},
		},
		rows: []domain.SkillRow{
			{Name: "alpha", Group: domain.GroupLocal, Claude: batchSkill(domain.SkillStateOn)},
			{Name: "beta", Group: domain.GroupLocal, Claude: batchSkill(domain.SkillStateOn)},
		},
		activeTool: domain.ToolClaude,
	}

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	if len(model.pending) != 2 {
		t.Fatalf("pending after first g = %#v, want 2", model.pending)
	}
	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})

	if len(model.pending) != 0 {
		t.Fatalf("pending after second g = %#v, want empty", model.pending)
	}
	if !strings.Contains(model.message, "2 pending changes updated") {
		t.Fatalf("message = %q, want update count for removed pending", model.message)
	}
}

func TestGroupToggleMessageShowsChangedRemovedAndConflictSkips(t *testing.T) {
	model := Model{
		allRows: []domain.SkillRow{
			{Name: "cancel", Group: domain.GroupLocal, Claude: batchSkill(domain.SkillStateOff)},
			{Name: "change", Group: domain.GroupLocal, Claude: batchSkill(domain.SkillStateOn)},
			{Name: "conflict", Group: domain.GroupLocal, Claude: &domain.ToolSkill{State: domain.SkillStateConflict, Conflict: &domain.Conflict{}}},
		},
		rows: []domain.SkillRow{
			{Name: "cancel", Group: domain.GroupLocal, Claude: batchSkill(domain.SkillStateOff)},
		},
		activeTool: domain.ToolClaude,
		pending: map[pendingKey]domain.OperationKind{
			{tool: domain.ToolClaude, skillName: "cancel"}: domain.OperationEnable,
		},
	}

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})

	for _, want := range []string{"Group local", "2 pending changes updated", "1 changed", "1 removed", "Skipped 1 conflict"} {
		if !strings.Contains(model.message, want) {
			t.Fatalf("message = %q, want %q", model.message, want)
		}
	}
}

func TestGroupToggleNoopMessageIncludesSkippedReasons(t *testing.T) {
	model := Model{
		allRows: []domain.SkillRow{
			{Name: "readonly", Group: domain.GroupLocal, Claude: &domain.ToolSkill{State: domain.SkillStateReadOnly, ReadOnly: true}},
			{Name: "conflict", Group: domain.GroupLocal, Claude: &domain.ToolSkill{State: domain.SkillStateConflict, Conflict: &domain.Conflict{}}},
		},
		rows: []domain.SkillRow{
			{Name: "readonly", Group: domain.GroupLocal, Claude: &domain.ToolSkill{State: domain.SkillStateReadOnly, ReadOnly: true}},
		},
		activeTool: domain.ToolClaude,
	}

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})

	for _, want := range []string{"Group local: no applicable cells.", "Skipped 1 conflict", "1 read-only", "2 missing"} {
		if !strings.Contains(model.message, want) {
			t.Fatalf("message = %q, want %q", model.message, want)
		}
	}
}

func TestGroupToggleDoesNotApplyFilesystemChanges(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	activePath := filepath.Join(p.ClaudeUserSkills, "alpha")
	disabledPath := filepath.Join(p.ClaudeDisabledDir, "alpha")
	mkdirSkill(t, activePath)
	model := New(p)

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})

	assertPendingKind(t, model.pending, domain.ToolClaude, "alpha", domain.OperationDisable)
	if _, err := os.Lstat(activePath); err != nil {
		t.Fatalf("active path changed after g: %v", err)
	}
	if _, err := os.Lstat(disabledPath); !os.IsNotExist(err) {
		t.Fatalf("disabled path err after g = %v, want not exist", err)
	}
	if _, err := os.Lstat(p.StateFile); !os.IsNotExist(err) {
		t.Fatalf("state file err after g = %v, want not exist", err)
	}
}

func TestAllVisibleToggleHandlesMixedVisibleScope(t *testing.T) {
	model := Model{
		rows: []domain.SkillRow{
			{
				Name:   "two-sided",
				Claude: batchSkill(domain.SkillStateOn),
				Codex:  batchSkill(domain.SkillStateOn),
			},
			{
				Name:   "one-sided",
				Claude: batchSkill(domain.SkillStateOn),
			},
			{
				Name:  "off",
				Codex: batchSkill(domain.SkillStateOff),
			},
			{
				Name:  "readonly",
				Codex: &domain.ToolSkill{State: domain.SkillStateReadOnly, ReadOnly: true},
			},
			{
				Name:   "conflict",
				Claude: &domain.ToolSkill{State: domain.SkillStateConflict, Conflict: &domain.Conflict{}},
			},
		},
		activeTool: domain.ToolClaude,
	}

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})

	assertPendingKind(t, model.pending, domain.ToolCodex, "off", domain.OperationEnable)
	if len(model.pending) != 1 {
		t.Fatalf("pending = %#v, want only OFF cell enable in mixed visible scope", model.pending)
	}
	for _, want := range []string{"All visible rows", "1 pending change updated", "Skipped 1 conflict", "1 read-only", "4 missing"} {
		if !strings.Contains(model.message, want) {
			t.Fatalf("message = %q, want %q", model.message, want)
		}
	}
}

func TestAllVisibleToggleStagesBothToolsForTwoSidedRow(t *testing.T) {
	model := Model{
		rows: []domain.SkillRow{
			{
				Name:   "shared",
				Claude: batchSkill(domain.SkillStateOn),
				Codex:  batchSkill(domain.SkillStateOn),
			},
		},
		activeTool: domain.ToolClaude,
	}

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})

	assertPendingKind(t, model.pending, domain.ToolClaude, "shared", domain.OperationDisable)
	assertPendingKind(t, model.pending, domain.ToolCodex, "shared", domain.OperationDisable)
}

func TestAllVisibleToggleRepeatedPressRemovesPendingChanges(t *testing.T) {
	model := Model{
		rows: []domain.SkillRow{
			{Name: "alpha", Claude: batchSkill(domain.SkillStateOn)},
			{Name: "beta", Claude: batchSkill(domain.SkillStateOn)},
		},
		activeTool: domain.ToolClaude,
	}

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})
	if len(model.pending) != 2 {
		t.Fatalf("pending after first A = %#v, want 2", model.pending)
	}
	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})

	if len(model.pending) != 0 {
		t.Fatalf("pending after second A = %#v, want empty", model.pending)
	}
	if !strings.Contains(model.message, "2 pending changes updated") {
		t.Fatalf("message = %q, want removed update count", model.message)
	}
}

func TestAllVisibleToggleUsesRowsAfterAllFilters(t *testing.T) {
	model := Model{
		allRows: []domain.SkillRow{
			{Name: "match-visible", Source: domain.SourceLocal, Group: domain.GroupLocal, Claude: batchSkill(domain.SkillStateOn)},
			{Name: "match-wrong-source", Source: domain.SourceSymlinkRepo, Group: domain.GroupLocal, Claude: batchSkill(domain.SkillStateOn)},
			{Name: "match-wrong-group", Source: domain.SourceLocal, Group: domain.GroupLabel("android/skills"), Claude: batchSkill(domain.SkillStateOn)},
			{Name: "miss-text", Source: domain.SourceLocal, Group: domain.GroupLocal, Claude: batchSkill(domain.SkillStateOn)},
			{Name: "match-readonly", Source: domain.SourceLocal, Group: domain.GroupLocal, Codex: &domain.ToolSkill{State: domain.SkillStateReadOnly, ReadOnly: true}},
		},
		activeTool:   domain.ToolClaude,
		showReadOnly: true,
		textFilter:   "match",
		sourceFilter: domain.SourceLocal,
		groupFilter:  domain.GroupLocal,
	}
	model.rebuildRows()

	if got := rowNames(model.rows).String(); got != "match-visible,match-readonly" {
		t.Fatalf("visible rows = %s, want match-visible,match-readonly", got)
	}

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})

	assertPendingKind(t, model.pending, domain.ToolClaude, "match-visible", domain.OperationDisable)
	if len(model.pending) != 1 {
		t.Fatalf("pending = %#v, want only visible toggleable cell", model.pending)
	}
}

func TestAllVisibleToggleUsesFilteredRowsNotViewportRange(t *testing.T) {
	rows := make([]domain.SkillRow, 0, 8)
	for i := 0; i < 8; i++ {
		name := fmt.Sprintf("row-%d", i)
		rows = append(rows, domain.SkillRow{Name: name, Claude: batchSkill(domain.SkillStateOn)})
	}
	model := Model{
		rows:       rows,
		activeTool: domain.ToolClaude,
		cursor:     7,
		height:     8,
	}
	start, end := model.visibleRange()
	if end-start >= len(model.rows) {
		t.Fatalf("test setup visible range = %d-%d, want clipped viewport", start, end)
	}

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})

	if len(model.pending) != len(rows) {
		t.Fatalf("pending len = %d, want all filtered rows %d", len(model.pending), len(rows))
	}
	assertPendingKind(t, model.pending, domain.ToolClaude, "row-0", domain.OperationDisable)
	assertPendingKind(t, model.pending, domain.ToolClaude, "row-7", domain.OperationDisable)
}

func TestAllVisibleToggleNoopsForEmptyAndReadOnlyOnlyRows(t *testing.T) {
	model := Model{activeTool: domain.ToolClaude}

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})
	if model.message != "No visible skills." {
		t.Fatalf("message = %q, want no visible skills", model.message)
	}

	model.rows = []domain.SkillRow{
		{Name: "readonly", Codex: &domain.ToolSkill{State: domain.SkillStateReadOnly, ReadOnly: true}},
	}
	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})
	for _, want := range []string{"All visible rows: no applicable cells.", "Skipped 1 read-only", "1 missing"} {
		if !strings.Contains(model.message, want) {
			t.Fatalf("message = %q, want %q", model.message, want)
		}
	}
	if len(model.pending) != 0 {
		t.Fatalf("pending = %#v, want empty", model.pending)
	}
}

func TestAllVisibleToggleDoesNotApplyFilesystemChanges(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	activePath := filepath.Join(p.ClaudeUserSkills, "alpha")
	disabledPath := filepath.Join(p.ClaudeDisabledDir, "alpha")
	mkdirSkill(t, activePath)
	model := New(p)

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})

	assertPendingKind(t, model.pending, domain.ToolClaude, "alpha", domain.OperationDisable)
	if _, err := os.Lstat(activePath); err != nil {
		t.Fatalf("active path changed after A: %v", err)
	}
	if _, err := os.Lstat(disabledPath); !os.IsNotExist(err) {
		t.Fatalf("disabled path err after A = %v, want not exist", err)
	}
	if _, err := os.Lstat(p.StateFile); !os.IsNotExist(err) {
		t.Fatalf("state file err after A = %v, want not exist", err)
	}
}

func TestAllVisibleToggleKeyIsTextInputWhileEditingFilter(t *testing.T) {
	model := Model{
		rows: []domain.SkillRow{
			{Name: "alpha", Claude: batchSkill(domain.SkillStateOn)},
		},
		activeTool: domain.ToolClaude,
	}

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})

	if model.textFilter != "A" {
		t.Fatalf("textFilter = %q, want A", model.textFilter)
	}
	if len(model.pending) != 0 {
		t.Fatalf("pending = %#v, want empty while editing filter", model.pending)
	}
}

func TestInvalidSkillDirectoriesRemainHidden(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	mkdirSkill(t, filepath.Join(p.ClaudeUserSkills, "valid"))
	if err := os.MkdirAll(filepath.Join(p.ClaudeUserSkills, "invalid-entry"), 0o755); err != nil {
		t.Fatalf("mkdir invalid skill: %v", err)
	}

	model := New(p)

	if got := rowNames(model.rows).String(); got != "valid" {
		t.Fatalf("rows = %s, want valid", got)
	}
}

func TestInvalidReadOnlySkillDirectoriesRemainHidden(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	mkdirSkill(t, filepath.Join(p.ClaudeUserSkills, "valid"))
	mkdirSkill(t, filepath.Join(p.CodexSystemSkills, "imagegen"))
	if err := os.MkdirAll(filepath.Join(p.CodexSystemSkills, "invalid"), 0o755); err != nil {
		t.Fatalf("mkdir invalid read-only skill: %v", err)
	}
	model := New(p)

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})

	if got := rowNames(model.rows).String(); got != "imagegen,valid" {
		t.Fatalf("rows = %s, want imagegen,valid", got)
	}
}

func TestDetailsToggleShowsSelectedSkillDetails(t *testing.T) {
	model := Model{
		rows: []domain.SkillRow{
			{
				Name:        "alpha",
				Description: "Alpha description",
				Source:      domain.SourceSymlinkRepo,
				Group:       domain.GroupLabel("android/skills"),
				Claude: &domain.ToolSkill{
					Tool:          domain.ToolClaude,
					Name:          "alpha",
					State:         domain.SkillStateOn,
					Source:        domain.SourceSymlinkRepo,
					Group:         domain.GroupLabel("android/skills"),
					EntryType:     domain.EntryTypeSymlink,
					ActivePath:    "/tmp/home/.claude/skills/alpha",
					SkillFilePath: "/tmp/home/.claude/skills/alpha/SKILL.md",
					SymlinkTarget: "/tmp/repo/alpha",
					RepoOrigin:    "https://example.com/repo.git",
					RepoCommit:    "abc1234",
				},
				Codex: &domain.ToolSkill{
					Tool:         domain.ToolCodex,
					Name:         "alpha",
					State:        domain.SkillStateOff,
					Source:       domain.SourceSkillsCLI,
					Group:        domain.GroupSkillsCLI,
					EntryType:    domain.EntryTypeDir,
					DisabledPath: "/tmp/home/.skill-manager/disabled/codex/alpha",
				},
			},
		},
		activeTool: domain.ToolClaude,
		pending: map[pendingKey]domain.OperationKind{
			{tool: domain.ToolClaude, skillName: "alpha"}: domain.OperationDisable,
		},
	}

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})

	view := model.View()
	for _, want := range []string{
		"Details",
		"Skill: alpha",
		"Description: Alpha description",
		"Group: android/skills",
		"Source: symlink repo",
		"Claude:",
		"State: ON",
		"Pending: disable",
		"Group: android/skills",
		"Source: symlink repo",
		"Active path: /tmp/home/.claude/skills/alpha",
		"Skill file: /tmp/home/.claude/skills/alpha/SKILL.md",
		"Symlink target: /tmp/repo/alpha",
		"Repo origin: https://example.com/repo.git",
		"Repo commit: abc1234",
		"Codex:",
		"State: OFF",
		"Group: Skills CLI",
		"Source: Skills CLI",
		"Disabled path: /tmp/home/.skill-manager/disabled/codex/alpha",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("View() = %q, want %q", view, want)
		}
	}

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if model.showDetails {
		t.Fatalf("showDetails = true, want false after second d")
	}
}

func TestDetailsShowConflictFields(t *testing.T) {
	model := Model{
		rows: []domain.SkillRow{
			{
				Name:   "alpha",
				Source: domain.SourceLocal,
				Group:  domain.GroupLocal,
				Claude: &domain.ToolSkill{
					Tool:         domain.ToolClaude,
					Name:         "alpha",
					State:        domain.SkillStateConflict,
					Source:       domain.SourceLocal,
					Group:        domain.GroupLocal,
					EntryType:    domain.EntryTypeDir,
					DisabledPath: "/tmp/disabled/alpha",
					Conflict: &domain.Conflict{
						OriginalPath: "/tmp/active/alpha",
						DisabledPath: "/tmp/disabled/alpha",
						BlockerPath:  "/tmp/active/alpha",
						Message:      "restore blocked",
					},
				},
			},
		},
		activeTool: domain.ToolClaude,
	}

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})

	view := model.View()
	for _, want := range []string{
		"Conflict:",
		"Original path: /tmp/active/alpha",
		"Disabled path: /tmp/disabled/alpha",
		"Blocker path: /tmp/active/alpha",
		"Message: restore blocked",
		"Codex:",
		"not present",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("View() = %q, want %q", view, want)
		}
	}
}

func TestDetailsToggleIsReadOnly(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	activePath := filepath.Join(p.ClaudeUserSkills, "alpha")
	disabledPath := filepath.Join(p.ClaudeDisabledDir, "alpha")
	mkdirSkill(t, activePath)
	model := New(p)
	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
	pendingBefore := len(model.pending)

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})

	if !model.showDetails {
		t.Fatalf("showDetails = false, want true")
	}
	if len(model.pending) != pendingBefore {
		t.Fatalf("pending len = %d, want %d", len(model.pending), pendingBefore)
	}
	if _, err := os.Lstat(activePath); err != nil {
		t.Fatalf("active path changed after details toggle: %v", err)
	}
	if _, err := os.Lstat(disabledPath); !os.IsNotExist(err) {
		t.Fatalf("disabled path err after details toggle = %v, want not exist", err)
	}
}

func TestDetailsShowDisabledSkillMetadataFromScanner(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	activePath := filepath.Join(p.ClaudeUserSkills, "alpha")
	mkdirSkillWithContent(t, activePath, `---
name: "Alpha"
description: "Disabled alpha description"
---
`)
	model := New(p)

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyEnter})
	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})

	view := model.View()
	for _, want := range []string{
		"Description: Disabled alpha description",
		"Group: local",
		"Source: local",
		"State: OFF",
		"Disabled path: " + filepath.Join(p.ClaudeDisabledDir, "alpha"),
		"Skill file: " + filepath.Join(p.ClaudeDisabledDir, "alpha", "SKILL.md"),
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("View() = %q, want %q", view, want)
		}
	}
}

func TestRescanReloadsRowsFromDisk(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	mkdirSkill(t, filepath.Join(p.ClaudeUserSkills, "alpha"))
	model := New(p)

	mkdirSkill(t, filepath.Join(p.ClaudeUserSkills, "beta"))
	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})

	if got := rowNames(model.rows).String(); got != "alpha,beta" {
		t.Fatalf("rows after rescan = %s, want alpha,beta", got)
	}
	if !strings.Contains(model.message, "Rescanned") {
		t.Fatalf("message = %q, want rescan message", model.message)
	}
}

func TestUpdateTogglesActiveToolAndNavigates(t *testing.T) {
	model := Model{
		rows: []domain.SkillRow{
			{Name: "alpha"},
			{Name: "beta"},
		},
		activeTool: domain.ToolClaude,
	}

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = updated.(Model)
	if model.activeTool != domain.ToolCodex {
		t.Fatalf("activeTool = %q, want codex", model.activeTool)
	}
	if model.message != "Active column: Codex" {
		t.Fatalf("message = %q, want active column message", model.message)
	}
	if view := model.View(); !strings.Contains(view, "Active column: Codex") || !strings.Contains(view, "[Codex]") {
		t.Fatalf("View() after tab = %q, want visible Codex active indicator", view)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(Model)
	if model.cursor != 1 {
		t.Fatalf("cursor after down = %d, want 1", model.cursor)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(Model)
	if model.cursor != 1 {
		t.Fatalf("cursor after bounded down = %d, want 1", model.cursor)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	model = updated.(Model)
	if model.cursor != 0 {
		t.Fatalf("cursor after k = %d, want 0", model.cursor)
	}
}

func TestViewKeepsCursorVisibleWithinTerminalHeight(t *testing.T) {
	rows := make([]domain.SkillRow, 30)
	for i := range rows {
		rows[i] = domain.SkillRow{
			Name:   "skill-" + string(rune('a'+i%26)),
			Source: domain.SourceLocal,
			Claude: &domain.ToolSkill{
				Tool:  domain.ToolClaude,
				Name:  "skill",
				State: domain.SkillStateOn,
			},
		}
	}
	rows[20].Name = "selected-skill"

	model := Model{
		rows:       rows,
		cursor:     20,
		activeTool: domain.ToolClaude,
		height:     12,
	}

	view := model.View()
	if !strings.Contains(view, "selected-skill") {
		t.Fatalf("View() = %q, want selected row visible", view)
	}
	if !strings.Contains(view, "Rows:") {
		t.Fatalf("View() = %q, want clipped row range indicator", view)
	}
}

func TestUpdateHandlesEmptyRows(t *testing.T) {
	model := Model{activeTool: domain.ToolClaude}

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(Model)

	if model.cursor != 0 {
		t.Fatalf("cursor = %d, want 0", model.cursor)
	}
	if !strings.Contains(model.View(), "No skills found.") {
		t.Fatalf("View() = %q, want empty message", model.View())
	}
}

func TestSpaceTogglesPendingStateForActiveCell(t *testing.T) {
	model := Model{
		rows: []domain.SkillRow{
			{
				Name: "alpha",
				Claude: &domain.ToolSkill{
					Tool:  domain.ToolClaude,
					Name:  "alpha",
					State: domain.SkillStateOn,
				},
			},
		},
		activeTool: domain.ToolClaude,
	}

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})

	key := pendingKey{tool: domain.ToolClaude, skillName: "alpha"}
	if got := model.pending[key]; got != domain.OperationDisable {
		t.Fatalf("pending[%#v] = %q, want disable", key, got)
	}
	if !strings.Contains(model.View(), "[ON->OFF]") {
		t.Fatalf("View() = %q, want pending state", model.View())
	}

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
	if len(model.pending) != 0 {
		t.Fatalf("pending len = %d, want 0", len(model.pending))
	}
}

func TestBothTogglesPossibleCells(t *testing.T) {
	model := Model{
		rows: []domain.SkillRow{
			{
				Name: "shared",
				Claude: &domain.ToolSkill{
					Tool:  domain.ToolClaude,
					Name:  "shared",
					State: domain.SkillStateOn,
				},
				Codex: &domain.ToolSkill{
					Tool:  domain.ToolCodex,
					Name:  "shared",
					State: domain.SkillStateOff,
				},
			},
		},
		activeTool: domain.ToolClaude,
	}

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})

	if got := model.pending[pendingKey{tool: domain.ToolClaude, skillName: "shared"}]; got != domain.OperationDisable {
		t.Fatalf("claude pending = %q, want disable", got)
	}
	if got := model.pending[pendingKey{tool: domain.ToolCodex, skillName: "shared"}]; got != domain.OperationEnable {
		t.Fatalf("codex pending = %q, want enable", got)
	}
}

func TestUndoAndClearPendingChanges(t *testing.T) {
	model := Model{
		rows: []domain.SkillRow{
			{
				Name: "alpha",
				Claude: &domain.ToolSkill{
					Tool:  domain.ToolClaude,
					Name:  "alpha",
					State: domain.SkillStateOn,
				},
				Codex: &domain.ToolSkill{
					Tool:  domain.ToolCodex,
					Name:  "alpha",
					State: domain.SkillStateOn,
				},
			},
		},
		activeTool: domain.ToolClaude,
		pending: map[pendingKey]domain.OperationKind{
			{tool: domain.ToolClaude, skillName: "alpha"}: domain.OperationDisable,
			{tool: domain.ToolCodex, skillName: "alpha"}:  domain.OperationDisable,
		},
	}

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})
	if _, ok := model.pending[pendingKey{tool: domain.ToolClaude, skillName: "alpha"}]; ok {
		t.Fatalf("claude pending still present after undo: %#v", model.pending)
	}
	if _, ok := model.pending[pendingKey{tool: domain.ToolCodex, skillName: "alpha"}]; !ok {
		t.Fatalf("codex pending removed by active-cell undo: %#v", model.pending)
	}

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'U'}})
	if len(model.pending) != 0 {
		t.Fatalf("pending len = %d, want 0", len(model.pending))
	}
}

func TestQuitWarnsWhenPendingChangesExist(t *testing.T) {
	model := Model{
		rows:       []domain.SkillRow{{Name: "alpha"}},
		activeTool: domain.ToolClaude,
		pending: map[pendingKey]domain.OperationKind{
			{tool: domain.ToolClaude, skillName: "alpha"}: domain.OperationDisable,
		},
	}

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	model = updated.(Model)

	if cmd != nil {
		t.Fatalf("first q cmd = %#v, want nil", cmd)
	}
	if !model.confirmQuit {
		t.Fatalf("confirmQuit = false, want true")
	}
	if !strings.Contains(model.message, "Pending changes") {
		t.Fatalf("message = %q, want pending warning", model.message)
	}

	_, cmd = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatalf("second q cmd = nil, want quit command")
	}
}

func TestApplyPendingChangesMovesSkillAndRescans(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	activePath := filepath.Join(p.ClaudeUserSkills, "alpha")
	disabledPath := filepath.Join(p.ClaudeDisabledDir, "alpha")
	mkdirSkill(t, activePath)
	model := New(p)

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
	if len(model.pending) != 1 {
		t.Fatalf("pending len after toggle = %d, want 1", len(model.pending))
	}
	if _, err := os.Lstat(activePath); err != nil {
		t.Fatalf("active alpha changed before apply: %v", err)
	}
	if _, err := os.Lstat(disabledPath); !os.IsNotExist(err) {
		t.Fatalf("disabled alpha err before apply = %v, want not exist", err)
	}

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyEnter})

	if _, err := os.Lstat(activePath); !os.IsNotExist(err) {
		t.Fatalf("active alpha err = %v, want not exist", err)
	}
	if _, err := os.Lstat(disabledPath); err != nil {
		t.Fatalf("disabled alpha missing: %v", err)
	}
	if len(model.pending) != 0 {
		t.Fatalf("pending len after apply = %d, want 0", len(model.pending))
	}
	if len(model.rows) != 1 || model.rows[0].Claude == nil || model.rows[0].Claude.State != domain.SkillStateOff {
		t.Fatalf("rows after apply = %#v, want disabled alpha", model.rows)
	}
	if !strings.Contains(model.message, "Applied 1 change") {
		t.Fatalf("message = %q, want applied count", model.message)
	}
}

func TestApplyPendingChangesEnablesAndDisablesExpectedCells(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	disablePath := filepath.Join(p.CodexUserSkills, "disable-me")
	enableOriginalPath := filepath.Join(p.ClaudeUserSkills, "enable-me")
	enableDisabledPath := filepath.Join(p.ClaudeDisabledDir, "enable-me")
	mkdirSkill(t, disablePath)
	mkdirSkill(t, enableDisabledPath)
	if err := state.New(p).Save(state.Manifest{Disabled: []state.DisabledEntry{
		{
			Tool:         domain.ToolClaude,
			SkillName:    "enable-me",
			OriginalPath: enableOriginalPath,
			DisabledPath: enableDisabledPath,
			EntryType:    domain.EntryTypeDir,
			Source:       domain.SourceLocal,
			DisabledAt:   time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC),
		},
	}}); err != nil {
		t.Fatalf("save state: %v", err)
	}
	model := New(p)
	model.pending = map[pendingKey]domain.OperationKind{
		{tool: domain.ToolCodex, skillName: "disable-me"}: domain.OperationDisable,
		{tool: domain.ToolClaude, skillName: "enable-me"}: domain.OperationEnable,
	}

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyEnter})

	if _, err := os.Lstat(disablePath); !os.IsNotExist(err) {
		t.Fatalf("codex disable-me active err = %v, want not exist", err)
	}
	if _, err := os.Lstat(filepath.Join(p.CodexDisabledDir, "disable-me")); err != nil {
		t.Fatalf("codex disable-me disabled missing: %v", err)
	}
	if _, err := os.Lstat(enableOriginalPath); err != nil {
		t.Fatalf("claude enable-me active missing: %v", err)
	}
	if _, err := os.Lstat(enableDisabledPath); !os.IsNotExist(err) {
		t.Fatalf("claude enable-me disabled err = %v, want not exist", err)
	}
	if len(model.pending) != 0 {
		t.Fatalf("pending len after mixed apply = %d, want 0", len(model.pending))
	}
	if got := rowNames(model.rows).String(); got != "disable-me,enable-me" {
		t.Fatalf("rows after mixed apply = %s, want disable-me,enable-me", got)
	}
	if model.rows[0].Codex == nil || model.rows[0].Codex.State != domain.SkillStateOff {
		t.Fatalf("disable row = %#v, want Codex OFF", model.rows[0])
	}
	if model.rows[1].Claude == nil || model.rows[1].Claude.State != domain.SkillStateOn {
		t.Fatalf("enable row = %#v, want Claude ON", model.rows[1])
	}
}

func TestOrderedPendingKeysUsesApplyOrder(t *testing.T) {
	model := Model{
		pending: map[pendingKey]domain.OperationKind{
			{tool: domain.ToolCodex, skillName: "b"}:  domain.OperationEnable,
			{tool: domain.ToolClaude, skillName: "z"}: domain.OperationDisable,
			{tool: domain.ToolClaude, skillName: "a"}: domain.OperationEnable,
			{tool: domain.ToolCodex, skillName: "a"}:  domain.OperationDisable,
		},
	}

	got := model.orderedPendingKeys()
	want := []pendingKey{
		{tool: domain.ToolClaude, skillName: "z"},
		{tool: domain.ToolCodex, skillName: "a"},
		{tool: domain.ToolClaude, skillName: "a"},
		{tool: domain.ToolCodex, skillName: "b"},
	}

	if len(got) != len(want) {
		t.Fatalf("ordered len = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ordered[%d] = %#v, want %#v", i, got[i], want[i])
		}
	}
}

func TestHiddenPendingChangesStillApply(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	activePath := filepath.Join(p.ClaudeUserSkills, "alpha")
	disabledPath := filepath.Join(p.ClaudeDisabledDir, "alpha")
	mkdirSkill(t, activePath)
	mkdirSkill(t, filepath.Join(p.ClaudeUserSkills, "beta"))
	model := New(p)

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyEnter})
	if got := rowNames(model.rows).String(); got != "beta" {
		t.Fatalf("filtered rows = %s, want beta", got)
	}

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyEnter})

	if _, err := os.Lstat(activePath); !os.IsNotExist(err) {
		t.Fatalf("active alpha err = %v, want not exist", err)
	}
	if _, err := os.Lstat(disabledPath); err != nil {
		t.Fatalf("disabled alpha missing: %v", err)
	}
	if len(model.pending) != 0 {
		t.Fatalf("pending len after hidden apply = %d, want 0", len(model.pending))
	}
}

func batchSkill(state domain.SkillState) *domain.ToolSkill {
	return &domain.ToolSkill{State: state}
}

func assertPendingKind(t *testing.T, pending map[pendingKey]domain.OperationKind, tool domain.Tool, skillName string, want domain.OperationKind) {
	t.Helper()
	got, ok := pending[pendingKey{tool: tool, skillName: skillName}]
	if !ok {
		t.Fatalf("pending %s/%s missing; pending=%#v", tool, skillName, pending)
	}
	if got != want {
		t.Fatalf("pending %s/%s = %q, want %q; pending=%#v", tool, skillName, got, want, pending)
	}
}

func updateModel(t *testing.T, model Model, msg tea.KeyMsg) Model {
	t.Helper()
	updated, _ := model.Update(msg)
	return updated.(Model)
}

type rowNameList []string

func rowNames(rows []domain.SkillRow) rowNameList {
	names := make([]string, 0, len(rows))
	for _, row := range rows {
		names = append(names, row.Name)
	}
	return rowNameList(names)
}

func (names rowNameList) String() string {
	return strings.Join(names, ",")
}

func mkdirSkill(t *testing.T, dir string) {
	t.Helper()
	mkdirSkillWithContent(t, dir, "# Skill\n")
}

func mkdirSkillWithContent(t *testing.T, dir, contents string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(contents), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}
}
