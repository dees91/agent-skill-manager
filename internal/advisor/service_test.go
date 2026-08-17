package advisor

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dees91/agent-skill-manager/internal/model"
	"github.com/dees91/agent-skill-manager/internal/ops"
	"github.com/dees91/agent-skill-manager/internal/paths"
	"github.com/dees91/agent-skill-manager/internal/state"
)

const (
	receiptA = "11111111111111111111111111111111"
	receiptB = "22222222222222222222222222222222"
)

func TestActivateAndCleanupRestoresDisabledSkill(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	disableFixture(t, p, model.ToolCodex, "ffmpeg", model.EntryTypeDir, "")
	service := fixedService(p, receiptA)

	activated, err := service.Activate(model.ToolCodex, []string{"ffmpeg"}, false)
	if err != nil {
		t.Fatalf("Activate() error = %v", err)
	}
	if activated.ReceiptID != receiptA || len(activated.Actions) != 1 || activated.Actions[0].Action != ActionEnable {
		t.Fatalf("Activate() = %#v", activated)
	}
	assertExists(t, filepath.Join(p.CodexUserSkills, "ffmpeg", "SKILL.md"))
	assertMissing(t, filepath.Join(p.CodexDisabledDir, "ffmpeg"))
	assertMode(t, p.AdvisorFile, 0o600)

	status, err := service.Status(nil)
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if len(status.Receipts) != 1 || status.Receipts[0].ReceiptID != receiptA {
		t.Fatalf("Status() = %#v", status)
	}

	cleaned, err := service.Cleanup(receiptA, false)
	if err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}
	if len(cleaned.Actions) != 1 || cleaned.Actions[0].Action != ActionDisable {
		t.Fatalf("Cleanup() = %#v", cleaned)
	}
	assertMissing(t, filepath.Join(p.CodexUserSkills, "ffmpeg"))
	assertExists(t, filepath.Join(p.CodexDisabledDir, "ffmpeg", "SKILL.md"))
	status, err = service.Status(nil)
	if err != nil || len(status.Receipts) != 0 {
		t.Fatalf("Status() after cleanup = %#v, %v", status, err)
	}
	backups, err := filepath.Glob(filepath.Join(p.BackupDir, backupPrefix+"*"+backupSuffix))
	if err != nil || len(backups) != 1 {
		t.Fatalf("advisor backups = %v, %v; want one", backups, err)
	}
}

func TestActivateLeavesPreexistingOnSkillUnowned(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	makeSkill(t, filepath.Join(p.ClaudeUserSkills, "remotion"))
	service := fixedService(p, receiptA)

	result, err := service.Activate(model.ToolClaude, []string{"remotion"}, false)
	if err != nil {
		t.Fatalf("Activate() error = %v", err)
	}
	if result.ReceiptID != "" || len(result.Actions) != 1 || result.Actions[0].Action != ActionAlreadyOn {
		t.Fatalf("Activate() = %#v", result)
	}
	assertMissing(t, p.AdvisorFile)
	assertExists(t, filepath.Join(p.ClaudeUserSkills, "remotion", "SKILL.md"))
}

func TestSharedReceiptsKeepSkillOnUntilLastCleanup(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	disableFixture(t, p, model.ToolCodex, "video-toolkit", model.EntryTypeDir, "")
	first := fixedService(p, receiptA)
	second := fixedService(p, receiptB)

	if _, err := first.Activate(model.ToolCodex, []string{"video-toolkit"}, false); err != nil {
		t.Fatalf("first Activate() error = %v", err)
	}
	shared, err := second.Activate(model.ToolCodex, []string{"video-toolkit"}, false)
	if err != nil {
		t.Fatalf("second Activate() error = %v", err)
	}
	if shared.Actions[0].Action != ActionShare {
		t.Fatalf("second Activate() action = %q, want share", shared.Actions[0].Action)
	}

	released, err := first.Cleanup(receiptA, false)
	if err != nil {
		t.Fatalf("first Cleanup() error = %v", err)
	}
	if released.Actions[0].Action != ActionRelease {
		t.Fatalf("first Cleanup() action = %q, want release", released.Actions[0].Action)
	}
	assertExists(t, filepath.Join(p.CodexUserSkills, "video-toolkit", "SKILL.md"))

	disabled, err := second.Cleanup(receiptB, false)
	if err != nil {
		t.Fatalf("second Cleanup() error = %v", err)
	}
	if disabled.Actions[0].Action != ActionDisable {
		t.Fatalf("second Cleanup() action = %q, want disable", disabled.Actions[0].Action)
	}
	assertExists(t, filepath.Join(p.CodexDisabledDir, "video-toolkit", "SKILL.md"))
}

