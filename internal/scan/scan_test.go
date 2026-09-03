package scan

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

func TestManagedScansSymlinkAndDirectorySkills(t *testing.T) {
	home := t.TempDir()
	p := paths.ForHome(home)
	sourceDir := filepath.Join(home, "source-skill")
	mkdirSkill(t, sourceDir)
	mkdirSkill(t, filepath.Join(p.CodexUserSkills, "local-skill"))
	mkdirAll(t, p.ClaudeUserSkills)
	symlinkPath := filepath.Join(p.ClaudeUserSkills, "linked-skill")
	if err := os.Symlink(sourceDir, symlinkPath); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	got, err := New(p).Managed()
	if err != nil {
		t.Fatalf("Managed() error = %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("Managed() returned %d skills, want 2: %#v", len(got), got)
	}

	linked := findSkill(t, got, model.ToolClaude, "linked-skill")
	if linked.EntryType != model.EntryTypeSymlink {
		t.Fatalf("linked EntryType = %q, want %q", linked.EntryType, model.EntryTypeSymlink)
	}
	if linked.SymlinkTarget != sourceDir {
		t.Fatalf("linked SymlinkTarget = %q, want %q", linked.SymlinkTarget, sourceDir)
	}
	if linked.ActivePath != symlinkPath {
		t.Fatalf("linked ActivePath = %q, want %q", linked.ActivePath, symlinkPath)
	}
	if linked.SkillFilePath != filepath.Join(symlinkPath, "SKILL.md") {
		t.Fatalf("linked SkillFilePath = %q, want symlink-relative SKILL.md", linked.SkillFilePath)
	}
	if linked.State != model.SkillStateOn || linked.Source != model.SourceUnknown || linked.ReadOnly {
		t.Fatalf("linked state/source/readonly = %q/%q/%v, want ON/unknown/false", linked.State, linked.Source, linked.ReadOnly)
	}
	if linked.Group != model.GroupUnknown {
		t.Fatalf("linked Group = %q, want %q", linked.Group, model.GroupUnknown)
	}

	local := findSkill(t, got, model.ToolCodex, "local-skill")
	if local.EntryType != model.EntryTypeDir {
		t.Fatalf("local EntryType = %q, want %q", local.EntryType, model.EntryTypeDir)
	}
	if local.Source != model.SourceLocal {
		t.Fatalf("local Source = %q, want %q", local.Source, model.SourceLocal)
	}
	if local.Group != model.GroupLocal {
		t.Fatalf("local Group = %q, want %q", local.Group, model.GroupLocal)
	}
	if local.SymlinkTarget != "" {
		t.Fatalf("local SymlinkTarget = %q, want empty", local.SymlinkTarget)
	}
}

func TestManagedScansMuseUserSkills(t *testing.T) {
	home := t.TempDir()
	p := paths.ForHome(home)
	mkdirSkill(t, filepath.Join(p.MuseUserSkills, "muse-only"))

	got, err := New(p).Managed()
	if err != nil {
		t.Fatalf("Managed() error = %v", err)
	}
	skill := findSkill(t, got, model.ToolMuse, "muse-only")
	if skill.EntryType != model.EntryTypeDir || skill.Source != model.SourceLocal || skill.Group != model.GroupLocal {
		t.Fatalf("muse skill = %#v, want dir local/local", skill)
	}
	if skill.State != model.SkillStateOn || skill.ReadOnly {
		t.Fatalf("muse state/readonly = %q/%v, want ON/false", skill.State, skill.ReadOnly)
	}

	rows := RowsFromSkills(got)
	if len(rows) != 1 || rows[0].Muse == nil || rows[0].Claude != nil || rows[0].Codex != nil {
		t.Fatalf("rows = %#v, want one Muse-only row", rows)
	}
}

func TestManagedClassifiesManifestOwnedLocalPathLinks(t *testing.T) {
	home := t.TempDir()
	p := paths.ForHome(home)
	sourceRoot := filepath.Join(home, "workspace", "sample-pack")
	skillDir := filepath.Join(sourceRoot, "skills", "alpha")
	mkdirSkill(t, skillDir)
	for _, tool := range model.Tools() {
		dir, _ := p.UserSkillsDirFor(tool)
		mkdirAll(t, dir)
		if err := os.Symlink(skillDir, filepath.Join(dir, "alpha")); err != nil {
			t.Fatalf("create %s local source link: %v", tool, err)
		}
	}
	mkdirAll(t, filepath.Dir(p.AgentsSkillLock))
	writeFile(t, p.AgentsSkillLock, `{"skills":{"alpha":{"source":"external/skills"}}}`)
	saveState(t, p, state.Manifest{LocalSources: []state.LocalSourceEntry{{
		OriginalPath: sourceRoot, CanonicalPath: sourceRoot, Group: model.GroupLabel("sample-pack"),
		InstalledSkills: []state.InstalledSkillEntry{{Name: "alpha", RelativePath: "skills/alpha", Tools: model.Tools()}},
	}}})

	got, err := New(p).Managed()
	if err != nil {
		t.Fatalf("Managed() error = %v", err)
	}
	for _, tool := range model.Tools() {
		skill := findSkill(t, got, tool, "alpha")
		if skill.Source != model.SourceLocalPath || skill.Group != model.GroupLabel("sample-pack") {
			t.Fatalf("%s source/group = %q/%q, want local path/sample-pack", tool, skill.Source, skill.Group)
		}
	}
}

func TestManagedAppliesMetadata(t *testing.T) {
	home := t.TempDir()
	p := paths.ForHome(home)
	skillDir := filepath.Join(p.ClaudeUserSkills, "frontmatter-skill")
	mkdirAll(t, skillDir)
	writeFile(t, filepath.Join(skillDir, "SKILL.md"), `---
name: "Frontmatter Name"
description: "Useful description"
---
`)

	got, err := New(p).Managed()
	if err != nil {
		t.Fatalf("Managed() error = %v", err)
	}

	skill := findSkill(t, got, model.ToolClaude, "frontmatter-skill")
	if skill.DisplayName != "Frontmatter Name" {
		t.Fatalf("DisplayName = %q, want Frontmatter Name", skill.DisplayName)
	}
	if skill.Description != "Useful description" {
		t.Fatalf("Description = %q, want Useful description", skill.Description)
	}
}

func TestManagedLabelsCodexLockfileSkillAsSkillsCLI(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	home := t.TempDir()
	p := paths.ForHome(home)
	sourceDir := filepath.Join(home, "find-skills-source")
	mkdirSkill(t, sourceDir)
	runGitForTest(t, sourceDir, "init")
	runGitForTest(t, sourceDir, "config", "user.email", "skill-manager@example.test")
	runGitForTest(t, sourceDir, "config", "user.name", "Skill Manager Test")
	runGitForTest(t, sourceDir, "add", "SKILL.md")
	runGitForTest(t, sourceDir, "commit", "-m", "Add skill")
	runGitForTest(t, sourceDir, "remote", "add", "origin", "https://example.test/find-skills.git")
	mkdirAll(t, p.CodexUserSkills)
	if err := os.Symlink(sourceDir, filepath.Join(p.CodexUserSkills, "find-skills")); err != nil {
		t.Fatalf("create symlink: %v", err)
	}
	writeFile(t, p.AgentsSkillLock, `{
  "skills": {
    "find-skills": {
      "source": "vercel-labs/skills",
      "skillPath": "skills/find-skills/SKILL.md"
    }
  }
}`)

	got, err := New(p).Managed()
	if err != nil {
		t.Fatalf("Managed() error = %v", err)
	}

	skill := findSkill(t, got, model.ToolCodex, "find-skills")
	if skill.EntryType != model.EntryTypeSymlink {
		t.Fatalf("EntryType = %q, want symlink", skill.EntryType)
	}
	if skill.Source != model.SourceSkillsCLI {
		t.Fatalf("Source = %q, want %q", skill.Source, model.SourceSkillsCLI)
	}
	if skill.Group != model.GroupSkillsCLI {
		t.Fatalf("Group = %q, want %q", skill.Group, model.GroupSkillsCLI)
	}
	if skill.RepoOrigin != "https://example.test/find-skills.git" {
		t.Fatalf("RepoOrigin = %q, want remote origin", skill.RepoOrigin)
	}
	if skill.RepoCommit == "" {
		t.Fatal("RepoCommit is empty, want short commit")
	}
}

func TestManagedLabelsDirectorySkillFromCodexLockfileAsSkillsCLI(t *testing.T) {
	home := t.TempDir()
	p := paths.ForHome(home)
	mkdirSkill(t, filepath.Join(p.CodexUserSkills, "find-skills"))
	writeFile(t, p.AgentsSkillLock, `{
  "installedSkills": [
    {
      "name": "find-skills"
    }
  ]
}`)

	got, err := New(p).Managed()
	if err != nil {
		t.Fatalf("Managed() error = %v", err)
	}

	skill := findSkill(t, got, model.ToolCodex, "find-skills")
	if skill.EntryType != model.EntryTypeDir {
		t.Fatalf("EntryType = %q, want dir", skill.EntryType)
	}
	if skill.Source != model.SourceSkillsCLI {
		t.Fatalf("Source = %q, want %q", skill.Source, model.SourceSkillsCLI)
	}
	if skill.Group != model.GroupSkillsCLI {
		t.Fatalf("Group = %q, want %q", skill.Group, model.GroupSkillsCLI)
	}
}

func TestManagedLabelsSymlinkGitRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	home := t.TempDir()
	p := paths.ForHome(home)
	sourceDir := filepath.Join(home, "repo-skill")
	mkdirSkill(t, sourceDir)
	runGitForTest(t, sourceDir, "init")
	runGitForTest(t, sourceDir, "config", "user.email", "skill-manager@example.test")
	runGitForTest(t, sourceDir, "config", "user.name", "Skill Manager Test")
	runGitForTest(t, sourceDir, "add", "SKILL.md")
	runGitForTest(t, sourceDir, "commit", "-m", "Add skill")
	runGitForTest(t, sourceDir, "remote", "add", "origin", "https://example.test/repo.git")

	mkdirAll(t, p.ClaudeUserSkills)
	if err := os.Symlink(sourceDir, filepath.Join(p.ClaudeUserSkills, "repo-skill")); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	got, err := New(p).Managed()
	if err != nil {
		t.Fatalf("Managed() error = %v", err)
	}

	skill := findSkill(t, got, model.ToolClaude, "repo-skill")
	if skill.Source != model.SourceSymlinkRepo {
		t.Fatalf("Source = %q, want %q", skill.Source, model.SourceSymlinkRepo)
	}
	if skill.RepoOrigin != "https://example.test/repo.git" {
		t.Fatalf("RepoOrigin = %q, want remote origin", skill.RepoOrigin)
	}
	if skill.Group != model.GroupLabel("example.test/repo") {
		t.Fatalf("Group = %q, want example.test/repo", skill.Group)
	}
	if skill.RepoCommit == "" {
		t.Fatal("RepoCommit is empty, want short commit")
	}
}

