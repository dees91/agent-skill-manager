package install

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dees91/agent-skill-manager/internal/model"
	"github.com/dees91/agent-skill-manager/internal/paths"
	"github.com/dees91/agent-skill-manager/internal/state"
)

func TestUpdateServicePlanLocalDoesNotFetchOrMutate(t *testing.T) {
	fixture := newUpdateGitFixture(t)
	remoteCommit := fixture.commitRemoteChange("update alpha", map[string]string{"skills/alpha/SKILL.md": "# alpha v2\n"}, nil)
	stateBefore, err := os.ReadFile(fixture.paths.StateFile)
	if err != nil {
		t.Fatalf("read state before: %v", err)
	}

	plan, err := NewUpdateService(fixture.paths, nil).PlanLocal(fixture.repository)
	if err != nil {
		t.Fatalf("PlanLocal() error = %v", err)
	}
	if plan.Checkout.HeadCommit != fixture.initialCommit || plan.Checkout.UpstreamCommit != fixture.initialCommit {
		t.Fatalf("checkout = %#v, want cached initial commit", plan.Checkout)
	}
	if got := fixture.gitCheckout("rev-parse", "origin/main"); got != fixture.initialCommit {
		t.Fatalf("origin/main after dry-run = %q, want %q (remote is %q)", got, fixture.initialCommit, remoteCommit)
	}
	stateAfter, err := os.ReadFile(fixture.paths.StateFile)
	if err != nil {
		t.Fatalf("read state after: %v", err)
	}
	if string(stateAfter) != string(stateBefore) {
		t.Fatal("PlanLocal() changed state")
	}
}

func TestUpdateServiceApplyFastForwardsAndUpdatesState(t *testing.T) {
	fixture := newUpdateGitFixture(t)
	targetCommit := fixture.commitRemoteChange("update alpha", map[string]string{"skills/alpha/SKILL.md": "# alpha v2\n"}, nil)
	linkPath := filepath.Join(fixture.paths.ClaudeUserSkills, "alpha")
	rawTargetBefore, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("read active link before: %v", err)
	}

	result, err := NewUpdateService(fixture.paths, nil).Apply(fixture.repository)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if !result.Updated || result.UpToDate || result.PreviousCommit != fixture.initialCommit || result.CurrentCommit != targetCommit {
		t.Fatalf("result = %#v, want fast-forward", result)
	}
	if got := fixture.gitCheckout("rev-parse", "HEAD"); got != targetCommit {
		t.Fatalf("HEAD = %q, want %q", got, targetCommit)
	}
	manifest, err := state.New(fixture.paths).Load()
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	repository, ok := manifest.GetRepository("github.com", "owner/repo")
	if !ok || repository.LastSeenCommit != targetCommit {
		t.Fatalf("repository state = %#v, want commit %s", repository, targetCommit)
	}
	rawTargetAfter, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("read active link after: %v", err)
	}
	if rawTargetAfter != rawTargetBefore {
		t.Fatalf("active link target changed from %q to %q", rawTargetBefore, rawTargetAfter)
	}
	contents, err := os.ReadFile(filepath.Join(fixture.checkoutPath, "skills", "alpha", "SKILL.md"))
	if err != nil || string(contents) != "# alpha v2\n" {
		t.Fatalf("updated SKILL.md = %q err=%v", contents, err)
	}
}

func TestUpdateServiceBlocksRemovedInstalledSkillBeforeMerge(t *testing.T) {
	fixture := newUpdateGitFixture(t)
	targetCommit := fixture.commitRemoteChange("remove alpha", nil, []string{"skills/alpha/SKILL.md"})

	result, err := NewUpdateService(fixture.paths, nil).Apply(fixture.repository)
	if err == nil || !strings.Contains(err.Error(), "installed skills missing regular SKILL.md") {
		t.Fatalf("Apply() result=%#v error=%v, want removed skill conflict", result, err)
	}
	if got := fixture.gitCheckout("rev-parse", "HEAD"); got != fixture.initialCommit {
		t.Fatalf("HEAD = %q, want unchanged %q", got, fixture.initialCommit)
	}
	if got := fixture.gitCheckout("rev-parse", "origin/main"); got != targetCommit {
		t.Fatalf("origin/main = %q, want fetched %q", got, targetCommit)
	}
	manifest, loadErr := state.New(fixture.paths).Load()
	if loadErr != nil {
		t.Fatalf("load state: %v", loadErr)
	}
	repository, _ := manifest.GetRepository("github.com", "owner/repo")
	if repository.LastSeenCommit != fixture.initialCommit {
		t.Fatalf("LastSeenCommit = %q, want unchanged", repository.LastSeenCommit)
	}
}

