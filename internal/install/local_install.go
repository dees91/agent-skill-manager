package install

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dees91/agent-skill-manager/internal/model"
	"github.com/dees91/agent-skill-manager/internal/paths"
	"github.com/dees91/agent-skill-manager/internal/state"
)

// LocalInstallPlan is a side-effect-free plan for link-in-place installation.
type LocalInstallPlan struct {
	Source           LocalSource
	Links            []LinkPlan
	AlreadyInstalled []AlreadyInstalled
}

// LocalApplyResult describes local symlink application and state persistence.
type LocalApplyResult struct {
	Created          []LinkPlan
	AlreadyInstalled []AlreadyInstalled
	RolledBack       []LinkPlan
	Source           state.LocalSourceEntry
}

// LocalApplyService applies local link-in-place install plans.
type LocalApplyService struct {
	paths        paths.Paths
	store        state.Store
	now          func() time.Time
	symlink      func(string, string) error
	saveManifest func(state.Manifest) error
	backedUp     bool
}

// NewLocalApplyService creates a local install service.
func NewLocalApplyService(p paths.Paths) *LocalApplyService {
	store := state.New(p)
	return &LocalApplyService{
		paths:        p,
		store:        store,
		now:          time.Now,
		symlink:      os.Symlink,
		saveManifest: store.Save,
	}
}

// PlanLocalInstall builds and preflights a local link-in-place install.
func PlanLocalInstall(p paths.Paths, manifest state.Manifest, source LocalSource, discovered []DiscoveredSkill, options PlanOptions) (LocalInstallPlan, error) {
	if strings.TrimSpace(source.OriginalPath) == "" || strings.TrimSpace(source.CanonicalPath) == "" || source.Group == "" {
		return LocalInstallPlan{}, fmt.Errorf("local source identity is incomplete")
	}
	selectedCells, missingSkills, err := selectInstallCells(p, source.CanonicalPath, discovered, options)
	if err != nil {
		return LocalInstallPlan{}, err
	}
	if existing, ok := manifest.GetLocalSource(source.CanonicalPath); ok {
		allowedLinks, allowedDisabled := prospectiveLocalAuditAllowances(p, manifest, selectedCells)
		if _, err := auditLocalSourceReferences(p, manifest, existing, true, allowedLinks, allowedDisabled); err != nil {
			return LocalInstallPlan{}, err
		}
	}

	plan := LocalInstallPlan{Source: source, Links: []LinkPlan{}, AlreadyInstalled: []AlreadyInstalled{}}
	conflicts := []PreflightConflict{}
	if len(missingSkills) == 0 {
		for _, selected := range selectedCells {
			skill, tool := selected.Skill, selected.Tool
			if owner := managedCellOwner(manifest, tool, skill.Name, source.CanonicalPath); owner != "" {
				activeDir, _ := p.UserSkillsDirFor(tool)
				conflicts = append(conflicts, PreflightConflict{
					SkillName:  skill.Name,
					Tool:       tool,
					TargetPath: filepath.Join(activeDir, skill.Name),
					Reason:     "cell is already owned by " + owner,
					Expected:   skill.Path,
				})
				continue
			}
			link, already, conflict := planSkillTool(p, manifest, skill, tool)
			if conflict != nil {
				conflicts = append(conflicts, *conflict)
				continue
			}
			if already != nil {
				plan.AlreadyInstalled = append(plan.AlreadyInstalled, *already)
				continue
			}
			if link != nil {
				plan.Links = append(plan.Links, *link)
			}
		}
	}
	if len(missingSkills) > 0 || len(conflicts) > 0 {
		sort.Strings(missingSkills)
		sortConflicts(conflicts)
		return LocalInstallPlan{}, PlanError{MissingSkills: missingSkills, Conflicts: conflicts}
	}
	return plan, nil
}

