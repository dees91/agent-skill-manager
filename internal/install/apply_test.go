package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dees91/agent-skill-manager/internal/model"
	"github.com/dees91/agent-skill-manager/internal/paths"
	"github.com/dees91/agent-skill-manager/internal/scan"
	"github.com/dees91/agent-skill-manager/internal/state"
)

func TestApplyCreatesSymlinksUpdatesManifestAndRescans(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	skills := discoveredSkills(t, p, "alpha")
	identity := mustIdentity(t)
	plan, err := PlanInstall(p, state.Manifest{}, identity, testCheckoutPath(t, p), skills, PlanOptions{})
	if err != nil {
		t.Fatalf("PlanInstall() error = %v", err)
	}
	service := NewApplyService(p)
	service.now = fixedApplyNow

	result, err := service.Apply(plan, "abc123")
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if len(result.Created) != 4 {
		t.Fatalf("Created len = %d, want 4", len(result.Created))
	}
	assertSymlinkTarget(t, filepath.Join(p.ClaudeUserSkills, "alpha"), skills[0].Path)
	assertSymlinkTarget(t, filepath.Join(p.CodexUserSkills, "alpha"), skills[0].Path)
	assertSymlinkTarget(t, filepath.Join(p.MuseUserSkills, "alpha"), skills[0].Path)
	assertSymlinkTarget(t, filepath.Join(p.GrokUserSkills, "alpha"), skills[0].Path)

	managed, err := scan.New(p).Managed()
	if err != nil {
		t.Fatalf("scan managed: %v", err)
	}
	if len(managed) != 4 {
		t.Fatalf("managed skills len = %d, want 4: %#v", len(managed), managed)
	}

	manifest := loadInstallManifest(t, p)
	repo, ok := manifest.GetRepository(identity.Host, identity.RepoPath)
	if !ok {
		t.Fatal("repository manifest entry missing")
	}
	if repo.OriginalURL != identity.OriginalURL ||
		repo.CanonicalURL != identity.CanonicalURL ||
		repo.CheckoutPath != plan.CheckoutPath ||
		repo.Group != identity.Group ||
		repo.LastSeenCommit != "abc123" ||
		!repo.InstalledAt.Equal(fixedApplyNow().UTC()) {
		t.Fatalf("repository entry = %#v, want install metadata", repo)
	}
	if len(repo.InstalledSkills) != 1 {
		t.Fatalf("InstalledSkills len = %d, want 1", len(repo.InstalledSkills))
	}
	installed := repo.InstalledSkills[0]
	if installed.Name != "alpha" || installed.RelativePath != "skills/alpha" {
		t.Fatalf("installed skill = %#v, want alpha relative path", installed)
	}
	wantTools := []model.Tool{model.ToolClaude, model.ToolCodex, model.ToolMuse, model.ToolGrok}
	if !sameToolSlice(installed.Tools, wantTools) {
		t.Fatalf("installed tools = %#v, want %#v", installed.Tools, wantTools)
	}
}

func TestApplyLeavesAlreadyInstalledEntriesUntouchedAndUpdatesManifest(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	skills := discoveredSkills(t, p, "alpha", "beta")
	mkdirAll(t, p.ClaudeUserSkills)
	activeAlpha := filepath.Join(p.ClaudeUserSkills, "alpha")
	if err := os.Symlink(skills[0].Path, activeAlpha); err != nil {
		t.Fatalf("create existing symlink: %v", err)
	}
	disabledBeta := filepath.Join(p.CodexDisabledDir, "beta")
	mkdirAll(t, filepath.Dir(disabledBeta))
	if err := os.Symlink(skills[1].Path, disabledBeta); err != nil {
		t.Fatalf("create disabled symlink: %v", err)
	}
	manifest := state.Manifest{Disabled: []state.DisabledEntry{{
		Tool:          model.ToolCodex,
		SkillName:     "beta",
		OriginalPath:  filepath.Join(p.CodexUserSkills, "beta"),
		DisabledPath:  disabledBeta,
		EntryType:     model.EntryTypeSymlink,
		SymlinkTarget: skills[1].Path,
	}}}
	if err := state.New(p).Save(manifest); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	plan, err := PlanInstall(p, manifest, mustIdentity(t), testCheckoutPath(t, p), skills, PlanOptions{
		Tools:      []model.Tool{model.ToolClaude, model.ToolCodex},
		SkillNames: []string{"alpha", "beta"},
	})
	if err != nil {
		t.Fatalf("PlanInstall() error = %v", err)
	}
	service := NewApplyService(p)
	result, err := service.Apply(plan, "def456")
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	assertSymlinkTarget(t, activeAlpha, skills[0].Path)
	assertSymlinkTarget(t, disabledBeta, skills[1].Path)
	if _, err := os.Lstat(filepath.Join(p.CodexUserSkills, "beta")); !os.IsNotExist(err) {
		t.Fatalf("disabled beta was enabled or unexpected lstat error: %v", err)
	}
	if len(result.AlreadyInstalled) != 2 {
		t.Fatalf("AlreadyInstalled len = %d, want 2", len(result.AlreadyInstalled))
	}

	repo, ok := loadInstallManifest(t, p).GetRepository(mustIdentity(t).Host, mustIdentity(t).RepoPath)
	if !ok {
		t.Fatal("repository manifest entry missing")
	}
	if len(repo.InstalledSkills) != 2 {
		t.Fatalf("InstalledSkills = %#v, want alpha and beta", repo.InstalledSkills)
	}
}