func TestManagedGroupsGitHubHTTPSRemote(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	home := t.TempDir()
	p := paths.ForHome(home)
	sourceDir := filepath.Join(home, "android-skill")
	mkdirSkill(t, sourceDir)
	initGitSkill(t, sourceDir, "https://github.com/android/skills.git")

	mkdirAll(t, p.ClaudeUserSkills)
	if err := os.Symlink(sourceDir, filepath.Join(p.ClaudeUserSkills, "agp-9-upgrade")); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	got, err := New(p).Managed()
	if err != nil {
		t.Fatalf("Managed() error = %v", err)
	}

	skill := findSkill(t, got, model.ToolClaude, "agp-9-upgrade")
	if skill.Group != model.GroupLabel("android/skills") {
		t.Fatalf("Group = %q, want android/skills", skill.Group)
	}
}

func TestManagedGroupsGitHubSSHRemote(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	home := t.TempDir()
	p := paths.ForHome(home)
	sourceDir := filepath.Join(home, "compose-skill")
	mkdirSkill(t, sourceDir)
	initGitSkill(t, sourceDir, "git@github.com:skydoves/compose-performance-skills.git")

	mkdirAll(t, p.CodexUserSkills)
	if err := os.Symlink(sourceDir, filepath.Join(p.CodexUserSkills, "optimizing-lazy-layouts")); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	got, err := New(p).Managed()
	if err != nil {
		t.Fatalf("Managed() error = %v", err)
	}

	skill := findSkill(t, got, model.ToolCodex, "optimizing-lazy-layouts")
	if skill.Group != model.GroupLabel("skydoves/compose-performance-skills") {
		t.Fatalf("Group = %q, want skydoves/compose-performance-skills", skill.Group)
	}
}

