package metadata

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// SkillMetadata contains optional display fields parsed from SKILL.md.
type SkillMetadata struct {
	Name                   string
	Description            string
	WhenToUse              string
	DisableModelInvocation bool
}

// ReadSkillMetadata reads easy-to-parse frontmatter from a SKILL.md file.
// Missing, unreadable, or malformed metadata falls back to the directory name.
func ReadSkillMetadata(skillFilePath, fallbackName string) SkillMetadata {
	meta := SkillMetadata{Name: fallbackName}

	data, err := os.ReadFile(skillFilePath)
	if err != nil {
		return meta
	}

	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return meta
	}

	closeIndex := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			closeIndex = i
			break
		}
	}
	if closeIndex == -1 {
		return meta
	}

	var frontmatter struct {
		Name                   string `yaml:"name"`
		Description            string `yaml:"description"`
		WhenToUse              string `yaml:"when_to_use"`
		DisableModelInvocation bool   `yaml:"disable-model-invocation"`
	}
	if err := yaml.Unmarshal([]byte(strings.Join(lines[1:closeIndex], "\n")), &frontmatter); err != nil {
		return meta
	}
	if value := strings.TrimSpace(frontmatter.Name); value != "" {
		meta.Name = value
	}
	meta.Description = strings.TrimSpace(frontmatter.Description)
	meta.WhenToUse = strings.TrimSpace(frontmatter.WhenToUse)
	meta.DisableModelInvocation = frontmatter.DisableModelInvocation

	return meta
}

// ReadSkillsLockNames reads skill identifiers from a Skills CLI lockfile.
// Missing or malformed lockfiles are treated as empty.
func ReadSkillsLockNames(lockFilePath string) map[string]struct{} {
	names := make(map[string]struct{})
	data, err := os.ReadFile(lockFilePath)
	if err != nil {
		return names
	}

	var document any
	if err := json.Unmarshal(data, &document); err != nil {
		return names
	}

	collectLockNames(document, names)
	return names
}

func collectLockNames(value any, names map[string]struct{}) {
	switch typed := value.(type) {
	case map[string]any:
		for _, key := range []string{"skills", "installedSkills", "installed", "entries"} {
			if container, ok := typed[key]; ok {
				collectSkillContainer(container, names)
			}
		}
		collectNamedFields(typed, names)
		for _, child := range typed {
			collectLockNames(child, names)
		}
	case []any:
		for _, child := range typed {
			collectLockNames(child, names)
		}
	}
}

func collectSkillContainer(value any, names map[string]struct{}) {
	switch typed := value.(type) {
	case map[string]any:
		for name, child := range typed {
			addName(names, name)
			collectNamedFieldsFromRecord(child, names)
		}
	case []any:
		for _, child := range typed {
			collectNamedFieldsFromRecord(child, names)
		}
	}
}

func collectNamedFieldsFromRecord(value any, names map[string]struct{}) {
	record, ok := value.(map[string]any)
	if !ok {
		return
	}
	collectNamedFields(record, names)
}

func collectNamedFields(record map[string]any, names map[string]struct{}) {
	for _, key := range []string{"name", "skill", "skillName", "id"} {
		if value, ok := record[key].(string); ok {
			addName(names, value)
		}
	}

	for _, key := range []string{"skillPath", "path"} {
		if value, ok := record[key].(string); ok {
			addName(names, skillNameFromPath(value))
		}
	}
}

func skillNameFromPath(path string) string {
	cleanPath := filepath.Clean(path)
	if filepath.Base(cleanPath) == "SKILL.md" {
		return filepath.Base(filepath.Dir(cleanPath))
	}
	return filepath.Base(cleanPath)
}

func addName(names map[string]struct{}, name string) {
	name = strings.TrimSpace(name)
	if name == "" || strings.ContainsAny(name, `/\`) || name == "." || name == ".." {
		return
	}
	names[name] = struct{}{}
}
