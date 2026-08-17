package advisor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dees91/agent-skill-manager/internal/model"
	"github.com/dees91/agent-skill-manager/internal/paths"
)

func TestStoreRejectsUnknownVersionAndUnsafePath(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	if err := os.MkdirAll(p.StateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.AdvisorFile, []byte(`{"version":99,"receipts":[],"leases":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := newStore(p).load(); err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("load() error = %v, want unsupported version", err)
	}

	if err := os.Remove(p.AdvisorFile); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(p.AdvisorFile, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := newStore(p).load(); err == nil || !strings.Contains(err.Error(), "expected a regular file") {
		t.Fatalf("load() unsafe path error = %v", err)
	}
}

func TestStoreRejectsTrailingJSON(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	if err := os.MkdirAll(p.StateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.AdvisorFile, []byte(`{"version":1,"receipts":[],"leases":[]} {}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := newStore(p).load(); err == nil || !strings.Contains(err.Error(), "trailing JSON") {
		t.Fatalf("load() error = %v, want trailing JSON", err)
	}
}

func TestStoreRejectsInconsistentClaims(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	contents := emptyFile()
	contents.Receipts = append(contents.Receipts, receipt{
		ID: receiptA, Tool: model.ToolCodex, CreatedAt: time.Now().UTC(), Skills: []string{"orphan"},
	})
	if _, err := validateFile(contents, p); err == nil || !strings.Contains(err.Error(), "inconsistent claim") {
		t.Fatalf("validateFile() error = %v, want inconsistent claim", err)
	}
}

func TestLockRejectsSymlink(t *testing.T) {
	p := paths.ForHome(t.TempDir())
	if err := os.MkdirAll(p.StateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(p.StateDir, "target")
	if err := os.WriteFile(target, []byte("target"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, p.AdvisorLockFile); err != nil {
		t.Fatal(err)
	}
	if _, err := newStore(p).lock(true); err == nil {
		t.Fatal("lock() error = nil, want symlink rejection")
	}
}
