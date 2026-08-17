package gui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dees91/agent-skill-manager/internal/model"
	"github.com/dees91/agent-skill-manager/internal/paths"
)

func TestSkillSetCRUDProjectionAndMissingMemberRetention(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	alphaClaude := filepath.Join(p.ClaudeUserSkills, "alpha")
	writeSkill(t, alphaClaude, "Alpha automation")
	writeSkill(t, filepath.Join(p.CodexUserSkills, "alpha"), "Alpha automation")
	writeSkill(t, filepath.Join(p.CodexUserSkills, "beta"), "Beta helper")

	service := New(p)
	if _, err := service.GetSnapshot(false); err != nil {
		t.Fatal(err)
	}
	created, err := service.CreateSkillSet(" Video production ", " Use for occasional media. ", []string{"beta", "alpha", "alpha"})
	if err != nil {
		t.Fatal(err)
	}
	if len(created.SkillSets) != 1 {
		t.Fatalf("created = %#v", created)
	}
	set := created.SkillSets[0]
	if set.Name != "Video production" || set.Description != "Use for occasional media." || len(set.Members) != 2 {
		t.Fatalf("set = %#v", set)
	}
	if set.Claude.AppliedStatus != skillSetStatusEnabled || set.Claude.Missing != 1 || set.Codex.AppliedStatus != skillSetStatusEnabled {
		t.Fatalf("tool summaries = claude %#v codex %#v", set.Claude, set.Codex)
	}
	if _, err := service.CreateSkillSet("video PRODUCTION", "", []string{"alpha"}); err == nil {
		t.Fatal("accepted duplicate set name")
	}
	if _, err := service.CreateSkillSet("Unknown", "", []string{"not-installed"}); err == nil {
		t.Fatal("accepted an unknown new member")
	}
	service.rows = append(service.rows, model.SkillRow{Name: "conflict-only", Codex: &model.ToolSkill{Name: "conflict-only", Tool: model.ToolCodex, State: model.SkillStateConflict}})
	if _, err := service.CreateSkillSet("Conflict", "", []string{"conflict-only"}); err == nil {
		t.Fatal("accepted a non-toggleable conflict as a new member")
	}

	if err := os.RemoveAll(alphaClaude); err != nil {
		t.Fatal(err)
	}
	snapshot, err := service.GetSnapshot(false)
	if err != nil {
		t.Fatal(err)
	}
	set = snapshot.SkillSets[0]
	if set.Claude.AppliedStatus != skillSetStatusUnavailable || set.Unavailable != 0 {
		t.Fatalf("missing projection = %#v", set)
	}
	if _, err := service.UpdateSkillSet(set.SetID, "Video production", "Updated", []string{"alpha", "beta"}); err != nil {
		t.Fatalf("existing missing member was not retained: %v", err)
	}

	writeSkill(t, alphaClaude, "Alpha automation")
	snapshot, err = service.GetSnapshot(false)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.SkillSets[0].Claude.AppliedStatus != skillSetStatusEnabled {
		t.Fatalf("reinstalled basename did not reconnect: %#v", snapshot.SkillSets[0].Claude)
	}
}