func TestApplyAllAlreadyInstalledPlanUpdatesManifest(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	skills := discoveredSkills(t, p, "alpha")
	mkdirAll(t, p.ClaudeUserSkills)
	activeAlpha := filepath.Join(p.ClaudeUserSkills, "alpha")
	if err := os.Symlink(skills[0].Path, activeAlpha); err != nil {
		t.Fatalf("create existing symlink: %v", err)
	}
	plan, err := PlanInstall(p, state.Manifest{}, mustIdentity(t), testCheckoutPath(t, p), skills, PlanOptions{Tools: []model.Tool{model.ToolClaude}})
	if err != nil {
		t.Fatalf("PlanInstall() error = %v", err)
	}
	if len(plan.Links) != 0 || len(plan.AlreadyInstalled) != 1 {
		t.Fatalf("plan = %#v, want all already installed", plan)
	}

	result, err := NewApplyService(p).Apply(plan, "abc123")
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if len(result.Created) != 0 {
		t.Fatalf("Created = %#v, want none", result.Created)
	}
	repo, ok := loadInstallManifest(t, p).GetRepository(mustIdentity(t).Host, mustIdentity(t).RepoPath)
	if !ok {
		t.Fatal("repository manifest entry missing")
	}
	if len(repo.InstalledSkills) != 1 || !sameToolSlice(repo.InstalledSkills[0].Tools, []model.Tool{model.ToolClaude}) {
		t.Fatalf("InstalledSkills = %#v, want alpha claude", repo.InstalledSkills)
	}
}

