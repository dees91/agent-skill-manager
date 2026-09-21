package install

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestFingerprintSkillDirMatchesIdenticalCopies(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "one", "demo")
	second := filepath.Join(root, "two", "demo")
	writeSkill(t, first)
	writeSkill(t, second)
	for _, dir := range []string{first, second} {
		if err := os.WriteFile(filepath.Join(dir, "helper.sh"), []byte("#!/bin/sh\necho hi\n"), 0o644); err != nil {
			t.Fatalf("write helper: %v", err)
		}
	}

	left, err := fingerprintSkillDir(first)
	if err != nil {
		t.Fatalf("fingerprintSkillDir() error = %v", err)
	}
	right, err := fingerprintSkillDir(second)
	if err != nil {
		t.Fatalf("fingerprintSkillDir() error = %v", err)
	}
	if left.hash == "" || right.hash == "" {
		t.Fatalf("hashes = %q, %q, want comparable hashes", left.hash, right.hash)
	}
	if left.hash != right.hash {
		t.Fatalf("hashes differ for identical copies: %q vs %q", left.hash, right.hash)
	}
	if left.short() == "-" || len(left.short()) != 12 {
		t.Fatalf("short() = %q, want 12 hex chars", left.short())
	}
}

func TestFingerprintSkillDirDetectsContentDifferences(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "one", "demo")
	second := filepath.Join(root, "two", "demo")
	writeSkillContent(t, first, "# One\n")
	writeSkillContent(t, second, "# Two\n")

	left, err := fingerprintSkillDir(first)
	if err != nil {
		t.Fatalf("fingerprintSkillDir() error = %v", err)
	}
	right, err := fingerprintSkillDir(second)
	if err != nil {
		t.Fatalf("fingerprintSkillDir() error = %v", err)
	}
	if left.hash == right.hash {
		t.Fatalf("hashes match for differing copies: %q", left.hash)
	}
}

func TestFingerprintSkillDirCoversSymlinksByTargetText(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "one", "demo")
	second := filepath.Join(root, "two", "demo")
	writeSkill(t, first)
	writeSkill(t, second)
	if err := os.Symlink("SKILL.md", filepath.Join(first, "alias.md")); err != nil {
		t.Fatalf("create symlink: %v", err)
	}
	if err := os.Symlink("other.md", filepath.Join(second, "alias.md")); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	left, err := fingerprintSkillDir(first)
	if err != nil {
		t.Fatalf("fingerprintSkillDir() error = %v", err)
	}
	right, err := fingerprintSkillDir(second)
	if err != nil {
		t.Fatalf("fingerprintSkillDir() error = %v", err)
	}
	if left.hash == right.hash {
		t.Fatalf("hashes match for differing symlinks")
	}
}

func TestFingerprintSkillDirSkipsComparisonBeyondFileCap(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "demo")
	writeSkill(t, dir)
	for i := 0; i <= maxFingerprintFiles; i++ {
		name := filepath.Join(dir, fmt.Sprintf("file-%04d.txt", i))
		if err := os.WriteFile(name, []byte("x"), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}

	got, err := fingerprintSkillDir(dir)
	if err != nil {
		t.Fatalf("fingerprintSkillDir() error = %v", err)
	}
	if got.hash != "" {
		t.Fatalf("hash = %q, want empty for an oversized skill", got.hash)
	}
	if got.short() != "-" {
		t.Fatalf("short() = %q, want -", got.short())
	}
}

func TestOversizedDuplicateGroupIsConflicting(t *testing.T) {
	checkout := t.TempDir()
	for _, dir := range []string{filepath.Join(checkout, "one", "demo"), filepath.Join(checkout, "two", "demo")} {
		writeSkill(t, dir)
		for i := 0; i <= maxFingerprintFiles; i++ {
			name := filepath.Join(dir, fmt.Sprintf("file-%04d.txt", i))
			if err := os.WriteFile(name, []byte("x"), 0o644); err != nil {
				t.Fatalf("write file: %v", err)
			}
		}
	}

	got, err := DiscoverSkills(checkout)
	if err != nil {
		t.Fatalf("DiscoverSkills() error = %v", err)
	}
	if len(got.Groups) != 1 || got.Groups[0].Identical {
		t.Fatalf("DiscoverSkills() groups = %#v, want one conflicting group", got.Groups)
	}
}
