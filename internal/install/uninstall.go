package install

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dees91/agent-skill-manager/internal/model"
	"github.com/dees91/agent-skill-manager/internal/paths"
	"github.com/dees91/agent-skill-manager/internal/state"
)

// UninstallPlan is a side-effect-free, validated whole-repository removal plan.
type UninstallPlan struct {
	Repository state.RepositoryEntry
	Checkout   ManagedCheckout
	References ReferenceAudit
}

// StagedMove records one exact path moved during transactional uninstall.
type StagedMove struct {
	OriginalPath string
	StagedPath   string
}

// UninstallResult describes completed logical removal and cleanup state.
type UninstallResult struct {
	Repository      state.RepositoryEntry
	RemovedActive   []RepositoryReference
	RemovedDisabled []RepositoryReference
	RemovedCheckout string
	RolledBack      []StagedMove
	CleanupPending  string
}

// UninstallService removes a complete Skill Manager-owned repository install.
type UninstallService struct {
	paths          paths.Paths
	store          state.Store
	runner         GitRunner
	backedUp       bool
	rename         func(string, string) error
	removeAll      func(string) error
	mkdirAll       func(string, os.FileMode) error
	mkdirTemp      func(string, string) (string, error)
	saveManifest   func(state.Manifest) error
	backupExisting func() (string, error)
}

// NewUninstallService creates an uninstall service. A nil runner uses real git.
func NewUninstallService(p paths.Paths, runner GitRunner) *UninstallService {
	if runner == nil {
		runner = ExecGitRunner{}
	}
	store := state.New(p)
	return &UninstallService{
		paths:          p,
		store:          store,
		runner:         runner,
		rename:         os.Rename,
		removeAll:      os.RemoveAll,
		mkdirAll:       os.MkdirAll,
		mkdirTemp:      os.MkdirTemp,
		saveManifest:   store.Save,
		backupExisting: store.BackupExisting,
	}
}

// Plan validates an uninstall without mutating filesystem, Git, or state.
func (s *UninstallService) Plan(repository state.RepositoryEntry) (UninstallPlan, error) {
	_, current, audit, checkout, err := s.prepare(repository)
	if err != nil {
		return UninstallPlan{}, err
	}
	return UninstallPlan{Repository: current, Checkout: checkout, References: audit}, nil
}

