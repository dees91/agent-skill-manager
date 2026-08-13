package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dees91/agent-skill-manager/internal/model"
	"github.com/dees91/agent-skill-manager/internal/paths"
)

func TestResolveLocalSourceAcceptsSupportedPathFormsAndCanonicalizesSymlinks(t *testing.T) {
	home := t.TempDir()
	p := paths.ForHome(home)
	cwd := filepath.Join(home, "workspace")
	realSource := filepath.Join(cwd, "sources", "my-skills")
	writeSkill(t, filepath.Join(realSource, "alpha"))
	canonicalSource, err := filepath.EvalSymlinks(realSource)
	if err != nil {
		t.Fatalf("canonicalize source: %v", err)
	}
	alias := filepath.Join(cwd, "source-alias")
	if err := os.Symlink(realSource, alias); err != nil {
		t.Fatalf("create source alias: %v", err)
	}

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "absolute", input: realSource, want: canonicalSource},
		{name: "explicit relative", input: "./sources/my-skills", want: canonicalSource},
		{name: "bare existing", input: "sources/my-skills", want: canonicalSource},
		{name: "home relative", input: "~/workspace/sources/my-skills", want: canonicalSource},
		{name: "symlink root", input: alias, want: canonicalSource},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if !LooksLikeLocalPathInput(test.input, home, cwd) {
				t.Fatalf("LooksLikeLocalPathInput(%q) = false", test.input)
			}
			got, err := ResolveLocalSource(p, cwd, test.input)
			if err != nil {
				t.Fatalf("ResolveLocalSource(%q) error = %v", test.input, err)
			}
			if got.CanonicalPath != test.want || got.Group != model.GroupLabel("my-skills") {
				t.Fatalf("ResolveLocalSource(%q) = %#v", test.input, got)
			}
		})
	}
}

func TestResolveLocalSourceRejectsMissingFilesAndProtectedOverlap(t *testing.T) {
	home := t.TempDir()
	p := paths.ForHome(home)
	cwd := filepath.Join(home, "workspace")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatalf("create cwd: %v", err)
	}
	plainFile := filepath.Join(cwd, "plain-file")
	if err := os.WriteFile(plainFile, []byte("data"), 0o644); err != nil {
		t.Fatalf("write plain file: %v", err)
	}
	if err := os.MkdirAll(p.ClaudeUserSkills, 0o755); err != nil {
		t.Fatalf("create protected path: %v", err)
	}

	tests := []struct {
		input string
		want  string
	}{
		{input: "./missing", want: "does not exist"},
		{input: plainFile, want: "not a directory"},
		{input: p.ClaudeUserSkills, want: "overlaps protected path"},
		{input: home, want: "overlaps protected path"},
		{input: "~someone/skills", want: "unsupported home-relative"},
	}
	for _, test := range tests {
		t.Run(filepath.Base(test.input), func(t *testing.T) {
			_, err := ResolveLocalSource(p, cwd, test.input)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("ResolveLocalSource(%q) error = %v, want %q", test.input, err, test.want)
			}
		})
	}
}

func TestResolveLocalSourceLookupAllowsMissingPath(t *testing.T) {
	home := t.TempDir()
	p := paths.ForHome(home)
	cwd := filepath.Join(home, "workspace")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatalf("create cwd: %v", err)
	}
	lookup, err := ResolveLocalSourceLookup(p, cwd, "./removed-source")
	if err != nil {
		t.Fatalf("ResolveLocalSourceLookup() error = %v", err)
	}
	want := filepath.Join(cwd, "removed-source")
	canonicalWant, err := canonicalizeAllowMissing(want)
	if err != nil {
		t.Fatalf("canonicalize missing path: %v", err)
	}
	if lookup.Exists || lookup.OriginalPath != want || lookup.CanonicalPath != canonicalWant {
		t.Fatalf("lookup = %#v, want original %s canonical %s", lookup, want, canonicalWant)
	}
}

func TestDiscoverLocalSkillsTreatsRootSkillAsOneAndCollectionsRecursively(t *testing.T) {
	t.Run("root skill", func(t *testing.T) {
		root := t.TempDir()
		writeSkill(t, root)
		writeSkill(t, filepath.Join(root, "nested", "ignored-child"))
		got, err := DiscoverLocalSkills(LocalSource{CanonicalPath: root})
		if err != nil {
			t.Fatalf("DiscoverLocalSkills() error = %v", err)
		}
		if len(got) != 1 || got[0].Path != root || got[0].RelativePath != "." {
			t.Fatalf("DiscoverLocalSkills() = %#v, want root only", got)
		}
	})

	t.Run("collection", func(t *testing.T) {
		root := t.TempDir()
		writeSkill(t, filepath.Join(root, "skills", "alpha"))
		writeSkill(t, filepath.Join(root, "skills", "beta"))
		got, err := DiscoverLocalSkills(LocalSource{CanonicalPath: root})
		if err != nil {
			t.Fatalf("DiscoverLocalSkills() error = %v", err)
		}
		if len(got) != 2 || got[0].Name != "alpha" || got[1].Name != "beta" {
			t.Fatalf("DiscoverLocalSkills() = %#v, want alpha/beta", got)
		}
	})
}

func TestLooksLikeLocalPathInputPreservesGitInputsAndMissingShorthand(t *testing.T) {
	home := t.TempDir()
	for _, input := range []string{"https://github.com/owner/repo", "git@github.com:owner/repo.git", "owner/repo"} {
		if LooksLikeLocalPathInput(input, home, home) {
			t.Fatalf("LooksLikeLocalPathInput(%q) = true", input)
		}
	}
}
