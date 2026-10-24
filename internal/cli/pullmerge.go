package cli

import (
	stdcontext "context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jirafs/jirafs/internal/context"
	"github.com/jirafs/jirafs/internal/jira"
	"github.com/jirafs/jirafs/internal/merge"
	"github.com/jirafs/jirafs/internal/mirror"
	"github.com/jirafs/jirafs/internal/schema"
	"gopkg.in/yaml.v3"
)

// mergeKind classifies what a pull did to a locally edited issue file.
type mergeKind int

const (
	// mergeKept: the remote has not changed since the last pull; the file is
	// untouched.
	mergeKept mergeKind = iota
	// mergeMerged: remote changes were merged with the local edits.
	mergeMerged
	// mergeConflicted: some fields changed on both sides; the local values
	// were kept and the remote values recorded under `conflicts:`.
	mergeConflicted
)

// mergeReport describes the result of mergeEditedFile.
type mergeReport struct {
	Kind   mergeKind
	Fields []string // conflicted fields
	NoBase bool     // no base snapshot existed
	Backup string   // path of the saved copy of the local file
	// Clean is true when the file was rewritten as a plain synced copy of the
	// remote (the local edits were already contained in it).
	Clean bool
}

// conflictEntry is one element of the `conflicts:` frontmatter list.
type conflictEntry struct {
	Field  string `yaml:"field"`
	Remote string `yaml:"remote"`
	Base   string `yaml:"base"`
}

// conflictsBlock renders the `conflicts:` frontmatter key.
func conflictsBlock(cs []merge.Conflict) string {
	entries := make([]conflictEntry, len(cs))
	for i, c := range cs {
		entries[i] = conflictEntry{Field: string(c.Field), Remote: c.Remote, Base: c.Base}
	}
	out, _ := yaml.Marshal(map[string][]conflictEntry{"conflicts": entries}) // plain strings always marshal
	return string(out)
}

// mergeEditedFile merges the remote issue into the locally edited file at
// dest. It first saves the local file under .jirafs/backup and never discards
// a local value: where both sides changed a field the local value stays.
func mergeEditedFile(dest, mirrorDir string, remote *schema.Issue, now time.Time) (mergeReport, error) {
	data, err := os.ReadFile(dest)
	if err != nil {
		return mergeReport{}, err
	}
	doc := string(data)
	local, perr := schema.ParseIssue(doc)
	if perr == nil && local.Context.Updated != "" && local.Context.Updated == remote.Context.Updated {
		return mergeReport{Kind: mergeKept}, nil
	}
	backup, err := mirror.SaveBackup(mirrorDir, remote.Identity.Key, doc, now)
	if err != nil {
		return mergeReport{}, fmt.Errorf("save backup: %w", err)
	}
	if perr != nil {
		return mergeReport{Kind: mergeConflicted, Fields: []string{"file"}, Backup: backup}, nil
	}

	base, haveBase := loadMergeBase(mirrorDir, remote.Identity.Key, dest)
	if !haveBase {
		base = nil
	}
	res := merge.Issue(base, local, *remote)
	rep := mergeReport{Kind: mergeMerged, NoBase: !haveBase, Backup: backup}

	remoteDoc := schema.RenderIssue(*remote)
	mergedDoc := schema.RenderIssue(res.Merged)
	if len(res.Conflicts) == 0 && mergedDoc == remoteDoc {
		rep.Clean = true
		return rep, writeSyncedIssue(dest, mirrorDir, remote)
	}

	if len(res.Conflicts) > 0 {
		rep.Kind = mergeConflicted
		for _, c := range res.Conflicts {
			rep.Fields = append(rep.Fields, string(c.Field))
		}
		mergedDoc = schema.AppendFrontmatter(mergedDoc, conflictsBlock(res.Conflicts))
	}
	// The recorded hash is that of the remote content, so the file keeps
	// reading as modified until its local changes are pushed or resolved.
	content := schema.StampSyncedHashAs(mergedDoc, schema.ContentHash(remoteDoc))
	if err := os.WriteFile(dest, []byte(content), 0o644); err != nil {
		return rep, err
	}
	if haveBase || len(res.Conflicts) == 0 {
		if err := mirror.SaveBase(mirrorDir, &res.NewBase); err != nil {
			return rep, err
		}
	}
	return rep, recordRemoteUpdated(mirrorDir, remote.Identity.Key, remote.Context.Updated)
}

