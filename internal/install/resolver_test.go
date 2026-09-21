package install

import (
	"errors"
	"strings"
	"testing"

	"github.com/dees91/agent-skill-manager/internal/state"
)

func TestParseSkillSelection(t *testing.T) {
	selection, err := ParseSkillSelection("  demo  ")
	if err != nil {
		t.Fatalf("ParseSkillSelection() error = %v", err)
	}
	if selection.Name != "demo" || selection.Path != "" {
		t.Fatalf("ParseSkillSelection() = %#v, want bare demo", selection)
	}

	selection, err = ParseSkillSelection("demo=plugin/skills/demo")
	if err != nil {
		t.Fatalf("ParseSkillSelection() error = %v", err)
	}
	if selection.Name != "demo" || selection.Path != "plugin/skills/demo" {
		t.Fatalf("ParseSkillSelection() = %#v, want qualified demo", selection)
	}

	selection, err = ParseSkillSelection("demo=./plugin//skills/demo")
	if err != nil {
		t.Fatalf("ParseSkillSelection() error = %v", err)
	}
	if selection.Path != "plugin/skills/demo" {
		t.Fatalf("Path = %q, want plugin/skills/demo", selection.Path)
	}

	for _, raw := range []string{"", "   ", "=skills/demo", "demo=", "demo=../escape", "demo=/abs/path"} {
		if _, err := ParseSkillSelection(raw); err == nil {
			t.Fatalf("ParseSkillSelection(%q) error = nil, want error", raw)
		}
	}
}

func TestSplitSkillRequestsQualifiedAndLiteralFallback(t *testing.T) {
	checkout := t.TempDir()
	writeSkill(t, checkout+"/skills/demo")
	// A skill literally named with "=" keeps working when the qualified form
	// matches nothing.
	writeSkill(t, checkout+"/skills/a=b")
	discovered, err := DiscoverSkills(checkout)
	if err != nil {
		t.Fatalf("DiscoverSkills() error = %v", err)
	}

	names, explicit, err := SplitSkillRequests(discovered, []string{"demo=skills/demo", "a=b"})
	if err != nil {
		t.Fatalf("SplitSkillRequests() error = %v", err)
	}
	if len(names) != 2 || names[0] != "demo" || names[1] != "a=b" {
		t.Fatalf("names = %q, want [demo a=b]", names)
	}
	if explicit["demo"] != "skills/demo" {
		t.Fatalf("explicit = %#v, want demo choice", explicit)
	}
	if _, ok := explicit["a=b"]; ok {
		t.Fatalf("explicit = %#v, want no choice for literal name", explicit)
	}

	edgeCheckout := t.TempDir()
	writeSkill(t, edgeCheckout+"/skills/demo=")
	writeSkill(t, edgeCheckout+"/skills/=demo")
	edgeDiscovered, err := DiscoverSkills(edgeCheckout)
	if err != nil {
		t.Fatalf("DiscoverSkills() error = %v", err)
	}
	names, explicit, err = SplitSkillRequests(edgeDiscovered, []string{"demo=", "=demo"})
	if err != nil {
		t.Fatalf("SplitSkillRequests() error = %v", err)
	}
	if len(names) != 2 || len(explicit) != 0 {
		t.Fatalf("names = %q explicit = %#v, want two bare literal names", names, explicit)
	}

	if _, _, err := SplitSkillRequests(discovered, []string{"demo=skills/demo", "demo=other/demo"}); err == nil {
		t.Fatal("SplitSkillRequests(conflicting paths) error = nil, want error")
	}
}

func resolverFixtureDiscovery(t *testing.T) (Discovery, string) {
	t.Helper()
	checkout := t.TempDir()
	writeSkill(t, checkout+"/skills/alpha")
	writeSkill(t, checkout+"/plugin/skills/beta")
	writeSkill(t, checkout+"/.agent/skills/beta")
	writeSkillContent(t, checkout+"/packs/one/gamma", "# One\n")
	writeSkillContent(t, checkout+"/packs/two/gamma", "# Two\n")
	discovered, err := DiscoverSkills(checkout)
	if err != nil {
		t.Fatalf("DiscoverSkills() error = %v", err)
	}
	return discovered, checkout
}

