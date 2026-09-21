package install

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/dees91/agent-skill-manager/internal/state"
)

func TestNewSkillsReportsUnrecordedNamesOnly(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, filepath.Join(root, "skills/alpha"))
	writeSkill(t, filepath.Join(root, "skills/moved/beta"))
	writeSkill(t, filepath.Join(root, "skills/gamma"))
	writeSkill(t, filepath.Join(root, "skills/in-progress/delta"))
	repository := state.RepositoryEntry{
		CheckoutPath: root,
		InstalledSkills: []state.InstalledSkillEntry{
			{Name: "alpha", RelativePath: "skills/alpha"},
			{Name: "beta", RelativePath: "skills/beta"},
		},
	}

	report, err := NewSkills(repository)
	if err != nil {
		t.Fatalf("NewSkills() error = %v", err)
	}
	got := []string{}
	for _, skill := range report.Skills {
		got = append(got, skill.Name+"="+skill.RelativePath)
	}
	if strings.Join(got, ",") != "delta=skills/in-progress/delta,gamma=skills/gamma" {
		t.Fatalf("NewSkills() = %v", got)
	}
	if len(report.Ambiguous) != 0 {
		t.Fatalf("Ambiguous = %#v, want none", report.Ambiguous)
	}
}

func TestNewSkillsReturnsEmptyWhenEverythingIsRecorded(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, filepath.Join(root, "alpha"))
	report, err := NewSkills(state.RepositoryEntry{CheckoutPath: root, InstalledSkills: []state.InstalledSkillEntry{{Name: "alpha", RelativePath: "alpha"}}})
	if err != nil || report.Skills == nil || len(report.Skills) != 0 || len(report.Ambiguous) != 0 {
		t.Fatalf("NewSkills() = %#v, %v; want empty non-nil", report, err)
	}
}

func TestNewSkillsResolvesIdenticalCopiesAndReportsConflicting(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, filepath.Join(root, "plugin/skills/beta"))
	writeSkill(t, filepath.Join(root, ".agent/skills/beta"))
	writeSkillContent(t, filepath.Join(root, "packs/one/gamma"), "# One\n")
	writeSkillContent(t, filepath.Join(root, "packs/two/gamma"), "# Two\n")

	report, err := NewSkills(state.RepositoryEntry{CheckoutPath: root})
	if err != nil {
		t.Fatalf("NewSkills() error = %v", err)
	}
	if len(report.Skills) != 1 || report.Skills[0].Name != "beta" || report.Skills[0].RelativePath != "plugin/skills/beta" {
		t.Fatalf("Skills = %#v, want canonical beta", report.Skills)
	}
	if len(report.Ambiguous) != 1 || report.Ambiguous[0].Name != "gamma" {
		t.Fatalf("Ambiguous = %#v, want gamma", report.Ambiguous)
	}
	if len(report.Ambiguous[0].Paths) != 2 {
		t.Fatalf("Ambiguous paths = %#v, want both copies", report.Ambiguous[0].Paths)
	}
}

func TestNewSkillsIgnoresRecordedNamesInDuplicateGroups(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, filepath.Join(root, "plugin/skills/beta"))
	writeSkill(t, filepath.Join(root, ".agent/skills/beta"))
	repository := state.RepositoryEntry{
		CheckoutPath: root,
		InstalledSkills: []state.InstalledSkillEntry{
			{Name: "beta", RelativePath: ".agent/skills/beta"},
		},
	}

	report, err := NewSkills(repository)
	if err != nil {
		t.Fatalf("NewSkills() error = %v", err)
	}
	if len(report.Skills) != 0 || len(report.Ambiguous) != 0 {
		t.Fatalf("report = %#v, want nothing new", report)
	}
}

func TestNewSkillsReturnsDiscoveryErrors(t *testing.T) {
	root := t.TempDir()
	if _, err := NewSkills(state.RepositoryEntry{CheckoutPath: filepath.Join(root, "missing")}); err == nil {
		t.Fatal("NewSkills() error = nil for missing checkout")
	}
}
