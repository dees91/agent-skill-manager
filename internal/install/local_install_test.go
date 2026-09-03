package install

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dees91/agent-skill-manager/internal/model"
	"github.com/dees91/agent-skill-manager/internal/paths"
	"github.com/dees91/agent-skill-manager/internal/state"
)

func TestLocalInstallApplyCreatesLinksAndPersistsOwnership(t *testing.T) {
	p, source, discovered := localInstallFixture(t, "alpha")
	plan, err := PlanLocalInstall(p, state.Manifest{}, source, discovered, PlanOptions{})
	if err != nil {
		t.Fatalf("PlanLocalInstall() error = %v", err)
	}
	if len(plan.Links) != 4 {
		t.Fatalf("Links len = %d, want Claude, Codex, Muse, and Grok", len(plan.Links))
	}
	result, err := NewLocalApplyService(p).Apply(plan)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if len(result.Created) != 4 || len(result.Source.InstalledSkills) != 1 {
		t.Fatalf("result = %#v, want four links and one skill", result)
	}
	for _, tool := range model.Tools() {
		dir, _ := p.UserSkillsDirFor(tool)
		linkPath := filepath.Join(dir, "alpha")
		target, err := os.Readlink(linkPath)
		if err != nil || target != discovered[0].Path {
			t.Fatalf("%s link target=%q err=%v, want %s", tool, target, err, discovered[0].Path)
		}
	}
	manifest, err := state.New(p).Load()
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if manifest.Version != 2 || len(manifest.LocalSources) != 1 || len(manifest.Repositories) != 0 {
		t.Fatalf("manifest = %#v, want one local source", manifest)
	}
	if _, err := os.Lstat(source.CanonicalPath); err != nil {
		t.Fatalf("source changed: %v", err)
	}
}

func TestLocalInstallReapplyIsIdempotentAndAddsNewSkills(t *testing.T) {
	p, source, discovered := localInstallFixture(t, "alpha")
	plan, err := PlanLocalInstall(p, state.Manifest{}, source, discovered, PlanOptions{Tools: []model.Tool{model.ToolClaude}})
	if err != nil {
		t.Fatalf("initial plan: %v", err)
	}
	if _, err := NewLocalApplyService(p).Apply(plan); err != nil {
		t.Fatalf("initial apply: %v", err)
	}
	writeSkill(t, filepath.Join(source.CanonicalPath, "skills", "beta"))
	discovered, err = DiscoverLocalSkills(source)
	if err != nil {
		t.Fatalf("discover after add: %v", err)
	}
	manifest, err := state.New(p).Load()
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	plan, err = PlanLocalInstall(p, manifest, source, discovered, PlanOptions{Tools: []model.Tool{model.ToolClaude}})
	if err != nil {
		t.Fatalf("reapply plan: %v", err)
	}
	if len(plan.Links) != 1 || plan.Links[0].Skill.Name != "beta" || len(plan.AlreadyInstalled) != 1 {
		t.Fatalf("reapply plan = %#v, want beta link and alpha already", plan)
	}
	result, err := NewLocalApplyService(p).Apply(plan)
	if err != nil {
		t.Fatalf("reapply: %v", err)
	}
	if len(result.Source.InstalledSkills) != 2 {
		t.Fatalf("InstalledSkills = %#v, want alpha and beta", result.Source.InstalledSkills)
	}
}

func TestLocalInstallAdoptsExactUnmanagedLink(t *testing.T) {
	p, source, discovered := localInstallFixture(t, "alpha")
	if err := os.MkdirAll(p.ClaudeUserSkills, 0o755); err != nil {
		t.Fatalf("create Claude dir: %v", err)
	}
	linkPath := filepath.Join(p.ClaudeUserSkills, "alpha")
	if err := os.Symlink(discovered[0].Path, linkPath); err != nil {
		t.Fatalf("create unmanaged link: %v", err)
	}
	plan, err := PlanLocalInstall(p, state.Manifest{}, source, discovered, PlanOptions{Tools: []model.Tool{model.ToolClaude}})
	if err != nil {
		t.Fatalf("PlanLocalInstall() error = %v", err)
	}
	if len(plan.Links) != 0 || len(plan.AlreadyInstalled) != 1 {
		t.Fatalf("plan = %#v, want adoption", plan)
	}
	if _, err := NewLocalApplyService(p).Apply(plan); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	manifest, err := state.New(p).Load()
	if err != nil || len(manifest.LocalSources) != 1 {
		t.Fatalf("manifest = %#v err=%v, want adopted ownership", manifest, err)
	}
}

