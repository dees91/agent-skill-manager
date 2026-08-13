package metadata

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadSkillMetadataParsesQuotedFrontmatter(t *testing.T) {
	path := writeSkillFile(t, `---
name: "Display Name"
description: 'Builds things: quickly'
---
# Body
`)

	got := ReadSkillMetadata(path, "fallback")

	if got.Name != "Display Name" {
		t.Fatalf("Name = %q, want Display Name", got.Name)
	}
	if got.Description != "Builds things: quickly" {
		t.Fatalf("Description = %q, want parsed description", got.Description)
	}
}

func TestReadSkillMetadataFallsBackWithoutFrontmatter(t *testing.T) {
	path := writeSkillFile(t, "# Body\n")

	got := ReadSkillMetadata(path, "dir-name")

	if got.Name != "dir-name" {
		t.Fatalf("Name = %q, want fallback", got.Name)
	}
	if got.Description != "" {
		t.Fatalf("Description = %q, want empty", got.Description)
	}
}

func TestReadSkillMetadataFallsBackForMalformedFrontmatter(t *testing.T) {
	path := writeSkillFile(t, `---
name: Broken
description: Should not be used
`)

	got := ReadSkillMetadata(path, "dir-name")

	if got.Name != "dir-name" {
		t.Fatalf("Name = %q, want fallback", got.Name)
	}
	if got.Description != "" {
		t.Fatalf("Description = %q, want empty", got.Description)
	}
}

func TestReadSkillMetadataFallsBackNameWhenNameAbsent(t *testing.T) {
	path := writeSkillFile(t, `---
description: Useful helper
---
`)

	got := ReadSkillMetadata(path, "dir-name")

	if got.Name != "dir-name" {
		t.Fatalf("Name = %q, want fallback", got.Name)
	}
	if got.Description != "Useful helper" {
		t.Fatalf("Description = %q, want parsed description", got.Description)
	}
}

func TestReadSkillMetadataParsesContextFieldsAndMultilineDescription(t *testing.T) {
	path := writeSkillFile(t, `---
name: context-helper
description: >-
  Explains context budgets
  for installed skills.
when_to_use: Use for token accounting
disable-model-invocation: true
---
`)

	got := ReadSkillMetadata(path, "fallback")

	if got.Description != "Explains context budgets for installed skills." {
		t.Fatalf("Description = %q", got.Description)
	}
	if got.WhenToUse != "Use for token accounting" || !got.DisableModelInvocation {
		t.Fatalf("metadata = %#v", got)
	}
}

func TestReadSkillsLockNamesReadsSkillsMapAndFields(t *testing.T) {
	lockPath := writeLockFile(t, `{
  "skills": {
    "find-skills": {
      "skillPath": "skills/find-skills/SKILL.md"
    }
  },
  "records": [
    {"name": "named-skill"},
    {"skillPath": "nested/path/path-skill/SKILL.md"}
  ],
  "lastSelectedAgents": ["codex"]
}`)

	got := ReadSkillsLockNames(lockPath)

	for _, name := range []string{"find-skills", "named-skill", "path-skill"} {
		if _, ok := got[name]; !ok {
			t.Fatalf("ReadSkillsLockNames() missing %q in %#v", name, got)
		}
	}
	if _, ok := got["codex"]; ok {
		t.Fatalf("ReadSkillsLockNames() included unrelated array string %q", "codex")
	}
}

func TestReadSkillsLockNamesMalformedIsEmpty(t *testing.T) {
	lockPath := writeLockFile(t, `{not-json`)

	got := ReadSkillsLockNames(lockPath)

	if len(got) != 0 {
		t.Fatalf("ReadSkillsLockNames() = %#v, want empty", got)
	}
}

func writeSkillFile(t *testing.T, contents string) string {
	t.Helper()
	return writeTempFile(t, "SKILL.md", contents)
}

func writeLockFile(t *testing.T, contents string) string {
	t.Helper()
	return writeTempFile(t, ".skill-lock.json", contents)
}

func writeTempFile(t *testing.T, name, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}