func TestActivateDryRunDoesNotMutate(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	disableFixture(t, p, model.ToolClaude, "review", model.EntryTypeDir, "")
	before := readFile(t, p.StateFile)

	result, err := fixedService(p, receiptA).Activate(model.ToolClaude, []string{"review"}, true)
	if err != nil {
		t.Fatalf("Activate(dry-run) error = %v", err)
	}
	if !result.DryRun || result.ReceiptID != "" || result.Actions[0].Action != ActionEnable {
		t.Fatalf("Activate(dry-run) = %#v", result)
	}
	assertMissing(t, p.AdvisorFile)
	assertMissing(t, filepath.Join(p.ClaudeUserSkills, "review"))
	if after := readFile(t, p.StateFile); string(after) != string(before) {
		t.Fatalf("state changed during dry-run\nafter=%s\nbefore=%s", after, before)
	}
}

func TestActivateValidatesSelectionBeforeMutation(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	disableFixture(t, p, model.ToolCodex, "valid", model.EntryTypeDir, "")
	service := fixedService(p, receiptA)

	tests := []struct {
		name   string
		skills []string
		want   string
	}{
		{name: "empty", skills: nil, want: "requires 1-5"},
		{name: "duplicate", skills: []string{"valid", "valid"}, want: "duplicate"},
		{name: "too many", skills: []string{"a", "b", "c", "d", "e", "f"}, want: "requires 1-5"},
		{name: "missing alongside valid", skills: []string{"valid", "missing"}, want: "not installed"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := service.Activate(model.ToolCodex, test.skills, false)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Activate() error = %v, want %q", err, test.want)
			}
			assertMissing(t, filepath.Join(p.CodexUserSkills, "valid"))
			assertExists(t, filepath.Join(p.CodexDisabledDir, "valid", "SKILL.md"))
			assertMissing(t, p.AdvisorFile)
		})
	}
}

func TestActivateRejectsConflictAndReadOnlyCells(t *testing.T) {
	t.Run("conflict", func(t *testing.T) {
		p := paths.ForHome(t.TempDir())
		disableFixture(t, p, model.ToolCodex, "conflict", model.EntryTypeDir, "")
		makeSkill(t, filepath.Join(p.CodexUserSkills, "conflict"))
		_, err := fixedService(p, receiptA).Activate(model.ToolCodex, []string{"conflict"}, false)
		if err == nil || !strings.Contains(err.Error(), "conflict") {
			t.Fatalf("Activate(conflict) error = %v", err)
		}
		assertMissing(t, p.AdvisorFile)
	})
	t.Run("read-only", func(t *testing.T) {
		p := paths.ForHome(t.TempDir())
		makeSkill(t, filepath.Join(p.CodexSystemSkills, "imagegen"))
		_, err := fixedService(p, receiptA).Activate(model.ToolCodex, []string{"imagegen"}, false)
		if err == nil || !strings.Contains(err.Error(), "not toggleable") {
			t.Fatalf("Activate(read-only) error = %v", err)
		}
		assertMissing(t, p.AdvisorFile)
	})
}