// Apply creates local symlinks and records ownership only after a successful apply.
func (s *LocalApplyService) Apply(plan LocalInstallPlan) (LocalApplyResult, error) {
	result := LocalApplyResult{
		Created:          []LinkPlan{},
		AlreadyInstalled: append([]AlreadyInstalled(nil), plan.AlreadyInstalled...),
		RolledBack:       []LinkPlan{},
	}
	if err := s.validatePlan(plan); err != nil {
		return result, err
	}
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
	if existing, ok := manifest.GetLocalSource(plan.Source.CanonicalPath); ok {
		allowedLinks, allowedDisabled := plannedLocalAuditAllowances(plan)
		if _, err := auditLocalSourceReferences(s.paths, manifest, existing, true, allowedLinks, allowedDisabled); err != nil {
			return result, err
		}
	}
	checkOwnership := func(tool model.Tool, skillName string) error {
		if owner := managedCellOwner(manifest, tool, skillName, plan.Source.CanonicalPath); owner != "" {
			return fmt.Errorf("install target %s/%s became owned by %s", tool, skillName, owner)
		}
		return nil
	}
	for _, link := range plan.Links {
		if err := checkOwnership(link.Tool, link.Skill.Name); err != nil {
			return result, err
		}
	}
	for _, already := range plan.AlreadyInstalled {
		if err := checkOwnership(already.Tool, already.Skill.Name); err != nil {
			return result, err
		}
	}
	validator := NewApplyService(s.paths)
	converted := InstallPlan{CheckoutPath: plan.Source.CanonicalPath, Links: plan.Links, AlreadyInstalled: plan.AlreadyInstalled}
	if err := validator.validateInstallCells(converted); err != nil {
		return result, err
	}
	if err := validator.revalidateAlreadyInstalled(converted, manifest); err != nil {
		return result, err
	}
	if err := revalidateLinksAgainstManifest(converted, manifest); err != nil {
		return result, err
	}

	rollback := func(original error) (LocalApplyResult, error) {
		result.RolledBack = rollbackCreated(result.Created)
		return result, combineRollbackError(original, result.Created, result.RolledBack)
	}
	for _, link := range plan.Links {
		if err := os.MkdirAll(filepath.Dir(link.TargetPath), 0o755); err != nil {
			return rollback(fmt.Errorf("create symlink parent %s: %w", filepath.Dir(link.TargetPath), err))
		}
		if err := s.symlink(link.Skill.Path, link.TargetPath); err != nil {
			return rollback(fmt.Errorf("create symlink %s -> %s: %w", link.TargetPath, link.Skill.Path, err))
		}
		result.Created = append(result.Created, link)
	}

	entry, err := localSourceEntryForPlan(plan, manifest, s.now().UTC())
	if err != nil {
		return rollback(err)
	}
	manifest.UpsertLocalSource(entry)
	if err := s.saveManifest(manifest); err != nil {
		return rollback(fmt.Errorf("save local install state: %w", err))
	}
	result.Source, _ = manifest.GetLocalSource(plan.Source.CanonicalPath)
	return result, nil
}

func prospectiveLocalAuditAllowances(p paths.Paths, manifest state.Manifest, cells []selectedInstallCell) (map[string]bool, map[string]bool) {
	links := map[string]bool{}
	disabledCells := map[string]bool{}
	for _, cell := range cells {
		skill, tool := cell.Skill, cell.Tool
		activeDir, _ := p.UserSkillsDirFor(tool)
		activePath := filepath.Join(activeDir, skill.Name)
		if _, err := os.Lstat(activePath); err == nil {
			links[filepath.Clean(activePath)] = true
			continue
		}
		if disabled, ok := manifest.Get(tool, skill.Name); ok {
			links[filepath.Clean(disabled.DisabledPath)] = true
			disabledCells[repositoryCellKey(tool, skill.Name)] = true
		}
	}
	return links, disabledCells
}

func plannedLocalAuditAllowances(plan LocalInstallPlan) (map[string]bool, map[string]bool) {
	links := map[string]bool{}
	disabledCells := map[string]bool{}
	for _, already := range plan.AlreadyInstalled {
		switch already.State {
		case model.SkillStateOn:
			links[filepath.Clean(already.TargetPath)] = true
		case model.SkillStateOff:
			links[filepath.Clean(already.DisabledPath)] = true
			disabledCells[repositoryCellKey(already.Tool, already.Skill.Name)] = true
		}
	}
	return links, disabledCells
}

func (s *LocalApplyService) validatePlan(plan LocalInstallPlan) error {
	resolved, err := ResolveLocalSource(s.paths, string(filepath.Separator), plan.Source.OriginalPath)
	if err != nil {
		return err
	}
	if !samePath(resolved.CanonicalPath, plan.Source.CanonicalPath) || resolved.Group != plan.Source.Group {
		return fmt.Errorf("local source identity changed since planning")
	}
	converted := InstallPlan{CheckoutPath: plan.Source.CanonicalPath, Links: plan.Links, AlreadyInstalled: plan.AlreadyInstalled}
	return NewApplyService(s.paths).validateInstallCells(converted)
}