func TestResolveDiscoverySelectsAllAndAutoResolvesIdentical(t *testing.T) {
	discovered, _ := resolverFixtureDiscovery(t)

	resolution, err := ResolveDiscovery(discovered, nil, nil, nil)
	if err == nil {
		t.Fatal("ResolveDiscovery() error = nil, want ambiguous error for gamma")
	}
	var ambiguous AmbiguousSkillsError
	if !errors.As(err, &ambiguous) {
		t.Fatalf("ResolveDiscovery() error = %T %v, want AmbiguousSkillsError", err, err)
	}
	if len(ambiguous.Groups) != 1 || ambiguous.Groups[0].Name != "gamma" {
		t.Fatalf("Ambiguous groups = %#v, want gamma", ambiguous.Groups)
	}
	if !strings.Contains(err.Error(), "--skill <name>=<path>") {
		t.Fatalf("error = %q, want qualification hint", err.Error())
	}

	resolution, err = ResolveDiscovery(discovered, []string{"alpha", "beta"}, nil, nil)
	if err != nil {
		t.Fatalf("ResolveDiscovery() error = %v", err)
	}
	if len(resolution.Selected) != 2 || len(resolution.Missing) != 0 {
		t.Fatalf("resolution = %#v, want alpha and beta", resolution)
	}
	if len(resolution.Resolved) != 1 || resolution.Resolved[0].Name != "beta" || resolution.Resolved[0].Copies != 2 {
		t.Fatalf("resolved = %#v, want beta with 2 copies", resolution.Resolved)
	}
	if resolution.Resolved[0].RelativePath != "plugin/skills/beta" {
		t.Fatalf("resolved path = %q, want canonical plugin/skills/beta", resolution.Resolved[0].RelativePath)
	}
}

func TestResolveDiscoveryQualifiedChoiceAndErrors(t *testing.T) {
	discovered, _ := resolverFixtureDiscovery(t)

	resolution, err := ResolveDiscovery(discovered, []string{"gamma"}, map[string]string{"gamma": "packs/two/gamma"}, nil)
	if err != nil {
		t.Fatalf("ResolveDiscovery() error = %v", err)
	}
	if len(resolution.Selected) != 1 || resolution.Selected[0].RelativePath != "packs/two/gamma" {
		t.Fatalf("selected = %#v, want packs/two/gamma", resolution.Selected)
	}
	if len(resolution.Resolved) != 0 {
		t.Fatalf("resolved = %#v, want none for an explicit choice", resolution.Resolved)
	}

	if _, err := ResolveDiscovery(discovered, []string{"gamma"}, map[string]string{"gamma": "packs/missing/gamma"}, nil); err == nil {
		t.Fatal("ResolveDiscovery(wrong path) error = nil, want error")
	} else if !strings.Contains(err.Error(), "has no copy at") {
		t.Fatalf("error = %q, want no-copy message", err.Error())
	}

	if _, err := ResolveDiscovery(discovered, []string{"alpha"}, map[string]string{"alpha": "elsewhere/alpha"}, nil); err == nil {
		t.Fatal("ResolveDiscovery(unique wrong path) error = nil, want error")
	} else if !strings.Contains(err.Error(), "has a single copy at") {
		t.Fatalf("error = %q, want single-copy message", err.Error())
	}
}

