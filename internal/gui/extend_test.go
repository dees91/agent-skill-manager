package gui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dees91/agent-skill-manager/internal/install"
	"github.com/dees91/agent-skill-manager/internal/model"
	"github.com/dees91/agent-skill-manager/internal/paths"
	"github.com/dees91/agent-skill-manager/internal/state"
)

func extendLocalSourceFixture(t *testing.T, p paths.Paths, dir string, names ...string) {
	t.Helper()
	raw := filepath.Join(p.Home, "workspace", dir)
	for _, name := range names {
		writeSkill(t, filepath.Join(raw, "skills", name), name+" description")
	}
	resolved, err := install.ResolveLocalSource(p, p.Home, raw)
	if err != nil {
		t.Fatalf("ResolveLocalSource() error = %v", err)
	}
	discovered, err := install.DiscoverLocalSkills(resolved)
	if err != nil {
		t.Fatalf("DiscoverLocalSkills() error = %v", err)
	}
	plan, err := install.PlanLocalInstall(p, state.Manifest{}, resolved, discovered, install.PlanOptions{Tools: []model.Tool{model.ToolClaude}})
	if err != nil {
		t.Fatalf("PlanLocalInstall() error = %v", err)
	}
	if _, err := install.NewLocalApplyService(p).Apply(plan); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
}

func disableGUIExtendSkill(t *testing.T, service *Service, name, tool string) {
	t.Helper()
	if _, err := service.ToggleCell(name, tool); err != nil {
		t.Fatalf("ToggleCell(%s) error = %v", name, err)
	}
	result := service.ApplyPending(false)
	if result.Failure != nil {
		t.Fatalf("ApplyPending() failure = %+v", result.Failure)
	}
}

func TestExtendSourcePreviewAfterClaudeOnlyLocalInstall(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	extendLocalSourceFixture(t, p, "shop", "alpha", "beta")
	service := New(p)

	preview, err := service.PreviewExtend("muse")
	if err != nil {
		t.Fatalf("PreviewExtend() error = %v", err)
	}
	if len(preview.Sources) != 1 {
		t.Fatalf("sources = %d, want 1", len(preview.Sources))
	}
	source := preview.Sources[0]
	if source.Kind != "local" {
		t.Fatalf("kind = %q, want local", source.Kind)
	}
	if source.Status != "ready" {
		t.Fatalf("status = %q, want ready", source.Status)
	}
	if source.SkillCount != 2 || len(source.SkillNames) != 2 {
		t.Fatalf("source = %+v, want 2 skills", source)
	}
	if source.Created != 2 || source.AlreadyInstalled != 0 || source.DisabledAfter != 0 {
		t.Fatalf("source = %+v, want only created cells", source)
	}
	if len(source.Skipped) != 0 || len(source.Conflicts) != 0 {
		t.Fatalf("source = %+v, want no skipped or conflicts", source)
	}
	if preview.CreateCount != 2 || preview.BlockedCount != 0 {
		t.Fatalf("preview = %+v, want 2 created and 0 blocked", preview)
	}
	if preview.Tool != "muse" {
		t.Fatalf("Tool = %q, want muse", preview.Tool)
	}
	// Preview must not mutate the home or the manifest.
	if _, err := os.Lstat(filepath.Join(p.MuseUserSkills, "alpha")); !os.IsNotExist(err) {
		t.Fatalf("preview created a muse link")
	}
}

func TestExtendSourceApplyLinksAndMirrorsOff(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	extendLocalSourceFixture(t, p, "shop", "alpha", "beta")
	service := New(p)
	disableGUIExtendSkill(t, service, "beta", "claude")

	applied := service.ExtendSources("muse", false)
	if applied.Failure != nil {
		t.Fatalf("ExtendSources() failure = %+v", applied.Failure)
	}
	if applied.CreatedLinks != 2 {
		t.Fatalf("created = %d, want 2", applied.CreatedLinks)
	}
	wantMessage := "1 source(s) extended to muse: 2 created, 0 already installed, 1 disabled."
	if applied.Message != wantMessage {
		t.Fatalf("message = %q, want %q", applied.Message, wantMessage)
	}
	if target, err := os.Readlink(filepath.Join(p.MuseUserSkills, "alpha")); err != nil {
		t.Fatalf("muse link for alpha missing: %v", err)
	} else if !strings.HasSuffix(target, filepath.Join("shop", "skills", "alpha")) {
		t.Fatalf("muse link target = %q", target)
	}
	if _, err := os.Lstat(filepath.Join(p.MuseDisabledDir, "beta")); err != nil {
		t.Fatalf("mirrored disabled entry for beta missing: %v", err)
	}
	manifest, err := state.New(p).Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(manifest.LocalSources) != 1 {
		t.Fatalf("local sources = %d, want 1", len(manifest.LocalSources))
	}
	for _, name := range []string{"alpha", "beta"} {
		tools := map[model.Tool]bool{}
		for _, skill := range manifest.LocalSources[0].InstalledSkills {
			if skill.Name == name {
				for _, tool := range skill.Tools {
					tools[tool] = true
				}
			}
		}
		if !tools[model.ToolMuse] {
			t.Fatalf("manifest has no muse tool recorded for %s", name)
		}
	}
	if _, ok := manifest.Get(model.ToolMuse, "beta"); !ok {
		t.Fatalf("manifest has no disabled muse record for beta")
	}
	snapshot, err := service.GetSnapshot(false)
	if err != nil {
		t.Fatalf("GetSnapshot() error = %v", err)
	}
	if snapshot.Stats.Muse.On != 1 || snapshot.Stats.Muse.Off != 1 {
		t.Fatalf("muse stats = %+v, want 1 on and 1 off", snapshot.Stats.Muse)
	}

	rerun := service.ExtendSources("muse", false)
	if rerun.Failure != nil {
		t.Fatalf("second ExtendSources() failure = %+v", rerun.Failure)
	}
	if rerun.CreatedLinks != 0 {
		t.Fatalf("second run created = %d, want 0", rerun.CreatedLinks)
	}
}