func TestApplyMergesRepositoryManifestAcrossInstalls(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	identity := mustIdentity(t)
	checkout := testCheckoutPath(t, p)
	installedAt := time.Date(2026, 5, 8, 9, 0, 0, 0, time.UTC)
	if err := state.New(p).Save(state.Manifest{Repositories: []state.RepositoryEntry{{
		OriginalURL:    identity.OriginalURL,
		CanonicalURL:   identity.CanonicalURL,
		Host:           identity.Host,
		RepoPath:       identity.RepoPath,
		CheckoutPath:   checkout,
		Group:          identity.Group,
		InstalledAt:    installedAt,
		LastSeenCommit: "old",
		InstalledSkills: []state.InstalledSkillEntry{{
			Name:         "alpha",
			RelativePath: "skills/alpha",
			Tools:        []model.Tool{model.ToolClaude},
		}},
	}}}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	skills := discoveredSkills(t, p, "beta")
	plan, err := PlanInstall(p, loadInstallManifest(t, p), identity, checkout, skills, PlanOptions{Tools: []model.Tool{model.ToolCodex}})
	if err != nil {
		t.Fatalf("PlanInstall() error = %v", err)
	}
	service := NewApplyService(p)
	service.now = func() time.Time { return installedAt.Add(2 * time.Hour) }

	if _, err := service.Apply(plan, "new"); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	repo, ok := loadInstallManifest(t, p).GetRepository(identity.Host, identity.RepoPath)
	if !ok {
		t.Fatal("repository manifest entry missing")
	}
	if !repo.InstalledAt.Equal(installedAt) {
		t.Fatalf("InstalledAt = %s, want preserved %s", repo.InstalledAt, installedAt)
	}
	if repo.LastSeenCommit != "new" {
		t.Fatalf("LastSeenCommit = %q, want new", repo.LastSeenCommit)
	}
	if len(repo.InstalledSkills) != 2 {
		t.Fatalf("InstalledSkills = %#v, want preserved alpha plus beta", repo.InstalledSkills)
	}
}

func TestApplyLoadsStateBeforeCreatingSymlinks(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	skills := discoveredSkills(t, p, "alpha")
	plan, err := PlanInstall(p, state.Manifest{}, mustIdentity(t), testCheckoutPath(t, p), skills, PlanOptions{Tools: []model.Tool{model.ToolClaude}})
	if err != nil {
		t.Fatalf("PlanInstall() error = %v", err)
	}
	mkdirAll(t, p.StateDir)
	if err := os.WriteFile(p.StateFile, []byte("{bad json"), 0o644); err != nil {
		t.Fatalf("write bad state: %v", err)
	}

	_, err = NewApplyService(p).Apply(plan, "abc123")
	if err == nil {
		t.Fatal("Apply() error = nil, want state load failure")
	}
	if _, statErr := os.Lstat(filepath.Join(p.ClaudeUserSkills, "alpha")); !os.IsNotExist(statErr) {
		t.Fatalf("symlink was created despite state load failure, lstat err = %v", statErr)
	}
}

func TestApplyRevalidatesAlreadyInstalledBeforeCreatingLinks(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	skills := discoveredSkills(t, p, "alpha", "beta")
	mkdirAll(t, p.ClaudeUserSkills)
	activeAlpha := filepath.Join(p.ClaudeUserSkills, "alpha")
	if err := os.Symlink(skills[0].Path, activeAlpha); err != nil {
		t.Fatalf("create existing symlink: %v", err)
	}
	manifest := state.Manifest{}
	plan, err := PlanInstall(p, manifest, mustIdentity(t), testCheckoutPath(t, p), skills, PlanOptions{Tools: []model.Tool{model.ToolClaude}})
	if err != nil {
		t.Fatalf("PlanInstall() error = %v", err)
	}
	if err := os.Remove(activeAlpha); err != nil {
		t.Fatalf("remove active alpha: %v", err)
	}
	if err := os.Symlink(skills[1].Path, activeAlpha); err != nil {
		t.Fatalf("create stale active alpha: %v", err)
	}

	_, err = NewApplyService(p).Apply(plan, "abc123")
	if err == nil {
		t.Fatal("Apply() error = nil, want stale already-installed error")
	}
	if !strings.Contains(err.Error(), "active symlink target changed") {
		t.Fatalf("Apply() error = %v, want stale already-installed error", err)
	}
	if _, statErr := os.Lstat(filepath.Join(p.ClaudeUserSkills, "beta")); !os.IsNotExist(statErr) {
		t.Fatalf("beta symlink was created despite stale already-installed entry, lstat err = %v", statErr)
	}
}

func TestApplyRevalidatesDisabledStateBeforeCreatingLinks(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	skills := discoveredSkills(t, p, "alpha", "beta")
	plan, err := PlanInstall(p, state.Manifest{}, mustIdentity(t), testCheckoutPath(t, p), skills, PlanOptions{Tools: []model.Tool{model.ToolClaude}})
	if err != nil {
		t.Fatalf("PlanInstall() error = %v", err)
	}
	manifest := state.Manifest{Disabled: []state.DisabledEntry{{
		Tool:          model.ToolClaude,
		SkillName:     "alpha",
		OriginalPath:  filepath.Join(p.ClaudeUserSkills, "alpha"),
		DisabledPath:  filepath.Join(p.ClaudeDisabledDir, "alpha"),
		EntryType:     model.EntryTypeSymlink,
		SymlinkTarget: skills[0].Path,
	}}}
	if err := state.New(p).Save(manifest); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	_, err = NewApplyService(p).Apply(plan, "abc123")
	if err == nil {
		t.Fatal("Apply() error = nil, want stale disabled-state error")
	}
	if !strings.Contains(err.Error(), "became disabled") {
		t.Fatalf("Apply() error = %v, want became disabled", err)
	}
	if _, statErr := os.Lstat(filepath.Join(p.ClaudeUserSkills, "beta")); !os.IsNotExist(statErr) {
		t.Fatalf("beta symlink was created despite stale disabled state, lstat err = %v", statErr)
	}
}

func TestApplyRevalidatesDisabledAlreadyInstalledActivePathAbsent(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	skills := discoveredSkills(t, p, "alpha")
	disabledAlpha := filepath.Join(p.ClaudeDisabledDir, "alpha")
	mkdirAll(t, filepath.Dir(disabledAlpha))
	if err := os.Symlink(skills[0].Path, disabledAlpha); err != nil {
		t.Fatalf("create disabled symlink: %v", err)
	}
	manifest := state.Manifest{Disabled: []state.DisabledEntry{{
		Tool:          model.ToolClaude,
		SkillName:     "alpha",
		OriginalPath:  filepath.Join(p.ClaudeUserSkills, "alpha"),
		DisabledPath:  disabledAlpha,
		EntryType:     model.EntryTypeSymlink,
		SymlinkTarget: skills[0].Path,
	}}}
	plan, err := PlanInstall(p, manifest, mustIdentity(t), testCheckoutPath(t, p), skills, PlanOptions{Tools: []model.Tool{model.ToolClaude}})
	if err != nil {
		t.Fatalf("PlanInstall() error = %v", err)
	}
	mkdirAll(t, p.ClaudeUserSkills)
	if err := os.Symlink(skills[0].Path, filepath.Join(p.ClaudeUserSkills, "alpha")); err != nil {
		t.Fatalf("create stale active symlink: %v", err)
	}
	if err := state.New(p).Save(manifest); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	_, err = NewApplyService(p).Apply(plan, "abc123")
	if err == nil {
		t.Fatal("Apply() error = nil, want disabled active target conflict")
	}
	if !strings.Contains(err.Error(), "disabled but active target exists") {
		t.Fatalf("Apply() error = %v, want disabled active target conflict", err)
	}
}

func TestApplyRejectsSymlinkedSkillDirectory(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	checkout := testCheckoutPath(t, p)
	outside := filepath.Join(t.TempDir(), "outside-skill")
	mkdirAll(t, outside)
	if err := os.WriteFile(filepath.Join(outside, "SKILL.md"), []byte("# Outside\n"), 0o644); err != nil {
		t.Fatalf("write outside skill: %v", err)
	}
	symlinkSkill := filepath.Join(checkout, "skills", "linked")
	mkdirAll(t, filepath.Dir(symlinkSkill))
	if err := os.Symlink(outside, symlinkSkill); err != nil {
		t.Fatalf("create symlinked skill dir: %v", err)
	}
	plan := InstallPlan{
		Identity:     mustIdentity(t),
		CheckoutPath: checkout,
		Links: []LinkPlan{{
			Skill:      DiscoveredSkill{Name: "linked", Path: symlinkSkill},
			Tool:       model.ToolClaude,
			TargetPath: filepath.Join(p.ClaudeUserSkills, "linked"),
		}},
	}

	_, err := NewApplyService(p).Apply(plan, "abc123")
	if err == nil {
		t.Fatal("Apply() error = nil, want symlinked skill rejection")
	}
	if !strings.Contains(err.Error(), "is a symlink") {
		t.Fatalf("Apply() error = %v, want symlinked skill rejection", err)
	}
}

func TestApplyRollsBackOnlyCreatedSymlinksOnFailure(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	skills := discoveredSkills(t, p, "alpha", "beta")
	mkdirAll(t, p.ClaudeUserSkills)
	preExisting := filepath.Join(p.ClaudeUserSkills, "pre-existing")
	if err := os.Symlink(skills[0].Path, preExisting); err != nil {
		t.Fatalf("create pre-existing symlink: %v", err)
	}
	plan := InstallPlan{
		Identity:     mustIdentity(t),
		CheckoutPath: testCheckoutPath(t, p),
		Group:        mustIdentity(t).Group,
		Links: []LinkPlan{
			{Skill: skills[0], Tool: model.ToolClaude, TargetPath: filepath.Join(p.ClaudeUserSkills, "alpha")},
			{Skill: skills[1], Tool: model.ToolCodex, TargetPath: filepath.Join(p.CodexUserSkills, "beta")},
		},
	}
	service := NewApplyService(p)
	symlinkCalls := 0
	service.symlink = func(oldname, newname string) error {
		symlinkCalls++
		if symlinkCalls == 2 {
			return os.ErrPermission
		}
		return os.Symlink(oldname, newname)
	}

	result, err := service.Apply(plan, "abc123")
	if err == nil {
		t.Fatal("Apply() error = nil, want symlink failure")
	}
	if len(result.RolledBack) != 1 {
		t.Fatalf("RolledBack = %#v, want alpha rollback", result.RolledBack)
	}
	if _, statErr := os.Lstat(filepath.Join(p.ClaudeUserSkills, "alpha")); !os.IsNotExist(statErr) {
		t.Fatalf("alpha symlink was not rolled back, lstat err = %v", statErr)
	}
	assertSymlinkTarget(t, preExisting, skills[0].Path)
	assertExists(t, plan.CheckoutPath)
	if _, ok := loadInstallManifest(t, p).GetRepository(mustIdentity(t).Host, mustIdentity(t).RepoPath); ok {
		t.Fatal("repository state updated despite symlink failure")
	}
}

func TestApplyBacksUpExistingStateOncePerService(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	if err := state.New(p).Save(state.Manifest{}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	skills := discoveredSkills(t, p, "alpha", "beta")
	service := NewApplyService(p)
	service.now = fixedApplyNow

	planAlpha, err := PlanInstall(p, loadInstallManifest(t, p), mustIdentity(t), testCheckoutPath(t, p), []DiscoveredSkill{skills[0]}, PlanOptions{Tools: []model.Tool{model.ToolClaude}})
	if err != nil {
		t.Fatalf("PlanInstall(alpha) error = %v", err)
	}
	if _, err := service.Apply(planAlpha, "a"); err != nil {
		t.Fatalf("Apply(alpha) error = %v", err)
	}
	planBeta, err := PlanInstall(p, loadInstallManifest(t, p), mustIdentity(t), testCheckoutPath(t, p), []DiscoveredSkill{skills[1]}, PlanOptions{Tools: []model.Tool{model.ToolClaude}})
	if err != nil {
		t.Fatalf("PlanInstall(beta) error = %v", err)
	}
	if _, err := service.Apply(planBeta, "b"); err != nil {
		t.Fatalf("Apply(beta) error = %v", err)
	}

	entries, err := os.ReadDir(p.BackupDir)
	if err != nil {
		t.Fatalf("read backups: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("backup count = %d, want 1", len(entries))
	}
}

func TestApplyRejectsLocalOwnershipAddedAfterPlanning(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	skills := discoveredSkills(t, p, "alpha")
	plan, err := PlanInstall(p, state.Manifest{}, mustIdentity(t), testCheckoutPath(t, p), skills, PlanOptions{Tools: []model.Tool{model.ToolClaude}})
	if err != nil {
		t.Fatalf("PlanInstall() error = %v", err)
	}
	localPath, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	manifest := state.Manifest{LocalSources: []state.LocalSourceEntry{{
		OriginalPath:  localPath,
		CanonicalPath: localPath,
		Group:         model.GroupLabel("local-skills"),
		InstalledSkills: []state.InstalledSkillEntry{{
			Name:         "alpha",
			RelativePath: "alpha",
			Tools:        []model.Tool{model.ToolClaude},
		}},
	}}}
	if err := state.New(p).Save(manifest); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	_, err = NewApplyService(p).Apply(plan, "abc123")
	if err == nil || !strings.Contains(err.Error(), "became owned by local source "+localPath) {
		t.Fatalf("Apply() error = %v, want local ownership drift", err)
	}
	if _, statErr := os.Lstat(filepath.Join(p.ClaudeUserSkills, "alpha")); !os.IsNotExist(statErr) {
		t.Fatalf("target mutated despite ownership drift: %v", statErr)
	}
}

func TestApplyRejectsOtherRepositoryOwnershipAddedAfterPlanning(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	skills := discoveredSkills(t, p, "alpha")
	plan, err := PlanInstall(p, state.Manifest{}, mustIdentity(t), testCheckoutPath(t, p), skills, PlanOptions{Tools: []model.Tool{model.ToolCodex}})
	if err != nil {
		t.Fatalf("PlanInstall() error = %v", err)
	}
	manifest := state.Manifest{Repositories: []state.RepositoryEntry{{
		Host: "github.com", RepoPath: "other/repo",
		InstalledSkills: []state.InstalledSkillEntry{{
			Name: "alpha", RelativePath: "skills/alpha", Tools: []model.Tool{model.ToolCodex},
		}},
	}}}
	if err := state.New(p).Save(manifest); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	_, err = NewApplyService(p).Apply(plan, "abc123")
	if err == nil || !strings.Contains(err.Error(), "became owned by repository github.com/other/repo") {
		t.Fatalf("Apply() error = %v, want repository ownership drift", err)
	}
	if _, statErr := os.Lstat(filepath.Join(p.CodexUserSkills, "alpha")); !os.IsNotExist(statErr) {
		t.Fatalf("target mutated despite ownership drift: %v", statErr)
	}
}

func fixedApplyNow() time.Time {
	return time.Date(2026, 5, 8, 16, 0, 0, 0, time.UTC)
}

func loadInstallManifest(t *testing.T, p paths.Paths) state.Manifest {
	t.Helper()
	manifest, err := state.New(p).Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	return manifest
}

func assertSymlinkTarget(t *testing.T, path, want string) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("lstat %s: %v", path, err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("%s mode = %v, want symlink", path, info.Mode())
	}
	got, err := os.Readlink(path)
	if err != nil {
		t.Fatalf("readlink %s: %v", path, err)
	}
	if got != want {
		t.Fatalf("readlink %s = %q, want %q", path, got, want)
	}
}

func assertExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
}
