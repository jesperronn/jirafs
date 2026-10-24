package cli

import (
	stdcontext "context"
	"fmt"
	"github.com/jirafs/jirafs/internal/config"
	"github.com/jirafs/jirafs/internal/context"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jirafs/jirafs/internal/jira"
	"github.com/jirafs/jirafs/internal/mirror"
	"github.com/jirafs/jirafs/internal/schema"
)

// archiveRemoteIssue marks the local file of key as gone (movedTo == "") or
// moved and moves it into <dir>/_archive. Nothing is ever deleted: local edits
// stay in the archived file. It reports whether the file carried local edits.
// With dryRun it only reports. The base snapshot moves along with the file.
func archiveRemoteIssue(path, mirrorDir string, key schema.IssueKey, movedTo string, dryRun bool) (edited bool, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	doc := string(data)
	edited = classifyLocalIssue(path, "") == localEdited
	archiveDir := filepath.Join(filepath.Dir(path), "_archive")
	dest := filepath.Join(archiveDir, filepath.Base(path))
	if _, serr := os.Stat(dest); serr == nil {
		return edited, fmt.Errorf("%s already exists; move it aside and retry", dest)
	}
	if dryRun {
		return edited, nil
	}
	issue, perr := schema.ParseIssue(doc)
	if perr != nil {
		return edited, fmt.Errorf("parse %s: %s", path, perr.Error())
	}
	issue.RemoteMetadata.StateFile = string(schema.StateArchived)
	if movedTo == "" {
		issue.RemoteMetadata.RemoteState = "gone"
	} else {
		issue.RemoteMetadata.RemoteState = "moved"
		issue.RemoteMetadata.MovedTo = movedTo
	}
	if err := os.MkdirAll(archiveDir, 0o755); err != nil {
		return edited, err
	}
	if err := os.WriteFile(path, []byte(schema.RenderIssue(issue)), 0o644); err != nil {
		return edited, err
	}
	if err := os.Rename(path, dest); err != nil {
		return edited, err
	}
	return edited, mirror.ArchiveBase(mirrorDir, key)
}

// goneLine is the one-line report for a gone issue.
func goneLine(key schema.IssueKey, edited, dryRun bool) string {
	prefix := ""
	if dryRun {
		prefix = "would "
	}
	note := "moved to"
	if dryRun {
		note = "move to"
	}
	kept := ""
	if edited {
		kept = " with your local edits kept"
	}
	return fmt.Sprintf("%sgone %s: %s _archive/%s.md%s (restore: mv _archive/%s.md .)", prefix, key, note, key, kept, key)
}

// movedLine is the one-line report for a moved issue.
func movedLine(oldKey, newKey schema.IssueKey, edited, dryRun bool) string {
	prefix := ""
	if dryRun {
		prefix = "would "
	}
	kept := ""
	if edited {
		kept = fmt.Sprintf(" (local edits kept in _archive/%s.md)", oldKey)
	}
	return fmt.Sprintf("%smoved %s -> %s%s: jirafs pull %s", prefix, oldKey, newKey, kept, newKey)
}

// handleGoneIssue archives the local file of a not-found issue. handled is
// false when there is no local file, so the caller keeps its normal error.
func handleGoneIssue(pc *pullGitCommit, m *mirror.Mirror, path, mirrorDir string, key schema.IssueKey, dryRun bool) (handled, changed bool, err error) {
	if _, serr := os.Stat(path); serr != nil {
		return false, false, nil
	}
	edited, err := archiveRemoteIssue(path, mirrorDir, key, "", dryRun)
	if err != nil {
		return true, false, err
	}
	fmt.Fprintln(pullStdout, goneLine(key, edited, dryRun))
	if dryRun {
		return true, false, nil
	}
	pc.recordArchive(path, key, "")
	return true, m.ForgetIssue(key), nil
}

// handleMovedIssue writes the fetched issue under its new key, archives the
// old file and rekeys mirror membership. Local edits at the new key are kept.
func handleMovedIssue(pc *pullGitCommit, m *mirror.Mirror, localDir, mirrorDir string, oldKey schema.IssueKey, issue *schema.Issue, dryRun bool) error {
	newKey := issue.Identity.Key
	oldPath := filepath.Join(localDir, string(oldKey)+".md")
	newPath := filepath.Join(localDir, string(newKey)+".md")
	edited := false
	_, statErr := os.Stat(oldPath)
	hasOld := statErr == nil
	if hasOld {
		var err error
		if edited, err = archiveRemoteIssue(oldPath, mirrorDir, oldKey, string(newKey), dryRun); err != nil {
			return err
		}
	}
	if !dryRun {
		if hasOld {
			pc.recordArchive(oldPath, oldKey, newKey)
		}
		if classifyLocalIssue(newPath, "") != localEdited {
			if err := writeSyncedIssue(newPath, mirrorDir, issue); err != nil {
				return err
			}
			pc.recordWritten(newPath, newKey)
		}
		m.RenameIssue(oldKey, newKey)
		m.SetRemoteUpdated(newKey, issue.Context.Updated)
	}
	fmt.Fprintln(pullStdout, movedLine(oldKey, newKey, edited, dryRun))
	return nil
}