func (s *ApplyService) validateInstallCells(plan InstallPlan) error {
	seenTargets := map[string]bool{}
	seenCells := map[string]bool{}
	for _, link := range plan.Links {
		if err := s.validateLinkPlan(plan.CheckoutPath, link); err != nil {
			return err
		}
		targetKey := filepath.Clean(link.TargetPath)
		if seenTargets[targetKey] {
			return fmt.Errorf("duplicate install target %s", link.TargetPath)
		}
		seenTargets[targetKey] = true
		cellKey := repositoryCellKey(link.Tool, link.Skill.Name)
		if seenCells[cellKey] {
			return fmt.Errorf("duplicate install cell %s/%s", link.Tool, link.Skill.Name)
		}
		seenCells[cellKey] = true
		if err := validatePathFreeForApply(link.TargetPath); err != nil {
			return fmt.Errorf("validate install target %s: %w", link.TargetPath, err)
		}
	}
	for _, already := range plan.AlreadyInstalled {
		if err := s.validateAlreadyInstalledPlan(plan.CheckoutPath, already); err != nil {
			return err
		}
		cellKey := repositoryCellKey(already.Tool, already.Skill.Name)
		if seenCells[cellKey] {
			return fmt.Errorf("duplicate install cell %s/%s", already.Tool, already.Skill.Name)
		}
		seenCells[cellKey] = true
	}
	return nil
}

func managedCellOwner(manifest state.Manifest, tool model.Tool, skillName, currentLocalPath string) string {
	if owner := localCellOwner(manifest, tool, skillName, currentLocalPath); owner != "" {
		return owner
	}
	return repositoryCellOwner(manifest, tool, skillName, "", "")
}

func repositoryCellOwner(manifest state.Manifest, tool model.Tool, skillName, currentHost, currentRepoPath string) string {
	for _, repository := range manifest.Repositories {
		if currentHost != "" && repository.Host == currentHost && repository.RepoPath == currentRepoPath {
			continue
		}
		for _, skill := range repository.InstalledSkills {
			if skill.Name == skillName && containsTool(skill.Tools, tool) {
				return "repository " + repository.Host + "/" + repository.RepoPath
			}
		}
	}
	return ""
}

func gitInstallCellOwner(manifest state.Manifest, tool model.Tool, skillName string, identity RepoIdentity) string {
	if owner := localCellOwner(manifest, tool, skillName, ""); owner != "" {
		return owner
	}
	return repositoryCellOwner(manifest, tool, skillName, identity.Host, identity.RepoPath)
}

func localCellOwner(manifest state.Manifest, tool model.Tool, skillName, currentLocalPath string) string {
	for _, source := range manifest.LocalSources {
		if currentLocalPath != "" && samePath(source.CanonicalPath, currentLocalPath) {
			continue
		}
		for _, skill := range source.InstalledSkills {
			if skill.Name == skillName && containsTool(skill.Tools, tool) {
				return "local source " + source.CanonicalPath
			}
		}
	}
	return ""
}

func containsTool(tools []model.Tool, wanted model.Tool) bool {
	for _, tool := range tools {
		if tool == wanted {
			return true
		}
	}
	return false
}

func localSourceEntryForPlan(plan LocalInstallPlan, manifest state.Manifest, now time.Time) (state.LocalSourceEntry, error) {
	entry, ok := manifest.GetLocalSource(plan.Source.CanonicalPath)
	if !ok || entry.InstalledAt.IsZero() {
		entry.InstalledAt = now
	}
	entry.OriginalPath = plan.Source.OriginalPath
	entry.CanonicalPath = plan.Source.CanonicalPath
	entry.Group = plan.Source.Group
	skills := map[string]state.InstalledSkillEntry{}
	for _, skill := range entry.InstalledSkills {
		skills[skill.Name+"\x00"+skill.RelativePath] = skill
	}
	add := func(skill DiscoveredSkill, tool model.Tool) error {
		relativePath, err := filepath.Rel(plan.Source.CanonicalPath, skill.Path)
		if err != nil {
			return fmt.Errorf("resolve local installed skill path for %s: %w", skill.Name, err)
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
		if err := add(link.Skill, link.Tool); err != nil {
			return state.LocalSourceEntry{}, err
		}
	}
	for _, already := range plan.AlreadyInstalled {
		if err := add(already.Skill, already.Tool); err != nil {
			return state.LocalSourceEntry{}, err
		}
	}
	entry.InstalledSkills = make([]state.InstalledSkillEntry, 0, len(skills))
	for _, skill := range skills {
		entry.InstalledSkills = append(entry.InstalledSkills, skill)
	}
	return entry, nil
}
