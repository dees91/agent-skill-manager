package staging

import (
	"testing"

	"github.com/dees91/agent-skill-manager/internal/model"
)

func TestToggleCellRoundTrip(t *testing.T) {
	store := Memory{}
	row := model.SkillRow{Name: "alpha", Claude: &model.ToolSkill{State: model.SkillStateOn}}

	first := ToggleCell(store, row, model.ToolClaude)
	if !first.Changed || first.Removed || store[Key{Tool: model.ToolClaude, SkillName: "alpha"}] != model.OperationDisable {
		t.Fatalf("first toggle = %#v, store=%#v", first, store)
	}
	second := ToggleCell(store, row, model.ToolClaude)
	if !second.Changed || !second.Removed || len(store) != 0 {
		t.Fatalf("second toggle = %#v, store=%#v", second, store)
	}
}

func TestToggleBatchMixedScopeAndSkips(t *testing.T) {
	store := Memory{}
	rows := []model.SkillRow{
		{Name: "mixed", Claude: &model.ToolSkill{State: model.SkillStateOn}, Codex: &model.ToolSkill{State: model.SkillStateOff}},
		{Name: "readonly", Claude: &model.ToolSkill{State: model.SkillStateReadOnly, ReadOnly: true}},
		{Name: "conflict", Claude: &model.ToolSkill{State: model.SkillStateConflict, Conflict: &model.Conflict{}}},
	}

	result := ToggleBatch(store, rows, model.Tools())
	if result.Changed != 1 || result.SkippedReadOnly != 1 || result.SkippedConflict != 1 || result.SkippedMissing != 5 {
		t.Fatalf("result = %#v", result)
	}
	if got := store[Key{Tool: model.ToolCodex, SkillName: "mixed"}]; got != model.OperationEnable {
		t.Fatalf("pending = %q, want enable", got)
	}
}

func TestToggleBatchActsAsBatchUndo(t *testing.T) {
	store := Memory{
		{Tool: model.ToolClaude, SkillName: "alpha"}: model.OperationDisable,
		{Tool: model.ToolClaude, SkillName: "beta"}:  model.OperationDisable,
	}
	rows := []model.SkillRow{
		{Name: "alpha", Claude: &model.ToolSkill{State: model.SkillStateOn}},
		{Name: "beta", Claude: &model.ToolSkill{State: model.SkillStateOn}},
	}

	result := ToggleBatch(store, rows, []model.Tool{model.ToolClaude})
	if result.Removed != 2 || result.Changed != 0 || len(store) != 0 {
		t.Fatalf("result = %#v, store=%#v", result, store)
	}
}
