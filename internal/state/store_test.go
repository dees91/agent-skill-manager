package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dees91/agent-skill-manager/internal/model"
	"github.com/dees91/agent-skill-manager/internal/paths"
)

func TestLoadMissingStateReturnsEmptyManifest(t *testing.T) {
	store := New(paths.ForHome(t.TempDir()))

	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got.Version != manifestVersion {
		t.Fatalf("Version = %d, want %d", got.Version, manifestVersion)
	}
	if len(got.Disabled) != 0 {
		t.Fatalf("Disabled = %#v, want empty", got.Disabled)
	}
	if len(got.Repositories) != 0 {
		t.Fatalf("Repositories = %#v, want empty", got.Repositories)
	}
	if len(got.LocalSources) != 0 {
		t.Fatalf("LocalSources = %#v, want empty", got.LocalSources)
	}
}

func TestLoadBackwardCompatibleDisabledOnlyManifest(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	if err := os.MkdirAll(p.StateDir, 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	oldManifest := `{
  "version": 1,
  "disabled": [
    {
      "tool": "claude",
      "skillName": "edge-to-edge",
      "originalPath": "/active/edge-to-edge",
      "disabledPath": "/disabled/edge-to-edge",
      "entryType": "symlink",
      "source": "symlink repo",
      "group": "android/skills",
      "disabledAt": "2026-05-08T12:30:00Z"
    }
  ]
}`
	if err := os.WriteFile(p.StateFile, []byte(oldManifest), 0o644); err != nil {
		t.Fatalf("write old state: %v", err)
	}

	got, err := New(p).Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(got.Disabled) != 1 {
		t.Fatalf("Disabled len = %d, want 1", len(got.Disabled))
	}
	if len(got.Repositories) != 0 {
		t.Fatalf("Repositories = %#v, want empty", got.Repositories)
	}
	if got.Version != 2 || len(got.LocalSources) != 0 {
		t.Fatalf("migrated manifest = %#v, want version 2 with no local sources", got)
	}
}

func TestDisabledPathUsesExpectedLayout(t *testing.T) {
	home := t.TempDir()
	store := New(paths.ForHome(home))

	claudePath, err := store.DisabledPath(model.ToolClaude, "agp-9-upgrade")
	if err != nil {
		t.Fatalf("DisabledPath(claude) error = %v", err)
	}
	wantClaude := filepath.Join(home, ".skill-manager", "disabled", "claude", "agp-9-upgrade")
	if claudePath != wantClaude {
		t.Fatalf("DisabledPath(claude) = %q, want %q", claudePath, wantClaude)
	}

	codexPath, err := store.DisabledPath(model.ToolCodex, "find-skills")
	if err != nil {
		t.Fatalf("DisabledPath(codex) error = %v", err)
	}
	wantCodex := filepath.Join(home, ".skill-manager", "disabled", "codex", "find-skills")
	if codexPath != wantCodex {
		t.Fatalf("DisabledPath(codex) = %q, want %q", codexPath, wantCodex)
	}
}

func TestDisabledPathRejectsInvalidInputs(t *testing.T) {
	store := New(paths.ForHome(t.TempDir()))

	if _, err := store.DisabledPath(model.Tool("bad"), "skill"); err == nil {
		t.Fatal("DisabledPath(invalid tool) error = nil, want error")
	}
	for _, name := range []string{"", ".", "..", "nested/skill"} {
		t.Run(name, func(t *testing.T) {
			if _, err := store.DisabledPath(model.ToolClaude, name); err == nil {
				t.Fatalf("DisabledPath(%q) error = nil, want error", name)
			}
		})
	}
}

func TestSaveWritesValidJSON(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	store := New(p)
	disabledAt := time.Date(2026, 5, 8, 12, 30, 0, 0, time.UTC)
	manifest := Manifest{
		Disabled: []DisabledEntry{
			{
				Tool:          model.ToolClaude,
				SkillName:     "edge-to-edge",
				OriginalPath:  filepath.Join(p.ClaudeUserSkills, "edge-to-edge"),
				DisabledPath:  filepath.Join(p.ClaudeDisabledDir, "edge-to-edge"),
				EntryType:     model.EntryTypeSymlink,
				SymlinkTarget: "/tmp/source",
				Source:        model.SourceSymlinkRepo,
				Group:         model.GroupLabel("android/skills"),
				DisabledAt:    disabledAt,
			},
		},
	}

	if err := store.Save(manifest); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	data, err := os.ReadFile(p.StateFile)
	if err != nil {
		t.Fatalf("read state file: %v", err)
	}

	var decoded Manifest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("state file is invalid JSON: %v\n%s", err, string(data))
	}
	if decoded.Version != manifestVersion {
		t.Fatalf("Version = %d, want %d", decoded.Version, manifestVersion)
	}
	if len(decoded.Disabled) != 1 {
		t.Fatalf("Disabled len = %d, want 1", len(decoded.Disabled))
	}
	entry := decoded.Disabled[0]
	if entry.Tool != model.ToolClaude ||
		entry.SkillName != "edge-to-edge" ||
		entry.OriginalPath == "" ||
		entry.DisabledPath == "" ||
		entry.EntryType != model.EntryTypeSymlink ||
		entry.SymlinkTarget != "/tmp/source" ||
		entry.Source != model.SourceSymlinkRepo ||
		entry.Group != model.GroupLabel("android/skills") ||
		!entry.DisabledAt.Equal(disabledAt) {
		t.Fatalf("decoded entry = %#v, want saved values", entry)
	}
}

