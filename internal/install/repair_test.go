package install

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/dees91/agent-skill-manager/internal/model"
	"github.com/dees91/agent-skill-manager/internal/state"
)

func TestParseWorktreeEntriesClassifiesOrdersAndKeepsExactPaths(t *testing.T) {
	output := strings.Join([]string{
		"? zeta.txt",
		"! .cache/",
		"1 .M N... 100644 100644 100644 aaa bbb skills/alpha/SKILL.md",
		"2 R. N... 100644 100644 100644 ccc ddd R100 renamed/new.txt",
		"renamed/old.txt",
		"1 A. N... 000000 100644 100644 eee fff staged.txt",
		"?  leading space.txt",
		"1 .M N... 100644 100644 100644 ggg hhh dir/two words.txt",
	}, "\x00")

	entries := parseWorktreeEntries(output)

	want := []WorktreeEntry{
		{RelPath: " leading space.txt", Class: WorktreeEntryUntracked, Status: "?"},
		{RelPath: ".cache", Class: WorktreeEntryIgnored, Status: "!", IsDir: true},
		{RelPath: "dir/two words.txt", Class: WorktreeEntryTracked, Status: ".M"},
		{RelPath: "renamed/new.txt", Class: WorktreeEntryTracked, Status: "R."},
		{RelPath: "renamed/old.txt", Class: WorktreeEntryTracked, Status: "R."},
		{RelPath: "skills/alpha/SKILL.md", Class: WorktreeEntryTracked, Status: ".M"},
		{RelPath: "staged.txt", Class: WorktreeEntryTracked, Status: "A."},
		{RelPath: "zeta.txt", Class: WorktreeEntryUntracked, Status: "?"},
	}
	if len(entries) != len(want) {
		t.Fatalf("parseWorktreeEntries() = %#v, want %d entries", entries, len(want))
	}
	for i, entry := range entries {
		if entry != want[i] {
			t.Fatalf("entry %d = %#v, want %#v", i, entry, want[i])
		}
	}
}

func TestRepairPreservesWhitespaceInFilenames(t *testing.T) {
	fixture := newUpdateGitFixture(t)
	strayPath := filepath.Join(fixture.checkoutPath, " stray.txt")
	mustWriteFile(t, strayPath, "lead")

	plan, err := NewRepairService(fixture.paths, nil).Plan(fixture.repository)
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if len(plan.Entries) != 1 || plan.Entries[0].RelPath != " stray.txt" {
		t.Fatalf("Plan() entries = %#v, want the exact leading-space path", plan.Entries)
	}

	result, err := NewRepairService(fixture.paths, nil).Apply(fixture.repository)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if !result.Clean || len(result.StagedEntries) != 1 {
		t.Fatalf("Apply() = %#v, want the file staged", result)
	}
	if _, err := os.Lstat(strayPath); !os.IsNotExist(err) {
		t.Fatalf("%q survived repair: %v", strayPath, err)
	}
	assertTrashEmpty(t, fixture)
}

func TestCheckoutConflictKeepsMessageAndClassifiesKind(t *testing.T) {
	fixture := newUpdateGitFixture(t)
	mustWriteFile(t, filepath.Join(fixture.checkoutPath, "stray.txt"), "data")

	_, err := NewUpdateService(fixture.paths, nil).PlanLocal(fixture.repository)

	conflict, ok := AsCheckoutConflict(err)
	if !ok {
		t.Fatalf("AsCheckoutConflict() = false for %v", err)
	}
	if conflict.Kind != CheckoutConflictDirtyWorktree || !conflict.Repairable() {
		t.Fatalf("conflict = %#v, want repairable dirty worktree", conflict)
	}
	if !strings.Contains(err.Error(), "has tracked, untracked, or ignored worktree changes") {
		t.Fatalf("Error() = %q, want unchanged message", err.Error())
	}
	if len(conflict.Entries) != 1 || conflict.Entries[0].RelPath != "stray.txt" || conflict.Entries[0].Class != WorktreeEntryUntracked {
		t.Fatalf("entries = %#v, want the stray untracked file", conflict.Entries)
	}
	if !strings.Contains(conflict.Cause(), "1 untracked") || strings.Contains(conflict.Cause(), fixture.checkoutPath) {
		t.Fatalf("Cause() = %q, want a path-free summary", conflict.Cause())
	}
}

