package favorites

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dees91/agent-skill-manager/internal/paths"
)

func TestStoreSetRoundTripAndIdempotency(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	store := New(p)
	clock := time.Date(2026, 8, 17, 8, 0, 0, 0, time.UTC)
	store.now = func() time.Time {
		clock = clock.Add(time.Nanosecond)
		return clock
	}

	file, err := store.Set(" video-encode ", true)
	if err != nil {
		t.Fatal(err)
	}
	if !file.Contains("video-encode") || len(file.Skills) != 1 {
		t.Fatalf("file = %#v", file)
	}
	if _, err := store.Set("video-encode", true); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Set("media-compose", true); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Set("missing", false); err != nil {
		t.Fatal(err)
	}
	file, err = store.Set("video-encode", false)
	if err != nil {
		t.Fatal(err)
	}
	if file.Contains("video-encode") || !file.Contains("media-compose") {
		t.Fatalf("file = %#v", file)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Skills) != 1 || loaded.Skills[0] != "media-compose" {
		t.Fatalf("loaded = %#v", loaded)
	}
	if mode := fileMode(t, p.StateDir); mode != 0o700 {
		t.Fatalf("state dir mode = %o", mode)
	}
	if mode := fileMode(t, p.FavoritesFile); mode != 0o600 {
		t.Fatalf("favorites mode = %o", mode)
	}
	backups, err := filepath.Glob(filepath.Join(p.BackupDir, "favorites-*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 2 {
		t.Fatalf("backups = %d, want 2", len(backups))
	}
}

func TestStoreNormalizesPersistedSkills(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	if err := os.MkdirAll(p.StateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.FavoritesFile, []byte(`{"version":1,"skills":["zeta","alpha","zeta"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := New(p).Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Skills) != 2 || file.Skills[0] != "alpha" || file.Skills[1] != "zeta" {
		t.Fatalf("skills = %#v", file.Skills)
	}
	if mode := fileMode(t, p.FavoritesFile); mode != 0o600 {
		t.Fatalf("favorites mode = %o", mode)
	}
}

func TestStoreRejectsInvalidFormatAndUnsafePath(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	store := New(p)
	for _, name := range []string{"", ".", "..", "../alpha", "alpha/beta", "alpha\x00beta"} {
		if _, err := store.Set(name, true); err == nil {
			t.Fatalf("expected invalid-name error for %q", name)
		}
	}
	if err := os.MkdirAll(p.StateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.FavoritesFile, []byte(`{"version":99,"skills":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(); err == nil {
		t.Fatal("expected unsupported version error")
	}
	if err := os.WriteFile(p.FavoritesFile, []byte(`{"version":1,"skills":[`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(); err == nil {
		t.Fatal("expected malformed JSON error")
	}
	if err := os.Remove(p.FavoritesFile); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(p.StateDir, "target.json")
	if err := os.WriteFile(target, []byte(`{"version":1,"skills":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, p.FavoritesFile); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(); err == nil {
		t.Fatal("expected unsafe-path error")
	}
}

func TestStoreRetainsBoundedAndRecentBackups(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	store := New(p)
	clock := time.Date(2026, 8, 17, 9, 0, 0, 0, time.UTC)
	store.now = func() time.Time {
		clock = clock.Add(time.Second)
		return clock
	}
	if _, err := store.Set("seed", true); err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 13; index++ {
		if _, err := store.Set("skill-"+time.Unix(int64(index), 0).UTC().Format("150405"), true); err != nil {
			t.Fatal(err)
		}
	}
	backups, err := filepath.Glob(filepath.Join(p.BackupDir, "favorites-*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != maxBackups {
		t.Fatalf("backups = %d, want %d", len(backups), maxBackups)
	}

	expired := filepath.Join(p.BackupDir, "favorites-"+clock.Add(-31*24*time.Hour).Format(backupTimeFormat)+".json")
	if err := os.WriteFile(expired, []byte(`{"version":1,"skills":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Set("latest", true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(expired); !os.IsNotExist(err) {
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
