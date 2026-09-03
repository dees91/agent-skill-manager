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
	if source.SkillCount != 2 || len(source.SkillNames) != 2 {
		t.Fatalf("source = %+v, want 2 skills", source)
	}
	if source.Created != 2 || source.AlreadyInstalled != 0 || source.DisabledAfter != 0 {
		t.Fatalf("source = %+v, want only created cells", source)
	}
	if preview.MuseCount != 1 {
		t.Fatalf("MuseCount = %d, want 1", preview.MuseCount)
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

	applied, err := service.ExtendSources("muse")
	if err != nil {
		t.Fatalf("ExtendSources() error = %v", err)
	}
	if applied.CreatedLinks != 2 {
		t.Fatalf("created = %d, want 2", applied.CreatedLinks)
	}
	wantMessage := "1 source(s) extended to muse: 2 created, 0 already installed.; 1 disabled."
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

	rerun, err := service.ExtendSources("muse")
	if err != nil {
		t.Fatalf("second ExtendSources() error = %v", err)
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

	applied, err := service.ExtendSources("muse")
	if err == nil {
		t.Fatalf("ExtendSources() = %+v, want conflict error", applied)
	}
	if !strings.Contains(err.Error(), "extend --tool muse failed for source") {
		t.Fatalf("error = %q, want prefixed source failure", err.Error())
	}
	if _, statErr := os.Lstat(filepath.Join(p.MuseUserSkills, "zzz")); !os.IsNotExist(statErr) {
		t.Fatalf("second source was touched after the first failure")
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
	if _, err := service.ExtendSources("grok"); err == nil {
		t.Fatal("ExtendSources(grok) succeeded, want error")
	}
}

func TestExtendSourceApplyBlockedWhilePending(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	extendLocalSourceFixture(t, p, "shop", "alpha")
	service := New(p)
	if _, err := service.ToggleCell("alpha", "claude"); err != nil {
		t.Fatalf("ToggleCell() error = %v", err)
	}
	if _, err := service.ExtendSources("muse"); err == nil {
		t.Fatal("ExtendSources() with pending changes succeeded, want error")
	} else if !strings.Contains(err.Error(), "pending") {
		t.Fatalf("error = %q, want pending-changes rejection", err.Error())
	}
}
