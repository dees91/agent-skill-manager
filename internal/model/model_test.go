package model

import "testing"

func TestParseTool(t *testing.T) {
	tests := []struct {
		value string
		want  Tool
		ok    bool
	}{
		{value: "claude", want: ToolClaude, ok: true},
		{value: "codex", want: ToolCodex, ok: true},
		{value: "muse", want: ToolMuse, ok: true},
		{value: "other", ok: false},
		{value: "", ok: false},
	}

	for _, test := range tests {
		t.Run(test.value, func(t *testing.T) {
			got, ok := ParseTool(test.value)
			if ok != test.ok {
				t.Fatalf("ParseTool(%q) ok = %v, want %v", test.value, ok, test.ok)
			}
			if got != test.want {
				t.Fatalf("ParseTool(%q) = %q, want %q", test.value, got, test.want)
			}
		})
	}
}

func TestRequiredStateSourceAndGroupLabels(t *testing.T) {
	for _, state := range []SkillState{
		SkillStateOn,
		SkillStateOff,
		SkillStateReadOnly,
		SkillStateMissing,
		SkillStatePending,
		SkillStateConflict,
	} {
		if state.String() == "" {
			t.Fatalf("state %q has empty display string", state)
		}
	}

	for _, source := range []SourceLabel{
		SourceSymlinkRepo,
		SourceSkillsCLI,
		SourceLocal,
		SourceLocalPath,
		SourceCodexSystem,
		SourceClaudePlugin,
		SourceUnknown,
	} {
		if source.String() == "" {
			t.Fatalf("source %q has empty display string", source)
		}
	}

	for _, group := range []GroupLabel{
		GroupLabel("android/skills"),
		GroupLabel("skydoves/compose-performance-skills"),
		GroupSkillsCLI,
		GroupLocal,
		GroupCodexSystem,
		GroupClaudePlugin,
		GroupUnknown,
	} {
		if group.String() == "" {
			t.Fatalf("group %q has empty display string", group)
		}
	}
}

func TestToolSkillToggleable(t *testing.T) {
	tests := []struct {
		name  string
		skill ToolSkill
		want  bool
	}{
		{
			name:  "managed on",
			skill: ToolSkill{State: SkillStateOn},
			want:  true,
		},
		{
			name:  "managed off",
			skill: ToolSkill{State: SkillStateOff},
			want:  true,
		},
		{
			name:  "read only flag",
			skill: ToolSkill{State: SkillStateOn, ReadOnly: true},
			want:  false,
		},
		{
			name:  "read only state",
			skill: ToolSkill{State: SkillStateReadOnly},
			want:  false,
		},
		{
			name:  "missing",
			skill: ToolSkill{State: SkillStateMissing},
			want:  false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.skill.Toggleable(); got != test.want {
				t.Fatalf("Toggleable() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestSkillTypesExposeGroupLabels(t *testing.T) {
	toolSkill := ToolSkill{Group: GroupSkillsCLI}
	if toolSkill.Group != GroupSkillsCLI {
		t.Fatalf("ToolSkill.Group = %q, want %q", toolSkill.Group, GroupSkillsCLI)
	}

	row := SkillRow{Group: GroupLabel("android/skills")}
	if row.Group != GroupLabel("android/skills") {
		t.Fatalf("SkillRow.Group = %q, want android/skills", row.Group)
	}
}
