package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dees91/agent-skill-manager/internal/model"
	"github.com/dees91/agent-skill-manager/internal/ops"
	"github.com/dees91/agent-skill-manager/internal/paths"
	"github.com/dees91/agent-skill-manager/internal/scan"
	"github.com/dees91/agent-skill-manager/internal/state"
)

func TestPlanInstallOffPlansDisabledPaths(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	skills := discoveredSkills(t, p, "alpha")
	plan, err := PlanInstall(p, state.Manifest{}, mustIdentity(t), testCheckoutPath(t, p), skills, PlanOptions{Off: true})
	if err != nil {
		t.Fatalf("PlanInstall() error = %v", err)
	}
	if !plan.Off || len(plan.Links) != 4 {
		t.Fatalf("plan = %#v, want four OFF links", plan)
	}
	for _, link := range plan.Links {
		disabledDir, _ := p.DisabledDirFor(link.Tool)
		activeDir, _ := p.UserSkillsDirFor(link.Tool)
		if link.DisabledPath != filepath.Join(disabledDir, "alpha") || link.TargetPath != filepath.Join(activeDir, "alpha") || link.LinkPath() != link.DisabledPath {
			t.Fatalf("link = %#v, want disabled link path and active target", link)
		}
	}
}

func TestPlanInstallOffRejectsOccupiedDisabledPath(t *testing.T) {
	for _, occupant := range []string{"symlink", "directory"} {
		t.Run(occupant, func(t *testing.T) {
			p := paths.ForHome(t.TempDir())
			skills := discoveredSkills(t, p, "alpha")
			blocker := filepath.Join(p.ClaudeDisabledDir, "alpha")
			mkdirAll(t, filepath.Dir(blocker))
			if occupant == "symlink" {
				if err := os.Symlink(skills.Skills[0].Path, blocker); err != nil {
					t.Fatalf("create blocker: %v", err)
				}
			} else {
				mkdirAll(t, blocker)
			}
			_, err := PlanInstall(p, state.Manifest{}, mustIdentity(t), testCheckoutPath(t, p), skills, PlanOptions{Tools: []model.Tool{model.ToolClaude}, Off: true})
			if err == nil || !strings.Contains(err.Error(), "disabled path already exists") {
				t.Fatalf("PlanInstall() error = %v, want occupied disabled path", err)
			}

			plan, err := PlanInstall(p, state.Manifest{}, mustIdentity(t), testCheckoutPath(t, p), skills, PlanOptions{Tools: []model.Tool{model.ToolClaude}})
			if err != nil || len(plan.Links) != 1 || plan.Links[0].DisabledPath != "" {
				t.Fatalf("ON PlanInstall() = %#v, %v; want the disabled path ignored", plan, err)
			}
		})
	}
}

func TestPlanInstallOffKeepsAlreadyInstalledState(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	identity, checkout := extendGitFixture(t, p, "https://github.com/owner/repo.git", "alpha", "beta")
	installClaudeOnly(t, p, identity, checkout)
	disableForTool(t, p, model.ToolClaude, "beta")
	discovered, err := DiscoverSkills(checkout)
	if err != nil {
		t.Fatalf("DiscoverSkills() error = %v", err)
	}

	plan, err := PlanInstall(p, loadInstallManifest(t, p), identity, checkout, discovered, PlanOptions{Tools: []model.Tool{model.ToolClaude}, Off: true})
	if err != nil {
		t.Fatalf("PlanInstall() error = %v", err)
	}
	if len(plan.Links) != 0 || len(plan.AlreadyInstalled) != 2 {
		t.Fatalf("plan = %#v, want both cells already installed", plan)
	}
	states := map[string]model.SkillState{}
	for _, already := range plan.AlreadyInstalled {
		states[already.Skill.Name] = already.State
	}
	if states["alpha"] != model.SkillStateOn || states["beta"] != model.SkillStateOff {
		t.Fatalf("states = %#v, want alpha ON and beta OFF", states)
	}
}

