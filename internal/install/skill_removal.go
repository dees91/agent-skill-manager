package install

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dees91/agent-skill-manager/internal/model"
	"github.com/dees91/agent-skill-manager/internal/paths"
	"github.com/dees91/agent-skill-manager/internal/state"
)

// SkillRemovalPlan is a validated removal of some recorded skills of one
// source. References holds only the links of the selected skills.
type SkillRemovalPlan struct {
	Group      model.GroupLabel
	Skills     []state.InstalledSkillEntry
	References []RepositoryReference
}

// SkillRemovalResult describes removed links and any recovery residue.
type SkillRemovalResult struct {
	Group           model.GroupLabel
	Skills          []state.InstalledSkillEntry
	RemovedActive   []RepositoryReference
	RemovedDisabled []RepositoryReference
	RolledBack      []StagedMove
	CleanupPending  string
}

// SkillRemovalService removes selected skills from one Git or local source
// (Iteration 27). It never touches the managed checkout, the local source
// directory, or Git refs, so it needs no clean checkout and never fetches.
type SkillRemovalService struct {
	paths          paths.Paths
	store          state.Store
	backedUp       bool
	rename         func(string, string) error
	removeAll      func(string) error
	mkdirAll       func(string, os.FileMode) error
	mkdirTemp      func(string, string) (string, error)
	saveManifest   func(state.Manifest) error
	backupExisting func() (string, error)
}

// NewSkillRemovalService creates a skill removal service.
func NewSkillRemovalService(p paths.Paths) *SkillRemovalService {
	store := state.New(p)
	return &SkillRemovalService{
		paths:          p,
		store:          store,
		rename:         os.Rename,
		removeAll:      os.RemoveAll,
		mkdirAll:       os.MkdirAll,
		mkdirTemp:      os.MkdirTemp,
		saveManifest:   store.Save,
		backupExisting: store.BackupExisting,
	}
}

// removalTarget is one prepared removal with the manifest edit that records it.
type removalTarget struct {
	manifest state.Manifest
	plan     SkillRemovalPlan
	commit   func(*state.Manifest) bool
}

// PlanRepository validates a removal from a Git source without mutation.
func (s *SkillRemovalService) PlanRepository(repository state.RepositoryEntry, names []string) (SkillRemovalPlan, error) {
	target, err := s.prepareRepository(repository, names)
	return target.plan, err
}

// ApplyRepository removes the selected skills from a Git source.
func (s *SkillRemovalService) ApplyRepository(repository state.RepositoryEntry, names []string) (SkillRemovalResult, error) {
	target, err := s.prepareRepository(repository, names)
	if err != nil {
		return SkillRemovalResult{Group: repository.Group, RemovedActive: []RepositoryReference{}, RemovedDisabled: []RepositoryReference{}, RolledBack: []StagedMove{}}, err
	}
	return s.apply(target)
}

// PlanLocal validates a removal from a local source without mutation.
func (s *SkillRemovalService) PlanLocal(source state.LocalSourceEntry, names []string) (SkillRemovalPlan, error) {
	target, err := s.prepareLocal(source, names)
	return target.plan, err
}

// ApplyLocal removes the selected skills from a local source.
func (s *SkillRemovalService) ApplyLocal(source state.LocalSourceEntry, names []string) (SkillRemovalResult, error) {
	target, err := s.prepareLocal(source, names)
	if err != nil {
		return SkillRemovalResult{Group: source.Group, RemovedActive: []RepositoryReference{}, RemovedDisabled: []RepositoryReference{}, RolledBack: []StagedMove{}}, err
	}
	return s.apply(target)
}

func (s *SkillRemovalService) prepareRepository(repository state.RepositoryEntry, names []string) (removalTarget, error) {
	manifest, err := s.store.Load()
	if err != nil {
		return removalTarget{}, err
	}
	current, ok := manifest.GetRepository(repository.Host, repository.RepoPath)
	if !ok {
		return removalTarget{}, fmt.Errorf("managed repository %s/%s not found in state", repository.Host, repository.RepoPath)
	}
	selected, err := resolveRemovalSelection(current.Group, current.InstalledSkills, names)
	if err != nil {
		return removalTarget{}, err
	}
	audit, err := AuditRepositoryReferences(s.paths, manifest, current)
	if err != nil {
		return removalTarget{}, err
	}
	kept := keptSkills(current.InstalledSkills, selected)
	commit := func(m *state.Manifest) bool {
		latest, ok := m.GetRepository(current.Host, current.RepoPath)
		if !ok {
			return false
		}
		latest.InstalledSkills = kept
		m.UpsertRepository(latest)
		return true
	}
	return removalTarget{manifest: manifest, plan: removalPlan(current.Group, selected, audit.References), commit: commit}, nil
}

func (s *SkillRemovalService) prepareLocal(source state.LocalSourceEntry, names []string) (removalTarget, error) {
	manifest, err := s.store.Load()
	if err != nil {
		return removalTarget{}, err
	}
	current, ok := manifest.GetLocalSource(source.CanonicalPath)
	if !ok {
		return removalTarget{}, fmt.Errorf("local source %s not found in state", source.CanonicalPath)
	}
	selected, err := resolveRemovalSelection(current.Group, current.InstalledSkills, names)
	if err != nil {
		return removalTarget{}, err
	}
	audit, err := AuditLocalSourceReferences(s.paths, manifest, current, false)
	if err != nil {
		return removalTarget{}, err
	}
	kept := keptSkills(current.InstalledSkills, selected)
	commit := func(m *state.Manifest) bool {
		latest, ok := m.GetLocalSource(current.CanonicalPath)
		if !ok {
			return false
		}
		latest.InstalledSkills = kept
		m.UpsertLocalSource(latest)
		return true
	}
	return removalTarget{manifest: manifest, plan: removalPlan(current.Group, selected, audit.References), commit: commit}, nil
}