func TestManagedGroupsNonGitHubSSHRemote(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	home := t.TempDir()
	p := paths.ForHome(home)
	sourceDir := filepath.Join(home, "gitlab-skill")
	mkdirSkill(t, sourceDir)
	initGitSkill(t, sourceDir, "git@gitlab.example:platform/skills.git")

	mkdirAll(t, p.ClaudeUserSkills)
	if err := os.Symlink(sourceDir, filepath.Join(p.ClaudeUserSkills, "gitlab-skill")); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	got, err := New(p).Managed()
	if err != nil {
		t.Fatalf("Managed() error = %v", err)
	}

	skill := findSkill(t, got, model.ToolClaude, "gitlab-skill")
	if skill.Group != model.GroupLabel("gitlab.example/platform/skills") {
		t.Fatalf("Group = %q, want gitlab.example/platform/skills", skill.Group)
	}
}

func TestManagedGroupsGitRepoWithoutRemote(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	home := t.TempDir()
	p := paths.ForHome(home)
	sourceDir := filepath.Join(home, "repo-root-skill")
	mkdirSkill(t, sourceDir)
	initGitSkill(t, sourceDir, "")

	mkdirAll(t, p.ClaudeUserSkills)
	if err := os.Symlink(sourceDir, filepath.Join(p.ClaudeUserSkills, "repo-root-skill")); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	got, err := New(p).Managed()
	if err != nil {
		t.Fatalf("Managed() error = %v", err)
	}

	skill := findSkill(t, got, model.ToolClaude, "repo-root-skill")
	if skill.Group != model.GroupLabel("repo-root-skill") {
		t.Fatalf("Group = %q, want repo-root-skill", skill.Group)
	}
}

