package install

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dees91/agent-skill-manager/internal/model"
	"github.com/dees91/agent-skill-manager/internal/paths"
)

// LocalSource identifies a user-owned directory used for link-in-place installs.
type LocalSource struct {
	OriginalPath  string
	CanonicalPath string
	Group         model.GroupLabel
}

// LocalSourceLookup is a normalized local input that may no longer exist.
type LocalSourceLookup struct {
	OriginalPath  string
	CanonicalPath string
	Exists        bool
}

// LooksLikeLocalPathInput reports whether an install argument is explicitly a
// path or names an existing filesystem entry relative to cwd.
func LooksLikeLocalPathInput(raw, home, cwd string) bool {
	input := strings.TrimSpace(raw)
	if input == "" || strings.HasPrefix(input, "https://") || looksLikeSCPGitURL(input) {
		return false
	}
	if input == "~" || strings.HasPrefix(input, "~/") || filepath.IsAbs(input) || strings.HasPrefix(input, "./") || strings.HasPrefix(input, "../") {
		return true
	}
	expanded, err := expandLocalInput(input, home, cwd)
	if err != nil {
		return false
	}
	_, err = os.Lstat(expanded)
	return err == nil
}

// ResolveLocalSource validates and canonicalizes an existing local directory.
func ResolveLocalSource(p paths.Paths, cwd, raw string) (LocalSource, error) {
	lookup, err := ResolveLocalSourceLookup(p, cwd, raw)
	if err != nil {
		return LocalSource{}, err
	}
	if !lookup.Exists {
		return LocalSource{}, fmt.Errorf("local source path %s does not exist", lookup.OriginalPath)
	}
	info, err := os.Lstat(lookup.CanonicalPath)
	if err != nil {
		return LocalSource{}, fmt.Errorf("inspect local source path %s: %w", lookup.CanonicalPath, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return LocalSource{}, fmt.Errorf("local source path %s did not resolve to a real directory", lookup.CanonicalPath)
	}
	if !info.IsDir() {
		return LocalSource{}, fmt.Errorf("local source path %s is not a directory", lookup.CanonicalPath)
	}
	if err := validateLocalSourceBoundaries(p, lookup.CanonicalPath); err != nil {
		return LocalSource{}, err
	}
	group := model.GroupLabel(filepath.Base(lookup.CanonicalPath))
	if group == "" || group == "." || group.String() == string(filepath.Separator) {
		return LocalSource{}, fmt.Errorf("cannot derive group from local source path %s", lookup.CanonicalPath)
	}
	return LocalSource{OriginalPath: lookup.OriginalPath, CanonicalPath: lookup.CanonicalPath, Group: group}, nil
}

// ResolveLocalSourceLookup normalizes a local path without requiring the final
// source to exist. This lets uninstall clean recorded broken symlinks.
func ResolveLocalSourceLookup(p paths.Paths, cwd, raw string) (LocalSourceLookup, error) {
	original, err := expandLocalInput(strings.TrimSpace(raw), p.Home, cwd)
	if err != nil {
		return LocalSourceLookup{}, err
	}
	original, err = filepath.Abs(filepath.Clean(original))
	if err != nil {
		return LocalSourceLookup{}, fmt.Errorf("resolve local source path %q: %w", raw, err)
	}
	canonical, err := filepath.EvalSymlinks(original)
	if err == nil {
		canonical, err = filepath.Abs(filepath.Clean(canonical))
		if err != nil {
			return LocalSourceLookup{}, fmt.Errorf("resolve canonical local source path %s: %w", original, err)
		}
		return LocalSourceLookup{OriginalPath: original, CanonicalPath: canonical, Exists: true}, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return LocalSourceLookup{}, fmt.Errorf("resolve local source path %s: %w", original, err)
	}
	canonical, canonicalErr := canonicalizeAllowMissing(original)
	if canonicalErr != nil {
		return LocalSourceLookup{}, fmt.Errorf("resolve missing local source path %s: %w", original, canonicalErr)
	}
	return LocalSourceLookup{OriginalPath: original, CanonicalPath: canonical, Exists: false}, nil
}

// DiscoverLocalSkills treats a root SKILL.md as one exact skill and otherwise
// uses recursive repository-style discovery.
func DiscoverLocalSkills(source LocalSource) ([]DiscoveredSkill, error) {
	skillFile := filepath.Join(source.CanonicalPath, "SKILL.md")
	info, err := os.Lstat(skillFile)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return nil, fmt.Errorf("local root skill file %s is not a regular file", skillFile)
		}
		return []DiscoveredSkill{{
			Name:         filepath.Base(source.CanonicalPath),
			Path:         source.CanonicalPath,
			RelativePath: ".",
		}}, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("inspect local root skill file %s: %w", skillFile, err)
	}
	return DiscoverSkills(source.CanonicalPath)
}

func expandLocalInput(input, home, cwd string) (string, error) {
	if input == "" {
		return "", fmt.Errorf("local source path is required")
	}
	if input == "~" {
		input = home
	} else if strings.HasPrefix(input, "~/") {
		input = filepath.Join(home, strings.TrimPrefix(input, "~/"))
	} else if strings.HasPrefix(input, "~") {
		return "", fmt.Errorf("unsupported home-relative path %q", input)
	}
	if !filepath.IsAbs(input) {
		if strings.TrimSpace(cwd) == "" {
			return "", fmt.Errorf("resolve local source path %q: current directory is unavailable", input)
		}
		input = filepath.Join(cwd, input)
	}
	return filepath.Clean(input), nil
}

func validateLocalSourceBoundaries(p paths.Paths, sourcePath string) error {
	protected := []string{
		p.StateDir,
		p.ClaudeUserSkills,
		p.CodexUserSkills,
		p.MuseUserSkills,
		p.CodexSystemSkills,
		p.ClaudePluginCache,
	}
	for _, candidate := range protected {
		canonicalProtected, err := canonicalizeAllowMissing(candidate)
		if err != nil {
			return fmt.Errorf("resolve protected path %s: %w", candidate, err)
		}
		if pathInside(sourcePath, canonicalProtected) || pathInside(canonicalProtected, sourcePath) {
			return fmt.Errorf("local source path %s overlaps protected path %s", sourcePath, candidate)
		}
	}
	return nil
}

func canonicalizeAllowMissing(input string) (string, error) {
	absolute, err := filepath.Abs(filepath.Clean(input))
	if err != nil {
		return "", err
	}
	cursor := absolute
	suffix := []string{}
	for {
		resolved, err := filepath.EvalSymlinks(cursor)
		if err == nil {
			for i := len(suffix) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, suffix[i])
			}
			return filepath.Abs(filepath.Clean(resolved))
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(cursor)
		if parent == cursor {
			return absolute, nil
		}
		suffix = append(suffix, filepath.Base(cursor))
		cursor = parent
	}
}
