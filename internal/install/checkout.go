package install

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GitRunner executes git commands for checkout management.
type GitRunner interface {
	RunGit(args ...string) (string, error)
}

// ExecGitRunner executes git commands through the local git binary.
type ExecGitRunner struct{}

// RunGit runs git with the provided arguments and returns trimmed output.
func (ExecGitRunner) RunGit(args ...string) (string, error) {
	command := exec.Command("git", args...)
	output, err := command.CombinedOutput()
	trimmed := strings.TrimSpace(string(output))
	if err != nil {
		if trimmed == "" {
			return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
		}
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, trimmed)
	}
	return trimmed, nil
}

// CheckoutService clones or reuses managed repository checkouts.
type CheckoutService struct {
	runner GitRunner
}

// NewCheckoutService creates a checkout service. A nil runner uses the real git binary.
func NewCheckoutService(runner GitRunner) CheckoutService {
	if runner == nil {
		runner = ExecGitRunner{}
	}
	return CheckoutService{runner: runner}
}

// CheckoutOptions controls checkout behavior.
type CheckoutOptions struct {
	DryRun bool
}

// CheckoutResult describes the checkout action taken.
type CheckoutResult struct {
	CheckoutPath   string
	Cloned         bool
	Reused         bool
	WouldClone     bool
	LastSeenCommit string
}

// EnsureCheckout clones a missing checkout or verifies an existing matching checkout.
func (s CheckoutService) EnsureCheckout(identity RepoIdentity, checkoutPath string, options CheckoutOptions) (CheckoutResult, error) {
	if err := validateCheckoutIdentity(identity); err != nil {
		return CheckoutResult{}, err
	}
	if strings.TrimSpace(checkoutPath) == "" {
		return CheckoutResult{}, fmt.Errorf("checkout conflict: missing checkout path")
	}
	checkoutPath = filepath.Clean(checkoutPath)
	result := CheckoutResult{CheckoutPath: checkoutPath}

	info, err := os.Lstat(checkoutPath)
	if err != nil {
		if os.IsNotExist(err) {
			if options.DryRun {
				result.WouldClone = true
				return result, nil
			}
			if err := os.MkdirAll(filepath.Dir(checkoutPath), 0o700); err != nil {
				return result, fmt.Errorf("create checkout parent %s: %w", filepath.Dir(checkoutPath), err)
			}
			if err := os.Chmod(filepath.Dir(checkoutPath), 0o700); err != nil {
				return result, fmt.Errorf("secure checkout parent %s: %w", filepath.Dir(checkoutPath), err)
			}
			if _, err := s.runner.RunGit("clone", identity.OriginalURL, checkoutPath); err != nil {
				return result, fmt.Errorf("clone repository %s into %s: %w", identity.OriginalURL, checkoutPath, err)
			}
			result.Cloned = true
			result.LastSeenCommit = s.lastSeenCommit(checkoutPath)
			return result, nil
		}
		return result, fmt.Errorf("inspect checkout path %s: %w", checkoutPath, err)
	}

	if info.Mode()&os.ModeSymlink != 0 {
		return result, fmt.Errorf("checkout conflict: %s is a symlink", checkoutPath)
	}
	if !info.IsDir() {
		return result, fmt.Errorf("checkout conflict: %s exists and is not a directory", checkoutPath)
	}

	if err := s.validateExistingCheckout(identity, checkoutPath); err != nil {
		return result, err
	}
	result.Reused = true
	result.LastSeenCommit = s.lastSeenCommit(checkoutPath)
	return result, nil
}

func validateCheckoutIdentity(identity RepoIdentity) error {
	if strings.TrimSpace(identity.OriginalURL) == "" {
		return fmt.Errorf("checkout conflict: missing repository URL")
	}
	if strings.TrimSpace(identity.CanonicalURL) == "" ||
		strings.TrimSpace(identity.Host) == "" ||
		strings.TrimSpace(identity.RepoPath) == "" {
		return fmt.Errorf("checkout conflict: missing repository identity")
	}
	return nil
}

func (s CheckoutService) validateExistingCheckout(identity RepoIdentity, checkoutPath string) error {
	inside, err := s.runner.RunGit("-C", checkoutPath, "rev-parse", "--is-inside-work-tree")
	if err != nil || strings.TrimSpace(inside) != "true" {
		if err != nil {
			return fmt.Errorf("checkout conflict: %s is not a git checkout: %w", checkoutPath, err)
		}
		return fmt.Errorf("checkout conflict: %s is not a git checkout", checkoutPath)
	}
	topLevel, err := s.runner.RunGit("-C", checkoutPath, "rev-parse", "--show-toplevel")
	if err != nil {
		return fmt.Errorf("checkout conflict: cannot resolve git checkout root for %s: %w", checkoutPath, err)
	}
	topLevelPath, err := equivalentPath(strings.TrimSpace(topLevel))
	if err != nil {
		return fmt.Errorf("checkout conflict: cannot resolve git checkout root for %s: %w", checkoutPath, err)
	}
	checkoutRoot, err := equivalentPath(checkoutPath)
	if err != nil {
		return fmt.Errorf("checkout conflict: cannot resolve checkout path %s: %w", checkoutPath, err)
	}
	if topLevelPath != checkoutRoot {
		return fmt.Errorf("checkout conflict: %s is inside git checkout %s, not the checkout root", checkoutPath, strings.TrimSpace(topLevel))
	}

	origin, err := s.runner.RunGit("-C", checkoutPath, "config", "--get", "remote.origin.url")
	if err != nil || strings.TrimSpace(origin) == "" {
		if err != nil {
			return fmt.Errorf("checkout conflict: cannot read origin remote for %s: %w", checkoutPath, err)
		}
		return fmt.Errorf("checkout conflict: missing origin remote for %s", checkoutPath)
	}
	remoteIdentity, err := NormalizeGitURL(origin)
	if err != nil {
		return fmt.Errorf("checkout conflict: unsupported origin remote %q for %s: %w", origin, checkoutPath, err)
	}
	if remoteIdentity.CanonicalURL != identity.CanonicalURL {
		return fmt.Errorf("checkout conflict: origin remote %s does not match requested %s", remoteIdentity.CanonicalURL, identity.CanonicalURL)
	}
	return nil
}

func (s CheckoutService) lastSeenCommit(checkoutPath string) string {
	commit, err := s.runner.RunGit("-C", checkoutPath, "rev-parse", "HEAD")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(commit)
}

func equivalentPath(path string) (string, error) {
	absolute, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	evaluated, err := filepath.EvalSymlinks(absolute)
	if err == nil {
		return filepath.Clean(evaluated), nil
	}
	return absolute, nil
}
