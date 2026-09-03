package tui

import (
	"testing"

	"github.com/dees91/agent-skill-manager/internal/model"
)

func TestFixedTableWidthWithoutSkillTracksToolCount(t *testing.T) {
	tools := len(model.Tools())
	separators := tools + 2
	want := 2 + separators + tools*stateColumnWidth
	if got := fixedTableWidthWithoutSkill(); got != want {
		t.Fatalf("fixedTableWidthWithoutSkill() = %d, want %d for %d tools", got, want, tools)
	}
}
