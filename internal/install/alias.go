package install

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/dees91/agent-skill-manager/internal/model"
	"github.com/dees91/agent-skill-manager/internal/paths"
	"github.com/dees91/agent-skill-manager/internal/state"
)

// maxInstalledNameLength is the Agent Skills name length limit.
const maxInstalledNameLength = 64

// SourceRef holds the sanitized name segments used to suggest an installed
// name for a skill whose plain name is owned by another source.
type SourceRef struct {
	Owner string
	Repo  string
}

// GitSourceRef derives suggestion segments from a repository identity: the
// first repository path segment (the owner) and the last (the repository).
func GitSourceRef(identity RepoIdentity) SourceRef {
	segments := strings.Split(strings.Trim(identity.RepoPath, "/"), "/")
	ref := SourceRef{Owner: SanitizeNameSegment(segments[0])}
	if len(segments) > 1 {
		ref.Repo = SanitizeNameSegment(segments[len(segments)-1])
	}
	return ref
}

// LocalSourceRef derives suggestion segments from a local source root: its
// basename, with the parent directory basename as the fallback segment.
func LocalSourceRef(canonicalPath string) SourceRef {
	cleaned := filepath.Clean(canonicalPath)
	return SourceRef{
		Owner: SanitizeNameSegment(filepath.Base(cleaned)),
		Repo:  SanitizeNameSegment(filepath.Base(filepath.Dir(cleaned))),
	}
}

// SanitizeNameSegment lowercases value and maps it onto the Agent Skills name
// alphabet: runs of other characters become one hyphen, and leading or
// trailing hyphens are dropped.
func SanitizeNameSegment(value string) string {
	var builder strings.Builder
	pendingHyphen := false
	for _, r := range strings.ToLower(value) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			if pendingHyphen && builder.Len() > 0 {
				builder.WriteByte('-')
			}
			pendingHyphen = false
			builder.WriteRune(r)
			continue
		}
		pendingHyphen = true
	}
	return builder.String()
}

// SuggestInstalledName proposes a free installed name for name: first
// <name>-<owner>, then <name>-<owner>-<repo>, then a numeric suffix. It
// returns "" when name itself cannot form a valid installed name.
func SuggestInstalledName(name string, source SourceRef, taken func(string) bool) string {
	base := SanitizeNameSegment(name)
	if base == "" {
		return ""
	}
	candidates := []string{}
	if source.Owner != "" {
		candidates = append(candidates, joinNameSegments(base, source.Owner))
		if source.Repo != "" && source.Repo != source.Owner {
			candidates = append(candidates, joinNameSegments(base, source.Owner, source.Repo))
		}
	}
	stem := base
	if len(candidates) > 0 {
		stem = candidates[0]
	}
	for i := 2; i < 100; i++ {
		candidates = append(candidates, joinNameSegments(stem, strconv.Itoa(i)))
	}
	for _, candidate := range candidates {
		if candidate == name || !state.ValidInstalledName(candidate) {
			continue
		}
		if taken == nil || !taken(candidate) {
			return candidate
		}
	}
	return ""
}

// joinNameSegments joins sanitized segments with hyphens and keeps the result
// within the installed-name length limit by shortening the leading segment.
func joinNameSegments(first string, rest ...string) string {
	suffix := ""
	for _, segment := range rest {
		suffix += "-" + segment
	}
	if len(first)+len(suffix) > maxInstalledNameLength {
		keep := maxInstalledNameLength - len(suffix)
		if keep < 1 {
			return strings.Trim((first + suffix)[:maxInstalledNameLength], "-")
		}
		first = strings.TrimRight(first[:keep], "-")
	}
	return first + suffix
}

// RecordedGitInstalledNames returns the installed name of every skill the
// repository records, keyed by source name.
func RecordedGitInstalledNames(manifest state.Manifest, host, repoPath string) map[string]string {
	repository, ok := manifest.GetRepository(host, repoPath)
	if !ok {
		return map[string]string{}
	}
	return recordedInstalledNames(repository.InstalledSkills)
}

// RecordedLocalInstalledNames returns the installed name of every skill the
// local source records, keyed by source name.
func RecordedLocalInstalledNames(manifest state.Manifest, canonicalPath string) map[string]string {
	source, ok := manifest.GetLocalSource(canonicalPath)
	if !ok {
		return map[string]string{}
	}
	return recordedInstalledNames(source.InstalledSkills)
}

func recordedInstalledNames(installed []state.InstalledSkillEntry) map[string]string {
	names := map[string]string{}
	for _, skill := range installed {
		name := strings.TrimSpace(skill.Name)
		if name == "" {
			continue
		}
		names[name] = skill.InstalledName()
	}
	return names
}

