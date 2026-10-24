package mirror

import (
	"os"
	"path/filepath"

	"github.com/jirafs/jirafs/internal/schema"
)

// BaseDir returns the directory holding base snapshots for mirrorDir. It is
// jirafs-internal state and must not be edited by hand.
func BaseDir(mirrorDir string) string {
	return filepath.Join(mirrorDir, ".jirafs", "base")
}

// BasePath returns the path of the base snapshot of key.
func BasePath(mirrorDir string, key schema.IssueKey) string {
	return filepath.Join(BaseDir(mirrorDir), string(key)+".md")
}

// SaveBase records the rendered remote-side content of issue (volatile fields
// excluded) as the merge base of its key. The write is atomic: a temp file in
// the same directory is renamed over the destination. It does nothing when
// mirrorDir is empty.
func SaveBase(mirrorDir string, issue *schema.Issue) error {
	if mirrorDir == "" {
		return nil
	}
	dir := BaseDir(mirrorDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	content := schema.StripVolatile(schema.RenderIssue(*issue))
	tmp, err := os.CreateTemp(dir, ".base-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.WriteString(content); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, BasePath(mirrorDir, issue.Identity.Key)); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return nil
}

// LoadBase returns the stored base snapshot of key. A missing or unparseable
// base yields (nil, false); it is never an error. The snapshot is parsed the
// way a local issue file is, so the two compare field by field.
func LoadBase(mirrorDir string, key schema.IssueKey) (*schema.Issue, bool) {
	if mirrorDir == "" {
		return nil, false
	}
	data, err := os.ReadFile(BasePath(mirrorDir, key))
	if err != nil {
		return nil, false
	}
	issue, perr := schema.ParseIssue(string(data))
	if perr != nil {
		return nil, false
	}
	return &issue, true
}

// ArchiveBase moves the base snapshot of key into the _archive subdirectory of
// the base store, so it is kept but no longer used as a merge base. A missing
// base is not an error.
func ArchiveBase(mirrorDir string, key schema.IssueKey) error {
	if mirrorDir == "" {
		return nil
	}
	src := BasePath(mirrorDir, key)
	if _, err := os.Stat(src); err != nil {
		return nil
	}
	dir := filepath.Join(BaseDir(mirrorDir), "_archive")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.Rename(src, filepath.Join(dir, string(key)+".md"))
}
