package install

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dees91/agent-skill-manager/internal/model"
	"github.com/dees91/agent-skill-manager/internal/paths"
	"github.com/dees91/agent-skill-manager/internal/state"
)

// ApplyService applies install plans to the filesystem and state manifest.
type ApplyService struct {
	paths    paths.Paths
	store    state.Store
	now      func() time.Time
	symlink  func(string, string) error
	backedUp bool
}

// NewApplyService creates an install apply service for the provided paths.
func NewApplyService(p paths.Paths) *ApplyService {
	return &ApplyService{
		paths:   p,
		store:   state.New(p),
		now:     time.Now,
		symlink: os.Symlink,
	}
}

// ApplyResult describes install symlink application and manifest update.
type ApplyResult struct {
	Created          []LinkPlan
	AlreadyInstalled []AlreadyInstalled
	RolledBack       []LinkPlan
	Repository       state.RepositoryEntry
}

// Apply creates planned symlinks and updates repository install state on success.
func (s *ApplyService) Apply(plan InstallPlan, lastSeenCommit string) (ApplyResult, error) {
	result := ApplyResult{
		Created:          []LinkPlan{},
		AlreadyInstalled: append([]AlreadyInstalled(nil), plan.AlreadyInstalled...),
		RolledBack:       []LinkPlan{},
	}

	if err := s.validatePlanForApply(plan); err != nil {
		return result, err
	}
	plan.CheckoutPath, _ = filepath.Abs(filepath.Clean(plan.CheckoutPath))
	if !s.backedUp {
		if _, err := s.store.BackupExisting(); err != nil {
			return result, err
		}
		s.backedUp = true
	}
	manifest, err := s.store.Load()
	if err != nil {
		return result, err
	}
	if err := revalidateGitCellOwnership(plan, manifest); err != nil {
		return result, err
	}
	if err := s.revalidateAlreadyInstalled(plan, manifest); err != nil {
		return result, err
	}
	if err := revalidateLinksAgainstManifest(plan, manifest); err != nil {
		return result, err
	}

	for _, link := range plan.Links {
		if err := os.MkdirAll(filepath.Dir(link.TargetPath), 0o755); err != nil {
			result.RolledBack = rollbackCreated(result.Created)
			return result, combineRollbackError(fmt.Errorf("create symlink parent %s: %w", filepath.Dir(link.TargetPath), err), result.Created, result.RolledBack)
		}
		if err := s.symlink(link.Skill.Path, link.TargetPath); err != nil {
			result.RolledBack = rollbackCreated(result.Created)
			return result, combineRollbackError(fmt.Errorf("create symlink %s -> %s: %w", link.TargetPath, link.Skill.Path, err), result.Created, result.RolledBack)
		}
		result.Created = append(result.Created, link)
	}

	repository, err := s.repositoryEntryForPlan(plan, manifest, lastSeenCommit)
	if err != nil {
		return result, err
	}
	manifest.UpsertRepository(repository)
	if err := s.store.Save(manifest); err != nil {
		return result, err
	}
	result.Repository, _ = manifest.GetRepository(plan.Identity.Host, plan.Identity.RepoPath)
	return result, nil
}

func revalidateGitCellOwnership(plan InstallPlan, manifest state.Manifest) error {
	check := func(tool model.Tool, skillName string) error {
		if owner := gitInstallCellOwner(manifest, tool, skillName, plan.Identity); owner != "" {
			return fmt.Errorf("install cell %s/%s became owned by %s", tool, skillName, owner)
		}
		return nil
	}
	for _, link := range plan.Links {
		if err := check(link.Tool, link.Skill.Name); err != nil {
			return err
		}
	}
	for _, already := range plan.AlreadyInstalled {
		if err := check(already.Tool, already.Skill.Name); err != nil {
			return err
		}
	}
	return nil
}

