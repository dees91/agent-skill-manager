package install

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDiscoverSkillsFindsRootSkill(t *testing.T) {
	checkout := t.TempDir()
	writeSkill(t, checkout)

	got, err := DiscoverSkills(checkout)
	if err != nil {
		t.Fatalf("DiscoverSkills() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("DiscoverSkills() len = %d, want 1: %#v", len(got), got)
	}
	if got[0].Name != filepath.Base(checkout) {
		t.Fatalf("Name = %q, want %q", got[0].Name, filepath.Base(checkout))
	}
	if got[0].Path != filepath.Clean(checkout) {
		t.Fatalf("Path = %q, want %q", got[0].Path, filepath.Clean(checkout))
	}
	if got[0].RelativePath != "." {
		t.Fatalf("RelativePath = %q, want .", got[0].RelativePath)
	}
}

func TestDiscoverSkillsFindsNestedSkillsAndSkipsInvalidDirs(t *testing.T) {
	checkout := t.TempDir()
	writeSkill(t, filepath.Join(checkout, "skills", "zeta"))
	writeSkill(t, filepath.Join(checkout, "skills", "alpha"))
	mkdir(t, filepath.Join(checkout, "skills", "invalid"))

	got, err := DiscoverSkills(checkout)
	if err != nil {
		t.Fatalf("DiscoverSkills() error = %v", err)
	}

	want := []DiscoveredSkill{
		{
			Name:         "alpha",
			Path:         filepath.Join(checkout, "skills", "alpha"),
			RelativePath: "skills/alpha",
		},
		{
			Name:         "zeta",
			Path:         filepath.Join(checkout, "skills", "zeta"),
			RelativePath: "skills/zeta",
		},
	}
	assertDiscoveredSkills(t, got, want)
}

func TestDiscoverSkillsIgnoresHeavyGeneratedDirectories(t *testing.T) {
	checkout := t.TempDir()
	for _, ignored := range []string{".git", "node_modules", ".venv", "vendor", "build", "dist"} {
		writeSkill(t, filepath.Join(checkout, ignored, "hidden-skill"))
	}
	writeSkill(t, filepath.Join(checkout, "skills", "visible"))

	got, err := DiscoverSkills(checkout)
	if err != nil {
		t.Fatalf("DiscoverSkills() error = %v", err)
	}

	want := []DiscoveredSkill{
		{
			Name:         "visible",
			Path:         filepath.Join(checkout, "skills", "visible"),
			RelativePath: "skills/visible",
		},
	}
	assertDiscoveredSkills(t, got, want)
}

func TestDiscoverSkillsDoesNotTraverseSymlinkedDirectories(t *testing.T) {
	checkout := t.TempDir()
	outside := t.TempDir()
	writeSkill(t, filepath.Join(outside, "linked-skill"))
	if err := os.Symlink(filepath.Join(outside, "linked-skill"), filepath.Join(checkout, "linked-skill")); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	got, err := DiscoverSkills(checkout)
	if err != nil {
		t.Fatalf("DiscoverSkills() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("DiscoverSkills() = %#v, want no symlinked directory skills", got)
	}
}

func TestDiscoverSkillsRejectsDuplicateSkillBasenames(t *testing.T) {
	checkout := t.TempDir()
	writeSkill(t, filepath.Join(checkout, "packs", "one", "duplicate"))
	writeSkill(t, filepath.Join(checkout, "packs", "two", "duplicate"))

	_, err := DiscoverSkills(checkout)
	if err == nil {
		t.Fatal("DiscoverSkills() error = nil, want duplicate error")
	}
	message := err.Error()
	for _, want := range []string{
		"duplicate skill names discovered",
		"duplicate",
		"packs/one/duplicate",
		"packs/two/duplicate",
	} {
		if !strings.Contains(message, want) {
			t.Fatalf("DiscoverSkills() error = %q, want substring %q", message, want)
		}
	}
}

func TestDiscoverSkillsRejectsInvalidCheckoutPath(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	if _, err := DiscoverSkills(missing); err == nil {
		t.Fatal("DiscoverSkills(missing) error = nil, want error")
	}

	fileCheckout := filepath.Join(t.TempDir(), "checkout-file")
	if err := os.WriteFile(fileCheckout, []byte("not a dir"), 0o644); err != nil {
		t.Fatalf("write checkout file: %v", err)
	}
	_, err := DiscoverSkills(fileCheckout)
	if err == nil {
		t.Fatal("DiscoverSkills(file) error = nil, want error")
	}
	if !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("DiscoverSkills(file) error = %q, want not a directory", err)
	}
}

func TestDiscoverSkillsReportsUnreadableDirectories(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod permissions are not reliable for this test on Windows")
	}
	checkout := t.TempDir()
	unreadable := filepath.Join(checkout, "unreadable")
	mkdir(t, unreadable)
	if err := os.Chmod(unreadable, 0); err != nil {
		t.Fatalf("chmod unreadable dir: %v", err)
	}
	defer func() {
		_ = os.Chmod(unreadable, 0o755)
	}()

	_, err := DiscoverSkills(checkout)
	if err == nil {
		t.Fatal("DiscoverSkills(unreadable child) error = nil, want error")
	}
	if !strings.Contains(err.Error(), "discover skills") || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("DiscoverSkills(unreadable child) error = %q, want discovery permission error", err)
	}
}

func writeSkill(t *testing.T, dir string) {
	t.Helper()
	mkdir(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# Skill\n"), 0o644); err != nil {
		t.Fatalf("write SKILL.md in %s: %v", dir, err)
	}
}

func mkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create dir %s: %v", dir, err)
	}
}

func assertDiscoveredSkills(t *testing.T, got, want []DiscoveredSkill) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("DiscoverSkills() len = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("DiscoverSkills()[%d] = %#v, want %#v", i, got[i], want[i])
		}
	}
}
