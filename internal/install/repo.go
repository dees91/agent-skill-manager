package install

import (
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"strings"

	"github.com/dees91/agent-skill-manager/internal/model"
	"github.com/dees91/agent-skill-manager/internal/paths"
)

// RepoIdentity is the normalized identity for a supported Git repository URL.
type RepoIdentity struct {
	OriginalURL  string
	CanonicalURL string
	Host         string
	RepoPath     string
	Group        model.GroupLabel
}

// NormalizeGitURL parses supported repository inputs into a stable identity.
func NormalizeGitURL(raw string) (RepoIdentity, error) {
	input := strings.TrimSpace(raw)
	if input == "" {
		return RepoIdentity{}, fmt.Errorf("unsupported git URL: empty input")
	}

	if looksLikeSCPGitURL(input) {
		return normalizeSCPGitURL(input)
	}
	if looksLikeLocalPath(input) {
		return RepoIdentity{}, fmt.Errorf("unsupported git URL %q: local paths are not supported", raw)
	}

	parsed, err := url.Parse(input)
	if err != nil {
		return RepoIdentity{}, fmt.Errorf("unsupported git URL %q: %w", raw, err)
	}
	if parsed.Scheme == "" {
		return RepoIdentity{}, fmt.Errorf("unsupported git URL %q: GitHub shorthand is not supported", raw)
	}
	if parsed.Scheme != "https" {
		return RepoIdentity{}, fmt.Errorf("unsupported git URL %q: only HTTPS and SSH git URLs are supported", raw)
	}
	if parsed.Host == "" {
		return RepoIdentity{}, fmt.Errorf("unsupported git URL %q: missing host", raw)
	}
	if parsed.Port() != "" {
		return RepoIdentity{}, fmt.Errorf("unsupported git URL %q: explicit ports are not supported", raw)
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return RepoIdentity{}, fmt.Errorf("unsupported git URL %q: user info, query, and fragment are not supported", raw)
	}

	return buildIdentity(input, parsed.Hostname(), parsed.Path)
}

func normalizeSCPGitURL(input string) (RepoIdentity, error) {
	before, after, ok := strings.Cut(input, ":")
	if !ok || after == "" {
		return RepoIdentity{}, fmt.Errorf("unsupported git URL %q: missing SSH repository path", input)
	}
	user, host, ok := strings.Cut(before, "@")
	if !ok || user == "" || host == "" {
		return RepoIdentity{}, fmt.Errorf("unsupported git URL %q: invalid SSH git URL", input)
	}
	if strings.ContainsAny(before, `/\`) || strings.Contains(host, ":") {
		return RepoIdentity{}, fmt.Errorf("unsupported git URL %q: invalid SSH host", input)
	}
	if user != "git" {
		return RepoIdentity{}, fmt.Errorf("unsupported git URL %q: SSH git URL must use git user", input)
	}
	return buildIdentity(input, host, after)
}

// CheckoutPath returns the managed checkout path for a repository identity.
func CheckoutPath(p paths.Paths, identity RepoIdentity) (string, error) {
	if identity.Host == "" || identity.RepoPath == "" {
		return "", fmt.Errorf("cannot resolve checkout path: missing repository identity")
	}
	if err := validateHost(identity.Host); err != nil {
		return "", fmt.Errorf("cannot resolve checkout path: %w", err)
	}
	if err := validateRepoPath(identity.RepoPath); err != nil {
		return "", fmt.Errorf("cannot resolve checkout path: %w", err)
	}

	base := filepath.Clean(p.ReposDir)
	checkout := filepath.Join(base, filepath.FromSlash(identity.Host), filepath.FromSlash(identity.RepoPath))
	rel, err := filepath.Rel(base, checkout)
	if err != nil {
		return "", fmt.Errorf("cannot resolve checkout path: %w", err)
	}
	if rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." || filepath.IsAbs(rel) {
		return "", fmt.Errorf("cannot resolve checkout path: repository path escapes %s", p.ReposDir)
	}
	return checkout, nil
}

func buildIdentity(originalURL, host, repoPath string) (RepoIdentity, error) {
	host = strings.ToLower(strings.TrimSpace(host))
	if err := validateHost(host); err != nil {
		return RepoIdentity{}, fmt.Errorf("unsupported git URL %q: %w", originalURL, err)
	}

	normalizedPath, err := normalizeRepoPath(repoPath)
	if err != nil {
		return RepoIdentity{}, fmt.Errorf("unsupported git URL %q: %w", originalURL, err)
	}

	canonicalURL := "https://" + host + "/" + normalizedPath
	return RepoIdentity{
		OriginalURL:  originalURL,
		CanonicalURL: canonicalURL,
		Host:         host,
		RepoPath:     normalizedPath,
		Group:        model.GroupLabel(normalizedPath),
	}, nil
}

func normalizeRepoPath(repoPath string) (string, error) {
	repoPath = strings.TrimSpace(repoPath)
	repoPath = strings.TrimPrefix(repoPath, "/")
	repoPath = strings.TrimRight(repoPath, "/")
	if repoPath == "" {
		return "", fmt.Errorf("missing repository path")
	}
	if strings.HasSuffix(repoPath, ".git") {
		repoPath = strings.TrimSuffix(repoPath, ".git")
		repoPath = strings.TrimRight(repoPath, "/")
	}
	if repoPath == "" {
		return "", fmt.Errorf("missing repository path")
	}

	cleaned := path.Clean(repoPath)
	if cleaned == "." || cleaned != repoPath {
		return "", fmt.Errorf("unsafe repository path %q", repoPath)
	}
	if err := validateRepoPath(cleaned); err != nil {
		return "", err
	}
	return cleaned, nil
}

func validateRepoPath(repoPath string) error {
	if repoPath == "" {
		return fmt.Errorf("missing repository path")
	}
	if strings.HasPrefix(repoPath, "/") {
		return fmt.Errorf("unsafe repository path %q", repoPath)
	}
	if strings.Contains(repoPath, `\`) {
		return fmt.Errorf("unsafe repository path %q", repoPath)
	}
	parts := strings.Split(repoPath, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return fmt.Errorf("unsafe repository path %q", repoPath)
		}
	}
	return nil
}

func validateHost(host string) error {
	if host == "" {
		return fmt.Errorf("missing host")
	}
	if host == "." || host == ".." || strings.ContainsAny(host, `/\:`) || strings.TrimSpace(host) != host {
		return fmt.Errorf("unsafe host %q", host)
	}
	return nil
}

func looksLikeLocalPath(input string) bool {
	return strings.HasPrefix(input, "/") ||
		strings.HasPrefix(input, "./") ||
		strings.HasPrefix(input, "../") ||
		strings.HasPrefix(input, "~/") ||
		strings.Contains(input, `\`)
}

func looksLikeSCPGitURL(input string) bool {
	before, _, ok := strings.Cut(input, ":")
	return ok && strings.Contains(before, "@") && !strings.ContainsAny(before, `/\`)
}
