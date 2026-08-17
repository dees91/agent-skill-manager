package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/dees91/agent-skill-manager/internal/install"
	"github.com/dees91/agent-skill-manager/internal/model"
	"github.com/dees91/agent-skill-manager/internal/paths"
	"github.com/dees91/agent-skill-manager/internal/state"
)

func TestRunDefaultsToTUI(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	var stdout, stderr strings.Builder
	called := false

	code := RunWithPathsAndTUI(nil, &stdout, &stderr, p, func(got paths.Paths) error {
		called = true
		if got.Home != p.Home {
			t.Fatalf("TUI path home = %q, want %q", got.Home, p.Home)
		}
		return nil
	})

	if code != 0 {
		t.Fatalf("Run() code = %d, want 0", code)
	}
	if !called {
		t.Fatal("TUI runner was not called")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunTUICommandUsesInjectedRunner(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	var stdout, stderr strings.Builder
	called := false

	code := RunWithPathsAndTUI([]string{"tui"}, &stdout, &stderr, p, func(paths.Paths) error {
		called = true
		return nil
	})

	if code != 0 {
		t.Fatalf("RunWithPathsAndTUI(tui) code = %d, want 0", code)
	}
	if !called {
		t.Fatal("TUI runner was not called")
	}
	if stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("stdout=%q stderr=%q, want both empty", stdout.String(), stderr.String())
	}
}

func TestRunTUIErrorFails(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	var stdout, stderr strings.Builder

	code := RunWithPathsAndTUI([]string{"tui"}, &stdout, &stderr, p, func(paths.Paths) error {
		return errors.New("tui failed")
	})

	if code == 0 {
		t.Fatal("RunWithPathsAndTUI(tui error) code = 0, want non-zero")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "tui failed") {
		t.Fatalf("stderr = %q, want tui error", stderr.String())
	}
}

func TestRunHelpListsCommands(t *testing.T) {
	var stdout, stderr strings.Builder

	code := Run([]string{"help"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("Run(help) code = %d, want 0", code)
	}
	for _, command := range []string{"tui", "version", "list", "status", "groups", "repos", "install", "update", "uninstall", "enable", "disable", "advisor"} {
		if !strings.Contains(stdout.String(), command) {
			t.Fatalf("stdout = %q, want command %q", stdout.String(), command)
		}
	}
	for _, want := range []string{"--dry-run", "--skill", "both"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout = %q, want %q", stdout.String(), want)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunVersionEntryPoints(t *testing.T) {
	originalVersion := Version
	Version = "v0.4.0"
	t.Cleanup(func() { Version = originalVersion })

	for _, args := range [][]string{{"version"}, {"--version"}} {
		var stdout, stderr strings.Builder
		code := Run(args, &stdout, &stderr)

		if code != 0 {
			t.Fatalf("Run(%q) code = %d, want 0", args, code)
		}
		if stdout.String() != "skill-manager 0.4.0\n" {
			t.Fatalf("Run(%q) stdout = %q", args, stdout.String())
		}
		if stderr.Len() != 0 {
			t.Fatalf("Run(%q) stderr = %q, want empty", args, stderr.String())
		}
	}
}

func TestRunVersionRejectsArguments(t *testing.T) {
	for _, args := range [][]string{{"version", "extra"}, {"--version", "extra"}} {
		var stdout, stderr strings.Builder
		code := Run(args, &stdout, &stderr)

		if code == 0 {
			t.Fatalf("Run(%q) code = 0, want non-zero", args)
		}
		if stdout.Len() != 0 {
			t.Fatalf("Run(%q) stdout = %q, want empty", args, stdout.String())
		}
		if !strings.Contains(stderr.String(), "does not accept arguments") {
			t.Fatalf("Run(%q) stderr = %q", args, stderr.String())
		}
	}
}

func TestResolveVersion(t *testing.T) {
	tests := []struct {
		name          string
		linkerVersion string
		buildInfo     *debug.BuildInfo
		buildInfoOK   bool
		want          string
	}{
		{name: "development", linkerVersion: "dev", want: "dev"},
		{name: "linker", linkerVersion: "v0.4.0", buildInfo: &debug.BuildInfo{Main: debug.Module{Version: "v9.9.9"}}, buildInfoOK: true, want: "0.4.0"},
		{name: "tagged module", linkerVersion: "dev", buildInfo: &debug.BuildInfo{Main: debug.Module{Version: "v0.5.0"}}, buildInfoOK: true, want: "0.5.0"},
		{name: "devel module", linkerVersion: "dev", buildInfo: &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, buildInfoOK: true, want: "dev"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := resolveVersion(test.linkerVersion, test.buildInfo, test.buildInfoOK); got != test.want {
				t.Fatalf("resolveVersion() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestRunUpdateEmptyManifestIsNoop(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	var stdout, stderr strings.Builder

	code := RunWithPaths([]string{"update"}, &stdout, &stderr, p)

	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("RunWithPaths(update) code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "No managed repositories recorded") {
		t.Fatalf("stdout = %q, want empty message", stdout.String())
	}
	assertMissing(t, p.StateFile)
}

func TestRunInstallLocalPathAndListClassification(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	source := filepath.Join(p.Home, "workspace", "sample-pack")
	skillPath := filepath.Join(source, "skills", "alpha")
	mkdirSkill(t, skillPath)
	var stdout, stderr strings.Builder

	code := RunWithPaths([]string{"install", source}, &stdout, &stderr, p)

	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("local install code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	for _, want := range []string{"install local source:", "group: sample-pack", "created 2 symlink(s)", "source remains in place:"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout = %q, want %q", stdout.String(), want)
		}
	}
	for _, tool := range model.Tools() {
		dir, _ := p.UserSkillsDirFor(tool)
		linkPath := filepath.Join(dir, "alpha")
		if _, err := os.Readlink(linkPath); err != nil {
			t.Fatalf("%s local link missing: %v", tool, err)
		}
	}
	manifest := loadState(t, p)
	if manifest.Version != 2 || len(manifest.LocalSources) != 1 || manifest.LocalSources[0].Group != model.GroupLabel("sample-pack") {
		t.Fatalf("manifest = %#v, want one sample-pack local source", manifest)
	}
	var listOut, listErr strings.Builder
	if code := RunWithPaths([]string{"list"}, &listOut, &listErr, p); code != 0 || listErr.Len() != 0 {
		t.Fatalf("list code=%d stderr=%q", code, listErr.String())
	}
	if !strings.Contains(listOut.String(), "alpha") || !strings.Contains(listOut.String(), "local path") {
		t.Fatalf("list output = %q, want local path alpha", listOut.String())
	}
	assertExists(t, filepath.Join(skillPath, "SKILL.md"))
}

func TestRunInstallLocalDryRunDoesNotMutate(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	source := filepath.Join(p.Home, "workspace", "local-pack")
	mkdirSkill(t, filepath.Join(source, "skills", "alpha"))
	var stdout, stderr strings.Builder

	code := RunWithPaths([]string{"install", source, "--tool", "codex", "--dry-run"}, &stdout, &stderr, p)

	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("local dry-run code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	for _, want := range []string{"dry-run: install local source", "would link codex/alpha", "would preserve source"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout = %q, want %q", stdout.String(), want)
		}
	}
	assertMissing(t, filepath.Join(p.CodexUserSkills, "alpha"))
	assertMissing(t, p.StateFile)
	assertExists(t, filepath.Join(source, "skills", "alpha", "SKILL.md"))
}

func TestRunUninstallLocalPathPreservesSource(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	source := filepath.Join(p.Home, "workspace", "local-pack")
	skillFile := filepath.Join(source, "skills", "alpha", "SKILL.md")
	mkdirSkill(t, filepath.Dir(skillFile))
	var installOut, installErr strings.Builder
	if code := RunWithPaths([]string{"install", source, "--tool", "claude"}, &installOut, &installErr, p); code != 0 {
		t.Fatalf("install code=%d stdout=%q stderr=%q", code, installOut.String(), installErr.String())
	}
	var stdout, stderr strings.Builder

	code := RunWithPaths([]string{"uninstall", source}, &stdout, &stderr, p)

	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("local uninstall code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	for _, want := range []string{"removed 1 active symlink", "preserved source:", "uninstalled local source local-pack"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout = %q, want %q", stdout.String(), want)
		}
	}
	assertMissing(t, filepath.Join(p.ClaudeUserSkills, "alpha"))
	assertExists(t, skillFile)
	if len(loadState(t, p).LocalSources) != 0 {
		t.Fatal("local source state remains after uninstall")
	}
}

func TestRunUninstallLocalDryRunDoesNotMutate(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	source := filepath.Join(p.Home, "workspace", "local-pack")
	mkdirSkill(t, filepath.Join(source, "skills", "alpha"))
	var installOut, installErr strings.Builder
	if code := RunWithPaths([]string{"install", source, "--tool", "claude"}, &installOut, &installErr, p); code != 0 {
		t.Fatalf("install code=%d stderr=%q", code, installErr.String())
	}
	stateBefore := readFile(t, p.StateFile)
	var stdout, stderr strings.Builder

	code := RunWithPaths([]string{"uninstall", source, "--dry-run"}, &stdout, &stderr, p)

	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("dry-run code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	for _, want := range []string{"dry-run: uninstall local source", "would remove on claude/alpha", "would preserve source"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout = %q, want %q", stdout.String(), want)
		}
	}
	assertExists(t, filepath.Join(p.ClaudeUserSkills, "alpha"))
	assertExists(t, filepath.Join(source, "skills", "alpha", "SKILL.md"))
	if stateAfter := readFile(t, p.StateFile); string(stateAfter) != string(stateBefore) {
		t.Fatal("local uninstall dry-run changed state")
	}
}

