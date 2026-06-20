package testutil

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Golden manages comparison of test output against golden (expected) files.
//
// Golden files live alongside fixtures, under a golden/ subdirectory.
// A golden key "issue.synced/ABC-123" maps to golden/issue/synced/ABC-123.
type Golden struct {
	root string
}

// NewGolden creates a Golden rooted at the given directory.
func NewGolden(root string) (*Golden, error) {
	if !filepath.IsAbs(root) {
		abs, err := filepath.Abs(root)
		if err != nil {
			return nil, fmt.Errorf("testutil: resolve golden root: %w", err)
		}
		root = abs
	}
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return nil, fmt.Errorf("testutil: golden root %q does not exist", root)
	}
	return &Golden{root: root}, nil
}

// MustGolden is like NewGolden but panics on error.
func MustGolden(root string) *Golden {
	g, err := NewGolden(root)
	if err != nil {
		panic(fmt.Sprintf("testutil: %v", err))
	}
	return g
}

// GoldenPath returns the absolute path for a golden file identified by its
// dot-separated key.
func (g *Golden) GoldenPath(key string) string {
	rel := keyToRel(key)
	return filepath.Join(g.root, "golden", rel)
}

// GoldenExists reports whether a golden file exists at the given key.
func (g *Golden) GoldenExists(key string) bool {
	_, err := os.Stat(g.GoldenPath(key))
	return err == nil
}

// AssertEqual compares actual output against the golden file and fails the
// test if they differ. It writes the golden file if it does not exist,
// allowing new golden files to be created during test development.
//
// If updateGolden is true, the golden file is always overwritten with
// actual output (useful for updating expected output).
func AssertEqual(t *testing.T, key, actual string, updateGolden bool) {
	t.Helper()

	goldenPath := filepath.Join(t.Name(), key)
	fullPath := filepath.Join(goldenPath)

	// Ensure golden directory exists
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("testutil: create golden dir: %v", err)
	}

	expected, err := os.ReadFile(fullPath)
	if os.IsNotExist(err) {
		if updateGolden {
			if writeErr := os.WriteFile(fullPath, []byte(actual), 0o644); writeErr != nil {
				t.Fatalf("testutil: create golden file: %v", writeErr)
			}
			t.Logf("testutil: created golden file %s", fullPath)
			return
		}
		t.Fatalf("testutil: golden file %q does not exist (run with UPDATE_GOLDEN=1 to create)", fullPath)
	}
	if err != nil {
		t.Fatalf("testutil: read golden file: %v", err)
	}

	if string(expected) != actual {
		t.Errorf("testutil: golden mismatch for %q\n--- expected ---\n%s\n--- actual ---\n%s\n--- diff ---\n%s\n",
			key, truncate(expected, 200), truncate([]byte(actual), 200),
			diffLines(string(expected), actual))
	}
}

// AssertRaw is like AssertEqual but works with []byte.
func AssertRaw(t *testing.T, key string, actual []byte, updateGolden bool) {
	t.Helper()
	AssertEqual(t, key, string(actual), updateGolden)
}

// MustReadGolden reads a golden file and returns its content, failing the
// test if it does not exist.
func MustReadGolden(t *testing.T, golden *Golden, key string) string {
	t.Helper()
	path := golden.GoldenPath(key)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("testutil: read golden %q: %v", key, err)
	}
	return string(data)
}

// WriteGolden writes content to a golden file, creating directories as needed.
func WriteGolden(golden *Golden, key, content string) error {
	path := golden.GoldenPath(key)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("testutil: write golden: %w", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("testutil: write golden: %w", err)
	}
	return nil
}

// truncate truncates a byte slice to the given max length, adding an
// ellipsis if truncated.
func truncate(data []byte, max int) string {
	s := string(data)
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// diffLines produces a simple line-based diff between expected and actual strings.
func diffLines(expected, actual string) string {
	expLines := strings.Split(expected, "\n")
	actLines := strings.Split(actual, "\n")

	var buf bytes.Buffer
	maxLines := len(expLines)
	if len(actLines) > maxLines {
		maxLines = len(actLines)
	}

	for i := 0; i < maxLines; i++ {
		var exp, act string
		if i < len(expLines) {
			exp = expLines[i]
		}
		if i < len(actLines) {
			act = actLines[i]
		}
		if exp != act {
			prefix := fmt.Sprintf("line %d: ", i+1)
			if i < len(expLines) && i < len(actLines) {
				buf.WriteString(fmt.Sprintf("%s- %s\n", prefix, exp))
				buf.WriteString(fmt.Sprintf("%s+ %s\n", prefix, act))
			} else if i >= len(expLines) {
				buf.WriteString(fmt.Sprintf("%s+ %s\n", prefix, act))
			} else {
				buf.WriteString(fmt.Sprintf("%s- %s\n", prefix, exp))
			}
		}
	}

	if buf.Len() == 0 {
		return "(no line differences)"
	}
	return buf.String()
}
