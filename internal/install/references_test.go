package install

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dees91/agent-skill-manager/internal/model"
	"github.com/dees91/agent-skill-manager/internal/paths"
	"github.com/dees91/agent-skill-manager/internal/state"
)

func TestAuditRepositoryReferencesAcceptsActiveAndDisabledLinks(t *testing.T) {
	p, repository, skillPaths := referenceRepositoryFixture(t, []string{"alpha", "beta"})
	activeAlpha := filepath.Join(p.ClaudeUserSkills, "alpha")
	mustSymlink(t, skillPaths["alpha"], activeAlpha)
	disabledBeta := filepath.Join(p.CodexDisabledDir, "beta")
	mustSymlink(t, skillPaths["beta"], disabledBeta)
	blocker := filepath.Join(p.CodexUserSkills, "beta")
	mustWriteFile(t, blocker, "unrelated blocker")

	repository.InstalledSkills = []state.InstalledSkillEntry{
		{Name: "alpha", RelativePath: "skills/alpha", Tools: []model.Tool{model.ToolClaude}},
		{Name: "beta", RelativePath: "skills/beta", Tools: []model.Tool{model.ToolCodex}},
	}
	manifest := state.Manifest{
		Repositories: []state.RepositoryEntry{repository},
		Disabled: []state.DisabledEntry{{
			Tool:          model.ToolCodex,
			SkillName:     "beta",
			OriginalPath:  blocker,
			DisabledPath:  disabledBeta,
			EntryType:     model.EntryTypeSymlink,
			SymlinkTarget: skillPaths["beta"],
			Source:        model.SourceSymlinkRepo,
			Group:         repository.Group,
			DisabledAt:    time.Now(),
		}},
	}

	audit, err := AuditRepositoryReferences(p, manifest, repository)
	if err != nil {
		t.Fatalf("AuditRepositoryReferences() error = %v", err)
	}
	if len(audit.References) != 2 {
		t.Fatalf("References len = %d, want 2", len(audit.References))
	}
	if audit.References[0].State != model.SkillStateOn || audit.References[0].SkillName != "alpha" {
		t.Fatalf("first reference = %#v, want active alpha", audit.References[0])
	}
	if audit.References[1].State != model.SkillStateOff || audit.References[1].SkillName != "beta" {
		t.Fatalf("second reference = %#v, want disabled beta", audit.References[1])
	}
	if contents, err := os.ReadFile(blocker); err != nil || string(contents) != "unrelated blocker" {
		t.Fatalf("blocker changed: contents=%q err=%v", contents, err)
	}
}

func TestAuditRepositoryReferencesReportsMissingChangedAndExtraLinks(t *testing.T) {
	p, repository, skillPaths := referenceRepositoryFixture(t, []string{"alpha", "beta"})
	repository.InstalledSkills = []state.InstalledSkillEntry{
		{Name: "alpha", RelativePath: "skills/alpha", Tools: []model.Tool{model.ToolClaude}},
		{Name: "beta", RelativePath: "skills/beta", Tools: []model.Tool{model.ToolCodex}},
	}
	wrongTarget := filepath.Join(t.TempDir(), "wrong")
	if err := os.MkdirAll(wrongTarget, 0o755); err != nil {
		t.Fatalf("create wrong target: %v", err)
	}
	mustSymlink(t, wrongTarget, filepath.Join(p.CodexUserSkills, "beta"))
	extraPath := filepath.Join(p.ClaudeUserSkills, "extra")
	mustSymlink(t, skillPaths["alpha"], extraPath)

	_, err := AuditRepositoryReferences(p, state.Manifest{Repositories: []state.RepositoryEntry{repository}}, repository)
	if err == nil {
		t.Fatal("AuditRepositoryReferences() error = nil, want conflicts")
	}
	var auditErr ReferenceAuditError
	if !errors.As(err, &auditErr) {
		t.Fatalf("error = %T %v, want ReferenceAuditError", err, err)
	}
	if len(auditErr.Conflicts) != 3 {
		t.Fatalf("conflicts = %#v, want missing, changed, and extra", auditErr.Conflicts)
	}
	for _, want := range []string{"expected managed symlink is missing", "managed symlink points", "extra managed symlink"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want %q", err, want)
		}
	}
}