func TestRepairServiceClearsEveryDirtyWorktreeShape(t *testing.T) {
	tests := []struct {
		name       string
		dirty      func(t *testing.T, fixture updateGitFixture)
		wantStaged []string
	}{
		{
			name: "untracked file",
			dirty: func(t *testing.T, fixture updateGitFixture) {
				mustWriteFile(t, filepath.Join(fixture.checkoutPath, "profilers", "bin", "trace_processor"), "binary")
			},
			wantStaged: []string{"profilers/bin/trace_processor"},
		},
		{
			name: "ignored directory",
			dirty: func(t *testing.T, fixture updateGitFixture) {
				mustWriteFile(t, filepath.Join(fixture.checkoutPath, ".cache", "generated"), "data")
			},
			wantStaged: []string{".cache/generated"},
		},
		{
			name: "modified tracked file",
			dirty: func(t *testing.T, fixture updateGitFixture) {
				mustWriteFile(t, filepath.Join(fixture.checkoutPath, "skills", "alpha", "SKILL.md"), "# local edit\n")
			},
			wantStaged: []string{"skills/alpha/SKILL.md"},
		},
		{
			name: "deleted tracked file",
			dirty: func(t *testing.T, fixture updateGitFixture) {
				if err := os.Remove(filepath.Join(fixture.checkoutPath, ".gitignore")); err != nil {
					t.Fatalf("remove tracked file: %v", err)
				}
			},
			wantStaged: []string{},
		},
		{
			name: "staged addition",
			dirty: func(t *testing.T, fixture updateGitFixture) {
				mustWriteFile(t, filepath.Join(fixture.checkoutPath, "added.txt"), "new")
				fixture.gitCheckout("add", "added.txt")
			},
			wantStaged: []string{"added.txt"},
		},
		{
			name: "staged rename",
			dirty: func(t *testing.T, fixture updateGitFixture) {
				fixture.gitCheckout("mv", ".gitignore", "ignore-rules")
			},
			wantStaged: []string{"ignore-rules"},
		},
		{
			name: "mixed changes",
			dirty: func(t *testing.T, fixture updateGitFixture) {
				mustWriteFile(t, filepath.Join(fixture.checkoutPath, "stray.txt"), "x")
				mustWriteFile(t, filepath.Join(fixture.checkoutPath, ".cache", "generated"), "y")
				mustWriteFile(t, filepath.Join(fixture.checkoutPath, "skills", "alpha", "SKILL.md"), "# edited\n")
			},
			wantStaged: []string{".cache/generated", "skills/alpha/SKILL.md", "stray.txt"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newUpdateGitFixture(t)
			test.dirty(t, fixture)

			plan, err := NewRepairService(fixture.paths, nil).Plan(fixture.repository)
			if err != nil {
				t.Fatalf("Plan() error = %v", err)
			}
			if plan.Clean || len(plan.Entries) == 0 {
				t.Fatalf("Plan() = %#v, want a dirty plan", plan)
			}
			if entriesExist := worktreePathsExist(fixture.checkoutPath, test.wantStaged); !entriesExist {
				t.Fatalf("fixture did not create %v", test.wantStaged)
			}

			result, err := NewRepairService(fixture.paths, nil).Apply(fixture.repository)
			if err != nil {
				t.Fatalf("Apply() error = %v", err)
			}
			if !result.Clean || result.CleanupPending != "" {
				t.Fatalf("Apply() = %#v, want a clean repair", result)
			}
			if got := stagedRelPaths(result); !equalStrings(got, test.wantStaged) {
				t.Fatalf("staged = %v, want %v", got, test.wantStaged)
			}
			for _, entry := range result.StagedEntries {
				_, err := os.Lstat(filepath.Join(fixture.checkoutPath, filepath.FromSlash(entry.RelPath)))
				if entry.Class == WorktreeEntryTracked {
					continue
				}
				if !os.IsNotExist(err) {
					t.Fatalf("%s still present after repair: %v", entry.RelPath, err)
				}
			}
			assertTrashEmpty(t, fixture)
			if _, err := NewUpdateService(fixture.paths, nil).PlanLocal(fixture.repository); err != nil {
				t.Fatalf("PlanLocal() after repair error = %v", err)
			}
			if contents := readFileTest(t, filepath.Join(fixture.checkoutPath, "skills", "alpha", "SKILL.md")); contents != "# alpha v1\n" {
				t.Fatalf("tracked content = %q, want the HEAD content restored", contents)
			}
			if contents := readFileTest(t, filepath.Join(fixture.checkoutPath, ".gitignore")); contents != ".cache/\n" {
				t.Fatalf(".gitignore = %q, want the HEAD content restored", contents)
			}
		})
	}
}

