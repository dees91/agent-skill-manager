package install

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dees91/agent-skill-manager/internal/model"
	"github.com/dees91/agent-skill-manager/internal/ops"
	"github.com/dees91/agent-skill-manager/internal/paths"
	"github.com/dees91/agent-skill-manager/internal/state"
)

func gitInitCheckout(t *testing.T, dir, originURL string) {
	t.Helper()
	steps := [][]string{
		{"init", "--quiet", "-b", "main"},
		{"add", "-A"},
		{"-c", "user.email=extend@test", "-c", "user.name=extend", "commit", "--quiet", "-m", "init"},
		{"remote", "add", "origin", originURL},
		{"config", "branch.main.remote", "origin"},
		{"config", "branch.main.merge", "refs/heads/main"},
		{"update-ref", "refs/remotes/origin/main", "HEAD"},
	}
	for _, args := range steps {
		command := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
		}
	}
}

func extendGitFixture(t *testing.T, p paths.Paths, rawURL string, names ...string) (RepoIdentity, string) {
	t.Helper()
	identity, err := NormalizeGitURL(rawURL)
	if err != nil {
		t.Fatalf("NormalizeGitURL() error = %v", err)
	}
	checkout, err := CheckoutPath(p, identity)
	if err != nil {
		t.Fatalf("CheckoutPath() error = %v", err)
	}
	for _, name := range names {
		writeSkill(t, filepath.Join(checkout, "skills", name))
	}
	return identity, checkout
}

func installClaudeOnly(t *testing.T, p paths.Paths, identity RepoIdentity, checkout string) state.Manifest {
	t.Helper()
	discovered, err := DiscoverSkills(checkout)
	if err != nil {
		t.Fatalf("DiscoverSkills() error = %v", err)
	}
	plan, err := PlanInstall(p, state.Manifest{}, identity, checkout, discovered, PlanOptions{Tools: []model.Tool{model.ToolClaude}})
	if err != nil {
		t.Fatalf("PlanInstall() error = %v", err)
	}
	if _, err := NewApplyService(p).Apply(plan, ""); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	return loadInstallManifest(t, p)
}

func disableForTool(t *testing.T, p paths.Paths, tool model.Tool, names ...string) {
	t.Helper()
	service := ops.New(p)
	operations := make([]model.PlannedOperation, 0, len(names))
	for _, name := range names {
		operation, err := service.PlanDisable(tool, name)
		if err != nil {
			t.Fatalf("PlanDisable(%s) error = %v", name, err)
		}
		operations = append(operations, operation)
	}
	if result := service.Apply(operations); result.Failed != nil {
		t.Fatalf("Apply() failed: %v", result.Failed.Err)
	}
}

func extendSourceByGroup(plan ExtendPlan, group string) ExtendSourcePlan {
	for _, source := range plan.Sources {
		if source.Group.String() == group {
			return source
		}
	}
	return ExtendSourcePlan{}
}

func TestPlanExtendSelectsCellsAndDisableAfter(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	identity, checkout := extendGitFixture(t, p, "https://github.com/owner/repo.git", "alpha", "beta")
	manifest := installClaudeOnly(t, p, identity, checkout)
	disableForTool(t, p, model.ToolClaude, "beta")
	manifest = loadInstallManifest(t, p)

	plan, err := PlanExtend(p, manifest, model.ToolMuse)
	if err != nil {
		t.Fatalf("PlanExtend() error = %v", err)
	}
	if len(plan.Sources) != 1 {
		t.Fatalf("Sources len = %d, want 1", len(plan.Sources))
	}
	source := plan.Sources[0]
	if source.Status != ExtendStatusReady {
		t.Fatalf("Status = %q, want ready", source.Status)
	}
	if len(source.Links) != 2 {
		t.Fatalf("Links len = %d, want 2", len(source.Links))
	}
	if len(source.DisableAfter) != 1 || source.DisableAfter[0] != "beta" {
		t.Fatalf("DisableAfter = %#v, want [beta]", source.DisableAfter)
	}
	if len(source.AlreadyInstalled) != 0 || len(source.Skipped) != 0 {
		t.Fatalf("plan = %#v, want no already-installed or skips", source)
	}
}

