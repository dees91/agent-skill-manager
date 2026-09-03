package gui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dees91/agent-skill-manager/internal/model"
	"github.com/dees91/agent-skill-manager/internal/paths"
	"github.com/dees91/agent-skill-manager/internal/state"
)

func TestEmptySnapshotSerializesCollectionsAsArrays(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	snapshot, err := New(p).GetSnapshot(false)
	if err != nil {
		t.Fatal(err)
	}
	contents, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	var projection map[string]json.RawMessage
	if err := json.Unmarshal(contents, &projection); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"rows", "skillSets", "groups", "sources", "managedSources", "conflicts", "pending"} {
		if string(projection[field]) != "[]" {
			t.Fatalf("empty snapshot field %s = %s, want []", field, projection[field])
		}
	}
}

func TestSnapshotSummariesAndReadOnly(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	writeSkill(t, filepath.Join(p.ClaudeUserSkills, "shared"), "Shared skill")
	writeSkill(t, filepath.Join(p.CodexUserSkills, "shared"), "Shared skill")
	writeSkill(t, filepath.Join(p.CodexUserSkills, "codex-only"), "Codex only")
	writeSkill(t, filepath.Join(p.CodexSystemSkills, "system"), "System")

	service := New(p)
	snapshot, err := service.GetSnapshot(false)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Stats.ManagedSkills != 2 || snapshot.Stats.Claude.On != 1 || snapshot.Stats.Codex.On != 2 {
		t.Fatalf("stats = %#v", snapshot.Stats)
	}
	if len(snapshot.Rows) != 2 || snapshot.IncludeReadOnly {
		t.Fatalf("snapshot = %#v", snapshot)
	}

	withReadOnly, err := service.GetSnapshot(true)
	if err != nil {
		t.Fatal(err)
	}
	if len(withReadOnly.Rows) != 3 || withReadOnly.Stats.Codex.ReadOnly != 1 || withReadOnly.Stats.ReadOnlySkills != 1 {
		t.Fatalf("read-only snapshot = %#v", withReadOnly)
	}
}

func TestFirstSnapshotSecuresStateAndSanitizesLegacyCatalogCache(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	if err := os.MkdirAll(filepath.Dir(p.SkillsSHCacheFile), 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := `{"version":1,"pages":{},"searches":{"private query":{"view":"search","total":0,"skills":[]}},"details":{}}`
	if err := os.WriteFile(p.SkillsSHCacheFile, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := New(p).GetSnapshot(false); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(p.SkillsSHCacheFile)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(contents), "private query") || !strings.Contains(string(contents), `"version": 2`) {
		t.Fatalf("catalog privacy migration failed: %s", contents)
	}
	for _, path := range []string{p.StateDir, filepath.Dir(p.SkillsSHCacheFile)} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat directory %s: %v", path, err)
		}
		if info.Mode().Perm() != 0o700 {
			t.Fatalf("directory %s mode = %v", path, info.Mode().Perm())
		}
	}
	info, err := os.Stat(p.SkillsSHCacheFile)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("catalog cache mode = %v", info.Mode().Perm())
	}
}

func TestSnapshotProjectsOnlyManifestOwnedSourcesWithOpaqueIDs(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	repository := state.RepositoryEntry{
		OriginalURL: "https://github.com/demo/skills", Host: "github.com", RepoPath: "demo/skills",
		CheckoutPath: filepath.Join(p.ReposDir, "github.com", "demo", "skills"), Group: "demo/skills", InstalledAt: time.Unix(100, 0).UTC(),
		InstalledSkills: []state.InstalledSkillEntry{{Name: "alpha", RelativePath: "alpha", Tools: []model.Tool{model.ToolClaude}}},
	}
	local := state.LocalSourceEntry{OriginalPath: filepath.Join(p.Home, "src"), CanonicalPath: filepath.Join(p.Home, "src"), Group: "src", InstalledAt: time.Unix(200, 0).UTC(), InstalledSkills: []state.InstalledSkillEntry{{Name: "beta", RelativePath: "beta", Tools: []model.Tool{model.ToolCodex}}}}
	if err := state.New(p).Save(state.Manifest{Repositories: []state.RepositoryEntry{repository}, LocalSources: []state.LocalSourceEntry{local}}); err != nil {
		t.Fatal(err)
	}
	snapshot, err := New(p).GetSnapshot(false)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.ManagedSources) != 2 {
		t.Fatalf("managed sources = %#v", snapshot.ManagedSources)
	}
	for _, source := range snapshot.ManagedSources {
		if source.SourceID == "" || strings.Contains(source.SourceID, p.Home) {
			t.Fatalf("source ID leaked a path: %#v", source)
		}
		switch source.Kind {
		case sourceKindGit:
			if source.UpdateMode != "Managed Git" || source.UpdateHint != "Use Update to fetch changes." {
				t.Fatalf("Git update behavior = %#v", source)
			}
		case sourceKindLocal:
			if source.UpdateMode != "Linked folder" || source.UpdateHint != "Changes are read directly; no update needed." {
				t.Fatalf("local update behavior = %#v", source)
			}
		default:
			t.Fatalf("unexpected managed source kind: %#v", source)
		}
	}
}