func TestSaveWritesPredictableEmptySlices(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	store := New(p)

	if err := store.Save(Manifest{}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	data, err := os.ReadFile(p.StateFile)
	if err != nil {
		t.Fatalf("read state file: %v", err)
	}
	var decoded struct {
		Version      int   `json:"version"`
		Disabled     []any `json:"disabled"`
		Repositories []any `json:"repositories"`
		LocalSources []any `json:"localSources"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("state file is invalid JSON: %v\n%s", err, string(data))
	}
	if decoded.Version != manifestVersion {
		t.Fatalf("Version = %d, want %d", decoded.Version, manifestVersion)
	}
	if decoded.Disabled == nil || len(decoded.Disabled) != 0 {
		t.Fatalf("Disabled = %#v, want empty non-nil slice", decoded.Disabled)
	}
	if decoded.Repositories == nil || len(decoded.Repositories) != 0 {
		t.Fatalf("Repositories = %#v, want empty non-nil slice", decoded.Repositories)
	}
	if decoded.LocalSources == nil || len(decoded.LocalSources) != 0 {
		t.Fatalf("LocalSources = %#v, want empty non-nil slice", decoded.LocalSources)
	}
}

func TestSaveLoadRepositoryManifest(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	store := New(p)
	installedAt := time.Date(2026, 5, 8, 13, 45, 0, 0, time.UTC)
	manifest := Manifest{
		Repositories: []RepositoryEntry{
			{
				OriginalURL:    "git@github.com:addyosmani/agent-skills.git",
				CanonicalURL:   "https://github.com/addyosmani/agent-skills",
				Host:           "github.com",
				RepoPath:       "addyosmani/agent-skills",
				CheckoutPath:   filepath.Join(p.ReposDir, "github.com", "addyosmani", "agent-skills"),
				Group:          model.GroupLabel("addyosmani/agent-skills"),
				InstalledAt:    installedAt,
				LastSeenCommit: "abc123",
				InstalledSkills: []InstalledSkillEntry{
					{
						Name:         "zeta",
						RelativePath: "skills/zeta",
						Tools:        []model.Tool{model.ToolCodex, model.Tool("zed"), model.ToolClaude, model.ToolCodex},
					},
					{
						Name:         "alpha",
						RelativePath: "skills/alpha",
						Tools:        []model.Tool{model.ToolCodex},
					},
				},
			},
		},
	}

	if err := store.Save(manifest); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	entry, ok := got.GetRepository("github.com", "addyosmani/agent-skills")
	if !ok {
		t.Fatal("GetRepository() ok = false, want true")
	}
	if entry.OriginalURL != "git@github.com:addyosmani/agent-skills.git" ||
		entry.CanonicalURL != "https://github.com/addyosmani/agent-skills" ||
		entry.Host != "github.com" ||
		entry.RepoPath != "addyosmani/agent-skills" ||
		entry.CheckoutPath != filepath.Join(p.ReposDir, "github.com", "addyosmani", "agent-skills") ||
		entry.Group != model.GroupLabel("addyosmani/agent-skills") ||
		entry.LastSeenCommit != "abc123" ||
		!entry.InstalledAt.Equal(installedAt) {
		t.Fatalf("repository entry = %#v, want saved metadata", entry)
	}
	if len(entry.InstalledSkills) != 2 {
		t.Fatalf("InstalledSkills len = %d, want 2", len(entry.InstalledSkills))
	}
	if entry.InstalledSkills[0].Name != "alpha" || entry.InstalledSkills[1].Name != "zeta" {
		t.Fatalf("InstalledSkills order = %#v, want sorted by name", entry.InstalledSkills)
	}
	wantTools := []model.Tool{model.ToolClaude, model.ToolCodex, model.Tool("zed")}
	if !sameTools(entry.InstalledSkills[1].Tools, wantTools) {
		t.Fatalf("Tools = %#v, want %#v", entry.InstalledSkills[1].Tools, wantTools)
	}
}

func TestBackupExistingCopiesManifest(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	store := New(p)
	store.now = func() time.Time {
		return time.Date(2026, 5, 8, 12, 30, 45, 123, time.UTC)
	}

	manifest := Manifest{
		Disabled: []DisabledEntry{
			{Tool: model.ToolCodex, SkillName: "find-skills", Source: model.SourceSkillsCLI},
		},
	}
	if err := store.Save(manifest); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	original, err := os.ReadFile(p.StateFile)
	if err != nil {
		t.Fatalf("read original state: %v", err)
	}

	backupPath, err := store.BackupExisting()
	if err != nil {
		t.Fatalf("BackupExisting() error = %v", err)
	}

	wantPath := filepath.Join(p.BackupDir, "state-20260508T123045.000000123Z.json")
	if backupPath != wantPath {
		t.Fatalf("BackupExisting() path = %q, want %q", backupPath, wantPath)
	}
	backup, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if string(backup) != string(original) {
		t.Fatalf("backup contents differ\nbackup=%s\noriginal=%s", backup, original)
	}
}

func TestBackupExistingMissingManifestIsNoop(t *testing.T) {
	store := New(paths.ForHome(t.TempDir()))

	backupPath, err := store.BackupExisting()
	if err != nil {
		t.Fatalf("BackupExisting() error = %v", err)
	}
	if backupPath != "" {
		t.Fatalf("BackupExisting() path = %q, want empty", backupPath)
	}
}

func TestManifestHelpers(t *testing.T) {
	var manifest Manifest
	entry := DisabledEntry{Tool: model.ToolClaude, SkillName: "local-only-skill", OriginalPath: "/original"}

	manifest.Upsert(entry)
	got, ok := manifest.Get(model.ToolClaude, "local-only-skill")
	if !ok {
		t.Fatal("Get() ok = false, want true")
	}
	if got.OriginalPath != "/original" {
		t.Fatalf("OriginalPath = %q, want /original", got.OriginalPath)
	}

	entry.OriginalPath = "/updated"
	manifest.Upsert(entry)
	got, ok = manifest.Get(model.ToolClaude, "local-only-skill")
	if !ok {
		t.Fatal("Get() after update ok = false, want true")
	}
	if got.OriginalPath != "/updated" {
		t.Fatalf("OriginalPath after update = %q, want /updated", got.OriginalPath)
	}
	if len(manifest.Disabled) != 1 {
		t.Fatalf("Disabled len = %d, want 1", len(manifest.Disabled))
	}

	if !manifest.Remove(model.ToolClaude, "local-only-skill") {
		t.Fatal("Remove() = false, want true")
	}
	if _, ok := manifest.Get(model.ToolClaude, "local-only-skill"); ok {
		t.Fatal("Get() after remove ok = true, want false")
	}
	if len(manifest.Disabled) != 0 {
		t.Fatalf("Disabled len after remove = %d, want 0", len(manifest.Disabled))
	}
}

func TestRepositoryManifestHelpers(t *testing.T) {
	var manifest Manifest
	entry := RepositoryEntry{
		OriginalURL:  "https://github.com/owner/repo",
		CanonicalURL: "https://github.com/owner/repo",
		Host:         "github.com",
		RepoPath:     "owner/repo",
		InstalledSkills: []InstalledSkillEntry{
			{
				Name:         "beta",
				RelativePath: "skills/beta",
				Tools:        []model.Tool{model.ToolCodex, model.ToolClaude, model.ToolCodex},
			},
		},
	}

	manifest.UpsertRepository(entry)
	got, ok := manifest.GetRepository("github.com", "owner/repo")
	if !ok {
		t.Fatal("GetRepository() ok = false, want true")
	}
	if got.OriginalURL != "https://github.com/owner/repo" {
		t.Fatalf("OriginalURL = %q, want original value", got.OriginalURL)
	}
	wantTools := []model.Tool{model.ToolClaude, model.ToolCodex}
	if !sameTools(got.InstalledSkills[0].Tools, wantTools) {
		t.Fatalf("Tools = %#v, want %#v", got.InstalledSkills[0].Tools, wantTools)
	}

	entry.OriginalURL = "git@github.com:owner/repo.git"
	entry.InstalledSkills = []InstalledSkillEntry{
		{Name: "alpha", RelativePath: "skills/alpha", Tools: []model.Tool{model.ToolCodex}},
	}
	manifest.UpsertRepository(entry)
	got, ok = manifest.GetRepository("github.com", "owner/repo")
	if !ok {
		t.Fatal("GetRepository() after update ok = false, want true")
	}
	if got.OriginalURL != "git@github.com:owner/repo.git" {
		t.Fatalf("OriginalURL after update = %q, want git URL", got.OriginalURL)
	}
	if len(manifest.Repositories) != 1 {
		t.Fatalf("Repositories len = %d, want 1", len(manifest.Repositories))
	}

	manifest.UpsertRepository(RepositoryEntry{
		OriginalURL: "https://git.example.com/team/tooling",
		Host:        "git.example.com",
		RepoPath:    "team/tooling",
	})
	if len(manifest.Repositories) != 2 {
		t.Fatalf("Repositories len after second repo = %d, want 2", len(manifest.Repositories))
	}
	if manifest.Repositories[0].Host != "git.example.com" || manifest.Repositories[1].Host != "github.com" {
		t.Fatalf("Repositories order = %#v, want sorted by host/repo path", manifest.Repositories)
	}

	if !manifest.RemoveRepository("github.com", "owner/repo") {
		t.Fatal("RemoveRepository() = false, want true")
	}
	if _, ok := manifest.GetRepository("github.com", "owner/repo"); ok {
		t.Fatal("GetRepository() after remove ok = true, want false")
	}
	if manifest.RemoveRepository("github.com", "owner/repo") {
		t.Fatal("second RemoveRepository() = true, want false")
	}
}

func TestLoadCoalescesDuplicateRepositoryIdentities(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	store := New(p)
	manifest := Manifest{
		Repositories: []RepositoryEntry{
			{
				OriginalURL: "https://github.com/owner/repo",
				Host:        "github.com",
				RepoPath:    "owner/repo",
			},
			{
				OriginalURL: "git@github.com:owner/repo.git",
				Host:        "github.com",
				RepoPath:    "owner/repo",
			},
		},
	}
	if err := store.Save(manifest); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(got.Repositories) != 1 {
		t.Fatalf("Repositories len = %d, want 1", len(got.Repositories))
	}
	if got.Repositories[0].OriginalURL != "git@github.com:owner/repo.git" {
		t.Fatalf("OriginalURL = %q, want last duplicate to win", got.Repositories[0].OriginalURL)
	}
}

func TestSaveLoadAndManageLocalSources(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	store := New(p)
	installedAt := time.Date(2026, 8, 11, 14, 30, 0, 0, time.UTC)
	alpha := filepath.Join(p.Home, "alpha-source")
	zeta := filepath.Join(p.Home, "zeta-source")
	manifest := Manifest{}
	manifest.UpsertLocalSource(LocalSourceEntry{
		OriginalPath:  zeta,
		CanonicalPath: zeta,
		Group:         model.GroupLabel("zeta-source"),
		InstalledAt:   installedAt,
		InstalledSkills: []InstalledSkillEntry{
			{Name: "beta", RelativePath: "skills/beta", Tools: []model.Tool{model.ToolCodex, model.ToolClaude, model.ToolCodex}},
		},
	})
	manifest.UpsertLocalSource(LocalSourceEntry{OriginalPath: alpha, CanonicalPath: alpha, Group: model.GroupLabel("alpha-source")})
	if err := store.Save(manifest); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Version != 2 || len(got.LocalSources) != 2 {
		t.Fatalf("manifest = %#v, want version 2 and two local sources", got)
	}
	if got.LocalSources[0].CanonicalPath != alpha || got.LocalSources[1].CanonicalPath != zeta {
		t.Fatalf("LocalSources order = %#v, want canonical path order", got.LocalSources)
	}
	entry, ok := got.GetLocalSource(zeta)
	if !ok || !entry.InstalledAt.Equal(installedAt) || len(entry.InstalledSkills) != 1 {
		t.Fatalf("GetLocalSource() = %#v ok=%v", entry, ok)
	}
	if !sameTools(entry.InstalledSkills[0].Tools, []model.Tool{model.ToolClaude, model.ToolCodex}) {
		t.Fatalf("local source tools = %#v, want normalized", entry.InstalledSkills[0].Tools)
	}
	if !got.RemoveLocalSource(alpha) || got.RemoveLocalSource(alpha) {
		t.Fatalf("RemoveLocalSource() did not remove exactly once: %#v", got.LocalSources)
	}
}

func TestLoadAndSaveRejectNewerManifestVersion(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	if err := os.MkdirAll(p.StateDir, 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	if err := os.WriteFile(p.StateFile, []byte(`{"version":99,"disabled":[],"repositories":[],"localSources":[]}`), 0o644); err != nil {
		t.Fatalf("write newer manifest: %v", err)
	}
	if _, err := New(p).Load(); err == nil {
		t.Fatal("Load() newer version error = nil")
	}
	if err := New(p).Save(Manifest{Version: 99}); err == nil {
		t.Fatal("Save() newer version error = nil")
	}
}

func sameTools(got, want []model.Tool) bool {
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
