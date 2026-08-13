package install

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestEnsureCheckoutClonesMissingCheckout(t *testing.T) {
	identity := mustIdentity(t)
	checkoutPath := filepath.Join(t.TempDir(), "repos", "github.com", "addyosmani", "agent-skills")
	runner := newFakeGitRunner()
	runner.outputs["clone "+identity.OriginalURL+" "+checkoutPath] = ""
	runner.outputs["-C "+checkoutPath+" rev-parse HEAD"] = "abc123\n"

	result, err := NewCheckoutService(runner).EnsureCheckout(identity, checkoutPath, CheckoutOptions{})
	if err != nil {
		t.Fatalf("EnsureCheckout() error = %v", err)
	}
	if !result.Cloned || result.Reused || result.WouldClone {
		t.Fatalf("CheckoutResult flags = %#v, want cloned only", result)
	}
	if result.CheckoutPath != checkoutPath {
		t.Fatalf("CheckoutPath = %q, want %q", result.CheckoutPath, checkoutPath)
	}
	if result.LastSeenCommit != "abc123" {
		t.Fatalf("LastSeenCommit = %q, want abc123", result.LastSeenCommit)
	}
	if _, err := os.Stat(filepath.Dir(checkoutPath)); err != nil {
		t.Fatalf("parent directory was not created: %v", err)
	}
	wantCalls := [][]string{
		{"clone", identity.OriginalURL, checkoutPath},
		{"-C", checkoutPath, "rev-parse", "HEAD"},
	}
	if !reflect.DeepEqual(runner.calls, wantCalls) {
		t.Fatalf("git calls = %#v, want %#v", runner.calls, wantCalls)
	}
	assertNoPull(t, runner.calls)
}

