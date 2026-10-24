package cli

import (
	"os"
	"path/filepath"

	"github.com/jirafs/jirafs/internal/mirror"
	"github.com/jirafs/jirafs/internal/schema"
)

// writeSyncedIssue renders issue, stamps it with its synced_hash and writes it
// to dest and stores its remote-side content as the merge base under
// .jirafs/base in mirrorDir. When mirrorDir holds a mirror.yml, the issue's remote updated
// timestamp is recorded there so `jirafs status` can detect remote changes
// offline. Mirror bookkeeping failures are returned so callers never silently
// lose conflict information.
func writeSyncedIssue(dest, mirrorDir string, issue *schema.Issue) error {
	content := schema.StampSyncedHash(schema.RenderIssue(*issue))
	if err := os.WriteFile(dest, []byte(content), 0o644); err != nil {
		return err
	}
	if err := mirror.SaveBase(mirrorDir, issue); err != nil {
		return err
	}
	return recordRemoteUpdated(mirrorDir, issue.Identity.Key, issue.Context.Updated)
}

// recordRemoteUpdated stores updated as the remote timestamp of key in the
// mirror.yml of mirrorDir. It does nothing when there is no mirror.yml or
// nothing to record.
func recordRemoteUpdated(mirrorDir string, key schema.IssueKey, updated string) error {
	if mirrorDir == "" || updated == "" {
		return nil
	}
	path := filepath.Join(mirrorDir, "mirror.yml")
	if _, err := os.Stat(path); err != nil {
		return nil
	}
	m, _, err := loadMirrorYAML(mirrorDir)
	if err != nil {
		return err
	}
	m.SetRemoteUpdated(key, updated)
	return saveMirrorYAML(path, m)
}
