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

func TestParseToolTarget(t *testing.T) {
	tests := []struct {
		value string
		want  []model.Tool
		ok    bool
	}{
		{value: "", want: model.Tools(), ok: true},
		{value: "both", want: model.Tools(), ok: true},
		{value: "all", want: model.Tools(), ok: true},
		{value: "claude", want: []model.Tool{model.ToolClaude}, ok: true},
		{value: "codex", want: []model.Tool{model.ToolCodex}, ok: true},
		{value: "muse", want: []model.Tool{model.ToolMuse}, ok: true},
		{value: "grok", want: []model.Tool{model.ToolGrok}, ok: true},
		{value: "bad", ok: false},
	}

	for _, test := range tests {
		t.Run(test.value, func(t *testing.T) {
			got, err := ParseToolTarget(test.value)
			if test.ok && err != nil {
				t.Fatalf("ParseToolTarget() error = %v", err)
			}
			if !test.ok {
				if err == nil {
					t.Fatal("ParseToolTarget() error = nil, want error")
				}
				return
			}
			if !sameToolSlice(got, test.want) {
				t.Fatalf("ParseToolTarget() = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestPlanInstallDefaultsToBothToolsAndAllSkills(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	skills := discoveredSkills(t, p, "alpha", "beta")

	plan, err := PlanInstall(p, state.Manifest{}, mustIdentity(t), testCheckoutPath(t, p), skills, PlanOptions{})
	if err != nil {
		t.Fatalf("PlanInstall() error = %v", err)
	}

	want := []LinkPlan{
		{Skill: skills[0], Tool: model.ToolClaude, TargetPath: filepath.Join(p.ClaudeUserSkills, "alpha")},
		{Skill: skills[0], Tool: model.ToolCodex, TargetPath: filepath.Join(p.CodexUserSkills, "alpha")},
		{Skill: skills[0], Tool: model.ToolMuse, TargetPath: filepath.Join(p.MuseUserSkills, "alpha")},
		{Skill: skills[0], Tool: model.ToolGrok, TargetPath: filepath.Join(p.GrokUserSkills, "alpha")},
		{Skill: skills[1], Tool: model.ToolClaude, TargetPath: filepath.Join(p.ClaudeUserSkills, "beta")},
		{Skill: skills[1], Tool: model.ToolCodex, TargetPath: filepath.Join(p.CodexUserSkills, "beta")},
		{Skill: skills[1], Tool: model.ToolMuse, TargetPath: filepath.Join(p.MuseUserSkills, "beta")},
		{Skill: skills[1], Tool: model.ToolGrok, TargetPath: filepath.Join(p.GrokUserSkills, "beta")},
	}
	assertLinkPlans(t, plan.Links, want)
	if len(plan.AlreadyInstalled) != 0 {
		t.Fatalf("AlreadyInstalled = %#v, want empty", plan.AlreadyInstalled)
	}
	if plan.Group != model.GroupLabel("addyosmani/agent-skills") {
		t.Fatalf("Group = %q, want addyosmani/agent-skills", plan.Group)
	}
}

func TestPlanInstallSupportsSingleToolAndSelectedSkills(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	skills := discoveredSkills(t, p, "alpha", "beta", "gamma")

	plan, err := PlanInstall(p, state.Manifest{}, mustIdentity(t), testCheckoutPath(t, p), skills, PlanOptions{
		Tools:      []model.Tool{model.ToolCodex, model.ToolCodex},
		SkillNames: []string{"gamma", "alpha", "gamma"},
	})
	if err != nil {
		t.Fatalf("PlanInstall() error = %v", err)
	}

	want := []LinkPlan{
		{Skill: skills[0], Tool: model.ToolCodex, TargetPath: filepath.Join(p.CodexUserSkills, "alpha")},
		{Skill: skills[2], Tool: model.ToolCodex, TargetPath: filepath.Join(p.CodexUserSkills, "gamma")},
	}
	assertLinkPlans(t, plan.Links, want)
}

func TestPlanInstallSupportsExactSkillToolCells(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	skills := discoveredSkills(t, p, "alpha", "beta")

	plan, err := PlanInstall(p, state.Manifest{}, mustIdentity(t), testCheckoutPath(t, p), skills, PlanOptions{Cells: []InstallCell{
		{SkillName: "beta", Tool: model.ToolCodex},
		{SkillName: "alpha", Tool: model.ToolClaude},
		{SkillName: "alpha", Tool: model.ToolClaude},
	}})
	if err != nil {
		t.Fatalf("PlanInstall() error = %v", err)
	}
	want := []LinkPlan{
		{Skill: skills[0], Tool: model.ToolClaude, TargetPath: filepath.Join(p.ClaudeUserSkills, "alpha")},
		{Skill: skills[1], Tool: model.ToolCodex, TargetPath: filepath.Join(p.CodexUserSkills, "beta")},
	}
	assertLinkPlans(t, plan.Links, want)
}

func TestPlanInstallRejectsMixedExactAndCartesianSelection(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	skills := discoveredSkills(t, p, "alpha")
	_, err := PlanInstall(p, state.Manifest{}, mustIdentity(t), testCheckoutPath(t, p), skills, PlanOptions{
		Tools: []model.Tool{model.ToolClaude}, Cells: []InstallCell{{SkillName: "alpha", Tool: model.ToolCodex}},
	})
	if err == nil || !strings.Contains(err.Error(), "cannot be combined") {
		t.Fatalf("PlanInstall() error = %v, want mixed-selection rejection", err)
	}
}

func TestPlanInstallReportsMissingSelectedSkills(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	skills := discoveredSkills(t, p, "alpha")

	_, err := PlanInstall(p, state.Manifest{}, mustIdentity(t), testCheckoutPath(t, p), skills, PlanOptions{
		SkillNames: []string{"missing", "alpha", "missing"},
	})
	var planErr PlanError
	if !errors.As(err, &planErr) {
		t.Fatalf("PlanInstall() error = %T %v, want PlanError", err, err)
	}
	if len(planErr.MissingSkills) != 1 || planErr.MissingSkills[0] != "missing" {
		t.Fatalf("MissingSkills = %#v, want [missing]", planErr.MissingSkills)
	}
}

func TestPlanInstallTreatsSameActiveSymlinkAsAlreadyInstalled(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	skills := discoveredSkills(t, p, "alpha")
	mkdirAll(t, p.ClaudeUserSkills)
	if err := os.Symlink(skills[0].Path, filepath.Join(p.ClaudeUserSkills, "alpha")); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	plan, err := PlanInstall(p, state.Manifest{}, mustIdentity(t), testCheckoutPath(t, p), skills, PlanOptions{Tools: []model.Tool{model.ToolClaude}})
	if err != nil {
		t.Fatalf("PlanInstall() error = %v", err)
	}
	if len(plan.Links) != 0 {
		t.Fatalf("Links = %#v, want empty", plan.Links)
	}
	if len(plan.AlreadyInstalled) != 1 {
		t.Fatalf("AlreadyInstalled = %#v, want one entry", plan.AlreadyInstalled)
	}
	already := plan.AlreadyInstalled[0]
	if already.State != model.SkillStateOn || already.Tool != model.ToolClaude || already.TargetPath != filepath.Join(p.ClaudeUserSkills, "alpha") {
		t.Fatalf("AlreadyInstalled = %#v, want ON claude alpha", already)
	}
}

func TestPlanInstallMatchesRelativeActiveSymlinkTarget(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	skills := discoveredSkills(t, p, "alpha")
	mkdirAll(t, p.ClaudeUserSkills)
	relativeTarget, err := filepath.Rel(p.ClaudeUserSkills, skills[0].Path)
	if err != nil {
		t.Fatalf("relative target: %v", err)
	}
	if err := os.Symlink(relativeTarget, filepath.Join(p.ClaudeUserSkills, "alpha")); err != nil {
		t.Fatalf("create relative symlink: %v", err)
	}

	plan, err := PlanInstall(p, state.Manifest{}, mustIdentity(t), testCheckoutPath(t, p), skills, PlanOptions{Tools: []model.Tool{model.ToolClaude}})
	if err != nil {
		t.Fatalf("PlanInstall() error = %v", err)
	}
	if len(plan.AlreadyInstalled) != 1 || plan.AlreadyInstalled[0].State != model.SkillStateOn {
		t.Fatalf("AlreadyInstalled = %#v, want matching active symlink", plan.AlreadyInstalled)
	}
}

func TestPlanInstallTreatsSameDisabledSymlinkAsInstalledOff(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	skills := discoveredSkills(t, p, "alpha")
	disabledPath := filepath.Join(p.CodexDisabledDir, "alpha")
	originalPath := filepath.Join(p.CodexUserSkills, "alpha")
	manifest := state.Manifest{Disabled: []state.DisabledEntry{{
		Tool:          model.ToolCodex,
		SkillName:     "alpha",
		OriginalPath:  originalPath,
		DisabledPath:  disabledPath,
		EntryType:     model.EntryTypeSymlink,
		SymlinkTarget: skills[0].Path,
		Source:        model.SourceSymlinkRepo,
		Group:         model.GroupLabel("addyosmani/agent-skills"),
	}}}

	plan, err := PlanInstall(p, manifest, mustIdentity(t), testCheckoutPath(t, p), skills, PlanOptions{Tools: []model.Tool{model.ToolCodex}})
	if err != nil {
		t.Fatalf("PlanInstall() error = %v", err)
	}
	if len(plan.Links) != 0 {
		t.Fatalf("Links = %#v, want empty", plan.Links)
	}
	if len(plan.AlreadyInstalled) != 1 {
		t.Fatalf("AlreadyInstalled = %#v, want one entry", plan.AlreadyInstalled)
	}
	already := plan.AlreadyInstalled[0]
	if already.State != model.SkillStateOff || already.DisabledPath != disabledPath {
		t.Fatalf("AlreadyInstalled = %#v, want OFF with disabled path", already)
	}
}

func TestPlanInstallMatchesRelativeDisabledSymlinkTarget(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	skills := discoveredSkills(t, p, "alpha")
	disabledPath := filepath.Join(p.CodexDisabledDir, "alpha")
	originalPath := filepath.Join(p.CodexUserSkills, "alpha")
	relativeTarget, err := filepath.Rel(filepath.Dir(originalPath), skills[0].Path)
	if err != nil {
		t.Fatalf("relative target: %v", err)
	}
	manifest := state.Manifest{Disabled: []state.DisabledEntry{{
		Tool:          model.ToolCodex,
		SkillName:     "alpha",
		OriginalPath:  originalPath,
		DisabledPath:  disabledPath,
		EntryType:     model.EntryTypeSymlink,
		SymlinkTarget: relativeTarget,
	}}}

	plan, err := PlanInstall(p, manifest, mustIdentity(t), testCheckoutPath(t, p), skills, PlanOptions{Tools: []model.Tool{model.ToolCodex}})
	if err != nil {
		t.Fatalf("PlanInstall() error = %v", err)
	}
	if len(plan.AlreadyInstalled) != 1 || plan.AlreadyInstalled[0].State != model.SkillStateOff {
		t.Fatalf("AlreadyInstalled = %#v, want matching disabled relative symlink", plan.AlreadyInstalled)
	}
}

func TestPlanInstallPreflightConflicts(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	skills := discoveredSkills(t, p, "alpha", "beta", "gamma")
	mkdirAll(t, p.ClaudeUserSkills)
	if err := os.WriteFile(filepath.Join(p.ClaudeUserSkills, "alpha"), []byte("blocker"), 0o644); err != nil {
		t.Fatalf("write file blocker: %v", err)
	}
	mkdirAll(t, filepath.Join(p.ClaudeUserSkills, "beta"))
	if err := os.Symlink(filepath.Join(t.TempDir(), "elsewhere"), filepath.Join(p.ClaudeUserSkills, "gamma")); err != nil {
		t.Fatalf("create wrong symlink: %v", err)
	}

	_, err := PlanInstall(p, state.Manifest{}, mustIdentity(t), testCheckoutPath(t, p), skills, PlanOptions{Tools: []model.Tool{model.ToolClaude}})
	var planErr PlanError
	if !errors.As(err, &planErr) {
		t.Fatalf("PlanInstall() error = %T %v, want PlanError", err, err)
	}
	if len(planErr.Conflicts) != 3 {
		t.Fatalf("Conflicts = %#v, want 3", planErr.Conflicts)
	}
	if planErr.Conflicts[0].SkillName != "alpha" || planErr.Conflicts[1].SkillName != "beta" || planErr.Conflicts[2].SkillName != "gamma" {
		t.Fatalf("Conflicts order = %#v, want alpha beta gamma", planErr.Conflicts)
	}
}

func TestPlanInstallConflictsWhenDisabledStatePointsElsewhere(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	skills := discoveredSkills(t, p, "alpha")
	manifest := state.Manifest{Disabled: []state.DisabledEntry{{
		Tool:          model.ToolClaude,
		SkillName:     "alpha",
		DisabledPath:  filepath.Join(p.ClaudeDisabledDir, "alpha"),
		EntryType:     model.EntryTypeSymlink,
		SymlinkTarget: filepath.Join(t.TempDir(), "other"),
	}}}

	_, err := PlanInstall(p, manifest, mustIdentity(t), testCheckoutPath(t, p), skills, PlanOptions{Tools: []model.Tool{model.ToolClaude}})
	var planErr PlanError
	if !errors.As(err, &planErr) {
		t.Fatalf("PlanInstall() error = %T %v, want PlanError", err, err)
	}
	if len(planErr.Conflicts) != 1 || !strings.Contains(planErr.Conflicts[0].Reason, "disabled state") {
		t.Fatalf("Conflicts = %#v, want disabled state conflict", planErr.Conflicts)
	}
}

func TestPlanInstallValidatesInputs(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	skills := discoveredSkills(t, p, "alpha")
	identity := mustIdentity(t)
	checkout := testCheckoutPath(t, p)
	tests := []struct {
		name      string
		identity  RepoIdentity
		checkout  string
		skills    []DiscoveredSkill
		options   PlanOptions
		wantError string
	}{
		{name: "missing url", identity: RepoIdentity{CanonicalURL: identity.CanonicalURL, Host: identity.Host, RepoPath: identity.RepoPath}, checkout: checkout, skills: skills, wantError: "repository URL"},
		{name: "missing identity", identity: RepoIdentity{OriginalURL: identity.OriginalURL}, checkout: checkout, skills: skills, wantError: "repository identity"},
		{name: "missing checkout", identity: identity, checkout: " ", skills: skills, wantError: "checkout path"},
		{name: "invalid tool", identity: identity, checkout: checkout, skills: skills, options: PlanOptions{Tools: []model.Tool{model.Tool("bad")}}, wantError: "invalid install tool"},
		{name: "duplicate discovered", identity: identity, checkout: checkout, skills: append(skills, skills[0]), wantError: "duplicate discovered skill name"},
		{name: "empty selected name", identity: identity, checkout: checkout, skills: skills, options: PlanOptions{SkillNames: []string{" "}}, wantError: "selected skill name"},
		{name: "empty discovery", identity: identity, checkout: checkout, skills: nil, wantError: "no installable skills"},
		{name: "unsafe discovered name", identity: identity, checkout: checkout, skills: []DiscoveredSkill{{Name: "../escape", Path: filepath.Join(checkout, "escape")}}, wantError: "valid basename"},
		{name: "empty discovered path", identity: identity, checkout: checkout, skills: []DiscoveredSkill{{Name: "alpha"}}, wantError: "empty path"},
		{name: "relative discovered path", identity: identity, checkout: checkout, skills: []DiscoveredSkill{{Name: "alpha", Path: "skills/alpha"}}, wantError: "path must be absolute"},
		{name: "outside discovered path", identity: identity, checkout: checkout, skills: []DiscoveredSkill{{Name: "alpha", Path: filepath.Join(t.TempDir(), "alpha")}}, wantError: "escapes checkout"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := PlanInstall(p, state.Manifest{}, test.identity, test.checkout, test.skills, test.options)
			if err == nil {
				t.Fatal("PlanInstall() error = nil, want error")
			}
			if !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("PlanInstall() error = %q, want substring %q", err, test.wantError)
			}
		})
	}
}

func TestPlanInstallDoesNotMutateFilesystemOrState(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	skills := discoveredSkills(t, p, "alpha")
	mkdirAll(t, p.ClaudeUserSkills)
	lockContents := []byte(`{"skills":[]}`)
	if err := os.MkdirAll(filepath.Dir(p.AgentsSkillLock), 0o755); err != nil {
		t.Fatalf("create agents dir: %v", err)
	}
	if err := os.WriteFile(p.AgentsSkillLock, lockContents, 0o644); err != nil {
		t.Fatalf("write lockfile: %v", err)
	}
	manifest := state.Manifest{}

	_, err := PlanInstall(p, manifest, mustIdentity(t), testCheckoutPath(t, p), skills, PlanOptions{Tools: []model.Tool{model.ToolClaude}})
	if err != nil {
		t.Fatalf("PlanInstall() error = %v", err)
	}
	if _, err := os.Lstat(filepath.Join(p.ClaudeUserSkills, "alpha")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("target path was mutated, lstat err = %v", err)
	}
	gotLock, err := os.ReadFile(p.AgentsSkillLock)
	if err != nil {
		t.Fatalf("read lockfile: %v", err)
	}
	if string(gotLock) != string(lockContents) {
		t.Fatalf("lockfile mutated: %s", gotLock)
	}
	if len(manifest.Repositories) != 0 || len(manifest.Disabled) != 0 {
		t.Fatalf("manifest argument mutated: %#v", manifest)
	}
}

func TestPlanInstallRejectsCellOwnedByLocalSource(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	skills := discoveredSkills(t, p, "alpha")
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

	_, err = PlanInstall(p, manifest, mustIdentity(t), testCheckoutPath(t, p), skills, PlanOptions{Tools: []model.Tool{model.ToolClaude}})
	var planErr PlanError
	if !errors.As(err, &planErr) || len(planErr.Conflicts) != 1 {
		t.Fatalf("PlanInstall() error = %T %v, want one ownership conflict", err, err)
	}
	if !strings.Contains(planErr.Conflicts[0].Reason, "owned by local source "+localPath) {
		t.Fatalf("conflict = %#v, want local ownership reason", planErr.Conflicts[0])
	}
}

func TestPlanInstallRejectsCellOwnedByAnotherRepository(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	skills := discoveredSkills(t, p, "alpha")
	manifest := state.Manifest{Repositories: []state.RepositoryEntry{{
		Host: "github.com", RepoPath: "other/repo",
		InstalledSkills: []state.InstalledSkillEntry{{
			Name: "alpha", RelativePath: "skills/alpha", Tools: []model.Tool{model.ToolCodex},
		}},
	}}}

	_, err := PlanInstall(p, manifest, mustIdentity(t), testCheckoutPath(t, p), skills, PlanOptions{Tools: []model.Tool{model.ToolCodex}})
	var planErr PlanError
	if !errors.As(err, &planErr) || len(planErr.Conflicts) != 1 {
		t.Fatalf("PlanInstall() error = %T %v, want one ownership conflict", err, err)
	}
	if !strings.Contains(planErr.Conflicts[0].Reason, "owned by repository github.com/other/repo") {
		t.Fatalf("conflict = %#v, want repository ownership reason", planErr.Conflicts[0])
	}
}

func discoveredSkills(t *testing.T, p paths.Paths, names ...string) []DiscoveredSkill {
	t.Helper()
	skills := make([]DiscoveredSkill, 0, len(names))
	for _, name := range names {
		dir := filepath.Join(p.ReposDir, "github.com", "addyosmani", "agent-skills", "skills", name)
		mkdirAll(t, dir)
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# "+name+"\n"), 0o644); err != nil {
			t.Fatalf("write SKILL.md: %v", err)
		}
		skills = append(skills, DiscoveredSkill{
			Name:         name,
			Path:         dir,
			RelativePath: "skills/" + name,
		})
	}
	return skills
}

func mkdirAll(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
}

func testCheckoutPath(t *testing.T, p paths.Paths) string {
	t.Helper()
	checkout, err := CheckoutPath(p, mustIdentity(t))
	if err != nil {
		t.Fatalf("CheckoutPath() error = %v", err)
	}
	return checkout
}

func sameToolSlice(got, want []model.Tool) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func assertLinkPlans(t *testing.T, got, want []LinkPlan) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("Links len = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i].Skill != want[i].Skill || got[i].Tool != want[i].Tool || got[i].TargetPath != want[i].TargetPath {
			t.Fatalf("Links[%d] = %#v, want %#v", i, got[i], want[i])
		}
	}
}
