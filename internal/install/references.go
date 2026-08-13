package install

import (
	"errors"
	"fmt"
	"os"
	pathpkg "path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dees91/agent-skill-manager/internal/model"
	"github.com/dees91/agent-skill-manager/internal/paths"
	"github.com/dees91/agent-skill-manager/internal/state"
)

// RepositoryReference is one active or disabled managed symlink owned by an install source.
type RepositoryReference struct {
	Tool           model.Tool
	SkillName      string
	RelativePath   string
	SkillPath      string
	LinkPath       string
	SymlinkTarget  string
	State          model.SkillState
	DisabledRecord *state.DisabledEntry
}

// ReferenceConflict describes managed reference state that is unsafe to mutate.
type ReferenceConflict struct {
	Tool      model.Tool
	SkillName string
	Path      string
	Reason    string
}

// ReferenceAudit is the validated set of repository-owned symlinks.
type ReferenceAudit struct {
	Identity   RepoIdentity
	Repository state.RepositoryEntry
	References []RepositoryReference
}

// ReferenceAuditError reports every deterministic conflict found during an audit.
type ReferenceAuditError struct {
	Conflicts []ReferenceConflict
}

func (e ReferenceAuditError) Error() string {
	parts := make([]string, len(e.Conflicts))
	for i, conflict := range e.Conflicts {
		cell := conflict.SkillName
		if conflict.Tool != "" {
			cell = conflict.Tool.String() + "/" + conflict.SkillName
		}
		if cell == "" {
			cell = "repository"
		}
		parts[i] = fmt.Sprintf("%s at %s: %s", cell, conflict.Path, conflict.Reason)
	}
	return "managed reference conflicts: " + strings.Join(parts, "; ")
}

