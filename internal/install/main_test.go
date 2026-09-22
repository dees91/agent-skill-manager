package install

import (
	"os"
	"testing"

	"github.com/dees91/agent-skill-manager/internal/gittest"
)

func TestMain(m *testing.M) {
	gittest.DisableAutoMaintenance()
	os.Exit(m.Run())
}
