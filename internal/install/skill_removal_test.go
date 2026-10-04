package install

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dees91/agent-skill-manager/internal/model"
	"github.com/dees91/agent-skill-manager/internal/state"
)

// newTwoSkillGitFixture extends the update fixture with a second skill "beta"
// linked to Claude under the install name "beta-acme".
func newTwoSkillGitFixture(t *testing.T) updateGitFixture {
	t.Helper()
	fixture := newUpdateGitFixture(t)
	commit := fixture.commitRemoteChange("add beta", map[string]string{"skills/beta/SKILL.md": "# beta\n"}, nil)
	fixture.gitCheckout("fetch", "origin")
	fixture.gitCheckout("merge", "--ff-only", commit)
	mustSymlink(t, filepath.Join(fixture.checkoutPath, "skills", "beta"), filepath.Join(fixture.paths.ClaudeUserSkills, "beta-acme"))
	fixture.repository.LastSeenCommit = commit
	fixture.repository.InstalledSkills = append(fixture.repository.InstalledSkills, state.InstalledSkillEntry{
		Name:         "beta",
		InstalledAs:  "beta-acme",
		RelativePath: "skills/beta",
		Tools:        []model.Tool{model.ToolClaude},
	})
	if err := state.New(fixture.paths).Save(state.Manifest{Repositories: []state.RepositoryEntry{fixture.repository}}); err != nil {
		t.Fatalf("save state: %v", err)
	}
	fixture.initialCommit = commit
	return fixture
}

func recordedSkillNames(t *testing.T, fixture updateGitFixture) []string {
	t.Helper()
	manifest, err := state.New(fixture.paths).Load()
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	repository, ok := manifest.GetRepository(fixture.repository.Host, fixture.repository.RepoPath)
	if !ok {
		t.Fatal("repository missing from state")
	}
	names := []string{}
	for _, skill := range repository.InstalledSkills {
		names = append(names, skill.Name)
	}
	return names
}

func TestSkillRemovalRemovesOneAliasedSkillAndKeepsSource(t *testing.T) {
	fixture := newTwoSkillGitFixture(t)
	betaLink := filepath.Join(fixture.paths.ClaudeUserSkills, "beta-acme")

	result, err := NewSkillRemovalService(fixture.paths).ApplyRepository(fixture.repository, []string{"beta"})
	if err != nil {
		t.Fatalf("ApplyRepository() error = %v", err)
	}
	if len(result.RemovedActive) != 1 || result.RemovedActive[0].InstalledName != "beta-acme" {
		t.Fatalf("removed active = %#v, want beta-acme", result.RemovedActive)
	}
	if _, err := os.Lstat(betaLink); !os.IsNotExist(err) {
		t.Fatalf("beta link remains: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(fixture.paths.ClaudeUserSkills, "alpha")); err != nil {
		t.Fatalf("alpha link changed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(fixture.checkoutPath, "skills", "beta", "SKILL.md")); err != nil {
		t.Fatalf("checkout changed: %v", err)
	}
	if got := recordedSkillNames(t, fixture); strings.Join(got, ",") != "alpha" {
		t.Fatalf("recorded skills = %v, want [alpha]", got)
	}
	entries, err := os.ReadDir(fixture.paths.TrashDir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("trash entries = %v err=%v, want empty", entries, err)
	}
}

func TestSkillRemovalByInstallNameRemovesDisabledRecord(t *testing.T) {
	fixture := newTwoSkillGitFixture(t)
	activePath := filepath.Join(fixture.paths.ClaudeUserSkills, "beta-acme")
	disabledPath := filepath.Join(fixture.paths.ClaudeDisabledDir, "beta-acme")
	betaPath := filepath.Join(fixture.checkoutPath, "skills", "beta")
	if err := os.Remove(activePath); err != nil {
		t.Fatalf("remove active link: %v", err)
	}
	mustSymlink(t, betaPath, disabledPath)
	manifest, err := state.New(fixture.paths).Load()
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	manifest.Upsert(state.DisabledEntry{
		Tool: model.ToolClaude, SkillName: "beta-acme", OriginalPath: activePath, DisabledPath: disabledPath,
		EntryType: model.EntryTypeSymlink, SymlinkTarget: betaPath, Source: model.SourceSymlinkRepo, Group: fixture.repository.Group,
	})
	if err := state.New(fixture.paths).Save(manifest); err != nil {
		t.Fatalf("save disabled state: %v", err)
	}

	result, err := NewSkillRemovalService(fixture.paths).ApplyRepository(fixture.repository, []string{"beta-acme"})
	if err != nil {
		t.Fatalf("ApplyRepository() error = %v", err)
	}
	if len(result.RemovedDisabled) != 1 || len(result.RemovedActive) != 0 {
		t.Fatalf("result = %#v, want one disabled link", result)
	}
	if _, err := os.Lstat(disabledPath); !os.IsNotExist(err) {
		t.Fatalf("disabled link remains: %v", err)
	}
	loaded, err := state.New(fixture.paths).Load()
	if err != nil {
		t.Fatalf("load final state: %v", err)
	}
	if len(loaded.Disabled) != 0 {
		t.Fatalf("disabled records = %#v, want none", loaded.Disabled)
	}
}

func TestSkillRemovalRejectsInvalidSelectionWithoutChange(t *testing.T) {
	for _, tc := range []struct {
		name  string
		names []string
		want  string
	}{
		{name: "unknown", names: []string{"gamma"}, want: `"gamma" is not recorded`},
		{name: "every skill", names: []string{"alpha", "beta"}, want: "uninstall the source"},
		{name: "same skill twice", names: []string{"beta", "beta-acme"}, want: "selected more than once"},
		{name: "empty", names: nil, want: "select at least one skill"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newTwoSkillGitFixture(t)
			before, _ := os.ReadFile(fixture.paths.StateFile)

			_, err := NewSkillRemovalService(fixture.paths).ApplyRepository(fixture.repository, tc.names)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ApplyRepository() error = %v, want %q", err, tc.want)
			}
			after, _ := os.ReadFile(fixture.paths.StateFile)
			if string(after) != string(before) {
				t.Fatal("state changed after rejected selection")
			}
			if _, err := os.Lstat(filepath.Join(fixture.paths.ClaudeUserSkills, "beta-acme")); err != nil {
				t.Fatalf("beta link changed: %v", err)
			}
		})
	}
}

