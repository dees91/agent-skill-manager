package install

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/dees91/agent-skill-manager/internal/model"
	"github.com/dees91/agent-skill-manager/internal/paths"
)

func TestNormalizeGitURLAcceptsSupportedForms(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		original string
	}{
		{
			name:     "https",
			input:    "https://github.com/addyosmani/agent-skills",
			original: "https://github.com/addyosmani/agent-skills",
		},
		{
			name:     "https git suffix",
			input:    "https://github.com/addyosmani/agent-skills.git",
			original: "https://github.com/addyosmani/agent-skills.git",
		},
		{
			name:     "https trailing slash",
			input:    "https://github.com/addyosmani/agent-skills/",
			original: "https://github.com/addyosmani/agent-skills/",
		},
		{
			name:     "ssh scp syntax",
			input:    "git@github.com:addyosmani/agent-skills.git",
			original: "git@github.com:addyosmani/agent-skills.git",
		},
		{
			name:     "trim input whitespace",
			input:    "  https://GitHub.com/addyosmani/agent-skills.git  ",
			original: "https://GitHub.com/addyosmani/agent-skills.git",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := NormalizeGitURL(test.input)
			if err != nil {
				t.Fatalf("NormalizeGitURL(%q) unexpected error: %v", test.input, err)
			}
			if got.OriginalURL != test.original {
				t.Fatalf("OriginalURL = %q, want %q", got.OriginalURL, test.original)
			}
			if got.CanonicalURL != "https://github.com/addyosmani/agent-skills" {
				t.Fatalf("CanonicalURL = %q, want https://github.com/addyosmani/agent-skills", got.CanonicalURL)
			}
			if got.Host != "github.com" {
				t.Fatalf("Host = %q, want github.com", got.Host)
			}
			if got.RepoPath != "addyosmani/agent-skills" {
				t.Fatalf("RepoPath = %q, want addyosmani/agent-skills", got.RepoPath)
			}
			if got.Group != model.GroupLabel("addyosmani/agent-skills") {
				t.Fatalf("Group = %q, want addyosmani/agent-skills", got.Group)
			}
		})
	}
}

func TestNormalizeGitURLRejectsUnsupportedInputs(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "empty", raw: "", want: "empty"},
		{name: "local absolute path", raw: "/tmp/repo", want: "local paths"},
		{name: "local relative path", raw: "./repo", want: "local paths"},
		{name: "github shorthand", raw: "addyosmani/agent-skills", want: "shorthand"},
		{name: "git scheme", raw: "git://github.com/addyosmani/agent-skills", want: "only HTTPS and SSH"},
		{name: "http scheme", raw: "http://github.com/addyosmani/agent-skills", want: "only HTTPS and SSH"},
		{name: "missing host", raw: "https:///addyosmani/agent-skills", want: "missing host"},
		{name: "explicit port", raw: "https://git.example.com:8443/org/repo", want: "explicit ports"},
		{name: "missing path", raw: "https://github.com", want: "missing repository path"},
		{name: "query", raw: "https://github.com/addyosmani/agent-skills?tab=readme", want: "query"},
		{name: "fragment", raw: "https://github.com/addyosmani/agent-skills#readme", want: "fragment"},
		{name: "userinfo", raw: "https://token@github.com/addyosmani/agent-skills", want: "user info"},
		{name: "unsafe dot segment", raw: "https://github.com/addyosmani/../agent-skills", want: "unsafe repository path"},
		{name: "unsafe escaped dot segment", raw: "https://github.com/addyosmani/%2e%2e/agent-skills", want: "unsafe repository path"},
		{name: "ssh missing path", raw: "git@github.com:", want: "missing SSH repository path"},
		{name: "ssh missing user", raw: "@github.com:addyosmani/agent-skills.git", want: "invalid SSH git URL"},
		{name: "ssh wrong user", raw: "example-user@github.com:addyosmani/agent-skills.git", want: "SSH git URL must use git user"},
		{name: "ssh backslash path", raw: `git@github.com:addyosmani\agent-skills.git`, want: "unsafe repository path"},
		{name: "local drive style", raw: `C:\repo`, want: "local paths"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NormalizeGitURL(test.raw)
			if err == nil {
				t.Fatalf("NormalizeGitURL(%q) succeeded, want error", test.raw)
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("NormalizeGitURL(%q) error = %q, want substring %q", test.raw, err, test.want)
			}
		})
	}
}

func TestCheckoutPathResolvesUnderManagedReposDir(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	identity, err := NormalizeGitURL("https://github.com/addyosmani/agent-skills.git")
	if err != nil {
		t.Fatalf("NormalizeGitURL unexpected error: %v", err)
	}

	got, err := CheckoutPath(p, identity)
	if err != nil {
		t.Fatalf("CheckoutPath unexpected error: %v", err)
	}

	want := filepath.Join(p.ReposDir, "github.com", "addyosmani", "agent-skills")
	if got != want {
		t.Fatalf("CheckoutPath = %q, want %q", got, want)
	}
}

func TestCheckoutPathDefendsAgainstEscapingRepoPath(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	_, err := CheckoutPath(p, RepoIdentity{
		Host:     "github.com",
		RepoPath: "../outside",
	})
	if err == nil {
		t.Fatal("CheckoutPath succeeded for escaping repo path, want error")
	}
	if !strings.Contains(err.Error(), "unsafe repository path") {
		t.Fatalf("CheckoutPath error = %q, want unsafe repository path", err)
	}
}

func TestCheckoutPathDefendsAgainstUnsafeHost(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	_, err := CheckoutPath(p, RepoIdentity{
		Host:     "../github.com",
		RepoPath: "addyosmani/agent-skills",
	})
	if err == nil {
		t.Fatal("CheckoutPath succeeded for unsafe host, want error")
	}
	if !strings.Contains(err.Error(), "unsafe host") {
		t.Fatalf("CheckoutPath error = %q, want unsafe host", err)
	}
}