// resolveInstalledNames decides the installed name of every selected skill.
// A recorded name wins; an explicit name that differs from the recorded one
// is drift. Explicit names must be valid, unique, and must not reuse a name
// discovered in the same source or recorded for another of its skills, even
// one outside the selection. The result maps source name to InstalledAs and
// omits skills installed under their plain name.
func resolveInstalledNames(discovered Discovery, selected []string, explicit, recorded map[string]string) (map[string]string, error) {
	selectedSet := make(map[string]bool, len(selected))
	for _, name := range selected {
		selectedSet[name] = true
	}
	keys := make([]string, 0, len(explicit))
	for name := range explicit {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	for _, name := range keys {
		if !selectedSet[name] {
			return nil, fmt.Errorf("--as names skill %q, which is not selected for install", name)
		}
	}

	result := map[string]string{}
	usedBy := map[string]string{}
	recordedNames := make([]string, 0, len(recorded))
	for name := range recorded {
		recordedNames = append(recordedNames, name)
	}
	sort.Strings(recordedNames)
	for _, name := range recordedNames {
		if !selectedSet[name] {
			usedBy[recorded[name]] = name
		}
	}
	names := append([]string(nil), selected...)
	sort.Strings(names)
	for _, name := range names {
		wanted, hasExplicit := explicit[name]
		wanted = strings.TrimSpace(wanted)
		recordedName, hasRecord := recorded[name]
		installed := name
		switch {
		case hasRecord && hasExplicit && wanted != recordedName:
			return nil, fmt.Errorf("skill %q is installed as %q; uninstall the source and reinstall to use %q", name, recordedName, wanted)
		case hasRecord:
			installed = recordedName
		case hasExplicit:
			installed = wanted
		}
		if installed != name {
			if !state.ValidInstalledName(installed) {
				return nil, fmt.Errorf("invalid install name %q for skill %q: use 1-64 lowercase letters, digits, and single hyphens", installed, name)
			}
			if discovered.HasName(installed) {
				return nil, fmt.Errorf("install name %q for skill %q is another skill in the same source", installed, name)
			}
			result[name] = installed
		}
		if previous, ok := usedBy[installed]; ok {
			first, second := previous, name
			if second < first {
				first, second = second, first
			}
			return nil, fmt.Errorf("skills %q and %q both use install name %q", first, second, installed)
		}
		usedBy[installed] = name
	}
	return result, nil
}

// installedNameTaken reports whether an installed name is unavailable for a
// suggestion: owned by any recorded source, discovered in the current source,
// requested as another alias, or already present in a tool directory.
func installedNameTaken(p paths.Paths, manifest state.Manifest, discovered Discovery, aliases map[string]string, tools []model.Tool) func(string) bool {
	owned := map[string]bool{}
	for _, repository := range manifest.Repositories {
		for _, skill := range repository.InstalledSkills {
			owned[skill.InstalledName()] = true
		}
	}
	for _, source := range manifest.LocalSources {
		for _, skill := range source.InstalledSkills {
			owned[skill.InstalledName()] = true
		}
	}
	for _, entry := range manifest.Disabled {
		owned[entry.SkillName] = true
	}
	for _, alias := range aliases {
		owned[alias] = true
	}
	store := state.New(p)
	return func(name string) bool {
		if owned[name] || discovered.HasName(name) {
			return true
		}
		for _, tool := range tools {
			if dir, ok := p.UserSkillsDirFor(tool); ok && pathExists(filepath.Join(dir, name)) {
				return true
			}
			if disabled, err := store.DisabledPath(tool, name); err == nil && pathExists(disabled) {
				return true
			}
		}
		return false
	}
}

func pathExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil || !errors.Is(err, os.ErrNotExist)
}

// NewSkillInstalledNameSuggestion returns a free install name for a skill that
// is new in a recorded repository when another source owns its plain name for
// one of tools, or "" when the plain name is free. sourceNames lists every
// skill name in the repository so a suggestion never reuses one of them.
func NewSkillInstalledNameSuggestion(p paths.Paths, manifest state.Manifest, repository state.RepositoryEntry, name string, tools []model.Tool, sourceNames []string) string {
	identity := RepoIdentity{Host: repository.Host, RepoPath: repository.RepoPath}
	owned := false
	for _, tool := range tools {
		if gitInstallCellOwner(manifest, tool, name, identity).found() {
			owned = true
			break
		}
	}
	if !owned {
		return ""
	}
	skills := make([]DiscoveredSkill, 0, len(sourceNames))
	seen := map[string]bool{}
	for _, sourceName := range sourceNames {
		if !seen[sourceName] {
			seen[sourceName] = true
			skills = append(skills, DiscoveredSkill{Name: sourceName})
		}
	}
	taken := installedNameTaken(p, manifest, Discovery{Skills: skills}, nil, tools)
	return SuggestInstalledName(name, GitSourceRef(identity), taken)
}

// GitNameOwnedElsewhere reports whether another recorded source owns name
// as an installed name for any tool, excluding the given repository.
func GitNameOwnedElsewhere(manifest state.Manifest, identity RepoIdentity, name string) bool {
	for _, tool := range model.Tools() {
		if gitInstallCellOwner(manifest, tool, name, identity).found() {
			return true
		}
	}
	return false
}

// LocalNameOwnedElsewhere reports whether another recorded source owns name
// as an installed name for any tool, excluding the given local source.
func LocalNameOwnedElsewhere(manifest state.Manifest, canonicalPath, name string) bool {
	for _, tool := range model.Tools() {
		if managedCellOwner(manifest, tool, name, canonicalPath).found() {
			return true
		}
	}
	return false
}

// validateUniqueInstalledNames rejects a source record in which two skills
// resolve to one install name; apply checks it on the fresh manifest before
// saving so a stale plan can never write a record the audits refuse.
func validateUniqueInstalledNames(skills []state.InstalledSkillEntry) error {
	usedBy := map[string]string{}
	for _, skill := range skills {
		installed := skill.InstalledName()
		if previous, ok := usedBy[installed]; ok && previous != skill.Name {
			return fmt.Errorf("skills %s and %s share install name %s", previous, skill.Name, installed)
		}
		usedBy[installed] = skill.Name
	}
	return nil
}
