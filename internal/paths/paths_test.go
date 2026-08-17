package paths

import (
	"path/filepath"
	"testing"

	"github.com/dees91/agent-skill-manager/internal/model"
)

func TestForHomeDerivesMVPPaths(t *testing.T) {
	home := filepath.Join(string(filepath.Separator), "tmp", "skill-manager-home")

	got := ForHome(home)

	want := map[string]string{
		"ClaudeUserSkills":  filepath.Join(home, ".claude", "skills"),
		"CodexUserSkills":   filepath.Join(home, ".agents", "skills"),
		"CodexSystemSkills": filepath.Join(home, ".codex", "skills", ".system"),
		"ClaudePluginCache": filepath.Join(home, ".claude", "plugins", "cache"),
		"AgentsSkillLock":   filepath.Join(home, ".agents", ".skill-lock.json"),
		"StateDir":          filepath.Join(home, ".skill-manager"),
		"StateFile":         filepath.Join(home, ".skill-manager", "state.json"),
		"AdvisorFile":       filepath.Join(home, ".skill-manager", "advisor-activations.json"),
		"AdvisorLockFile":   filepath.Join(home, ".skill-manager", "advisor.lock"),
		"SkillSetsFile":     filepath.Join(home, ".skill-manager", "skill-sets.json"),
		"FavoritesFile":     filepath.Join(home, ".skill-manager", "favorites.json"),
		"CacheDir":          filepath.Join(home, ".skill-manager", "cache"),
		"SkillsSHCacheFile": filepath.Join(home, ".skill-manager", "cache", "skills-sh", "catalog-v1.json"),
		"BackupDir":         filepath.Join(home, ".skill-manager", "backups"),
		"DisabledDir":       filepath.Join(home, ".skill-manager", "disabled"),
		"ClaudeDisabledDir": filepath.Join(home, ".skill-manager", "disabled", "claude"),
		"CodexDisabledDir":  filepath.Join(home, ".skill-manager", "disabled", "codex"),
		"ReposDir":          filepath.Join(home, ".skill-manager", "repos"),
		"TrashDir":          filepath.Join(home, ".skill-manager", "trash"),
	}

	if got.Home != home {
		t.Fatalf("Home = %q, want %q", got.Home, home)
	}
	if got.ClaudeUserSkills != want["ClaudeUserSkills"] {
		t.Fatalf("ClaudeUserSkills = %q, want %q", got.ClaudeUserSkills, want["ClaudeUserSkills"])
	}
	if got.CodexUserSkills != want["CodexUserSkills"] {
		t.Fatalf("CodexUserSkills = %q, want %q", got.CodexUserSkills, want["CodexUserSkills"])
	}
	if got.CodexSystemSkills != want["CodexSystemSkills"] {
		t.Fatalf("CodexSystemSkills = %q, want %q", got.CodexSystemSkills, want["CodexSystemSkills"])
	}
	if got.ClaudePluginCache != want["ClaudePluginCache"] {
		t.Fatalf("ClaudePluginCache = %q, want %q", got.ClaudePluginCache, want["ClaudePluginCache"])
	}
	if got.AgentsSkillLock != want["AgentsSkillLock"] {
		t.Fatalf("AgentsSkillLock = %q, want %q", got.AgentsSkillLock, want["AgentsSkillLock"])
	}
	if got.StateDir != want["StateDir"] {
		t.Fatalf("StateDir = %q, want %q", got.StateDir, want["StateDir"])
	}
	if got.StateFile != want["StateFile"] {
		t.Fatalf("StateFile = %q, want %q", got.StateFile, want["StateFile"])
	}
	if got.AdvisorFile != want["AdvisorFile"] {
		t.Fatalf("AdvisorFile = %q, want %q", got.AdvisorFile, want["AdvisorFile"])
	}
	if got.AdvisorLockFile != want["AdvisorLockFile"] {
		t.Fatalf("AdvisorLockFile = %q, want %q", got.AdvisorLockFile, want["AdvisorLockFile"])
	}
	if got.SkillSetsFile != want["SkillSetsFile"] {
		t.Fatalf("SkillSetsFile = %q, want %q", got.SkillSetsFile, want["SkillSetsFile"])
	}
	if got.FavoritesFile != want["FavoritesFile"] {
		t.Fatalf("FavoritesFile = %q, want %q", got.FavoritesFile, want["FavoritesFile"])
	}
	if got.CacheDir != want["CacheDir"] {
		t.Fatalf("CacheDir = %q, want %q", got.CacheDir, want["CacheDir"])
	}
	if got.SkillsSHCacheFile != want["SkillsSHCacheFile"] {
		t.Fatalf("SkillsSHCacheFile = %q, want %q", got.SkillsSHCacheFile, want["SkillsSHCacheFile"])
	}
	if got.BackupDir != want["BackupDir"] {
		t.Fatalf("BackupDir = %q, want %q", got.BackupDir, want["BackupDir"])
	}
	if got.DisabledDir != want["DisabledDir"] {
		t.Fatalf("DisabledDir = %q, want %q", got.DisabledDir, want["DisabledDir"])
	}
	if got.ClaudeDisabledDir != want["ClaudeDisabledDir"] {
		t.Fatalf("ClaudeDisabledDir = %q, want %q", got.ClaudeDisabledDir, want["ClaudeDisabledDir"])
	}
	if got.CodexDisabledDir != want["CodexDisabledDir"] {
		t.Fatalf("CodexDisabledDir = %q, want %q", got.CodexDisabledDir, want["CodexDisabledDir"])
	}
	if got.ReposDir != want["ReposDir"] {
		t.Fatalf("ReposDir = %q, want %q", got.ReposDir, want["ReposDir"])
	}
	if got.TrashDir != want["TrashDir"] {
		t.Fatalf("TrashDir = %q, want %q", got.TrashDir, want["TrashDir"])
	}
}

func TestToolSpecificPathHelpers(t *testing.T) {
	p := ForHome(filepath.Join(string(filepath.Separator), "tmp", "skill-manager-home"))

	if got, ok := p.UserSkillsDirFor(model.ToolClaude); !ok || got != p.ClaudeUserSkills {
		t.Fatalf("UserSkillsDirFor(claude) = %q, %v; want %q, true", got, ok, p.ClaudeUserSkills)
	}
	if got, ok := p.UserSkillsDirFor(model.ToolCodex); !ok || got != p.CodexUserSkills {
		t.Fatalf("UserSkillsDirFor(codex) = %q, %v; want %q, true", got, ok, p.CodexUserSkills)
	}
	if got, ok := p.UserSkillsDirFor(model.Tool("bad")); ok || got != "" {
		t.Fatalf("UserSkillsDirFor(bad) = %q, %v; want empty, false", got, ok)
	}

	if got, ok := p.DisabledDirFor(model.ToolClaude); !ok || got != p.ClaudeDisabledDir {
		t.Fatalf("DisabledDirFor(claude) = %q, %v; want %q, true", got, ok, p.ClaudeDisabledDir)
	}
	if got, ok := p.DisabledDirFor(model.ToolCodex); !ok || got != p.CodexDisabledDir {
		t.Fatalf("DisabledDirFor(codex) = %q, %v; want %q, true", got, ok, p.CodexDisabledDir)
	}
	if got, ok := p.DisabledDirFor(model.Tool("bad")); ok || got != "" {
		t.Fatalf("DisabledDirFor(bad) = %q, %v; want empty, false", got, ok)
	}
}