func TestResolveDiscoveryPrefersRecordedPathAndReportsDrift(t *testing.T) {
	discovered, _ := resolverFixtureDiscovery(t)

	// The recorded copy wins over the canonical ranking.
	resolution, err := ResolveDiscovery(discovered, []string{"beta"}, nil, map[string]string{"beta": ".agent/skills/beta"})
	if err != nil {
		t.Fatalf("ResolveDiscovery() error = %v", err)
	}
	if resolution.Selected[0].RelativePath != ".agent/skills/beta" {
		t.Fatalf("selected = %#v, want recorded copy", resolution.Selected)
	}

	// An explicit choice wins over the recorded path.
	resolution, err = ResolveDiscovery(discovered, []string{"beta"},
		map[string]string{"beta": "plugin/skills/beta"},
		map[string]string{"beta": ".agent/skills/beta"})
	if err != nil {
		t.Fatalf("ResolveDiscovery() error = %v", err)
	}
	if resolution.Selected[0].RelativePath != "plugin/skills/beta" {
		t.Fatalf("selected = %#v, want explicit copy", resolution.Selected)
	}

	// A recorded path that no longer holds the skill is drift, not a switch.
	_, err = ResolveDiscovery(discovered, []string{"beta"}, nil, map[string]string{"beta": "gone/beta"})
	if err == nil {
		t.Fatal("ResolveDiscovery(drift) error = nil, want error")
	} else if !strings.Contains(err.Error(), "no longer holds the skill") || !strings.Contains(err.Error(), "uninstall the source and reinstall") {
		t.Fatalf("error = %q, want drift message with guidance", err.Error())
	}

	// Drift on a now-unique skill carries the same guidance.
	_, err = ResolveDiscovery(discovered, []string{"alpha"}, nil, map[string]string{"alpha": "gone/alpha"})
	if err == nil {
		t.Fatal("ResolveDiscovery(unique drift) error = nil, want error")
	} else if !strings.Contains(err.Error(), "no longer holds the skill") || !strings.Contains(err.Error(), "uninstall the source and reinstall") {
		t.Fatalf("error = %q, want drift message with guidance", err.Error())
	}
}

func TestResolveDiscoveryMissing(t *testing.T) {
	discovered, _ := resolverFixtureDiscovery(t)

	resolution, err := ResolveDiscovery(discovered, []string{"alpha", "missing"}, nil, nil)
	if err != nil {
		t.Fatalf("ResolveDiscovery() error = %v", err)
	}
	if len(resolution.Missing) != 1 || resolution.Missing[0] != "missing" {
		t.Fatalf("missing = %#v, want [missing]", resolution.Missing)
	}
}

func TestCellsToRequests(t *testing.T) {
	names, explicit, err := CellsToRequests([]InstallCell{
		{SkillName: "beta", Path: "plugin/skills/beta"},
		{SkillName: "beta", Path: "plugin/skills/beta"},
		{SkillName: "alpha"},
	})
	if err != nil {
		t.Fatalf("CellsToRequests() error = %v", err)
	}
	if len(names) != 2 || len(explicit) != 1 || explicit["beta"] != "plugin/skills/beta" {
		t.Fatalf("names = %q explicit = %#v", names, explicit)
	}

	if _, _, err := CellsToRequests([]InstallCell{{SkillName: "beta", Path: "a"}, {SkillName: "beta", Path: "b"}}); err == nil {
		t.Fatal("CellsToRequests(conflicting paths) error = nil, want error")
	}
	if _, _, err := CellsToRequests([]InstallCell{{SkillName: "  "}}); err == nil {
		t.Fatal("CellsToRequests(empty name) error = nil, want error")
	}
	if _, _, err := CellsToRequests([]InstallCell{{SkillName: "beta", Path: "../escape"}}); err == nil {
		t.Fatal("CellsToRequests(escaping path) error = nil, want error")
	}
}

func TestRecordedSkillPaths(t *testing.T) {
	manifest := state.Manifest{
		Repositories: []state.RepositoryEntry{{
			Host:     "github.com",
			RepoPath: "owner/repo",
			InstalledSkills: []state.InstalledSkillEntry{
				{Name: "beta", RelativePath: ".agent/skills/beta"},
			},
		}},
		LocalSources: []state.LocalSourceEntry{{
			CanonicalPath: "/tmp/source",
			InstalledSkills: []state.InstalledSkillEntry{
				{Name: "alpha", RelativePath: "skills/alpha"},
			},
		}},
	}
	git := RecordedGitSkillPaths(manifest, "github.com", "owner/repo")
	if git["beta"] != ".agent/skills/beta" {
		t.Fatalf("git paths = %#v", git)
	}
	if len(RecordedGitSkillPaths(manifest, "github.com", "other/repo")) != 0 {
		t.Fatal("other repository must not seed choices")
	}
	local := RecordedLocalSkillPaths(manifest, "/tmp/source")
	if local["alpha"] != "skills/alpha" {
		t.Fatalf("local paths = %#v", local)
	}
}