// Apply stages exact owned paths, removes state, then deletes staging.
func (s *UninstallService) Apply(repository state.RepositoryEntry) (UninstallResult, error) {
	result := UninstallResult{Repository: repository, RemovedActive: []RepositoryReference{}, RemovedDisabled: []RepositoryReference{}, RolledBack: []StagedMove{}}
	manifest, current, audit, checkout, err := s.prepare(repository)
	if err != nil {
		return result, err
	}
	result.Repository = current
	if !s.backedUp {
		if _, err := s.backupExisting(); err != nil {
			return result, err
		}
		s.backedUp = true
	}
	if err := s.ensureTrashRoot(); err != nil {
		return result, err
	}
	stagingRoot, err := s.mkdirTemp(s.paths.TrashDir, "uninstall-")
	if err != nil {
		return result, fmt.Errorf("create uninstall staging directory: %w", err)
	}
	stagingRoot = filepath.Clean(stagingRoot)
	if stagingRoot == filepath.Clean(s.paths.TrashDir) || !pathInside(s.paths.TrashDir, stagingRoot) {
		return result, fmt.Errorf("unsafe uninstall staging path %s", stagingRoot)
	}

	moves := []StagedMove{}
	rollback := func(original error) (UninstallResult, error) {
		result.RolledBack = rollbackStagedMoves(moves, s.rename)
		if len(result.RolledBack) != len(moves) {
			result.CleanupPending = stagingRoot
			original = fmt.Errorf("%w; rollback restored %d of %d staged paths; recovery data retained at %s", original, len(result.RolledBack), len(moves), stagingRoot)
			return result, original
		}
		cleanupErr := s.removeAll(stagingRoot)
		if cleanupErr != nil {
			result.CleanupPending = stagingRoot
			original = fmt.Errorf("%w; cleanup staging %s: %v", original, stagingRoot, cleanupErr)
		}
		return result, original
	}

	for _, reference := range audit.References {
		stagedPath := filepath.Join(stagingRoot, "links", reference.State.String(), reference.Tool.String(), reference.SkillName)
		if err := s.mkdirAll(filepath.Dir(stagedPath), 0o755); err != nil {
			return rollback(fmt.Errorf("create uninstall staging parent: %w", err))
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

	stagedCheckout := filepath.Join(stagingRoot, "checkout")
	if err := s.rename(checkout.Path, stagedCheckout); err != nil {
		return rollback(fmt.Errorf("stage repository checkout %s: %w", checkout.Path, err))
	}
	moves = append(moves, StagedMove{OriginalPath: checkout.Path, StagedPath: stagedCheckout})
	result.RemovedCheckout = checkout.Path

	for _, reference := range audit.References {
		if reference.State == model.SkillStateOff {
			manifest.Remove(reference.Tool, reference.SkillName)
		}
	}
	if !manifest.RemoveRepository(current.Host, current.RepoPath) {
		return rollback(fmt.Errorf("managed repository %s/%s disappeared from state", current.Host, current.RepoPath))
	}
	if err := s.saveManifest(manifest); err != nil {
		return rollback(fmt.Errorf("save state after staging uninstall: %w", err))
	}
	if err := s.removeAll(stagingRoot); err != nil {
		result.CleanupPending = stagingRoot
		return result, fmt.Errorf("uninstall completed but cleanup remains at %s: %w", stagingRoot, err)
	}
	return result, nil
}

func (s *UninstallService) prepare(repository state.RepositoryEntry) (state.Manifest, state.RepositoryEntry, ReferenceAudit, ManagedCheckout, error) {
	manifest, err := s.store.Load()
	if err != nil {
		return state.Manifest{}, state.RepositoryEntry{}, ReferenceAudit{}, ManagedCheckout{}, err
	}
	current, ok := manifest.GetRepository(repository.Host, repository.RepoPath)
	if !ok {
		return state.Manifest{}, state.RepositoryEntry{}, ReferenceAudit{}, ManagedCheckout{}, fmt.Errorf("managed repository %s/%s not found in state", repository.Host, repository.RepoPath)
	}
	audit, err := AuditRepositoryReferences(s.paths, manifest, current)
	if err != nil {
		return state.Manifest{}, state.RepositoryEntry{}, ReferenceAudit{}, ManagedCheckout{}, err
	}
	checkout, err := inspectManagedCheckout(audit.Identity, audit.Repository.CheckoutPath, s.runner)
	if err != nil {
		return state.Manifest{}, state.RepositoryEntry{}, ReferenceAudit{}, ManagedCheckout{}, err
	}
	if err := requireFastForward(checkout, checkout.UpstreamCommit, s.runner); err != nil {
		return state.Manifest{}, state.RepositoryEntry{}, ReferenceAudit{}, ManagedCheckout{}, err
	}
	return manifest, current, audit, checkout, nil
}

func (s *UninstallService) ensureTrashRoot() error {
	return ensureTrashRoot(s.paths.TrashDir, s.mkdirAll)
}

func ensureTrashRoot(trashDir string, mkdirAll func(string, os.FileMode) error) error {
	info, err := os.Lstat(trashDir)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("unsafe trash path %s: expected a real directory", trashDir)
		}
		return nil
	}
	if !os.IsNotExist(err) {
		return fmt.Errorf("inspect trash path %s: %w", trashDir, err)
	}
	if err := mkdirAll(trashDir, 0o755); err != nil {
		return fmt.Errorf("create trash directory %s: %w", trashDir, err)
	}
	return nil
}

func rollbackStagedMoves(moves []StagedMove, rename func(string, string) error) []StagedMove {
	restored := []StagedMove{}
	for i := len(moves) - 1; i >= 0; i-- {
		move := moves[i]
		if err := rename(move.StagedPath, move.OriginalPath); err == nil {
			restored = append(restored, move)
		}
	}
	return restored
}
