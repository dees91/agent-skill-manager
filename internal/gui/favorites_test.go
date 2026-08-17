package gui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dees91/agent-skill-manager/internal/model"
	"github.com/dees91/agent-skill-manager/internal/paths"
)

func TestFavoriteMutationProjectsAndPreservesPending(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	writeSkill(t, filepath.Join(p.ClaudeUserSkills, "alpha"), "Alpha")
	service := New(p)
	if _, err := service.GetSnapshot(false); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ToggleCell("alpha", "claude"); err != nil {
		t.Fatal(err)
	}
	result, err := service.SetSkillFavorite("alpha", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Favorites) != 1 || result.Favorites[0] != "alpha" || service.PendingCount() != 1 {
		t.Fatalf("result = %#v pending=%d", result, service.PendingCount())
	}
	snapshot := service.snapshotLocked()
	if len(snapshot.Rows) != 1 || !snapshot.Rows[0].Favorite || len(snapshot.Pending) != 1 {
		t.Fatalf("snapshot = %#v", snapshot)
	}

	result, err = service.SetSkillFavorite("alpha", true)
	if err != nil || len(result.Favorites) != 1 {
		t.Fatalf("idempotent add = %#v err=%v", result, err)
	}
	result, err = service.SetSkillFavorite("alpha", false)
	if err != nil || len(result.Favorites) != 0 || service.PendingCount() != 1 {
		t.Fatalf("remove = %#v pending=%d err=%v", result, service.PendingCount(), err)
	}
}

func TestFavoriteEligibilityIncludesConflictAndExcludesReadOnly(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	writeSkill(t, filepath.Join(p.CodexSystemSkills, "system"), "System")
	service := New(p)
	if _, err := service.GetSnapshot(true); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetSkillFavorite("system", true); err == nil || !strings.Contains(err.Error(), "managed user skill") {
		t.Fatalf("read-only favorite error = %v", err)
	}

	service.rows = append(service.rows, model.SkillRow{
		Name: "blocked",
		Codex: &model.ToolSkill{
			Name: "blocked", Tool: model.ToolCodex, State: model.SkillStateConflict,
			Conflict: &model.Conflict{Message: "Restore blocked."},
		},
	})
	if _, err := service.SetSkillFavorite("blocked", true); err != nil {
		t.Fatalf("managed conflict was rejected: %v", err)
	}
	rows := service.snapshotLocked().Rows
	favoriteByName := map[string]bool{}
	for _, row := range rows {
		favoriteByName[row.Name] = row.Favorite
	}
	if len(rows) != 2 || !favoriteByName["blocked"] || favoriteByName["system"] {
		t.Fatalf("rows = %#v", rows)
	}
}

func TestFavoriteCorruptionIsIsolatedFromCoreSkills(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	writeSkill(t, filepath.Join(p.ClaudeUserSkills, "alpha"), "Alpha")
	if err := os.MkdirAll(p.StateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.FavoritesFile, []byte(`{"version":1,"skills":[`), 0o600); err != nil {
		t.Fatal(err)
	}
	service := New(p)
	snapshot, err := service.GetSnapshot(false)
	if err != nil {
		t.Fatalf("core snapshot was blocked: %v", err)
	}
	if len(snapshot.Rows) != 1 || snapshot.FavoritesWarning == "" || snapshot.Rows[0].Favorite {
		t.Fatalf("snapshot = %#v", snapshot)
	}
	if _, err := service.ToggleCell("alpha", "claude"); err != nil {
		t.Fatalf("core toggle was blocked: %v", err)
	}
	if _, err := service.SetSkillFavorite("alpha", true); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("SetSkillFavorite() error = %v", err)
	}
}

func TestFavoriteReconnectsByBasenameAndRemovalAllowsMissingName(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	active := filepath.Join(p.ClaudeUserSkills, "alpha")
	writeSkill(t, active, "Alpha")
	service := New(p)
	if _, err := service.GetSnapshot(false); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetSkillFavorite("alpha", true); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(active); err != nil {
		t.Fatal(err)
	}
	snapshot, err := service.GetSnapshot(false)
	if err != nil || len(snapshot.Rows) != 0 {
		t.Fatalf("missing snapshot = %#v err=%v", snapshot, err)
	}
	writeSkill(t, active, "Alpha reinstalled")
	snapshot, err = service.GetSnapshot(false)
	if err != nil || len(snapshot.Rows) != 1 || !snapshot.Rows[0].Favorite {
		t.Fatalf("reconnected snapshot = %#v err=%v", snapshot, err)
	}
	if err := os.RemoveAll(active); err != nil {
		t.Fatal(err)
	}
	if _, err := service.GetSnapshot(false); err != nil {
		t.Fatal(err)
	}
	result, err := service.SetSkillFavorite("alpha", false)
	if err != nil || len(result.Favorites) != 0 {
		t.Fatalf("remove missing favorite = %#v err=%v", result, err)
	}
}

func TestSourceUninstallReportsAndRetainsFavorite(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	sourcePath := filepath.Join(p.Home, "workspace", "media-pack")
	writeSkill(t, filepath.Join(sourcePath, "skills", "media-compose"), "Compose media")
	service := New(p)
	draft, err := service.PrepareLocalInstall(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	review, err := service.ReviewInstall(draft.DraftID, []InstallCellRequest{{SkillName: "media-compose", Tool: "codex"}})
	if err != nil {
		t.Fatal(err)
	}
	installed := service.ApplyInstall(review.ReviewID, false)
	if installed.Failure != nil {
		t.Fatalf("install = %#v", installed)
	}
	if _, err := service.SetSkillFavorite("media-compose", true); err != nil {
		t.Fatal(err)
	}
	sourceID := installed.Snapshot.ManagedSources[0].SourceID
	preview, err := service.PreviewUninstall(sourceID)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.AffectedFavorites) != 1 || preview.AffectedFavorites[0] != "media-compose" || preview.FavoriteImpactWarning != "" {
		t.Fatalf("preview = %#v", preview)
	}
	removed := service.UninstallSource(sourceID, "media-pack", false)
	if removed.Failure != nil || len(removed.Snapshot.Rows) != 0 {
		t.Fatalf("uninstall = %#v", removed)
	}
	file, err := service.favoriteStore.Load()
	if err != nil || !file.Contains("media-compose") {
		t.Fatalf("retained favorites = %#v err=%v", file, err)
	}
}

func TestFavoriteMutationRejectsBusySourceOperation(t *testing.T) {
	service := New(paths.ForHome(t.TempDir()))
	service.sourceBusy.Store(true)
	defer service.sourceBusy.Store(false)
	if _, err := service.SetSkillFavorite("alpha", true); err == nil || !strings.Contains(err.Error(), "source operation") {
		t.Fatalf("busy error = %v", err)
	}
}