func TestReadOnlyScansCodexSystemSkills(t *testing.T) {
	home := t.TempDir()
	p := paths.ForHome(home)
	mkdirSkill(t, filepath.Join(p.CodexSystemSkills, "imagegen"))
	mkdirAll(t, filepath.Join(p.CodexSystemSkills, "invalid"))

	got, err := New(p).ReadOnly()
	if err != nil {
		t.Fatalf("ReadOnly() error = %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("ReadOnly() returned %d skills, want 1: %#v", len(got), got)
	}
	skill := findSkill(t, got, model.ToolCodex, "imagegen")
	if skill.State != model.SkillStateReadOnly {
		t.Fatalf("State = %q, want %q", skill.State, model.SkillStateReadOnly)
	}
	if skill.Source != model.SourceCodexSystem {
		t.Fatalf("Source = %q, want %q", skill.Source, model.SourceCodexSystem)
	}
	if skill.Group != model.GroupCodexSystem {
		t.Fatalf("Group = %q, want %q", skill.Group, model.GroupCodexSystem)
	}
	if !skill.ReadOnly {
		t.Fatal("ReadOnly = false, want true")
	}
	if skill.Toggleable() {
		t.Fatal("Toggleable() = true, want false")
	}
}

func TestReadOnlyScansClaudePluginCacheSkills(t *testing.T) {
	home := t.TempDir()
	p := paths.ForHome(home)
	pluginSkillDir := filepath.Join(p.ClaudePluginCache, "github", "owner", "plugin", "skills", "codex-cli-runtime")
	mkdirSkill(t, pluginSkillDir)
	mkdirSkill(t, filepath.Join(p.ClaudePluginCache, "github", "owner", "other-plugin", "skills", "codex-cli-runtime"))
	mkdirSkill(t, filepath.Join(p.ClaudePluginCache, "github", "owner", "plugin", "not-skills", "ignored"))

	got, err := New(p).ReadOnly()
	if err != nil {
		t.Fatalf("ReadOnly() error = %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("ReadOnly() returned %d skills, want 2: %#v", len(got), got)
	}
	for _, skill := range got {
		if skill.Tool != model.ToolClaude || skill.Name != "codex-cli-runtime" {
			t.Fatalf("skill = %#v, want Claude codex-cli-runtime", skill)
		}
		if skill.State != model.SkillStateReadOnly {
			t.Fatalf("State = %q, want %q", skill.State, model.SkillStateReadOnly)
		}
		if skill.Source != model.SourceClaudePlugin {
			t.Fatalf("Source = %q, want %q", skill.Source, model.SourceClaudePlugin)
		}
		if skill.Group != model.GroupClaudePlugin {
			t.Fatalf("Group = %q, want %q", skill.Group, model.GroupClaudePlugin)
		}
		if !skill.ReadOnly {
			t.Fatal("ReadOnly = false, want true")
		}
		if !hasPathPrefix(skill.ActivePath, p.ClaudePluginCache) {
			t.Fatalf("ActivePath = %q, want under %q", skill.ActivePath, p.ClaudePluginCache)
		}
		if skill.Toggleable() {
			t.Fatal("Toggleable() = true, want false")
		}
	}
}

func TestReadOnlyMissingBaseDirectoriesAreEmpty(t *testing.T) {
	p := paths.ForHome(t.TempDir())

	got, err := New(p).ReadOnly()
	if err != nil {
		t.Fatalf("ReadOnly() error = %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("ReadOnly() returned %d skills, want 0: %#v", len(got), got)
	}
}

func TestDisabledScansOffAndConflictEntries(t *testing.T) {
	home := t.TempDir()
	p := paths.ForHome(home)
	offDisabledPath := filepath.Join(p.ClaudeDisabledDir, "off-skill")
	conflictDisabledPath := filepath.Join(p.CodexDisabledDir, "conflict-skill")
	conflictOriginalPath := filepath.Join(p.CodexUserSkills, "conflict-skill")
	mkdirAll(t, offDisabledPath)
	writeFile(t, filepath.Join(offDisabledPath, "SKILL.md"), `---
name: "Off Skill"
description: "Disabled description"
---
`)
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
			Source:       "",
		},
		{
			Tool:         model.Tool("unknown"),
			SkillName:    "ignored",
			OriginalPath: "/ignored",
			DisabledPath: "/ignored",
			EntryType:    model.EntryTypeDir,
		},
	}})

	got, err := New(p).Disabled()
	if err != nil {
		t.Fatalf("Disabled() error = %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("Disabled() returned %d skills, want 2: %#v", len(got), got)
	}
	off := findSkill(t, got, model.ToolClaude, "off-skill")
	if off.State != model.SkillStateOff {
		t.Fatalf("off State = %q, want %q", off.State, model.SkillStateOff)
	}
	if off.DisabledPath != offDisabledPath {
		t.Fatalf("off DisabledPath = %q, want %q", off.DisabledPath, offDisabledPath)
	}
	if off.DisplayName != "Off Skill" {
		t.Fatalf("off DisplayName = %q, want Off Skill", off.DisplayName)
	}
	if off.Description != "Disabled description" {
		t.Fatalf("off Description = %q, want Disabled description", off.Description)
	}
	if off.SkillFilePath != filepath.Join(offDisabledPath, "SKILL.md") {
		t.Fatalf("off SkillFilePath = %q, want disabled SKILL.md", off.SkillFilePath)
	}
	if off.Conflict != nil {
		t.Fatalf("off Conflict = %#v, want nil", off.Conflict)
	}
	if off.Group != model.GroupLocal {
		t.Fatalf("off Group = %q, want %q", off.Group, model.GroupLocal)
	}

	conflict := findSkill(t, got, model.ToolCodex, "conflict-skill")
	if conflict.State != model.SkillStateConflict {
		t.Fatalf("conflict State = %q, want %q", conflict.State, model.SkillStateConflict)
	}
	if conflict.Source != model.SourceUnknown {
		t.Fatalf("conflict Source = %q, want unknown", conflict.Source)
	}
	if conflict.Group != model.GroupUnknown {
		t.Fatalf("conflict Group = %q, want %q", conflict.Group, model.GroupUnknown)
	}
	if conflict.Conflict == nil {
		t.Fatal("conflict Conflict = nil, want details")
	}
	if conflict.Conflict.OriginalPath != conflictOriginalPath ||
		conflict.Conflict.DisabledPath != conflictDisabledPath ||
		conflict.Conflict.BlockerPath != conflictOriginalPath {
		t.Fatalf("Conflict = %#v, want original/disabled/blocker paths", conflict.Conflict)
	}
}

