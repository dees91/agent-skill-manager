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

func TestAuditLocalSourceReferencesAcceptsActiveAndDisabledWithBlocker(t *testing.T) {
	p, source, discovered := localInstallFixture(t, "alpha", "beta")
	plan, err := PlanLocalInstall(p, state.Manifest{}, source, discovered, PlanOptions{})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if _, err := NewLocalApplyService(p).Apply(plan); err != nil {
		t.Fatalf("apply: %v", err)
	}
	manifest, _ := state.New(p).Load()
	activePath := filepath.Join(p.CodexUserSkills, "beta")
	disabledPath := filepath.Join(p.CodexDisabledDir, "beta")
	if err := os.MkdirAll(filepath.Dir(disabledPath), 0o755); err != nil {
		t.Fatalf("create disabled parent: %v", err)
	}
	if err := os.Rename(activePath, disabledPath); err != nil {
		t.Fatalf("disable beta: %v", err)
	}
	mustWriteFile(t, activePath, "unrelated blocker")
	betaPath := filepath.Join(source.CanonicalPath, "skills", "beta")
	manifest.Upsert(state.DisabledEntry{
		Tool: model.ToolCodex, SkillName: "beta", OriginalPath: activePath, DisabledPath: disabledPath,
		EntryType: model.EntryTypeSymlink, SymlinkTarget: betaPath, Source: model.SourceLocalPath, Group: source.Group,
	})
	if err := state.New(p).Save(manifest); err != nil {
		t.Fatalf("save disabled state: %v", err)
	}
	entry, _ := manifest.GetLocalSource(source.CanonicalPath)
	audit, err := AuditLocalSourceReferences(p, manifest, entry, true)
	if err != nil {
		t.Fatalf("AuditLocalSourceReferences() error = %v", err)
	}
	if len(audit.References) != 4 {
		t.Fatalf("References = %#v, want four cells", audit.References)
	}
	contents, err := os.ReadFile(activePath)
	if err != nil || string(contents) != "unrelated blocker" {
		t.Fatalf("blocker changed: %q err=%v", contents, err)
	}
}

func TestAuditLocalSourceReferencesAllowsMissingSourceOnlyForCleanup(t *testing.T) {
	p, source, discovered := localInstallFixture(t, "alpha")
	plan, _ := PlanLocalInstall(p, state.Manifest{}, source, discovered, PlanOptions{Tools: []model.Tool{model.ToolClaude}})
	if _, err := NewLocalApplyService(p).Apply(plan); err != nil {
		t.Fatalf("apply: %v", err)
	}
	manifest, _ := state.New(p).Load()
	entry, _ := manifest.GetLocalSource(source.CanonicalPath)
	if err := os.RemoveAll(source.CanonicalPath); err != nil {
		t.Fatalf("remove local source: %v", err)
	}
	if _, err := AuditLocalSourceReferences(p, manifest, entry, true); err == nil {
		t.Fatal("require-source audit error = nil")
	}
	if _, err := AuditLocalSourceReferences(p, manifest, entry, false); err != nil {
		t.Fatalf("cleanup audit error = %v", err)
	}
}

func TestAuditLocalSourceReferencesRejectsExtraManagedLink(t *testing.T) {
	p, source, discovered := localInstallFixture(t, "alpha")
	plan, _ := PlanLocalInstall(p, state.Manifest{}, source, discovered, PlanOptions{Tools: []model.Tool{model.ToolClaude}})
	if _, err := NewLocalApplyService(p).Apply(plan); err != nil {
		t.Fatalf("apply: %v", err)
	}
	extra := filepath.Join(p.CodexUserSkills, "extra")
	mustSymlink(t, discovered[0].Path, extra)
	manifest, _ := state.New(p).Load()
	entry, _ := manifest.GetLocalSource(source.CanonicalPath)
	_, err := AuditLocalSourceReferences(p, manifest, entry, false)
	if err == nil || !strings.Contains(err.Error(), "extra managed symlink") {
		t.Fatalf("AuditLocalSourceReferences() error = %v, want extra link conflict", err)
	}
}

