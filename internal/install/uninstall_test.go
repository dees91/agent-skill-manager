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

func TestUninstallServicePlanDoesNotMutate(t *testing.T) {
	fixture := newUpdateGitFixture(t)
	stateBefore, err := os.ReadFile(fixture.paths.StateFile)
	if err != nil {
		t.Fatalf("read state before: %v", err)
	}

	plan, err := NewUninstallService(fixture.paths, nil).Plan(fixture.repository)
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if len(plan.References.References) != 1 || plan.References.References[0].State != model.SkillStateOn {
		t.Fatalf("references = %#v, want one active", plan.References.References)
	}
	if _, err := os.Lstat(fixture.activeSkillPath); err != nil {
		t.Fatalf("checkout changed during plan: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(fixture.paths.ClaudeUserSkills, "alpha")); err != nil {
		t.Fatalf("active link changed during plan: %v", err)
	}
	stateAfter, err := os.ReadFile(fixture.paths.StateFile)
	if err != nil {
		t.Fatalf("read state after: %v", err)
	}
	if string(stateAfter) != string(stateBefore) {
		t.Fatal("Plan() changed state")
	}
	if _, err := os.Lstat(fixture.paths.TrashDir); !os.IsNotExist(err) {
		t.Fatalf("Plan() created trash: %v", err)
	}
}

func TestUninstallServiceApplyRemovesRepositoryAndActiveLinks(t *testing.T) {
	fixture := newUpdateGitFixture(t)
	linkPath := filepath.Join(fixture.paths.ClaudeUserSkills, "alpha")

	result, err := NewUninstallService(fixture.paths, nil).Apply(fixture.repository)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if len(result.RemovedActive) != 1 || len(result.RemovedDisabled) != 0 || result.RemovedCheckout != fixture.checkoutPath {
		t.Fatalf("result = %#v, want one active link and checkout", result)
	}
	if _, err := os.Lstat(linkPath); !os.IsNotExist(err) {
		t.Fatalf("active link remains: %v", err)
	}
	if _, err := os.Lstat(fixture.checkoutPath); !os.IsNotExist(err) {
		t.Fatalf("checkout remains: %v", err)
	}
	manifest, err := state.New(fixture.paths).Load()
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if len(manifest.Repositories) != 0 || len(manifest.Disabled) != 0 {
		t.Fatalf("manifest = %#v, want no repository state", manifest)
	}
	entries, err := os.ReadDir(fixture.paths.TrashDir)
	if err != nil {
		t.Fatalf("read trash: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("trash entries = %#v, want empty", entries)
	}
}

func TestUninstallServiceRemovesDisabledLinkAndPreservesBlocker(t *testing.T) {
	fixture := newUpdateGitFixture(t)
	activePath := filepath.Join(fixture.paths.ClaudeUserSkills, "alpha")
	if err := os.Remove(activePath); err != nil {
		t.Fatalf("remove active link: %v", err)
	}
	disabledPath := filepath.Join(fixture.paths.ClaudeDisabledDir, "alpha")
	mustSymlink(t, fixture.activeSkillPath, disabledPath)
	mustWriteFile(t, activePath, "unrelated blocker")
	manifest, err := state.New(fixture.paths).Load()
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	manifest.Disabled = []state.DisabledEntry{{
		Tool:          model.ToolClaude,
		SkillName:     "alpha",
		OriginalPath:  activePath,
		DisabledPath:  disabledPath,
		EntryType:     model.EntryTypeSymlink,
		SymlinkTarget: fixture.activeSkillPath,
		Source:        model.SourceSymlinkRepo,
		Group:         fixture.repository.Group,
	}}
	if err := state.New(fixture.paths).Save(manifest); err != nil {
		t.Fatalf("save disabled state: %v", err)
	}

	result, err := NewUninstallService(fixture.paths, nil).Apply(fixture.repository)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
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
	loaded, err := state.New(fixture.paths).Load()
	if err != nil {
		t.Fatalf("load final state: %v", err)
	}
	if len(loaded.Disabled) != 0 || len(loaded.Repositories) != 0 {
		t.Fatalf("final manifest = %#v, want removed records", loaded)
	}
}

func TestUninstallServiceRollsBackOnStateSaveFailure(t *testing.T) {
	fixture := newUpdateGitFixture(t)
	linkPath := filepath.Join(fixture.paths.ClaudeUserSkills, "alpha")
	service := NewUninstallService(fixture.paths, nil)
	service.saveManifest = func(state.Manifest) error { return errors.New("save failed") }

	result, err := service.Apply(fixture.repository)
	if err == nil || !strings.Contains(err.Error(), "save failed") {
		t.Fatalf("Apply() result=%#v error=%v, want save failure", result, err)
	}
	if len(result.RolledBack) != 2 {
		t.Fatalf("RolledBack = %#v, want link and checkout", result.RolledBack)
	}
	if _, err := os.Lstat(linkPath); err != nil {
		t.Fatalf("active link not restored: %v", err)
	}
	if _, err := os.Lstat(fixture.checkoutPath); err != nil {
		t.Fatalf("checkout not restored: %v", err)
	}
	manifest, loadErr := state.New(fixture.paths).Load()
	if loadErr != nil || len(manifest.Repositories) != 1 {
		t.Fatalf("state after rollback = %#v err=%v", manifest, loadErr)
	}
}

func TestUninstallServiceRollsBackOnCheckoutStageFailure(t *testing.T) {
	fixture := newUpdateGitFixture(t)
	linkPath := filepath.Join(fixture.paths.ClaudeUserSkills, "alpha")
	service := NewUninstallService(fixture.paths, nil)
	service.rename = func(oldPath, newPath string) error {
		if filepath.Clean(oldPath) == filepath.Clean(fixture.checkoutPath) {
			return errors.New("checkout stage failed")
		}
		return os.Rename(oldPath, newPath)
	}

	result, err := service.Apply(fixture.repository)
	if err == nil || !strings.Contains(err.Error(), "checkout stage failed") {
		t.Fatalf("Apply() result=%#v error=%v, want stage failure", result, err)
	}
	if len(result.RolledBack) != 1 {
		t.Fatalf("RolledBack = %#v, want active link", result.RolledBack)
	}
	if _, err := os.Lstat(linkPath); err != nil {
		t.Fatalf("active link not restored: %v", err)
	}
	if _, err := os.Lstat(fixture.checkoutPath); err != nil {
		t.Fatalf("checkout changed: %v", err)
	}
}

func TestUninstallServiceRetainsStagingWhenRollbackIsIncomplete(t *testing.T) {
	fixture := newUpdateGitFixture(t)
	linkPath := filepath.Join(fixture.paths.ClaudeUserSkills, "alpha")
	service := NewUninstallService(fixture.paths, nil)
	service.saveManifest = func(state.Manifest) error { return errors.New("save failed") }
	service.rename = func(oldPath, newPath string) error {
		if filepath.Clean(newPath) == filepath.Clean(linkPath) {
			return errors.New("link restore failed")
		}
		return os.Rename(oldPath, newPath)
	}

	result, err := service.Apply(fixture.repository)
	if err == nil || !strings.Contains(err.Error(), "recovery data retained") {
		t.Fatalf("Apply() result=%#v error=%v, want incomplete rollback", result, err)
	}
	if len(result.RolledBack) != 1 || result.CleanupPending == "" {
		t.Fatalf("result = %#v, want checkout restored and staging retained", result)
	}
	stagedLink := filepath.Join(result.CleanupPending, "links", "ON", "claude", "alpha")
	if _, err := os.Lstat(stagedLink); err != nil {
		t.Fatalf("recovery symlink missing from staging: %v", err)
	}
	if _, err := os.Lstat(fixture.checkoutPath); err != nil {
		t.Fatalf("checkout not restored: %v", err)
	}
	manifest, loadErr := state.New(fixture.paths).Load()
	if loadErr != nil || len(manifest.Repositories) != 1 {
		t.Fatalf("state after incomplete rollback = %#v err=%v", manifest, loadErr)
	}
}

func TestUninstallServiceReportsPostSaveCleanupFailure(t *testing.T) {
	fixture := newUpdateGitFixture(t)
	service := NewUninstallService(fixture.paths, nil)
	service.removeAll = func(string) error { return errors.New("cleanup failed") }

	result, err := service.Apply(fixture.repository)
	if err == nil || !strings.Contains(err.Error(), "uninstall completed but cleanup remains") {
		t.Fatalf("Apply() result=%#v error=%v, want cleanup failure", result, err)
	}
	if result.CleanupPending == "" {
		t.Fatal("CleanupPending is empty")
	}
	if _, err := os.Lstat(result.CleanupPending); err != nil {
		t.Fatalf("cleanup staging missing: %v", err)
	}
	manifest, loadErr := state.New(fixture.paths).Load()
	if loadErr != nil || len(manifest.Repositories) != 0 {
		t.Fatalf("state after logical uninstall = %#v err=%v", manifest, loadErr)
	}
	if _, err := os.Lstat(fixture.checkoutPath); !os.IsNotExist(err) {
		t.Fatalf("checkout original path remains: %v", err)
	}
}

func TestUninstallServiceBlocksDirtyAndExtraReferences(t *testing.T) {
	t.Run("dirty checkout", func(t *testing.T) {
		fixture := newUpdateGitFixture(t)
		mustWriteFile(t, filepath.Join(fixture.checkoutPath, ".cache", "generated"), "data")
		_, err := NewUninstallService(fixture.paths, nil).Plan(fixture.repository)
		if err == nil || !strings.Contains(err.Error(), "worktree changes") {
			t.Fatalf("Plan() error = %v, want dirty conflict", err)
		}
	})

	t.Run("extra managed link", func(t *testing.T) {
		fixture := newUpdateGitFixture(t)
		mustSymlink(t, fixture.activeSkillPath, filepath.Join(fixture.paths.CodexUserSkills, "extra"))
		_, err := NewUninstallService(fixture.paths, nil).Plan(fixture.repository)
		if err == nil || !strings.Contains(err.Error(), "extra managed symlink") {
			t.Fatalf("Plan() error = %v, want extra reference conflict", err)
		}
	})
}

func TestUninstallServiceRemovesRecordedRepositoryWithNoSkills(t *testing.T) {
	fixture := newUpdateGitFixture(t)
	if err := os.Remove(filepath.Join(fixture.paths.ClaudeUserSkills, "alpha")); err != nil {
		t.Fatalf("remove active link: %v", err)
	}
	manifest, err := state.New(fixture.paths).Load()
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	repository, _ := manifest.GetRepository("github.com", "owner/repo")
	repository.InstalledSkills = []state.InstalledSkillEntry{}
	manifest.UpsertRepository(repository)
	if err := state.New(fixture.paths).Save(manifest); err != nil {
		t.Fatalf("save zero-skill state: %v", err)
	}

	result, err := NewUninstallService(fixture.paths, nil).Apply(repository)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if len(result.RemovedActive) != 0 || len(result.RemovedDisabled) != 0 {
		t.Fatalf("result = %#v, want no links", result)
	}
	if _, err := os.Lstat(fixture.checkoutPath); !os.IsNotExist(err) {
		t.Fatalf("checkout remains: %v", err)
	}
}