func TestRunUpdateLocalPathReportsLiveSource(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	source := filepath.Join(p.Home, "workspace", "local-pack")
	mkdirSkill(t, filepath.Join(source, "skills", "alpha"))
	var installOut, installErr strings.Builder
	if code := RunWithPaths([]string{"install", source, "--tool", "codex"}, &installOut, &installErr, p); code != 0 {
		t.Fatalf("install code=%d stderr=%q", code, installErr.String())
	}
	stateBefore := readFile(t, p.StateFile)
	var stdout, stderr strings.Builder

	code := RunWithPaths([]string{"update", source}, &stdout, &stderr, p)

	if code != 0 || stderr.Len() != 0 || !strings.Contains(stdout.String(), "is link-in-place and does not require update") {
		t.Fatalf("local update code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if stateAfter := readFile(t, p.StateFile); string(stateAfter) != string(stateBefore) {
		t.Fatal("local update changed state")
	}
}

func TestRunUpdateDryRunDoesNotFetchOrMutate(t *testing.T) {
	const gitURL = "https://github.com/owner/update-dry-run"
	source := createSourceRepo(t, "alpha")
	withGitInsteadOf(t, gitURL, source)
	p := paths.ForHome(t.TempDir())
	var installOut, installErr strings.Builder
	if code := RunWithPaths([]string{"install", gitURL, "--tool", "claude"}, &installOut, &installErr, p); code != 0 {
		t.Fatalf("install code=%d stdout=%q stderr=%q", code, installOut.String(), installErr.String())
	}
	checkout := checkoutPathForTest(t, p, gitURL)
	oldHead := gitOutputForTest(t, checkout, "rev-parse", "HEAD")
	upstream := gitOutputForTest(t, checkout, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")
	if err := os.WriteFile(filepath.Join(source, "skills", "alpha", "SKILL.md"), []byte("# updated\n"), 0o644); err != nil {
		t.Fatalf("update source skill: %v", err)
	}
	runGitForTest(t, source, "add", ".")
	runGitForTest(t, source, "commit", "-m", "Update alpha")
	stateBefore, err := os.ReadFile(p.StateFile)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}

	var stdout, stderr strings.Builder
	code := RunWithPaths([]string{"update", gitURL, "--dry-run"}, &stdout, &stderr, p)

	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("update dry-run code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	for _, want := range []string{"dry-run: update owner/update-dry-run", "would fetch origin", "remote target unavailable without fetch"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout = %q, want %q", stdout.String(), want)
		}
	}
	if got := gitOutputForTest(t, checkout, "rev-parse", "HEAD"); got != oldHead {
		t.Fatalf("HEAD = %q, want unchanged %q", got, oldHead)
	}
	if got := gitOutputForTest(t, checkout, "rev-parse", upstream); got != oldHead {
		t.Fatalf("%s = %q, want unchanged %q", upstream, got, oldHead)
	}
	stateAfter, err := os.ReadFile(p.StateFile)
	if err != nil {
		t.Fatalf("read state after: %v", err)
	}
	if string(stateAfter) != string(stateBefore) {
		t.Fatal("update dry-run changed state")
	}
}

func TestRunUpdateFastForwardsSelectedRepository(t *testing.T) {
	const gitURL = "https://github.com/owner/update-apply"
	source := createSourceRepo(t, "alpha")
	withGitInsteadOf(t, gitURL, source)
	p := paths.ForHome(t.TempDir())
	var installOut, installErr strings.Builder
	if code := RunWithPaths([]string{"install", gitURL, "--tool", "claude"}, &installOut, &installErr, p); code != 0 {
		t.Fatalf("install code=%d stdout=%q stderr=%q", code, installOut.String(), installErr.String())
	}
	if err := os.WriteFile(filepath.Join(source, "skills", "alpha", "SKILL.md"), []byte("# updated\n"), 0o644); err != nil {
		t.Fatalf("update source skill: %v", err)
	}
	runGitForTest(t, source, "add", ".")
	runGitForTest(t, source, "commit", "-m", "Update alpha")
	wantCommit := gitOutputForTest(t, source, "rev-parse", "HEAD")

	var stdout, stderr strings.Builder
	code := RunWithPaths([]string{"update", gitURL}, &stdout, &stderr, p)

	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("update code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "updated owner/update-apply") || !strings.Contains(stdout.String(), "updated 1 repository(s)") {
		t.Fatalf("stdout = %q, want update summary", stdout.String())
	}
	checkout := checkoutPathForTest(t, p, gitURL)
	if got := gitOutputForTest(t, checkout, "rev-parse", "HEAD"); got != wantCommit {
		t.Fatalf("HEAD = %q, want %q", got, wantCommit)
	}
	repository, ok := loadState(t, p).GetRepository("github.com", "owner/update-apply")
	if !ok || repository.LastSeenCommit != wantCommit {
		t.Fatalf("repository = %#v, want updated commit", repository)
	}
}

func TestRunUpdateAllStopsOnFirstFailureAndKeepsCompletedPrefix(t *testing.T) {
	const firstURL = "https://github.com/owner/a-update"
	const secondURL = "https://github.com/owner/b-update"
	firstSource := createSourceRepo(t, "alpha")
	secondSource := createSourceRepo(t, "beta")
	withGitInsteadOfPairs(t, [2]string{firstURL, firstSource}, [2]string{secondURL, secondSource})
	p := paths.ForHome(t.TempDir())
	for _, repository := range []struct {
		url   string
		skill string
	}{{firstURL, "alpha"}, {secondURL, "beta"}} {
		var stdout, stderr strings.Builder
		if code := RunWithPaths([]string{"install", repository.url, "--tool", "claude"}, &stdout, &stderr, p); code != 0 {
			t.Fatalf("install %s code=%d stdout=%q stderr=%q", repository.url, code, stdout.String(), stderr.String())
		}
	}

	manifest := loadState(t, p)
	first, ok := manifest.GetRepository("github.com", "owner/a-update")
	if !ok {
		t.Fatal("first repository missing from state")
	}
	first.LastSeenCommit = "stale"
	manifest.UpsertRepository(first)
	if err := state.New(p).Save(manifest); err != nil {
		t.Fatalf("save stale first repository state: %v", err)
	}
	secondCheckout := checkoutPathForTest(t, p, secondURL)
	if err := os.WriteFile(filepath.Join(secondCheckout, "local.txt"), []byte("block update\n"), 0o644); err != nil {
		t.Fatalf("write dirty checkout file: %v", err)
	}

	var stdout, stderr strings.Builder
	code := RunWithPaths([]string{"update"}, &stdout, &stderr, p)

	if code == 0 {
		t.Fatalf("update all code=0 stdout=%q stderr=%q, want failure", stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "up-to-date owner/a-update") || !strings.Contains(stderr.String(), "owner/b-update") {
		t.Fatalf("update all stdout=%q stderr=%q, want completed prefix and second failure", stdout.String(), stderr.String())
	}
	firstCommit := gitOutputForTest(t, checkoutPathForTest(t, p, firstURL), "rev-parse", "HEAD")
	updatedManifest := loadState(t, p)
	updatedFirst, _ := updatedManifest.GetRepository("github.com", "owner/a-update")
	if updatedFirst.LastSeenCommit != firstCommit {
		t.Fatalf("first lastSeenCommit=%q, want completed prefix %q", updatedFirst.LastSeenCommit, firstCommit)
	}
}

func TestRunUpdateParserAndSelectionErrors(t *testing.T) {
	tests := [][]string{
		{"update", "--bad"},
		{"update", "https://github.com/owner/repo", "extra"},
		{"update", "https://github.com/owner/repo", "--dry-run", "extra"},
	}
	for _, args := range tests {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			var stdout, stderr strings.Builder
			if code := RunWithPaths(args, &stdout, &stderr, paths.ForHome(t.TempDir())); code == 0 {
				t.Fatalf("RunWithPaths(%v) code = 0, want error", args)
			}
			if !strings.Contains(stderr.String(), "Run \"skill-manager help\"") {
				t.Fatalf("stderr = %q, want usage hint", stderr.String())
			}
		})
	}

	var stdout, stderr strings.Builder
	code := RunWithPaths([]string{"update", "https://github.com/owner/missing"}, &stdout, &stderr, paths.ForHome(t.TempDir()))
	if code == 0 || !strings.Contains(stderr.String(), "not found in state") {
		t.Fatalf("missing update code=%d stderr=%q", code, stderr.String())
	}
}

func TestRunUninstallDryRunDoesNotMutate(t *testing.T) {
	const gitURL = "https://github.com/owner/uninstall-dry-run"
	source := createSourceRepo(t, "alpha")
	withGitInsteadOf(t, gitURL, source)
	p := paths.ForHome(t.TempDir())
	var installOut, installErr strings.Builder
	if code := RunWithPaths([]string{"install", gitURL, "--tool", "claude"}, &installOut, &installErr, p); code != 0 {
		t.Fatalf("install code=%d stdout=%q stderr=%q", code, installOut.String(), installErr.String())
	}
	checkout := checkoutPathForTest(t, p, gitURL)
	linkPath := filepath.Join(p.ClaudeUserSkills, "alpha")
	stateBefore, err := os.ReadFile(p.StateFile)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}

	var stdout, stderr strings.Builder
	code := RunWithPaths([]string{"uninstall", gitURL, "--dry-run"}, &stdout, &stderr, p)

	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("uninstall dry-run code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	for _, want := range []string{"dry-run: uninstall owner/uninstall-dry-run", "would remove on claude/alpha", "would remove checkout", "would remove repository"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout = %q, want %q", stdout.String(), want)
		}
	}
	if _, err := os.Lstat(checkout); err != nil {
		t.Fatalf("checkout changed: %v", err)
	}
	if _, err := os.Lstat(linkPath); err != nil {
		t.Fatalf("link changed: %v", err)
	}
	stateAfter, err := os.ReadFile(p.StateFile)
	if err != nil {
		t.Fatalf("read state after: %v", err)
	}
	if string(stateAfter) != string(stateBefore) {
		t.Fatal("uninstall dry-run changed state")
	}
	assertMissing(t, p.TrashDir)
}

