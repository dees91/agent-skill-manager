package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dees91/agent-skill-manager/internal/paths"
	"github.com/dees91/agent-skill-manager/internal/state"
)

// RepairPlan is a side-effect-free description of the worktree paths that block
// a managed checkout. An empty Entries slice means there is nothing to repair.
type RepairPlan struct {
	Repository   state.RepositoryEntry
	CheckoutPath string
	Entries      []WorktreeEntry
	Counts       map[string]int
	Clean        bool
}

// RepairResult describes a completed repair and any staging residue.
type RepairResult struct {
	Repository     state.RepositoryEntry
	CheckoutPath   string
	StagedEntries  []WorktreeEntry
	RestoredPaths  []string
	RolledBack     []StagedMove
	CleanupPending string
	Clean          bool
}

// RepairService clears a dirty Skill Manager-managed checkout by staging every
// offending path through the trash directory and restoring tracked paths from
// HEAD. It never touches state, symlinks, remote refs, or the local source of a
// link-in-place installation.
type RepairService struct {
	paths     paths.Paths
	store     state.Store
	runner    GitRunner
	rename    func(string, string) error
	removeAll func(string) error
	mkdirAll  func(string, os.FileMode) error
	mkdirTemp func(string, string) (string, error)
	lstat     func(string) (os.FileInfo, error)
}

// NewRepairService creates a repair service. A nil runner uses real git.
func NewRepairService(p paths.Paths, runner GitRunner) *RepairService {
	if runner == nil {
		runner = ExecGitRunner{}
	}
	return &RepairService{
		paths:     p,
		store:     state.New(p),
		runner:    runner,
		rename:    os.Rename,
		removeAll: os.RemoveAll,
		mkdirAll:  os.MkdirAll,
		mkdirTemp: os.MkdirTemp,
		lstat:     os.Lstat,
	}
}

// Plan validates ownership and reports the offending worktree paths without
// mutating the filesystem, Git, or state.
func (s *RepairService) Plan(repository state.RepositoryEntry) (RepairPlan, error) {
	audit, conflict, err := s.prepare(repository)
	if err != nil {
		return RepairPlan{}, err
	}
	plan := RepairPlan{
		Repository:   audit.Repository,
		CheckoutPath: audit.Repository.CheckoutPath,
		Entries:      conflict.Entries,
		Counts:       CountWorktreeEntries(conflict.Entries),
		Clean:        conflict.Kind == "",
	}
	if plan.Entries == nil {
		plan.Entries = []WorktreeEntry{}
	}
	return plan, nil
}

// Apply stages every offending worktree path into the trash directory, restores
// tracked paths from HEAD, verifies the checkout, then deletes staging.
func (s *RepairService) Apply(repository state.RepositoryEntry) (RepairResult, error) {
	result := RepairResult{Repository: repository, StagedEntries: []WorktreeEntry{}, RestoredPaths: []string{}, RolledBack: []StagedMove{}}
	audit, conflict, err := s.prepare(repository)
	if err != nil {
		return result, err
	}
	checkoutPath := audit.Repository.CheckoutPath
	result.Repository = audit.Repository
	result.CheckoutPath = checkoutPath
	if conflict.Kind == "" {
		result.Clean = true
		return result, nil
	}
	if err := s.ensureTrashRoot(); err != nil {
		return result, err
	}
	stagingRoot, err := s.mkdirTemp(s.paths.TrashDir, "repair-")
	if err != nil {
		return result, fmt.Errorf("create repair staging directory: %w", err)
	}
	stagingRoot = filepath.Clean(stagingRoot)
	if stagingRoot == filepath.Clean(s.paths.TrashDir) || !pathInside(s.paths.TrashDir, stagingRoot) {
		return result, fmt.Errorf("unsafe repair staging path %s", stagingRoot)
	}

	moves := []StagedMove{}
	rollback := func(original error) (RepairResult, error) {
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

	for _, entry := range conflict.Entries {
		originalPath := filepath.Join(checkoutPath, filepath.FromSlash(entry.RelPath))
		if _, err := s.lstat(originalPath); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return rollback(fmt.Errorf("inspect worktree path %s: %w", entry.RelPath, err))
		}
		stagedPath := filepath.Join(stagingRoot, "worktree", filepath.FromSlash(entry.RelPath))
		if err := s.mkdirAll(filepath.Dir(stagedPath), 0o700); err != nil {
			return rollback(fmt.Errorf("create repair staging parent for %s: %w", entry.RelPath, err))
		}
		if err := s.rename(originalPath, stagedPath); err != nil {
			return rollback(fmt.Errorf("stage worktree path %s: %w", entry.RelPath, err))
		}
		moves = append(moves, StagedMove{OriginalPath: originalPath, StagedPath: stagedPath})
		result.StagedEntries = append(result.StagedEntries, entry)
	}

	if err := s.restoreTracked(checkoutPath, conflict.Entries, &result); err != nil {
		return rollback(err)
	}
	if _, err := inspectManagedCheckout(audit.Identity, checkoutPath, s.runner); err != nil {
		return rollback(fmt.Errorf("repair did not clear the checkout: %w", err))
	}
	// Only after the checkout verifies clean, so rollback never needs a pruned parent.
	s.pruneEmptyParents(checkoutPath, moves)
	if err := s.removeAll(stagingRoot); err != nil {
		result.CleanupPending = stagingRoot
		return result, fmt.Errorf("repair completed but cleanup remains at %s: %w", stagingRoot, err)
	}
	result.Clean = true
	return result, nil
}

