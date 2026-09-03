package ops

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dees91/agent-skill-manager/internal/model"
	"github.com/dees91/agent-skill-manager/internal/paths"
	"github.com/dees91/agent-skill-manager/internal/state"
)

func TestSymlinkDisableEnableRoundTripMovesSymlinkItself(t *testing.T) {
	home := t.TempDir()
	p := paths.ForHome(home)
	sourceDir := filepath.Join(home, "source-skill")
	mkdirSkill(t, sourceDir)
	mkdirAll(t, p.ClaudeUserSkills)
	activePath := filepath.Join(p.ClaudeUserSkills, "linked")
	if err := os.Symlink(sourceDir, activePath); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	service := New(p)
	service.now = fixedNow

	disable, err := service.PlanDisable(model.ToolClaude, "linked")
	if err != nil {
		t.Fatalf("PlanDisable() error = %v", err)
	}
	if disable.EntryType != model.EntryTypeSymlink {
		t.Fatalf("EntryType = %q, want symlink", disable.EntryType)
	}
	if disable.SymlinkTarget != sourceDir {
		t.Fatalf("SymlinkTarget = %q, want %q", disable.SymlinkTarget, sourceDir)
	}

	result := service.Apply([]model.PlannedOperation{disable})
	assertApplySuccess(t, result, 1)

	assertMissing(t, activePath)
	assertSymlinkTarget(t, disable.ToPath, sourceDir)
	assertExists(t, sourceDir)

	manifest := loadManifest(t, p)
	entry, ok := manifest.Get(model.ToolClaude, "linked")
	if !ok {
		t.Fatal("manifest entry missing after disable")
	}
	if entry.OriginalPath != activePath || entry.DisabledPath != disable.ToPath || entry.EntryType != model.EntryTypeSymlink || entry.SymlinkTarget != sourceDir || !entry.DisabledAt.Equal(fixedNow().UTC()) {
		t.Fatalf("manifest entry = %#v, want disabled symlink metadata", entry)
	}

	enable, err := service.PlanEnable(model.ToolClaude, "linked")
	if err != nil {
		t.Fatalf("PlanEnable() error = %v", err)
	}
	result = service.Apply([]model.PlannedOperation{enable})
	assertApplySuccess(t, result, 1)

	assertSymlinkTarget(t, activePath, sourceDir)
	assertMissing(t, disable.ToPath)
	assertExists(t, sourceDir)
	manifest = loadManifest(t, p)
	if _, ok := manifest.Get(model.ToolClaude, "linked"); ok {
		t.Fatal("manifest entry still present after enable")
	}
}

func TestSymlinkRepoDisablePersistsGroup(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	home := t.TempDir()
	p := paths.ForHome(home)
	sourceDir := filepath.Join(home, "source-skill")
	mkdirSkill(t, sourceDir)
	initGitSkill(t, sourceDir, "https://github.com/android/skills.git")
	mkdirAll(t, p.ClaudeUserSkills)
	activePath := filepath.Join(p.ClaudeUserSkills, "agp-9-upgrade")
	if err := os.Symlink(sourceDir, activePath); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	service := New(p)
	service.now = fixedNow
	disable, err := service.PlanDisable(model.ToolClaude, "agp-9-upgrade")
	if err != nil {
		t.Fatalf("PlanDisable() error = %v", err)
	}
	if disable.Group != model.GroupLabel("android/skills") {
		t.Fatalf("disable Group = %q, want android/skills", disable.Group)
	}

	result := service.Apply([]model.PlannedOperation{disable})
	assertApplySuccess(t, result, 1)

	entry, ok := loadManifest(t, p).Get(model.ToolClaude, "agp-9-upgrade")
	if !ok {
		t.Fatal("manifest entry missing after disable")
	}
	if entry.Group != model.GroupLabel("android/skills") {
		t.Fatalf("manifest Group = %q, want android/skills", entry.Group)
	}
}