func TestPlanExtendSkips(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	identity, checkout := extendGitFixture(t, p, "https://github.com/owner/repo.git", "alpha")
	manifest := installClaudeOnly(t, p, identity, checkout)

	// Record a skill that no longer exists in the checkout.
	repo, ok := manifest.GetRepository(identity.Host, identity.RepoPath)
	if !ok {
		t.Fatal("repository manifest entry missing")
	}
	repo.InstalledSkills = append(repo.InstalledSkills, state.InstalledSkillEntry{
		Name:         "ghost",
		RelativePath: "skills/ghost",
		Tools:        []model.Tool{model.ToolClaude},
	})
	// Record a skill with no recorded tools.
	repo.InstalledSkills = append(repo.InstalledSkills, state.InstalledSkillEntry{
		Name:         "empty",
		RelativePath: "skills/alpha",
	})
	manifest.UpsertRepository(repo)

	// Record a second repository whose checkout directory is missing.
	goneIdentity, err := NormalizeGitURL("https://github.com/owner/gone")
	if err != nil {
		t.Fatalf("NormalizeGitURL() error = %v", err)
	}
	goneCheckout, err := CheckoutPath(p, goneIdentity)
	if err != nil {
		t.Fatalf("CheckoutPath() error = %v", err)
	}
	manifest.UpsertRepository(state.RepositoryEntry{
		OriginalURL:  goneIdentity.OriginalURL,
		CanonicalURL: goneIdentity.CanonicalURL,
		Host:         goneIdentity.Host,
		RepoPath:     goneIdentity.RepoPath,
		CheckoutPath: goneCheckout,
		Group:        goneIdentity.Group,
	})

	plan, err := PlanExtend(p, manifest, model.ToolMuse)
	if err != nil {
		t.Fatalf("PlanExtend() error = %v", err)
	}
	if len(plan.Sources) != 2 {
		t.Fatalf("Sources len = %d, want 2", len(plan.Sources))
	}
	ready := extendSourceByGroup(plan, "owner/repo")
	if ready.Status != ExtendStatusReady {
		t.Fatalf("repo Status = %q, want ready", ready.Status)
	}
	if len(ready.Skipped) != 2 {
		t.Fatalf("Skipped = %#v, want ghost and empty", ready.Skipped)
	}
	if ready.Skipped[0].SkillName != "empty" || ready.Skipped[0].Reason != "no recorded tools" {
		t.Fatalf("empty skip = %#v", ready.Skipped[0])
	}
	if ready.Skipped[1].SkillName != "ghost" || !strings.Contains(ready.Skipped[1].Reason, "not found in source at recorded path skills/ghost") {
		t.Fatalf("ghost skip = %#v", ready.Skipped[1])
	}
	gone := extendSourceByGroup(plan, "owner/gone")
	if gone.Status != ExtendStatusSkipped || gone.Reason == "" {
		t.Fatalf("gone = %#v, want skipped with reason", gone)
	}
}

func TestPlanExtendRejectsUnsupportedTool(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	if _, err := PlanExtend(p, state.Manifest{}, model.Tool("grok")); err == nil || !strings.Contains(err.Error(), `unsupported tool "grok"`) {
		t.Fatalf("PlanExtend() error = %v, want unsupported tool", err)
	}
}

func TestPlanExtendBlocksCrossSourceCollision(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	firstIdentity, firstCheckout := extendGitFixture(t, p, "https://github.com/owner/first.git", "shared")
	secondIdentity, secondCheckout := extendGitFixture(t, p, "https://github.com/owner/second.git", "shared")
	installClaudeOnly(t, p, firstIdentity, firstCheckout)
	manifest := loadInstallManifest(t, p)
	// Record the same skill name for the second source without touching the
	// occupied Claude target path.
	manifest.UpsertRepository(state.RepositoryEntry{
		OriginalURL:  secondIdentity.OriginalURL,
		CanonicalURL: secondIdentity.CanonicalURL,
		Host:         secondIdentity.Host,
		RepoPath:     secondIdentity.RepoPath,
		CheckoutPath: secondCheckout,
		Group:        secondIdentity.Group,
		InstalledSkills: []state.InstalledSkillEntry{
			{Name: "shared", RelativePath: "skills/shared", Tools: []model.Tool{model.ToolClaude}},
		},
	})
	if err := state.New(p).Save(manifest); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	manifest = loadInstallManifest(t, p)
	if len(manifest.Repositories) != 2 {
		t.Fatalf("Repositories len = %d, want 2", len(manifest.Repositories))
	}

	plan, err := PlanExtend(p, manifest, model.ToolMuse)
	if err != nil {
		t.Fatalf("PlanExtend() error = %v", err)
	}
	second := extendSourceByGroup(plan, "owner/second")
	if second.Status != ExtendStatusBlocked || second.Err == nil || !strings.Contains(second.Err.Error(), "is planned for owner/first") {
		t.Fatalf("second = %#v, want blocked on first group", second)
	}
}

