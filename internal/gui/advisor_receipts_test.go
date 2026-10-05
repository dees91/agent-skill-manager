package gui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dees91/agent-skill-manager/internal/advisor"
	"github.com/dees91/agent-skill-manager/internal/model"
	"github.com/dees91/agent-skill-manager/internal/paths"
	"github.com/dees91/agent-skill-manager/internal/state"
)

// The desktop receipts flow: the list previews each cleanup, Clean up all
// releases the healthy receipt and skips the blocked one, and Forget drops the
// blocked receipt without touching skill links.
func TestAdvisorReceiptsCleanUpAllSkipsBlockedAndForgetDropsIt(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	disabledAdvisorSkills(t, p, "alpha", "beta")
	healthy := activateAdvisorSkill(t, p, "alpha")
	blocked := activateAdvisorSkill(t, p, "beta")
	if err := os.RemoveAll(filepath.Join(p.CodexUserSkills, "beta")); err != nil {
		t.Fatal(err)
	}
	service := New(p)

	receipts, err := service.ListAdvisorReceipts()
	if err != nil || len(receipts) != 2 {
		t.Fatalf("ListAdvisorReceipts() = %#v, %v", receipts, err)
	}
	byID := map[string]AdvisorReceipt{}
	for _, receipt := range receipts {
		byID[receipt.ReceiptID] = receipt
	}
	if got := byID[healthy]; got.Blocked || got.Tool != "codex" || got.CreatedAt == "" || len(got.Skills) != 1 || got.Skills[0].Action != advisor.ActionDisable {
		t.Fatalf("healthy receipt = %#v", got)
	}
	if got := byID[blocked]; !got.Blocked || !strings.Contains(got.Cause, "codex/beta") || strings.Contains(got.Cause, p.Home) {
		t.Fatalf("blocked receipt = %#v, want a path-free cause", got)
	}

	cleaned := service.CleanupAllAdvisorReceipts(false)
	if cleaned.Failure != nil || strings.Join(cleaned.Cleaned, ",") != healthy {
		t.Fatalf("CleanupAllAdvisorReceipts() = %#v", cleaned)
	}
	if len(cleaned.Receipts) != 1 || cleaned.Receipts[0].ReceiptID != blocked {
		t.Fatalf("remaining receipts = %#v, want only the blocked one", cleaned.Receipts)
	}
	if _, err := os.Stat(filepath.Join(p.CodexDisabledDir, "alpha", "SKILL.md")); err != nil {
		t.Fatalf("alpha not restored to OFF: %v", err)
	}

	if refused := service.ForgetAdvisorReceipt(healthy, false); refused.Failure == nil {
		t.Fatalf("ForgetAdvisorReceipt(unknown) = %#v, want a failure", refused)
	}
	forgotten := service.ForgetAdvisorReceipt(blocked, false)
	if forgotten.Failure != nil || strings.Join(forgotten.Forgotten, ",") != blocked || len(forgotten.Receipts) != 0 {
		t.Fatalf("ForgetAdvisorReceipt() = %#v", forgotten)
	}
}

func disabledAdvisorSkills(t *testing.T, p paths.Paths, names ...string) {
	t.Helper()
	manifest := state.Manifest{}
	for _, name := range names {
		disabledPath := filepath.Join(p.CodexDisabledDir, name)
		writeSkill(t, disabledPath, "Skill "+name)
		manifest.Disabled = append(manifest.Disabled, state.DisabledEntry{
			Tool: model.ToolCodex, SkillName: name, OriginalPath: filepath.Join(p.CodexUserSkills, name),
			DisabledPath: disabledPath, EntryType: model.EntryTypeDir, Source: model.SourceLocal, Group: model.GroupLocal,
		})
	}
	if err := state.New(p).Save(manifest); err != nil {
		t.Fatalf("save state: %v", err)
	}
}

func activateAdvisorSkill(t *testing.T, p paths.Paths, name string) string {
	t.Helper()
	result, err := advisor.New(p).Activate(model.ToolCodex, []string{name}, false)
	if err != nil {
		t.Fatalf("Activate(%s) error = %v", name, err)
	}
	return result.ReceiptID
}