func TestDirectoryDisableEnableRoundTrip(t *testing.T) {
	home := t.TempDir()
	p := paths.ForHome(home)
	activePath := filepath.Join(p.CodexUserSkills, "local")
	mkdirSkill(t, activePath)

	service := New(p)
	disable, err := service.PlanDisable(model.ToolCodex, "local")
	if err != nil {
		t.Fatalf("PlanDisable() error = %v", err)
	}
	if disable.EntryType != model.EntryTypeDir {
		t.Fatalf("EntryType = %q, want dir", disable.EntryType)
	}
	if disable.Group != model.GroupLocal {
		t.Fatalf("Group = %q, want %q", disable.Group, model.GroupLocal)
	}

	result := service.Apply([]model.PlannedOperation{disable})
	assertApplySuccess(t, result, 1)
	assertMissing(t, activePath)
	assertExists(t, filepath.Join(disable.ToPath, "SKILL.md"))
	manifest := loadManifest(t, p)
	entry, ok := manifest.Get(model.ToolCodex, "local")
	if !ok {
		t.Fatal("manifest entry missing after directory disable")
	}
	if entry.OriginalPath != activePath || entry.DisabledPath != disable.ToPath || entry.EntryType != model.EntryTypeDir || entry.Source != model.SourceLocal || entry.Group != model.GroupLocal {
		t.Fatalf("manifest entry = %#v, want disabled directory metadata", entry)
	}

	enable, err := service.PlanEnable(model.ToolCodex, "local")
	if err != nil {
		t.Fatalf("PlanEnable() error = %v", err)
	}
	if enable.Group != model.GroupLocal {
		t.Fatalf("enable Group = %q, want %q", enable.Group, model.GroupLocal)
	}
	result = service.Apply([]model.PlannedOperation{enable})
	assertApplySuccess(t, result, 1)
	assertExists(t, filepath.Join(activePath, "SKILL.md"))
	assertMissing(t, disable.ToPath)
	manifest = loadManifest(t, p)
	if _, ok := manifest.Get(model.ToolCodex, "local"); ok {
		t.Fatal("manifest entry still present after directory enable")
	}
}

func TestMuseDirectoryDisableEnableRoundTrip(t *testing.T) {
	home := t.TempDir()
	p := paths.ForHome(home)
	activePath := filepath.Join(p.MuseUserSkills, "muse-local")
	mkdirSkill(t, activePath)

	service := New(p)
	disable, err := service.PlanDisable(model.ToolMuse, "muse-local")
	if err != nil {
		t.Fatalf("PlanDisable() error = %v", err)
	}
	if disable.EntryType != model.EntryTypeDir || disable.Group != model.GroupLocal {
		t.Fatalf("disable = %#v, want dir local", disable)
	}

	result := service.Apply([]model.PlannedOperation{disable})
	assertApplySuccess(t, result, 1)
	assertMissing(t, activePath)
	manifest := loadManifest(t, p)
	entry, ok := manifest.Get(model.ToolMuse, "muse-local")
	if !ok {
		t.Fatal("manifest entry missing after Muse disable")
	}
	if entry.OriginalPath != activePath || entry.DisabledPath != disable.ToPath {
		t.Fatalf("manifest entry = %#v, want Muse restore paths", entry)
	}

	enable, err := service.PlanEnable(model.ToolMuse, "muse-local")
	if err != nil {
		t.Fatalf("PlanEnable() error = %v", err)
	}
	result = service.Apply([]model.PlannedOperation{enable})
	assertApplySuccess(t, result, 1)
	assertExists(t, filepath.Join(activePath, "SKILL.md"))
	manifest = loadManifest(t, p)
	if _, ok := manifest.Get(model.ToolMuse, "muse-local"); ok {
		t.Fatal("manifest entry still present after Muse enable")
	}
}

func TestPlanDisableValidatesDestinationFree(t *testing.T) {
	home := t.TempDir()
	p := paths.ForHome(home)
	mkdirSkill(t, filepath.Join(p.ClaudeUserSkills, "edge-to-edge"))
	mkdirAll(t, filepath.Join(p.ClaudeDisabledDir, "edge-to-edge"))

	_, err := New(p).PlanDisable(model.ToolClaude, "edge-to-edge")
	if err == nil {
		t.Fatal("PlanDisable() error = nil, want error")
	}
}

