package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dees91/agent-skill-manager/internal/paths"
)

// An upstream repository that drops an installed skill blocks update; the CLI
// prints the exact uninstall --skill remedy, and running it unblocks update.
func TestRunUpdateRemedyRemovesSkillDroppedUpstream(t *testing.T) {
	const gitURL = "https://github.com/owner/drop-skill"
	source := createSourceRepo(t, "alpha", "beta")
	withGitInsteadOf(t, gitURL, source)
	p := paths.ForHome(t.TempDir())
	var installOut, installErr strings.Builder
	if code := RunWithPaths([]string{"install", gitURL, "--tool", "claude"}, &installOut, &installErr, p); code != 0 {
		t.Fatalf("install code=%d stderr=%q", code, installErr.String())
	}
	if err := os.RemoveAll(filepath.Join(source, "skills", "beta")); err != nil {
		t.Fatalf("remove beta upstream: %v", err)
	}
	runGitForTest(t, source, "add", "-A")
	runGitForTest(t, source, "commit", "-m", "Remove beta")

	var stdout, stderr strings.Builder
	if code := RunWithPaths([]string{"update", gitURL}, &stdout, &stderr, p); code == 0 {
		t.Fatalf("update code = 0, want blocked; stdout=%q", stdout.String())
	}
	remedy := "skill-manager uninstall " + gitURL + " --skill beta"
	if !strings.Contains(stderr.String(), remedy) {
		t.Fatalf("stderr = %q, want remedy %q", stderr.String(), remedy)
	}

	stdout.Reset()
	stderr.Reset()
	if code := RunWithPaths([]string{"uninstall", gitURL, "--skill", "beta", "--dry-run"}, &stdout, &stderr, p); code != 0 {
		t.Fatalf("dry-run code=%d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "would remove on claude/beta") || !strings.Contains(stdout.String(), "would keep checkout") {
		t.Fatalf("dry-run stdout = %q", stdout.String())
	}
	assertExists(t, filepath.Join(p.ClaudeUserSkills, "beta"))

	stdout.Reset()
	stderr.Reset()
	if code := RunWithPaths(strings.Fields(strings.TrimPrefix(remedy, "skill-manager ")), &stdout, &stderr, p); code != 0 {
		t.Fatalf("remedy code=%d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "removed beta from owner/drop-skill") {
		t.Fatalf("remedy stdout = %q", stdout.String())
	}
	assertMissing(t, filepath.Join(p.ClaudeUserSkills, "beta"))
	assertExists(t, filepath.Join(p.ClaudeUserSkills, "alpha"))

	stdout.Reset()
	stderr.Reset()
	if code := RunWithPaths([]string{"update", gitURL}, &stdout, &stderr, p); code != 0 {
		t.Fatalf("update after remedy code=%d stderr=%q", code, stderr.String())
	}
}

// When every installed skill is gone upstream, removal would refuse, so the
// remedy names a whole-source uninstall instead.
func TestRunUpdateRemedyNamesWholeUninstallWhenEverySkillDropped(t *testing.T) {
	const gitURL = "https://github.com/example/agent-skills"
	source := createSourceRepo(t, "alpha", "beta")
	withGitInsteadOf(t, gitURL, source)
	p := paths.ForHome(t.TempDir())
	var installOut, installErr strings.Builder
	if code := RunWithPaths([]string{"install", gitURL, "--tool", "claude", "--skill", "alpha"}, &installOut, &installErr, p); code != 0 {
		t.Fatalf("install code=%d stderr=%q", code, installErr.String())
	}
	if err := os.RemoveAll(filepath.Join(source, "skills", "alpha")); err != nil {
		t.Fatalf("remove alpha upstream: %v", err)
	}
	runGitForTest(t, source, "add", "-A")
	runGitForTest(t, source, "commit", "-m", "Remove alpha")

	var stdout, stderr strings.Builder
	if code := RunWithPaths([]string{"update", gitURL}, &stdout, &stderr, p); code == 0 {
		t.Fatal("update code = 0, want blocked")
	}
	if !strings.Contains(stderr.String(), `run "skill-manager uninstall `+gitURL+`" to uninstall the whole source`) || strings.Contains(stderr.String(), "--skill") {
		t.Fatalf("stderr = %q, want a whole-source uninstall remedy", stderr.String())
	}
}

func TestRunUninstallSkillParserErrors(t *testing.T) {
	for _, args := range [][]string{
		{"uninstall", "https://github.com/owner/repo", "--skill"},
		{"uninstall", "https://github.com/owner/repo", "--skill", ""},
		{"uninstall", "https://github.com/owner/repo", "--skill", "--dry-run"},
	} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			var stdout, stderr strings.Builder
			if code := RunWithPaths(args, &stdout, &stderr, paths.ForHome(t.TempDir())); code == 0 {
				t.Fatalf("RunWithPaths(%v) code = 0, want usage error", args)
			}
			if !strings.Contains(stderr.String(), "Run \"skill-manager help\"") {
				t.Fatalf("stderr = %q, want usage hint", stderr.String())
			}
		})
	}
}
