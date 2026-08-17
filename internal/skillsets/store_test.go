package skillsets

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dees91/agent-skill-manager/internal/paths"
)

func TestStoreCreateUpdateDeleteRoundTrip(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	store := New(p)
	clock := time.Date(2026, 8, 17, 8, 0, 0, 0, time.UTC)
	store.now = func() time.Time {
		clock = clock.Add(time.Nanosecond)
		return clock
	}
	ids := []string{"set-video", "set-writing"}
	store.newID = func() (string, error) {
		id := ids[0]
		ids = ids[1:]
		return id, nil
	}

	created, err := store.Create("  Video production  ", "  Render occasional demos. ", []string{"video-encode", "media-compose", "video-encode"})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID != "set-video" || created.Name != "Video production" || created.Description != "Render occasional demos." {
		t.Fatalf("created = %#v", created)
	}
	if got := created.Skills; len(got) != 2 || got[0] != "media-compose" || got[1] != "video-encode" {
		t.Fatalf("skills = %#v", got)
	}

	if _, err := store.Create("video PRODUCTION", "duplicate", []string{"media-compose"}); err == nil {
		t.Fatal("expected case-insensitive duplicate name error")
	}
	if _, err := store.Create("Writing", "", []string{"docs-review"}); err != nil {
		t.Fatal(err)
	}
	updated, err := store.Update(created.ID, "Video toolkit", "Use for release media.", []string{"media-compose", "missing-later"})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "Video toolkit" || len(updated.Skills) != 2 || !updated.UpdatedAt.After(updated.CreatedAt) {
		t.Fatalf("updated = %#v", updated)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Sets) != 2 || loaded.Sets[0].Name != "Video toolkit" || loaded.Sets[1].Name != "Writing" {
		t.Fatalf("loaded = %#v", loaded.Sets)
	}
	if mode := fileMode(t, p.StateDir); mode != 0o700 {
		t.Fatalf("state dir mode = %o", mode)
	}
	if mode := fileMode(t, p.SkillSetsFile); mode != 0o600 {
		t.Fatalf("Skill Sets mode = %o", mode)
	}

	removed, err := store.Delete(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if removed.ID != created.ID {
		t.Fatalf("removed = %#v", removed)
	}
	loaded, err = store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Sets) != 1 || loaded.Sets[0].Name != "Writing" {
		t.Fatalf("sets after delete = %#v", loaded.Sets)
	}

	backups, err := filepath.Glob(filepath.Join(p.BackupDir, "skill-sets-*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 3 {
		t.Fatalf("backups = %d, want 3", len(backups))
	}
	for _, backup := range backups {
		if mode := fileMode(t, backup); mode != 0o600 {
			t.Fatalf("backup %s mode = %o", backup, mode)
		}
	}
}

func TestStoreValidationAndFormatFailures(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	store := New(p)
	store.newID = func() (string, error) { return "set-one", nil }

	if _, err := store.Create("", "", []string{"alpha"}); err == nil {
		t.Fatal("expected empty name error")
	}
	if _, err := store.Create("Empty", "", nil); err == nil {
		t.Fatal("expected empty members error")
	}
	if _, err := store.Create("Unsafe", "", []string{"../alpha"}); err == nil {
		t.Fatal("expected invalid member error")
	}
	if _, err := store.Create("Null", "", []string{"alpha\x00beta"}); err == nil {
		t.Fatal("expected NUL member error")
	}

	if err := os.MkdirAll(p.StateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.SkillSetsFile, []byte(`{"version":99,"sets":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(); err == nil {
		t.Fatal("expected unsupported version error")
	}
	if err := os.WriteFile(p.SkillSetsFile, []byte(`{"version":1,"sets":[`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(); err == nil {
		t.Fatal("expected malformed JSON error")
	}
}

func TestStoreRetainsBoundedBackups(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	store := New(p)
	clock := time.Date(2026, 8, 17, 9, 0, 0, 0, time.UTC)
	store.now = func() time.Time {
		clock = clock.Add(time.Second)
		return clock
	}
	store.newID = func() (string, error) { return "set-one", nil }
	created, err := store.Create("One", "", []string{"alpha"})
	if err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 13; index++ {
		if _, err := store.Update(created.ID, "One", "revision", []string{"alpha"}); err != nil {
			t.Fatal(err)
		}
	}
	backups, err := filepath.Glob(filepath.Join(p.BackupDir, "skill-sets-*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != maxBackups {
		t.Fatalf("backups = %d, want %d", len(backups), maxBackups)
	}
}

func TestStoreRemovesExpiredBackups(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	store := New(p)
	now := time.Date(2026, 8, 17, 9, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	store.newID = func() (string, error) { return "set-one", nil }
	created, err := store.Create("One", "", []string{"alpha"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(p.BackupDir, 0o700); err != nil {
		t.Fatal(err)
	}
	expiredPath := filepath.Join(p.BackupDir, "skill-sets-"+now.Add(-31*24*time.Hour).Format(backupTimeFormat)+".json")
	if err := os.WriteFile(expiredPath, []byte(`{"version":1,"sets":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Update(created.ID, "One", "updated", []string{"alpha"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(expiredPath); !os.IsNotExist(err) {
		t.Fatalf("expired backup still exists: %v", err)
	}
}

func fileMode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Mode().Perm()
}