func TestUpdateServiceDoesNotAutoInstallNewSkills(t *testing.T) {
	fixture := newUpdateGitFixture(t)
	fixture.commitRemoteChange("add beta", map[string]string{"skills/beta/SKILL.md": "# beta\n"}, nil)

	if _, err := NewUpdateService(fixture.paths, nil).Apply(fixture.repository); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if _, err := os.Lstat(filepath.Join(fixture.paths.ClaudeUserSkills, "beta")); !os.IsNotExist(err) {
		t.Fatalf("beta link exists or stat failed: %v", err)
	}
	manifest, err := state.New(fixture.paths).Load()
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	repository, _ := manifest.GetRepository("github.com", "owner/repo")
	if len(repository.InstalledSkills) != 1 || repository.InstalledSkills[0].Name != "alpha" {
		t.Fatalf("InstalledSkills = %#v, want alpha only", repository.InstalledSkills)
	}
}

func TestUpdateServiceRejectsDirtyDetachedAndAheadCheckouts(t *testing.T) {
	t.Run("ignored worktree content", func(t *testing.T) {
		fixture := newUpdateGitFixture(t)
		mustWriteFile(t, filepath.Join(fixture.checkoutPath, ".cache", "generated"), "data")
		_, err := NewUpdateService(fixture.paths, nil).PlanLocal(fixture.repository)
		if err == nil || !strings.Contains(err.Error(), "worktree changes") {
			t.Fatalf("PlanLocal() error = %v, want dirty checkout", err)
		}
	})

	t.Run("detached head", func(t *testing.T) {
		fixture := newUpdateGitFixture(t)
		fixture.runGit("-C", fixture.checkoutPath, "checkout", "--detach")
		_, err := NewUpdateService(fixture.paths, nil).PlanLocal(fixture.repository)
		if err == nil || !strings.Contains(err.Error(), "detached HEAD") {
			t.Fatalf("PlanLocal() error = %v, want detached conflict", err)
		}
	})

	t.Run("local-only commit", func(t *testing.T) {
		fixture := newUpdateGitFixture(t)
		mustWriteFile(t, filepath.Join(fixture.checkoutPath, "local.txt"), "local")
		fixture.runGit("-C", fixture.checkoutPath, "add", "local.txt")
		fixture.runGit("-C", fixture.checkoutPath, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "local")
		_, err := NewUpdateService(fixture.paths, nil).PlanLocal(fixture.repository)
		if err == nil || !strings.Contains(err.Error(), "cannot fast-forward") {
			t.Fatalf("PlanLocal() error = %v, want local commit conflict", err)
		}
	})

	t.Run("symlink checkout", func(t *testing.T) {
		fixture := newUpdateGitFixture(t)
		realCheckout := fixture.checkoutPath + "-moved"
		if err := os.Rename(fixture.checkoutPath, realCheckout); err != nil {
			t.Fatalf("move checkout: %v", err)
		}
		if err := os.Symlink(realCheckout, fixture.checkoutPath); err != nil {
			t.Fatalf("replace checkout with symlink: %v", err)
		}
		_, err := NewUpdateService(fixture.paths, nil).PlanLocal(fixture.repository)
		if err == nil || !strings.Contains(err.Error(), "is a symlink") {
			t.Fatalf("PlanLocal() error = %v, want symlink conflict", err)
		}
	})
}

type updateGitFixture struct {
	t               *testing.T
	paths           paths.Paths
	remotePath      string
	sourcePath      string
	checkoutPath    string
	repository      state.RepositoryEntry
	initialCommit   string
	activeSkillPath string
}