// resolveRemovalSelection maps each name, by source name or install name, to
// one recorded skill. Iteration 26 keeps both name spaces disjoint within a
// source, so a name never identifies two skills.
func resolveRemovalSelection(group model.GroupLabel, installed []state.InstalledSkillEntry, names []string) ([]state.InstalledSkillEntry, error) {
	if len(names) == 0 {
		return nil, fmt.Errorf("select at least one skill to remove from %s", group)
	}
	byName := map[string]int{}
	for i, skill := range installed {
		byName[skill.Name] = i
		byName[skill.InstalledName()] = i
	}
	chosen := map[int]bool{}
	for _, name := range names {
		index, ok := byName[name]
		if !ok {
			return nil, fmt.Errorf("skill %q is not recorded for %s", name, group)
		}
		if chosen[index] {
			return nil, fmt.Errorf("skill %q is selected more than once", installed[index].Name)
		}
		chosen[index] = true
	}
	if len(chosen) == len(installed) {
		return nil, fmt.Errorf("cannot remove every skill of %s; uninstall the source instead", group)
	}
	selected := []state.InstalledSkillEntry{}
	for i, skill := range installed {
		if chosen[i] {
			selected = append(selected, skill)
		}
	}
	return selected, nil
}

func removalPlan(group model.GroupLabel, selected []state.InstalledSkillEntry, references []RepositoryReference) SkillRemovalPlan {
	installedNames := map[string]bool{}
	for _, skill := range selected {
		installedNames[skill.InstalledName()] = true
	}
	plan := SkillRemovalPlan{Group: group, Skills: selected, References: []RepositoryReference{}}
	for _, reference := range references {
		if installedNames[reference.InstalledName] {
			plan.References = append(plan.References, reference)
		}
	}
	return plan
}

// apply stages the selected links, saves the reduced source record once, and
// then deletes staging. A failed save moves every staged link back.
func (s *SkillRemovalService) apply(target removalTarget) (SkillRemovalResult, error) {
	plan := target.plan
	result := SkillRemovalResult{Group: plan.Group, Skills: plan.Skills, RemovedActive: []RepositoryReference{}, RemovedDisabled: []RepositoryReference{}, RolledBack: []StagedMove{}}
	if !s.backedUp {
		if _, err := s.backupExisting(); err != nil {
			return result, err
		}
		s.backedUp = true
	}
	if err := ensureTrashRoot(s.paths.TrashDir, s.mkdirAll); err != nil {
		return result, err
	}
	stagingRoot, err := s.mkdirTemp(s.paths.TrashDir, "remove-skill-")
	if err != nil {
		return result, fmt.Errorf("create skill removal staging directory: %w", err)
	}
	stagingRoot = filepath.Clean(stagingRoot)
	if stagingRoot == filepath.Clean(s.paths.TrashDir) || !pathInside(s.paths.TrashDir, stagingRoot) {
		return result, fmt.Errorf("unsafe skill removal staging path %s", stagingRoot)
	}

	moves := []StagedMove{}
	rollback := func(original error) (SkillRemovalResult, error) {
		result.RolledBack = rollbackStagedMoves(moves, s.rename)
		if len(result.RolledBack) != len(moves) {
			result.CleanupPending = stagingRoot
			return result, fmt.Errorf("%w; rollback restored %d of %d staged paths; recovery data retained at %s", original, len(result.RolledBack), len(moves), stagingRoot)
		}
		if cleanupErr := s.removeAll(stagingRoot); cleanupErr != nil {
			result.CleanupPending = stagingRoot
			return result, fmt.Errorf("%w; cleanup staging %s: %v", original, stagingRoot, cleanupErr)
		}
		return result, original
	}

	for _, reference := range plan.References {
		stagedPath := filepath.Join(stagingRoot, "links", reference.State.String(), reference.Tool.String(), reference.InstalledName)
		if err := s.mkdirAll(filepath.Dir(stagedPath), 0o700); err != nil {
			return rollback(fmt.Errorf("create skill removal staging parent: %w", err))
		}
		if err := s.rename(reference.LinkPath, stagedPath); err != nil {
			return rollback(fmt.Errorf("stage managed symlink %s: %w", reference.LinkPath, err))
		}
		moves = append(moves, StagedMove{OriginalPath: reference.LinkPath, StagedPath: stagedPath})
		if reference.State == model.SkillStateOff {
			result.RemovedDisabled = append(result.RemovedDisabled, reference)
		} else {
			result.RemovedActive = append(result.RemovedActive, reference)
		}
	}

	manifest := target.manifest
	for _, reference := range plan.References {
		if reference.State == model.SkillStateOff {
			manifest.Remove(reference.Tool, reference.InstalledName)
		}
	}
	if !target.commit(&manifest) {
		return rollback(fmt.Errorf("source %s disappeared from state", plan.Group))
	}
	if err := s.saveManifest(manifest); err != nil {
		return rollback(fmt.Errorf("save state after staging skill removal: %w", err))
	}
	if err := s.removeAll(stagingRoot); err != nil {
		result.CleanupPending = stagingRoot
		return result, fmt.Errorf("skill removal completed but cleanup remains at %s: %w", stagingRoot, err)
	}
	return result, nil
}

// keptSkills returns installed minus the selected entries, in recorded order.
func keptSkills(installed, selected []state.InstalledSkillEntry) []state.InstalledSkillEntry {
	removed := map[string]bool{}
	for _, skill := range selected {
		removed[skill.Name] = true
	}
	kept := []state.InstalledSkillEntry{}
	for _, skill := range installed {
		if !removed[skill.Name] {
			kept = append(kept, skill)
		}
	}
	return kept
}