func TestApplyOffMatchesManualDisableAndPassesAudits(t *testing.T) {
	const origin = "https://git.example.test/team/skills.git"
	offHome := paths.ForHome(t.TempDir())
	offIdentity, offCheckout := extendGitFixture(t, offHome, origin, "alpha")
	gitInitCheckout(t, offCheckout, offIdentity.OriginalURL)
	discovered, err := DiscoverSkills(offCheckout)
	if err != nil {
		t.Fatalf("DiscoverSkills() error = %v", err)
	}
	plan, err := PlanInstall(offHome, state.Manifest{}, offIdentity, offCheckout, discovered, PlanOptions{Off: true})
	if err != nil {
		t.Fatalf("PlanInstall() error = %v", err)
	}
	service := NewApplyService(offHome)
	service.now = fixedApplyNow
	if _, err := service.Apply(plan, "abc123"); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	onHome := paths.ForHome(t.TempDir())
	onIdentity, onCheckout := extendGitFixture(t, onHome, origin, "alpha")
	gitInitCheckout(t, onCheckout, onIdentity.OriginalURL)
	onDiscovered, err := DiscoverSkills(onCheckout)
	if err != nil {
		t.Fatalf("DiscoverSkills() error = %v", err)
	}
	onPlan, err := PlanInstall(onHome, state.Manifest{}, onIdentity, onCheckout, onDiscovered, PlanOptions{})
	if err != nil {
		t.Fatalf("ON PlanInstall() error = %v", err)
	}
	if _, err := NewApplyService(onHome).Apply(onPlan, "abc123"); err != nil {
		t.Fatalf("ON Apply() error = %v", err)
	}
	for _, tool := range model.Tools() {
		disableForTool(t, onHome, tool, "alpha")
	}

	manifest := loadInstallManifest(t, offHome)
	manual := loadInstallManifest(t, onHome)
	skillPath := filepath.Join(offCheckout, "skills", "alpha")
	for _, tool := range model.Tools() {
		activeDir, _ := offHome.UserSkillsDirFor(tool)
		disabledDir, _ := offHome.DisabledDirFor(tool)
		if _, err := os.Lstat(filepath.Join(activeDir, "alpha")); !os.IsNotExist(err) {
			t.Fatalf("%s alpha is visible in the active directory: %v", tool, err)
		}
		assertSymlinkTarget(t, filepath.Join(disabledDir, "alpha"), skillPath)
		entry, ok := manifest.Get(tool, "alpha")
		if !ok {
			t.Fatalf("%s alpha disabled record missing", tool)
		}
		want := state.DisabledEntry{
			Tool:          tool,
			SkillName:     "alpha",
			OriginalPath:  filepath.Join(activeDir, "alpha"),
			DisabledPath:  filepath.Join(disabledDir, "alpha"),
			EntryType:     model.EntryTypeSymlink,
			SymlinkTarget: skillPath,
			DisabledAt:    fixedApplyNow(),
		}
		manualEntry, _ := manual.Get(tool, "alpha")
		want.Source, want.Group = manualEntry.Source, manualEntry.Group
		if entry != want {
			t.Fatalf("%s record = %#v, want %#v", tool, entry, want)
		}
		if entry.Source != model.SourceSymlinkRepo || entry.Group == "" {
			t.Fatalf("%s record labels = %s/%s, want a classified Git source", tool, entry.Source, entry.Group)
		}
	}
	info, err := os.Stat(offHome.ClaudeDisabledDir)
	if err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("disabled dir mode = %v, %v; want 0700", info, err)
	}

	disabled, err := scan.New(offHome).Disabled()
	if err != nil {
		t.Fatalf("scan disabled: %v", err)
	}
	if len(disabled) != len(model.Tools()) {
		t.Fatalf("disabled skills = %#v, want one OFF cell per tool", disabled)
	}
	for _, skill := range disabled {
		if skill.Name != "alpha" || skill.State != model.SkillStateOff {
			t.Fatalf("disabled skill = %#v, want alpha OFF", skill)
		}
	}
	managed, err := scan.New(offHome).Managed()
	if err != nil || len(managed) != 0 {
		t.Fatalf("managed skills = %#v, %v; want nothing active", managed, err)
	}

	repository := mustExtendRepo(t, manifest, offIdentity)
	if _, err := AuditRepositoryReferences(offHome, manifest, repository); err != nil {
		t.Fatalf("AuditRepositoryReferences() error = %v", err)
	}
	if _, err := NewUninstallService(offHome, nil).Plan(repository); err != nil {
		t.Fatalf("Uninstall Plan() error = %v", err)
	}

	toggles := ops.New(offHome)
	enable, err := toggles.PlanEnable(model.ToolClaude, "alpha")
	if err != nil {
		t.Fatalf("PlanEnable() error = %v", err)
	}
	if result := toggles.Apply([]model.PlannedOperation{enable}); result.Failed != nil {
		t.Fatalf("enable failed: %v", result.Failed.Err)
	}
	assertSymlinkTarget(t, filepath.Join(offHome.ClaudeUserSkills, "alpha"), skillPath)
}