func (s *ApplyService) validatePlanForApply(plan InstallPlan) error {
	if err := validatePlanIdentity(plan.Identity); err != nil {
		return err
	}
	expectedCheckout, err := CheckoutPath(s.paths, plan.Identity)
	if err != nil {
		return err
	}
	planCheckout, err := filepath.Abs(filepath.Clean(plan.CheckoutPath))
	if err != nil {
		return fmt.Errorf("resolve plan checkout path: %w", err)
	}
	expectedCheckout, err = filepath.Abs(filepath.Clean(expectedCheckout))
	if err != nil {
		return fmt.Errorf("resolve expected checkout path: %w", err)
	}
	if planCheckout != expectedCheckout {
		return fmt.Errorf("install plan checkout path %s does not match expected %s", planCheckout, expectedCheckout)
	}
	plan.CheckoutPath = planCheckout
	return s.validateInstallCells(plan)
}

func (s *ApplyService) validateLinkPlan(checkoutPath string, link LinkPlan) error {
	if err := validateSkillForApply(checkoutPath, link.Skill); err != nil {
		return err
	}
	userDir, ok := s.paths.UserSkillsDirFor(link.Tool)
	if !ok {
		return fmt.Errorf("invalid install tool %q", link.Tool)
	}
	expectedTarget := filepath.Join(userDir, link.Skill.Name)
	if filepath.Clean(link.TargetPath) != expectedTarget {
		return fmt.Errorf("install target %s does not match expected %s", link.TargetPath, expectedTarget)
	}
	return nil
}

func (s *ApplyService) validateAlreadyInstalledPlan(checkoutPath string, already AlreadyInstalled) error {
	if err := validateSkillForApply(checkoutPath, already.Skill); err != nil {
		return err
	}
	userDir, ok := s.paths.UserSkillsDirFor(already.Tool)
	if !ok {
		return fmt.Errorf("invalid install tool %q", already.Tool)
	}
	expectedTarget := filepath.Join(userDir, already.Skill.Name)
	if filepath.Clean(already.TargetPath) != expectedTarget {
		return fmt.Errorf("already-installed target %s does not match expected %s", already.TargetPath, expectedTarget)
	}
	if already.State != model.SkillStateOn && already.State != model.SkillStateOff {
		return fmt.Errorf("already-installed %s/%s has unsupported state %s", already.Tool, already.Skill.Name, already.State)
	}
	return nil
}

func validateSkillForApply(checkoutPath string, skill DiscoveredSkill) error {
	if _, err := validateDiscoveredSkill(checkoutPath, skill); err != nil {
		return err
	}
	info, err := os.Lstat(skill.Path)
	if err != nil {
		return fmt.Errorf("inspect skill path %s: %w", skill.Path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("skill path %s is a symlink", skill.Path)
	}
	if !info.IsDir() {
		return fmt.Errorf("skill path %s is not a directory", skill.Path)
	}
	skillFile := filepath.Join(skill.Path, "SKILL.md")
	if info, err := os.Lstat(skillFile); err != nil || info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		if err != nil {
			return fmt.Errorf("inspect skill file %s: %w", skillFile, err)
		}
		return fmt.Errorf("skill file %s is not a regular file", skillFile)
	}
	return nil
}

func revalidateLinksAgainstManifest(plan InstallPlan, manifest state.Manifest) error {
	for _, link := range plan.Links {
		if entry, ok := manifest.Get(link.Tool, link.Skill.Name); ok {
			return fmt.Errorf("install target %s/%s became disabled at %s", link.Tool, link.Skill.Name, entry.DisabledPath)
		}
	}
	return nil
}