// prepare runs the shared ownership audit and returns the dirty-worktree
// conflict to repair. A zero-value conflict means the checkout is already
// usable. Every other blocker is returned as an error and never repaired.
func (s *RepairService) prepare(repository state.RepositoryEntry) (ReferenceAudit, CheckoutConflictError, error) {
	manifest, err := s.store.Load()
	if err != nil {
		return ReferenceAudit{}, CheckoutConflictError{}, err
	}
	current, ok := manifest.GetRepository(repository.Host, repository.RepoPath)
	if !ok {
		return ReferenceAudit{}, CheckoutConflictError{}, fmt.Errorf("managed repository %s/%s not found in state", repository.Host, repository.RepoPath)
	}
	audit, err := AuditRepositoryReferences(s.paths, manifest, current)
	if err != nil {
		return ReferenceAudit{}, CheckoutConflictError{}, err
	}
	checkoutPath := audit.Repository.CheckoutPath
	if !pathInside(s.paths.ReposDir, checkoutPath) || filepath.Clean(checkoutPath) == filepath.Clean(s.paths.ReposDir) {
		return ReferenceAudit{}, CheckoutConflictError{}, fmt.Errorf("unsafe repair checkout path %s", checkoutPath)
	}
	checkout, inspectErr := inspectManagedCheckout(audit.Identity, checkoutPath, s.runner)
	if inspectErr == nil {
		// A clean worktree still has to be usable; local commits are not repairable.
		if err := requireFastForward(checkout, checkout.UpstreamCommit, s.runner); err != nil {
			return ReferenceAudit{}, CheckoutConflictError{}, err
		}
		return audit, CheckoutConflictError{}, nil
	}
	conflict, ok := AsCheckoutConflict(inspectErr)
	if !ok || !conflict.Repairable() {
		return ReferenceAudit{}, CheckoutConflictError{}, inspectErr
	}
	if len(conflict.Entries) == 0 {
		return ReferenceAudit{}, CheckoutConflictError{}, fmt.Errorf("repair blocked: %s has worktree changes git could not enumerate", checkoutPath)
	}
	for _, entry := range conflict.Entries {
		if err := validateWorktreeEntry(checkoutPath, entry); err != nil {
			return ReferenceAudit{}, CheckoutConflictError{}, err
		}
		if skill, covered := entryCoversInstalledSkill(entry, audit.Repository); covered {
			return ReferenceAudit{}, CheckoutConflictError{}, fmt.Errorf("repair blocked: worktree path %q holds installed skill %q", entry.RelPath, skill)
		}
	}
	return audit, conflict, nil
}

