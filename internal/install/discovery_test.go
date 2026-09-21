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
	if len(got.Skills) != 1 {
		t.Fatalf("DiscoverSkills() skills len = %d, want 1: %#v", len(got.Skills), got)
	}
	if len(got.Groups) != 0 {
		t.Fatalf("DiscoverSkills() groups = %#v, want none", got.Groups)
	}
	if got.Skills[0].Name != filepath.Base(checkout) {
		t.Fatalf("Name = %q, want %q", got.Skills[0].Name, filepath.Base(checkout))
	}
	if got.Skills[0].Path != filepath.Clean(checkout) {
		t.Fatalf("Path = %q, want %q", got.Skills[0].Path, filepath.Clean(checkout))
	}
	if got.Skills[0].RelativePath != "." {
		t.Fatalf("RelativePath = %q, want .", got.Skills[0].RelativePath)
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
	assertDiscoveredSkills(t, got.Skills, want)
	if len(got.Groups) != 0 {
		t.Fatalf("DiscoverSkills() groups = %#v, want none", got.Groups)
	}
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
	assertDiscoveredSkills(t, got.Skills, want)
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
	if !got.Empty() {
		t.Fatalf("DiscoverSkills() = %#v, want no symlinked directory skills", got)
	}
}

func TestDiscoverSkillsGroupsIdenticalDuplicateBasenames(t *testing.T) {
	checkout := t.TempDir()
	writeSkill(t, filepath.Join(checkout, ".agent", "skills", "demo"))
	writeSkill(t, filepath.Join(checkout, "plugin", "skills", "demo"))
	writeSkill(t, filepath.Join(checkout, "tests", "fixtures", "demo"))

	got, err := DiscoverSkills(checkout)
	if err != nil {
		t.Fatalf("DiscoverSkills() error = %v", err)
	}
	if len(got.Skills) != 0 {
		t.Fatalf("DiscoverSkills() skills = %#v, want none", got.Skills)
	}
	if len(got.Groups) != 1 {
		t.Fatalf("DiscoverSkills() groups len = %d, want 1: %#v", len(got.Groups), got.Groups)
	}
	group := got.Groups[0]
	if group.Name != "demo" {
		t.Fatalf("Group Name = %q, want demo", group.Name)
	}
	if !group.Identical {
		t.Fatalf("Group Identical = false, want true for same-content copies")
	}
	// Canonical ranking prefers the visible distribution path over the hidden
	// harness copy and the test fixture.
	wantOrder := []string{"plugin/skills/demo", ".agent/skills/demo", "tests/fixtures/demo"}
	if len(group.Candidates) != len(wantOrder) {
		t.Fatalf("Candidates = %#v, want %d copies", group.Candidates, len(wantOrder))
	}
	for i, want := range wantOrder {
		if group.Candidates[i].RelativePath != want {
			t.Fatalf("Candidates[%d] = %q, want %q (full order %#v)", i, group.Candidates[i].RelativePath, want, group.Candidates)
		}
	}
	if len(group.Hashes) != len(wantOrder) {
		t.Fatalf("Hashes len = %d, want %d", len(group.Hashes), len(wantOrder))
	}
	for i, hash := range group.Hashes {
		if hash == "" || hash == "-" {
			t.Fatalf("Hashes[%d] = %q, want a comparable hash", i, hash)
		}
		if hash != group.Hashes[0] {
			t.Fatalf("Hashes[%d] = %q, want %q for identical copies", i, hash, group.Hashes[0])
		}
	}
}

func TestDiscoverSkillsMarksDifferingDuplicatesConflicting(t *testing.T) {
	checkout := t.TempDir()
	writeSkillContent(t, filepath.Join(checkout, "packs", "one", "duplicate"), "# One\n")
	writeSkillContent(t, filepath.Join(checkout, "packs", "two", "duplicate"), "# Two\n")

	got, err := DiscoverSkills(checkout)
	if err != nil {
		t.Fatalf("DiscoverSkills() error = %v", err)
	}
	if len(got.Groups) != 1 {
		t.Fatalf("DiscoverSkills() groups len = %d, want 1", len(got.Groups))
	}
	group := got.Groups[0]
	if group.Identical {
		t.Fatalf("Group Identical = true, want false for differing copies")
	}
	if group.Hashes[0] == group.Hashes[1] {
		t.Fatalf("Hashes = %q, want differing hashes", group.Hashes)
	}
}

func TestDiscoverSkillsTreatsSameSkillFileWithDifferentResourcesAsConflicting(t *testing.T) {
	checkout := t.TempDir()
	first := filepath.Join(checkout, "packs", "one", "duplicate")
	second := filepath.Join(checkout, "packs", "two", "duplicate")
	writeSkill(t, first)
	writeSkill(t, second)
	if err := os.WriteFile(filepath.Join(second, "helper.sh"), []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatalf("write helper: %v", err)
	}

	got, err := DiscoverSkills(checkout)
	if err != nil {
		t.Fatalf("DiscoverSkills() error = %v", err)
	}
	if len(got.Groups) != 1 || got.Groups[0].Identical {
		t.Fatalf("DiscoverSkills() groups = %#v, want one conflicting group", got.Groups)
	}
}

func TestDiscoverSkillsRanksDuplicateCopiesDeterministically(t *testing.T) {
	checkout := t.TempDir()
	// Same content everywhere; only the ranking order is under test.
	for _, dir := range []string{
		"b/deep/nested/demo",
		"a/demo",
		".hidden/demo",
		"tests/demo",
	} {
		writeSkill(t, filepath.Join(checkout, dir))
	}

	got, err := DiscoverSkills(checkout)
	if err != nil {
		t.Fatalf("DiscoverSkills() error = %v", err)
	}
	if len(got.Groups) != 1 {
		t.Fatalf("DiscoverSkills() groups len = %d, want 1", len(got.Groups))
	}
	var order []string
	for _, candidate := range got.Groups[0].Candidates {
		order = append(order, candidate.RelativePath)
	}
	// Visible beats hidden and fixtures; shallower beats deeper; lexical wins ties.
	want := []string{"a/demo", "b/deep/nested/demo", ".hidden/demo", "tests/demo"}
	if strings.Join(order, ",") != strings.Join(want, ",") {
		t.Fatalf("Candidate order = %v, want %v", order, want)
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
	writeSkillContent(t, dir, "# Skill\n")
}

func writeSkillContent(t *testing.T, dir, content string) {
	t.Helper()
	mkdir(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
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