func TestExtendSourceApplyStopsAtFirstBlockedSource(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	extendLocalSourceFixture(t, p, "aaa-source", "aaa")
	extendLocalSourceFixture(t, p, "zzz-source", "zzz")
	service := New(p)
	if err := os.MkdirAll(p.MuseUserSkills, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p.MuseUserSkills, "aaa"), []byte("blocker"), 0o644); err != nil {
		t.Fatal(err)
	}

	applied := service.ExtendSources("muse", false)
	if applied.Failure == nil {
		t.Fatalf("ExtendSources() = %+v, want conflict failure", applied)
	}
	if !strings.Contains(applied.Failure.Message, "extend --tool muse failed for source") {
		t.Fatalf("failure = %q, want prefixed source failure", applied.Failure.Message)
	}
	if len(applied.Snapshot.Rows) != 2 {
		t.Fatalf("failure snapshot rows = %d, want fresh rescan with 2 managed rows", len(applied.Snapshot.Rows))
	}
	if _, statErr := os.Lstat(filepath.Join(p.MuseUserSkills, "zzz")); !os.IsNotExist(statErr) {
		t.Fatalf("second source was touched after the first failure")
	}
}

func TestExtendSourcePreviewSurfacesBlockedSource(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	extendLocalSourceFixture(t, p, "shop", "alpha")
	service := New(p)
	if err := os.MkdirAll(p.MuseUserSkills, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p.MuseUserSkills, "alpha"), []byte("blocker"), 0o644); err != nil {
		t.Fatal(err)
	}

	preview, err := service.PreviewExtend("muse")
	if err != nil {
		t.Fatalf("PreviewExtend() error = %v", err)
	}
	if len(preview.Sources) != 1 {
		t.Fatalf("sources = %d, want 1", len(preview.Sources))
	}
	source := preview.Sources[0]
	if source.Status != "blocked" {
		t.Fatalf("status = %q, want blocked", source.Status)
	}
	if len(source.Conflicts) == 0 {
		t.Fatalf("source = %+v, want surfaced conflicts", source)
	}
	if preview.CreateCount != 0 || preview.BlockedCount != 1 {
		t.Fatalf("preview = %+v, want 0 created and 1 blocked", preview)
	}
}

func TestExtendSourcePreviewRejectsUnknownTool(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	service := New(p)
	if _, err := service.PreviewExtend("grok"); err == nil {
		t.Fatal("PreviewExtend(grok) succeeded, want error")
	} else if !strings.Contains(err.Error(), "unknown tool") {
		t.Fatalf("error = %q, want unknown tool", err.Error())
	}
	if result := service.ExtendSources("grok", false); result.Failure == nil {
		t.Fatalf("ExtendSources(grok) = %+v, want failure", result)
	} else if !strings.Contains(result.Failure.Message, "unknown tool") {
		t.Fatalf("failure = %q, want unknown tool", result.Failure.Message)
	}
}

func TestExtendSourceApplyBlockedWhilePending(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	extendLocalSourceFixture(t, p, "shop", "alpha")
	service := New(p)
	if _, err := service.ToggleCell("alpha", "claude"); err != nil {
		t.Fatalf("ToggleCell() error = %v", err)
	}
	if result := service.ExtendSources("muse", false); result.Failure == nil {
		t.Fatal("ExtendSources() with pending changes succeeded, want failure")
	} else if !strings.Contains(result.Failure.Message, "pending") {
		t.Fatalf("failure = %q, want pending-changes rejection", result.Failure.Message)
	}
}