func newUpdateGitFixture(t *testing.T) updateGitFixture {
	t.Helper()
	root := t.TempDir()
	p := paths.ForHome(filepath.Join(root, "home"))
	remotePath := filepath.Join(root, "remote.git")
	sourcePath := filepath.Join(root, "source")
	runGitTest(t, "init", "--bare", remotePath)
	runGitTest(t, "init", "-b", "main", sourcePath)
	mustWriteFile(t, filepath.Join(sourcePath, ".gitignore"), ".cache/\n")
	mustWriteFile(t, filepath.Join(sourcePath, "skills", "alpha", "SKILL.md"), "# alpha v1\n")
	runGitTest(t, "-C", sourcePath, "add", ".")
	runGitTest(t, "-C", sourcePath, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "initial")
	runGitTest(t, "-C", sourcePath, "remote", "add", "origin", remotePath)
	runGitTest(t, "-C", sourcePath, "push", "-u", "origin", "main")
	runGitTest(t, "--git-dir", remotePath, "symbolic-ref", "HEAD", "refs/heads/main")
	initialCommit := strings.TrimSpace(runGitTest(t, "-C", sourcePath, "rev-parse", "HEAD"))

	identity, err := NormalizeGitURL("https://github.com/owner/repo")
	if err != nil {
		t.Fatalf("NormalizeGitURL() error = %v", err)
	}
	checkoutPath, err := CheckoutPath(p, identity)
	if err != nil {
		t.Fatalf("CheckoutPath() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(checkoutPath), 0o755); err != nil {
		t.Fatalf("create checkout parent: %v", err)
	}
	gitConfigPath := filepath.Join(root, "gitconfig")
	gitConfig := "[url \"" + remotePath + "\"]\n\tinsteadOf = " + identity.OriginalURL + "\n"
	mustWriteFile(t, gitConfigPath, gitConfig)
	t.Setenv("GIT_CONFIG_GLOBAL", gitConfigPath)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	runGitTest(t, "clone", remotePath, checkoutPath)
	runGitTest(t, "-C", checkoutPath, "remote", "set-url", "origin", identity.OriginalURL)
	activeSkillPath := filepath.Join(checkoutPath, "skills", "alpha")
	mustSymlink(t, activeSkillPath, filepath.Join(p.ClaudeUserSkills, "alpha"))
	repository := state.RepositoryEntry{
		OriginalURL:    identity.OriginalURL,
		CanonicalURL:   identity.CanonicalURL,
		Host:           identity.Host,
		RepoPath:       identity.RepoPath,
		CheckoutPath:   checkoutPath,
		Group:          identity.Group,
		LastSeenCommit: initialCommit,
		InstalledSkills: []state.InstalledSkillEntry{{
			Name:         "alpha",
			RelativePath: "skills/alpha",
			Tools:        []model.Tool{model.ToolClaude},
		}},
	}
	if err := state.New(p).Save(state.Manifest{Repositories: []state.RepositoryEntry{repository}}); err != nil {
		t.Fatalf("save state: %v", err)
	}
	return updateGitFixture{t: t, paths: p, remotePath: remotePath, sourcePath: sourcePath, checkoutPath: checkoutPath, repository: repository, initialCommit: initialCommit, activeSkillPath: activeSkillPath}
}

func (f updateGitFixture) commitRemoteChange(message string, writes map[string]string, removes []string) string {
	f.t.Helper()
	for relativePath, contents := range writes {
		mustWriteFile(f.t, filepath.Join(f.sourcePath, filepath.FromSlash(relativePath)), contents)
	}
	for _, relativePath := range removes {
		if err := os.Remove(filepath.Join(f.sourcePath, filepath.FromSlash(relativePath))); err != nil {
			f.t.Fatalf("remove %s: %v", relativePath, err)
		}
	}
	f.runGit("-C", f.sourcePath, "add", "-A")
	f.runGit("-C", f.sourcePath, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", message)
	f.runGit("-C", f.sourcePath, "push", "origin", "main")
	return f.gitSource("rev-parse", "HEAD")
}

func (f updateGitFixture) gitCheckout(args ...string) string {
	f.t.Helper()
	return strings.TrimSpace(runGitTest(f.t, append([]string{"-C", f.checkoutPath}, args...)...))
}

func (f updateGitFixture) gitSource(args ...string) string {
	f.t.Helper()
	return strings.TrimSpace(runGitTest(f.t, append([]string{"-C", f.sourcePath}, args...)...))
}

func (f updateGitFixture) runGit(args ...string) string {
	f.t.Helper()
	return runGitTest(f.t, args...)
}

func runGitTest(t *testing.T, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
	return string(output)
}
