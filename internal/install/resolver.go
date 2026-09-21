package install

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dees91/agent-skill-manager/internal/state"
)

// maxAmbiguousPathsShown bounds per-group path lists in ambiguous-skill
// errors. Fan-out repositories may hold dozens of copies; the error names the
// first copies and counts the rest instead of flooding the terminal.
const maxAmbiguousPathsShown = 10

// SkillSelection is one parsed --skill value: either a bare skill name or a
// name qualified with a source-relative path (name=relative/path).
type SkillSelection struct {
	Name string
	Path string
	Raw  string
}

// ParseSkillSelection parses one raw --skill value. The path side is
// normalized to slash-separated relative form and rejected when it escapes
// the source root.
func ParseSkillSelection(raw string) (SkillSelection, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return SkillSelection{}, fmt.Errorf("selected skill name is required")
	}
	name, path, hasPath := strings.Cut(trimmed, "=")
	if !hasPath {
		return SkillSelection{Name: trimmed, Raw: trimmed}, nil
	}
	selection := SkillSelection{Name: strings.TrimSpace(name), Raw: trimmed}
	if selection.Name == "" {
		return SkillSelection{}, fmt.Errorf("selected skill name is required")
	}
	normalized, err := normalizeSelectionPath(path)
	if err != nil {
		return SkillSelection{}, fmt.Errorf("skill %q: %w", selection.Name, err)
	}
	selection.Path = normalized
	return selection, nil
}

// normalizeSelectionPath validates a user-supplied source-relative skill path.
func normalizeSelectionPath(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("skill path after \"=\" is required")
	}
	if filepath.IsAbs(trimmed) {
		return "", fmt.Errorf("skill path %q must be relative to the source", trimmed)
	}
	cleaned := filepath.ToSlash(filepath.Clean(trimmed))
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("skill path %q escapes the source", trimmed)
	}
	return cleaned, nil
}

// AmbiguousGroup is one skill name whose copies differ, so no copy may be
// picked automatically. Paths are source-relative and safe to display.
type AmbiguousGroup struct {
	Name   string
	Paths  []string
	Hashes []string
}

// AmbiguousSkillsError reports skill names that need an explicit
// --skill <name>=<path> choice because their copies differ.
type AmbiguousSkillsError struct {
	Groups  []AmbiguousGroup
	Missing []string
}

func (e AmbiguousSkillsError) Error() string {
	parts := make([]string, 0, len(e.Groups)+1)
	for _, group := range e.Groups {
		shown := make([]string, 0, len(group.Paths))
		for i, path := range group.Paths {
			if i >= maxAmbiguousPathsShown {
				break
			}
			hash := ""
			if i < len(group.Hashes) && group.Hashes[i] != "" && group.Hashes[i] != "-" {
				hash = " [" + group.Hashes[i] + "]"
			}
			shown = append(shown, path+hash)
		}
		detail := strings.Join(shown, ", ")
		if len(group.Paths) > len(shown) {
			detail += fmt.Sprintf(", and %d more", len(group.Paths)-len(shown))
		}
		parts = append(parts, fmt.Sprintf("%q has %d copies with differing content (%s)", group.Name, len(group.Paths), detail))
	}
	message := "ambiguous skills; qualify with --skill <name>=<path>: " + strings.Join(parts, "; ")
	if len(e.Missing) > 0 {
		message += "; missing skills: " + strings.Join(e.Missing, ", ")
	}
	return message
}

func ambiguousGroupFromDuplicate(group DuplicateGroup) AmbiguousGroup {
	paths := make([]string, len(group.Candidates))
	for i, candidate := range group.Candidates {
		paths[i] = candidate.RelativePath
	}
	return AmbiguousGroup{Name: group.Name, Paths: paths, Hashes: append([]string(nil), group.Hashes...)}
}

// ResolvedDuplicate reports one identical-copies group that resolution
// collapsed to its canonical copy without asking.
type ResolvedDuplicate struct {
	Name         string
	Copies       int
	RelativePath string
}

// Resolution is the outcome of ResolveDiscovery.
type Resolution struct {
	Selected []DiscoveredSkill
	Missing  []string
	Resolved []ResolvedDuplicate
}

