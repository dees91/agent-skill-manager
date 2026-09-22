// Package gittest configures git child processes started by tests.
package gittest

import (
	"os"
	"strconv"
)

// DisableAutoMaintenance sets maintenance.auto=false for every git process
// the test binary starts. Otherwise commit, merge, and pull spawn a detached
// "git maintenance run --auto" that briefly creates .git/objects/maintenance.lock
// after the command returns, racing tests that snapshot repository trees.
func DisableAutoMaintenance() {
	count, err := strconv.Atoi(os.Getenv("GIT_CONFIG_COUNT"))
	if err != nil || count < 0 {
		count = 0
	}
	index := strconv.Itoa(count)
	os.Setenv("GIT_CONFIG_KEY_"+index, "maintenance.auto")
	os.Setenv("GIT_CONFIG_VALUE_"+index, "false")
	os.Setenv("GIT_CONFIG_COUNT", strconv.Itoa(count+1))
}