func TestEnsureCheckoutMissingDryRunDoesNotMutateOrRunGit(t *testing.T) {
	identity := mustIdentity(t)
	home := t.TempDir()
	checkoutPath := filepath.Join(home, "repos", "github.com", "addyosmani", "agent-skills")
	runner := newFakeGitRunner()

	result, err := NewCheckoutService(runner).EnsureCheckout(identity, checkoutPath, CheckoutOptions{DryRun: true})
	if err != nil {
		t.Fatalf("EnsureCheckout() error = %v", err)
	}
	if !result.WouldClone || result.Cloned || result.Reused {
		t.Fatalf("CheckoutResult flags = %#v, want would-clone only", result)
	}
	if _, err := os.Stat(filepath.Join(home, "repos")); !os.IsNotExist(err) {
		t.Fatalf("dry-run created checkout parent or unexpected stat error: %v", err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("git calls = %#v, want none", runner.calls)
	}
}

func TestEnsureCheckoutReusesExistingMatchingCheckout(t *testing.T) {
	identity := mustIdentity(t)
	checkoutPath := existingDir(t)
	runner := matchingCheckoutRunner(checkoutPath, "git@github.com:addyosmani/agent-skills.git", "def456\n")

	result, err := NewCheckoutService(runner).EnsureCheckout(identity, checkoutPath, CheckoutOptions{})
	if err != nil {
		t.Fatalf("EnsureCheckout() error = %v", err)
	}
	if !result.Reused || result.Cloned || result.WouldClone {
		t.Fatalf("CheckoutResult flags = %#v, want reused only", result)
	}
	if result.LastSeenCommit != "def456" {
		t.Fatalf("LastSeenCommit = %q, want def456", result.LastSeenCommit)
	}
	wantCalls := [][]string{
		{"-C", checkoutPath, "rev-parse", "--is-inside-work-tree"},
		{"-C", checkoutPath, "rev-parse", "--show-toplevel"},
		{"-C", checkoutPath, "config", "--get", "remote.origin.url"},
		{"-C", checkoutPath, "rev-parse", "HEAD"},
	}
	if !reflect.DeepEqual(runner.calls, wantCalls) {
		t.Fatalf("git calls = %#v, want %#v", runner.calls, wantCalls)
	}
	assertNoPull(t, runner.calls)
}

func TestEnsureCheckoutDryRunValidatesExistingCheckout(t *testing.T) {
	identity := mustIdentity(t)
	checkoutPath := existingDir(t)
	runner := matchingCheckoutRunner(checkoutPath, "https://github.com/addyosmani/agent-skills.git", "def456\n")

	result, err := NewCheckoutService(runner).EnsureCheckout(identity, checkoutPath, CheckoutOptions{DryRun: true})
	if err != nil {
		t.Fatalf("EnsureCheckout() error = %v", err)
	}
	if !result.Reused || result.Cloned || result.WouldClone {
		t.Fatalf("CheckoutResult flags = %#v, want reused only", result)
	}
	if len(runner.calls) != 4 {
		t.Fatalf("git calls = %#v, want existing checkout validation calls", runner.calls)
	}
	assertNoPull(t, runner.calls)
}

func TestEnsureCheckoutRejectsRemoteMismatch(t *testing.T) {
	identity := mustIdentity(t)
	checkoutPath := existingDir(t)
	runner := matchingCheckoutRunner(checkoutPath, "https://github.com/other/repo.git", "def456\n")

	_, err := NewCheckoutService(runner).EnsureCheckout(identity, checkoutPath, CheckoutOptions{})
	if err == nil {
		t.Fatal("EnsureCheckout() error = nil, want remote mismatch")
	}
	if !strings.Contains(err.Error(), "does not match requested") {
		t.Fatalf("EnsureCheckout() error = %q, want remote mismatch", err)
	}
	assertNoPull(t, runner.calls)
}

func TestEnsureCheckoutRejectsNonGitDirectory(t *testing.T) {
	identity := mustIdentity(t)
	checkoutPath := existingDir(t)
	runner := newFakeGitRunner()
	runner.errors["-C "+checkoutPath+" rev-parse --is-inside-work-tree"] = errors.New("not a git repository")

	_, err := NewCheckoutService(runner).EnsureCheckout(identity, checkoutPath, CheckoutOptions{})
	if err == nil {
		t.Fatal("EnsureCheckout() error = nil, want non-git conflict")
	}
	if !strings.Contains(err.Error(), "not a git checkout") {
		t.Fatalf("EnsureCheckout() error = %q, want non-git conflict", err)
	}
}

func TestEnsureCheckoutRejectsWorktreeSubdirectory(t *testing.T) {
	identity := mustIdentity(t)
	repoRoot := existingDir(t)
	checkoutPath := filepath.Join(repoRoot, "nested")
	if err := os.MkdirAll(checkoutPath, 0o755); err != nil {
		t.Fatalf("create nested checkout path: %v", err)
	}
	runner := newFakeGitRunner()
	runner.outputs["-C "+checkoutPath+" rev-parse --is-inside-work-tree"] = "true\n"
	runner.outputs["-C "+checkoutPath+" rev-parse --show-toplevel"] = repoRoot + "\n"

	_, err := NewCheckoutService(runner).EnsureCheckout(identity, checkoutPath, CheckoutOptions{})
	if err == nil {
		t.Fatal("EnsureCheckout() error = nil, want checkout root conflict")
	}
	if !strings.Contains(err.Error(), "not the checkout root") {
		t.Fatalf("EnsureCheckout() error = %q, want checkout root conflict", err)
	}
}

func TestEnsureCheckoutRejectsExistingFileAndSymlink(t *testing.T) {
	identity := mustIdentity(t)
	filePath := filepath.Join(t.TempDir(), "checkout-file")
	if err := os.WriteFile(filePath, []byte("file"), 0o644); err != nil {
		t.Fatalf("write file checkout: %v", err)
	}
	_, err := NewCheckoutService(newFakeGitRunner()).EnsureCheckout(identity, filePath, CheckoutOptions{})
	if err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("EnsureCheckout(file) error = %v, want not a directory", err)
	}

	target := existingDir(t)
	symlinkPath := filepath.Join(t.TempDir(), "checkout-link")
	if err := os.Symlink(target, symlinkPath); err != nil {
		t.Fatalf("create symlink checkout: %v", err)
	}
	_, err = NewCheckoutService(newFakeGitRunner()).EnsureCheckout(identity, symlinkPath, CheckoutOptions{})
	if err == nil || !strings.Contains(err.Error(), "is a symlink") {
		t.Fatalf("EnsureCheckout(symlink) error = %v, want symlink conflict", err)
	}
}

func TestEnsureCheckoutRejectsMissingOrUnsupportedOrigin(t *testing.T) {
	tests := []struct {
		name        string
		origin      string
		originError error
		want        string
	}{
		{name: "missing origin", originError: errors.New("exit status 1"), want: "cannot read origin remote"},
		{name: "blank origin", origin: "  \n", want: "missing origin remote"},
		{name: "unsupported origin", origin: "/tmp/local-repo", want: "unsupported origin remote"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			identity := mustIdentity(t)
			checkoutPath := existingDir(t)
			runner := newFakeGitRunner()
			runner.outputs["-C "+checkoutPath+" rev-parse --is-inside-work-tree"] = "true\n"
			runner.outputs["-C "+checkoutPath+" rev-parse --show-toplevel"] = checkoutPath + "\n"
			originKey := "-C " + checkoutPath + " config --get remote.origin.url"
			if test.originError != nil {
				runner.errors[originKey] = test.originError
			} else {
				runner.outputs[originKey] = test.origin
			}

			_, err := NewCheckoutService(runner).EnsureCheckout(identity, checkoutPath, CheckoutOptions{})
			if err == nil {
				t.Fatal("EnsureCheckout() error = nil, want origin conflict")
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("EnsureCheckout() error = %q, want substring %q", err, test.want)
			}
		})
	}
}