func TestRunUninstallRemovesWholeRepository(t *testing.T) {
	const gitURL = "https://github.com/owner/uninstall-apply"
	source := createSourceRepo(t, "alpha")
	withGitInsteadOf(t, gitURL, source)
	p := paths.ForHome(t.TempDir())
	var installOut, installErr strings.Builder
	if code := RunWithPaths([]string{"install", gitURL, "--tool", "claude"}, &installOut, &installErr, p); code != 0 {
		t.Fatalf("install code=%d stdout=%q stderr=%q", code, installOut.String(), installErr.String())
	}
	checkout := checkoutPathForTest(t, p, gitURL)
	linkPath := filepath.Join(p.ClaudeUserSkills, "alpha")

	var stdout, stderr strings.Builder
	code := RunWithPaths([]string{"uninstall", gitURL}, &stdout, &stderr, p)

	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("uninstall code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	for _, want := range []string{"removed 1 active symlink", "removed checkout", "uninstalled owner/uninstall-apply"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout = %q, want %q", stdout.String(), want)
		}
	}
	assertMissing(t, checkout)
	assertMissing(t, linkPath)
	manifest := loadState(t, p)
	if len(manifest.Repositories) != 0 || len(manifest.Disabled) != 0 {
		t.Fatalf("manifest = %#v, want repository removed", manifest)
	}
}

func TestRunUninstallParserAndSelectionErrors(t *testing.T) {
	tests := [][]string{
		{"uninstall"},
		{"uninstall", "--dry-run"},
		{"uninstall", "https://github.com/owner/repo", "--bad"},
		{"uninstall", "https://github.com/owner/repo", "--dry-run", "extra"},
	}
	for _, args := range tests {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			var stdout, stderr strings.Builder
			if code := RunWithPaths(args, &stdout, &stderr, paths.ForHome(t.TempDir())); code == 0 {
				t.Fatalf("RunWithPaths(%v) code = 0, want error", args)
			}
			if !strings.Contains(stderr.String(), "Run \"skill-manager help\"") {
				t.Fatalf("stderr = %q, want usage hint", stderr.String())
			}
		})
	}

	var stdout, stderr strings.Builder
	code := RunWithPaths([]string{"uninstall", "https://github.com/owner/missing"}, &stdout, &stderr, paths.ForHome(t.TempDir()))
	if code == 0 || !strings.Contains(stderr.String(), "not found in state") {
		t.Fatalf("missing uninstall code=%d stderr=%q", code, stderr.String())
	}
}

