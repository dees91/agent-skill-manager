package gui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dees91/agent-skill-manager/internal/install"
	"github.com/dees91/agent-skill-manager/internal/model"
	"github.com/dees91/agent-skill-manager/internal/paths"
	"github.com/dees91/agent-skill-manager/internal/state"
)

func TestInspectSourcesReportsRepairableAndCleanHealth(t *testing.T) {
	fixture := newRepairFixture(t)
	service := New(fixture.paths)

	clean, err := service.InspectSources()
	if err != nil {
		t.Fatalf("InspectSources() error = %v", err)
	}
	if len(clean) != 1 || clean[0].Status != SourceHealthOK || clean[0].Repairable {
		t.Fatalf("health = %#v, want a single healthy source", clean)
	}

	writeRepairFile(t, filepath.Join(fixture.checkoutPath, "generated", "trace_processor"), "binary")

	dirty, err := service.InspectSources()
	if err != nil {
		t.Fatalf("InspectSources() error = %v", err)
	}
	if len(dirty) != 1 {
		t.Fatalf("health = %#v, want one source", dirty)
	}
	entry := dirty[0]
	if entry.Status != SourceHealthNeedsRepair || !entry.Repairable || entry.Kind != string(install.CheckoutConflictDirtyWorktree) {
		t.Fatalf("health = %#v, want a repairable dirty worktree", entry)
	}
	if entry.SourceID != clean[0].SourceID || entry.Group != "owner/repo" {
		t.Fatalf("health identity = %#v, want the stable opaque source id", entry)
	}
	if strings.Contains(entry.Cause, fixture.paths.Home) || strings.Contains(entry.Remedy, fixture.paths.Home) {
		t.Fatalf("health leaks filesystem paths: %#v", entry)
	}
}

func TestInspectSourcesReportsNonRepairableBlockers(t *testing.T) {
	fixture := newRepairFixture(t)
	runGitRepair(t, "-C", fixture.checkoutPath, "checkout", "--detach")

	health, err := New(fixture.paths).InspectSources()
	if err != nil {
		t.Fatalf("InspectSources() error = %v", err)
	}
	if len(health) != 1 || health[0].Status != SourceHealthBlocked || health[0].Repairable {
		t.Fatalf("health = %#v, want a blocked, non-repairable source", health)
	}
	if health[0].Kind != string(install.CheckoutConflictDetachedHead) {
		t.Fatalf("kind = %q, want detached head", health[0].Kind)
	}
}

func TestPreviewAndRepairSourceClearTheCheckout(t *testing.T) {
	fixture := newRepairFixture(t)
	writeRepairFile(t, filepath.Join(fixture.checkoutPath, "generated", "trace_processor"), "binary")
	writeRepairFile(t, filepath.Join(fixture.checkoutPath, ".cache", "index"), "cache")
	service := New(fixture.paths)
	snapshot, err := service.GetSnapshot(false)
	if err != nil {
		t.Fatalf("GetSnapshot() error = %v", err)
	}
	sourceID := snapshot.ManagedSources[0].SourceID

	preview, err := service.PreviewRepair(sourceID)
	if err != nil {
		t.Fatalf("PreviewRepair() error = %v", err)
	}
	if preview.Clean || len(preview.Entries) != 2 || preview.UntrackedCount != 1 || preview.IgnoredCount != 1 {
		t.Fatalf("preview = %#v, want one untracked and one ignored path", preview)
	}
	for _, entry := range preview.Entries {
		if filepath.IsAbs(entry.Path) || strings.Contains(entry.Path, fixture.paths.Home) {
			t.Fatalf("preview entry %#v leaks an absolute path", entry)
		}
	}
	if _, err := os.Lstat(filepath.Join(fixture.checkoutPath, "generated", "trace_processor")); err != nil {
		t.Fatalf("preview mutated the checkout: %v", err)
	}

	result := service.RepairSource(sourceID, false)

	if result.Failure != nil {
		t.Fatalf("RepairSource() failure = %#v", result.Failure)
	}
	if len(result.Completed) != 1 || result.Completed[0].Status != "repaired" || result.Completed[0].SourceID != sourceID {
		t.Fatalf("completed = %#v, want one repaired source", result.Completed)
	}
	if len(result.Snapshot.ManagedSources) != 1 {
		t.Fatalf("snapshot = %#v, want a fresh projection", result.Snapshot)
	}
	if _, err := os.Lstat(filepath.Join(fixture.checkoutPath, "generated")); !os.IsNotExist(err) {
		t.Fatalf("blocking path survived repair: %v", err)
	}
	if entries, err := os.ReadDir(fixture.paths.TrashDir); err == nil && len(entries) != 0 {
		t.Fatalf("staging residue = %#v", entries)
	}

	health, err := service.InspectSources()
	if err != nil || len(health) != 1 || health[0].Status != SourceHealthOK {
		t.Fatalf("health = %#v err=%v, want a healthy source after repair", health, err)
	}

	repeat := service.RepairSource(sourceID, false)
	if repeat.Failure != nil || repeat.Completed[0].Status != "already-clean" {
		t.Fatalf("second repair = %#v, want an idempotent no-op", repeat)
	}
}