func TestRepairServiceIsIdempotentOnCleanCheckouts(t *testing.T) {
	fixture := newUpdateGitFixture(t)

	plan, err := NewRepairService(fixture.paths, nil).Plan(fixture.repository)
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if !plan.Clean || len(plan.Entries) != 0 {
		t.Fatalf("Plan() = %#v, want a clean plan", plan)
	}

	result, err := NewRepairService(fixture.paths, nil).Apply(fixture.repository)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if !result.Clean || len(result.StagedEntries) != 0 {
		t.Fatalf("Apply() = %#v, want a no-op repair", result)
	}
	assertTrashEmpty(t, fixture)
}

func TestRepairServiceRefusesNonRepairableBlockers(t *testing.T) {
	tests := []struct {
		name   string
		break_ func(t *testing.T, fixture updateGitFixture)
		want   string
	}{
		{
			name:   "detached head",
			break_: func(t *testing.T, fixture updateGitFixture) { fixture.gitCheckout("checkout", "--detach") },
			want:   "detached HEAD",
		},
		{
			name: "local-only commit",
			break_: func(t *testing.T, fixture updateGitFixture) {
				mustWriteFile(t, filepath.Join(fixture.checkoutPath, "local.txt"), "local")
				fixture.gitCheckout("add", "local.txt")
				fixture.gitCheckout("-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "local")
			},
			want: "cannot fast-forward",
		},
		{
			name: "missing installed skill file",
			break_: func(t *testing.T, fixture updateGitFixture) {
				mustWriteFile(t, filepath.Join(fixture.checkoutPath, "stray.txt"), "x")
				if err := os.Remove(filepath.Join(fixture.checkoutPath, "skills", "alpha", "SKILL.md")); err != nil {
					t.Fatalf("remove installed skill file: %v", err)
				}
			},
			want: "managed reference conflicts",
		},
		{
			name: "managed reference drift",
			break_: func(t *testing.T, fixture updateGitFixture) {
				mustWriteFile(t, filepath.Join(fixture.checkoutPath, "stray.txt"), "x")
				if err := os.Remove(filepath.Join(fixture.paths.ClaudeUserSkills, "alpha")); err != nil {
					t.Fatalf("remove managed symlink: %v", err)
				}
			},
			want: "managed reference conflicts",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newUpdateGitFixture(t)
			test.break_(t, fixture)

			_, planErr := NewRepairService(fixture.paths, nil).Plan(fixture.repository)
			if planErr == nil || !strings.Contains(planErr.Error(), test.want) {
				t.Fatalf("Plan() error = %v, want %q", planErr, test.want)
			}
			_, applyErr := NewRepairService(fixture.paths, nil).Apply(fixture.repository)
			if applyErr == nil || !strings.Contains(applyErr.Error(), test.want) {
				t.Fatalf("Apply() error = %v, want %q", applyErr, test.want)
			}
			assertTrashEmpty(t, fixture)
		})
	}
}

