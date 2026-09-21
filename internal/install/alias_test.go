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

func TestSanitizeNameSegment(t *testing.T) {
	cases := map[string]string{
		"acme":          "acme",
		"Example-Labs":  "example-labs",
		"sample_pack":   "sample-pack",
		"--odd..Name--": "odd-name",
		"a  b":          "a-b",
		"___":           "",
	}
	for input, want := range cases {
		if got := SanitizeNameSegment(input); got != want {
			t.Errorf("SanitizeNameSegment(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestSuggestInstalledNameFallsBackToRepoThenNumber(t *testing.T) {
	source := GitSourceRef(RepoIdentity{RepoPath: "Acme/Tools"})
	if source.Owner != "acme" || source.Repo != "tools" {
		t.Fatalf("GitSourceRef() = %#v", source)
	}
	taken := map[string]bool{}
	isTaken := func(name string) bool { return taken[name] }
	if got := SuggestInstalledName("alpha", source, isTaken); got != "alpha-acme" {
		t.Fatalf("suggestion = %q, want alpha-acme", got)
	}
	taken["alpha-acme"] = true
	if got := SuggestInstalledName("alpha", source, isTaken); got != "alpha-acme-tools" {
		t.Fatalf("suggestion = %q, want alpha-acme-tools", got)
	}
	taken["alpha-acme-tools"] = true
	if got := SuggestInstalledName("alpha", source, isTaken); got != "alpha-acme-2" {
		t.Fatalf("suggestion = %q, want alpha-acme-2", got)
	}
	local := LocalSourceRef("/work/Sample_Pack")
	if got := SuggestInstalledName("alpha", local, nil); got != "alpha-sample-pack" {
		t.Fatalf("local suggestion = %q, want alpha-sample-pack", got)
	}
	long := strings.Repeat("a", 60)
	if got := SuggestInstalledName(long, source, nil); len(got) > 64 || !state.ValidInstalledName(got) || !strings.HasSuffix(got, "-acme") {
		t.Fatalf("long suggestion = %q, want valid name ending in -acme", got)
	}
}

func TestResolveInstalledNames(t *testing.T) {
	discovered := Discovery{Skills: []DiscoveredSkill{{Name: "alpha"}, {Name: "beta"}}}
	cases := []struct {
		name     string
		selected []string
		explicit map[string]string
		recorded map[string]string
		want     map[string]string
		err      string
	}{
		{name: "explicit alias", selected: []string{"alpha"}, explicit: map[string]string{"alpha": "alpha-acme"}, want: map[string]string{"alpha": "alpha-acme"}},
		{name: "plain explicit is no alias", selected: []string{"alpha"}, explicit: map[string]string{"alpha": "alpha"}, want: map[string]string{}},
		{name: "recorded alias reused", selected: []string{"alpha"}, recorded: map[string]string{"alpha": "alpha-acme"}, want: map[string]string{"alpha": "alpha-acme"}},
		{name: "recorded alias matches explicit", selected: []string{"alpha"}, explicit: map[string]string{"alpha": "alpha-acme"}, recorded: map[string]string{"alpha": "alpha-acme"}, want: map[string]string{"alpha": "alpha-acme"}},
		{name: "recorded alias drift", selected: []string{"alpha"}, explicit: map[string]string{"alpha": "alpha-other"}, recorded: map[string]string{"alpha": "alpha-acme"}, err: "uninstall the source and reinstall"},
		{name: "recorded plain drift", selected: []string{"alpha"}, explicit: map[string]string{"alpha": "alpha-acme"}, recorded: map[string]string{"alpha": "alpha"}, err: "is installed as \"alpha\""},
		{name: "invalid charset", selected: []string{"alpha"}, explicit: map[string]string{"alpha": "Alpha_Acme"}, err: "invalid install name"},
		{name: "other discovered name", selected: []string{"alpha"}, explicit: map[string]string{"alpha": "beta"}, err: "another skill in the same source"},
		{name: "two aliases collide", selected: []string{"alpha", "beta"}, explicit: map[string]string{"alpha": "shared", "beta": "shared"}, err: "both use install name"},
		{name: "unselected key", selected: []string{"alpha"}, explicit: map[string]string{"beta": "beta-acme"}, err: "not selected"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveInstalledNames(discovered, tc.selected, tc.explicit, tc.recorded)
			if tc.err != "" {
				if err == nil || !strings.Contains(err.Error(), tc.err) {
					t.Fatalf("error = %v, want %q", err, tc.err)
				}
				return
			}
			if err != nil {
				t.Fatalf("error = %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("result = %#v, want %#v", got, tc.want)
			}
			for key, value := range tc.want {
				if got[key] != value {
					t.Fatalf("result = %#v, want %#v", got, tc.want)
				}
			}
		})
	}
}

// aliasLocalSource creates a local source named dir under the home workspace.
func aliasLocalSource(t *testing.T, p paths.Paths, dir string, names ...string) (LocalSource, Discovery) {
	t.Helper()
	root := filepath.Join(p.Home, "workspace", dir)
	for _, name := range names {
		writeSkill(t, filepath.Join(root, "skills", name))
	}
	source, err := ResolveLocalSource(p, filepath.Join(p.Home, "workspace"), root)
	if err != nil {
		t.Fatalf("ResolveLocalSource() error = %v", err)
	}
	discovered, err := DiscoverLocalSkills(source)
	if err != nil {
		t.Fatalf("DiscoverLocalSkills() error = %v", err)
	}
	return source, discovered
}

func applyLocal(t *testing.T, p paths.Paths, source LocalSource, discovered Discovery, options PlanOptions) LocalApplyResult {
	t.Helper()
	plan, err := PlanLocalInstall(p, loadInstallManifest(t, p), source, discovered, options)
	if err != nil {
		t.Fatalf("PlanLocalInstall() error = %v", err)
	}
	result, err := NewLocalApplyService(p).Apply(plan)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	return result
}

func singleConflict(t *testing.T, err error) PreflightConflict {
	t.Helper()
	var planErr PlanError
	if !errors.As(err, &planErr) || len(planErr.Conflicts) != 1 {
		t.Fatalf("error = %v, want one plan conflict", err)
	}
	return planErr.Conflicts[0]
}

func TestPlanInstallSuggestsAliasAndInstallsUnderIt(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	claudeOnly := PlanOptions{Tools: []model.Tool{model.ToolClaude}}
	local, localDiscovered := aliasLocalSource(t, p, "sample-pack", "alpha")
	applyLocal(t, p, local, localDiscovered, claudeOnly)

	identity, checkout := extendGitFixture(t, p, "https://github.com/acme/tools.git", "alpha")
	discovered, err := DiscoverSkills(checkout)
	if err != nil {
		t.Fatalf("DiscoverSkills() error = %v", err)
	}
	_, err = PlanInstall(p, loadInstallManifest(t, p), identity, checkout, discovered, claudeOnly)
	conflict := singleConflict(t, err)
	if !strings.HasPrefix(conflict.Reason, "cell is already owned by local source ") || conflict.SuggestedAs != "alpha-acme" || conflict.OwnerGroup != "sample-pack" {
		t.Fatalf("conflict = %#v, want owner reason, alpha-acme suggestion, sample-pack owner", conflict)
	}

	aliased := PlanOptions{Tools: []model.Tool{model.ToolClaude}, InstalledAs: map[string]string{"alpha": "alpha-acme"}}
	plan, err := PlanInstall(p, loadInstallManifest(t, p), identity, checkout, discovered, aliased)
	if err != nil {
		t.Fatalf("aliased PlanInstall() error = %v", err)
	}
	if len(plan.Links) != 1 || plan.Links[0].TargetPath != filepath.Join(p.ClaudeUserSkills, "alpha-acme") {
		t.Fatalf("Links = %#v, want claude/alpha-acme", plan.Links)
	}
	if _, err := NewApplyService(p).Apply(plan, ""); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	assertSymlinkTarget(t, filepath.Join(p.ClaudeUserSkills, "alpha-acme"), filepath.Join(checkout, "skills", "alpha"))
	assertSymlinkTarget(t, filepath.Join(p.ClaudeUserSkills, "alpha"), filepath.Join(local.CanonicalPath, "skills", "alpha"))
	manifest := loadInstallManifest(t, p)
	repository, _ := manifest.GetRepository(identity.Host, identity.RepoPath)
	if len(repository.InstalledSkills) != 1 || repository.InstalledSkills[0].Name != "alpha" || repository.InstalledSkills[0].InstalledAs != "alpha-acme" {
		t.Fatalf("InstalledSkills = %#v, want alpha installed as alpha-acme", repository.InstalledSkills)
	}
	if _, err := AuditRepositoryReferences(p, manifest, repository); err != nil {
		t.Fatalf("AuditRepositoryReferences() error = %v", err)
	}

	// Reinstall without --as reuses the recorded alias; a different one is drift.
	plan, err = PlanInstall(p, manifest, identity, checkout, discovered, claudeOnly)
	if err != nil {
		t.Fatalf("reinstall PlanInstall() error = %v", err)
	}
	if len(plan.Links) != 0 || len(plan.AlreadyInstalled) != 1 || plan.AlreadyInstalled[0].InstalledName() != "alpha-acme" {
		t.Fatalf("reinstall plan = %#v, want alpha-acme already installed", plan)
	}
	drift := PlanOptions{Tools: []model.Tool{model.ToolClaude}, InstalledAs: map[string]string{"alpha": "alpha-other"}}
	if _, err := PlanInstall(p, manifest, identity, checkout, discovered, drift); err == nil || !strings.Contains(err.Error(), "uninstall the source and reinstall") {
		t.Fatalf("drift error = %v, want uninstall guidance", err)
	}
}

func TestPlanInstallRecordedPlainNameGetsGuidanceWithoutSuggestion(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	first, firstDiscovered := aliasLocalSource(t, p, "first-pack", "alpha")
	second, secondDiscovered := aliasLocalSource(t, p, "second-pack", "alpha")
	applyLocal(t, p, first, firstDiscovered, PlanOptions{Tools: []model.Tool{model.ToolCodex}})
	applyLocal(t, p, second, secondDiscovered, PlanOptions{Tools: []model.Tool{model.ToolClaude}})

	_, err := PlanLocalInstall(p, loadInstallManifest(t, p), second, secondDiscovered, PlanOptions{Tools: []model.Tool{model.ToolCodex}})
	conflict := singleConflict(t, err)
	if conflict.SuggestedAs != "" || !strings.Contains(conflict.Reason, "reinstall it with --as") {
		t.Fatalf("conflict = %#v, want guidance and no suggestion", conflict)
	}
}

func TestPlanInstallAliasCollidingWithExistingPathConflicts(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	source, discovered := aliasLocalSource(t, p, "sample-pack", "alpha")
	mkdir(t, filepath.Join(p.ClaudeUserSkills, "alpha-sample-pack"))
	options := PlanOptions{Tools: []model.Tool{model.ToolClaude}, InstalledAs: map[string]string{"alpha": "alpha-sample-pack"}}
	_, err := PlanLocalInstall(p, state.Manifest{}, source, discovered, options)
	if conflict := singleConflict(t, err); conflict.Reason != "target path already exists" {
		t.Fatalf("conflict = %#v, want existing path", conflict)
	}
}

func TestLocalAliasOffUninstallAndExtend(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	first, firstDiscovered := aliasLocalSource(t, p, "first-pack", "alpha")
	second, secondDiscovered := aliasLocalSource(t, p, "second-pack", "alpha")
	applyLocal(t, p, first, firstDiscovered, PlanOptions{Tools: []model.Tool{model.ToolClaude}})
	applyLocal(t, p, second, secondDiscovered, PlanOptions{Tools: []model.Tool{model.ToolClaude}, Off: true, InstalledAs: map[string]string{"alpha": "alpha-second-pack"}})

	manifest := loadInstallManifest(t, p)
	disabled, ok := manifest.Get(model.ToolClaude, "alpha-second-pack")
	if !ok || disabled.OriginalPath != filepath.Join(p.ClaudeUserSkills, "alpha-second-pack") || disabled.Group != "second-pack" {
		t.Fatalf("disabled entry = %#v ok=%v, want alpha-second-pack in second-pack", disabled, ok)
	}
	if _, err := os.Lstat(filepath.Join(p.ClaudeUserSkills, "alpha-second-pack")); !os.IsNotExist(err) {
		t.Fatalf("active alias link exists for an OFF install: %v", err)
	}
	for _, source := range manifest.LocalSources {
		if _, err := AuditLocalSourceReferences(p, manifest, source, true); err != nil {
			t.Fatalf("audit %s: %v", source.Group, err)
		}
	}

	extend, err := PlanExtend(p, manifest, model.ToolMuse)
	if err != nil {
		t.Fatalf("PlanExtend() error = %v", err)
	}
	for _, group := range []string{"first-pack", "second-pack"} {
		if status := extendSourceByGroup(extend, group).Status; status != ExtendStatusReady {
			t.Fatalf("%s extend status = %q (%v), want ready", group, status, extendSourceByGroup(extend, group).Err)
		}
	}
	if after := extendSourceByGroup(extend, "second-pack").DisableAfter; len(after) != 1 || after[0] != "alpha-second-pack" {
		t.Fatalf("DisableAfter = %#v, want alpha-second-pack", after)
	}
	if _, err := NewExtendService(p).Apply(model.ToolMuse, nil); err != nil {
		t.Fatalf("extend Apply() error = %v", err)
	}
	assertSymlinkTarget(t, filepath.Join(p.MuseUserSkills, "alpha"), filepath.Join(first.CanonicalPath, "skills", "alpha"))
	manifest = loadInstallManifest(t, p)
	if _, ok := manifest.Get(model.ToolMuse, "alpha-second-pack"); !ok {
		t.Fatalf("muse alias was not disabled after extend: %#v", manifest.Disabled)
	}

	entry, _ := manifest.GetLocalSource(second.CanonicalPath)
	result, err := NewLocalUninstallService(p).Apply(entry)
	if err != nil {
		t.Fatalf("uninstall Apply() error = %v", err)
	}
	if len(result.RemovedDisabled) != 2 {
		t.Fatalf("RemovedDisabled = %#v, want claude and muse aliases", result.RemovedDisabled)
	}
	manifest = loadInstallManifest(t, p)
	if len(manifest.Disabled) != 0 || len(manifest.LocalSources) != 1 {
		t.Fatalf("manifest after uninstall = %#v, want only first-pack", manifest)
	}
	assertSymlinkTarget(t, filepath.Join(p.ClaudeUserSkills, "alpha"), filepath.Join(first.CanonicalPath, "skills", "alpha"))
}

func TestAuditRejectsTwoSkillsSharingInstalledName(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	source, discovered := aliasLocalSource(t, p, "sample-pack", "alpha", "beta")
	result := applyLocal(t, p, source, discovered, PlanOptions{Tools: []model.Tool{model.ToolClaude}, SkillNames: []string{"alpha"}})
	entry := result.Source
	entry.InstalledSkills = append(entry.InstalledSkills, state.InstalledSkillEntry{Name: "beta", InstalledAs: "alpha", RelativePath: "skills/beta", Tools: []model.Tool{model.ToolClaude}})
	manifest := loadInstallManifest(t, p)
	if _, err := AuditLocalSourceReferences(p, manifest, entry, true); err == nil || !strings.Contains(err.Error(), "share install name alpha") {
		t.Fatalf("audit error = %v, want shared install name conflict", err)
	}
}