func TestDisabledScansSymlinkRepoMetadata(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	home := t.TempDir()
	p := paths.ForHome(home)
	sourceDir := filepath.Join(home, "repo-skill-source")
	mkdirAll(t, sourceDir)
	writeFile(t, filepath.Join(sourceDir, "SKILL.md"), `---
name: "Repo Skill"
description: "Repo description"
---
`)
	runGitForTest(t, sourceDir, "init")
	runGitForTest(t, sourceDir, "config", "user.email", "skill-manager@example.test")
	runGitForTest(t, sourceDir, "config", "user.name", "Skill Manager Test")
	runGitForTest(t, sourceDir, "add", "SKILL.md")
	runGitForTest(t, sourceDir, "commit", "-m", "Add skill")
	runGitForTest(t, sourceDir, "remote", "add", "origin", "https://example.test/disabled-repo.git")

	disabledPath := filepath.Join(p.ClaudeDisabledDir, "repo-skill")
	mkdirAll(t, filepath.Dir(disabledPath))
	if err := os.Symlink(sourceDir, disabledPath); err != nil {
		t.Fatalf("create symlink: %v", err)
	}
	saveState(t, p, state.Manifest{Disabled: []state.DisabledEntry{
		{
			Tool:          model.ToolClaude,
			SkillName:     "repo-skill",
			OriginalPath:  filepath.Join(p.ClaudeUserSkills, "repo-skill"),
			DisabledPath:  disabledPath,
			EntryType:     model.EntryTypeSymlink,
			SymlinkTarget: sourceDir,
			Source:        model.SourceSymlinkRepo,
		},
	}})

	got, err := New(p).Disabled()
	if err != nil {
		t.Fatalf("Disabled() error = %v", err)
	}

	skill := findSkill(t, got, model.ToolClaude, "repo-skill")
	if skill.DisplayName != "Repo Skill" {
		t.Fatalf("DisplayName = %q, want Repo Skill", skill.DisplayName)
	}
	if skill.Description != "Repo description" {
		t.Fatalf("Description = %q, want Repo description", skill.Description)
	}
	if skill.SkillFilePath != filepath.Join(disabledPath, "SKILL.md") {
		t.Fatalf("SkillFilePath = %q, want disabled symlink SKILL.md", skill.SkillFilePath)
	}
	if skill.RepoOrigin != "https://example.test/disabled-repo.git" {
		t.Fatalf("RepoOrigin = %q, want remote origin", skill.RepoOrigin)
	}
	if skill.Group != model.GroupLabel("example.test/disabled-repo") {
		t.Fatalf("Group = %q, want example.test/disabled-repo", skill.Group)
	}
	if skill.RepoCommit == "" {
		t.Fatal("RepoCommit is empty, want short commit")
	}
}

