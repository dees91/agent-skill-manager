package advisor

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dees91/agent-skill-manager/internal/model"
	"github.com/dees91/agent-skill-manager/internal/paths"
)

// driftedReceipt activates a symlinked Codex skill under each receipt and then
// repoints the active link, so cleanup of every receipt is blocked by drift.
func driftedReceipt(t *testing.T, p paths.Paths, name string, receipts ...string) string {
	t.Helper()
	firstTarget := filepath.Join(p.Home, "sources", "first")
	secondTarget := filepath.Join(p.Home, "sources", "second")
	makeSkill(t, firstTarget)
	makeSkill(t, secondTarget)
	disableFixture(t, p, model.ToolCodex, name, model.EntryTypeSymlink, firstTarget)
	for _, id := range receipts {
		if _, err := fixedService(p, id).Activate(model.ToolCodex, []string{name}, false); err != nil {
			t.Fatalf("Activate(%s) error = %v", id, err)
		}
	}
	activePath := filepath.Join(p.CodexUserSkills, name)
	if err := os.Remove(activePath); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secondTarget, activePath); err != nil {
		t.Fatal(err)
	}
	return secondTarget
}

func TestCleanupReportsBlockedReceiptWithoutPaths(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	driftedReceipt(t, p, "linked", receiptA)

	_, err := fixedService(p, receiptA).Cleanup(receiptA, true)

	var blocked CleanupBlockedError
	if !errors.As(err, &blocked) {
		t.Fatalf("Cleanup() error = %v, want CleanupBlockedError", err)
	}
	if blocked.Tool != model.ToolCodex || blocked.Skill != "linked" || blocked.Reason != BlockedDrift {
		t.Fatalf("blocked = %#v", blocked)
	}
	if strings.Contains(blocked.Cause(), p.Home) {
		t.Fatalf("Cause() = %q, want no filesystem path", blocked.Cause())
	}
}

func TestForgetBlockedReceiptKeepsLinksAndState(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	target := driftedReceipt(t, p, "linked", receiptA)
	stateBefore := readFile(t, p.StateFile)
	service := fixedService(p, receiptA)

	dry, err := service.Forget(receiptA, true)
	if err != nil || !dry.DryRun || strings.Join(dry.Skills, ",") != "linked" {
		t.Fatalf("Forget(dry-run) = %#v, %v", dry, err)
	}
	if status, _ := service.Status(nil); len(status.Receipts) != 1 {
		t.Fatalf("dry-run removed the receipt: %#v", status)
	}

	forgotten, err := service.Forget(receiptA, false)
	if err != nil {
		t.Fatalf("Forget() error = %v", err)
	}
	if forgotten.Tool != model.ToolCodex || strings.Join(forgotten.Skills, ",") != "linked" {
		t.Fatalf("Forget() = %#v", forgotten)
	}
	status, err := service.Status(nil)
	if err != nil || len(status.Receipts) != 0 {
		t.Fatalf("Status() = %#v, %v; want no receipts", status, err)
	}
	if got, err := os.Readlink(filepath.Join(p.CodexUserSkills, "linked")); err != nil || got != target {
		t.Fatalf("active link = %q, %v; want unchanged %q", got, err, target)
	}
	if string(readFile(t, p.StateFile)) != string(stateBefore) {
		t.Fatal("Forget() changed state.json")
	}
	if strings.Contains(string(readFile(t, p.AdvisorFile)), `"linked"`) {
		t.Fatal("lease without claims remains in the advisor file")
	}
}

func TestForgetSharedLeaseRemovesOnlyThatClaim(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	driftedReceipt(t, p, "shared-linked", receiptA, receiptB)

	if _, err := fixedService(p, receiptA).Forget(receiptA, false); err != nil {
		t.Fatalf("Forget() error = %v", err)
	}

	status, err := fixedService(p, receiptB).Status(nil)
	if err != nil || len(status.Receipts) != 1 || status.Receipts[0].ReceiptID != receiptB {
		t.Fatalf("Status() = %#v, %v; want only receipt B", status, err)
	}
	if _, err := fixedService(p, receiptB).Forget(receiptB, false); err != nil {
		t.Fatalf("Forget(B) error = %v, want the remaining claim to be forgettable", err)
	}
}

func TestForgetRefusesReceiptThatCleanupCanRelease(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	disableFixture(t, p, model.ToolCodex, "ffmpeg", model.EntryTypeDir, "")
	service := fixedService(p, receiptA)
	if _, err := service.Activate(model.ToolCodex, []string{"ffmpeg"}, false); err != nil {
		t.Fatalf("Activate() error = %v", err)
	}

	_, err := service.Forget(receiptA, false)

	if err == nil || !strings.Contains(err.Error(), "advisor cleanup") {
		t.Fatalf("Forget() error = %v, want a pointer to advisor cleanup", err)
	}
	if status, _ := service.Status(nil); len(status.Receipts) != 1 {
		t.Fatalf("Forget() removed a releasable receipt: %#v", status)
	}
}