func TestCleanupAcceptsAlreadyRestoredSkill(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	disableFixture(t, p, model.ToolClaude, "docs", model.EntryTypeDir, "")
	service := fixedService(p, receiptA)
	if _, err := service.Activate(model.ToolClaude, []string{"docs"}, false); err != nil {
		t.Fatalf("Activate() error = %v", err)
	}
	operation, err := ops.New(p).PlanDisable(model.ToolClaude, "docs")
	if err != nil {
		t.Fatalf("PlanDisable() error = %v", err)
	}
	if result := ops.New(p).Apply([]model.PlannedOperation{operation}); result.Failed != nil {
		t.Fatalf("manual disable failed: %v", result.Failed.Err)
	}

	cleaned, err := service.Cleanup(receiptA, false)
	if err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}
	if cleaned.Actions[0].Action != ActionAlreadyOff {
		t.Fatalf("Cleanup() action = %q, want already_off", cleaned.Actions[0].Action)
	}
}

func TestCleanupDryRunKeepsReceiptAndSkillOn(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	disableFixture(t, p, model.ToolClaude, "research", model.EntryTypeDir, "")
	service := fixedService(p, receiptA)
	if _, err := service.Activate(model.ToolClaude, []string{"research"}, false); err != nil {
		t.Fatalf("Activate() error = %v", err)
	}
	before := readFile(t, p.AdvisorFile)

	result, err := service.Cleanup(receiptA, true)
	if err != nil {
		t.Fatalf("Cleanup(dry-run) error = %v", err)
	}
	if !result.DryRun || result.Actions[0].Action != ActionDisable {
		t.Fatalf("Cleanup(dry-run) = %#v", result)
	}
	assertExists(t, filepath.Join(p.ClaudeUserSkills, "research", "SKILL.md"))
	if after := readFile(t, p.AdvisorFile); string(after) != string(before) {
		t.Fatalf("advisor state changed during cleanup dry-run\nafter=%s\nbefore=%s", after, before)
	}
}

func TestCleanupBlocksChangedSymlink(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	firstTarget := filepath.Join(p.Home, "sources", "first")
	secondTarget := filepath.Join(p.Home, "sources", "second")
	makeSkill(t, firstTarget)
	makeSkill(t, secondTarget)
	disableFixture(t, p, model.ToolCodex, "linked", model.EntryTypeSymlink, firstTarget)
	service := fixedService(p, receiptA)
	if _, err := service.Activate(model.ToolCodex, []string{"linked"}, false); err != nil {
		t.Fatalf("Activate() error = %v", err)
	}
	activePath := filepath.Join(p.CodexUserSkills, "linked")
	if err := os.Remove(activePath); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secondTarget, activePath); err != nil {
		t.Fatal(err)
	}

	_, err := service.Cleanup(receiptA, false)
	if err == nil || !strings.Contains(err.Error(), "drift") {
		t.Fatalf("Cleanup() error = %v, want drift", err)
	}
	status, statusErr := service.Status(nil)
	if statusErr != nil || len(status.Receipts) != 1 {
		t.Fatalf("Status() = %#v, %v; receipt should remain", status, statusErr)
	}
	gotTarget, err := os.Readlink(activePath)
	if err != nil || gotTarget != secondTarget {
		t.Fatalf("active symlink target = %q, %v; want %q", gotTarget, err, secondTarget)
	}
}

func TestCleanupBlocksChangedSymlinkWithSharedReceipts(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	firstTarget := filepath.Join(p.Home, "sources", "first")
	secondTarget := filepath.Join(p.Home, "sources", "second")
	makeSkill(t, firstTarget)
	makeSkill(t, secondTarget)
	disableFixture(t, p, model.ToolCodex, "shared-linked", model.EntryTypeSymlink, firstTarget)
	first := fixedService(p, receiptA)
	second := fixedService(p, receiptB)
	if _, err := first.Activate(model.ToolCodex, []string{"shared-linked"}, false); err != nil {
		t.Fatalf("first Activate() error = %v", err)
	}
	if _, err := second.Activate(model.ToolCodex, []string{"shared-linked"}, false); err != nil {
		t.Fatalf("second Activate() error = %v", err)
	}
	activePath := filepath.Join(p.CodexUserSkills, "shared-linked")
	if err := os.Remove(activePath); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secondTarget, activePath); err != nil {
		t.Fatal(err)
	}

	_, err := first.Cleanup(receiptA, false)
	if err == nil || !strings.Contains(err.Error(), "drift") {
		t.Fatalf("Cleanup() error = %v, want drift", err)
	}
	status, statusErr := first.Status(nil)
	if statusErr != nil || len(status.Receipts) != 2 {
		t.Fatalf("Status() = %#v, %v; both receipts should remain", status, statusErr)
	}
}

