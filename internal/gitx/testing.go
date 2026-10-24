package gitx

import (
	"os"
	"testing"
)

// RequireGit skips the test when git is missing, or fails it when
// JIRAFS_REQUIRE_GIT=1 so that bin/test always exercises git tests.
func RequireGit(t *testing.T) {
	t.Helper()
	if Available() {
		return
	}
	if os.Getenv("JIRAFS_REQUIRE_GIT") == "1" {
		t.Fatal("git is required (JIRAFS_REQUIRE_GIT=1) but not on PATH")
	}
	t.Skip("git not available")
}

// HermeticEnv points git at an empty config and a fixed identity for the
// duration of the test, so repos created by tests never read user config.
func HermeticEnv(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_AUTHOR_NAME", "Test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "Test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.com")
}
