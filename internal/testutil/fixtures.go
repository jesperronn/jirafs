// Package testutil provides fixture loading and golden-test helpers for jirafs tests.
package testutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Fixtures manages loading test fixtures from a directory tree.
//
// Fixtures are stored under a fixtures/ directory relative to the test
// package. Each fixture is identified by a dot-separated path such as
// "issue.synced/ABC-123" which maps to fixtures/issue/synced/ABC-123.
type Fixtures struct {
	root string
}

// NewFixtures creates a Fixtures rooted at the given directory.
// The root is resolved relative to the calling test's file if it is
// a relative path.
func NewFixtures(root string) (*Fixtures, error) {
	if !filepath.IsAbs(root) {
		abs, err := filepath.Abs(root)
		if err != nil {
			return nil, fmt.Errorf("testutil: resolve fixtures root: %w", err)
		}
		root = abs
	}
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return nil, fmt.Errorf("testutil: fixtures root %q does not exist", root)
	}
	return &Fixtures{root: root}, nil
}

// MustFixtures is like NewFixtures but panics on error.
// Use in test Init functions or when the fixtures directory is expected
// to always exist.
func MustFixtures(root string) *Fixtures {
	f, err := NewFixtures(root)
	if err != nil {
		panic(fmt.Sprintf("testutil: %v", err))
	}
	return f
}

// FixturePath returns the absolute path for a fixture identified by its
// dot-separated key. The key "issue.synced/ABC-123" maps to
// root/issue/synced/ABC-123.
func (f *Fixtures) FixturePath(key string) string {
	rel := keyToRel(key)
	return filepath.Join(f.root, rel)
}

// Read reads a fixture by key and returns its content.
// Returns an error if the fixture does not exist or cannot be read.
func (f *Fixtures) Read(key string) (string, error) {
	path := f.FixturePath(key)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("testutil: read fixture %q: %w", key, err)
	}
	return string(data), nil
}

// ReadRaw returns the raw []byte content of a fixture.
func (f *Fixtures) ReadRaw(key string) ([]byte, error) {
	path := f.FixturePath(key)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("testutil: read fixture %q: %w", key, err)
	}
	return data, nil
}

// Exists reports whether a fixture exists at the given key.
func (f *Fixtures) Exists(key string) bool {
	_, err := os.Stat(f.FixturePath(key))
	return err == nil
}

// List returns all fixture keys under a given prefix.
// The prefix "issue" returns keys like "issue.synced/ABC-123".
// An empty prefix returns all fixtures.
func (f *Fixtures) List(prefix string) ([]string, error) {
	var keys []string
	prefixPath := prefix
	if prefix != "" {
		prefixPath = prefixToDir(prefix)
	}
	err := filepath.Walk(f.root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(f.root, path)
		if err != nil {
			return err
		}
		// Convert path separators to dots for the directory part
		dirKey := dirToKey(filepath.Dir(rel))
		if prefixPath != "" {
			dirPath := filepath.Dir(rel)
			if !strings.HasPrefix(dirPath, prefixPath) && dirPath != strings.TrimSuffix(prefixPath, "/") {
				return nil
			}
		}
		key := dirKey + "/" + info.Name()
		keys = append(keys, key)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("testutil: list fixtures: %w", err)
	}
	return keys, nil
}

// WriteFixture writes content to a fixture location.
// Creates parent directories as needed.
func (f *Fixtures) WriteFixture(key, content string) error {
	path := f.FixturePath(key)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("testutil: write fixture %q: %w", key, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("testutil: write fixture %q: %w", key, err)
	}
	return nil
}

// keyToRel converts a dot-separated fixture key to a filesystem path.
// "issue.synced/ABC-123" -> "issue/synced/ABC-123"
func keyToRel(key string) string {
	parts := strings.Split(key, "/")
	if len(parts) == 0 {
		return ""
	}
	dirParts := strings.Split(parts[0], ".")
	if len(parts) == 1 {
		return filepath.Join(append(dirParts, "")...)
	}
	return filepath.Join(append(dirParts, parts[1:]...)...)
}

// prefixToDir converts a prefix like "issue" to "issue/".
func prefixToDir(prefix string) string {
	return strings.ReplaceAll(prefix, ".", "/") + "/"
}

// dirToKey converts a directory path like "issue/synced" to "issue.synced".
func dirToKey(dir string) string {
	return strings.ReplaceAll(dir, "/", ".")
}
