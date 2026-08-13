package install

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dees91/agent-skill-manager/internal/model"
	"github.com/dees91/agent-skill-manager/internal/paths"
	"github.com/dees91/agent-skill-manager/internal/state"
)

// LocalUninstallPlan is a validated whole-local-source removal plan.
type LocalUninstallPlan struct {
	Source     state.LocalSourceEntry
	References LocalReferenceAudit
}

// LocalUninstallResult describes logical removal and any recovery residue.
type LocalUninstallResult struct {
	Source          state.LocalSourceEntry
	RemovedActive   []RepositoryReference
	RemovedDisabled []RepositoryReference
	RolledBack      []StagedMove
	CleanupPending  string
}

// LocalUninstallService removes links and state without touching local source files.
type LocalUninstallService struct {
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

// NewLocalUninstallService creates a local-source uninstall service.
func NewLocalUninstallService(p paths.Paths) *LocalUninstallService {
	store := state.New(p)
	return &LocalUninstallService{
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

// Plan validates uninstall without filesystem or manifest mutation.
func (s *LocalUninstallService) Plan(source state.LocalSourceEntry) (LocalUninstallPlan, error) {
	_, current, audit, err := s.prepare(source)
	if err != nil {
		return LocalUninstallPlan{}, err
	}
	return LocalUninstallPlan{Source: current, References: audit}, nil
}

// Apply stages exact links, saves reduced state, then removes staging.
func (s *LocalUninstallService) Apply(source state.LocalSourceEntry) (LocalUninstallResult, error) {
	result := LocalUninstallResult{Source: source, RemovedActive: []RepositoryReference{}, RemovedDisabled: []RepositoryReference{}, RolledBack: []StagedMove{}}
	manifest, current, audit, err := s.prepare(source)
	if err != nil {
		return result, err
	}
	result.Source = current
	if !s.backedUp {
		if _, err := s.backupExisting(); err != nil {
			return result, err
		}
		s.backedUp = true
	}
	if err := ensureTrashRoot(s.paths.TrashDir, s.mkdirAll); err != nil {
		return result, err
	}
	stagingRoot, err := s.mkdirTemp(s.paths.TrashDir, "uninstall-local-")
	if err != nil {
		return result, fmt.Errorf("create local uninstall staging directory: %w", err)
	}
	stagingRoot = filepath.Clean(stagingRoot)
	if stagingRoot == filepath.Clean(s.paths.TrashDir) || !pathInside(s.paths.TrashDir, stagingRoot) {
		return result, fmt.Errorf("unsafe local uninstall staging path %s", stagingRoot)
	}

	moves := []StagedMove{}
	rollback := func(original error) (LocalUninstallResult, error) {
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

	for _, reference := range audit.References {
		stagedPath := filepath.Join(stagingRoot, "links", reference.State.String(), reference.Tool.String(), reference.SkillName)
		if err := s.mkdirAll(filepath.Dir(stagedPath), 0o755); err != nil {
			return rollback(fmt.Errorf("create local uninstall staging parent: %w", err))
		}
		if err := s.rename(reference.LinkPath, stagedPath); err != nil {
			return rollback(fmt.Errorf("stage local source symlink %s: %w", reference.LinkPath, err))
		}
		moves = append(moves, StagedMove{OriginalPath: reference.LinkPath, StagedPath: stagedPath})
		if reference.State == model.SkillStateOff {
			result.RemovedDisabled = append(result.RemovedDisabled, reference)
		} else {
			result.RemovedActive = append(result.RemovedActive, reference)
		}
	}
	for _, reference := range audit.References {
		if reference.State == model.SkillStateOff {
			manifest.Remove(reference.Tool, reference.SkillName)
		}
	}
	if !manifest.RemoveLocalSource(current.CanonicalPath) {
		return rollback(fmt.Errorf("local source %s disappeared from state", current.CanonicalPath))
	}
	if err := s.saveManifest(manifest); err != nil {
		return rollback(fmt.Errorf("save state after staging local uninstall: %w", err))
	}
	if err := s.removeAll(stagingRoot); err != nil {
		result.CleanupPending = stagingRoot
		return result, fmt.Errorf("local uninstall completed but cleanup remains at %s: %w", stagingRoot, err)
	}
	return result, nil
}

func (s *LocalUninstallService) prepare(source state.LocalSourceEntry) (state.Manifest, state.LocalSourceEntry, LocalReferenceAudit, error) {
	manifest, err := s.store.Load()
	if err != nil {
		return state.Manifest{}, state.LocalSourceEntry{}, LocalReferenceAudit{}, err
	}
	current, ok := manifest.GetLocalSource(source.CanonicalPath)
	if !ok {
		return state.Manifest{}, state.LocalSourceEntry{}, LocalReferenceAudit{}, fmt.Errorf("local source %s not found in state", source.CanonicalPath)
	}
	audit, err := AuditLocalSourceReferences(s.paths, manifest, current, false)
	if err != nil {
		return state.Manifest{}, state.LocalSourceEntry{}, LocalReferenceAudit{}, err
	}
	return manifest, current, audit, nil
}
