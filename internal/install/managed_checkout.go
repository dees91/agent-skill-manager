package install

import (
	"fmt"
	"os"
	"strings"
)

// ManagedCheckout describes the clean checked-out branch used by update and uninstall.
type ManagedCheckout struct {
	Path           string
	Branch         string
	Upstream       string
	HeadCommit     string
	UpstreamCommit string
}

func inspectManagedCheckout(identity RepoIdentity, checkoutPath string, runner GitRunner) (ManagedCheckout, error) {
	info, err := os.Lstat(checkoutPath)
	if err != nil {
		return ManagedCheckout{}, fmt.Errorf("inspect managed checkout %s: %w", checkoutPath, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return ManagedCheckout{}, conflictf(CheckoutConflictSymlink, checkoutPath, "checkout conflict: %s is a symlink", checkoutPath)
	}
	if !info.IsDir() {
		return ManagedCheckout{}, conflictf(CheckoutConflictNotDirectory, checkoutPath, "checkout conflict: %s is not a directory", checkoutPath)
	}
	service := NewCheckoutService(runner)
	if err := service.validateExistingCheckout(identity, checkoutPath); err != nil {
		return ManagedCheckout{}, err
	}
	status, err := runner.RunGit("-C", checkoutPath, "status", "--porcelain", "--untracked-files=all", "--ignored")
	if err != nil {
		return ManagedCheckout{}, fmt.Errorf("inspect checkout status for %s: %w", checkoutPath, err)
	}
	if strings.TrimSpace(status) != "" {
		return ManagedCheckout{}, dirtyWorktreeConflict(checkoutPath, runner)
	}
	return resolveManagedCheckoutRefs(checkoutPath, runner)
}

// resolveManagedCheckoutRefs validates branch, upstream, and commit resolution
// without inspecting the worktree. Repair needs these blockers on a dirty
// checkout, where inspectManagedCheckout reports the worktree first and returns
// before reaching them.
func resolveManagedCheckoutRefs(checkoutPath string, runner GitRunner) (ManagedCheckout, error) {
	branch, err := runner.RunGit("-C", checkoutPath, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil || strings.TrimSpace(branch) == "" {
		return ManagedCheckout{}, conflictf(CheckoutConflictDetachedHead, checkoutPath, "checkout conflict: %s is in detached HEAD state", checkoutPath)
	}
	upstream, err := runner.RunGit("-C", checkoutPath, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")
	if err != nil || strings.TrimSpace(upstream) == "" {
		return ManagedCheckout{}, conflictf(CheckoutConflictNoUpstream, checkoutPath, "checkout conflict: branch %s has no upstream", strings.TrimSpace(branch))
	}
	upstream = strings.TrimSpace(upstream)
	if !strings.HasPrefix(upstream, "origin/") || strings.TrimPrefix(upstream, "origin/") == "" {
		return ManagedCheckout{}, conflictf(CheckoutConflictForeignUpstream, checkoutPath, "checkout conflict: branch %s tracks non-origin upstream %s", strings.TrimSpace(branch), upstream)
	}
	head, err := runner.RunGit("-C", checkoutPath, "rev-parse", "HEAD")
	if err != nil || strings.TrimSpace(head) == "" {
		return ManagedCheckout{}, conflictf(CheckoutConflictUnresolvedRef, checkoutPath, "checkout conflict: cannot resolve HEAD for %s", checkoutPath)
	}
	upstreamCommit, err := runner.RunGit("-C", checkoutPath, "rev-parse", upstream)
	if err != nil || strings.TrimSpace(upstreamCommit) == "" {
		return ManagedCheckout{}, conflictf(CheckoutConflictUnresolvedRef, checkoutPath, "checkout conflict: cannot resolve upstream %s", upstream)
	}
	return ManagedCheckout{
		Path:           checkoutPath,
		Branch:         strings.TrimSpace(branch),
		Upstream:       upstream,
		HeadCommit:     strings.TrimSpace(head),
		UpstreamCommit: strings.TrimSpace(upstreamCommit),
	}, nil
}

func requireFastForward(checkout ManagedCheckout, targetCommit string, runner GitRunner) error {
	targetCommit = strings.TrimSpace(targetCommit)
	if targetCommit == "" {
		return conflictf(CheckoutConflictUnresolvedRef, checkout.Path, "checkout conflict: missing target commit for %s", checkout.Path)
	}
	if _, err := runner.RunGit("-C", checkout.Path, "merge-base", "--is-ancestor", checkout.HeadCommit, targetCommit); err != nil {
		return conflictf(CheckoutConflictDiverged, checkout.Path, "checkout conflict: %s cannot fast-forward from %s to %s", checkout.Path, checkout.HeadCommit, targetCommit)
	}
	return nil
}
