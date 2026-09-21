package gui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dees91/agent-skill-manager/internal/paths"
)

// installFirstAlpha records first-pack as the owner of alpha for Claude.
func installFirstAlpha(t *testing.T, service *Service, p paths.Paths) {
	t.Helper()
	first := filepath.Join(p.Home, "workspace", "first-pack")
	writeSkill(t, filepath.Join(first, "skills", "alpha"), "Alpha one")
	draft, err := service.PrepareLocalInstall(first)
	if err != nil {
		t.Fatal(err)
	}
	review, err := service.ReviewInstall(draft.DraftID, []InstallCellRequest{{SkillName: "alpha", Tool: "claude"}}, false)
	if err != nil || !review.Ready {
		t.Fatalf("first review = %#v err=%v", review, err)
	}
	if result := service.ApplyInstall(review.ReviewID, false); result.Failure != nil {
		t.Fatalf("first apply = %#v", result.Failure)
	}
}

func TestLocalInstallNeedsNameDraftReviewAndApplyWithAlias(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	service := New(p)
	installFirstAlpha(t, service, p)
	second := filepath.Join(p.Home, "workspace", "second-pack")
	writeSkill(t, filepath.Join(second, "skills", "alpha"), "Alpha two")
	writeSkill(t, filepath.Join(second, "skills", "beta"), "Beta")

	draft, err := service.PrepareLocalInstall(second)
	if err != nil {
		t.Fatal(err)
	}
	alpha, beta := draft.Candidates[0], draft.Candidates[1]
	if !alpha.NeedsName || alpha.SuggestedAs != "alpha-second-pack" {
		t.Fatalf("alpha = %#v, want needs-name with alpha-second-pack", alpha)
	}
	for _, cell := range []InstallCandidateCell{alpha.Claude, alpha.Codex, alpha.Muse, alpha.Grok} {
		if cell.Status != "needs-name" || cell.Message != "choose an install name; alpha is owned by first-pack" || strings.Contains(cell.Message, p.Home) {
			t.Fatalf("alpha cell = %#v, want path-free needs-name", cell)
		}
	}
	if beta.NeedsName || beta.Claude.Status != "available" {
		t.Fatalf("beta = %#v, want plain available row", beta)
	}

	rejected := map[string][]InstallCellRequest{
		"is not needed":             {{SkillName: "beta", Tool: "claude", InstalledAs: "beta-second-pack"}},
		"must differ":               {{SkillName: "alpha", Tool: "claude", InstalledAs: "beta"}},
		"invalid install name":      {{SkillName: "alpha", Tool: "claude", InstalledAs: "Alpha_Two"}},
		"conflicting install names": {{SkillName: "alpha", Tool: "claude", InstalledAs: "alpha-two"}, {SkillName: "alpha", Tool: "codex", InstalledAs: "alpha-other"}},
	}
	for want, selections := range rejected {
		if _, err := service.ReviewInstall(draft.DraftID, selections, false); err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("ReviewInstall(%#v) error = %v, want %q", selections, err, want)
		}
	}
	plain, err := service.ReviewInstall(draft.DraftID, []InstallCellRequest{{SkillName: "alpha", Tool: "claude"}}, false)
	if err != nil || plain.Ready || len(plain.Conflicts) != 1 || plain.Conflicts[0].SuggestedAs != "alpha-second-pack" {
		t.Fatalf("plain review = %#v err=%v, want blocked with suggestion", plain, err)
	}

	review, err := service.ReviewInstall(draft.DraftID, []InstallCellRequest{
		{SkillName: "alpha", Tool: "claude", InstalledAs: "alpha-second-pack"},
		{SkillName: "alpha", Tool: "codex", InstalledAs: "alpha-second-pack"},
	}, false)
	if err != nil || !review.Ready || review.CreateCount != 2 || review.Selections[0].InstalledAs != "alpha-second-pack" {
		t.Fatalf("review = %#v err=%v", review, err)
	}
	result := service.ApplyInstall(review.ReviewID, false)
	if result.Failure != nil || result.CreatedLinks != 2 {
		t.Fatalf("apply = %#v", result)
	}
	for _, dir := range []string{p.ClaudeUserSkills, p.CodexUserSkills} {
		if _, err := os.Readlink(filepath.Join(dir, "alpha-second-pack")); err != nil {
			t.Fatalf("alias link missing in %s: %v", dir, err)
		}
	}

	again, err := service.PrepareLocalInstall(second)
	if err != nil {
		t.Fatal(err)
	}
	if again.Candidates[0].NeedsName || again.Candidates[0].InstalledAs != "alpha-second-pack" || again.Candidates[0].Claude.Status != "already-on" {
		t.Fatalf("recorded alpha = %#v, want recorded alias already on", again.Candidates[0])
	}
}

func TestLocalInstallApplyRevalidatesAliasPath(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	service := New(p)
	installFirstAlpha(t, service, p)
	second := filepath.Join(p.Home, "workspace", "second-pack")
	writeSkill(t, filepath.Join(second, "skills", "alpha"), "Alpha two")
	draft, err := service.PrepareLocalInstall(second)
	if err != nil {
		t.Fatal(err)
	}
	review, err := service.ReviewInstall(draft.DraftID, []InstallCellRequest{{SkillName: "alpha", Tool: "claude", InstalledAs: "alpha-second-pack"}}, false)
	if err != nil || !review.Ready {
		t.Fatalf("review = %#v err=%v", review, err)
	}
	if err := os.MkdirAll(filepath.Join(p.ClaudeUserSkills, "alpha-second-pack"), 0o755); err != nil {
		t.Fatal(err)
	}
	result := service.ApplyInstall(review.ReviewID, false)
	if result.Failure == nil || !strings.Contains(result.Failure.Message, "install preflight changed: target path already exists") {
		t.Fatalf("result = %#v, want revalidation failure", result.Failure)
	}
}