// restoreTracked returns tracked paths to their HEAD content. Paths that exist
// only in the index, such as staged additions and the new name of a staged
// rename, have no HEAD blob and are only removed from the index.
func (s *RepairService) restoreTracked(checkoutPath string, entries []WorktreeEntry, result *RepairResult) error {
	tracked := []string{}
	for _, entry := range entries {
		if entry.Class == WorktreeEntryTracked {
			tracked = append(tracked, entry.RelPath)
		}
	}
	if len(tracked) == 0 {
		return nil
	}
	inHead, err := s.pathsInHead(checkoutPath, tracked)
	if err != nil {
		return err
	}
	restore := []string{}
	unstage := []string{}
	for _, relative := range tracked {
		if inHead[relative] {
			restore = append(restore, relative)
			continue
		}
		unstage = append(unstage, relative)
	}
	if len(unstage) > 0 {
		args := append([]string{"-C", checkoutPath, "reset", "--quiet", "HEAD", "--"}, unstage...)
		if _, err := s.runner.RunGit(args...); err != nil {
			return fmt.Errorf("unstage added paths in %s: %w", checkoutPath, err)
		}
		result.RestoredPaths = append(result.RestoredPaths, unstage...)
	}
	if len(restore) > 0 {
		args := append([]string{"-C", checkoutPath, "checkout", "--quiet", "HEAD", "--"}, restore...)
		if _, err := s.runner.RunGit(args...); err != nil {
			return fmt.Errorf("restore tracked paths in %s: %w", checkoutPath, err)
		}
		result.RestoredPaths = append(result.RestoredPaths, restore...)
	}
	return nil
}

// pathsInHead reports which of the given checkout-relative paths exist in HEAD.
func (s *RepairService) pathsInHead(checkoutPath string, relativePaths []string) (map[string]bool, error) {
	args := append([]string{"-C", checkoutPath, "ls-tree", "-r", "-z", "--name-only", "HEAD", "--"}, relativePaths...)
	output, err := s.runner.RunGit(args...)
	if err != nil {
		return nil, fmt.Errorf("resolve HEAD paths in %s: %w", checkoutPath, err)
	}
	present := map[string]bool{}
	for _, name := range strings.Split(output, "\x00") {
		name = strings.TrimSpace(name)
		if name != "" {
			present[name] = true
		}
	}
	return present, nil
}

// pruneEmptyParents removes directories left empty by staging. os.Remove only
// succeeds on an empty directory, and the walk never reaches the checkout root.
func (s *RepairService) pruneEmptyParents(checkoutPath string, moves []StagedMove) {
	root := filepath.Clean(checkoutPath)
	for i := len(moves) - 1; i >= 0; i-- {
		directory := filepath.Dir(moves[i].OriginalPath)
		for {
			directory = filepath.Clean(directory)
			if directory == root || !pathInside(root, directory) {
				break
			}
			if err := os.Remove(directory); err != nil {
				break
			}
			directory = filepath.Dir(directory)
		}
	}
}

func (s *RepairService) ensureTrashRoot() error {
	return ensureTrashRoot(s.paths.TrashDir, s.mkdirAll)
}

// entryCoversInstalledSkill reports whether staging the entry would break an
// installed skill: moving its directory out from under the managed symlink, or
// removing an untracked SKILL.md that HEAD cannot restore. A stray file inside a
// skill directory, or a tracked file that HEAD restores, is fine.
func entryCoversInstalledSkill(entry WorktreeEntry, repository state.RepositoryEntry) (string, bool) {
	candidate := strings.Trim(entry.RelPath, "/")
	for _, skill := range repository.InstalledSkills {
		recorded := strings.Trim(filepath.ToSlash(skill.RelativePath), "/")
		if recorded == "" {
			continue
		}
		if recorded == candidate || strings.HasPrefix(recorded, candidate+"/") {
			return skill.Name, true
		}
		if entry.Class != WorktreeEntryTracked && candidate == recorded+"/SKILL.md" {
			return skill.Name, true
		}
	}
	return "", false
}

// validateWorktreeEntry rejects any reported path that could escape the managed
// checkout or reach Git's own metadata.
func validateWorktreeEntry(checkoutPath string, entry WorktreeEntry) error {
	relative := entry.RelPath
	if strings.TrimSpace(relative) == "" {
		return fmt.Errorf("repair blocked: empty worktree path in %s", checkoutPath)
	}
	if strings.Contains(relative, "\\") {
		return fmt.Errorf("repair blocked: unsupported worktree path %q", relative)
	}
	if filepath.IsAbs(relative) || strings.HasPrefix(relative, "/") {
		return fmt.Errorf("repair blocked: absolute worktree path %q", relative)
	}
	for _, segment := range strings.Split(relative, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return fmt.Errorf("repair blocked: unsafe worktree path %q", relative)
		}
		if segment == ".git" {
			return fmt.Errorf("repair blocked: worktree path %q is inside git metadata", relative)
		}
	}
	target := filepath.Join(checkoutPath, filepath.FromSlash(relative))
	if !pathInside(checkoutPath, target) || filepath.Clean(target) == filepath.Clean(checkoutPath) {
		return fmt.Errorf("repair blocked: worktree path %q escapes %s", relative, checkoutPath)
	}
	return nil
}