func TestAuditRepositoryReferencesRejectsUnrecordedDisabledManifestReference(t *testing.T) {
	p, repository, skillPaths := referenceRepositoryFixture(t, []string{"alpha"})
	repository.InstalledSkills = []state.InstalledSkillEntry{}
	disabledPath := filepath.Join(p.ClaudeDisabledDir, "alpha")
	mustSymlink(t, skillPaths["alpha"], disabledPath)
	manifest := state.Manifest{
		Repositories: []state.RepositoryEntry{repository},
		Disabled: []state.DisabledEntry{{
			Tool:          model.ToolClaude,
			SkillName:     "alpha",
			OriginalPath:  filepath.Join(p.ClaudeUserSkills, "alpha"),
			DisabledPath:  disabledPath,
			EntryType:     model.EntryTypeSymlink,
			SymlinkTarget: skillPaths["alpha"],
		}},
	}

	_, err := AuditRepositoryReferences(p, manifest, repository)
	if err == nil || !strings.Contains(err.Error(), "not recorded by repository") {
		t.Fatalf("AuditRepositoryReferences() error = %v, want unrecorded manifest conflict", err)
	}
}

func TestAuditRepositoryReferencesRejectsCheckoutPathMismatch(t *testing.T) {
	p, repository, _ := referenceRepositoryFixture(t, nil)
	repository.CheckoutPath = filepath.Join(p.ReposDir, "github.com", "other", "repo")

	_, err := AuditRepositoryReferences(p, state.Manifest{}, repository)
	if err == nil || !strings.Contains(err.Error(), "does not match expected") {
		t.Fatalf("AuditRepositoryReferences() error = %v, want checkout mismatch", err)
	}
}

func referenceRepositoryFixture(t *testing.T, skillNames []string) (paths.Paths, state.RepositoryEntry, map[string]string) {
	t.Helper()
	p := paths.ForHome(t.TempDir())
	identity, err := NormalizeGitURL("https://github.com/owner/repo")
	if err != nil {
		t.Fatalf("NormalizeGitURL() error = %v", err)
	}
	checkoutPath, err := CheckoutPath(p, identity)
	if err != nil {
		t.Fatalf("CheckoutPath() error = %v", err)
	}
	if err := os.MkdirAll(checkoutPath, 0o755); err != nil {
		t.Fatalf("create checkout: %v", err)
	}
	skillPaths := map[string]string{}
	for _, name := range skillNames {
		skillPath := filepath.Join(checkoutPath, "skills", name)
		mustWriteFile(t, filepath.Join(skillPath, "SKILL.md"), "# "+name)
		skillPaths[name] = skillPath
	}
	return p, state.RepositoryEntry{
		OriginalURL:  identity.OriginalURL,
		CanonicalURL: identity.CanonicalURL,
		Host:         identity.Host,
		RepoPath:     identity.RepoPath,
		CheckoutPath: checkoutPath,
		Group:        identity.Group,
	}, skillPaths
}

func mustWriteFile(t *testing.T, filePath, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		t.Fatalf("create parent for %s: %v", filePath, err)
	}
	if err := os.WriteFile(filePath, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", filePath, err)
	}
}

func mustSymlink(t *testing.T, target, linkPath string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(linkPath), 0o755); err != nil {
		t.Fatalf("create parent for %s: %v", linkPath, err)
	}
	if err := os.Symlink(target, linkPath); err != nil {
		t.Fatalf("symlink %s -> %s: %v", linkPath, target, err)
	}
}
