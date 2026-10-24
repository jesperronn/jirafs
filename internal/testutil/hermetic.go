package testutil

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"net/http"

	"github.com/jirafs/jirafs/internal/netguard"
)

// RunHermetic is meant to be called from TestMain(m). It points HOME and
// XDG_CONFIG_HOME at a fresh temp dir, clears every JIRAFS_* variable (notably
// JIRAFS_ALLOW_LIVE_ENDPOINTS), runs the tests and cleans up, so no test can
// read real user settings or resolve real credentials.
func RunHermetic(m *testing.M) {
	os.Exit(RunHermeticCode(m))
}

// RunHermeticCode is RunHermetic without the final os.Exit: it returns the
// exit code so a TestMain can run extra checks after the tests finish.
func RunHermeticCode(m *testing.M) int {
	if os.Getenv("GO_WANT_HELPER_PROCESS") == "1" {
		// Re-exec'd helper child: the parent already chose a hermetic HOME.
		return m.Run()
	}
	dir, err := os.MkdirTemp("", "jirafs-test-home-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "testutil: cannot create temp HOME:", err)
		return 1
	}
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "JIRAFS_") {
			os.Unsetenv(strings.SplitN(kv, "=", 2)[0])
		}
	}
	for _, kv := range os.Environ() {
		k := strings.SplitN(kv, "=", 2)[0]
		if strings.HasPrefix(k, "OP_") {
			os.Unsetenv(k)
		}
	}
	for _, k := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "http_proxy", "https_proxy", "all_proxy"} {
		os.Unsetenv(k)
	}
	os.Setenv("NO_PROXY", "127.0.0.1,localhost,::1")
	os.Setenv("no_proxy", "127.0.0.1,localhost,::1")
	http.DefaultTransport = netguard.Wrap(http.DefaultTransport)
	os.Setenv("HOME", dir)
	os.Setenv("XDG_CONFIG_HOME", dir)
	code := m.Run()
	os.RemoveAll(dir)
	return code
}
