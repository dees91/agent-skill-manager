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

// fixturePathSegments deprioritize test and example copies when several
// directories share one skill name. They only affect the canonical ranking;
// they never hide a skill.
var fixturePathSegments = map[string]bool{
	"test":     true,
	"tests":    true,
	"testdata": true,
	"fixtures": true,
	"fixture":  true,
	"examples": true,
	"example":  true,
}

// DiscoveredSkill is a valid installable skill found in a repository checkout.
type DiscoveredSkill struct {
	Name         string
	Path         string
	RelativePath string
}

// DuplicateGroup is one skill name discovered at two or more paths inside a
// single source. Candidates are in canonical rank order with the preferred
// copy first. Identical is true only when every copy shares the same content
// fingerprint, in which case the preferred copy may be used without asking.
type DuplicateGroup struct {
	Name       string
	Candidates []DiscoveredSkill
	Identical  bool
	Hashes     []string
}

// Discovery is the grouped result of recursive skill discovery. Unique holds
// skills found at exactly one path; Groups holds skill names found at two or
// more paths. Callers resolve groups through ResolveDiscovery instead of
// failing the whole discovery.
type Discovery struct {
	Skills []DiscoveredSkill
	Groups []DuplicateGroup
}

// Empty reports whether no installable skill was discovered.
func (d Discovery) Empty() bool {
	return len(d.Skills) == 0 && len(d.Groups) == 0
}

// NameCount returns the number of distinct discovered skill names.
func (d Discovery) NameCount() int {
	return len(d.Skills) + len(d.Groups)
}

// All returns every discovered skill copy, including each duplicate option.
func (d Discovery) All() []DiscoveredSkill {
	all := make([]DiscoveredSkill, 0, len(d.Skills))
	all = append(all, d.Skills...)
	for _, group := range d.Groups {
		all = append(all, group.Candidates...)
	}
	return all
}

// Names returns every discovered skill name in sorted order.
func (d Discovery) Names() []string {
	names := make([]string, 0, len(d.Skills)+len(d.Groups))
	for _, skill := range d.Skills {
		names = append(names, skill.Name)
	}
	for _, group := range d.Groups {
		names = append(names, group.Name)
	}
	sort.Strings(names)
	return names
}

// HasName reports whether a skill name was discovered.
func (d Discovery) HasName(name string) bool {
	for _, skill := range d.Skills {
		if skill.Name == name {
			return true
		}
	}
	for _, group := range d.Groups {
		if group.Name == name {
			return true
		}
	}
	return false
}

// DiscoverSkills recursively finds installable skills inside a repository
// checkout. Skills sharing one basename are grouped instead of failing the
// discovery; see ResolveDiscovery for how callers pick one copy.
func DiscoverSkills(checkoutPath string) (Discovery, error) {
	root, err := filepath.Abs(filepath.Clean(checkoutPath))
	if err != nil {
		return Discovery{}, fmt.Errorf("resolve checkout path %s: %w", checkoutPath, err)
	}
	info, err := os.Stat(root)
	if err != nil {
		return Discovery{}, fmt.Errorf("inspect checkout path %s: %w", checkoutPath, err)
	}
	if !info.IsDir() {
		return Discovery{}, fmt.Errorf("checkout path %s is not a directory", checkoutPath)
	}

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
			byName[skill.Name] = append(byName[skill.Name], skill)
		} else if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("inspect skill file %s: %w", skillFile, err)
		}
		return nil
	})
	if err != nil {
		return Discovery{}, fmt.Errorf("discover skills in %s: %w", checkoutPath, err)
	}

	return groupDiscoveredSkills(byName)
}

// groupDiscoveredSkills splits collected skills into unique names and ranked,
// fingerprinted duplicate groups.
func groupDiscoveredSkills(byName map[string][]DiscoveredSkill) (Discovery, error) {
	result := Discovery{Skills: []DiscoveredSkill{}, Groups: []DuplicateGroup{}}
	for name, skills := range byName {
		if len(skills) < 2 {
			result.Skills = append(result.Skills, skills[0])
			continue
		}
		group, err := rankDuplicateGroup(name, skills)
		if err != nil {
			return Discovery{}, err
		}
		result.Groups = append(result.Groups, group)
	}
	sort.SliceStable(result.Skills, func(i, j int) bool {
		return result.Skills[i].Name < result.Skills[j].Name
	})
	sort.SliceStable(result.Groups, func(i, j int) bool {
		return result.Groups[i].Name < result.Groups[j].Name
	})
	return result, nil
}

// rankDuplicateGroup orders copies of one skill name canonically and compares
// their content. The ranking prefers real distribution paths over test or
// example copies, visible directories over hidden ones, shallower paths over
// deeper ones, and breaks remaining ties lexicographically so the choice is
// stable across runs.
func rankDuplicateGroup(name string, skills []DiscoveredSkill) (DuplicateGroup, error) {
	ordered := append([]DiscoveredSkill(nil), skills...)
	sort.SliceStable(ordered, func(i, j int) bool {
		left, right := duplicateRank(ordered[i]), duplicateRank(ordered[j])
		if left.fixture != right.fixture {
			return left.fixture < right.fixture
		}
		if left.hidden != right.hidden {
			return left.hidden < right.hidden
		}
		if left.depth != right.depth {
			return left.depth < right.depth
		}
		return ordered[i].RelativePath < ordered[j].RelativePath
	})

	hashes := make([]string, len(ordered))
	full := make([]string, len(ordered))
	for i, skill := range ordered {
		fingerprint, err := fingerprintSkillDir(skill.Path)
		if err != nil {
			return DuplicateGroup{}, fmt.Errorf("fingerprint skill %s at %s: %w", name, skill.RelativePath, err)
		}
		full[i] = fingerprint.hash
		hashes[i] = fingerprint.short()
	}
	identical := true
	for _, hash := range full {
		if hash == "" || hash != full[0] {
			identical = false
			break
		}
	}
	return DuplicateGroup{Name: name, Candidates: ordered, Identical: identical, Hashes: hashes}, nil
}

type duplicateRankKey struct {
	fixture int
	hidden  int
	depth   int
}

func duplicateRank(skill DiscoveredSkill) duplicateRankKey {
	segments := strings.Split(skill.RelativePath, "/")
	key := duplicateRankKey{depth: len(segments)}
	for _, segment := range segments {
		if segment == "." || segment == "" {
			continue
		}
		if fixturePathSegments[strings.ToLower(segment)] {
			key.fixture = 1
		}
		if strings.HasPrefix(segment, ".") {
			key.hidden++
		}
	}
	return key
}