func TestDisabledPreservesGroupFromState(t *testing.T) {
	home := t.TempDir()
	p := paths.ForHome(home)
	disabledPath := filepath.Join(p.CodexDisabledDir, "stored-group")
	mkdirSkill(t, disabledPath)
	saveState(t, p, state.Manifest{Disabled: []state.DisabledEntry{
		{
			Tool:         model.ToolCodex,
			SkillName:    "stored-group",
			OriginalPath: filepath.Join(p.CodexUserSkills, "stored-group"),
			DisabledPath: disabledPath,
			EntryType:    model.EntryTypeDir,
			Source:       model.SourceLocal,
			Group:        model.GroupLabel("stored/custom"),
		},
	}})

	got, err := New(p).Disabled()
	if err != nil {
		t.Fatalf("Disabled() error = %v", err)
	}

	skill := findSkill(t, got, model.ToolCodex, "stored-group")
	if skill.Group != model.GroupLabel("stored/custom") {
		t.Fatalf("Group = %q, want stored/custom", skill.Group)
	}
}

func TestManagedHidesEntriesWithoutSkillMD(t *testing.T) {
	home := t.TempDir()
	p := paths.ForHome(home)
	mkdirAll(t, filepath.Join(p.ClaudeUserSkills, "invalid-dir"))
	invalidTarget := filepath.Join(home, "invalid-target")
	mkdirAll(t, invalidTarget)
	mkdirAll(t, p.CodexUserSkills)
	if err := os.Symlink(invalidTarget, filepath.Join(p.CodexUserSkills, "invalid-link")); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	got, err := New(p).Managed()
	if err != nil {
		t.Fatalf("Managed() error = %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("Managed() returned %d skills, want 0: %#v", len(got), got)
	}
}

func TestManagedMissingBaseDirectoriesAreEmpty(t *testing.T) {
	p := paths.ForHome(t.TempDir())

	got, err := New(p).Managed()
	if err != nil {
		t.Fatalf("Managed() error = %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("Managed() returned %d skills, want 0: %#v", len(got), got)
	}
}

func TestManagedBasePathFileReturnsError(t *testing.T) {
	home := t.TempDir()
	p := paths.ForHome(home)
	writeFile(t, p.ClaudeUserSkills, "not a directory")

	_, err := New(p).Managed()
	if err == nil {
		t.Fatal("Managed() error = nil, want error")
	}
}

func TestRowsFromSkillsGroupsSameNameAcrossTools(t *testing.T) {
	skills := []model.ToolSkill{
		{Tool: model.ToolCodex, Name: "shared", ActivePath: "/codex/shared", Source: model.SourceSymlinkRepo, Group: model.GroupLabel("android/skills")},
		{Tool: model.ToolClaude, Name: "shared", ActivePath: "/claude/shared", Source: model.SourceSymlinkRepo, Group: model.GroupLabel("android/skills")},
		{Tool: model.ToolClaude, Name: "alpha", ActivePath: "/claude/alpha", Source: model.SourceLocal, Group: model.GroupLocal},
	}

	rows := RowsFromSkills(skills)

	if len(rows) != 2 {
		t.Fatalf("RowsFromSkills() returned %d rows, want 2: %#v", len(rows), rows)
	}
	if rows[0].Name != "alpha" || rows[1].Name != "shared" {
		t.Fatalf("row order = %q, %q; want alpha, shared", rows[0].Name, rows[1].Name)
	}
	shared := rows[1]
	if shared.Claude == nil || shared.Codex == nil {
		t.Fatalf("shared row = %#v, want both tool cells", shared)
	}
	if shared.Claude.ActivePath != "/claude/shared" {
		t.Fatalf("shared Claude path = %q, want /claude/shared", shared.Claude.ActivePath)
	}
	if shared.Codex.ActivePath != "/codex/shared" {
		t.Fatalf("shared Codex path = %q, want /codex/shared", shared.Codex.ActivePath)
	}
	if shared.Group != model.GroupLabel("android/skills") {
		t.Fatalf("shared Group = %q, want android/skills", shared.Group)
	}
}

func TestRowsFromSkillsGroupMergeRules(t *testing.T) {
	tests := []struct {
		name  string
		left  model.GroupLabel
		right model.GroupLabel
		want  model.GroupLabel
	}{
		{name: "same known", left: model.GroupLabel("android/skills"), right: model.GroupLabel("android/skills"), want: model.GroupLabel("android/skills")},
		{name: "known plus unknown", left: model.GroupLabel("android/skills"), right: model.GroupUnknown, want: model.GroupLabel("android/skills")},
		{name: "known plus empty", left: model.GroupLabel("android/skills"), right: "", want: model.GroupLabel("android/skills")},
		{name: "mixed known", left: model.GroupLabel("android/skills"), right: model.GroupLocal, want: model.GroupUnknown},
		{name: "empty plus unknown", left: "", right: model.GroupUnknown, want: model.GroupUnknown},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rows := RowsFromSkills([]model.ToolSkill{
				{Tool: model.ToolClaude, Name: "skill", State: model.SkillStateOn, Group: test.left},
				{Tool: model.ToolCodex, Name: "skill", State: model.SkillStateOn, Group: test.right},
			})

			if len(rows) != 1 {
				t.Fatalf("RowsFromSkills() returned %d rows, want 1: %#v", len(rows), rows)
			}
			if rows[0].Group != test.want {
				t.Fatalf("Group = %q, want %q", rows[0].Group, test.want)
			}
		})
	}
}

