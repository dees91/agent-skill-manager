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
	remove    func(string) error
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
		remove:    os.Remove,
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
	// Snapshot the index before any mutation. restoreTracked rewrites it through
	// `git checkout HEAD` and `git reset HEAD`, so rolling back renames alone
	// would silently discard the user's staged content.
	indexTree, err := s.snapshotIndex(checkoutPath)
	if err != nil {
		return result, err
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
	// Paths that were already absent are never staged, but restoreTracked
	// recreates the ones HEAD holds. Rollback has to remove them again, or a
	// failed repair leaves files behind while reporting a complete rollback.
	absent := []string{}
	rollback := func(original error) (RepairResult, error) {
		result.RolledBack = rollbackStagedMoves(moves, s.rename)
		recreatedErr := s.removeRecreatedPaths(checkoutPath, absent)
		indexErr := s.restoreIndex(checkoutPath, indexTree)
		if len(result.RolledBack) != len(moves) || recreatedErr != nil || indexErr != nil {
			result.CleanupPending = stagingRoot
			details := []string{fmt.Sprintf("rollback restored %d of %d staged paths", len(result.RolledBack), len(moves))}
			if recreatedErr != nil {
				details = append(details, fmt.Sprintf("paths recreated by repair were not removed: %v", recreatedErr))
			}
			if indexErr != nil {
				details = append(details, fmt.Sprintf("the Git index was not restored: %v", indexErr))
			}
			return result, fmt.Errorf("%w; %s; recovery data retained at %s", original, strings.Join(details, "; "), stagingRoot)
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
				absent = append(absent, entry.RelPath)
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
	// Prove every recorded link still resolves before the recovery data goes away.
	if err := s.reauditReferences(audit.Repository); err != nil {
		return rollback(fmt.Errorf("repair broke a recorded reference: %w", err))
	}
	// Only after the checkout verifies clean, so rollback never needs a pruned parent.
	staged := make([]string, 0, len(moves))
	for _, move := range moves {
		staged = append(staged, move.OriginalPath)
	}
	s.pruneEmptyParents(checkoutPath, staged)
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
	_, inspectErr := inspectManagedCheckout(audit.Identity, checkoutPath, s.runner)
	conflict, isConflict := AsCheckoutConflict(inspectErr)
	if inspectErr != nil && (!isConflict || !conflict.Repairable()) {
		return ReferenceAudit{}, CheckoutConflictError{}, inspectErr
	}
	// The branch, upstream, and ancestry blockers are not repairable and must be
	// rejected before any mutation. inspectManagedCheckout reports the worktree
	// first and returns before reaching them, so a dirty checkout would otherwise
	// start staging and only discover them during final verification, or repair
	// would report success while update still fails.
	refs, err := resolveManagedCheckoutRefs(checkoutPath, s.runner)
	if err != nil {
		return ReferenceAudit{}, CheckoutConflictError{}, err
	}
	if err := requireFastForward(refs, refs.UpstreamCommit, s.runner); err != nil {
		return ReferenceAudit{}, CheckoutConflictError{}, err
	}
	if inspectErr == nil {
		return audit, CheckoutConflictError{}, nil
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
	if err := s.requireRestorableSkillFiles(checkoutPath, conflict.Entries, audit.Repository); err != nil {
		return ReferenceAudit{}, CheckoutConflictError{}, err
	}
	return audit, conflict, nil
}

// requireRestorableSkillFiles refuses to stage an installed skill's SKILL.md
// unless HEAD holds it as a regular file that the restore step can put back.
// Tracked status alone is not enough: a staged addition is tracked but absent
// from HEAD, so staging it would leave the recorded symlink dangling.
func (s *RepairService) requireRestorableSkillFiles(checkoutPath string, entries []WorktreeEntry, repository state.RepositoryEntry) error {
	affected := map[string]string{}
	candidates := []string{}
	for _, entry := range entries {
		for _, skill := range repository.InstalledSkills {
			recorded := strings.Trim(filepath.ToSlash(skill.RelativePath), "/")
			if recorded == "" || entry.RelPath != recorded+"/SKILL.md" {
				continue
			}
			affected[entry.RelPath] = skill.Name
			candidates = append(candidates, entry.RelPath)
		}
	}
	if len(candidates) == 0 {
		return nil
	}
	modes, err := s.headBlobModes(checkoutPath, candidates)
	if err != nil {
		return err
	}
	for _, relative := range candidates {
		mode, ok := modes[relative]
		if !ok {
			return fmt.Errorf("repair blocked: %q holds installed skill %q and is missing from HEAD", relative, affected[relative])
		}
		if !regularBlobMode(mode) {
			return fmt.Errorf("repair blocked: %q holds installed skill %q and is not a regular file in HEAD", relative, affected[relative])
		}
	}
	return nil
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
	inHead, err := s.headBlobModes(checkoutPath, tracked)
	if err != nil {
		return err
	}
	restore := []string{}
	unstage := []string{}
	for _, relative := range tracked {
		if _, ok := inHead[relative]; ok {
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

// headBlobModes maps each of the given checkout-relative paths that exists in
// HEAD to its octal mode. The default ls-tree format is used rather than
// --name-only because every record then starts with the mode digits, so the
// trimming GitRunner cannot eat a leading space belonging to a filename.
func (s *RepairService) headBlobModes(checkoutPath string, relativePaths []string) (map[string]string, error) {
	args := append([]string{"-C", checkoutPath, "ls-tree", "-r", "-z", "HEAD", "--"}, relativePaths...)
	output, err := s.runner.RunGit(args...)
	if err != nil {
		return nil, fmt.Errorf("resolve HEAD paths in %s: %w", checkoutPath, err)
	}
	modes := map[string]string{}
	for _, record := range strings.Split(output, "\x00") {
		// <mode> SP <type> SP <object> TAB <path>
		tab := strings.IndexByte(record, '\t')
		if tab < 0 {
			continue
		}
		fields := strings.Fields(record[:tab])
		path := record[tab+1:]
		if len(fields) != 3 || path == "" {
			continue
		}
		modes[path] = fields[0]
	}
	return modes, nil
}

// regularBlobMode reports whether a HEAD mode is a regular file, as opposed to
// a symlink, gitlink, or directory.
func regularBlobMode(mode string) bool {
	return mode == "100644" || mode == "100755"
}

// removeRecreatedPaths deletes the files restoreTracked recreated at paths that
// were absent before the repair started, returning the worktree to its exact
// pre-operation shape. Only non-directory paths inside the checkout are removed.
func (s *RepairService) removeRecreatedPaths(checkoutPath string, relativePaths []string) error {
	root := filepath.Clean(checkoutPath)
	removed := []string{}
	for _, relative := range relativePaths {
		target := filepath.Join(root, filepath.FromSlash(relative))
		if !pathInside(root, target) {
			continue
		}
		info, err := s.lstat(target)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("inspect recreated path %s: %w", relative, err)
		}
		if info.IsDir() {
			return fmt.Errorf("recreated path %s is a directory", relative)
		}
		if err := s.remove(target); err != nil {
			return fmt.Errorf("remove recreated path %s: %w", relative, err)
		}
		removed = append(removed, target)
	}
	s.pruneEmptyParents(root, removed)
	return nil
}

// pruneEmptyParents removes directories left empty by staging. os.Remove only
// succeeds on an empty directory, and the walk never reaches the checkout root.
func (s *RepairService) pruneEmptyParents(checkoutPath string, paths []string) {
	root := filepath.Clean(checkoutPath)
	for i := len(paths) - 1; i >= 0; i-- {
		directory := filepath.Dir(paths[i])
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

// snapshotIndex records the current index as a tree object so a failed repair
// can put the index back exactly as the user left it.
func (s *RepairService) snapshotIndex(checkoutPath string) (string, error) {
	tree, err := s.runner.RunGit("-C", checkoutPath, "write-tree")
	if err != nil {
		return "", fmt.Errorf("snapshot Git index for %s: %w", checkoutPath, err)
	}
	tree = strings.TrimSpace(tree)
	if tree == "" {
		return "", fmt.Errorf("snapshot Git index for %s: empty tree", checkoutPath)
	}
	return tree, nil
}

// restoreIndex puts the index back to a snapshot without touching the worktree.
func (s *RepairService) restoreIndex(checkoutPath, tree string) error {
	if tree == "" {
		return nil
	}
	if _, err := s.runner.RunGit("-C", checkoutPath, "read-tree", tree); err != nil {
		return fmt.Errorf("restore Git index for %s: %w", checkoutPath, err)
	}
	return nil
}

// reauditReferences re-runs the ownership audit against fresh state.
func (s *RepairService) reauditReferences(repository state.RepositoryEntry) error {
	manifest, err := s.store.Load()
	if err != nil {
		return err
	}
	current, ok := manifest.GetRepository(repository.Host, repository.RepoPath)
	if !ok {
		return fmt.Errorf("managed repository %s/%s not found in state", repository.Host, repository.RepoPath)
	}
	_, err = AuditRepositoryReferences(s.paths, manifest, current)
	return err
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
