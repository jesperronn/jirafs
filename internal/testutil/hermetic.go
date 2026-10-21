package testutil

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// RunHermetic is meant to be called from TestMain(m). It points HOME and
// XDG_CONFIG_HOME at a fresh temp dir, clears every JIRAFS_* variable (notably
// JIRAFS_ALLOW_LIVE_ENDPOINTS), runs the tests and cleans up, so no test can
// read real user settings or resolve real credentials.
func RunHermetic(m *testing.M) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") == "1" {
		// Re-exec'd helper child: the parent already chose a hermetic HOME.
		os.Exit(m.Run())
	}
	dir, err := os.MkdirTemp("", "jirafs-test-home-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "testutil: cannot create temp HOME:", err)
		os.Exit(1)
	}
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "JIRAFS_") {
			os.Unsetenv(strings.SplitN(kv, "=", 2)[0])
		}
	}
	os.Setenv("HOME", dir)
	os.Setenv("XDG_CONFIG_HOME", dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