func TestRepairServiceRollsBackStagedPathsOnFailure(t *testing.T) {
	fixture := newUpdateGitFixture(t)
	mustWriteFile(t, filepath.Join(fixture.checkoutPath, "first.txt"), "one")
	mustWriteFile(t, filepath.Join(fixture.checkoutPath, "second.txt"), "two")

	service := NewRepairService(fixture.paths, nil)
	moved := 0
	realRename := service.rename
	service.rename = func(oldPath, newPath string) error {
		if strings.HasSuffix(oldPath, "second.txt") && moved > 0 {
			return errors.New("boom")
		}
		moved++
		return realRename(oldPath, newPath)
	}

	result, err := service.Apply(fixture.repository)
	if err == nil || !strings.Contains(err.Error(), "stage worktree path second.txt") {
		t.Fatalf("Apply() error = %v, want a staging failure", err)
	}
	if len(result.RolledBack) != 1 || result.CleanupPending != "" {
		t.Fatalf("result = %#v, want one rolled back move and no residue", result)
	}
	if contents := readFileTest(t, filepath.Join(fixture.checkoutPath, "first.txt")); contents != "one" {
		t.Fatalf("first.txt = %q, want the original content restored", contents)
	}
	assertTrashEmpty(t, fixture)
}

func TestRepairServiceReportsCleanupResidue(t *testing.T) {
	fixture := newUpdateGitFixture(t)
	mustWriteFile(t, filepath.Join(fixture.checkoutPath, "stray.txt"), "x")

	service := NewRepairService(fixture.paths, nil)
	service.removeAll = func(string) error { return errors.New("locked") }

	result, err := service.Apply(fixture.repository)
	if err == nil || !strings.Contains(err.Error(), "cleanup remains at") {
		t.Fatalf("Apply() error = %v, want retained cleanup", err)
	}
	if result.CleanupPending == "" || !pathInside(fixture.paths.TrashDir, result.CleanupPending) {
		t.Fatalf("CleanupPending = %q, want a path inside the trash directory", result.CleanupPending)
	}
	if _, err := os.Lstat(filepath.Join(result.CleanupPending, "worktree", "stray.txt")); err != nil {
		t.Fatalf("staged copy missing: %v", err)
	}
}

func TestRepairServiceNeverIssuesDestructiveGitCommands(t *testing.T) {
	fixture := newUpdateGitFixture(t)
	mustWriteFile(t, filepath.Join(fixture.checkoutPath, "stray.txt"), "x")
	mustWriteFile(t, filepath.Join(fixture.checkoutPath, "skills", "alpha", "SKILL.md"), "# edited\n")
	mustWriteFile(t, filepath.Join(fixture.checkoutPath, "added.txt"), "new")
	fixture.gitCheckout("add", "added.txt")

	runner := &recordingGitRunner{}
	if _, err := NewRepairService(fixture.paths, runner).Apply(fixture.repository); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	assertNoPull(t, runner.calls)
	for _, call := range runner.calls {
		for _, arg := range call {
			switch arg {
			case "clean", "--hard", "-f", "--force", "rm", "push":
				t.Fatalf("git calls include %q: %#v", arg, runner.calls)
			}
		}
	}
}

func TestValidateWorktreeEntryRejectsUnsafePaths(t *testing.T) {
	tests := []struct {
		name    string
		relPath string
		want    string
	}{
		{name: "empty", relPath: "", want: "empty worktree path"},
		{name: "absolute", relPath: "/etc/passwd", want: "absolute worktree path"},
		{name: "parent escape", relPath: "../outside.txt", want: "unsafe worktree path"},
		{name: "nested parent escape", relPath: "skills/../../outside.txt", want: "unsafe worktree path"},
		{name: "git metadata", relPath: ".git/config", want: "inside git metadata"},
		{name: "backslash", relPath: "skills\\alpha", want: "unsupported worktree path"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateWorktreeEntry("/checkout/root", WorktreeEntry{RelPath: test.relPath, Class: WorktreeEntryUntracked})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("validateWorktreeEntry() error = %v, want %q", err, test.want)
			}
		})
	}
}

type recordingGitRunner struct {
	calls [][]string
}

func (r *recordingGitRunner) RunGit(args ...string) (string, error) {
	r.calls = append(r.calls, append([]string(nil), args...))
	return ExecGitRunner{}.RunGit(args...)
}