func TestLocalUninstallRemovesLinksAndStateButPreservesSource(t *testing.T) {
	p, source, discovered := localInstallFixture(t, "alpha")
	plan, _ := PlanLocalInstall(p, state.Manifest{}, source, discovered, PlanOptions{})
	result, err := NewLocalApplyService(p).Apply(plan)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	uninstallResult, err := NewLocalUninstallService(p).Apply(result.Source)
	if err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	if len(uninstallResult.RemovedActive) != 2 || len(uninstallResult.RemovedDisabled) != 0 {
		t.Fatalf("result = %#v, want two active links", uninstallResult)
	}
	for _, tool := range model.Tools() {
		dir, _ := p.UserSkillsDirFor(tool)
		if _, err := os.Lstat(filepath.Join(dir, "alpha")); !os.IsNotExist(err) {
			t.Fatalf("%s link remains: %v", tool, err)
		}
	}
	if _, err := os.Lstat(filepath.Join(source.CanonicalPath, "skills", "alpha", "SKILL.md")); err != nil {
		t.Fatalf("source was not preserved: %v", err)
	}
	manifest, _ := state.New(p).Load()
	if len(manifest.LocalSources) != 0 || len(manifest.Disabled) != 0 {
		t.Fatalf("manifest = %#v, want local state removed", manifest)
	}
}

