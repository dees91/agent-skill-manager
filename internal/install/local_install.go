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
	Source             LocalSource
	Links              []LinkPlan
	AlreadyInstalled   []AlreadyInstalled
	Off                bool
	ResolvedDuplicates []ResolvedDuplicate
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
	classify     managedClassifier
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
		classify:     defaultManagedClassifier(p),
	}
}

// PlanLocalInstall builds and preflights a local link-in-place install.
func PlanLocalInstall(p paths.Paths, manifest state.Manifest, source LocalSource, discovered Discovery, options PlanOptions) (LocalInstallPlan, error) {
	if strings.TrimSpace(source.OriginalPath) == "" || strings.TrimSpace(source.CanonicalPath) == "" || source.Group == "" {
		return LocalInstallPlan{}, fmt.Errorf("local source identity is incomplete")
	}
	recorded := RecordedLocalSkillPaths(manifest, source.CanonicalPath)
	recordedNames := RecordedLocalInstalledNames(manifest, source.CanonicalPath)
	selectedCells, resolution, err := selectInstallCells(p, source.CanonicalPath, discovered, options, recorded, recordedNames)
	if err != nil {
		return LocalInstallPlan{}, err
	}
	missingSkills := resolution.Missing
	if existing, ok := manifest.GetLocalSource(source.CanonicalPath); ok {
		allowedLinks, allowedDisabled := prospectiveLocalAuditAllowances(p, manifest, selectedCells)
		if _, err := auditLocalSourceReferences(p, manifest, existing, true, allowedLinks, allowedDisabled); err != nil {
			return LocalInstallPlan{}, err
		}
	}

	plan := LocalInstallPlan{Source: source, Links: []LinkPlan{}, AlreadyInstalled: []AlreadyInstalled{}, Off: options.Off, ResolvedDuplicates: resolution.Resolved}
	conflicts := []PreflightConflict{}
	suggest := ownershipSuggester(p, manifest, discovered, selectedCells, recordedNames, LocalSourceRef(source.CanonicalPath))
	if len(missingSkills) == 0 {
		for _, selected := range selectedCells {
			skill, tool := selected.Skill, selected.Tool
			if owner := managedCellOwner(manifest, tool, selected.installedName(), source.CanonicalPath); owner.found() {
				conflicts = append(conflicts, suggest(selected, owner))
				continue
			}
			link, already, conflict := planSkillToolWithOptions(p, manifest, skill, tool, selected.InstalledAs, options.Off)
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
	checkOwnership := func(tool model.Tool, installedName string) error {
		if owner := managedCellOwner(manifest, tool, installedName, plan.Source.CanonicalPath); owner.found() {
			return fmt.Errorf("install target %s/%s became owned by %s", tool, installedName, owner)
		}
		return nil
	}
	for _, link := range plan.Links {
		if err := checkOwnership(link.Tool, link.InstalledName()); err != nil {
			return result, err
		}
	}
	for _, already := range plan.AlreadyInstalled {
		if err := checkOwnership(already.Tool, already.InstalledName()); err != nil {
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
	created, err := createPlannedLinks(s.paths, plan.Links, s.symlink)
	result.Created = created
	if err != nil {
		return rollback(err)
	}

	entry, err := localSourceEntryForPlan(plan, manifest, s.now().UTC())
	if err != nil {
		return rollback(err)
	}
	manifest.UpsertLocalSource(entry)
	addDisabledRecords(&manifest, result.Created, s.classify, s.now().UTC())
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
		name, tool := cell.installedName(), cell.Tool
		activeDir, _ := p.UserSkillsDirFor(tool)
		activePath := filepath.Join(activeDir, name)
		if _, err := os.Lstat(activePath); err == nil {
			links[filepath.Clean(activePath)] = true
			continue
		}
		if disabled, ok := manifest.Get(tool, name); ok {
			links[filepath.Clean(disabled.DisabledPath)] = true
			disabledCells[repositoryCellKey(tool, name)] = true
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
			disabledCells[repositoryCellKey(already.Tool, already.InstalledName())] = true
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
	installedNames := map[string]string{}
	sameInstalledName := func(skillName, installedName string) error {
		if previous, ok := installedNames[skillName]; ok && previous != installedName {
			return fmt.Errorf("skill %s uses install names %s and %s", skillName, previous, installedName)
		}
		installedNames[skillName] = installedName
		return nil
	}
	for _, link := range plan.Links {
		if err := s.validateLinkPlan(plan.CheckoutPath, link); err != nil {
			return err
		}
		targetKey := filepath.Clean(link.LinkPath())
		if seenTargets[targetKey] {
			return fmt.Errorf("duplicate install target %s", link.LinkPath())
		}
		seenTargets[targetKey] = true
		cellKey := repositoryCellKey(link.Tool, link.InstalledName())
		if seenCells[cellKey] {
			return fmt.Errorf("duplicate install cell %s/%s", link.Tool, link.InstalledName())
		}
		seenCells[cellKey] = true
		if err := sameInstalledName(link.Skill.Name, link.InstalledName()); err != nil {
			return err
		}
		if err := validatePathFreeForApply(link.TargetPath); err != nil {
			return fmt.Errorf("validate install target %s: %w", link.TargetPath, err)
		}
		if link.DisabledPath != "" {
			if err := validatePathFreeForApply(link.DisabledPath); err != nil {
				return fmt.Errorf("validate install disabled path %s: %w", link.DisabledPath, err)
			}
		}
	}
	for _, already := range plan.AlreadyInstalled {
		if err := s.validateAlreadyInstalledPlan(plan.CheckoutPath, already); err != nil {
			return err
		}
		cellKey := repositoryCellKey(already.Tool, already.InstalledName())
		if seenCells[cellKey] {
			return fmt.Errorf("duplicate install cell %s/%s", already.Tool, already.InstalledName())
		}
		seenCells[cellKey] = true
		if err := sameInstalledName(already.Skill.Name, already.InstalledName()); err != nil {
			return err
		}
	}
	return nil
}

// cellOwner identifies the recorded source that owns one tool/installed-name
// cell. The zero value means the cell is free.
type cellOwner struct {
	kind  string
	id    string
	group model.GroupLabel
}

func (o cellOwner) found() bool {
	return o.kind != ""
}

func (o cellOwner) String() string {
	if !o.found() {
		return ""
	}
	return o.kind + " " + o.id
}

func managedCellOwner(manifest state.Manifest, tool model.Tool, installedName, currentLocalPath string) cellOwner {
	if owner := localCellOwner(manifest, tool, installedName, currentLocalPath); owner.found() {
		return owner
	}
	return repositoryCellOwner(manifest, tool, installedName, "", "")
}

func repositoryCellOwner(manifest state.Manifest, tool model.Tool, installedName, currentHost, currentRepoPath string) cellOwner {
	for _, repository := range manifest.Repositories {
		if currentHost != "" && repository.Host == currentHost && repository.RepoPath == currentRepoPath {
			continue
		}
		for _, skill := range repository.InstalledSkills {
			if skill.InstalledName() == installedName && containsTool(skill.Tools, tool) {
				return cellOwner{kind: "repository", id: repository.Host + "/" + repository.RepoPath, group: repository.Group}
			}
		}
	}
	return cellOwner{}
}

func gitInstallCellOwner(manifest state.Manifest, tool model.Tool, installedName string, identity RepoIdentity) cellOwner {
	if owner := localCellOwner(manifest, tool, installedName, ""); owner.found() {
		return owner
	}
	return repositoryCellOwner(manifest, tool, installedName, identity.Host, identity.RepoPath)
}

func localCellOwner(manifest state.Manifest, tool model.Tool, installedName, currentLocalPath string) cellOwner {
	for _, source := range manifest.LocalSources {
		if currentLocalPath != "" && samePath(source.CanonicalPath, currentLocalPath) {
			continue
		}
		for _, skill := range source.InstalledSkills {
			if skill.InstalledName() == installedName && containsTool(skill.Tools, tool) {
				return cellOwner{kind: "local source", id: source.CanonicalPath, group: source.Group}
			}
		}
	}
	return cellOwner{}
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
	add := func(skill DiscoveredSkill, tool model.Tool, installedAs string) error {
		relativePath, err := filepath.Rel(plan.Source.CanonicalPath, skill.Path)
		if err != nil {
			return fmt.Errorf("resolve local installed skill path for %s: %w", skill.Name, err)
		}
		relativePath = filepath.ToSlash(relativePath)
		key := skill.Name + "\x00" + relativePath
		installed, err := recordInstalledSkillTool(skills[key], skill.Name, relativePath, installedAs, tool)
		if err != nil {
			return err
		}
		skills[key] = installed
		return nil
	}
	for _, link := range plan.Links {
		if err := add(link.Skill, link.Tool, link.InstalledAs); err != nil {
			return state.LocalSourceEntry{}, err
		}
	}
	for _, already := range plan.AlreadyInstalled {
		if err := add(already.Skill, already.Tool, already.InstalledAs); err != nil {
			return state.LocalSourceEntry{}, err
		}
	}
	entry.InstalledSkills = make([]state.InstalledSkillEntry, 0, len(skills))
	for _, skill := range skills {
		entry.InstalledSkills = append(entry.InstalledSkills, skill)
	}
	return entry, nil
}

// recordInstalledSkillTool adds one tool to a manifest skill record. A skill
// keeps one installed name across tools; a different name for a recorded
// skill is drift that only uninstall plus reinstall may change.
func recordInstalledSkillTool(installed state.InstalledSkillEntry, name, relativePath, installedAs string, tool model.Tool) (state.InstalledSkillEntry, error) {
	if installedAs == name {
		installedAs = ""
	}
	if installed.Name != "" && installed.InstalledAs != installedAs {
		return state.InstalledSkillEntry{}, fmt.Errorf("skill %s is recorded as %s, not %s", name, installed.InstalledName(), installedNameOf(DiscoveredSkill{Name: name}, installedAs))
	}
	installed.Name = name
	installed.InstalledAs = installedAs
	installed.RelativePath = relativePath
	installed.Tools = append(installed.Tools, tool)
	return installed, nil
}