func TestSkillSetPreviewToggleOverlapAndDeletePreservePending(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	for _, name := range []string{"alpha", "beta"} {
		writeSkill(t, filepath.Join(p.CodexUserSkills, name), name)
	}
	service := New(p)
	if _, err := service.GetSnapshot(false); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ToggleCell("beta", "codex"); err != nil {
		t.Fatal(err)
	}
	if result := service.ApplyPending(false); result.Failure != nil {
		t.Fatalf("disable beta = %#v", result)
	}
	first, err := service.CreateSkillSet("First", "", []string{"alpha", "beta"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.CreateSkillSet("Second", "", []string{"beta"})
	if err != nil {
		t.Fatal(err)
	}
	firstID, secondID := first.SkillSets[0].SetID, ""
	for _, set := range second.SkillSets {
		if set.Name == "Second" {
			secondID = set.SetID
		}
	}

	preview, err := service.PreviewSkillSetToggle(firstID, []string{"codex"})
	if err != nil {
		t.Fatal(err)
	}
	if preview.Direction != "enable" || preview.Eligible != 2 || preview.Counts.Changed != 1 {
		t.Fatalf("preview = %#v", preview)
	}
	action, err := service.ToggleSkillSet(firstID, []string{"codex"})
	if err != nil {
		t.Fatal(err)
	}
	if len(action.Pending) != 1 || action.Pending[0].SkillName != "beta" || action.SkillSets[0].Codex.EffectiveStatus != skillSetStatusEnabled {
		t.Fatalf("action = %#v", action)
	}
	var overlapping SkillSet
	for _, set := range action.SkillSets {
		if set.SetID == secondID {
			overlapping = set
		}
	}
	if overlapping.Codex.EffectiveStatus != skillSetStatusEnabled || overlapping.Pending != 1 {
		t.Fatalf("overlap projection = %#v", overlapping)
	}

	deleted, err := service.DeleteSkillSet(firstID)
	if err != nil {
		t.Fatal(err)
	}
	if len(deleted.SkillSets) != 1 || service.PendingCount() != 1 {
		t.Fatalf("delete changed Pending or wrong sets: %#v pending=%d", deleted, service.PendingCount())
	}
}

func TestSkillSetCorruptionIsIsolatedFromCoreSnapshot(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	writeSkill(t, filepath.Join(p.ClaudeUserSkills, "alpha"), "Alpha")
	if err := os.MkdirAll(p.StateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.SkillSetsFile, []byte(`{"version":1,"sets":[`), 0o600); err != nil {
		t.Fatal(err)
	}
	service := New(p)
	snapshot, err := service.GetSnapshot(false)
	if err != nil {
		t.Fatalf("core snapshot was blocked: %v", err)
	}
	if len(snapshot.Rows) != 1 || snapshot.SkillSetsWarning == "" || len(snapshot.SkillSets) != 0 {
		t.Fatalf("snapshot = %#v", snapshot)
	}
	if _, err := service.ToggleCell("alpha", "claude"); err != nil {
		t.Fatalf("core toggle was blocked: %v", err)
	}
	if _, err := service.CreateSkillSet("Broken", "", []string{"alpha"}); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("CreateSkillSet() error = %v", err)
	}
}

func TestSourceUninstallReportsAndRetainsSkillSetImpact(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	sourcePath := filepath.Join(p.Home, "workspace", "media-pack")
	writeSkill(t, filepath.Join(sourcePath, "skills", "media-compose"), "Compose media")
	service := New(p)
	draft, err := service.PrepareLocalInstall(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	review, err := service.ReviewInstall(draft.DraftID, []InstallCellRequest{{SkillName: "media-compose", Tool: "codex"}})
	if err != nil {
		t.Fatal(err)
	}
	installed := service.ApplyInstall(review.ReviewID, false)
	if installed.Failure != nil {
		t.Fatalf("install = %#v", installed)
	}
	created, err := service.CreateSkillSet("Media", "", []string{"media-compose"})
	if err != nil {
		t.Fatal(err)
	}
	setID := created.SkillSets[0].SetID
	sourceID := installed.Snapshot.ManagedSources[0].SourceID
	preview, err := service.PreviewUninstall(sourceID)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.AffectedSkillSets) != 1 || preview.AffectedSkillSets[0].SetID != setID || preview.AffectedSkillSets[0].Skills[0] != "media-compose" {
		t.Fatalf("preview impact = %#v", preview)
	}
	removed := service.UninstallSource(sourceID, "media-pack", false)
	if removed.Failure != nil || len(removed.Snapshot.SkillSets) != 1 || removed.Snapshot.SkillSets[0].Unavailable != 1 {
		t.Fatalf("uninstall result = %#v", removed)
	}
}
