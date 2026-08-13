package install

import (
	"fmt"
	"strings"

	"github.com/dees91/agent-skill-manager/internal/paths"
	"github.com/dees91/agent-skill-manager/internal/state"
)

// UpdatePlan describes a locally validated repository update. Dry-run plans do
// not fetch and therefore expose only the currently cached upstream commit.
type UpdatePlan struct {
	Repository state.RepositoryEntry
	Checkout   ManagedCheckout
	References ReferenceAudit
}

// UpdateResult describes one real repository update attempt.
type UpdateResult struct {
	Repository     state.RepositoryEntry
	PreviousCommit string
	CurrentCommit  string
	Updated        bool
	UpToDate       bool
}

// UpdateService validates and fast-forwards managed repository checkouts.
type UpdateService struct {
	paths    paths.Paths
	store    state.Store
	runner   GitRunner
	backedUp bool
}

// NewUpdateService creates an update service. A nil runner uses the real git binary.
func NewUpdateService(p paths.Paths, runner GitRunner) *UpdateService {
	if runner == nil {
		runner = ExecGitRunner{}
	}
	return &UpdateService{paths: p, store: state.New(p), runner: runner}
}

// PlanLocal validates a repository without fetching or mutating Git/state.
func (s *UpdateService) PlanLocal(repository state.RepositoryEntry) (UpdatePlan, error) {
	manifest, err := s.store.Load()
	if err != nil {
		return UpdatePlan{}, err
	}
	current, ok := manifest.GetRepository(repository.Host, repository.RepoPath)
	if !ok {
		return UpdatePlan{}, fmt.Errorf("managed repository %s/%s not found in state", repository.Host, repository.RepoPath)
	}
	audit, err := AuditRepositoryReferences(s.paths, manifest, current)
	if err != nil {
		return UpdatePlan{}, err
	}
	checkout, err := inspectManagedCheckout(audit.Identity, audit.Repository.CheckoutPath, s.runner)
	if err != nil {
		return UpdatePlan{}, err
	}
	if err := requireFastForward(checkout, checkout.UpstreamCommit, s.runner); err != nil {
		return UpdatePlan{}, err
	}
	return UpdatePlan{Repository: current, Checkout: checkout, References: audit}, nil
}

// Apply fetches, preflights installed skill paths, fast-forwards the checkout,
// and persists the successful commit.
func (s *UpdateService) Apply(repository state.RepositoryEntry) (UpdateResult, error) {
	result := UpdateResult{Repository: repository}
	manifest, err := s.store.Load()
	if err != nil {
		return result, err
	}
	current, ok := manifest.GetRepository(repository.Host, repository.RepoPath)
	if !ok {
		return result, fmt.Errorf("managed repository %s/%s not found in state", repository.Host, repository.RepoPath)
	}
	result.Repository = current
	audit, err := AuditRepositoryReferences(s.paths, manifest, current)
	if err != nil {
		return result, err
	}
	checkout, err := inspectManagedCheckout(audit.Identity, audit.Repository.CheckoutPath, s.runner)
	if err != nil {
		return result, err
	}
	result.PreviousCommit = checkout.HeadCommit

	if !s.backedUp {
		if _, err := s.store.BackupExisting(); err != nil {
			return result, err
		}
		s.backedUp = true
	}
	if _, err := s.runner.RunGit("-C", checkout.Path, "fetch", "--prune", "origin"); err != nil {
		return result, fmt.Errorf("fetch origin for %s: %w", checkout.Path, err)
	}
	targetCommit, err := s.runner.RunGit("-C", checkout.Path, "rev-parse", checkout.Upstream)
	if err != nil || strings.TrimSpace(targetCommit) == "" {
		return result, fmt.Errorf("resolve fetched upstream %s for %s", checkout.Upstream, checkout.Path)
	}
	targetCommit = strings.TrimSpace(targetCommit)
	if err := requireFastForward(checkout, targetCommit, s.runner); err != nil {
		return result, err
	}
	if err := preflightInstalledSkillsAtCommit(checkout.Path, targetCommit, current.InstalledSkills, s.runner); err != nil {
		return result, err
	}

	if targetCommit != checkout.HeadCommit {
		if _, err := s.runner.RunGit("-C", checkout.Path, "merge", "--ff-only", targetCommit); err != nil {
			return result, fmt.Errorf("fast-forward %s to %s: %w", checkout.Path, targetCommit, err)
		}
		result.Updated = true
	} else {
		result.UpToDate = true
	}
	result.CurrentCommit = targetCommit

	current.LastSeenCommit = targetCommit
	manifest.UpsertRepository(current)
	if err := s.store.Save(manifest); err != nil {
		if result.Updated {
			return result, fmt.Errorf("checkout updated to %s but state save failed: %w", targetCommit, err)
		}
		return result, err
	}
	result.Repository = current
	return result, nil
}

func preflightInstalledSkillsAtCommit(checkoutPath, targetCommit string, installed []state.InstalledSkillEntry, runner GitRunner) error {
	missing := []string{}
	for _, skill := range installed {
		_, relativePath, err := normalizeRecordedSkill(checkoutPath, skill)
		if err != nil {
			return err
		}
		skillFile := "SKILL.md"
		if relativePath != "." {
			skillFile = relativePath + "/SKILL.md"
		}
		output, err := runner.RunGit("-C", checkoutPath, "ls-tree", targetCommit, "--", skillFile)
		if err != nil || !regularGitFile(strings.TrimSpace(output), skillFile) {
			missing = append(missing, fmt.Sprintf("%s (%s)", skill.Name, skillFile))
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("update blocked: installed skills missing regular SKILL.md at target commit: %s", strings.Join(missing, ", "))
	}
	return nil
}

func regularGitFile(output, expectedPath string) bool {
	if output == "" || strings.Contains(output, "\n") {
		return false
	}
	metadata, pathText, ok := strings.Cut(output, "\t")
	if !ok || pathText != expectedPath {
		return false
	}
	fields := strings.Fields(metadata)
	if len(fields) != 3 || fields[1] != "blob" {
		return false
	}
	return fields[0] == "100644" || fields[0] == "100755"
}
