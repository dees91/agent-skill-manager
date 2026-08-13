package install

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var ignoredDiscoveryDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	".venv":        true,
	"vendor":       true,
	"build":        true,
	"dist":         true,
}

// DiscoveredSkill is a valid installable skill found in a repository checkout.
type DiscoveredSkill struct {
	Name         string
	Path         string
	RelativePath string
}

// DiscoverSkills recursively finds installable skills inside a repository checkout.
func DiscoverSkills(checkoutPath string) ([]DiscoveredSkill, error) {
	root, err := filepath.Abs(filepath.Clean(checkoutPath))
	if err != nil {
		return nil, fmt.Errorf("resolve checkout path %s: %w", checkoutPath, err)
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("inspect checkout path %s: %w", checkoutPath, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("checkout path %s is not a directory", checkoutPath)
	}

	var discovered []DiscoveredSkill
	byName := map[string][]DiscoveredSkill{}
	err = filepath.WalkDir(root, func(currentPath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() {
			return nil
		}
		if currentPath != root && ignoredDiscoveryDirs[entry.Name()] {
			return filepath.SkipDir
		}

		skillFile := filepath.Join(currentPath, "SKILL.md")
		if info, err := os.Stat(skillFile); err == nil && !info.IsDir() {
			relativePath, err := filepath.Rel(root, currentPath)
			if err != nil {
				return fmt.Errorf("resolve relative skill path for %s: %w", currentPath, err)
			}
			skill := DiscoveredSkill{
				Name:         filepath.Base(currentPath),
				Path:         filepath.Clean(currentPath),
				RelativePath: filepath.ToSlash(relativePath),
			}
			discovered = append(discovered, skill)
			byName[skill.Name] = append(byName[skill.Name], skill)
		} else if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("inspect skill file %s: %w", skillFile, err)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("discover skills in %s: %w", checkoutPath, err)
	}

	if err := duplicateSkillNameError(byName); err != nil {
		return nil, err
	}

	sort.SliceStable(discovered, func(i, j int) bool {
		if discovered[i].Name != discovered[j].Name {
			return discovered[i].Name < discovered[j].Name
		}
		return discovered[i].RelativePath < discovered[j].RelativePath
	})
	return discovered, nil
}

func duplicateSkillNameError(byName map[string][]DiscoveredSkill) error {
	var messages []string
	for name, skills := range byName {
		if len(skills) < 2 {
			continue
		}
		paths := make([]string, len(skills))
		for i, skill := range skills {
			paths[i] = skill.RelativePath
		}
		sort.Strings(paths)
		messages = append(messages, fmt.Sprintf("%s (%s)", name, strings.Join(paths, ", ")))
	}
	if len(messages) == 0 {
		return nil
	}
	sort.Strings(messages)
	return fmt.Errorf("duplicate skill names discovered: %s", strings.Join(messages, "; "))
}