func TestRowsFromSkillsHideReadOnlyOnlyRowsByDefault(t *testing.T) {
	skills := []model.ToolSkill{
		{Tool: model.ToolCodex, Name: "imagegen", State: model.SkillStateReadOnly, Source: model.SourceCodexSystem, ReadOnly: true},
		{Tool: model.ToolClaude, Name: "managed", State: model.SkillStateOn, Source: model.SourceLocal},
		{Tool: model.ToolCodex, Name: "managed", State: model.SkillStateReadOnly, Source: model.SourceCodexSystem, ReadOnly: true},
	}

	rows := RowsFromSkills(skills)

	if len(rows) != 1 {
		t.Fatalf("RowsFromSkills() returned %d rows, want 1: %#v", len(rows), rows)
	}
	if rows[0].Name != "managed" {
		t.Fatalf("row name = %q, want managed", rows[0].Name)
	}
	if rows[0].Claude == nil || !rows[0].Claude.Toggleable() {
		t.Fatalf("managed row Claude cell = %#v, want toggleable", rows[0].Claude)
	}
	if rows[0].Codex != nil {
		t.Fatalf("managed row Codex cell = %#v, want read-only hidden by default", rows[0].Codex)
	}
}

func TestRowsFromSkillsCanIncludeReadOnlyOnlyRows(t *testing.T) {
	skills := []model.ToolSkill{
		{Tool: model.ToolCodex, Name: "imagegen", State: model.SkillStateReadOnly, Source: model.SourceCodexSystem, ReadOnly: true},
	}

	rows := RowsFromSkillsWithOptions(skills, RowOptions{IncludeReadOnly: true})

	if len(rows) != 1 {
		t.Fatalf("RowsFromSkillsWithOptions() returned %d rows, want 1: %#v", len(rows), rows)
	}
	if rows[0].Name != "imagegen" || rows[0].Codex == nil {
		t.Fatalf("row = %#v, want Codex imagegen", rows[0])
	}
}

func TestRowsFromSkillsPrefersManagedOverReadOnlyForSameToolAndName(t *testing.T) {
	skills := []model.ToolSkill{
		{Tool: model.ToolCodex, Name: "shared", State: model.SkillStateOn, Source: model.SourceLocal, ActivePath: "/managed"},
		{Tool: model.ToolCodex, Name: "shared", State: model.SkillStateReadOnly, Source: model.SourceCodexSystem, ActivePath: "/readonly", ReadOnly: true},
	}

	rows := RowsFromSkillsWithOptions(skills, RowOptions{IncludeReadOnly: true})

	if len(rows) != 1 {
		t.Fatalf("RowsFromSkillsWithOptions() returned %d rows, want 1: %#v", len(rows), rows)
	}
	if rows[0].Codex == nil {
		t.Fatalf("row = %#v, want Codex cell", rows[0])
	}
	if rows[0].Codex.ActivePath != "/managed" {
		t.Fatalf("Codex ActivePath = %q, want /managed", rows[0].Codex.ActivePath)
	}
	if rows[0].Source != model.SourceLocal {
		t.Fatalf("row Source = %q, want %q", rows[0].Source, model.SourceLocal)
	}
	if rows[0].Group != model.GroupUnknown {
		t.Fatalf("row Group = %q, want unknown", rows[0].Group)
	}
}

func TestRowsFromSkillsPrefersConflictOverActiveBlocker(t *testing.T) {
	skills := []model.ToolSkill{
		{Tool: model.ToolCodex, Name: "blocked", State: model.SkillStateOn, Source: model.SourceLocal, ActivePath: "/active"},
		{Tool: model.ToolCodex, Name: "blocked", State: model.SkillStateConflict, Source: model.SourceLocal, DisabledPath: "/disabled", Conflict: &model.Conflict{OriginalPath: "/active", DisabledPath: "/disabled", BlockerPath: "/active"}},
	}

	rows := RowsFromSkills(skills)

	if len(rows) != 1 {
		t.Fatalf("RowsFromSkills() returned %d rows, want 1: %#v", len(rows), rows)
	}
	if rows[0].Codex == nil {
		t.Fatalf("row = %#v, want Codex cell", rows[0])
	}
	if rows[0].Codex.State != model.SkillStateConflict {
		t.Fatalf("Codex State = %q, want %q", rows[0].Codex.State, model.SkillStateConflict)
	}
	if rows[0].Codex.Conflict == nil {
		t.Fatal("Codex Conflict = nil, want details")
	}
}