func TestPlanEnableValidatesOriginalPathFree(t *testing.T) {
	home := t.TempDir()
	p := paths.ForHome(home)
	disabledPath := filepath.Join(p.CodexDisabledDir, "find-skills")
	originalPath := filepath.Join(p.CodexUserSkills, "find-skills")
	mkdirSkill(t, disabledPath)
	mkdirSkill(t, originalPath)
	store := state.New(p)
	if err := store.Save(state.Manifest{Disabled: []state.DisabledEntry{{
		Tool:         model.ToolCodex,
		SkillName:    "find-skills",
		OriginalPath: originalPath,
		DisabledPath: disabledPath,
		EntryType:    model.EntryTypeDir,
		Source:       model.SourceSkillsCLI,
	}}}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	_, err := New(p).PlanEnable(model.ToolCodex, "find-skills")
	if err == nil {
		t.Fatal("PlanEnable() error = nil, want conflict error")
	}
	if !strings.Contains(err.Error(), originalPath) && !strings.Contains(err.Error(), "destination") {
		t.Fatalf("PlanEnable() error = %v, want blocker path or destination context", err)
	}
	assertExists(t, filepath.Join(disabledPath, "SKILL.md"))
	assertExists(t, filepath.Join(originalPath, "SKILL.md"))
}

func TestSortOperationsForApplyDeterministic(t *testing.T) {
	operations := []model.PlannedOperation{
		{Kind: model.OperationEnable, Tool: model.ToolCodex, SkillName: "b"},
		{Kind: model.OperationDisable, Tool: model.ToolCodex, SkillName: "b"},
		{Kind: model.OperationEnable, Tool: model.ToolClaude, SkillName: "a"},
		{Kind: model.OperationDisable, Tool: model.ToolClaude, SkillName: "c"},
	}

	got := SortOperationsForApply(operations)

	want := []model.PlannedOperation{
		{Kind: model.OperationDisable, Tool: model.ToolClaude, SkillName: "c"},
		{Kind: model.OperationDisable, Tool: model.ToolCodex, SkillName: "b"},
		{Kind: model.OperationEnable, Tool: model.ToolClaude, SkillName: "a"},
		{Kind: model.OperationEnable, Tool: model.ToolCodex, SkillName: "b"},
	}
	for i := range want {
		if got[i].Kind != want[i].Kind || got[i].Tool != want[i].Tool || got[i].SkillName != want[i].SkillName {
			t.Fatalf("ordered[%d] = %#v, want %#v", i, got[i], want[i])
		}
	}
	if operations[0].Kind != model.OperationEnable || operations[0].Tool != model.ToolCodex {
		t.Fatalf("SortOperationsForApply mutated input: %#v", operations)
	}
}

func TestApplyStopsOnFirstFailureAndPersistsCompletedState(t *testing.T) {
	home := t.TempDir()
	p := paths.ForHome(home)
	mkdirSkill(t, filepath.Join(p.ClaudeUserSkills, "a-good"))
	mkdirSkill(t, filepath.Join(p.ClaudeUserSkills, "b-bad"))

	service := New(p)
	service.now = fixedNow
	aGood, err := service.PlanDisable(model.ToolClaude, "a-good")
	if err != nil {
		t.Fatalf("PlanDisable(a-good) error = %v", err)
	}
	bBad, err := service.PlanDisable(model.ToolClaude, "b-bad")
	if err != nil {
		t.Fatalf("PlanDisable(b-bad) error = %v", err)
	}
	mkdirAll(t, bBad.ToPath)

	result := service.Apply([]model.PlannedOperation{bBad, aGood})

	if result.Failed == nil {
		t.Fatal("Apply() Failed = nil, want failure")
	}
	if result.Failed.Operation.SkillName != "b-bad" {
		t.Fatalf("failed operation = %#v, want b-bad", result.Failed.Operation)
	}
	if len(result.Completed) != 1 || result.Completed[0].SkillName != "a-good" {
		t.Fatalf("Completed = %#v, want only a-good", result.Completed)
	}
	assertMissing(t, aGood.FromPath)
	assertExists(t, filepath.Join(aGood.ToPath, "SKILL.md"))
	assertExists(t, filepath.Join(bBad.FromPath, "SKILL.md"))

	manifest := loadManifest(t, p)
	if _, ok := manifest.Get(model.ToolClaude, "a-good"); !ok {
		t.Fatal("a-good missing from state after completed disable")
	}
	if _, ok := manifest.Get(model.ToolClaude, "b-bad"); ok {
		t.Fatal("b-bad present in state despite failed disable")
	}
}

func TestPlanDisableRejectsReadOnlySkill(t *testing.T) {
	home := t.TempDir()
	p := paths.ForHome(home)
	mkdirSkill(t, filepath.Join(p.CodexSystemSkills, "imagegen"))

	_, err := New(p).PlanDisable(model.ToolCodex, "imagegen")
	if err == nil {
		t.Fatal("PlanDisable(read-only) error = nil, want error")
	}
}

func TestPlanBatchPlansManyDisablesWithOneRepoMetadataLookup(t *testing.T) {
	home := t.TempDir()
	p := paths.ForHome(home)
	repoDir := filepath.Join(home, "repo")
	mkdirAll(t, filepath.Join(repoDir, ".git"))
	mkdirAll(t, p.ClaudeUserSkills)
	countPath := installFakeGitCounter(t)

	const skillCount = 8
	requests := make([]PlanRequest, 0, skillCount)
	for i := 0; i < skillCount; i++ {
		name := fmt.Sprintf("skill-%02d", i)
		sourceDir := filepath.Join(repoDir, name)
		mkdirSkill(t, sourceDir)
		if err := os.Symlink(sourceDir, filepath.Join(p.ClaudeUserSkills, name)); err != nil {
			t.Fatalf("create symlink: %v", err)
		}
		requests = append(requests, PlanRequest{
			Kind:      model.OperationDisable,
			Tool:      model.ToolClaude,
			SkillName: name,
		})
	}

	operations, err := New(p).PlanBatch(requests)
	if err != nil {
		t.Fatalf("PlanBatch() error = %v", err)
	}

	if len(operations) != skillCount {
		t.Fatalf("operations len = %d, want %d", len(operations), skillCount)
	}
	for _, operation := range operations {
		if operation.Source != model.SourceSymlinkRepo || operation.Group != model.GroupLabel("android/skills") {
			t.Fatalf("operation metadata = %#v, want symlink repo android/skills", operation)
		}
	}
	if got := fakeGitCallCount(t, countPath); got != 2 {
		t.Fatalf("git call count = %d, want 2 origin/commit lookups for one repo", got)
	}
}

func fixedNow() time.Time {
	return time.Date(2026, 5, 8, 12, 30, 0, 0, time.UTC)
}

func mkdirSkill(t *testing.T, dir string) {
	t.Helper()
	mkdirAll(t, dir)
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

func initGitSkill(t *testing.T, dir, remote string) {
	t.Helper()
	runGitForTest(t, dir, "init")
	runGitForTest(t, dir, "config", "user.email", "skill-manager@example.test")
	runGitForTest(t, dir, "config", "user.name", "Skill Manager Test")
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

func assertApplySuccess(t *testing.T, result ApplyResult, completed int) {
	t.Helper()
	if result.Failed != nil {
		t.Fatalf("Apply() failed: op=%#v err=%v", result.Failed.Operation, result.Failed.Err)
	}
	if len(result.Completed) != completed {
		t.Fatalf("Completed len = %d, want %d", len(result.Completed), completed)
	}
}

func assertSymlinkTarget(t *testing.T, path, want string) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("lstat %s: %v", path, err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("%s mode = %v, want symlink", path, info.Mode())
	}
	got, err := os.Readlink(path)
	if err != nil {
		t.Fatalf("readlink %s: %v", path, err)
	}
	if got != want {
		t.Fatalf("readlink %s = %q, want %q", path, got, want)
	}
}

func assertMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("lstat %s error = %v, want not exists", path, err)
	}
}

func assertExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
}

func loadManifest(t *testing.T, p paths.Paths) state.Manifest {
	t.Helper()
	manifest, err := state.New(p).Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	return manifest
}

func installFakeGitCounter(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	countPath := filepath.Join(dir, "count")
	scriptPath := filepath.Join(dir, "git")
	script := fmt.Sprintf(`#!/bin/sh
printf 'x\n' >> %q
if [ "$3" = "config" ] && [ "$4" = "--get" ] && [ "$5" = "remote.origin.url" ]; then
  echo "https://github.com/android/skills.git"
  exit 0
fi
if [ "$3" = "rev-parse" ] && [ "$4" = "--short" ] && [ "$5" = "HEAD" ]; then
  echo "abc1234"
  exit 0
fi
if [ "$3" = "rev-parse" ] && [ "$4" = "--show-toplevel" ]; then
  echo "$2"
  exit 0
fi
exit 1
`, countPath)
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake git: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return countPath
}

func fakeGitCallCount(t *testing.T, countPath string) int {
	t.Helper()
	data, err := os.ReadFile(countPath)
	if errors.Is(err, os.ErrNotExist) {
		return 0
	}
	if err != nil {
		t.Fatalf("read fake git count: %v", err)
	}
	return strings.Count(string(data), "\n")
}