// checkVanishedMembers verifies scope members that have a local file but were
// absent from a full search: Jira may have deleted or moved them. Other
// outcomes (still exists under the same key) are left alone.
func checkVanishedMembers(pc *pullGitCommit, cmdCtx stdcontext.Context, client jira.Client, m *mirror.Mirror, scopeName, localDir, mirrorDir string, seen map[schema.IssueKey]bool, dryRun bool) (changed bool, failed int) {
	var keys []schema.IssueKey
	for _, mem := range m.ScopeMembers {
		if mem.Scope == scopeName && !seen[mem.Key] {
			if _, err := os.Stat(filepath.Join(localDir, string(mem.Key)+".md")); err == nil {
				keys = append(keys, mem.Key)
			}
		}
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	for _, key := range keys {
		path := filepath.Join(localDir, string(key)+".md")
		issue, err := client.FetchIssue(cmdCtx, string(key))
		switch {
		case err != nil && jira.IsClientError(err, jira.ErrNotFound):
			_, c, herr := handleGoneIssue(pc, m, path, mirrorDir, key, dryRun)
			changed = changed || c
			if herr != nil {
				fmt.Fprintf(pullStderr, "%s: %v\n", key, herr)
				failed++
			}
		case err != nil:
			fmt.Fprintf(pullStderr, "%s: cannot check remote state: %v\n", key, err)
			failed++
		case issue != nil && issue.Identity.Key != "" && !strings.EqualFold(string(issue.Identity.Key), string(key)):
			if herr := handleMovedIssue(pc, m, localDir, mirrorDir, key, issue, dryRun); herr != nil {
				fmt.Fprintf(pullStderr, "%s: %v\n", key, herr)
				failed++
			} else {
				changed = true
			}
		}
	}
	return changed, failed
}

// pullGoneIssue handles `pull KEY` for an issue Jira no longer has. handled is
// false when there is no local file to archive.
func pullGoneIssue(pc *pullGitCommit, m *mirror.Mirror, mirrorPath string, settings *config.Settings, ctx *context.Context, key string, dryRun bool) (code int, handled bool) {
	proj := settings.Projects[ctx.Name]
	if len(proj.LocalDirs) == 0 {
		return 0, false
	}
	path := filepath.Join(proj.LocalDirs[0], key+".md")
	ok, changed, err := handleGoneIssue(pc, m, path, ctx.MirrorDir, schema.IssueKey(key), dryRun)
	if !ok {
		return 0, false
	}
	if err != nil {
		fmt.Fprintf(pullStderr, "jirafs pull: %s: %v\n", key, err)
		return 1, true
	}
	return saveIfChanged(m, mirrorPath, changed), true
}

// pullMovedIssue handles `pull KEY` when Jira answers with a different key.
func pullMovedIssue(pc *pullGitCommit, m *mirror.Mirror, mirrorPath string, settings *config.Settings, ctx *context.Context, key string, issue *schema.Issue, dryRun bool) int {
	proj := settings.Projects[ctx.Name]
	if len(proj.LocalDirs) == 0 {
		fmt.Fprintf(pullStderr, "jirafs pull: project %q has no local_dirs configured\n", ctx.Name)
		return 1
	}
	if !dryRun {
		if err := os.MkdirAll(proj.LocalDirs[0], 0o755); err != nil {
			fmt.Fprintf(pullStderr, "jirafs pull: %v\n", err)
			return 1
		}
	}
	if err := handleMovedIssue(pc, m, proj.LocalDirs[0], ctx.MirrorDir, schema.IssueKey(key), issue, dryRun); err != nil {
		fmt.Fprintf(pullStderr, "jirafs pull: %s: %v\n", key, err)
		return 1
	}
	return saveIfChanged(m, mirrorPath, !dryRun)
}

// saveIfChanged persists m when it changed and mirror.yml already exists.
func saveIfChanged(m *mirror.Mirror, mirrorPath string, changed bool) int {
	if !changed {
		return 0
	}
	if _, err := os.Stat(mirrorPath); err != nil {
		return 0
	}
	if err := saveMirrorYAML(mirrorPath, m); err != nil {
		fmt.Fprintf(pullStderr, "jirafs pull: cannot save mirror: %v\n", err)
		return 1
	}
	return 0
}

// recordArchive notes that the live file at oldPath was archived (newKey ""
// means gone, otherwise moved) so the git commit stages the removal and the
// new _archive file.
func (c *pullGitCommit) recordArchive(oldPath string, key, newKey schema.IssueKey) {
	if c == nil {
		return
	}
	archived := filepath.Join(filepath.Dir(oldPath), "_archive", filepath.Base(oldPath))
	c.recordWritten(oldPath, key)
	c.recordWritten(archived, key)
	if c.Origin == nil {
		c.Origin = map[string]string{}
	}
	c.Origin[archived] = oldPath
	if newKey == "" {
		c.Archived = append(c.Archived, key)
	} else {
		c.Moved = append(c.Moved, key)
	}
}

// recordWritten notes that path was written or removed for key.
func (c *pullGitCommit) recordWritten(path string, key schema.IssueKey) {
	if c == nil {
		return
	}
	if c.Written == nil {
		c.Written = map[string]schema.IssueKey{}
	}
	c.Written[path] = key
}
