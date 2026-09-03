package paths

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dees91/agent-skill-manager/internal/model"
)

// Paths contains all MVP filesystem locations derived from a user home.
type Paths struct {
	Home              string
	ClaudeUserSkills  string
	CodexUserSkills   string
	MuseUserSkills    string
	CodexSystemSkills string
	ClaudePluginCache string
	AgentsSkillLock   string
	StateDir          string
	StateFile         string
	AdvisorFile       string
	AdvisorLockFile   string
	SkillSetsFile     string
	FavoritesFile     string
	CacheDir          string
	SkillsSHCacheFile string
	BackupDir         string
	DisabledDir       string
	ClaudeDisabledDir string
	CodexDisabledDir  string
	MuseDisabledDir   string
	ReposDir          string
	TrashDir          string
}

// Default returns the MVP paths for the current OS user home.
func Default() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, fmt.Errorf("resolve user home: %w", err)
	}
	return ForHome(home), nil
}

// ForHome returns the MVP paths for a provided home directory.
func ForHome(home string) Paths {
	stateDir := filepath.Join(home, ".skill-manager")
	cacheDir := filepath.Join(stateDir, "cache")
	disabledDir := filepath.Join(stateDir, "disabled")

	return Paths{
		Home:              home,
		ClaudeUserSkills:  filepath.Join(home, ".claude", "skills"),
		CodexUserSkills:   filepath.Join(home, ".agents", "skills"),
		MuseUserSkills:    museUserSkillsDir(home),
		CodexSystemSkills: filepath.Join(home, ".codex", "skills", ".system"),
		ClaudePluginCache: filepath.Join(home, ".claude", "plugins", "cache"),
		AgentsSkillLock:   filepath.Join(home, ".agents", ".skill-lock.json"),
		StateDir:          stateDir,
		StateFile:         filepath.Join(stateDir, "state.json"),
		AdvisorFile:       filepath.Join(stateDir, "advisor-activations.json"),
		AdvisorLockFile:   filepath.Join(stateDir, "advisor.lock"),
		SkillSetsFile:     filepath.Join(stateDir, "skill-sets.json"),
		FavoritesFile:     filepath.Join(stateDir, "favorites.json"),
		CacheDir:          cacheDir,
		SkillsSHCacheFile: filepath.Join(cacheDir, "skills-sh", "catalog-v1.json"),
		BackupDir:         filepath.Join(stateDir, "backups"),
		DisabledDir:       disabledDir,
		ClaudeDisabledDir: filepath.Join(disabledDir, model.ToolClaude.String()),
		CodexDisabledDir:  filepath.Join(disabledDir, model.ToolCodex.String()),
		MuseDisabledDir:   filepath.Join(disabledDir, model.ToolMuse.String()),
		ReposDir:          filepath.Join(stateDir, "repos"),
		TrashDir:          filepath.Join(stateDir, "trash"),
	}
}

// museUserSkillsDir resolves the managed Muse skill directory. It honors
// XDG_CONFIG_HOME when set to an absolute path and otherwise falls back to
// ~/.config/muse/skills.
func museUserSkillsDir(home string) string {
	if raw := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); raw != "" {
		if cleaned := filepath.Clean(raw); filepath.IsAbs(cleaned) {
			return filepath.Join(cleaned, "muse", "skills")
		}
	}
	return filepath.Join(home, ".config", "muse", "skills")
}

// UserSkillsDirFor returns the active managed skill directory for a tool.
func (p Paths) UserSkillsDirFor(tool model.Tool) (string, bool) {
	switch tool {
	case model.ToolClaude:
		return p.ClaudeUserSkills, true
	case model.ToolCodex:
		return p.CodexUserSkills, true
	case model.ToolMuse:
		return p.MuseUserSkills, true
	default:
		return "", false
	}
}

// DisabledDirFor returns the disabled skill directory for a tool.
func (p Paths) DisabledDirFor(tool model.Tool) (string, bool) {
	switch tool {
	case model.ToolClaude:
		return p.ClaudeDisabledDir, true
	case model.ToolCodex:
		return p.CodexDisabledDir, true
	case model.ToolMuse:
		return p.MuseDisabledDir, true
	default:
		return "", false
	}
}
