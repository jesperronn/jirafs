package mirror

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jirafs/jirafs/internal/schema"
)

// BackupDir returns the directory holding backups of local files that a pull
// merged or marked conflicted.
func BackupDir(mirrorDir string) string {
	return filepath.Join(mirrorDir, ".jirafs", "backup")
}

// SaveBackup writes content to .jirafs/backup/KEY-<timestamp>.md under
// mirrorDir and returns its path. An existing backup is never overwritten: a
// numeric suffix is added instead.
func SaveBackup(mirrorDir string, key schema.IssueKey, content string, now time.Time) (string, error) {
	if mirrorDir == "" {
		return "", fmt.Errorf("no mirror directory to store a backup of %s", key)
	}
	dir := BackupDir(mirrorDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	stamp := now.UTC().Format("20060102T150405Z")
	for n := 0; ; n++ {
		name := fmt.Sprintf("%s-%s.md", key, stamp)
		if n > 0 {
			name = fmt.Sprintf("%s-%s-%d.md", key, stamp, n+1)
		}
		f, err := os.OpenFile(filepath.Join(dir, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if os.IsExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		path := f.Name()
		if _, err := f.WriteString(content); err != nil {
			_ = f.Close()
			return "", err
		}
		return path, f.Close()
	}
}