// mergeLine is the one-line pull report for an issue.
func mergeLine(key schema.IssueKey, rep mergeReport) string {
	switch rep.Kind {
	case mergeKept:
		return fmt.Sprintf("kept %s (local edits; remote unchanged since last pull; inspect: jirafs diff %s)", key, key)
	case mergeConflicted:
		line := fmt.Sprintf("conflict %s %s", key, strings.Join(rep.Fields, ","))
		if rep.NoBase {
			line += " (no base snapshot)"
		}
		return fmt.Sprintf("%s (backup: %s; review: jirafs diff %s)", line, rep.Backup, key)
	}
	return fmt.Sprintf("merged %s (backup: %s)", key, rep.Backup)
}

// pullMerges collects the merge outcomes of one scope pull.
type pullMerges struct {
	Lines      []string
	Merged     int
	Conflicted int
}

// summary returns ", merged N, conflicted M" (empty when there were none).
func (m pullMerges) summary() string {
	s := ""
	if m.Merged > 0 {
		s += fmt.Sprintf(", merged %d", m.Merged)
	}
	if m.Conflicted > 0 {
		s += fmt.Sprintf(", conflicted %d", m.Conflicted)
	}
	return s
}

// mergeEditedScopeIssue merges one locally edited issue found by a scope pull
// and records the outcome in res.
func mergeEditedScopeIssue(ctx stdcontext.Context, client jira.Client, projectCtx *context.Context, issue *schema.Issue, dest string, res *pullResult) {
	key := issue.Identity.Key
	remote := issue
	if full, err := client.FetchIssue(ctx, string(key)); err == nil && full != nil && full.Identity.Key != "" {
		remote = full
	}
	jira.AttachComments(ctx, client, remote, pullStderr)
	rep, err := mergeEditedFile(dest, projectCtx.MirrorDir, remote, pullNow())
	if err != nil {
		res.Failures = append(res.Failures, pullFailure{Key: key, Reason: fmt.Sprintf("merge into %s: %v", filepath.Base(dest), err)})
		return
	}
	res.Merges.add(key, rep)
	if rep.Kind == mergeKept {
		res.LocalEdits = append(res.LocalEdits, key)
	}
	if rep.Clean {
		res.Written[dest] = key
	}
	if remote.Context.Updated != "" {
		res.RemoteUpdated[key] = remote.Context.Updated
	}
}

func (m *pullMerges) add(key schema.IssueKey, rep mergeReport) {
	switch rep.Kind {
	case mergeMerged:
		m.Merged++
	case mergeConflicted:
		m.Conflicted++
	default:
		return
	}
	m.Lines = append(m.Lines, mergeLine(key, rep))
}

// pullMergeSingle handles `pull KEY` when the local file has unsynced edits.
// handled is false when the file is not edited and the caller should write it.
// clean is true when the file ended up a plain synced copy of the remote.
func pullMergeSingle(dest, mirrorDir string, remote *schema.Issue) (handled, clean bool, err error) {
	if classifyLocalIssue(dest, "") != localEdited {
		return false, false, nil
	}
	rep, err := mergeEditedFile(dest, mirrorDir, remote, pullNow())
	if err != nil {
		return true, false, err
	}
	fmt.Fprintf(pullStdout, "%s\n", mergeLine(remote.Identity.Key, rep))
	return true, rep.Clean, nil
}