func TestSkillRemovalRollsBackLinksWhenStateSaveFails(t *testing.T) {
	fixture := newTwoSkillGitFixture(t)
	before, _ := os.ReadFile(fixture.paths.StateFile)
	service := NewSkillRemovalService(fixture.paths)
	service.saveManifest = func(state.Manifest) error { return errors.New("disk full") }

	result, err := service.ApplyRepository(fixture.repository, []string{"beta"})
	if err == nil || !strings.Contains(err.Error(), "disk full") {
		t.Fatalf("ApplyRepository() error = %v, want save failure", err)
	}
	if len(result.RolledBack) != 1 || result.CleanupPending != "" {
		t.Fatalf("result = %#v, want one rolled back link and no pending cleanup", result)
	}
	if _, err := os.Lstat(filepath.Join(fixture.paths.ClaudeUserSkills, "beta-acme")); err != nil {
		t.Fatalf("beta link not restored: %v", err)
	}
	after, _ := os.ReadFile(fixture.paths.StateFile)
	if string(after) != string(before) {
		t.Fatal("state changed after failed save")
	}
}

// A repository that drops an installed skill blocks update with a typed error;
// removing that skill from the source lets the same update succeed.
func TestSkillRemovalUnblocksUpdateAfterUpstreamRemovedSkill(t *testing.T) {
	fixture := newTwoSkillGitFixture(t)
	target := fixture.commitRemoteChange("remove beta", nil, []string{"skills/beta/SKILL.md"})

	_, err := NewUpdateService(fixture.paths, nil).Apply(fixture.repository)
	var missing MissingUpstreamSkillsError
	if !errors.As(err, &missing) {
		t.Fatalf("Apply() error = %v, want MissingUpstreamSkillsError", err)
	}
	if missing.Kind() != SkillMissingUpstreamKind || len(missing.Skills) != 1 {
		t.Fatalf("missing = %#v, want one %s skill", missing, SkillMissingUpstreamKind)
	}
	if got := missing.Skills[0]; got.Name != "beta" || got.InstalledName() != "beta-acme" || got.RelativePath != "skills/beta" {
		t.Fatalf("missing skill = %#v", got)
	}

	if _, err := NewSkillRemovalService(fixture.paths).ApplyRepository(fixture.repository, []string{missing.Skills[0].Name}); err != nil {
		t.Fatalf("ApplyRepository() error = %v", err)
	}
	result, err := NewUpdateService(fixture.paths, nil).Apply(fixture.repository)
	if err != nil {
		t.Fatalf("update after removal error = %v", err)
	}
	if !result.Updated || result.CurrentCommit != target {
		t.Fatalf("update result = %#v, want fast-forward to %s", result, target)
	}
}

func TestSkillRemovalRemovesOneLocalSkill(t *testing.T) {
	p, source, discovered := localInstallFixture(t, "alpha", "beta")
	plan, err := PlanLocalInstall(p, state.Manifest{}, source, discovered, PlanOptions{Tools: []model.Tool{model.ToolClaude}})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if _, err := NewLocalApplyService(p).Apply(plan); err != nil {
		t.Fatalf("apply: %v", err)
	}
	manifest, _ := state.New(p).Load()
	entry, _ := manifest.GetLocalSource(source.CanonicalPath)

	result, err := NewSkillRemovalService(p).ApplyLocal(entry, []string{"beta"})
	if err != nil {
		t.Fatalf("ApplyLocal() error = %v", err)
	}
	if len(result.RemovedActive) != 1 {
		t.Fatalf("removed active = %#v, want one", result.RemovedActive)
	}
	if _, err := os.Lstat(filepath.Join(p.ClaudeUserSkills, "beta")); !os.IsNotExist(err) {
		t.Fatalf("beta link remains: %v", err)
	}
	if _, err := os.Stat(filepath.Join(source.CanonicalPath, "skills", "beta", "SKILL.md")); err != nil {
		t.Fatalf("local source changed: %v", err)
	}
	loaded, _ := state.New(p).Load()
	current, _ := loaded.GetLocalSource(source.CanonicalPath)
	if len(current.InstalledSkills) != 1 || current.InstalledSkills[0].Name != "alpha" {
		t.Fatalf("recorded skills = %#v, want only alpha", current.InstalledSkills)
	}
}