// ResolveDiscovery picks one skill per requested name. An empty requested
// list selects every discovered name. Explicit choices (qualified user input)
// win over recorded choices (manifest paths of the same source, which keep
// reinstalls stable when upstream gains another identical copy). Identical
// groups without a choice collapse to their canonical copy; conflicting
// groups without a matching choice fail with AmbiguousSkillsError.
func ResolveDiscovery(d Discovery, requested []string, explicit, recorded map[string]string) (Resolution, error) {
	unique := make(map[string]DiscoveredSkill, len(d.Skills))
	for _, skill := range d.Skills {
		if _, exists := unique[skill.Name]; exists {
			return Resolution{}, fmt.Errorf("duplicate discovery entry for skill %q", skill.Name)
		}
		unique[skill.Name] = skill
	}
	groups := make(map[string]DuplicateGroup, len(d.Groups))
	for _, group := range d.Groups {
		if _, exists := unique[group.Name]; exists {
			return Resolution{}, fmt.Errorf("duplicate discovery entry for skill %q", group.Name)
		}
		if _, exists := groups[group.Name]; exists {
			return Resolution{}, fmt.Errorf("duplicate discovery entry for skill %q", group.Name)
		}
		groups[group.Name] = group
	}

	names := d.Names()
	if len(requested) > 0 {
		wanted := map[string]bool{}
		for _, name := range requested {
			wanted[name] = true
		}
		filtered := make([]string, 0, len(names))
		for _, name := range names {
			if wanted[name] {
				filtered = append(filtered, name)
			}
		}
		names = filtered
	}

	result := Resolution{Selected: []DiscoveredSkill{}, Missing: []string{}, Resolved: []ResolvedDuplicate{}}
	var ambiguous []AmbiguousGroup
	for _, name := range names {
		if skill, ok := unique[name]; ok {
			if choice, hasChoice := resolveChoice(name, explicit, recorded); hasChoice && choice != skill.RelativePath {
				return Resolution{}, fmt.Errorf("skill %q has a single copy at %q, not %q", name, skill.RelativePath, choice)
			}
			result.Selected = append(result.Selected, skill)
			continue
		}
		group := groups[name]
		choice, hasChoice := resolveChoice(name, explicit, recorded)
		if !hasChoice {
			if group.Identical {
				result.Selected = append(result.Selected, group.Candidates[0])
				result.Resolved = append(result.Resolved, ResolvedDuplicate{Name: name, Copies: len(group.Candidates), RelativePath: group.Candidates[0].RelativePath})
				continue
			}
			ambiguous = append(ambiguous, ambiguousGroupFromDuplicate(group))
			continue
		}
		selected, ok := groupCandidateAt(group, choice)
		if !ok {
			return Resolution{}, mismatchedChoiceError(name, choice, group, explicitHas(explicit, name))
		}
		result.Selected = append(result.Selected, selected)
	}
	if len(requested) > 0 {
		known := map[string]bool{}
		for _, name := range names {
			known[name] = true
		}
		seenMissing := map[string]bool{}
		for _, name := range requested {
			if !known[name] && !seenMissing[name] {
				result.Missing = append(result.Missing, name)
				seenMissing[name] = true
			}
		}
		sort.Strings(result.Missing)
	}
	if len(ambiguous) > 0 {
		return Resolution{}, AmbiguousSkillsError{Groups: ambiguous, Missing: append([]string(nil), result.Missing...)}
	}
	return result, nil
}

func resolveChoice(name string, explicit, recorded map[string]string) (string, bool) {
	if choice := strings.TrimSpace(explicit[name]); choice != "" {
		return choice, true
	}
	if choice := strings.TrimSpace(recorded[name]); choice != "" {
		return choice, true
	}
	return "", false
}

func explicitHas(explicit map[string]string, name string) bool {
	return strings.TrimSpace(explicit[name]) != ""
}

func groupCandidateAt(group DuplicateGroup, relativePath string) (DiscoveredSkill, bool) {
	for _, candidate := range group.Candidates {
		if candidate.RelativePath == relativePath {
			return candidate, true
		}
	}
	return DiscoveredSkill{}, false
}