// AuditRepositoryReferences verifies all manifest-owned links and rejects extra
// managed-directory symlinks that point into the repository checkout.
func AuditRepositoryReferences(p paths.Paths, manifest state.Manifest, repository state.RepositoryEntry) (ReferenceAudit, error) {
	identity, checkoutPath, err := validateRecordedRepository(p, repository)
	if err != nil {
		return ReferenceAudit{}, err
	}
	repository.CheckoutPath = checkoutPath
	audit := ReferenceAudit{Identity: identity, Repository: repository, References: []RepositoryReference{}}
	conflicts := []ReferenceConflict{}
	expectedLinkPaths := map[string]bool{}
	expectedDisabledCells := map[string]bool{}
	seenCells := map[string]bool{}
	pathsByName := map[string]string{}
	validatedSkills := map[string]bool{}

	for _, installed := range repository.InstalledSkills {
		skillPath, relativePath, normalizeErr := normalizeRecordedSkill(checkoutPath, installed)
		if normalizeErr != nil {
			conflicts = append(conflicts, ReferenceConflict{SkillName: installed.Name, Path: checkoutPath, Reason: normalizeErr.Error()})
			continue
		}
		if previous, ok := pathsByName[installed.Name]; ok && previous != relativePath {
			conflicts = append(conflicts, ReferenceConflict{SkillName: installed.Name, Path: skillPath, Reason: fmt.Sprintf("duplicate skill name uses relative paths %s and %s", previous, relativePath)})
			continue
		}
		pathsByName[installed.Name] = relativePath
		if !validatedSkills[relativePath] {
			validatedSkills[relativePath] = true
			if err := validateSkillForApply(checkoutPath, DiscoveredSkill{Name: installed.Name, Path: skillPath, RelativePath: relativePath}); err != nil {
				conflicts = append(conflicts, ReferenceConflict{SkillName: installed.Name, Path: skillPath, Reason: err.Error()})
			}
		}

		for _, tool := range installed.Tools {
			activeDir, ok := p.UserSkillsDirFor(tool)
			if !ok {
				conflicts = append(conflicts, ReferenceConflict{Tool: tool, SkillName: installed.Name, Path: skillPath, Reason: "unsupported tool in repository manifest"})
				continue
			}
			cellKey := repositoryCellKey(tool, installed.Name)
			if seenCells[cellKey] {
				conflicts = append(conflicts, ReferenceConflict{Tool: tool, SkillName: installed.Name, Path: skillPath, Reason: "duplicate repository skill/tool cell"})
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
		if pathInside(checkoutPath, resolved) && !expectedDisabledCells[repositoryCellKey(disabled.Tool, disabled.SkillName)] {
			conflicts = append(conflicts, ReferenceConflict{Tool: disabled.Tool, SkillName: disabled.SkillName, Path: disabled.DisabledPath, Reason: "disabled manifest entry points into checkout but is not recorded by repository"})
		}
	}

	extraConflicts, err := findExtraManagedReferences(p, checkoutPath, expectedLinkPaths)
	if err != nil {
		return ReferenceAudit{}, err
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
		return ReferenceAudit{}, ReferenceAuditError{Conflicts: conflicts}
	}
	return audit, nil
}

func validateRecordedRepository(p paths.Paths, repository state.RepositoryEntry) (RepoIdentity, string, error) {
	input := strings.TrimSpace(repository.OriginalURL)
	if input == "" {
		input = strings.TrimSpace(repository.CanonicalURL)
	}
	if input == "" && repository.Host != "" && repository.RepoPath != "" {
		input = "https://" + repository.Host + "/" + repository.RepoPath
	}
	identity, err := NormalizeGitURL(input)
	if err != nil {
		return RepoIdentity{}, "", fmt.Errorf("invalid recorded repository identity: %w", err)
	}
	if identity.Host != repository.Host || identity.RepoPath != repository.RepoPath {
		return RepoIdentity{}, "", fmt.Errorf("recorded repository URL identity %s/%s does not match manifest %s/%s", identity.Host, identity.RepoPath, repository.Host, repository.RepoPath)
	}
	expectedCheckout, err := CheckoutPath(p, identity)
	if err != nil {
		return RepoIdentity{}, "", err
	}
	if !filepath.IsAbs(repository.CheckoutPath) {
		return RepoIdentity{}, "", fmt.Errorf("recorded checkout path %q is not absolute", repository.CheckoutPath)
	}
	recordedCheckout, err := filepath.Abs(filepath.Clean(repository.CheckoutPath))
	if err != nil {
		return RepoIdentity{}, "", fmt.Errorf("resolve recorded checkout path: %w", err)
	}
	expectedCheckout, err = filepath.Abs(filepath.Clean(expectedCheckout))
	if err != nil {
		return RepoIdentity{}, "", fmt.Errorf("resolve expected checkout path: %w", err)
	}
	if recordedCheckout != expectedCheckout {
		return RepoIdentity{}, "", fmt.Errorf("recorded checkout path %s does not match expected %s", recordedCheckout, expectedCheckout)
	}
	return identity, expectedCheckout, nil
}

func normalizeRecordedSkill(checkoutPath string, installed state.InstalledSkillEntry) (string, string, error) {
	name := strings.TrimSpace(installed.Name)
	if name == "" || name != installed.Name || filepath.Base(name) != name || name == "." || name == ".." {
		return "", "", fmt.Errorf("invalid recorded skill name %q", installed.Name)
	}
	relativePath := strings.TrimSpace(installed.RelativePath)
	if relativePath == "" || strings.Contains(relativePath, `\`) || strings.HasPrefix(relativePath, "/") {
		return "", "", fmt.Errorf("invalid recorded relative path %q", installed.RelativePath)
	}
	cleaned := pathpkg.Clean(relativePath)
	if cleaned != relativePath || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", "", fmt.Errorf("unsafe recorded relative path %q", installed.RelativePath)
	}
	if cleaned == "." {
		if filepath.Base(checkoutPath) != name {
			return "", "", fmt.Errorf("root skill name %q does not match checkout basename %q", name, filepath.Base(checkoutPath))
		}
	} else if pathpkg.Base(cleaned) != name {
		return "", "", fmt.Errorf("skill name %q does not match relative path %q", name, cleaned)
	}
	skillPath := filepath.Clean(filepath.Join(checkoutPath, filepath.FromSlash(cleaned)))
	if !pathInside(checkoutPath, skillPath) {
		return "", "", fmt.Errorf("recorded skill path escapes checkout")
	}
	return skillPath, cleaned, nil
}

func auditActiveReference(tool model.Tool, skillName, relativePath, skillPath, activePath string) (RepositoryReference, []ReferenceConflict) {
	reference := RepositoryReference{Tool: tool, SkillName: skillName, RelativePath: relativePath, SkillPath: skillPath, LinkPath: activePath, State: model.SkillStateOn}
	target, err := readExpectedSymlink(activePath, skillPath)
	if err != nil {
		return reference, []ReferenceConflict{{Tool: tool, SkillName: skillName, Path: activePath, Reason: err.Error()}}
	}
	reference.SymlinkTarget = target
	return reference, nil
}

func auditDisabledReference(p paths.Paths, disabled state.DisabledEntry, tool model.Tool, skillName, relativePath, skillPath, activePath string) (RepositoryReference, []ReferenceConflict) {
	disabledCopy := disabled
	reference := RepositoryReference{Tool: tool, SkillName: skillName, RelativePath: relativePath, SkillPath: skillPath, LinkPath: disabled.DisabledPath, State: model.SkillStateOff, DisabledRecord: &disabledCopy}
	conflicts := []ReferenceConflict{}
	expectedDisabledDir, _ := p.DisabledDirFor(tool)
	expectedDisabledPath := filepath.Join(expectedDisabledDir, skillName)
	if filepath.Clean(disabled.OriginalPath) != filepath.Clean(activePath) {
		conflicts = append(conflicts, ReferenceConflict{Tool: tool, SkillName: skillName, Path: disabled.OriginalPath, Reason: fmt.Sprintf("disabled original path does not match expected %s", activePath)})
	}
	if filepath.Clean(disabled.DisabledPath) != filepath.Clean(expectedDisabledPath) {
		conflicts = append(conflicts, ReferenceConflict{Tool: tool, SkillName: skillName, Path: disabled.DisabledPath, Reason: fmt.Sprintf("disabled path does not match expected %s", expectedDisabledPath)})
	}
	if disabled.EntryType != model.EntryTypeSymlink {
		conflicts = append(conflicts, ReferenceConflict{Tool: tool, SkillName: skillName, Path: disabled.DisabledPath, Reason: "disabled repository entry is not a symlink"})
	}
	if !samePath(resolveLinkTarget(disabled.OriginalPath, disabled.SymlinkTarget), skillPath) {
		conflicts = append(conflicts, ReferenceConflict{Tool: tool, SkillName: skillName, Path: disabled.DisabledPath, Reason: "disabled manifest symlink target does not match repository skill"})
	}
	target, err := readExpectedSymlink(disabled.DisabledPath, skillPath)
	if err != nil {
		conflicts = append(conflicts, ReferenceConflict{Tool: tool, SkillName: skillName, Path: disabled.DisabledPath, Reason: err.Error()})
	} else {
		reference.SymlinkTarget = target
	}
	return reference, conflicts
}

func readExpectedSymlink(linkPath, expectedTarget string) (string, error) {
	info, err := os.Lstat(linkPath)
	if err != nil {
		return "", fmt.Errorf("expected managed symlink is missing: %w", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return "", fmt.Errorf("expected managed path is not a symlink")
	}
	target, err := os.Readlink(linkPath)
	if err != nil {
		return "", fmt.Errorf("read managed symlink: %w", err)
	}
	if !samePath(resolveLinkTarget(linkPath, target), expectedTarget) {
		return target, fmt.Errorf("managed symlink points to %s, expected %s", resolveLinkTarget(linkPath, target), expectedTarget)
	}
	return target, nil
}

func findExtraManagedReferences(p paths.Paths, checkoutPath string, expected map[string]bool) ([]ReferenceConflict, error) {
	conflicts := []ReferenceConflict{}
	for _, baseDir := range []string{p.ClaudeUserSkills, p.CodexUserSkills, p.ClaudeDisabledDir, p.CodexDisabledDir} {
		entries, err := os.ReadDir(baseDir)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("scan managed references in %s: %w", baseDir, err)
		}
		for _, entry := range entries {
			entryPath := filepath.Join(baseDir, entry.Name())
			info, err := os.Lstat(entryPath)
			if err != nil {
				return nil, fmt.Errorf("inspect managed reference %s: %w", entryPath, err)
			}
			if info.Mode()&os.ModeSymlink == 0 {
				continue
			}
			target, err := os.Readlink(entryPath)
			if err != nil {
				return nil, fmt.Errorf("read managed reference %s: %w", entryPath, err)
			}
			if pathInside(checkoutPath, resolveLinkTarget(entryPath, target)) && !expected[filepath.Clean(entryPath)] {
				conflicts = append(conflicts, ReferenceConflict{SkillName: entry.Name(), Path: entryPath, Reason: "extra managed symlink points into repository checkout"})
			}
		}
	}
	return conflicts, nil
}

func pathInside(root, candidate string) bool {
	rootAbs, rootErr := filepath.Abs(filepath.Clean(root))
	candidateAbs, candidateErr := filepath.Abs(filepath.Clean(candidate))
	if rootErr != nil || candidateErr != nil {
		return false
	}
	relative, err := filepath.Rel(rootAbs, candidateAbs)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}

func repositoryCellKey(tool model.Tool, skillName string) string {
	return tool.String() + "\x00" + skillName
}

func sortReferenceConflicts(conflicts []ReferenceConflict) {
	sort.SliceStable(conflicts, func(i, j int) bool {
		if conflicts[i].Tool != conflicts[j].Tool {
			return conflicts[i].Tool.String() < conflicts[j].Tool.String()
		}
		if conflicts[i].SkillName != conflicts[j].SkillName {
			return conflicts[i].SkillName < conflicts[j].SkillName
		}
		if conflicts[i].Path != conflicts[j].Path {
			return conflicts[i].Path < conflicts[j].Path
		}
		return conflicts[i].Reason < conflicts[j].Reason
	})
}
