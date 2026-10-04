package gui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dees91/agent-skill-manager/internal/install"
	"github.com/dees91/agent-skill-manager/internal/model"
	"github.com/dees91/agent-skill-manager/internal/state"
)

// The desktop remedy flow: update stops on a skill the repository dropped, the
// failure names it, the preview lists the source's skills with their impact,
// removal of the named skill succeeds, and the next update fast-forwards.
func TestUpdateFailureOffersRemovalOfSkillDroppedUpstream(t *testing.T) {
	fixture := newRepairFixture(t)
	addRepairFixtureSkill(t, fixture, "beta")
	if err := os.RemoveAll(filepath.Join(fixture.sourcePath, "skills", "beta")); err != nil {
		t.Fatalf("remove beta upstream: %v", err)
	}
	runGitRepair(t, "-C", fixture.sourcePath, "add", "-A")
	runGitRepair(t, "-C", fixture.sourcePath, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "remove beta")
	runGitRepair(t, "-C", fixture.sourcePath, "push", "origin", "main")
	service := New(fixture.paths)
	if _, err := service.SetSkillFavorite("beta", true); err != nil {
		t.Fatalf("SetSkillFavorite() error = %v", err)
	}

	result := service.UpdateAllSources(false)

	failure := result.Failure
	if failure == nil || failure.Kind != install.SkillMissingUpstreamKind {
		t.Fatalf("failure = %#v, want %s", failure, install.SkillMissingUpstreamKind)
	}
	if failure.Repairable || failure.SourceID == "" || strings.Join(failure.MissingSkills, ",") != "beta" || failure.Cause == "" || failure.Remedy == "" {
		t.Fatalf("failure = %#v, want the missing skill, cause, and remedy", failure)
	}

	preview, err := service.PreviewRemoveSkills(failure.SourceID)
	if err != nil {
		t.Fatalf("PreviewRemoveSkills() error = %v", err)
	}
	if len(preview.Skills) != 2 || preview.Group != "owner/repo" {
		t.Fatalf("preview = %#v, want both recorded skills", preview)
	}
	beta := preview.Skills[1]
	if beta.Name != "beta" || beta.ActiveLinks != 1 || beta.DisabledLinks != 0 || !beta.Favorite {
		t.Fatalf("beta = %#v, want one active link and a favorite", beta)
	}
	if preview.Skills[0].Favorite {
		t.Fatalf("alpha = %#v, want no favorite", preview.Skills[0])
	}

	removed := service.RemoveSourceSkills(failure.SourceID, failure.MissingSkills, false)
	if removed.Failure != nil || removed.RemovedActive != 1 {
		t.Fatalf("RemoveSourceSkills() = %#v", removed)
	}
	if _, err := os.Lstat(filepath.Join(fixture.paths.ClaudeUserSkills, "beta")); !os.IsNotExist(err) {
		t.Fatalf("beta link remains: %v", err)
	}

	if updated := service.UpdateAllSources(false); updated.Failure != nil {
		t.Fatalf("update after removal failure = %#v", updated.Failure)
	}
}

func TestRemoveSourceSkillsRejectsEverySkill(t *testing.T) {
	fixture := newRepairFixture(t)
	service := New(fixture.paths)
	snapshot, err := service.GetSnapshot(false)
	if err != nil {
		t.Fatalf("GetSnapshot() error = %v", err)
	}

	result := service.RemoveSourceSkills(snapshot.ManagedSources[0].SourceID, []string{"alpha"}, false)

	if result.Failure == nil || !strings.Contains(result.Failure.Message, "uninstall the source") {
		t.Fatalf("failure = %#v, want the every-skill guard", result.Failure)
	}
	if _, err := os.Lstat(filepath.Join(fixture.paths.ClaudeUserSkills, "alpha")); err != nil {
		t.Fatalf("alpha link changed: %v", err)
	}
}

// addRepairFixtureSkill publishes one more skill, fast-forwards the checkout,
// links it to Claude, and records it.
func addRepairFixtureSkill(t *testing.T, fixture repairFixture, name string) {
	t.Helper()
	writeRepairFile(t, filepath.Join(fixture.sourcePath, "skills", name, "SKILL.md"), "---\nname: "+name+"\ndescription: "+name+"\n---\n")
	runGitRepair(t, "-C", fixture.sourcePath, "add", ".")
	runGitRepair(t, "-C", fixture.sourcePath, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "add "+name)
	runGitRepair(t, "-C", fixture.sourcePath, "push", "origin", "main")
	runGitRepair(t, "-C", fixture.checkoutPath, "pull", "--ff-only")
	if err := os.Symlink(filepath.Join(fixture.checkoutPath, "skills", name), filepath.Join(fixture.paths.ClaudeUserSkills, name)); err != nil {
		t.Fatalf("link %s: %v", name, err)
	}
	store := state.New(fixture.paths)
	manifest, err := store.Load()
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	repository := manifest.Repositories[0]
	repository.InstalledSkills = append(repository.InstalledSkills, state.InstalledSkillEntry{Name: name, RelativePath: "skills/" + name, Tools: []model.Tool{model.ToolClaude}})
	repository.LastSeenCommit = strings.TrimSpace(runGitRepair(t, "-C", fixture.checkoutPath, "rev-parse", "HEAD"))
	manifest.UpsertRepository(repository)
	if err := store.Save(manifest); err != nil {
		t.Fatalf("save state: %v", err)
	}
}