func TestLocalUninstallCleansBrokenLinkAfterSourceRemoval(t *testing.T) {
	p, source, discovered := localInstallFixture(t, "alpha")
	plan, _ := PlanLocalInstall(p, state.Manifest{}, source, discovered, PlanOptions{Tools: []model.Tool{model.ToolClaude}})
	installResult, err := NewLocalApplyService(p).Apply(plan)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if err := os.RemoveAll(source.CanonicalPath); err != nil {
		t.Fatalf("remove source: %v", err)
	}
	if _, err := NewLocalUninstallService(p).Apply(installResult.Source); err != nil {
		t.Fatalf("uninstall missing source: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(p.ClaudeUserSkills, "alpha")); !os.IsNotExist(err) {
		t.Fatalf("broken link remains: %v", err)
	}
	manifest, _ := state.New(p).Load()
	if len(manifest.LocalSources) != 0 {
		t.Fatalf("LocalSources = %#v, want empty", manifest.LocalSources)
	}
}

func TestLocalUninstallRemovesDisabledLinkAndPreservesActiveBlocker(t *testing.T) {
	p, source, discovered := localInstallFixture(t, "alpha")
	plan, _ := PlanLocalInstall(p, state.Manifest{}, source, discovered, PlanOptions{Tools: []model.Tool{model.ToolClaude}})
	installResult, err := NewLocalApplyService(p).Apply(plan)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	activePath := filepath.Join(p.ClaudeUserSkills, "alpha")
	disabledPath := filepath.Join(p.ClaudeDisabledDir, "alpha")
	if err := os.MkdirAll(filepath.Dir(disabledPath), 0o755); err != nil {
		t.Fatalf("create disabled parent: %v", err)
	}
	if err := os.Rename(activePath, disabledPath); err != nil {
		t.Fatalf("move active link: %v", err)
	}
	mustWriteFile(t, activePath, "unrelated blocker")
	manifest, _ := state.New(p).Load()
	manifest.Upsert(state.DisabledEntry{
		Tool: model.ToolClaude, SkillName: "alpha", OriginalPath: activePath, DisabledPath: disabledPath,
		EntryType: model.EntryTypeSymlink, SymlinkTarget: discovered[0].Path, Source: model.SourceLocalPath, Group: source.Group,
	})
	if err := state.New(p).Save(manifest); err != nil {
		t.Fatalf("save disabled state: %v", err)
	}

	result, err := NewLocalUninstallService(p).Apply(installResult.Source)
	if err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	if len(result.RemovedDisabled) != 1 || len(result.RemovedActive) != 0 {
		t.Fatalf("result = %#v, want one disabled link", result)
	}
	contents, err := os.ReadFile(activePath)
	if err != nil || string(contents) != "unrelated blocker" {
		t.Fatalf("blocker = %q err=%v, want preserved", contents, err)
	}
	if _, err := os.Lstat(disabledPath); !os.IsNotExist(err) {
		t.Fatalf("disabled link remains: %v", err)
	}
	if _, err := os.Lstat(source.CanonicalPath); err != nil {
		t.Fatalf("source changed: %v", err)
	}
}

func TestLocalUninstallRollsBackOnSaveFailure(t *testing.T) {
	p, source, discovered := localInstallFixture(t, "alpha")
	plan, _ := PlanLocalInstall(p, state.Manifest{}, source, discovered, PlanOptions{Tools: []model.Tool{model.ToolClaude}})
	installResult, err := NewLocalApplyService(p).Apply(plan)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	service := NewLocalUninstallService(p)
	service.saveManifest = func(state.Manifest) error { return errors.New("save failed") }
	result, err := service.Apply(installResult.Source)
	if err == nil || !strings.Contains(err.Error(), "save failed") || len(result.RolledBack) != 1 {
		t.Fatalf("Apply() result=%#v error=%v, want rollback", result, err)
	}
	if _, err := os.Lstat(filepath.Join(p.ClaudeUserSkills, "alpha")); err != nil {
		t.Fatalf("link not restored: %v", err)
	}
	if _, err := os.Lstat(source.CanonicalPath); err != nil {
		t.Fatalf("source changed: %v", err)
	}
	manifest, _ := state.New(p).Load()
	if len(manifest.LocalSources) != 1 {
		t.Fatalf("manifest = %#v, want source retained", manifest)
	}
}

func TestLocalUninstallRetainsRecoveryDataAfterIncompleteRollback(t *testing.T) {
	p, source, discovered := localInstallFixture(t, "alpha")
	plan, _ := PlanLocalInstall(p, state.Manifest{}, source, discovered, PlanOptions{Tools: []model.Tool{model.ToolClaude}})
	installResult, err := NewLocalApplyService(p).Apply(plan)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	activePath := filepath.Join(p.ClaudeUserSkills, "alpha")
	service := NewLocalUninstallService(p)
	service.saveManifest = func(state.Manifest) error { return errors.New("save failed") }
	service.rename = func(oldPath, newPath string) error {
		if filepath.Clean(newPath) == filepath.Clean(activePath) {
			return errors.New("restore failed")
		}
		return os.Rename(oldPath, newPath)
	}

	result, err := service.Apply(installResult.Source)
	if err == nil || !strings.Contains(err.Error(), "recovery data retained") || result.CleanupPending == "" {
		t.Fatalf("Apply() result=%#v error=%v, want retained recovery", result, err)
	}
	stagedLink := filepath.Join(result.CleanupPending, "links", "ON", "claude", "alpha")
	if _, err := os.Lstat(stagedLink); err != nil {
		t.Fatalf("staged recovery link missing: %v", err)
	}
	if _, err := os.Lstat(source.CanonicalPath); err != nil {
		t.Fatalf("source changed: %v", err)
	}
}

func TestLocalUninstallReportsPostSaveCleanupFailure(t *testing.T) {
	p, source, discovered := localInstallFixture(t, "alpha")
	plan, _ := PlanLocalInstall(p, state.Manifest{}, source, discovered, PlanOptions{Tools: []model.Tool{model.ToolClaude}})
	installResult, err := NewLocalApplyService(p).Apply(plan)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	service := NewLocalUninstallService(p)
	service.removeAll = func(string) error { return errors.New("cleanup failed") }

	result, err := service.Apply(installResult.Source)
	if err == nil || !strings.Contains(err.Error(), "cleanup remains") || result.CleanupPending == "" {
		t.Fatalf("Apply() result=%#v error=%v, want cleanup residue", result, err)
	}
	manifest, loadErr := state.New(p).Load()
	if loadErr != nil || len(manifest.LocalSources) != 0 {
		t.Fatalf("manifest = %#v err=%v, want logical uninstall", manifest, loadErr)
	}
	if _, err := os.Lstat(source.CanonicalPath); err != nil {
		t.Fatalf("source changed: %v", err)
	}
}
