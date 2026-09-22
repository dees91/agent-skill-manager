package gittest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDisableAutoMaintenanceStopsCommitFromStartingMaintenance(t *testing.T) {
	DisableAutoMaintenance()
	dir := t.TempDir()
	run := func(env []string, args ...string) string {
		t.Helper()
		command := exec.Command("git", append([]string{"-C", dir}, args...)...)
		command.Env = append(os.Environ(), env...)
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
		return string(output)
	}
	run(nil, "init", "--quiet")
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("alpha\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	run(nil, "add", ".")

	trace := run([]string{"GIT_TRACE=1"}, "-c", "user.email=t@example.test", "-c", "user.name=t", "-c", "commit.gpgsign=false", "commit", "--quiet", "-m", "alpha")

	if strings.Contains(trace, "maintenance run") {
		t.Fatalf("git commit started background maintenance:\n%s", trace)
	}
}
