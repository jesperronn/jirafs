// Package testutil provides fixture loading and golden-test helpers for jirafs tests.
package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

// TestFixtures returns a Fixtures instance rooted at the fixtures/ directory
// relative to the test file. Call from t.TempDir() or t.Parallel() tests.
func TestFixtures(t *testing.T) *Fixtures {
	t.Helper()
	// Resolve fixtures relative to the test file's directory.
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("testutil: get cwd: %v", err)
	}
	root := filepath.Join(cwd, "fixtures")
	if _, err := os.Stat(root); os.IsNotExist(err) {
		t.Fatalf("testutil: fixtures directory %q does not exist", root)
	}
	return MustFixtures(root)
}

// TestGolden returns a Golden instance rooted at the golden/ directory
// relative to the test file's directory.
func TestGolden(t *testing.T) *Golden {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("testutil: get cwd: %v", err)
	}
	root := filepath.Join(cwd, "golden")
	if _, err := os.Stat(root); os.IsNotExist(err) {
		t.Fatalf("testutil: golden directory %q does not exist", root)
	}
	return MustGolden(root)
}
