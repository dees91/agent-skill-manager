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

	newSkills, err := NewSkills(repository)
	if err != nil {
		t.Fatalf("NewSkills() error = %v", err)
	}
	got := []string{}
	for _, skill := range newSkills {
		got = append(got, skill.Name+"="+skill.RelativePath)
	}
	if strings.Join(got, ",") != "delta=skills/in-progress/delta,gamma=skills/gamma" {
		t.Fatalf("NewSkills() = %v", got)
	}
}

func TestNewSkillsReturnsEmptyWhenEverythingIsRecorded(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, filepath.Join(root, "alpha"))
	newSkills, err := NewSkills(state.RepositoryEntry{CheckoutPath: root, InstalledSkills: []state.InstalledSkillEntry{{Name: "alpha", RelativePath: "alpha"}}})
	if err != nil || newSkills == nil || len(newSkills) != 0 {
		t.Fatalf("NewSkills() = %#v, %v; want empty non-nil", newSkills, err)
	}
}

func TestNewSkillsReturnsDiscoveryErrors(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, filepath.Join(root, "one/alpha"))
	writeSkill(t, filepath.Join(root, "two/alpha"))
	if _, err := NewSkills(state.RepositoryEntry{CheckoutPath: root}); err == nil || !strings.Contains(err.Error(), "duplicate skill names") {
		t.Fatalf("NewSkills() error = %v, want duplicate conflict", err)
	}
	if _, err := NewSkills(state.RepositoryEntry{CheckoutPath: filepath.Join(root, "missing")}); err == nil {
		t.Fatal("NewSkills() error = nil for missing checkout")
	}
}