func stagedRelPaths(result RepairResult) []string {
	paths := []string{}
	for _, entry := range result.StagedEntries {
		paths = append(paths, entry.RelPath)
	}
	sort.Strings(paths)
	return paths
}

func worktreePathsExist(checkoutPath string, relativePaths []string) bool {
	for _, relative := range relativePaths {
		if _, err := os.Lstat(filepath.Join(checkoutPath, filepath.FromSlash(relative))); err != nil {
			return false
		}
	}
	return true
}

func equalStrings(got, want []string) bool {
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

func assertTrashEmpty(t *testing.T, fixture updateGitFixture) {
	t.Helper()
	entries, err := os.ReadDir(fixture.paths.TrashDir)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		t.Fatalf("read trash: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("trash entries = %#v, want empty", entries)
	}
}

func readFileTest(t *testing.T, filePath string) string {
	t.Helper()
	contents, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read %s: %v", filePath, err)
	}
	return string(contents)
}

func TestRepairServiceRefusesToStageAnInstalledSkillDirectory(t *testing.T) {
	fixture := newUpdateGitFixture(t)
	mustWriteFile(t, filepath.Join(fixture.checkoutPath, "skills", "beta", "SKILL.md"), "# beta\n")
	mustSymlink(t, filepath.Join(fixture.checkoutPath, "skills", "beta"), filepath.Join(fixture.paths.ClaudeUserSkills, "beta"))
	repository := fixture.repository
	repository.InstalledSkills = append(repository.InstalledSkills, state.InstalledSkillEntry{
		Name:         "beta",
		RelativePath: "skills/beta",
		Tools:        []model.Tool{model.ToolClaude},
	})
	if err := state.New(fixture.paths).Save(state.Manifest{Repositories: []state.RepositoryEntry{repository}}); err != nil {
		t.Fatalf("save state: %v", err)
	}

	_, err := NewRepairService(fixture.paths, nil).Apply(repository)

	if err == nil || !strings.Contains(err.Error(), `holds installed skill "beta"`) {
		t.Fatalf("Apply() error = %v, want a refusal naming the installed skill", err)
	}
	if _, statErr := os.Lstat(filepath.Join(fixture.checkoutPath, "skills", "beta", "SKILL.md")); statErr != nil {
		t.Fatalf("installed skill directory was disturbed: %v", statErr)
	}
	assertTrashEmpty(t, fixture)
}

func TestRepairRefusesStagedInstalledSkillFileMissingFromHead(t *testing.T) {
	fixture := newUpdateGitFixture(t)
	mustWriteFile(t, filepath.Join(fixture.checkoutPath, "skills", "beta", "SKILL.md"), "# beta\n")
	mustSymlink(t, filepath.Join(fixture.checkoutPath, "skills", "beta"), filepath.Join(fixture.paths.ClaudeUserSkills, "beta"))
	fixture.gitCheckout("add", "skills/beta/SKILL.md")
	repository := fixture.repository
	repository.InstalledSkills = append(repository.InstalledSkills, state.InstalledSkillEntry{
		Name:         "beta",
		RelativePath: "skills/beta",
		Tools:        []model.Tool{model.ToolClaude},
	})
	if err := state.New(fixture.paths).Save(state.Manifest{Repositories: []state.RepositoryEntry{repository}}); err != nil {
		t.Fatalf("save state: %v", err)
	}

	_, err := NewRepairService(fixture.paths, nil).Apply(repository)

	if err == nil || !strings.Contains(err.Error(), "is missing from HEAD") {
		t.Fatalf("Apply() error = %v, want a refusal naming the missing HEAD blob", err)
	}
	if _, statErr := os.Lstat(filepath.Join(fixture.checkoutPath, "skills", "beta", "SKILL.md")); statErr != nil {
		t.Fatalf("installed skill file was disturbed: %v", statErr)
	}
	if target, readErr := os.Readlink(filepath.Join(fixture.paths.ClaudeUserSkills, "beta")); readErr != nil {
		t.Fatalf("managed symlink was disturbed: %v (target %q)", readErr, target)
	}
	assertTrashEmpty(t, fixture)
}