func TestRunInstallDryRunMissingCheckoutDoesNotMutate(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	var stdout, stderr strings.Builder

	code := RunWithPaths([]string{"install", "https://github.com/addyosmani/agent-skills.git", "--dry-run"}, &stdout, &stderr, p)

	if code != 0 {
		t.Fatalf("RunWithPaths(install dry-run missing) code = %d stderr=%q", code, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{
		"dry-run: install https://github.com/addyosmani/agent-skills.git",
		"checkout:",
		"tools: claude,codex",
		"would clone https://github.com/addyosmani/agent-skills.git",
		"skill discovery unavailable",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	assertMissing(t, p.ReposDir)
	assertMissing(t, p.StateFile)
	assertMissing(t, p.BackupDir)
}

func TestRunInstallDryRunExistingCheckoutPlansSymlinks(t *testing.T) {
	p, _ := setupInstallDryRunCheckout(t, "https://github.com/addyosmani/agent-skills.git", "alpha", "beta")
	var stdout, stderr strings.Builder

	code := RunWithPaths([]string{"install", "git@github.com:addyosmani/agent-skills.git", "--dry-run"}, &stdout, &stderr, p)

	if code != 0 {
		t.Fatalf("RunWithPaths(install dry-run existing) code = %d stderr=%q", code, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{
		"discovered: 2 skill(s)",
		"would link claude/alpha",
		"would link codex/alpha",
		"would link claude/beta",
		"would link codex/beta",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	assertMissing(t, filepath.Join(p.ClaudeUserSkills, "alpha"))
	assertMissing(t, filepath.Join(p.CodexUserSkills, "beta"))
	assertMissing(t, p.StateFile)
}

func TestRunInstallDryRunRespectsToolAndSkillSelection(t *testing.T) {
	p, _ := setupInstallDryRunCheckout(t, "https://github.com/addyosmani/agent-skills.git", "alpha", "beta", "gamma")
	var stdout, stderr strings.Builder

	code := RunWithPaths([]string{"install", "https://github.com/addyosmani/agent-skills", "--tool", "codex", "--skill", "gamma", "--skill", "alpha", "--dry-run"}, &stdout, &stderr, p)

	if code != 0 {
		t.Fatalf("RunWithPaths(install dry-run selected) code = %d stderr=%q", code, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{"tools: codex", "would link codex/alpha", "would link codex/gamma"} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}
	for _, unwanted := range []string{"would link claude/", "would link codex/beta"} {
		if strings.Contains(output, unwanted) {
			t.Fatalf("stdout = %q, did not want %q", output, unwanted)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunInstallDryRunReportsConflictsWithoutMutation(t *testing.T) {
	p, _ := setupInstallDryRunCheckout(t, "https://github.com/addyosmani/agent-skills.git", "alpha")
	mkdirSkill(t, filepath.Join(p.ClaudeUserSkills, "alpha"))
	saveState(t, p, state.Manifest{})
	beforeState := readFile(t, p.StateFile)
	var stdout, stderr strings.Builder

	code := RunWithPaths([]string{"install", "https://github.com/addyosmani/agent-skills.git", "--tool", "claude", "--dry-run"}, &stdout, &stderr, p)

	if code == 0 {
		t.Fatal("RunWithPaths(install dry-run conflict) code = 0, want non-zero")
	}
	if !strings.Contains(stdout.String(), "discovered: 1 skill(s)") {
		t.Fatalf("stdout = %q, want discovered count", stdout.String())
	}
	for _, want := range []string{"conflict claude/alpha", "target path already exists"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr = %q, want %q", stderr.String(), want)
		}
	}
	afterState := readFile(t, p.StateFile)
	if string(afterState) != string(beforeState) {
		t.Fatalf("state changed during install dry-run conflict\nafter=%s\nbefore=%s", afterState, beforeState)
	}
	assertMissing(t, p.BackupDir)
}

func TestRunInstallDryRunReportsMissingSelectedSkillWithoutMutation(t *testing.T) {
	p, checkout := setupInstallDryRunCheckout(t, "https://github.com/addyosmani/agent-skills.git", "alpha")
	beforeCheckoutDigest := treeDigest(t, checkout)
	var stdout, stderr strings.Builder

	code := RunWithPaths([]string{"install", "https://github.com/addyosmani/agent-skills.git", "--skill", "alpha", "--skill", "missing", "--dry-run"}, &stdout, &stderr, p)

	if code == 0 {
		t.Fatal("RunWithPaths(install dry-run missing skill) code = 0, want non-zero")
	}
	if !strings.Contains(stdout.String(), "discovered: 1 skill(s)") {
		t.Fatalf("stdout = %q, want discovered count", stdout.String())
	}
	if !strings.Contains(stderr.String(), "missing skill: missing") {
		t.Fatalf("stderr = %q, want missing skill", stderr.String())
	}
	afterCheckoutDigest := treeDigest(t, checkout)
	if afterCheckoutDigest != beforeCheckoutDigest {
		t.Fatalf("checkout tree changed during strict dry-run\nafter=%s\nbefore=%s", afterCheckoutDigest, beforeCheckoutDigest)
	}
	assertMissing(t, filepath.Join(checkout, "skills", "missing"))
	assertMissing(t, filepath.Join(p.ClaudeUserSkills, "alpha"))
	assertMissing(t, filepath.Join(p.CodexUserSkills, "alpha"))
	assertMissing(t, p.StateFile)
	assertMissing(t, p.BackupDir)
}

func TestRunInstallDryRunValidatesExistingCheckout(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, p paths.Paths, checkout string)
		want  string
	}{
		{
			name: "non git directory",
			setup: func(t *testing.T, p paths.Paths, checkout string) {
				if err := os.MkdirAll(checkout, 0o755); err != nil {
					t.Fatalf("mkdir checkout: %v", err)
				}
			},
			want: "not a git checkout",
		},
		{
			name: "remote mismatch",
			setup: func(t *testing.T, p paths.Paths, checkout string) {
				createGitCheckout(t, checkout, "https://github.com/other/repo.git")
			},
			want: "does not match requested",
		},
		{
			name: "symlink checkout",
			setup: func(t *testing.T, p paths.Paths, checkout string) {
				target := filepath.Join(p.Home, "outside")
				createGitCheckout(t, target, "https://github.com/addyosmani/agent-skills.git")
				if err := os.MkdirAll(filepath.Dir(checkout), 0o755); err != nil {
					t.Fatalf("mkdir checkout parent: %v", err)
				}
				if err := os.Symlink(target, checkout); err != nil {
					t.Fatalf("create checkout symlink: %v", err)
				}
			},
			want: "is a symlink",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			p := paths.ForHome(t.TempDir())
			checkout := checkoutPathForTest(t, p, "https://github.com/addyosmani/agent-skills.git")
			test.setup(t, p, checkout)
			var stdout, stderr strings.Builder

			code := RunWithPaths([]string{"install", "https://github.com/addyosmani/agent-skills.git", "--dry-run"}, &stdout, &stderr, p)

			if code == 0 {
				t.Fatal("RunWithPaths(install dry-run invalid checkout) code = 0, want non-zero")
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty on checkout error", stdout.String())
			}
			if !strings.Contains(stderr.String(), test.want) {
				t.Fatalf("stderr = %q, want %q", stderr.String(), test.want)
			}
			assertMissing(t, p.StateFile)
		})
	}
}

func TestRunInstallRejectsUnsupportedURLsWithoutMutation(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	tests := []struct {
		name string
		url  string
		want string
	}{
		{name: "github shorthand", url: "addyosmani/agent-skills", want: "shorthand"},
		{name: "local file url", url: "file:///tmp/repo", want: "only HTTPS and SSH"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr strings.Builder

			code := RunWithPaths([]string{"install", test.url, "--dry-run"}, &stdout, &stderr, p)

			if code == 0 {
				t.Fatalf("RunWithPaths(install %s) code = 0, want non-zero", test.url)
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
			if !strings.Contains(stderr.String(), test.want) {
				t.Fatalf("stderr = %q, want %q", stderr.String(), test.want)
			}
			assertMissing(t, p.ReposDir)
			assertMissing(t, p.ClaudeUserSkills)
			assertMissing(t, p.CodexUserSkills)
			assertMissing(t, p.StateFile)
			assertMissing(t, p.BackupDir)
		})
	}
}

func TestRunInstallParserErrors(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing url", args: []string{"install", "--dry-run"}, want: "expected install"},
		{name: "duplicate tool", args: []string{"install", "https://github.com/addyosmani/agent-skills", "--tool", "codex", "--tool", "claude", "--dry-run"}, want: "--tool may be provided only once"},
		{name: "unknown flag", args: []string{"install", "https://github.com/addyosmani/agent-skills", "--branch", "main", "--dry-run"}, want: "unknown install flag"},
		{name: "missing tool value", args: []string{"install", "https://github.com/addyosmani/agent-skills", "--tool"}, want: "--tool requires a value"},
		{name: "flag as tool value", args: []string{"install", "https://github.com/addyosmani/agent-skills", "--tool", "--dry-run"}, want: "--tool requires a value"},
		{name: "missing skill value", args: []string{"install", "https://github.com/addyosmani/agent-skills", "--skill"}, want: "--skill requires a value"},
		{name: "flag as skill value", args: []string{"install", "https://github.com/addyosmani/agent-skills", "--skill", "--dry-run"}, want: "--skill requires a value"},
		{name: "extra positional", args: []string{"install", "https://github.com/addyosmani/agent-skills", "extra", "--dry-run"}, want: "unexpected install argument"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr strings.Builder
			code := RunWithPaths(test.args, &stdout, &stderr, p)
			if code == 0 {
				t.Fatalf("RunWithPaths(%v) code = 0, want non-zero", test.args)
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
			if !strings.Contains(stderr.String(), test.want) {
				t.Fatalf("stderr = %q, want %q", stderr.String(), test.want)
			}
		})
	}
}

func TestRunInstallCloneFailureDoesNotMutate(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	missingSource := filepath.Join(t.TempDir(), "missing-source")
	withGitInsteadOf(t, "https://github.com/addyosmani/agent-skills", missingSource)
	var stdout, stderr strings.Builder

	code := RunWithPaths([]string{"install", "https://github.com/addyosmani/agent-skills"}, &stdout, &stderr, p)

	if code == 0 {
		t.Fatal("RunWithPaths(install real missing remote) code = 0, want non-zero clone failure")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty on clone failure", stdout.String())
	}
	if !strings.Contains(stderr.String(), "clone repository") {
		t.Fatalf("stderr = %q, want clone failure", stderr.String())
	}
	assertMissing(t, filepath.Join(p.ClaudeUserSkills, "alpha"))
	assertMissing(t, p.StateFile)
}

func TestRunInstallClonesMissingCheckoutAndInstallsAllSkills(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	source := createSourceRepo(t, "alpha", "beta")
	withGitInsteadOf(t, "https://github.com/addyosmani/agent-skills", source)
	var stdout, stderr strings.Builder

	code := RunWithPaths([]string{"install", "https://github.com/addyosmani/agent-skills.git"}, &stdout, &stderr, p)

	if code != 0 {
		t.Fatalf("RunWithPaths(install real) code = %d stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	output := stdout.String()
	for _, want := range []string{
		"install: https://github.com/addyosmani/agent-skills.git",
		"cloned https://github.com/addyosmani/agent-skills.git",
		"created 4 symlink(s)",
		"installed 2 skill(s)",
		"start a new Claude/Codex session",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	for _, path := range []string{
		filepath.Join(p.ClaudeUserSkills, "alpha"),
		filepath.Join(p.CodexUserSkills, "alpha"),
		filepath.Join(p.ClaudeUserSkills, "beta"),
		filepath.Join(p.CodexUserSkills, "beta"),
	} {
		assertSymlink(t, path)
	}
	manifest := loadState(t, p)
	repo, ok := manifest.GetRepository("github.com", "addyosmani/agent-skills")
	if !ok {
		t.Fatal("repository manifest entry missing")
	}
	if repo.LastSeenCommit == "" || len(repo.InstalledSkills) != 2 {
		t.Fatalf("repo manifest = %#v, want commit and two installed skills", repo)
	}
}

func TestRunInstallReusesExistingCheckoutWithoutPull(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	source := createSourceRepo(t, "alpha")
	withGitInsteadOf(t, "https://github.com/addyosmani/agent-skills", source)
	var setupOut, setupErr strings.Builder
	if code := RunWithPaths([]string{"install", "https://github.com/addyosmani/agent-skills.git", "--dry-run"}, &setupOut, &setupErr, p); code != 0 {
		t.Fatalf("dry-run setup code = %d stderr=%q", code, setupErr.String())
	}
	if code := RunWithPaths([]string{"install", "https://github.com/addyosmani/agent-skills.git", "--tool", "codex"}, &setupOut, &setupErr, p); code != 0 {
		t.Fatalf("install setup code = %d stderr=%q", code, setupErr.String())
	}
	mkdirSkill(t, filepath.Join(source, "skills", "remote-only"))
	runGitForTest(t, source, "add", ".")
	runGitForTest(t, source, "commit", "-m", "Add remote-only skill")
	var stdout, stderr strings.Builder

	code := RunWithPaths([]string{"install", "https://github.com/addyosmani/agent-skills.git", "--tool", "claude"}, &stdout, &stderr, p)

	if code != 0 {
		t.Fatalf("RunWithPaths(install reuse) code = %d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "reused checkout") || !strings.Contains(stdout.String(), "created 1 symlink(s)") {
		t.Fatalf("stdout = %q, want reuse summary", stdout.String())
	}
	if strings.Contains(stdout.String(), "pull") || strings.Contains(stderr.String(), "pull") {
		t.Fatalf("output mentions pull unexpectedly stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	assertSymlink(t, filepath.Join(p.ClaudeUserSkills, "alpha"))
	assertMissing(t, filepath.Join(p.ClaudeUserSkills, "remote-only"))
}

func TestRunInstallRespectsToolAndSkillSelection(t *testing.T) {
	p, _ := setupInstallDryRunCheckout(t, "https://github.com/addyosmani/agent-skills.git", "alpha", "beta", "gamma")
	var stdout, stderr strings.Builder

	code := RunWithPaths([]string{"install", "https://github.com/addyosmani/agent-skills.git", "--tool", "codex", "--skill", "gamma", "--skill", "alpha"}, &stdout, &stderr, p)

	if code != 0 {
		t.Fatalf("RunWithPaths(install selected) code = %d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "created 2 symlink(s)") {
		t.Fatalf("stdout = %q, want created 2", stdout.String())
	}
	assertSymlink(t, filepath.Join(p.CodexUserSkills, "alpha"))
	assertSymlink(t, filepath.Join(p.CodexUserSkills, "gamma"))
	assertMissing(t, filepath.Join(p.CodexUserSkills, "beta"))
	assertMissing(t, filepath.Join(p.ClaudeUserSkills, "alpha"))
}

func TestRunInstallAlreadyActiveAndDisabledSameSkills(t *testing.T) {
	p, _ := setupInstallDryRunCheckout(t, "https://github.com/addyosmani/agent-skills.git", "alpha", "beta")
	checkout := checkoutPathForTest(t, p, "https://github.com/addyosmani/agent-skills.git")
	alphaPath := filepath.Join(checkout, "skills", "alpha")
	betaPath := filepath.Join(checkout, "skills", "beta")
	mkdirAll(t, p.ClaudeUserSkills)
	if err := os.Symlink(alphaPath, filepath.Join(p.ClaudeUserSkills, "alpha")); err != nil {
		t.Fatalf("create active alpha: %v", err)
	}
	disabledBeta := filepath.Join(p.ClaudeDisabledDir, "beta")
	mkdirAll(t, filepath.Dir(disabledBeta))
	if err := os.Symlink(betaPath, disabledBeta); err != nil {
		t.Fatalf("create disabled beta: %v", err)
	}
	saveState(t, p, state.Manifest{Disabled: []state.DisabledEntry{{
		Tool:          model.ToolClaude,
		SkillName:     "beta",
		OriginalPath:  filepath.Join(p.ClaudeUserSkills, "beta"),
		DisabledPath:  disabledBeta,
		EntryType:     model.EntryTypeSymlink,
		SymlinkTarget: betaPath,
	}}})
	var stdout, stderr strings.Builder

	code := RunWithPaths([]string{"install", "https://github.com/addyosmani/agent-skills.git", "--tool", "claude"}, &stdout, &stderr, p)

	if code != 0 {
		t.Fatalf("RunWithPaths(install already) code = %d stderr=%q", code, stderr.String())
	}
	for _, want := range []string{
		"created 0 symlink(s)",
		"already installed 2 item(s)",
		"already installed claude/alpha: ON",
		"already installed claude/beta: OFF",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout = %q, want %q", stdout.String(), want)
		}
	}
	if strings.Contains(stdout.String(), "already installed claude/beta: ON") {
		t.Fatalf("stdout = %q, want already installed summary", stdout.String())
	}
	assertMissing(t, filepath.Join(p.ClaudeUserSkills, "beta"))
	assertSymlink(t, disabledBeta)
}

func TestRunInstallConflictFailsBeforeCreatingSymlinks(t *testing.T) {
	p, _ := setupInstallDryRunCheckout(t, "https://github.com/addyosmani/agent-skills.git", "alpha", "beta")
	mkdirSkill(t, filepath.Join(p.ClaudeUserSkills, "alpha"))
	var stdout, stderr strings.Builder

	code := RunWithPaths([]string{"install", "https://github.com/addyosmani/agent-skills.git", "--tool", "claude"}, &stdout, &stderr, p)

	if code == 0 {
		t.Fatal("RunWithPaths(install conflict) code = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "conflict claude/alpha") {
		t.Fatalf("stderr = %q, want conflict", stderr.String())
	}
	assertMissing(t, filepath.Join(p.ClaudeUserSkills, "beta"))
	manifest := loadState(t, p)
	if _, ok := manifest.GetRepository("github.com", "addyosmani/agent-skills"); ok {
		t.Fatal("repository state updated despite conflict")
	}
}

func TestRunInstallRemoteMismatchFails(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	checkout := checkoutPathForTest(t, p, "https://github.com/addyosmani/agent-skills.git")
	createGitCheckout(t, checkout, "https://github.com/other/repo.git")
	mkdirSkill(t, filepath.Join(checkout, "skills", "alpha"))
	runGitForTest(t, checkout, "add", ".")
	runGitForTest(t, checkout, "commit", "-m", "Add skills")
	var stdout, stderr strings.Builder

	code := RunWithPaths([]string{"install", "https://github.com/addyosmani/agent-skills.git"}, &stdout, &stderr, p)

	if code == 0 {
		t.Fatal("RunWithPaths(install remote mismatch) code = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "does not match requested") {
		t.Fatalf("stderr = %q, want mismatch", stderr.String())
	}
	assertMissing(t, filepath.Join(p.ClaudeUserSkills, "alpha"))
}

func TestRunUnknownCommandFails(t *testing.T) {
	var stdout, stderr strings.Builder

	code := Run([]string{"nonsense"}, &stdout, &stderr)

	if code == 0 {
		t.Fatal("Run(unknown) code = 0, want non-zero")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), `unknown command "nonsense"`) {
		t.Fatalf("stderr = %q, want unknown command message", stderr.String())
	}
	if !strings.Contains(stderr.String(), `skill-manager help`) {
		t.Fatalf("stderr = %q, want help hint", stderr.String())
	}
}

func TestRunListPrintsSkillRows(t *testing.T) {
	p := setupListStatusFixture(t)
	var stdout, stderr strings.Builder

	code := RunWithPaths([]string{"list"}, &stdout, &stderr, p)

	if code != 0 {
		t.Fatalf("RunWithPaths(list) code = %d stderr=%q", code, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{"Skill", "Claude", "Codex", "Source", "active-claude", "ON", "imagegen", "RO", "off-skill", "OFF", "conflict-skill", "CONFLICT"} {
		if !strings.Contains(output, want) {
			t.Fatalf("list output = %q, want %q", output, want)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunListJSONReturnsVersionedPathFreeInventory(t *testing.T) {
	p := setupListStatusFixture(t)
	var stdout, stderr strings.Builder

	code := RunWithPaths([]string{"list", "--json"}, &stdout, &stderr, p)

	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("RunWithPaths(list --json) code=%d stderr=%q", code, stderr.String())
	}
	var output struct {
		APIVersion int `json:"apiVersion"`
		Skills     []struct {
			Name  string `json:"name"`
			Tools struct {
				Claude struct {
					State      string `json:"state"`
					Toggleable bool   `json:"toggleable"`
				} `json:"claude"`
				Codex struct {
					State      string `json:"state"`
					Toggleable bool   `json:"toggleable"`
				} `json:"codex"`
			} `json:"tools"`
		} `json:"skills"`
	}
	if err := json.Unmarshal([]byte(stdout.String()), &output); err != nil {
		t.Fatalf("decode list JSON: %v\n%s", err, stdout.String())
	}
	if output.APIVersion != 1 || len(output.Skills) != 4 {
		t.Fatalf("list JSON = %#v", output)
	}
	byName := map[string]struct {
		ClaudeState      string
		ClaudeToggleable bool
		CodexState       string
		CodexToggleable  bool
	}{}
	for _, skill := range output.Skills {
		byName[skill.Name] = struct {
			ClaudeState      string
			ClaudeToggleable bool
			CodexState       string
			CodexToggleable  bool
		}{skill.Tools.Claude.State, skill.Tools.Claude.Toggleable, skill.Tools.Codex.State, skill.Tools.Codex.Toggleable}
	}
	if got := byName["off-skill"]; got.ClaudeState != "off" || !got.ClaudeToggleable || got.CodexState != "missing" {
		t.Fatalf("off-skill JSON = %#v", got)
	}
	if got := byName["imagegen"]; got.CodexState != "read_only" || got.CodexToggleable {
		t.Fatalf("imagegen JSON = %#v", got)
	}
	if strings.Contains(stdout.String(), p.Home) {
		t.Fatalf("list JSON leaked a filesystem path: %s", stdout.String())
	}
}

func TestRunListJSONFiltersAvailableSkillsByToolAndMetadata(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	for _, name := range []string{"ffmpeg", "unrelated"} {
		disabledPath := filepath.Join(p.CodexDisabledDir, name)
		mkdirSkill(t, disabledPath)
	}
	saveState(t, p, state.Manifest{Disabled: []state.DisabledEntry{
		{
			Tool: model.ToolCodex, SkillName: "ffmpeg",
			OriginalPath: filepath.Join(p.CodexUserSkills, "ffmpeg"),
			DisabledPath: filepath.Join(p.CodexDisabledDir, "ffmpeg"),
			EntryType:    model.EntryTypeDir, Source: model.SourceLocal,
		},
		{
			Tool: model.ToolCodex, SkillName: "unrelated",
			OriginalPath: filepath.Join(p.CodexUserSkills, "unrelated"),
			DisabledPath: filepath.Join(p.CodexDisabledDir, "unrelated"),
			EntryType:    model.EntryTypeDir, Source: model.SourceLocal,
		},
	}})
	var stdout, stderr strings.Builder

	code := RunWithPaths([]string{"list", "--json", "--available-for", "codex", "--query", "FFMPEG", "--query", "remotion"}, &stdout, &stderr, p)

	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("filtered list code=%d stderr=%q", code, stderr.String())
	}
	var output listJSONOutput
	if err := json.Unmarshal([]byte(stdout.String()), &output); err != nil {
		t.Fatalf("decode filtered list: %v", err)
	}
	if output.APIVersion != 1 || len(output.Skills) != 1 || output.Skills[0].Name != "ffmpeg" {
		t.Fatalf("filtered list = %#v", output)
	}
}

func TestRunListJSONRejectsFiltersWithoutJSON(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	var stdout, stderr strings.Builder

	code := RunWithPaths([]string{"list", "--available-for", "codex"}, &stdout, &stderr, p)

	if code == 0 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "require --json") {
		t.Fatalf("filtered list code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestRunAdvisorJSONActivationStatusAndCleanup(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	disabledPath := filepath.Join(p.CodexDisabledDir, "ffmpeg")
	mkdirSkill(t, disabledPath)
	saveState(t, p, state.Manifest{Disabled: []state.DisabledEntry{{
		Tool:         model.ToolCodex,
		SkillName:    "ffmpeg",
		OriginalPath: filepath.Join(p.CodexUserSkills, "ffmpeg"),
		DisabledPath: disabledPath,
		EntryType:    model.EntryTypeDir,
		Source:       model.SourceLocal,
		Group:        model.GroupLocal,
	}}})
	var stdout, stderr strings.Builder
	code := RunWithPaths([]string{"advisor", "activate", "--tool", "codex", "--skill", "ffmpeg", "--json"}, &stdout, &stderr, p)
	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("advisor activate code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	var activation struct {
		APIVersion int    `json:"apiVersion"`
		ReceiptID  string `json:"receiptId"`
		Actions    []struct {
			Skill  string `json:"skill"`
			Action string `json:"action"`
		} `json:"actions"`
	}
	if err := json.Unmarshal([]byte(stdout.String()), &activation); err != nil {
		t.Fatal(err)
	}
	if activation.APIVersion != 1 || len(activation.ReceiptID) != 32 || len(activation.Actions) != 1 || activation.Actions[0].Action != "enable" {
		t.Fatalf("activation = %#v", activation)
	}
	assertExists(t, filepath.Join(p.CodexUserSkills, "ffmpeg", "SKILL.md"))

	stdout.Reset()
	stderr.Reset()
	code = RunWithPaths([]string{"advisor", "status", "--tool", "codex", "--json"}, &stdout, &stderr, p)
	if code != 0 || !strings.Contains(stdout.String(), activation.ReceiptID) || stderr.Len() != 0 {
		t.Fatalf("advisor status code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = RunWithPaths([]string{"advisor", "cleanup", "--receipt", activation.ReceiptID, "--json"}, &stdout, &stderr, p)
	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("advisor cleanup code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), `"action": "disable"`) {
		t.Fatalf("cleanup JSON = %s", stdout.String())
	}
	assertMissing(t, filepath.Join(p.CodexUserSkills, "ffmpeg"))
	assertExists(t, filepath.Join(p.CodexDisabledDir, "ffmpeg", "SKILL.md"))
}

func TestRunAdvisorJSONUsesStructuredErrors(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	var stdout, stderr strings.Builder

	code := RunWithPaths([]string{"advisor", "activate", "--tool", "codex", "--skill", "missing", "--json"}, &stdout, &stderr, p)

	if code == 0 || stdout.Len() != 0 {
		t.Fatalf("advisor error code=%d stdout=%q", code, stdout.String())
	}
	var output struct {
		APIVersion int `json:"apiVersion"`
		Error      struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(stderr.String()), &output); err != nil {
		t.Fatalf("decode advisor error: %v\n%s", err, stderr.String())
	}
	if output.APIVersion != 1 || output.Error.Code != "ACTIVATION_FAILED" || !strings.Contains(output.Error.Message, "not installed") {
		t.Fatalf("advisor error = %#v", output)
	}
}

func TestRunStatusSummarizesCounts(t *testing.T) {
	p := setupListStatusFixture(t)
	var stdout, stderr strings.Builder

	code := RunWithPaths([]string{"status"}, &stdout, &stderr, p)

	if code != 0 {
		t.Fatalf("RunWithPaths(status) code = %d stderr=%q", code, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{"ON: 1", "OFF: 1", "CONFLICT: 1", "RO: 1"} {
		if !strings.Contains(output, want) {
			t.Fatalf("status output = %q, want %q", output, want)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunGroupsSummarizesGroups(t *testing.T) {
	p := setupGroupsFixture(t)
	beforeState := readFile(t, p.StateFile)
	var stdout, stderr strings.Builder

	code := RunWithPaths([]string{"groups"}, &stdout, &stderr, p)

	if code != 0 {
		t.Fatalf("RunWithPaths(groups) code = %d stderr=%q", code, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{
		"Group", "Rows", "Claude", "Codex", "Sources",
		"local", "3", "ON:1 OFF:1 CONFLICT:0 RO:0", "ON:0 OFF:0 CONFLICT:1 RO:0", "local",
		"Codex system", "ON:0 OFF:0 CONFLICT:0 RO:1", "Codex system",
		"Claude plugin", "ON:0 OFF:0 CONFLICT:0 RO:1", "Claude plugin",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("groups output = %q, want %q", output, want)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	groupRows := groupsByName(t, output)
	for _, group := range []string{"Claude plugin", "Codex system", "local"} {
		if _, ok := groupRows[group]; !ok {
			t.Fatalf("groups output rows = %#v, want group %q", groupRows, group)
		}
	}
	if len(groupRows) != 3 {
		t.Fatalf("group row count = %d (%#v), want exactly 3", len(groupRows), groupRows)
	}
	afterState := readFile(t, p.StateFile)
	if string(afterState) != string(beforeState) {
		t.Fatalf("state changed during groups\nafter=%s\nbefore=%s", afterState, beforeState)
	}
	assertExists(t, filepath.Join(p.ClaudeUserSkills, "active-local", "SKILL.md"))
	assertExists(t, filepath.Join(p.CodexSystemSkills, "imagegen", "SKILL.md"))
}

func TestRunGroupsSummarizesRepoSkillsCLIAndDisabledGroups(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	p := paths.ForHome(t.TempDir())
	sourceDir := filepath.Join(p.Home, "android-source")
	mkdirSkill(t, sourceDir)
	initGitSkill(t, sourceDir, "https://github.com/android/skills.git")
	if err := os.MkdirAll(p.ClaudeUserSkills, 0o755); err != nil {
		t.Fatalf("mkdir Claude skills: %v", err)
	}
	if err := os.Symlink(sourceDir, filepath.Join(p.ClaudeUserSkills, "agp-9-upgrade")); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	mkdirSkill(t, filepath.Join(p.CodexUserSkills, "find-skills"))
	if err := os.WriteFile(p.AgentsSkillLock, []byte(`{
  "skills": {
    "find-skills": {
      "source": "vercel-labs/skills",
      "skillPath": "skills/find-skills/SKILL.md"
    }
  }
}`), 0o644); err != nil {
		t.Fatalf("write skill lock: %v", err)
	}

	disabledPath := filepath.Join(p.ClaudeDisabledDir, "stored-custom")
	mkdirSkill(t, disabledPath)
	saveState(t, p, state.Manifest{Disabled: []state.DisabledEntry{
		{
			Tool:         model.ToolClaude,
			SkillName:    "stored-custom",
			OriginalPath: filepath.Join(p.ClaudeUserSkills, "stored-custom"),
			DisabledPath: disabledPath,
			EntryType:    model.EntryTypeDir,
			Source:       model.SourceLocal,
			Group:        model.GroupLabel("stored/custom"),
		},
	}})
	beforeState := readFile(t, p.StateFile)
	var stdout, stderr strings.Builder

	code := RunWithPaths([]string{"groups"}, &stdout, &stderr, p)

	if code != 0 {
		t.Fatalf("RunWithPaths(groups) code = %d stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{"Group", "Rows", "Claude", "Codex", "Sources"} {
		if !strings.Contains(output, want) {
			t.Fatalf("groups output = %q, want %q", output, want)
		}
	}
	groupRows := groupsByName(t, output)
	if len(groupRows) != 3 {
		t.Fatalf("group row count = %d (%#v), want exactly 3", len(groupRows), groupRows)
	}
	assertGroupRow(t, groupRows, "android/skills", "1", "ON:1 OFF:0 CONFLICT:0 RO:0", "ON:0 OFF:0 CONFLICT:0 RO:0", "symlink repo")
	assertGroupRow(t, groupRows, "Skills CLI", "1", "ON:0 OFF:0 CONFLICT:0 RO:0", "ON:1 OFF:0 CONFLICT:0 RO:0", "Skills CLI")
	assertGroupRow(t, groupRows, "stored/custom", "1", "ON:0 OFF:1 CONFLICT:0 RO:0", "ON:0 OFF:0 CONFLICT:0 RO:0", "local")
	afterState := readFile(t, p.StateFile)
	if string(afterState) != string(beforeState) {
		t.Fatalf("state changed during groups\nafter=%s\nbefore=%s", afterState, beforeState)
	}
	assertExists(t, filepath.Join(p.CodexUserSkills, "find-skills", "SKILL.md"))
	assertExists(t, filepath.Join(p.ClaudeDisabledDir, "stored-custom", "SKILL.md"))
}

func TestRunGroupsDoesNotCreateState(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	mkdirSkill(t, filepath.Join(p.ClaudeUserSkills, "active-local"))
	var stdout, stderr strings.Builder

	code := RunWithPaths([]string{"groups"}, &stdout, &stderr, p)

	if code != 0 {
		t.Fatalf("RunWithPaths(groups) code = %d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "local") {
		t.Fatalf("stdout = %q, want local group", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	assertMissing(t, p.StateFile)
	assertExists(t, filepath.Join(p.ClaudeUserSkills, "active-local", "SKILL.md"))
}

func TestRunGroupsRejectsArguments(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	var stdout, stderr strings.Builder

	code := RunWithPaths([]string{"groups", "extra"}, &stdout, &stderr, p)

	if code == 0 {
		t.Fatal("RunWithPaths(groups extra) code = 0, want non-zero")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "groups does not accept arguments") {
		t.Fatalf("stderr = %q, want groups arg error", stderr.String())
	}
}

func TestRunReposEmptyManifestDoesNotCreateState(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	var stdout, stderr strings.Builder

	code := RunWithPaths([]string{"repos"}, &stdout, &stderr, p)

	if code != 0 {
		t.Fatalf("RunWithPaths(repos empty) code = %d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "No managed repositories recorded.") {
		t.Fatalf("stdout = %q, want empty state message", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	assertMissing(t, p.StateFile)
	assertMissing(t, p.BackupDir)
}

func TestRunReposPrintsManagedRepositoriesReadOnly(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	zetaCheckout := filepath.Join(p.ReposDir, "github.com", "zeta", "repo")
	saveState(t, p, state.Manifest{Repositories: []state.RepositoryEntry{
		{
			OriginalURL:    "https://github.com/zeta/repo.git",
			CanonicalURL:   "https://github.com/zeta/repo",
			Host:           "github.com",
			RepoPath:       "zeta/repo",
			CheckoutPath:   zetaCheckout,
			Group:          model.GroupLabel("zeta/repo"),
			LastSeenCommit: "def456",
			InstalledSkills: []state.InstalledSkillEntry{
				{
					Name:         "bravo",
					RelativePath: "skills/bravo",
					Tools:        []model.Tool{model.Tool("zed"), model.ToolCodex, model.ToolClaude},
				},
				{
					Name:         "charlie",
					RelativePath: "skills/charlie",
					Tools:        []model.Tool{model.Tool("alpha"), model.ToolCodex},
				},
			},
		},
		{
			CanonicalURL: "https://github.com/alpha/repo",
			Host:         "github.com",
			RepoPath:     "alpha/repo",
			InstalledSkills: []state.InstalledSkillEntry{
				{
					Name:         "solo",
					RelativePath: "solo",
				},
			},
		},
	}})
	beforeState := readFile(t, p.StateFile)
	var stdout, stderr strings.Builder

	code := RunWithPaths([]string{"repos"}, &stdout, &stderr, p)

	if code != 0 {
		t.Fatalf("RunWithPaths(repos) code = %d stderr=%q", code, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{
		"Group", "URL", "Checkout", "Commit", "Skills", "Tools",
		"github.com/alpha/repo", "https://github.com/alpha/repo",
		"zeta/repo", "https://github.com/zeta/repo.git", zetaCheckout, "def456",
		"claude,codex,alpha,zed",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("repos output = %q, want %q", output, want)
		}
	}
	alphaIndex := strings.Index(output, "github.com/alpha/repo")
	zetaIndex := strings.Index(output, "zeta/repo")
	if alphaIndex == -1 || zetaIndex == -1 || alphaIndex > zetaIndex {
		t.Fatalf("repos output order = %q, want alpha before zeta", output)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	afterState := readFile(t, p.StateFile)
	if string(afterState) != string(beforeState) {
		t.Fatalf("state changed during repos\nafter=%s\nbefore=%s", afterState, beforeState)
	}
	assertMissing(t, p.BackupDir)
}

func TestRunReposInvalidStateFails(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	if err := os.MkdirAll(p.StateDir, 0o755); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	if err := os.WriteFile(p.StateFile, []byte(`{`), 0o644); err != nil {
		t.Fatalf("write invalid state: %v", err)
	}
	var stdout, stderr strings.Builder

	code := RunWithPaths([]string{"repos"}, &stdout, &stderr, p)

	if code == 0 {
		t.Fatal("RunWithPaths(repos invalid state) code = 0, want non-zero")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	for _, want := range []string{"error:", "decode state manifest"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr = %q, want %q", stderr.String(), want)
		}
	}
	assertMissing(t, p.BackupDir)
}

func TestRunReposRejectsArguments(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	var stdout, stderr strings.Builder

	code := RunWithPaths([]string{"repos", "extra"}, &stdout, &stderr, p)

	if code == 0 {
		t.Fatal("RunWithPaths(repos extra) code = 0, want non-zero")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "repos does not accept arguments") {
		t.Fatalf("stderr = %q, want repos arg error", stderr.String())
	}
}

func TestRunDisableClaudeSkill(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	activePath := filepath.Join(p.ClaudeUserSkills, "edge-to-edge")
	mkdirSkill(t, activePath)
	var stdout, stderr strings.Builder

	code := RunWithPaths([]string{"disable", "--tool", "claude", "edge-to-edge"}, &stdout, &stderr, p)

	if code != 0 {
		t.Fatalf("RunWithPaths(disable) code = %d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "disabled claude/edge-to-edge") {
		t.Fatalf("stdout = %q, want disabled message", stdout.String())
	}
	assertMissing(t, activePath)
	assertExists(t, filepath.Join(p.ClaudeDisabledDir, "edge-to-edge", "SKILL.md"))
	manifest := loadState(t, p)
	if _, ok := manifest.Get(model.ToolClaude, "edge-to-edge"); !ok {
		t.Fatal("state entry missing after disable")
	}
}

func TestRunEnableCodexSkill(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	activePath := filepath.Join(p.CodexUserSkills, "find-skills")
	mkdirSkill(t, activePath)

	var stdout, stderr strings.Builder
	if code := RunWithPaths([]string{"disable", "--tool", "codex", "find-skills"}, &stdout, &stderr, p); code != 0 {
		t.Fatalf("disable setup code = %d stderr=%q", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()

	code := RunWithPaths([]string{"enable", "--tool", "codex", "find-skills"}, &stdout, &stderr, p)

	if code != 0 {
		t.Fatalf("RunWithPaths(enable) code = %d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "enabled codex/find-skills") {
		t.Fatalf("stdout = %q, want enabled message", stdout.String())
	}
	assertExists(t, filepath.Join(activePath, "SKILL.md"))
	assertMissing(t, filepath.Join(p.CodexDisabledDir, "find-skills"))
	manifest := loadState(t, p)
	if _, ok := manifest.Get(model.ToolCodex, "find-skills"); ok {
		t.Fatal("state entry still present after enable")
	}
}

func TestRunMutateInvalidToolFails(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	var stdout, stderr strings.Builder

	code := RunWithPaths([]string{"disable", "--tool", "bad", "skill"}, &stdout, &stderr, p)

	if code == 0 {
		t.Fatal("RunWithPaths(invalid tool) code = 0, want non-zero")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), `invalid tool "bad"`) {
		t.Fatalf("stderr = %q, want invalid tool", stderr.String())
	}
}

func TestRunMutateReadOnlyFailsCleanly(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	mkdirSkill(t, filepath.Join(p.CodexSystemSkills, "imagegen"))
	var stdout, stderr strings.Builder

	code := RunWithPaths([]string{"disable", "--tool", "codex", "imagegen"}, &stdout, &stderr, p)

	if code == 0 {
		t.Fatal("RunWithPaths(read-only disable) code = 0, want non-zero")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "read-only") {
		t.Fatalf("stderr = %q, want read-only error", stderr.String())
	}
	assertExists(t, filepath.Join(p.CodexSystemSkills, "imagegen", "SKILL.md"))
}

func TestRunDisableManagedSkillWithSameNameAsReadOnly(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	activePath := filepath.Join(p.CodexUserSkills, "imagegen")
	mkdirSkill(t, activePath)
	mkdirSkill(t, filepath.Join(p.CodexSystemSkills, "imagegen"))
	var stdout, stderr strings.Builder

	code := RunWithPaths([]string{"disable", "--tool", "codex", "imagegen"}, &stdout, &stderr, p)

	if code != 0 {
		t.Fatalf("RunWithPaths(disable duplicate read-only name) code = %d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "disabled codex/imagegen") {
		t.Fatalf("stdout = %q, want disabled message", stdout.String())
	}
	assertMissing(t, activePath)
	assertExists(t, filepath.Join(p.CodexSystemSkills, "imagegen", "SKILL.md"))
	assertExists(t, filepath.Join(p.CodexDisabledDir, "imagegen", "SKILL.md"))
}

func TestRunDisableDryRunDoesNotMutate(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	activePath := filepath.Join(p.ClaudeUserSkills, "edge-to-edge")
	disabledPath := filepath.Join(p.ClaudeDisabledDir, "edge-to-edge")
	mkdirSkill(t, activePath)
	var stdout, stderr strings.Builder

	code := RunWithPaths([]string{"disable", "--tool", "claude", "edge-to-edge", "--dry-run"}, &stdout, &stderr, p)

	if code != 0 {
		t.Fatalf("RunWithPaths(disable dry-run) code = %d stderr=%q", code, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{"dry-run: disable claude/edge-to-edge", activePath, disabledPath} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	assertExists(t, filepath.Join(activePath, "SKILL.md"))
	assertMissing(t, disabledPath)
	assertMissing(t, p.StateFile)
	assertMissing(t, p.BackupDir)
}

func TestRunEnableDryRunDoesNotMutateState(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	disabledPath := filepath.Join(p.CodexDisabledDir, "find-skills")
	originalPath := filepath.Join(p.CodexUserSkills, "find-skills")
	mkdirSkill(t, disabledPath)
	saveState(t, p, state.Manifest{Disabled: []state.DisabledEntry{{
		Tool:         model.ToolCodex,
		SkillName:    "find-skills",
		OriginalPath: originalPath,
		DisabledPath: disabledPath,
		EntryType:    model.EntryTypeDir,
		Source:       model.SourceSkillsCLI,
	}}})
	before := readFile(t, p.StateFile)
	var stdout, stderr strings.Builder

	code := RunWithPaths([]string{"enable", "--tool", "codex", "find-skills", "--dry-run"}, &stdout, &stderr, p)

	if code != 0 {
		t.Fatalf("RunWithPaths(enable dry-run) code = %d stderr=%q", code, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{"dry-run: enable codex/find-skills", disabledPath, originalPath} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	assertExists(t, filepath.Join(disabledPath, "SKILL.md"))
	assertMissing(t, originalPath)
	after := readFile(t, p.StateFile)
	if string(after) != string(before) {
		t.Fatalf("state changed during dry-run\nafter=%s\nbefore=%s", after, before)
	}
	assertMissing(t, p.BackupDir)
}

func TestRunEnableDryRunReportsConflictWithoutMutation(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	disabledPath := filepath.Join(p.CodexDisabledDir, "find-skills")
	originalPath := filepath.Join(p.CodexUserSkills, "find-skills")
	mkdirSkill(t, disabledPath)
	mkdirSkill(t, originalPath)
	saveState(t, p, state.Manifest{Disabled: []state.DisabledEntry{{
		Tool:         model.ToolCodex,
		SkillName:    "find-skills",
		OriginalPath: originalPath,
		DisabledPath: disabledPath,
		EntryType:    model.EntryTypeDir,
		Source:       model.SourceSkillsCLI,
	}}})
	before := readFile(t, p.StateFile)
	var stdout, stderr strings.Builder

	code := RunWithPaths([]string{"enable", "--tool", "codex", "find-skills", "--dry-run"}, &stdout, &stderr, p)

	if code == 0 {
		t.Fatal("RunWithPaths(enable dry-run conflict) code = 0, want non-zero")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "already exists") {
		t.Fatalf("stderr = %q, want conflict error", stderr.String())
	}
	assertExists(t, filepath.Join(disabledPath, "SKILL.md"))
	assertExists(t, filepath.Join(originalPath, "SKILL.md"))
	after := readFile(t, p.StateFile)
	if string(after) != string(before) {
		t.Fatalf("state changed during dry-run conflict\nafter=%s\nbefore=%s", after, before)
	}
	assertMissing(t, p.BackupDir)
}

func TestRunDryRunWrongPositionFails(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	var stdout, stderr strings.Builder

	code := RunWithPaths([]string{"disable", "--dry-run", "--tool", "claude", "skill"}, &stdout, &stderr, p)

	if code == 0 {
		t.Fatal("RunWithPaths(misplaced dry-run) code = 0, want non-zero")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "expected --tool") {
		t.Fatalf("stderr = %q, want usage error", stderr.String())
	}
}

func setupListStatusFixture(t *testing.T) paths.Paths {
	t.Helper()
	p := paths.ForHome(t.TempDir())
	mkdirSkill(t, filepath.Join(p.ClaudeUserSkills, "active-claude"))
	mkdirSkill(t, filepath.Join(p.CodexSystemSkills, "imagegen"))

	offDisabledPath := filepath.Join(p.ClaudeDisabledDir, "off-skill")
	conflictDisabledPath := filepath.Join(p.CodexDisabledDir, "conflict-skill")
	conflictOriginalPath := filepath.Join(p.CodexUserSkills, "conflict-skill")
	mkdirSkill(t, offDisabledPath)
	mkdirSkill(t, conflictDisabledPath)
	mkdirSkill(t, conflictOriginalPath)

	saveState(t, p, state.Manifest{Disabled: []state.DisabledEntry{
		{
			Tool:         model.ToolClaude,
			SkillName:    "off-skill",
			OriginalPath: filepath.Join(p.ClaudeUserSkills, "off-skill"),
			DisabledPath: offDisabledPath,
			EntryType:    model.EntryTypeDir,
			Source:       model.SourceLocal,
		},
		{
			Tool:         model.ToolCodex,
			SkillName:    "conflict-skill",
			OriginalPath: conflictOriginalPath,
			DisabledPath: conflictDisabledPath,
			EntryType:    model.EntryTypeDir,
			Source:       model.SourceLocal,
		},
	}})
	return p
}

func setupGroupsFixture(t *testing.T) paths.Paths {
	t.Helper()
	p := paths.ForHome(t.TempDir())
	mkdirSkill(t, filepath.Join(p.ClaudeUserSkills, "active-local"))
	mkdirSkill(t, filepath.Join(p.CodexSystemSkills, "imagegen"))
	mkdirSkill(t, filepath.Join(p.ClaudePluginCache, "github", "owner", "plugin", "skills", "plugin-skill"))

	offDisabledPath := filepath.Join(p.ClaudeDisabledDir, "off-local")
	conflictDisabledPath := filepath.Join(p.CodexDisabledDir, "conflict-local")
	conflictOriginalPath := filepath.Join(p.CodexUserSkills, "conflict-local")
	mkdirSkill(t, offDisabledPath)
	mkdirSkill(t, conflictDisabledPath)
	mkdirSkill(t, conflictOriginalPath)

	saveState(t, p, state.Manifest{Disabled: []state.DisabledEntry{
		{
			Tool:         model.ToolClaude,
			SkillName:    "off-local",
			OriginalPath: filepath.Join(p.ClaudeUserSkills, "off-local"),
			DisabledPath: offDisabledPath,
			EntryType:    model.EntryTypeDir,
			Source:       model.SourceLocal,
			Group:        model.GroupLocal,
		},
		{
			Tool:         model.ToolCodex,
			SkillName:    "conflict-local",
			OriginalPath: conflictOriginalPath,
			DisabledPath: conflictDisabledPath,
			EntryType:    model.EntryTypeDir,
			Source:       model.SourceLocal,
			Group:        model.GroupLocal,
		},
	}})
	return p
}

func setupInstallDryRunCheckout(t *testing.T, remote string, skillNames ...string) (paths.Paths, string) {
	t.Helper()
	p := paths.ForHome(t.TempDir())
	checkout := checkoutPathForTest(t, p, remote)
	createGitCheckout(t, checkout, remote)
	for _, name := range skillNames {
		mkdirSkill(t, filepath.Join(checkout, "skills", name))
	}
	runGitForTest(t, checkout, "add", ".")
	runGitForTest(t, checkout, "commit", "-m", "Add skills")
	return p, checkout
}

func checkoutPathForTest(t *testing.T, p paths.Paths, rawURL string) string {
	t.Helper()
	identity, err := install.NormalizeGitURL(rawURL)
	if err != nil {
		t.Fatalf("NormalizeGitURL() error = %v", err)
	}
	checkout, err := install.CheckoutPath(p, identity)
	if err != nil {
		t.Fatalf("CheckoutPath() error = %v", err)
	}
	return checkout
}

func createGitCheckout(t *testing.T, dir, remote string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir checkout: %v", err)
	}
	runGitForTest(t, dir, "init")
	runGitForTest(t, dir, "config", "user.email", "skill-manager@example.test")
	runGitForTest(t, dir, "config", "user.name", "Skill Manager Test")
	runGitForTest(t, dir, "config", "commit.gpgsign", "false")
	runGitForTest(t, dir, "config", "tag.gpgsign", "false")
	runGitForTest(t, dir, "config", "core.hooksPath", t.TempDir())
	if remote != "" {
		runGitForTest(t, dir, "remote", "add", "origin", remote)
	}
}

func createSourceRepo(t *testing.T, skillNames ...string) string {
	t.Helper()
	source := filepath.Join(t.TempDir(), "source")
	createGitCheckout(t, source, "")
	for _, name := range skillNames {
		mkdirSkill(t, filepath.Join(source, "skills", name))
	}
	runGitForTest(t, source, "add", ".")
	runGitForTest(t, source, "commit", "-m", "Add skills")
	return source
}

func withGitInsteadOf(t *testing.T, httpsPrefix, source string) {
	t.Helper()
	withGitInsteadOfPairs(t, [2]string{httpsPrefix, source})
}

func withGitInsteadOfPairs(t *testing.T, pairs ...[2]string) {
	t.Helper()
	configPath := filepath.Join(t.TempDir(), ".gitconfig")
	var contents strings.Builder
	for _, pair := range pairs {
		replacement := "file://" + filepath.ToSlash(pair[1])
		fmt.Fprintf(&contents, "[url %q]\n\tinsteadOf = %s.git\n\tinsteadOf = %s\n", replacement, pair[0], pair[0])
	}
	if err := os.WriteFile(configPath, []byte(contents.String()), 0o644); err != nil {
		t.Fatalf("write git config: %v", err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", configPath)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
}

func mkdirSkill(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# Skill\n"), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}
}

func mkdirAll(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
}

func saveState(t *testing.T, p paths.Paths, manifest state.Manifest) {
	t.Helper()
	if err := state.New(p).Save(manifest); err != nil {
		t.Fatalf("save state: %v", err)
	}
}

func loadState(t *testing.T, p paths.Paths) state.Manifest {
	t.Helper()
	manifest, err := state.New(p).Load()
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	return manifest
}

func assertMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("lstat %s error = %v, want not exist", path, err)
	}
}

func assertExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
}

func assertSymlink(t *testing.T, path string) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("lstat %s: %v", path, err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("%s mode = %v, want symlink", path, info.Mode())
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

func treeDigest(t *testing.T, root string) string {
	t.Helper()
	hash := sha256.New()
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		hash.Write([]byte(relative))
		hash.Write([]byte{0})
		hash.Write([]byte(info.Mode().String()))
		hash.Write([]byte{0})
		if entry.Type().IsRegular() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			hash.Write(data)
		}
		hash.Write([]byte{0})
		return nil
	})
	if err != nil {
		t.Fatalf("hash tree %s: %v", root, err)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func initGitSkill(t *testing.T, dir, remote string) {
	t.Helper()
	runGitForTest(t, dir, "init")
	runGitForTest(t, dir, "config", "user.email", "skill-manager@example.test")
	runGitForTest(t, dir, "config", "user.name", "Skill Manager Test")
	runGitForTest(t, dir, "config", "commit.gpgsign", "false")
	runGitForTest(t, dir, "config", "tag.gpgsign", "false")
	runGitForTest(t, dir, "config", "core.hooksPath", t.TempDir())
	runGitForTest(t, dir, "add", "SKILL.md")
	runGitForTest(t, dir, "commit", "-m", "Add skill")
	if remote != "" {
		runGitForTest(t, dir, "remote", "add", "origin", remote)
	}
}

func runGitForTest(t *testing.T, dir string, args ...string) {
	t.Helper()
	commandArgs := append([]string{"-C", dir}, args...)
	output, err := exec.Command("git", commandArgs...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(output))
	}
}

func gitOutputForTest(t *testing.T, dir string, args ...string) string {
	t.Helper()
	commandArgs := append([]string{"-C", dir}, args...)
	output, err := exec.Command("git", commandArgs...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(output))
	}
	return strings.TrimSpace(string(output))
}

func groupsByName(t *testing.T, output string) map[string]string {
	t.Helper()
	rows := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if strings.HasPrefix(line, "Group") {
			continue
		}
		fields := strings.Fields(line)
		switch {
		case strings.HasPrefix(line, "Claude plugin"):
			rows["Claude plugin"] = line
		case strings.HasPrefix(line, "Codex system"):
			rows["Codex system"] = line
		case strings.HasPrefix(line, "Skills CLI"):
			rows["Skills CLI"] = line
		case len(fields) > 0:
			rows[fields[0]] = line
		}
	}
	return rows
}

func assertGroupRow(t *testing.T, rows map[string]string, group, rowCount, claudeCounts, codexCounts, source string) {
	t.Helper()
	row, ok := rows[group]
	if !ok {
		t.Fatalf("groups output rows = %#v, want group %q", rows, group)
	}
	remainder := strings.TrimSpace(strings.TrimPrefix(row, group))
	fields := strings.Fields(remainder)
	if len(fields) < 10 {
		t.Fatalf("group row %q fields = %#v, want count, counts, and source", row, fields)
	}
	if fields[0] != rowCount {
		t.Fatalf("group row %q row count = %q, want %q", row, fields[0], rowCount)
	}
	if got := strings.Join(fields[1:5], " "); got != claudeCounts {
		t.Fatalf("group row %q Claude counts = %q, want %q", row, got, claudeCounts)
	}
	if got := strings.Join(fields[5:9], " "); got != codexCounts {
		t.Fatalf("group row %q Codex counts = %q, want %q", row, got, codexCounts)
	}
	if got := strings.Join(fields[9:], " "); got != source {
		t.Fatalf("group row %q source = %q, want %q", row, got, source)
	}
}