func TestLocalInstallReviewApplyAndTypedUninstall(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	sourcePath := filepath.Join(p.Home, "workspace", "local-pack")
	writeSkill(t, filepath.Join(sourcePath, "skills", "alpha"), "Alpha")
	service := New(p)
	draft, err := service.PrepareLocalInstall(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if draft.Kind != "local" || len(draft.Candidates) != 1 {
		t.Fatalf("draft = %#v", draft)
	}
	review, err := service.ReviewInstall(draft.DraftID, []InstallCellRequest{{SkillName: "alpha", Tool: "claude"}})
	if err != nil || !review.Ready || review.CreateCount != 1 {
		t.Fatalf("review = %#v err=%v", review, err)
	}
	result := service.ApplyInstall(review.ReviewID, false)
	if result.Failure != nil || result.CreatedLinks != 1 || len(result.Snapshot.ManagedSources) != 1 {
		t.Fatalf("apply = %#v", result)
	}
	if _, err := os.Lstat(filepath.Join(p.CodexUserSkills, "alpha")); !os.IsNotExist(err) {
		t.Fatalf("unselected Codex cell was created or returned unexpected error: %v", err)
	}
	sourceID := result.Snapshot.ManagedSources[0].SourceID
	preview, err := service.PreviewUninstall(sourceID)
	if err != nil || preview.ActiveLinks != 1 || !preview.PreservesSource {
		t.Fatalf("preview = %#v err=%v", preview, err)
	}
	blocked := service.UninstallSource(sourceID, "wrong", false)
	if blocked.Failure == nil {
		t.Fatal("uninstall accepted wrong confirmation")
	}
	removed := service.UninstallSource(sourceID, "local-pack", false)
	if removed.Failure != nil || removed.RemovedActive != 1 || len(removed.Snapshot.ManagedSources) != 0 {
		t.Fatalf("uninstall = %#v", removed)
	}
	if _, err := os.Lstat(filepath.Join(sourcePath, "skills", "alpha", "SKILL.md")); err != nil {
		t.Fatalf("local source was not preserved: %v", err)
	}
}

func TestGitInstallInspectionUsesOpaqueDraftAndExactCellApply(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	service := New(p)
	service.gitRunner = guiCloneRunner{t: t}

	draft, err := service.PrepareGitInstall("https://github.com/demo/gui-skills")
	if err != nil {
		t.Fatal(err)
	}
	if !draft.Cloned || draft.DraftID == "" || strings.Contains(draft.DraftID, p.Home) || len(draft.Candidates) != 1 {
		t.Fatalf("draft = %#v", draft)
	}
	review, err := service.ReviewInstall(draft.DraftID, []InstallCellRequest{{SkillName: "alpha", Tool: "codex"}})
	if err != nil || !review.Ready || review.CreateCount != 1 {
		t.Fatalf("review = %#v err=%v", review, err)
	}
	result := service.ApplyInstall(review.ReviewID, false)
	if result.Failure != nil || result.CreatedLinks != 1 || len(result.Snapshot.ManagedSources) != 1 {
		t.Fatalf("apply = %#v", result)
	}
	if _, err := os.Lstat(filepath.Join(p.ClaudeUserSkills, "alpha")); !os.IsNotExist(err) {
		t.Fatalf("unselected Claude cell was created or returned unexpected error: %v", err)
	}
	if target, err := os.Readlink(filepath.Join(p.CodexUserSkills, "alpha")); err != nil || !strings.HasSuffix(target, filepath.Join("gui-skills", "skills", "alpha")) {
		t.Fatalf("Codex link target = %q err=%v", target, err)
	}
}

func TestSourceOperationsRejectPendingSkillChanges(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	writeSkill(t, filepath.Join(p.ClaudeUserSkills, "alpha"), "Alpha")
	service := New(p)
	if _, err := service.GetSnapshot(false); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ToggleCell("alpha", "claude"); err != nil {
		t.Fatal(err)
	}
	localSource := filepath.Join(p.Home, "workspace", "pack")
	writeSkill(t, localSource, "Pack")
	if _, err := service.PrepareLocalInstall(localSource); err == nil || !strings.Contains(err.Error(), "pending") {
		t.Fatalf("PrepareLocalInstall() error = %v, want pending guard", err)
	}
}

func TestUpdateAllStopsAtFirstRepositoryInManifestOrder(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	manifest := state.Manifest{Repositories: []state.RepositoryEntry{
		{OriginalURL: "https://github.com/zeta/skills", Host: "github.com", RepoPath: "zeta/skills", CheckoutPath: filepath.Join(p.ReposDir, "github.com", "zeta", "skills"), Group: "zeta/skills"},
		{OriginalURL: "https://github.com/alpha/skills", Host: "github.com", RepoPath: "alpha/skills", CheckoutPath: filepath.Join(p.ReposDir, "github.com", "alpha", "skills"), Group: "alpha/skills"},
	}}
	if err := state.New(p).Save(manifest); err != nil {
		t.Fatal(err)
	}
	service := New(p)
	result := service.UpdateAllSources(false)
	if result.Failure == nil || result.Failure.Group != "alpha/skills" {
		t.Fatalf("failure = %#v, want first normalized repository", result.Failure)
	}
	if len(result.Completed) != 0 || !strings.Contains(result.Message, "after 0") {
		t.Fatalf("result = %#v, want empty completed prefix", result)
	}
}

type guiCloneRunner struct{ t *testing.T }

func (r guiCloneRunner) RunGit(args ...string) (string, error) {
	r.t.Helper()
	if len(args) == 3 && args[0] == "clone" {
		writeSkill(r.t, filepath.Join(args[2], "skills", "alpha"), "Alpha")
		return "", nil
	}
	if len(args) == 4 && args[0] == "-C" && args[2] == "rev-parse" && args[3] == "HEAD" {
		return "abc123", nil
	}
	return "", fmt.Errorf("unexpected git command: %v", args)
}

func TestToggleGroupAndApplyUsePendingSession(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	writeSkill(t, filepath.Join(p.ClaudeUserSkills, "alpha"), "Alpha")
	writeSkill(t, filepath.Join(p.CodexUserSkills, "alpha"), "Alpha")
	writeSkill(t, filepath.Join(p.MuseUserSkills, "alpha"), "Alpha")
	writeSkill(t, filepath.Join(p.GrokUserSkills, "alpha"), "Alpha")

	service := New(p)
	snapshot, err := service.GetSnapshot(false)
	if err != nil {
		t.Fatal(err)
	}
	group := snapshot.Rows[0].Group
	action, err := service.ToggleGroup(group)
	if err != nil {
		t.Fatal(err)
	}
	if len(action.Pending) != 4 || service.PendingCount() != 4 {
		t.Fatalf("action = %#v", action)
	}
	if !action.ContextBudgets.Claude.ProjectionChanged || !action.ContextBudgets.Codex.ProjectionChanged || !action.ContextBudgets.Muse.ProjectionChanged || !action.ContextBudgets.Grok.ProjectionChanged {
		t.Fatalf("context projections were not marked changed: %#v", action.ContextBudgets)
	}
	if action.ContextBudgets.Claude.Projected.RequestedCharacters >= action.ContextBudgets.Claude.Current.RequestedCharacters ||
		action.ContextBudgets.Codex.Projected.RequestedCharacters >= action.ContextBudgets.Codex.Current.RequestedCharacters ||
		action.ContextBudgets.Muse.Projected.RequestedCharacters >= action.ContextBudgets.Muse.Current.RequestedCharacters ||
		action.ContextBudgets.Grok.Projected.RequestedCharacters >= action.ContextBudgets.Grok.Current.RequestedCharacters {
		t.Fatalf("disable projections did not reduce catalog cost: %#v", action.ContextBudgets)
	}

	result := service.ApplyPending(false)
	if result.Failure != nil || len(result.Completed) != 4 || len(result.Snapshot.Pending) != 0 {
		t.Fatalf("apply = %#v", result)
	}
	if result.Snapshot.ContextBudgets.Claude.ProjectionChanged || result.Snapshot.ContextBudgets.Codex.ProjectionChanged || result.Snapshot.ContextBudgets.Muse.ProjectionChanged || result.Snapshot.ContextBudgets.Grok.ProjectionChanged {
		t.Fatalf("applied snapshot still has a projection: %#v", result.Snapshot.ContextBudgets)
	}
	for _, tool := range model.Tools() {
		active, _ := p.UserSkillsDirFor(tool)
		disabled, _ := p.DisabledDirFor(tool)
		if _, err := os.Lstat(filepath.Join(active, "alpha")); !os.IsNotExist(err) {
			t.Fatalf("active %s err = %v, want missing", tool, err)
		}
		if _, err := os.Lstat(filepath.Join(disabled, "alpha")); err != nil {
			t.Fatalf("disabled %s: %v", tool, err)
		}
	}
}

func TestScopedBulkTogglesOnlySelectedToolsAndDeduplicatesInputs(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	for _, skill := range []string{"alpha", "beta"} {
		writeSkill(t, filepath.Join(p.ClaudeUserSkills, skill), skill)
		writeSkill(t, filepath.Join(p.CodexUserSkills, skill), skill)
	}

	service := New(p)
	snapshot, err := service.GetSnapshot(false)
	if err != nil {
		t.Fatal(err)
	}
	group := snapshot.Rows[0].Group
	action, err := service.ToggleGroupScope(group, []string{"codex", "codex"})
	if err != nil {
		t.Fatal(err)
	}
	if action.Counts.Changed != 2 || len(action.Pending) != 2 {
		t.Fatalf("group action = %#v", action)
	}
	for _, pending := range action.Pending {
		if pending.Tool != "codex" {
			t.Fatalf("unexpected pending tool: %#v", pending)
		}
	}

	action, err = service.ToggleSkillScope([]string{"alpha", "alpha", "beta"}, []string{"claude"})
	if err != nil {
		t.Fatal(err)
	}
	if action.Counts.Changed != 2 || len(action.Pending) != 4 {
		t.Fatalf("skill action = %#v", action)
	}
}

func TestScopedBulkRejectsUnknownSkillsAndTools(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	writeSkill(t, filepath.Join(p.ClaudeUserSkills, "alpha"), "Alpha")
	service := New(p)
	if _, err := service.GetSnapshot(false); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ToggleSkillScope([]string{"alpha"}, nil); err == nil {
		t.Fatal("ToggleSkillScope accepted an empty tool scope")
	}
	if _, err := service.ToggleSkillScope([]string{"alpha"}, []string{"other"}); err == nil {
		t.Fatal("ToggleSkillScope accepted an unknown tool")
	}
	if _, err := service.ToggleSkillScope([]string{"missing"}, []string{"claude"}); err == nil {
		t.Fatal("ToggleSkillScope accepted an unknown skill")
	}
}

func TestApplyPreflightFailurePreservesPending(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	active := filepath.Join(p.ClaudeUserSkills, "alpha")
	writeSkill(t, active, "Alpha")
	service := New(p)
	if _, err := service.GetSnapshot(false); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ToggleCell("alpha", "claude"); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(active); err != nil {
		t.Fatal(err)
	}

	result := service.ApplyPending(false)
	if result.Failure == nil || result.Failure.Stage != "preflight" || service.PendingCount() != 1 {
		t.Fatalf("result = %#v, pending=%d", result, service.PendingCount())
	}
}

func TestToggleRejectsFrontendPathsAndUnknownRows(t *testing.T) {
	service := New(paths.ForHome(t.TempDir()))
	if _, err := service.ToggleCell("/tmp/not-a-skill", "claude"); err == nil {
		t.Fatal("ToggleCell accepted an unknown path-like identifier")
	}
	if _, err := service.ToggleCell("anything", "unknown"); err == nil {
		t.Fatal("ToggleCell accepted an unknown tool")
	}
}

func TestToggleBothReportsReadOnlyAndMissingSkips(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	writeSkill(t, filepath.Join(p.CodexSystemSkills, "system"), "System")
	service := New(p)
	if _, err := service.GetSnapshot(true); err != nil {
		t.Fatal(err)
	}

	result, err := service.ToggleBoth("system")
	if err != nil {
		t.Fatal(err)
	}
	if result.Counts.SkippedReadOnly != 1 || result.Counts.SkippedMissing != 3 || len(result.Pending) != 0 {
		t.Fatalf("result = %#v", result)
	}
}

func writeSkill(t *testing.T, dir, description string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: " + filepath.Base(dir) + "\ndescription: " + description + "\n---\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