func (s *ApplyService) revalidateAlreadyInstalled(plan InstallPlan, manifest state.Manifest) error {
	for _, already := range plan.AlreadyInstalled {
		switch already.State {
		case model.SkillStateOn:
			target, err := os.Readlink(already.TargetPath)
			if err != nil {
				return fmt.Errorf("already-installed %s/%s is not active: %w", already.Tool, already.Skill.Name, err)
			}
			if !samePath(resolveLinkTarget(already.TargetPath, target), already.Skill.Path) {
				return fmt.Errorf("already-installed %s/%s active symlink target changed", already.Tool, already.Skill.Name)
			}
		case model.SkillStateOff:
			entry, ok := manifest.Get(already.Tool, already.Skill.Name)
			if !ok {
				return fmt.Errorf("already-installed %s/%s disabled state is missing", already.Tool, already.Skill.Name)
			}
			if entry.EntryType != model.EntryTypeSymlink || !samePath(resolveLinkTarget(entry.OriginalPath, entry.SymlinkTarget), already.Skill.Path) {
				return fmt.Errorf("already-installed %s/%s disabled state target changed", already.Tool, already.Skill.Name)
			}
			if already.DisabledPath != "" && entry.DisabledPath != already.DisabledPath {
				return fmt.Errorf("already-installed %s/%s disabled path changed", already.Tool, already.Skill.Name)
			}
			if _, err := os.Lstat(already.TargetPath); err == nil {
				return fmt.Errorf("already-installed %s/%s is disabled but active target exists", already.Tool, already.Skill.Name)
			} else if !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("inspect disabled active target %s: %w", already.TargetPath, err)
			}
			target, err := os.Readlink(entry.DisabledPath)
			if err != nil {
				return fmt.Errorf("already-installed %s/%s disabled symlink is missing: %w", already.Tool, already.Skill.Name, err)
			}
			if !samePath(resolveLinkTarget(entry.DisabledPath, target), already.Skill.Path) {
				return fmt.Errorf("already-installed %s/%s disabled symlink target changed", already.Tool, already.Skill.Name)
			}
		}
	}
	return nil
}

func (s *ApplyService) repositoryEntryForPlan(plan InstallPlan, manifest state.Manifest, lastSeenCommit string) (state.RepositoryEntry, error) {
	installedAt := s.now().UTC()
	entry, ok := manifest.GetRepository(plan.Identity.Host, plan.Identity.RepoPath)
	if ok && !entry.InstalledAt.IsZero() {
		installedAt = entry.InstalledAt
	}
	entry.OriginalURL = plan.Identity.OriginalURL
	entry.CanonicalURL = plan.Identity.CanonicalURL
	entry.Host = plan.Identity.Host
	entry.RepoPath = plan.Identity.RepoPath
	entry.CheckoutPath = plan.CheckoutPath
	entry.Group = plan.Identity.Group
	entry.InstalledAt = installedAt
	entry.LastSeenCommit = lastSeenCommit

	skills := map[string]state.InstalledSkillEntry{}
	for _, skill := range entry.InstalledSkills {
		key := skill.Name + "\x00" + skill.RelativePath
		skills[key] = skill
	}
	addSkillTool := func(skill DiscoveredSkill, tool model.Tool) error {
		relativePath, err := filepath.Rel(plan.CheckoutPath, skill.Path)
		if err != nil {
			return fmt.Errorf("resolve installed skill path for %s: %w", skill.Name, err)
		}
		relativePath = filepath.ToSlash(relativePath)
		key := skill.Name + "\x00" + relativePath
		installed := skills[key]
		installed.Name = skill.Name
		installed.RelativePath = relativePath
		installed.Tools = append(installed.Tools, tool)
		skills[key] = installed
		return nil
	}
	for _, link := range plan.Links {
		if err := addSkillTool(link.Skill, link.Tool); err != nil {
			return state.RepositoryEntry{}, err
		}
	}
	for _, already := range plan.AlreadyInstalled {
		if err := addSkillTool(already.Skill, already.Tool); err != nil {
			return state.RepositoryEntry{}, err
		}
	}

	entry.InstalledSkills = make([]state.InstalledSkillEntry, 0, len(skills))
	for _, skill := range skills {
		entry.InstalledSkills = append(entry.InstalledSkills, skill)
	}
	return entry, nil
}

func rollbackCreated(created []LinkPlan) []LinkPlan {
	rolledBack := []LinkPlan{}
	for i := len(created) - 1; i >= 0; i-- {
		link := created[i]
		target, err := os.Readlink(link.TargetPath)
		if err != nil {
			continue
		}
		if !samePath(resolveLinkTarget(link.TargetPath, target), link.Skill.Path) {
			continue
		}
		if err := os.Remove(link.TargetPath); err == nil {
			rolledBack = append(rolledBack, link)
		}
	}
	return rolledBack
}

func combineRollbackError(original error, created, rolledBack []LinkPlan) error {
	if len(created) == len(rolledBack) {
		return original
	}
	return fmt.Errorf("%w; rollback removed %d of %d created symlinks", original, len(rolledBack), len(created))
}

func validatePathFreeForApply(path string) error {
	_, err := os.Lstat(path)
	if err == nil {
		return fmt.Errorf("%s already exists", path)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
