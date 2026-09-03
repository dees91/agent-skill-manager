package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dees91/agent-skill-manager/internal/model"
	"github.com/dees91/agent-skill-manager/internal/paths"
)

func setupExtendLocalSource(t *testing.T, p paths.Paths, dir string, skillNames ...string) string {
	t.Helper()
	source := filepath.Join(p.Home, "workspace", dir)
	for _, name := range skillNames {
		mkdirSkill(t, filepath.Join(source, "skills", name))
	}
	var installOut, installErr strings.Builder
	if code := RunWithPaths([]string{"install", source, "--tool", "claude"}, &installOut, &installErr, p); code != 0 {
		t.Fatalf("install code=%d stdout=%q stderr=%q", code, installOut.String(), installErr.String())
	}
	return source
}

func disableExtendSkill(t *testing.T, p paths.Paths, tool, name string) {
	t.Helper()
	var stdout, stderr strings.Builder
	if code := RunWithPaths([]string{"disable", "--tool", tool, name}, &stdout, &stderr, p); code != 0 {
		t.Fatalf("disable code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestRunExtendDryRunLeavesHomeAndStateUnchanged(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	setupExtendLocalSource(t, p, "local-pack", "alpha", "beta")
	disableExtendSkill(t, p, "claude", "beta")
	digestBefore := treeDigest(t, p.Home)
	stateBefore := readFile(t, p.StateFile)
	var stdout, stderr strings.Builder

	code := RunWithPaths([]string{"extend", "--tool", "muse", "--dry-run"}, &stdout, &stderr, p)

	if code != 0 {
		t.Fatalf("extend dry-run code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{
		"dry-run: extend to muse",
		"source local-pack (local path): would link 2, already installed 0, would disable 1, skipped 0",
		"would link muse/alpha:",
		"would disable muse/beta:",
		"would link 2 symlink(s) across 1 source(s); 0 already installed; 1 would be disabled; 0 skipped",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	assertMissing(t, filepath.Join(p.MuseUserSkills, "alpha"))
	if digestAfter := treeDigest(t, p.Home); digestAfter != digestBefore {
		t.Fatal("extend dry-run changed the home directory")
	}
	if stateAfter := readFile(t, p.StateFile); string(stateAfter) != string(stateBefore) {
		t.Fatal("extend dry-run changed state")
	}
}

func TestRunExtendApplyLinksAndMirrorsOff(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	setupExtendLocalSource(t, p, "local-pack", "alpha", "beta")
	disableExtendSkill(t, p, "claude", "beta")
	var stdout, stderr strings.Builder

	code := RunWithPaths([]string{"extend", "--tool", "muse"}, &stdout, &stderr, p)

	if code != 0 {
		t.Fatalf("extend code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{
		"extended local-pack: created 2 symlink(s); 0 already installed; 1 disabled; 0 skipped",
		"extended 2 symlink(s) across 1 source(s); 0 already installed; 1 disabled; 0 skipped",
		"start a new Claude/Codex/Muse session",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}
	assertSymlink(t, filepath.Join(p.MuseUserSkills, "alpha"))
	assertMissing(t, filepath.Join(p.MuseUserSkills, "beta"))
	assertExists(t, filepath.Join(p.MuseDisabledDir, "beta", "SKILL.md"))
	manifest := loadState(t, p)
	entry, ok := manifest.Get(model.ToolMuse, "beta")
	if !ok || entry.OriginalPath != filepath.Join(p.MuseUserSkills, "beta") {
		t.Fatalf("muse beta disabled entry = %#v, %v", entry, ok)
	}
	canonical, err := filepath.EvalSymlinks(filepath.Join(p.Home, "workspace", "local-pack"))
	if err != nil {
		t.Fatalf("EvalSymlinks() error = %v", err)
	}
	source, ok := manifest.GetLocalSource(canonical)
	if !ok {
		t.Fatal("local source manifest entry missing")
	}
	for _, skill := range source.InstalledSkills {
		found := false
		for _, tool := range skill.Tools {
			if tool == model.ToolMuse {
				found = true
			}
		}
		if !found {
			t.Fatalf("installed tools for %s = %#v, want muse recorded", skill.Name, skill.Tools)
		}
	}

	stdout.Reset()
	stderr.Reset()
	if code := RunWithPaths([]string{"extend", "--tool", "muse"}, &stdout, &stderr, p); code != 0 {
		t.Fatalf("rerun code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "created 0 symlink(s)") {
		t.Fatalf("rerun stdout = %q, want created 0", stdout.String())
	}
}

func TestRunExtendStopsAtFirstBlockedSource(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	setupExtendLocalSource(t, p, "local-pack-a", "alpha")
	setupExtendLocalSource(t, p, "local-pack-b", "beta")
	if err := os.MkdirAll(filepath.Join(p.MuseUserSkills, "beta"), 0o755); err != nil {
		t.Fatalf("mkdir blocker: %v", err)
	}
	var stdout, stderr strings.Builder

	code := RunWithPaths([]string{"extend", "--tool", "muse"}, &stdout, &stderr, p)

	if code == 0 {
		t.Fatalf("extend code=0 stdout=%q stderr=%q, want blocked failure", stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "extended local-pack-a: created 1 symlink(s)") {
		t.Fatalf("stdout = %q, want first-source prefix", stdout.String())
	}
	if !strings.Contains(stderr.String(), "error: extend --tool muse failed for source local-pack-b:") {
		t.Fatalf("stderr = %q, want extend local-pack-b failure", stderr.String())
	}
	assertSymlink(t, filepath.Join(p.MuseUserSkills, "alpha"))
}

func TestRunExtendGitSource(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	source := createSourceRepo(t, "alpha")
	withGitInsteadOf(t, "https://github.com/owner/skills", source)
	var installOut, installErr strings.Builder
	if code := RunWithPaths([]string{"install", "https://github.com/owner/skills.git", "--tool", "claude"}, &installOut, &installErr, p); code != 0 {
		t.Fatalf("install code=%d stdout=%q stderr=%q", code, installOut.String(), installErr.String())
	}
	var stdout, stderr strings.Builder

	code := RunWithPaths([]string{"extend", "--tool", "muse"}, &stdout, &stderr, p)

	if code != 0 {
		t.Fatalf("extend code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "extended owner/skills: created 1 symlink(s)") {
		t.Fatalf("stdout = %q, want extended owner/skills", stdout.String())
	}
	assertSymlink(t, filepath.Join(p.MuseUserSkills, "alpha"))
}

func TestRunExtendNoSourcesRecorded(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	var stdout, stderr strings.Builder

	code := RunWithPaths([]string{"extend", "--tool", "muse"}, &stdout, &stderr, p)

	if code != 0 {
		t.Fatalf("extend code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "No managed sources recorded.") {
		t.Fatalf("stdout = %q, want no-sources message", stdout.String())
	}
}

func TestRunExtendUsageErrors(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	cases := [][]string{
		{},
		{"--dry-run"},
		{"--tool"},
		{"--tool", "both"},
		{"--tool", "all"},
		{"--tool", "grok"},
		{"--tool", "claude", "extra"},
		{"--tool", "claude", "--tool", "codex"},
	}
	for _, args := range cases {
		var stdout, stderr strings.Builder
		if code := RunWithPaths(append([]string{"extend"}, args...), &stdout, &stderr, p); code == 0 {
			t.Fatalf("extend %q code=0, want usage error", args)
		} else if !strings.Contains(stderr.String(), "expected extend --tool <tool> [--dry-run]") {
			t.Fatalf("extend %q stderr=%q, want usage message", args, stderr.String())
		}
	}
}