func TestEnsureCheckoutCloneFailureDoesNotReportClonedOrCollectCommit(t *testing.T) {
	identity := mustIdentity(t)
	checkoutPath := filepath.Join(t.TempDir(), "repos", "github.com", "addyosmani", "agent-skills")
	runner := newFakeGitRunner()
	runner.errors["clone "+identity.OriginalURL+" "+checkoutPath] = errors.New("clone failed")

	result, err := NewCheckoutService(runner).EnsureCheckout(identity, checkoutPath, CheckoutOptions{})
	if err == nil {
		t.Fatal("EnsureCheckout() error = nil, want clone failure")
	}
	if result.Cloned || result.Reused || result.WouldClone || result.LastSeenCommit != "" {
		t.Fatalf("CheckoutResult after clone failure = %#v, want no success flags", result)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("git calls = %#v, want only clone", runner.calls)
	}
}

func TestEnsureCheckoutCommitFailureDoesNotFailCheckout(t *testing.T) {
	identity := mustIdentity(t)
	checkoutPath := existingDir(t)
	runner := matchingCheckoutRunner(checkoutPath, "https://github.com/addyosmani/agent-skills.git", "")
	runner.errors["-C "+checkoutPath+" rev-parse HEAD"] = errors.New("no commits")

	result, err := NewCheckoutService(runner).EnsureCheckout(identity, checkoutPath, CheckoutOptions{})
	if err != nil {
		t.Fatalf("EnsureCheckout() error = %v", err)
	}
	if !result.Reused {
		t.Fatalf("CheckoutResult = %#v, want reused", result)
	}
	if result.LastSeenCommit != "" {
		t.Fatalf("LastSeenCommit = %q, want empty", result.LastSeenCommit)
	}
}

func TestEnsureCheckoutValidatesInputs(t *testing.T) {
	checkoutPath := filepath.Join(t.TempDir(), "repo")
	tests := []struct {
		name     string
		identity RepoIdentity
		path     string
		want     string
	}{
		{name: "missing url", identity: RepoIdentity{CanonicalURL: "https://github.com/owner/repo", Host: "github.com", RepoPath: "owner/repo"}, path: checkoutPath, want: "missing repository URL"},
		{name: "blank url", identity: RepoIdentity{OriginalURL: " ", CanonicalURL: "https://github.com/owner/repo", Host: "github.com", RepoPath: "owner/repo"}, path: checkoutPath, want: "missing repository URL"},
		{name: "missing identity", identity: RepoIdentity{OriginalURL: "https://github.com/owner/repo"}, path: checkoutPath, want: "missing repository identity"},
		{name: "blank identity", identity: RepoIdentity{OriginalURL: "https://github.com/owner/repo", CanonicalURL: " ", Host: "github.com", RepoPath: "owner/repo"}, path: checkoutPath, want: "missing repository identity"},
		{name: "missing path", identity: mustIdentity(t), path: " ", want: "missing checkout path"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewCheckoutService(newFakeGitRunner()).EnsureCheckout(test.identity, test.path, CheckoutOptions{})
			if err == nil {
				t.Fatal("EnsureCheckout() error = nil, want validation error")
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("EnsureCheckout() error = %q, want substring %q", err, test.want)
			}
		})
	}
}

func mustIdentity(t *testing.T) RepoIdentity {
	t.Helper()
	identity, err := NormalizeGitURL("https://github.com/addyosmani/agent-skills.git")
	if err != nil {
		t.Fatalf("NormalizeGitURL() error = %v", err)
	}
	return identity
}

func existingDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create dir %s: %v", dir, err)
	}
	return dir
}

func matchingCheckoutRunner(checkoutPath, origin, commit string) *fakeGitRunner {
	runner := newFakeGitRunner()
	runner.outputs["-C "+checkoutPath+" rev-parse --is-inside-work-tree"] = "true\n"
	runner.outputs["-C "+checkoutPath+" rev-parse --show-toplevel"] = checkoutPath + "\n"
	runner.outputs["-C "+checkoutPath+" config --get remote.origin.url"] = origin
	runner.outputs["-C "+checkoutPath+" rev-parse HEAD"] = commit
	return runner
}

func assertNoPull(t *testing.T, calls [][]string) {
	t.Helper()
	for _, call := range calls {
		for _, arg := range call {
			if arg == "pull" {
				t.Fatalf("git calls include pull: %#v", calls)
			}
		}
	}
}

type fakeGitRunner struct {
	calls   [][]string
	outputs map[string]string
	errors  map[string]error
}

func newFakeGitRunner() *fakeGitRunner {
	return &fakeGitRunner{
		outputs: map[string]string{},
		errors:  map[string]error{},
	}
}

func (r *fakeGitRunner) RunGit(args ...string) (string, error) {
	r.calls = append(r.calls, append([]string(nil), args...))
	key := strings.Join(args, " ")
	if err, ok := r.errors[key]; ok {
		return "", err
	}
	if output, ok := r.outputs[key]; ok {
		return output, nil
	}
	return "", nil
}