func TestRepairRollbackRestoresTheGitIndex(t *testing.T) {
	fixture := newUpdateGitFixture(t)
	mustWriteFile(t, filepath.Join(fixture.checkoutPath, ".gitignore"), "staged-version/\n")
	fixture.gitCheckout("add", ".gitignore")
	mustWriteFile(t, filepath.Join(fixture.checkoutPath, ".gitignore"), "worktree-version/\n")
	stagedBlob := fixture.gitCheckout("rev-parse", ":.gitignore")

	service := NewRepairService(fixture.paths, nil)
	realRunner := service.runner
	service.runner = &failingVerifyRunner{inner: realRunner, checkoutPath: fixture.checkoutPath}

	result, err := service.Apply(fixture.repository)

	if err == nil || !strings.Contains(err.Error(), "repair did not clear the checkout") {
		t.Fatalf("Apply() error = %v, want the injected verification failure", err)
	}
	if restored := fixture.gitCheckout("rev-parse", ":.gitignore"); restored != stagedBlob {
		t.Fatalf("index blob = %q, want the original staged blob %q", restored, stagedBlob)
	}
	if contents := readFileTest(t, filepath.Join(fixture.checkoutPath, ".gitignore")); contents != "worktree-version/\n" {
		t.Fatalf(".gitignore = %q, want the original worktree content", contents)
	}
	if result.CleanupPending != "" {
		t.Fatalf("CleanupPending = %q, want a complete rollback", result.CleanupPending)
	}
	assertTrashEmpty(t, fixture)
}

func TestRepairRefusesDirtyCheckoutsWithNonRepairableRefBlockers(t *testing.T) {
	tests := []struct {
		name   string
		break_ func(t *testing.T, fixture updateGitFixture)
		want   string
	}{
		{
			name: "local-only commit",
			break_: func(t *testing.T, fixture updateGitFixture) {
				mustWriteFile(t, filepath.Join(fixture.checkoutPath, "committed.txt"), "local")
				fixture.gitCheckout("add", "committed.txt")
				fixture.gitCheckout("-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "local")
			},
			want: "cannot fast-forward",
		},
		{
			name:   "detached head",
			break_: func(t *testing.T, fixture updateGitFixture) { fixture.gitCheckout("checkout", "--detach") },
			want:   "detached HEAD",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newUpdateGitFixture(t)
			test.break_(t, fixture)
			strayPath := filepath.Join(fixture.checkoutPath, "stray.txt")
			mustWriteFile(t, strayPath, "block")

			_, planErr := NewRepairService(fixture.paths, nil).Plan(fixture.repository)
			if planErr == nil || !strings.Contains(planErr.Error(), test.want) {
				t.Fatalf("Plan() error = %v, want %q", planErr, test.want)
			}
			result, applyErr := NewRepairService(fixture.paths, nil).Apply(fixture.repository)
			if applyErr == nil || !strings.Contains(applyErr.Error(), test.want) {
				t.Fatalf("Apply() error = %v, want %q", applyErr, test.want)
			}
			if result.Clean || len(result.StagedEntries) != 0 {
				t.Fatalf("Apply() = %#v, want no mutation", result)
			}
			if _, err := os.Lstat(strayPath); err != nil {
				t.Fatalf("refused repair still moved %s: %v", strayPath, err)
			}
			assertTrashEmpty(t, fixture)
		})
	}
}

// failingVerifyRunner fails the second worktree status check, which is the
// verification repair runs after restoring tracked paths.
type failingVerifyRunner struct {
	inner        GitRunner
	checkoutPath string
	statusCalls  int
}

func (r *failingVerifyRunner) RunGit(args ...string) (string, error) {
	joined := strings.Join(args, " ")
	if strings.Contains(joined, "status --porcelain --untracked-files=all --ignored") {
		r.statusCalls++
		if r.statusCalls > 1 {
			return "M injected", nil
		}
	}
	return r.inner.RunGit(args...)
}