func TestActivateIsToolSpecific(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	disableFixture(t, p, model.ToolClaude, "shared-name", model.EntryTypeDir, "")
	disableFixture(t, p, model.ToolCodex, "shared-name", model.EntryTypeDir, "")
	if _, err := fixedService(p, receiptA).Activate(model.ToolCodex, []string{"shared-name"}, false); err != nil {
		t.Fatalf("Activate() error = %v", err)
	}
	assertExists(t, filepath.Join(p.CodexUserSkills, "shared-name", "SKILL.md"))
	assertMissing(t, filepath.Join(p.ClaudeUserSkills, "shared-name"))
	assertExists(t, filepath.Join(p.ClaudeDisabledDir, "shared-name", "SKILL.md"))
}

func TestConcurrentActivationsShareOneLease(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	disableFixture(t, p, model.ToolCodex, "parallel", model.EntryTypeDir, "")
	services := []*Service{fixedService(p, receiptA), fixedService(p, receiptB)}
	type outcome struct {
		result ActivateResult
		err    error
	}
	outcomes := make(chan outcome, len(services))
	var start sync.WaitGroup
	start.Add(1)
	for _, service := range services {
		go func(current *Service) {
			start.Wait()
			result, err := current.Activate(model.ToolCodex, []string{"parallel"}, false)
			outcomes <- outcome{result: result, err: err}
		}(service)
	}
	start.Done()
	actions := map[string]int{}
	for range services {
		outcome := <-outcomes
		if outcome.err != nil {
			t.Fatalf("concurrent Activate() error = %v", outcome.err)
		}
		actions[outcome.result.Actions[0].Action]++
	}
	if actions[ActionEnable] != 1 || actions[ActionShare] != 1 {
		t.Fatalf("concurrent actions = %#v, want one enable and one share", actions)
	}
	status, err := services[0].Status(nil)
	if err != nil || len(status.Receipts) != 2 {
		t.Fatalf("Status() = %#v, %v; want two receipts", status, err)
	}
}

func fixedService(p paths.Paths, id string) *Service {
	service := New(p)
	service.now = func() time.Time { return time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC) }
	service.store.now = service.now
	service.newID = func() (string, error) { return id, nil }
	return service
}

func disableFixture(t *testing.T, p paths.Paths, tool model.Tool, name string, entryType model.EntryType, symlinkTarget string) {
	t.Helper()
	disabledDir, _ := p.DisabledDirFor(tool)
	activeDir, _ := p.UserSkillsDirFor(tool)
	disabledPath := filepath.Join(disabledDir, name)
	if entryType == model.EntryTypeSymlink {
		if err := os.MkdirAll(filepath.Dir(disabledPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(symlinkTarget, disabledPath); err != nil {
			t.Fatal(err)
		}
	} else {
		makeSkill(t, disabledPath)
	}
	store := state.New(p)
	manifest, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	manifest.Upsert(state.DisabledEntry{
		Tool:          tool,
		SkillName:     name,
		OriginalPath:  filepath.Join(activeDir, name),
		DisabledPath:  disabledPath,
		EntryType:     entryType,
		SymlinkTarget: symlinkTarget,
		Source:        model.SourceLocal,
		Group:         model.GroupLocal,
		DisabledAt:    time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC),
	})
	if err := store.Save(manifest); err != nil {
		t.Fatal(err)
	}
}

func makeSkill(t *testing.T, directory string) {
	t.Helper()
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	contents := "---\nname: " + filepath.Base(directory) + "\ndescription: Test skill\n---\n"
	if err := os.WriteFile(filepath.Join(directory, "SKILL.md"), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); err != nil {
		t.Fatalf("expected %s to exist: %v", path, err)
	}
}

func assertMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be missing, got %v", path, err)
	}
}

func assertMode(t *testing.T, path string, mode os.FileMode) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != mode {
		t.Fatalf("mode(%s) = %o, want %o", path, got, mode)
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return contents
}