func TestUpdateFailureCarriesRepairableClassification(t *testing.T) {
	fixture := newRepairFixture(t)
	writeRepairFile(t, filepath.Join(fixture.checkoutPath, "stray.txt"), "block")
	service := New(fixture.paths)

	result := service.UpdateAllSources(false)

	if result.Failure == nil {
		t.Fatal("UpdateAllSources() succeeded, want a dirty checkout failure")
	}
	failure := result.Failure
	if !failure.Repairable || failure.Kind != string(install.CheckoutConflictDirtyWorktree) {
		t.Fatalf("failure = %#v, want a repairable dirty worktree", failure)
	}
	if failure.SourceID == "" || failure.Group != "owner/repo" {
		t.Fatalf("failure = %#v, want the opaque source id and group", failure)
	}
	if !strings.Contains(failure.Cause, "1 untracked") {
		t.Fatalf("cause = %q, want the untracked count", failure.Cause)
	}

	repaired := service.RepairSource(failure.SourceID, false)
	if repaired.Failure != nil {
		t.Fatalf("RepairSource() failure = %#v", repaired.Failure)
	}
}

func TestRepairRejectsPendingSkillChanges(t *testing.T) {
	fixture := newRepairFixture(t)
	writeRepairFile(t, filepath.Join(fixture.checkoutPath, "stray.txt"), "block")
	service := New(fixture.paths)
	snapshot, err := service.GetSnapshot(false)
	if err != nil {
		t.Fatalf("GetSnapshot() error = %v", err)
	}
	sourceID := snapshot.ManagedSources[0].SourceID
	if _, err := service.ToggleCell("alpha", "claude"); err != nil {
		t.Fatalf("ToggleCell() error = %v", err)
	}

	if _, err := service.PreviewRepair(sourceID); err == nil || !strings.Contains(err.Error(), "pending") {
		t.Fatalf("PreviewRepair() error = %v, want the pending guard", err)
	}
	result := service.RepairSource(sourceID, false)
	if result.Failure == nil || !strings.Contains(result.Failure.Message, "pending") {
		t.Fatalf("RepairSource() failure = %#v, want the pending guard", result.Failure)
	}
	if _, err := os.Lstat(filepath.Join(fixture.checkoutPath, "stray.txt")); err != nil {
		t.Fatalf("blocked repair still mutated the checkout: %v", err)
	}
}

type repairFixture struct {
	paths        paths.Paths
	checkoutPath string
}

func newRepairFixture(t *testing.T) repairFixture {
	t.Helper()
	root := t.TempDir()
	p := paths.ForHome(filepath.Join(root, "home"))
	remotePath := filepath.Join(root, "remote.git")
	sourcePath := filepath.Join(root, "source")
	runGitRepair(t, "init", "--bare", remotePath)
	runGitRepair(t, "init", "-b", "main", sourcePath)
	writeRepairFile(t, filepath.Join(sourcePath, ".gitignore"), ".cache/\n")
	writeRepairFile(t, filepath.Join(sourcePath, "skills", "alpha", "SKILL.md"), "---\nname: alpha\ndescription: Alpha\n---\n")
	runGitRepair(t, "-C", sourcePath, "add", ".")
	runGitRepair(t, "-C", sourcePath, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "initial")
	runGitRepair(t, "-C", sourcePath, "remote", "add", "origin", remotePath)
	runGitRepair(t, "-C", sourcePath, "push", "-u", "origin", "main")
	runGitRepair(t, "--git-dir", remotePath, "symbolic-ref", "HEAD", "refs/heads/main")

	identity, err := install.NormalizeGitURL("https://github.com/owner/repo")
	if err != nil {
		t.Fatalf("NormalizeGitURL() error = %v", err)
	}
	checkoutPath, err := install.CheckoutPath(p, identity)
	if err != nil {
		t.Fatalf("CheckoutPath() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(checkoutPath), 0o755); err != nil {
		t.Fatalf("create checkout parent: %v", err)
	}
	gitConfigPath := filepath.Join(root, "gitconfig")
	writeRepairFile(t, gitConfigPath, "[url \""+remotePath+"\"]\n\tinsteadOf = "+identity.OriginalURL+"\n")
	t.Setenv("GIT_CONFIG_GLOBAL", gitConfigPath)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	runGitRepair(t, "clone", remotePath, checkoutPath)
	runGitRepair(t, "-C", checkoutPath, "remote", "set-url", "origin", identity.OriginalURL)
	if err := os.MkdirAll(p.ClaudeUserSkills, 0o755); err != nil {
		t.Fatalf("create claude skills dir: %v", err)
	}
	if err := os.Symlink(filepath.Join(checkoutPath, "skills", "alpha"), filepath.Join(p.ClaudeUserSkills, "alpha")); err != nil {
		t.Fatalf("create managed symlink: %v", err)
	}
	commit := strings.TrimSpace(runGitRepair(t, "-C", checkoutPath, "rev-parse", "HEAD"))
	manifest := state.Manifest{Repositories: []state.RepositoryEntry{{
		OriginalURL:    identity.OriginalURL,
		CanonicalURL:   identity.CanonicalURL,
		Host:           identity.Host,
		RepoPath:       identity.RepoPath,
		CheckoutPath:   checkoutPath,
		Group:          identity.Group,
		LastSeenCommit: commit,
		InstalledSkills: []state.InstalledSkillEntry{{
			Name:         "alpha",
			RelativePath: "skills/alpha",
			Tools:        []model.Tool{model.ToolClaude},
		}},
	}}}
	if err := state.New(p).Save(manifest); err != nil {
		t.Fatalf("save state: %v", err)
	}
	return repairFixture{paths: p, checkoutPath: checkoutPath}
}

func runGitRepair(t *testing.T, args ...string) string {
	t.Helper()
	output, err := exec.Command("git", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
	return string(output)
}

func writeRepairFile(t *testing.T, filePath, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		t.Fatalf("create parent for %s: %v", filePath, err)
	}
	if err := os.WriteFile(filePath, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", filePath, err)
	}
}
