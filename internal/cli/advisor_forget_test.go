package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dees91/agent-skill-manager/internal/model"
	"github.com/dees91/agent-skill-manager/internal/paths"
	"github.com/dees91/agent-skill-manager/internal/state"
)

// advisor forget refuses a receipt that cleanup can release, and forgets it
// once the activated skill was removed outside Skill Manager.
func TestRunAdvisorForgetOnlyForgetsBlockedReceipt(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	disabledPath := filepath.Join(p.CodexDisabledDir, "alpha")
	mkdirSkill(t, disabledPath)
	saveState(t, p, state.Manifest{Disabled: []state.DisabledEntry{{
		Tool: model.ToolCodex, SkillName: "alpha", OriginalPath: filepath.Join(p.CodexUserSkills, "alpha"),
		DisabledPath: disabledPath, EntryType: model.EntryTypeDir, Source: model.SourceLocal, Group: model.GroupLocal,
	}}})
	var stdout, stderr strings.Builder
	if code := RunWithPaths([]string{"advisor", "activate", "--tool", "codex", "--skill", "alpha", "--json"}, &stdout, &stderr, p); code != 0 {
		t.Fatalf("activate code=%d stderr=%q", code, stderr.String())
	}
	var activation struct {
		ReceiptID string `json:"receiptId"`
	}
	if err := json.Unmarshal([]byte(stdout.String()), &activation); err != nil {
		t.Fatal(err)
	}

	stdout.Reset()
	stderr.Reset()
	if code := RunWithPaths([]string{"advisor", "forget", "--receipt", activation.ReceiptID, "--json"}, &stdout, &stderr, p); code == 0 {
		t.Fatalf("forget of a releasable receipt code = 0, stdout=%q", stdout.String())
	}
	if !strings.Contains(stderr.String(), `"code": "FORGET_FAILED"`) || !strings.Contains(stderr.String(), "advisor cleanup") {
		t.Fatalf("stderr = %q, want FORGET_FAILED pointing to advisor cleanup", stderr.String())
	}

	if err := os.RemoveAll(filepath.Join(p.CodexUserSkills, "alpha")); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	if code := RunWithPaths([]string{"advisor", "forget", "--receipt", activation.ReceiptID}, &stdout, &stderr, p); code != 0 {
		t.Fatalf("forget code=%d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "forgot receipt: "+activation.ReceiptID) || !strings.Contains(stdout.String(), "codex/alpha") {
		t.Fatalf("stdout = %q", stdout.String())
	}
	stdout.Reset()
	if code := RunWithPaths([]string{"advisor", "status", "--json"}, &stdout, &stderr, p); code != 0 || strings.Contains(stdout.String(), activation.ReceiptID) {
		t.Fatalf("status after forget = %q", stdout.String())
	}
}