func TestExtendServiceAppliesInOrderAndMirrorsOff(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	identity, checkout := extendGitFixture(t, p, "https://github.com/owner/repo.git", "alpha", "beta")
	gitInitCheckout(t, checkout, identity.OriginalURL)
	manifest := installClaudeOnly(t, p, identity, checkout)
	disableForTool(t, p, model.ToolClaude, "beta")

	result, err := NewExtendService(p).Apply(model.ToolMuse, nil)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if len(result.Completed) != 1 || result.Completed[0].Status != ExtendStatusExtended {
		t.Fatalf("Completed = %#v, want one extended source", result.Completed)
	}
	completed := result.Completed[0]
	if completed.Created != 2 || completed.Disabled != 1 {
		t.Fatalf("Completed = %#v, want 2 created and 1 disabled", completed)
	}
	assertSymlinkTarget(t, filepath.Join(p.MuseUserSkills, "alpha"), filepath.Join(checkout, "skills", "alpha"))
	manifest = loadInstallManifest(t, p)
	entry, ok := manifest.Get(model.ToolMuse, "beta")
	if !ok {
		t.Fatal("muse beta disabled entry missing")
	}
	if _, err := os.Lstat(filepath.Join(p.MuseUserSkills, "beta")); !os.IsNotExist(err) {
		t.Fatalf("muse beta still active: %v", err)
	}
	if entry.DisabledPath == "" {
		t.Fatalf("muse beta entry = %#v, want disabled path", entry)
	}

	if _, err := AuditRepositoryReferences(p, manifest, mustExtendRepo(t, manifest, identity)); err != nil {
		t.Fatalf("AuditRepositoryReferences() error = %v", err)
	}
	if _, err := NewUninstallService(p, nil).Plan(mustExtendRepo(t, manifest, identity)); err != nil {
		t.Fatalf("Uninstall Plan() error = %v", err)
	}

	rerun, err := NewExtendService(p).Apply(model.ToolMuse, nil)
	if err != nil {
		t.Fatalf("rerun Apply() error = %v", err)
	}
	created := 0
	for _, done := range rerun.Completed {
		created += done.Created
	}
	if created != 0 {
		t.Fatalf("rerun created = %d, want 0", created)
	}
}

func mustExtendRepo(t *testing.T, manifest state.Manifest, identity RepoIdentity) state.RepositoryEntry {
	t.Helper()
	repo, ok := manifest.GetRepository(identity.Host, identity.RepoPath)
	if !ok {
		t.Fatal("repository manifest entry missing")
	}
	return repo
}

func TestExtendServiceStopsAtFirstBlockedSource(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	firstIdentity, firstCheckout := extendGitFixture(t, p, "https://github.com/owner/first.git", "alpha")
	secondIdentity, secondCheckout := extendGitFixture(t, p, "https://github.com/owner/second.git", "gamma")
	installClaudeOnly(t, p, firstIdentity, firstCheckout)
	installClaudeOnly(t, p, secondIdentity, secondCheckout)
	_ = secondCheckout
	// A real directory blocks the gamma target for the second source.
	if err := os.MkdirAll(filepath.Join(p.MuseUserSkills, "gamma"), 0o755); err != nil {
		t.Fatalf("mkdir blocker: %v", err)
	}

	result, err := NewExtendService(p).Apply(model.ToolMuse, nil)
	var failure *ExtendFailure
	if !errors.As(err, &failure) {
		t.Fatalf("Apply() error = %v, want ExtendFailure", err)
	}
	if !strings.Contains(failure.Error(), "extend --tool muse failed for source owner/second:") {
		t.Fatalf("failure = %q, want extend owner/second", failure.Error())
	}
	if len(result.Completed) != 1 || result.Completed[0].Status != ExtendStatusExtended {
		t.Fatalf("Completed = %#v, want first-source prefix", result.Completed)
	}
	assertSymlinkTarget(t, filepath.Join(p.MuseUserSkills, "alpha"), filepath.Join(firstCheckout, "skills", "alpha"))
}

func TestExtendServiceLocalSource(t *testing.T) {
	p, source, discovered := localInstallFixture(t, "solo")
	manifest := state.Manifest{}
	plan, err := PlanLocalInstall(p, manifest, source, discovered, PlanOptions{Tools: []model.Tool{model.ToolClaude}})
	if err != nil {
		t.Fatalf("PlanLocalInstall() error = %v", err)
	}
	if _, err := NewLocalApplyService(p).Apply(plan); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	result, err := NewExtendService(p).Apply(model.ToolMuse, nil)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if len(result.Completed) != 1 || result.Completed[0].Status != ExtendStatusExtended || result.Completed[0].Created != 1 {
		t.Fatalf("Completed = %#v, want one extended local source", result.Completed)
	}
	manifest = loadInstallManifest(t, p)
	entry, ok := manifest.GetLocalSource(source.CanonicalPath)
	if !ok {
		t.Fatal("local source manifest entry missing")
	}
	if !hasRecordedTool(entry.InstalledSkills[0].Tools, model.ToolMuse) {
		t.Fatalf("installed tools = %#v, want muse recorded", entry.InstalledSkills[0].Tools)
	}
	assertSymlinkTarget(t, filepath.Join(p.MuseUserSkills, "solo"), filepath.Join(source.CanonicalPath, "skills", "solo"))
}

func TestFindExtraManagedReferencesReportsMuseLinks(t *testing.T) {
	p, repository, skillPaths := referenceRepositoryFixture(t, []string{"alpha"})
	mustSymlink(t, skillPaths["alpha"], filepath.Join(p.MuseUserSkills, "stray"))

	_, err := AuditRepositoryReferences(p, state.Manifest{}, repository)
	if err == nil || !strings.Contains(err.Error(), "extra") {
		t.Fatalf("AuditRepositoryReferences() error = %v, want extra muse reference", err)
	}
}