func TestApplyOffRollsBackLinksWhenStateSaveFails(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	skills := discoveredSkills(t, p, "alpha")
	identity := mustIdentity(t)
	plan, err := PlanInstall(p, state.Manifest{}, identity, testCheckoutPath(t, p), skills, PlanOptions{Tools: []model.Tool{model.ToolClaude, model.ToolCodex}, Off: true})
	if err != nil {
		t.Fatalf("PlanInstall() error = %v", err)
	}
	service := NewApplyService(p)
	service.saveManifest = func(state.Manifest) error { return os.ErrPermission }

	result, err := service.Apply(plan, "abc123")
	if err == nil || !strings.Contains(err.Error(), "save install state") {
		t.Fatalf("Apply() error = %v, want save failure", err)
	}
	if len(result.RolledBack) != 2 {
		t.Fatalf("RolledBack = %#v, want both links", result.RolledBack)
	}
	for _, link := range plan.Links {
		if _, err := os.Lstat(link.DisabledPath); !os.IsNotExist(err) {
			t.Fatalf("%s left behind: %v", link.DisabledPath, err)
		}
	}
	if _, ok := loadInstallManifest(t, p).GetRepository(identity.Host, identity.RepoPath); ok {
		t.Fatal("repository state written despite save failure")
	}
}

func TestApplyRollsBackActiveLinksWhenStateSaveFails(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	skills := discoveredSkills(t, p, "alpha")
	plan, err := PlanInstall(p, state.Manifest{}, mustIdentity(t), testCheckoutPath(t, p), skills, PlanOptions{Tools: []model.Tool{model.ToolClaude}})
	if err != nil {
		t.Fatalf("PlanInstall() error = %v", err)
	}
	service := NewApplyService(p)
	service.saveManifest = func(state.Manifest) error { return os.ErrPermission }

	if _, err := service.Apply(plan, "abc123"); err == nil {
		t.Fatal("Apply() error = nil, want save failure")
	}
	if _, err := os.Lstat(filepath.Join(p.ClaudeUserSkills, "alpha")); !os.IsNotExist(err) {
		t.Fatalf("active link left behind: %v", err)
	}
}

func TestApplyOffRejectsPathsTakenAfterPlanning(t *testing.T) {
	for _, taken := range []string{"active", "disabled"} {
		t.Run(taken, func(t *testing.T) {
			p := paths.ForHome(t.TempDir())
			skills := discoveredSkills(t, p, "alpha")
			plan, err := PlanInstall(p, state.Manifest{}, mustIdentity(t), testCheckoutPath(t, p), skills, PlanOptions{Tools: []model.Tool{model.ToolClaude}, Off: true})
			if err != nil {
				t.Fatalf("PlanInstall() error = %v", err)
			}
			blocker := plan.Links[0].TargetPath
			if taken == "disabled" {
				blocker = plan.Links[0].DisabledPath
			}
			mkdirAll(t, blocker)

			if _, err := NewApplyService(p).Apply(plan, "abc123"); err == nil || !strings.Contains(err.Error(), "already exists") {
				t.Fatalf("Apply() error = %v, want %s path rejection", err, taken)
			}
			if _, ok := loadInstallManifest(t, p).Get(model.ToolClaude, "alpha"); ok {
				t.Fatal("disabled record written despite rejection")
			}
		})
	}
}