func TestLocalInstallReapplyAdoptsNewExactLink(t *testing.T) {
	p, source, discovered := localInstallFixture(t, "alpha")
	plan, err := PlanLocalInstall(p, state.Manifest{}, source, discovered, PlanOptions{Tools: []model.Tool{model.ToolClaude}})
	if err != nil {
		t.Fatalf("initial plan: %v", err)
	}
	if _, err := NewLocalApplyService(p).Apply(plan); err != nil {
		t.Fatalf("initial apply: %v", err)
	}
	betaPath := filepath.Join(source.CanonicalPath, "skills", "beta")
	writeSkill(t, betaPath)
	mustSymlink(t, betaPath, filepath.Join(p.ClaudeUserSkills, "beta"))
	discovered, err = DiscoverLocalSkills(source)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	manifest, _ := state.New(p).Load()
	plan, err = PlanLocalInstall(p, manifest, source, discovered, PlanOptions{Tools: []model.Tool{model.ToolClaude}})
	if err != nil {
		t.Fatalf("reapply plan: %v", err)
	}
	if len(plan.Links) != 0 || len(plan.AlreadyInstalled) != 2 {
		t.Fatalf("plan = %#v, want alpha and adopted beta", plan)
	}
	result, err := NewLocalApplyService(p).Apply(plan)
	if err != nil {
		t.Fatalf("reapply: %v", err)
	}
	if len(result.Source.InstalledSkills) != 2 {
		t.Fatalf("InstalledSkills = %#v, want alpha and beta", result.Source.InstalledSkills)
	}
}

