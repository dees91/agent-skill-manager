package gui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dees91/agent-skill-manager/internal/model"
	"github.com/dees91/agent-skill-manager/internal/paths"
	"github.com/dees91/agent-skill-manager/internal/skillssh"
	"github.com/dees91/agent-skill-manager/internal/state"
)

func TestDiscoverProjectsAvailableOwnedAndConflictingTargets(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	checkout := filepath.Join(p.ReposDir, "github.com", "demo", "skills")
	writeSkill(t, filepath.Join(checkout, "skills", "alpha"), "Alpha")
	if err := os.MkdirAll(p.ClaudeUserSkills, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(checkout, "skills", "alpha"), filepath.Join(p.ClaudeUserSkills, "alpha")); err != nil {
		t.Fatal(err)
	}
	writeSkill(t, filepath.Join(p.CodexUserSkills, "alpha"), "Other alpha")
	manifest := state.Manifest{Repositories: []state.RepositoryEntry{{
		OriginalURL: "https://github.com/demo/skills", CanonicalURL: "https://github.com/demo/skills", Host: "github.com", RepoPath: "demo/skills", CheckoutPath: checkout, Group: "demo/skills",
		InstalledSkills: []state.InstalledSkillEntry{{Name: "alpha", RelativePath: "skills/alpha", Tools: []model.Tool{model.ToolClaude}}},
	}}}
	if err := state.New(p).Save(manifest); err != nil {
		t.Fatal(err)
	}
	service := New(p)
	service.catalog = &fakeCatalog{page: catalogPage(catalogGitSkill("demo/skills", "alpha"))}
	if _, err := service.GetSnapshot(false); err != nil {
		t.Fatal(err)
	}
	page, err := service.GetDiscoverPage("all-time", 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := page.Skills[0].Claude.Status; got != "installed-on" {
		t.Fatalf("Claude status = %q", got)
	}
	if got := page.Skills[0].Codex.Status; got != "conflict" || !strings.Contains(page.Skills[0].Codex.Message, "another") {
		t.Fatalf("Codex state = %#v", page.Skills[0].Codex)
	}
}

func TestDiscoverProjectsOwnedDisabledTarget(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	checkout := filepath.Join(p.ReposDir, "github.com", "demo", "skills")
	skillPath := filepath.Join(checkout, "alpha")
	writeSkill(t, skillPath, "Alpha")
	disabledPath := filepath.Join(p.ClaudeDisabledDir, "alpha")
	if err := os.MkdirAll(filepath.Dir(disabledPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(skillPath, disabledPath); err != nil {
		t.Fatal(err)
	}
	manifest := state.Manifest{
		Disabled:     []state.DisabledEntry{{Tool: model.ToolClaude, SkillName: "alpha", OriginalPath: filepath.Join(p.ClaudeUserSkills, "alpha"), DisabledPath: disabledPath, EntryType: model.EntryTypeSymlink, SymlinkTarget: skillPath, Source: model.SourceSymlinkRepo, Group: "demo/skills"}},
		Repositories: []state.RepositoryEntry{{OriginalURL: "https://github.com/demo/skills", CanonicalURL: "https://github.com/demo/skills", Host: "github.com", RepoPath: "demo/skills", CheckoutPath: checkout, Group: "demo/skills", InstalledSkills: []state.InstalledSkillEntry{{Name: "alpha", RelativePath: "alpha", Tools: []model.Tool{model.ToolClaude}}}}},
	}
	if err := state.New(p).Save(manifest); err != nil {
		t.Fatal(err)
	}
	service := New(p)
	service.catalog = &fakeCatalog{page: catalogPage(catalogGitSkill("demo/skills", "alpha"))}
	if _, err := service.GetSnapshot(false); err != nil {
		t.Fatal(err)
	}
	page, err := service.GetDiscoverPage("all-time", 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if page.Skills[0].Claude.Status != "installed-off" || page.Skills[0].Codex.Status != "available" {
		t.Fatalf("states = Claude %#v Codex %#v", page.Skills[0].Claude, page.Skills[0].Codex)
	}
}

func TestInstallDiscoverSkillCreatesOnlySelectedSkillAndAgent(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	skill := catalogGitSkill("demo/catalog-skills", "alpha")
	catalog := &fakeCatalog{page: catalogPage(skill), detail: skillssh.Detail{Skill: skill, Description: "Alpha", FetchedAt: time.Now(), AuditStatus: "external-only"}}
	service := New(p)
	service.catalog = catalog
	service.gitRunner = discoverCloneRunner{t: t}
	if _, err := service.GetSnapshot(false); err != nil {
		t.Fatal(err)
	}
	if _, err := service.GetDiscoverPage("all-time", 0, false); err != nil {
		t.Fatal(err)
	}
	result := service.InstallDiscoverSkill(skill.ID, []string{"codex"}, false)
	if result.Failure != nil || result.CreatedLinks != 1 {
		t.Fatalf("result = %#v", result)
	}
	if _, err := os.Lstat(filepath.Join(p.ClaudeUserSkills, "alpha")); !os.IsNotExist(err) {
		t.Fatalf("unselected Claude target exists or returned unexpected error: %v", err)
	}
	if target, err := os.Readlink(filepath.Join(p.CodexUserSkills, "alpha")); err != nil || !strings.HasSuffix(target, filepath.Join("catalog-skills", "skills", "alpha")) {
		t.Fatalf("selected Codex target = %q err=%v", target, err)
	}
	for _, name := range []string{"beta", "nested"} {
		if _, err := os.Lstat(filepath.Join(p.CodexUserSkills, name)); !os.IsNotExist(err) {
			t.Fatalf("unselected repository skill %s was installed: %v", name, err)
		}
	}
	manifest, err := state.New(p).Load()
	if err != nil {
		t.Fatal(err)
	}
	repository, ok := manifest.GetRepository("github.com", "demo/catalog-skills")
	if !ok || len(repository.InstalledSkills) != 1 || repository.InstalledSkills[0].Name != "alpha" || len(repository.InstalledSkills[0].Tools) != 1 || repository.InstalledSkills[0].Tools[0] != model.ToolCodex {
		t.Fatalf("repository ownership = %#v", repository)
	}
}

func TestInstallDiscoverSkillRejectsUnknownWellKnownOfflineAndPending(t *testing.T) {
	t.Run("unknown session ID", func(t *testing.T) {
		service := New(paths.ForHome(t.TempDir()))
		result := service.InstallDiscoverSkill("demo/skills/alpha", []string{"claude"}, false)
		if result.Failure == nil || result.Failure.Stage != "catalog" {
			t.Fatalf("result = %#v", result)
		}
	})

	t.Run("well-known", func(t *testing.T) {
		p := paths.ForHome(t.TempDir())
		skill := skillssh.Skill{ID: "example.com/alpha", SkillID: "alpha", Name: "alpha", Source: "example.com", SourceType: "well-known", URL: "https://www.skills.sh/site/example.com/alpha"}
		service := New(p)
		service.catalog = &fakeCatalog{page: catalogPage(skill), detail: skillssh.Detail{Skill: skill}}
		if _, err := service.GetSnapshot(false); err != nil {
			t.Fatal(err)
		}
		if _, err := service.GetDiscoverPage("all-time", 0, false); err != nil {
			t.Fatal(err)
		}
		result := service.InstallDiscoverSkill(skill.ID, []string{"claude"}, false)
		if result.Failure == nil || !strings.Contains(result.Failure.Message, "well-known") {
			t.Fatalf("result = %#v", result)
		}
	})

	t.Run("offline", func(t *testing.T) {
		p := paths.ForHome(t.TempDir())
		skill := catalogGitSkill("demo/skills", "alpha")
		service := New(p)
		service.catalog = &fakeCatalog{page: catalogPage(skill), detail: skillssh.Detail{Skill: skill, Offline: true}}
		if _, err := service.GetSnapshot(false); err != nil {
			t.Fatal(err)
		}
		if _, err := service.GetDiscoverPage("all-time", 0, false); err != nil {
			t.Fatal(err)
		}
		result := service.InstallDiscoverSkill(skill.ID, []string{"claude"}, false)
		if result.Failure == nil || !strings.Contains(result.Failure.Message, "offline") {
			t.Fatalf("result = %#v", result)
		}
	})

	t.Run("pending toggles", func(t *testing.T) {
		p := paths.ForHome(t.TempDir())
		writeSkill(t, filepath.Join(p.ClaudeUserSkills, "existing"), "Existing")
		skill := catalogGitSkill("demo/skills", "alpha")
		catalog := &fakeCatalog{page: catalogPage(skill), detail: skillssh.Detail{Skill: skill}}
		service := New(p)
		service.catalog = catalog
		if _, err := service.GetSnapshot(false); err != nil {
			t.Fatal(err)
		}
		if _, err := service.GetDiscoverPage("all-time", 0, false); err != nil {
			t.Fatal(err)
		}
		if _, err := service.ToggleCell("existing", "claude"); err != nil {
			t.Fatal(err)
		}
		result := service.InstallDiscoverSkill(skill.ID, []string{"claude"}, false)
		if result.Failure == nil || !strings.Contains(result.Failure.Message, "pending") || catalog.detailCalls != 0 {
			t.Fatalf("result = %#v detail calls=%d", result, catalog.detailCalls)
		}
	})

	t.Run("conflicting selected target", func(t *testing.T) {
		p := paths.ForHome(t.TempDir())
		writeSkill(t, filepath.Join(p.ClaudeUserSkills, "alpha"), "Unmanaged alpha")
		skill := catalogGitSkill("demo/skills", "alpha")
		service := New(p)
		service.catalog = &fakeCatalog{page: catalogPage(skill), detail: skillssh.Detail{Skill: skill}}
		if _, err := service.GetSnapshot(false); err != nil {
			t.Fatal(err)
		}
		if _, err := service.GetDiscoverPage("all-time", 0, false); err != nil {
			t.Fatal(err)
		}
		result := service.InstallDiscoverSkill(skill.ID, []string{"claude"}, false)
		if result.Failure == nil || !strings.Contains(result.Failure.Message, "not available") {
			t.Fatalf("result = %#v", result)
		}
	})

	t.Run("busy source operation", func(t *testing.T) {
		p := paths.ForHome(t.TempDir())
		skill := catalogGitSkill("demo/skills", "alpha")
		catalog := &fakeCatalog{page: catalogPage(skill), detail: skillssh.Detail{Skill: skill}}
		service := New(p)
		service.catalog = catalog
		if _, err := service.GetSnapshot(false); err != nil {
			t.Fatal(err)
		}
		if _, err := service.GetDiscoverPage("all-time", 0, false); err != nil {
			t.Fatal(err)
		}
		service.sourceBusy.Store(true)
		defer service.sourceBusy.Store(false)
		result := service.InstallDiscoverSkill(skill.ID, []string{"claude"}, false)
		if result.Failure == nil || !strings.Contains(result.Failure.Message, "already in progress") || catalog.detailCalls != 0 {
			t.Fatalf("result = %#v detail calls=%d", result, catalog.detailCalls)
		}
	})
}

type fakeCatalog struct {
	page        skillssh.Page
	detail      skillssh.Detail
	pageErr     error
	detailErr   error
	detailCalls int
}

func (f *fakeCatalog) GetPage(context.Context, skillssh.View, int, bool) (skillssh.Page, error) {
	return f.page, f.pageErr
}

func (f *fakeCatalog) Search(context.Context, string) (skillssh.Page, error) {
	return f.page, f.pageErr
}

func (f *fakeCatalog) GetDetail(_ context.Context, skill skillssh.Skill, _ bool) (skillssh.Detail, error) {
	f.detailCalls++
	if f.detail.Skill.ID == "" {
		f.detail.Skill = skill
	}
	return f.detail, f.detailErr
}

type discoverCloneRunner struct{ t *testing.T }

func (r discoverCloneRunner) RunGit(args ...string) (string, error) {
	r.t.Helper()
	if len(args) == 3 && args[0] == "clone" {
		writeSkill(r.t, filepath.Join(args[2], "skills", "alpha"), "Alpha")
		writeSkill(r.t, filepath.Join(args[2], "skills", "beta"), "Beta")
		writeSkill(r.t, filepath.Join(args[2], "nested"), "Nested")
		return "", nil
	}
	if len(args) == 4 && args[0] == "-C" && args[2] == "rev-parse" && args[3] == "HEAD" {
		return "abc123", nil
	}
	return "", fmt.Errorf("unexpected git command: %v", args)
}

func catalogGitSkill(source, slug string) skillssh.Skill {
	return skillssh.Skill{ID: source + "/" + slug, SkillID: slug, Name: slug, Source: source, Installs: 42, SourceType: "github", InstallURL: "https://github.com/" + source, URL: "https://www.skills.sh/" + source + "/" + slug}
}

func catalogPage(skills ...skillssh.Skill) skillssh.Page {
	return skillssh.Page{View: skillssh.ViewAllTime, Page: 0, Total: len(skills), Skills: skills, FetchedAt: time.Now()}
}
