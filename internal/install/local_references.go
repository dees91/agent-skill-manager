package install

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dees91/agent-skill-manager/internal/model"
	"github.com/dees91/agent-skill-manager/internal/paths"
	"github.com/dees91/agent-skill-manager/internal/state"
)

// LocalReferenceAudit is the validated set of symlinks owned by a local source.
type LocalReferenceAudit struct {
	Source     state.LocalSourceEntry
	References []RepositoryReference
}

// AuditLocalSourceReferences verifies local-source ownership. When
// requireSource is false, broken links may still be safely audited for cleanup.
func AuditLocalSourceReferences(p paths.Paths, manifest state.Manifest, source state.LocalSourceEntry, requireSource bool) (LocalReferenceAudit, error) {
	return auditLocalSourceReferences(p, manifest, source, requireSource, nil, nil)
}

func auditLocalSourceReferences(p paths.Paths, manifest state.Manifest, source state.LocalSourceEntry, requireSource bool, allowedLinkPaths, allowedDisabledCells map[string]bool) (LocalReferenceAudit, error) {
	canonicalPath, err := validateRecordedLocalSource(source)
	if err != nil {
		return LocalReferenceAudit{}, err
	}
	source.CanonicalPath = canonicalPath
	audit := LocalReferenceAudit{Source: source, References: []RepositoryReference{}}
	conflicts := []ReferenceConflict{}
	expectedLinkPaths := cloneStringBoolMap(allowedLinkPaths)
	expectedDisabledCells := cloneStringBoolMap(allowedDisabledCells)
	seenCells := map[string]bool{}
	pathsByName := map[string]string{}
	validatedSkills := map[string]bool{}

	for _, installed := range source.InstalledSkills {
		skillPath, relativePath, normalizeErr := normalizeRecordedSkill(canonicalPath, installed)
		if normalizeErr != nil {
			conflicts = append(conflicts, ReferenceConflict{SkillName: installed.Name, Path: canonicalPath, Reason: normalizeErr.Error()})
			continue
		}
		if previous, ok := pathsByName[installed.Name]; ok && previous != relativePath {
			conflicts = append(conflicts, ReferenceConflict{SkillName: installed.Name, Path: skillPath, Reason: fmt.Sprintf("duplicate skill name uses relative paths %s and %s", previous, relativePath)})
			continue
		}
		pathsByName[installed.Name] = relativePath
		if requireSource && !validatedSkills[relativePath] {
			validatedSkills[relativePath] = true
			if err := validateSkillForApply(canonicalPath, DiscoveredSkill{Name: installed.Name, Path: skillPath, RelativePath: relativePath}); err != nil {
				conflicts = append(conflicts, ReferenceConflict{SkillName: installed.Name, Path: skillPath, Reason: err.Error()})
			}
		}

		for _, tool := range installed.Tools {
			activeDir, ok := p.UserSkillsDirFor(tool)
			if !ok {
				conflicts = append(conflicts, ReferenceConflict{Tool: tool, SkillName: installed.Name, Path: skillPath, Reason: "unsupported tool in local source manifest"})
				continue
			}
			cellKey := repositoryCellKey(tool, installed.Name)
			if seenCells[cellKey] {
				conflicts = append(conflicts, ReferenceConflict{Tool: tool, SkillName: installed.Name, Path: skillPath, Reason: "duplicate local source skill/tool cell"})
				continue
			}
			seenCells[cellKey] = true
			activePath := filepath.Join(activeDir, installed.Name)
			if disabled, off := manifest.Get(tool, installed.Name); off {
				reference, referenceConflicts := auditDisabledReference(p, disabled, tool, installed.Name, relativePath, skillPath, activePath)
				conflicts = append(conflicts, referenceConflicts...)
				audit.References = append(audit.References, reference)
				expectedLinkPaths[filepath.Clean(reference.LinkPath)] = true
				expectedDisabledCells[cellKey] = true
				continue
			}
			reference, referenceConflicts := auditActiveReference(tool, installed.Name, relativePath, skillPath, activePath)
			conflicts = append(conflicts, referenceConflicts...)
			audit.References = append(audit.References, reference)
			expectedLinkPaths[filepath.Clean(reference.LinkPath)] = true
		}
	}

	for _, disabled := range manifest.Disabled {
		if disabled.EntryType != model.EntryTypeSymlink {
			continue
		}
		resolved := resolveLinkTarget(disabled.OriginalPath, disabled.SymlinkTarget)
		if pathInside(canonicalPath, resolved) && !expectedDisabledCells[repositoryCellKey(disabled.Tool, disabled.SkillName)] {
			conflicts = append(conflicts, ReferenceConflict{Tool: disabled.Tool, SkillName: disabled.SkillName, Path: disabled.DisabledPath, Reason: "disabled manifest entry points into local source but is not recorded by it"})
		}
	}
	extraConflicts, err := findExtraManagedReferences(p, canonicalPath, expectedLinkPaths)
	if err != nil {
		return LocalReferenceAudit{}, err
	}
	conflicts = append(conflicts, extraConflicts...)
	sortReferenceConflicts(conflicts)
	sort.SliceStable(audit.References, func(i, j int) bool {
		if audit.References[i].Tool != audit.References[j].Tool {
			return audit.References[i].Tool.String() < audit.References[j].Tool.String()
		}
		return audit.References[i].SkillName < audit.References[j].SkillName
	})
	if len(conflicts) > 0 {
		return LocalReferenceAudit{}, ReferenceAuditError{Conflicts: conflicts}
	}
	return audit, nil
}

func cloneStringBoolMap(input map[string]bool) map[string]bool {
	cloned := map[string]bool{}
	for key, value := range input {
		cloned[key] = value
	}
	return cloned
}

func validateRecordedLocalSource(source state.LocalSourceEntry) (string, error) {
	if strings.TrimSpace(source.CanonicalPath) == "" || !filepath.IsAbs(source.CanonicalPath) {
		return "", fmt.Errorf("recorded local source canonical path %q is not absolute", source.CanonicalPath)
	}
	canonicalPath, err := filepath.Abs(filepath.Clean(source.CanonicalPath))
	if err != nil {
		return "", fmt.Errorf("resolve recorded local source path: %w", err)
	}
	if canonicalPath != source.CanonicalPath {
		return "", fmt.Errorf("recorded local source canonical path %q is not normalized", source.CanonicalPath)
	}
	if strings.TrimSpace(source.OriginalPath) == "" || !filepath.IsAbs(source.OriginalPath) {
		return "", fmt.Errorf("recorded local source original path %q is not absolute", source.OriginalPath)
	}
	if source.Group == "" {
		return "", fmt.Errorf("recorded local source %s has no group", canonicalPath)
	}
	return canonicalPath, nil
}
