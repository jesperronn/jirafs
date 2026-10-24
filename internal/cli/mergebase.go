package cli

import (
	"path/filepath"

	"github.com/jirafs/jirafs/internal/gitx"
	"github.com/jirafs/jirafs/internal/mirror"
	"github.com/jirafs/jirafs/internal/schema"
)

// loadMergeBase returns the merge base of the issue file at path: the last
// known remote state of key. In git mode (`git: true` in mirror.yml) that is
// the file as of the newest `sync from`/`push to` commit, so commits the user
// made in between count as local edits. When that is unavailable (git mode
// off, no sync commit, history rewritten, file added after the sync commit,
// unparseable content) it falls back to the stored snapshot in
// .jirafs/base, and to no base (nil, false) when that is missing too. Both
// pull's merge and push's conflict detection call it.
func loadMergeBase(mirrorDir string, key schema.IssueKey, path string) (*schema.Issue, bool) {
	if base, ok := gitMergeBase(mirrorDir, path); ok {
		return base, true
	}
	return mirror.LoadBase(mirrorDir, key)
}

func gitMergeBase(mirrorDir, path string) (*schema.Issue, bool) {
	if mirrorDir == "" || path == "" || !gitx.Available() {
		return nil, false
	}
	m, _, err := loadMirrorYAML(mirrorDir)
	if err != nil || !m.Git || !gitx.IsRepo(mirrorDir) {
		return nil, false
	}
	sha, err := gitx.LastSyncCommit(mirrorDir)
	if err != nil || sha == "" {
		return nil, false
	}
	content, ok := gitx.ShowFile(mirrorDir, sha, absPath(path))
	if !ok {
		return nil, false
	}
	issue, perr := schema.ParseIssue(content)
	if perr != nil {
		return nil, false
	}
	return &issue, true
}

func absPath(p string) string {
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
}