func TestLocalInstallDirectSkillRootRoundTrip(t *testing.T) {
	home := t.TempDir()
	p := paths.ForHome(home)
	root := filepath.Join(home, "workspace", "single-skill")
	writeSkill(t, root)
	source, err := ResolveLocalSource(p, filepath.Dir(root), root)
	if err != nil {
		t.Fatalf("ResolveLocalSource() error = %v", err)
	}
	discovered, err := DiscoverLocalSkills(source)
	if err != nil {
		t.Fatalf("DiscoverLocalSkills() error = %v", err)
	}
	plan, err := PlanLocalInstall(p, state.Manifest{}, source, discovered, PlanOptions{Tools: []model.Tool{model.ToolCodex}})
	if err != nil {
		t.Fatalf("PlanLocalInstall() error = %v", err)
	}
	result, err := NewLocalApplyService(p).Apply(plan)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if len(result.Source.InstalledSkills) != 1 || result.Source.InstalledSkills[0].RelativePath != "." {
		t.Fatalf("source = %#v, want root relative path", result.Source)
	}
	if _, err := NewLocalUninstallService(p).Apply(result.Source); err != nil {
		t.Fatalf("uninstall root skill: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(root, "SKILL.md")); err != nil {
		t.Fatalf("root source changed: %v", err)
	}
}

func TestLocalInstallRejectsOwnershipAndRecordedDrift(t *testing.T) {
	t.Run("other local owner", func(t *testing.T) {
		p, source, discovered := localInstallFixture(t, "alpha")
		other := filepath.Join(p.Home, "other-source")
		manifest := state.Manifest{LocalSources: []state.LocalSourceEntry{{
			OriginalPath: other, CanonicalPath: other, Group: model.GroupLabel("other-source"),
			InstalledSkills: []state.InstalledSkillEntry{{Name: "alpha", RelativePath: "alpha", Tools: []model.Tool{model.ToolClaude}}},
		}}}
		_, err := PlanLocalInstall(p, manifest, source, discovered, PlanOptions{Tools: []model.Tool{model.ToolClaude}})
		if err == nil || !strings.Contains(err.Error(), "already owned by local source") {
			t.Fatalf("PlanLocalInstall() error = %v, want ownership conflict", err)
		}
	})

	t.Run("repository owner", func(t *testing.T) {
		p, source, discovered := localInstallFixture(t, "alpha")
		manifest := state.Manifest{Repositories: []state.RepositoryEntry{{
			Host: "github.com", RepoPath: "owner/repo",
			InstalledSkills: []state.InstalledSkillEntry{{Name: "alpha", RelativePath: "skills/alpha", Tools: []model.Tool{model.ToolCodex}}},
		}}}
		_, err := PlanLocalInstall(p, manifest, source, discovered, PlanOptions{Tools: []model.Tool{model.ToolCodex}})
		if err == nil || !strings.Contains(err.Error(), "already owned by repository") {
			t.Fatalf("PlanLocalInstall() error = %v, want repository ownership conflict", err)
		}
	})

	t.Run("existing source drift", func(t *testing.T) {
		p, source, discovered := localInstallFixture(t, "alpha")
		plan, err := PlanLocalInstall(p, state.Manifest{}, source, discovered, PlanOptions{Tools: []model.Tool{model.ToolClaude}})
		if err != nil {
			t.Fatalf("initial plan: %v", err)
		}
		if _, err := NewLocalApplyService(p).Apply(plan); err != nil {
			t.Fatalf("initial apply: %v", err)
		}
		if err := os.Remove(filepath.Join(p.ClaudeUserSkills, "alpha")); err != nil {
			t.Fatalf("remove managed link: %v", err)
		}
		manifest, _ := state.New(p).Load()
		_, err = PlanLocalInstall(p, manifest, source, discovered, PlanOptions{Tools: []model.Tool{model.ToolClaude}})
		if err == nil || !strings.Contains(err.Error(), "expected managed symlink is missing") {
			t.Fatalf("reapply error = %v, want drift", err)
		}
	})
}

func TestLocalInstallApplyRejectsOwnershipAddedAfterPlanning(t *testing.T) {
	p, source, discovered := localInstallFixture(t, "alpha")
	mustSymlink(t, discovered[0].Path, filepath.Join(p.ClaudeUserSkills, "alpha"))
	plan, err := PlanLocalInstall(p, state.Manifest{}, source, discovered, PlanOptions{Tools: []model.Tool{model.ToolClaude}})
	if err != nil || len(plan.AlreadyInstalled) != 1 {
		t.Fatalf("plan = %#v err=%v, want adoptable link", plan, err)
	}
	manifest := state.Manifest{Repositories: []state.RepositoryEntry{{
		Host: "github.com", RepoPath: "owner/repo",
		InstalledSkills: []state.InstalledSkillEntry{{Name: "alpha", RelativePath: "skills/alpha", Tools: []model.Tool{model.ToolClaude}}},
	}}}
	if err := state.New(p).Save(manifest); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	_, err = NewLocalApplyService(p).Apply(plan)
	if err == nil || !strings.Contains(err.Error(), "became owned by repository github.com/owner/repo") {
		t.Fatalf("Apply() error = %v, want ownership drift", err)
	}
	if len(loadLocalSources(t, p)) != 0 {
		t.Fatal("local ownership persisted despite repository ownership drift")
	}
}

func TestLocalInstallRollsBackCreatedLinksWhenStateSaveFails(t *testing.T) {
	p, source, discovered := localInstallFixture(t, "alpha")
	plan, err := PlanLocalInstall(p, state.Manifest{}, source, discovered, PlanOptions{})
	if err != nil {
		t.Fatalf("PlanLocalInstall() error = %v", err)
	}
	service := NewLocalApplyService(p)
	service.saveManifest = func(state.Manifest) error { return errors.New("save failed") }
	result, err := service.Apply(plan)
	if err == nil || !strings.Contains(err.Error(), "save failed") || len(result.RolledBack) != 4 {
		t.Fatalf("Apply() result=%#v error=%v, want rollback", result, err)
	}
	for _, tool := range model.Tools() {
		dir, _ := p.UserSkillsDirFor(tool)
		if _, err := os.Lstat(filepath.Join(dir, "alpha")); !os.IsNotExist(err) {
			t.Fatalf("%s link remains after rollback: %v", tool, err)
		}
	}
	if _, err := os.Lstat(source.CanonicalPath); err != nil {
		t.Fatalf("source changed after rollback: %v", err)
	}
}

func localInstallFixture(t *testing.T, names ...string) (paths.Paths, LocalSource, []DiscoveredSkill) {
	t.Helper()
	home := t.TempDir()
	p := paths.ForHome(home)
	root := filepath.Join(home, "workspace", "local-pack")
	for _, name := range names {
		writeSkill(t, filepath.Join(root, "skills", name))
	}
	source, err := ResolveLocalSource(p, filepath.Join(home, "workspace"), root)
	if err != nil {
		t.Fatalf("ResolveLocalSource() error = %v", err)
	}
	discovered, err := DiscoverLocalSkills(source)
	if err != nil {
		t.Fatalf("DiscoverLocalSkills() error = %v", err)
	}
	return p, source, discovered
}

func loadLocalSources(t *testing.T, p paths.Paths) []state.LocalSourceEntry {
	t.Helper()
	manifest, err := state.New(p).Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	return manifest.LocalSources
}