func TestGroupSummariesCountsAndSources(t *testing.T) {
	rows := []model.SkillRow{
		{
			Name:   "android-shared",
			Group:  model.GroupLabel("android/skills"),
			Source: model.SourceUnknown,
			Claude: &model.ToolSkill{State: model.SkillStateOn, Source: model.SourceSymlinkRepo},
			Codex:  &model.ToolSkill{State: model.SkillStateOff, Source: model.SourceSkillsCLI},
		},
		{
			Name:   "android-conflict",
			Group:  model.GroupLabel("android/skills"),
			Source: model.SourceSymlinkRepo,
			Claude: &model.ToolSkill{State: model.SkillStateConflict, Source: model.SourceSymlinkRepo},
		},
		{
			Name:   "android-codex",
			Group:  model.GroupLabel("android/skills"),
			Source: model.SourceUnknown,
			Claude: &model.ToolSkill{State: model.SkillStateReadOnly, Source: model.SourceClaudePlugin},
			Codex:  &model.ToolSkill{State: model.SkillStateOn, Source: model.SourceSymlinkRepo},
		},
		{
			Name:   "android-codex-conflict",
			Group:  model.GroupLabel("android/skills"),
			Source: model.SourceUnknown,
			Codex:  &model.ToolSkill{State: model.SkillStateConflict, Source: model.SourceLocal},
		},
		{
			Name:   "imagegen",
			Group:  model.GroupCodexSystem,
			Source: model.SourceCodexSystem,
			Codex:  &model.ToolSkill{State: model.SkillStateReadOnly, Source: model.SourceCodexSystem},
		},
		{
			Name:   "unknown",
			Group:  "",
			Source: model.SourceUnknown,
			Claude: &model.ToolSkill{State: model.SkillStateOff, Source: model.SourceUnknown},
		},
	}

	summaries := GroupSummaries(rows)

	if len(summaries) != 3 {
		t.Fatalf("GroupSummaries() returned %d summaries, want 3: %#v", len(summaries), summaries)
	}
	if summaries[0].Group != model.GroupCodexSystem ||
		summaries[1].Group != model.GroupLabel("android/skills") ||
		summaries[2].Group != model.GroupUnknown {
		t.Fatalf("summary order = %#v, want Codex system, android/skills, unknown", summaries)
	}

	codexSystem := summaries[0]
	if codexSystem.Rows != 1 || codexSystem.Claude != (model.ToolStateCounts{}) || codexSystem.Codex.ReadOnly != 1 {
		t.Fatalf("Codex system summary = %#v, want one Codex RO and no missing Claude count", codexSystem)
	}

	android := summaries[1]
	if android.Rows != 4 {
		t.Fatalf("android Rows = %d, want 4", android.Rows)
	}
	if android.Claude.On != 1 || android.Claude.Conflict != 1 || android.Claude.Off != 0 || android.Claude.ReadOnly != 1 {
		t.Fatalf("android Claude counts = %#v, want ON=1 CONFLICT=1 RO=1", android.Claude)
	}
	if android.Codex.On != 1 || android.Codex.Off != 1 || android.Codex.Conflict != 1 || android.Codex.ReadOnly != 0 {
		t.Fatalf("android Codex counts = %#v, want ON=1 OFF=1 CONFLICT=1", android.Codex)
	}
	if len(android.Sources) != 4 ||
		android.Sources[0] != model.SourceClaudePlugin ||
		android.Sources[1] != model.SourceSkillsCLI ||
		android.Sources[2] != model.SourceLocal ||
		android.Sources[3] != model.SourceSymlinkRepo {
		t.Fatalf("android Sources = %#v, want sorted cell sources", android.Sources)
	}
	if android.SourceText != "Claude plugin, Skills CLI, local, symlink repo" {
		t.Fatalf("android SourceText = %q, want sorted source text", android.SourceText)
	}

	unknown := summaries[2]
	if unknown.Group != model.GroupUnknown || unknown.Claude.Off != 1 || unknown.Codex != (model.ToolStateCounts{}) {
		t.Fatalf("unknown summary = %#v, want unknown group with one Claude OFF and no Codex count", unknown)
	}
}

func mkdirSkill(t *testing.T, dir string) {
	t.Helper()
	mkdirAll(t, dir)
	writeFile(t, filepath.Join(dir, "SKILL.md"), "# Skill\n")
}

func mkdirAll(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir parent for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
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

func saveState(t *testing.T, p paths.Paths, manifest state.Manifest) {
	t.Helper()
	if err := state.New(p).Save(manifest); err != nil {
		t.Fatalf("save state: %v", err)
	}
}

func hasPathPrefix(path, prefix string) bool {
	rel, err := filepath.Rel(prefix, path)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func findSkill(t *testing.T, skills []model.ToolSkill, tool model.Tool, name string) model.ToolSkill {
	t.Helper()
	for _, skill := range skills {
		if skill.Tool == tool && skill.Name == name {
			return skill
		}
	}
	t.Fatalf("skill %s/%s not found in %#v", tool, name, skills)
	return model.ToolSkill{}
}