func mismatchedChoiceError(name, choice string, group DuplicateGroup, isExplicit bool) error {
	paths := make([]string, len(group.Candidates))
	for i, candidate := range group.Candidates {
		paths[i] = candidate.RelativePath
	}
	shown := strings.Join(paths, ", ")
	if len(paths) > maxAmbiguousPathsShown {
		shown = strings.Join(paths[:maxAmbiguousPathsShown], ", ") + fmt.Sprintf(", and %d more", len(paths)-maxAmbiguousPathsShown)
	}
	if isExplicit {
		return fmt.Errorf("skill %q has no copy at %q (copies: %s)", name, choice, shown)
	}
	return fmt.Errorf("skill %q was recorded at %q, which no longer holds the skill (copies: %s); uninstall the source and reinstall to adopt a new location", name, choice, shown)
}

// SplitSkillRequests parses raw --skill values into requested names and
// qualified path choices. A value containing "=" whose qualified form matches
// nothing but whose whole text matches a discovered name is treated as a bare
// name, so literal "=" names keep working.
func SplitSkillRequests(d Discovery, raws []string) ([]string, map[string]string, error) {
	names := []string{}
	explicit := map[string]string{}
	seen := map[string]bool{}
	for _, raw := range raws {
		selection, err := ParseSkillSelection(raw)
		if err != nil {
			return nil, nil, err
		}
		if selection.Path == "" {
			if !seen[selection.Name] {
				names = append(names, selection.Name)
				seen[selection.Name] = true
			}
			continue
		}
		if !discoveryHasPath(d, selection.Name, selection.Path) && d.HasName(selection.Raw) {
			if !seen[selection.Raw] {
				names = append(names, selection.Raw)
				seen[selection.Raw] = true
			}
			continue
		}
		if previous, ok := explicit[selection.Name]; ok && previous != selection.Path {
			return nil, nil, fmt.Errorf("conflicting paths for skill %q: %q and %q", selection.Name, previous, selection.Path)
		}
		explicit[selection.Name] = selection.Path
		if !seen[selection.Name] {
			names = append(names, selection.Name)
			seen[selection.Name] = true
		}
	}
	return names, explicit, nil
}

// CellsToRequests converts exact install cells into requested names and
// qualified path choices. Cells agree on one path per skill name.
func CellsToRequests(cells []InstallCell) ([]string, map[string]string, error) {
	names := []string{}
	explicit := map[string]string{}
	seen := map[string]bool{}
	for _, cell := range cells {
		name := strings.TrimSpace(cell.SkillName)
		if name == "" {
			return nil, nil, fmt.Errorf("selected skill name is required")
		}
		if strings.TrimSpace(cell.Path) != "" {
			normalized, err := normalizeSelectionPath(cell.Path)
			if err != nil {
				return nil, nil, fmt.Errorf("skill %q: %w", name, err)
			}
			if previous, ok := explicit[name]; ok && previous != normalized {
				return nil, nil, fmt.Errorf("conflicting paths for skill %q: %q and %q", name, previous, normalized)
			}
			explicit[name] = normalized
		}
		if !seen[name] {
			names = append(names, name)
			seen[name] = true
		}
	}
	return names, explicit, nil
}

func discoveryHasPath(d Discovery, name, relativePath string) bool {
	for _, skill := range d.Skills {
		if skill.Name == name && skill.RelativePath == relativePath {
			return true
		}
	}
	for _, group := range d.Groups {
		if group.Name != name {
			continue
		}
		if _, ok := groupCandidateAt(group, relativePath); ok {
			return true
		}
		return false
	}
	return false
}

// RecordedGitSkillPaths returns manifest skill paths for one recorded
// repository so resolution reuses them instead of switching copies.
func RecordedGitSkillPaths(manifest state.Manifest, host, repoPath string) map[string]string {
	repository, ok := manifest.GetRepository(host, repoPath)
	if !ok {
		return map[string]string{}
	}
	return recordedSkillPaths(repository.InstalledSkills)
}

// RecordedLocalSkillPaths returns manifest skill paths for one recorded local
// source so resolution reuses them instead of switching copies.
func RecordedLocalSkillPaths(manifest state.Manifest, canonicalPath string) map[string]string {
	source, ok := manifest.GetLocalSource(canonicalPath)
	if !ok {
		return map[string]string{}
	}
	return recordedSkillPaths(source.InstalledSkills)
}

func recordedSkillPaths(installed []state.InstalledSkillEntry) map[string]string {
	paths := map[string]string{}
	for _, skill := range installed {
		name := strings.TrimSpace(skill.Name)
		relative := normalizeRecordedRelativePath(skill.RelativePath)
		if name == "" || relative == "" {
			continue
		}
		paths[name] = relative
	}
	return paths
}