func TestLocalApplyOffRecordsLocalSourceAndReportsAlreadyOff(t *testing.T) {
	p, source, discovered := localInstallFixture(t, "solo")
	plan, err := PlanLocalInstall(p, state.Manifest{}, source, discovered, PlanOptions{Tools: []model.Tool{model.ToolClaude, model.ToolGrok}, Off: true})
	if err != nil {
		t.Fatalf("PlanLocalInstall() error = %v", err)
	}
	if _, err := NewLocalApplyService(p).Apply(plan); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	manifest := loadInstallManifest(t, p)
	for _, tool := range []model.Tool{model.ToolClaude, model.ToolGrok} {
		entry, ok := manifest.Get(tool, "solo")
		if !ok || entry.Source != model.SourceLocalPath || entry.Group != source.Group {
			t.Fatalf("%s record = %#v, want local path source in group %s", tool, entry, source.Group)
		}
		activeDir, _ := p.UserSkillsDirFor(tool)
		if _, err := os.Lstat(filepath.Join(activeDir, "solo")); !os.IsNotExist(err) {
			t.Fatalf("%s solo is visible in the active directory: %v", tool, err)
		}
	}
	local, ok := manifest.GetLocalSource(source.CanonicalPath)
	if !ok {
		t.Fatal("local source entry missing")
	}
	if _, err := AuditLocalSourceReferences(p, manifest, local, true); err != nil {
		t.Fatalf("AuditLocalSourceReferences() error = %v", err)
	}

	again, err := PlanLocalInstall(p, manifest, source, discovered, PlanOptions{Tools: []model.Tool{model.ToolClaude, model.ToolGrok}, Off: true})
	if err != nil {
		t.Fatalf("second PlanLocalInstall() error = %v", err)
	}
	if len(again.Links) != 0 || len(again.AlreadyInstalled) != 2 || again.AlreadyInstalled[0].State != model.SkillStateOff {
		t.Fatalf("second plan = %#v, want already OFF cells", again)
	}
}

func TestLocalApplyOffRollsBackWhenStateSaveFails(t *testing.T) {
	p, source, discovered := localInstallFixture(t, "solo")
	plan, err := PlanLocalInstall(p, state.Manifest{}, source, discovered, PlanOptions{Tools: []model.Tool{model.ToolCodex}, Off: true})
	if err != nil {
		t.Fatalf("PlanLocalInstall() error = %v", err)
	}
	service := NewLocalApplyService(p)
	service.saveManifest = func(state.Manifest) error { return os.ErrPermission }
	if _, err := service.Apply(plan); err == nil {
		t.Fatal("Apply() error = nil, want save failure")
	}
	if _, err := os.Lstat(filepath.Join(p.CodexDisabledDir, "solo")); !os.IsNotExist(err) {
		t.Fatalf("disabled link left behind: %v", err)
	}
}

func TestApplyOffRecordsSkillsCLISourceForLockedCodexSkill(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	skills := discoveredSkills(t, p, "alpha")
	mkdirAll(t, filepath.Dir(p.AgentsSkillLock))
	if err := os.WriteFile(p.AgentsSkillLock, []byte(`{"skills":{"alpha":{}}}`), 0o644); err != nil {
		t.Fatalf("write skills lock: %v", err)
	}
	plan, err := PlanInstall(p, state.Manifest{}, mustIdentity(t), testCheckoutPath(t, p), skills, PlanOptions{Tools: []model.Tool{model.ToolCodex}, Off: true})
	if err != nil {
		t.Fatalf("PlanInstall() error = %v", err)
	}
	if _, err := NewApplyService(p).Apply(plan, "abc123"); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	entry, ok := loadInstallManifest(t, p).Get(model.ToolCodex, "alpha")
	if !ok || entry.Source != model.SourceSkillsCLI || entry.Group != model.GroupSkillsCLI {
		t.Fatalf("record = %#v, want skills CLI source", entry)
	}
}
