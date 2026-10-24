// Package cli provides command implementations for the jirafs CLI.
package cli

import (
	stdcontext "context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/jirafs/jirafs/internal/color"
	"github.com/jirafs/jirafs/internal/config"
	"github.com/jirafs/jirafs/internal/context"
	"github.com/jirafs/jirafs/internal/jira"
	"github.com/jirafs/jirafs/internal/plan"
	"github.com/jirafs/jirafs/internal/schema"
	"github.com/jirafs/jirafs/internal/sync"
)

var (
	syncStdout        io.Writer = os.Stdout
	syncStderr        io.Writer = os.Stderr
	syncClientFactory           = newJiraClient
)

// RunSync dispatches the `jirafs sync` subcommand. When an issue key is
// provided, it resolves the project context, fetches the remote issue from
// Jira, reads the local issue from the file system, builds and validates a
// plan, applies the plan, and pushes the updated remote back to Jira through
// the real service path. Without an issue key it exits 2: syncing multiple
// issues is not implemented yet.
func RunSync(args []string) int {
	cmdCtx, stop := commandContext()
	defer stop()
	// Check for help before loading settings.
	if wantsHelpOrWord(args) {
		printSyncHelp()
		return 0
	}

	fs := quietFlagSet("sync")
	projectFlag := fs.String("project", "", "project key or name to use")
	fs.String("cwd", "", "ignored; sync resolves the project from the current directory")
	applyFlag := fs.Bool("apply", false, "write the updated remote to the local issue file")
	dryRunFlag := fs.Bool("dry-run", false, "print the plan; update neither Jira nor the local file")
	if err := parseFlags(fs, args); err != nil {
		return flagError(syncStderr, "jirafs sync", err)
	}

	// Multi-issue sync is blocked for now; reject before loading anything.
	if len(fs.Args()) == 0 {
		return usageError(syncStderr, "jirafs sync", "syncing multiple issues is not implemented yet; sync one issue at a time: jirafs sync <KEY>")
	}

	if dryRunConflicts(syncStderr, "jirafs sync", *dryRunFlag, *applyFlag) {
		return 2
	}

	// Load settings.
	settings, ok := loadSettingsOrHint(syncStderr, "sync")
	if !ok {
		return 1
	}

	// Parse flags to extract --project and --cwd regardless of whether
	// an issue key is provided. This allows "jirafs sync --project TEST"
	// (no key) to resolve the project and list pending syncs.
	return runSyncIssue(cmdCtx, fs.Args(), settings, context.NewResolver(settings, *projectFlag), *applyFlag, *dryRunFlag)
}

// runSyncIssue handles `jirafs sync KEY`. It resolves the project context,
// fetches the remote issue from Jira through the real service path, reads
// the local issue from the file system, builds and validates a plan,
// applies the plan, and pushes the updated remote back to Jira.
func runSyncIssue(cmdCtx stdcontext.Context, args []string, settings *config.Settings, resolver *context.Resolver, apply, dryRun bool) int {
	if len(args) == 0 {
		fmt.Fprintln(syncStderr, "jirafs sync: missing issue key")
		return 1
	}
	if len(args) > 1 {
		fmt.Fprintln(syncStderr, "jirafs sync: too many positional arguments")
		return 1
	}
	key := args[0]

	cwd := "."
	ctx, ok := resolveSyncContext(settings, resolver)
	if !ok {
		return 1
	}

	// Create the Jira client.
	client, err := syncClientFactory(settings, ctx, cwd)
	if err != nil {
		fmt.Fprintf(syncStderr, "jirafs sync: cannot create Jira client: %v\n", err)
		return 1
	}

	// Fetch the remote issue from Jira.
	remote, err := client.FetchIssue(cmdCtx, key)
	if err != nil {
		fmt.Fprintf(syncStderr, "jirafs sync: %v\n", err)
		return 1
	}

	// Read the local issue from the file system using the project's local_dirs.
	localIssue, localPath, err := readLocalIssue(ctx, settings, key)
	if err != nil {
		fmt.Fprintf(syncStderr, "jirafs sync: %v\n", err)
		return 1
	}

	// The local status is the frontmatter `status:` value (parsed into
	// Context). A missing status means "no change", never a status edit.
	localIssue.Status = localIssue.Context.Status
	if localIssue.Status == "" {
		localIssue.Status = remote.Status
	}

	if remote.Status == "" && localIssue.Context.Status != "" {
		fmt.Fprintf(syncStderr, "jirafs sync: warning: remote status of %s is unknown; not changing status\n", key)
	}

	// Build the plan by comparing local and remote.
	ops, _, err := plan.BuildPlan(localIssue, *remote)
	if err != nil {
		fmt.Fprintf(syncStderr, "jirafs sync: %v\n", err)
		return 1
	}

	if len(ops) == 0 {
		fmt.Fprintf(syncStdout, "jirafs sync: issue %s in %s: no changes needed\n",
			key, localPath)
		return 0
	}

	// Validate and apply the plan through the sync service.
	// Use a temp directory as the archive path to avoid validation errors.
	// In production, the archive directory would be configured in the project settings.
	result := sync.Sync(localIssue, *remote, ops, os.TempDir())

	if len(result.Conflicts) > 0 {
		fmt.Fprintln(syncStdout, "jirafs sync: conflicts detected, no changes applied:")
		for _, c := range result.Conflicts {
			fmt.Fprintf(syncStdout, "  %s: %s (local=%q, remote=%q)\n",
				c.Type, c.Field, c.LocalValue, c.RemoteValue)
		}
		return 0
	}

	// Sort operations deterministically by field type.
	sort.Slice(ops, func(i, j int) bool {
		return ops[i].Field < ops[j].Field
	})

	// A status change needs the transitions API. Resolve it before anything
	// is written so a missing transition fails without a partial update.
	var statusTr *jira.Transition
	var transitionFields map[string]interface{}
	for _, op := range ops {
		if op.Field != schema.EditableFieldStatus {
			continue
		}
		transitions, err := client.FetchTransitions(cmdCtx, key)
		if err != nil {
			fmt.Fprintf(syncStderr, "jirafs sync: cannot fetch transitions for %s: %v\n", key, err)
			return 1
		}
		statusTr = findTransition(transitions, op.Value)
		if statusTr == nil {
			fmt.Fprint(syncStderr, noTransitionMessage(key, remote.Status, op.Value, transitions, localPath))
			return 1
		}
		var missing []jira.RequiredField
		for _, rf := range statusTr.RequiredFields() {
			if v, ok := localIssue.CustomFields[rf.ID]; ok && v != nil && v != "" {
				if transitionFields == nil {
					transitionFields = map[string]interface{}{}
				}
				transitionFields[rf.ID] = v
			} else {
				missing = append(missing, rf)
			}
		}
		if len(missing) > 0 {
			fmt.Fprint(syncStderr, gatedTransitionMessage(key, statusTr, missing, localPath))
			return 1
		}
	}

	fmt.Fprintf(syncStdout, "jirafs sync: %d operation(s) for issue %s in %s:\n",
		len(ops), key, localPath)
	for _, op := range ops {
		if op.Field == schema.EditableFieldStatus && statusTr != nil {
			fmt.Fprintf(syncStdout, "  transition status: %s \u2192 %s (via %q)\n",
				remote.Status, statusTr.To.Name, statusTr.Name)
			continue
		}
		fmt.Fprintf(syncStdout, "  %s %s = %s\n", op.Type, op.Field, op.Value)
	}

	if dryRun {
		dryRunDone(syncStdout, "jirafs sync "+key)
		return 0
	}

	// Push the updated remote back to Jira through the real service path.
	updatedRemote, err := client.UpdateIssue(cmdCtx, key, result.Remote)
	if err != nil {
		fmt.Fprintf(syncStderr, "jirafs sync: cannot update remote issue: %v\n", err)
		return 1
	}

	if statusTr != nil {
		if err := client.DoTransition(cmdCtx, key, statusTr.ID, transitionFields); err != nil {
			fmt.Fprintf(syncStderr, "jirafs sync: fields updated, but status transition %q failed (the field update was already applied): %v\n", statusTr.Name, err)
			return 1
		}
	}

	// Update the remote metadata in the result with the updated remote.
	result.Remote.RemoteMetadata = updatedRemote.RemoteMetadata

	if apply {
		// Write the updated remote back to the local issue file.
		if err := writeUpdatedLocalIssue(localPath, ctx.MirrorDir, result.Remote); err != nil {
			fmt.Fprintf(syncStderr, "jirafs sync: cannot write updated local issue: %v\n", err)
			return 1
		}
	}

	fmt.Fprintln(syncStdout, "jirafs sync: applied "+fmt.Sprint(len(ops))+" operation(s) successfully")
	return 0
}

// findTransition returns the transition whose target status name matches
// target case-insensitively, or nil.
func findTransition(ts []jira.Transition, target string) *jira.Transition {
	for i := range ts {
		if strings.EqualFold(ts[i].To.Name, target) {
			return &ts[i]
		}
	}
	return nil
}

// noTransitionMessage explains that no transition leads to target and lists
// the reachable statuses with a copy-pasteable next step.
func noTransitionMessage(key, from, target string, ts []jira.Transition, localPath string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "jirafs sync: cannot change status of %s from %q to %q: no available Jira transition leads to it\n", key, from, target)
	if len(ts) == 0 {
		b.WriteString("available target statuses: (none)\n")
	} else {
		b.WriteString("available target statuses:\n")
		for _, t := range ts {
			fmt.Fprintf(&b, "  %s (via %q)\n", t.To.Name, t.Name)
		}
	}
	fmt.Fprintf(&b, "fix: set status in %s to one of the above, or revert it: jirafs sync %s --dry-run\n", localPath, key)
	return b.String()
}

// gatedTransitionMessage explains that the transition needs screen fields
// the issue has no value for, with a copy-pasteable next step.
func gatedTransitionMessage(key string, tr *jira.Transition, missing []jira.RequiredField, localPath string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "jirafs sync: transition %q (to %q) on %s requires fields with no value; nothing was written\n", tr.Name, tr.To.Name, key)
	b.WriteString("required fields:\n")
	for _, f := range missing {
		fmt.Fprintf(&b, "  %s (%s)\n", f.ID, f.Name)
	}
	fmt.Fprintf(&b, "fix: add the value(s) under custom_fields in %s, then run: jirafs sync %s --dry-run\n", localPath, key)
	return b.String()
}

// writeUpdatedLocalIssue writes the updated remote issue back to the local
// file system using the canonical codec (schema.RenderIssue).
func writeUpdatedLocalIssue(localPath, mirrorDir string, issue *schema.Issue) error {
	return writeSyncedIssue(localPath, mirrorDir, issue)
}

// resolveSyncContext resolves the project context using the given resolver.
func resolveSyncContext(settings *config.Settings, resolver *context.Resolver) (*context.Context, bool) {
	cwd := "."
	ctx, err := resolver.Resolve(cwd)
	if err != nil {
		var ce *context.Error
		if context.IsContextError(err, &ce) {
			if ce.Code == config.ErrNoProjectResolved {
				fmt.Fprintf(syncStderr, "jirafs sync: no project resolved for cwd %q\n", cwd)
				if len(ce.Candidates) > 0 {
					fmt.Fprintln(syncStderr, "Available projects:")
					for _, name := range ce.Candidates {
						fmt.Fprintf(syncStderr, "  - %s\n", name)
					}
				}
				return nil, false
			}
			fmt.Fprintf(syncStderr, "jirafs sync: %v\n", err)
			return nil, false
		}
		fmt.Fprintf(syncStderr, "jirafs sync: %v\n", err)
		return nil, false
	}
	return ctx, true
}

// printSyncHelp prints usage information for the sync subcommand.
func printSyncHelp() {
	fmt.Fprintf(syncStdout, "%s\n", color.BoldBlue(syncStdout, "Usage:"))
	fmt.Fprintf(syncStdout, "  jirafs %s <%s> [flags]\n\n", color.Blue(syncStdout, "sync"), color.Yellow(syncStdout, "issue-key"))

	fmt.Fprintf(syncStdout, "%s\n\n", color.Dim(syncStdout, "An issue key is required: syncing multiple issues is not implemented yet."))

	fmt.Fprintf(syncStdout, "%s\n", color.Dim(syncStdout, "When called with an issue key, fetches the remote issue from Jira, reads"))
	fmt.Fprintf(syncStdout, "%s\n", color.Dim(syncStdout, "the local copy from the file system, builds and validates a plan, applies"))
	fmt.Fprintf(syncStdout, "%s\n\n", color.Dim(syncStdout, "the plan, and pushes the updated remote back to Jira through the real"))
	fmt.Fprintf(syncStdout, "%s\n\n", color.Dim(syncStdout, "service path."))

	fmt.Fprintf(syncStdout, "%s:\n", color.BoldGreen(syncStdout, "Flags"))
	fmt.Fprintf(syncStdout, "  %s KEY    %s\n", color.Yellow(syncStdout, "--project"), color.Dim(syncStdout, "project key or name to use"))
	fmt.Fprintf(syncStdout, "  %s        %s\n", color.Yellow(syncStdout, "--apply"), color.Dim(syncStdout, "write the updated remote to the local issue file"))
	fmt.Fprintf(syncStdout, "  %s      %s\n\n", color.Yellow(syncStdout, "--dry-run"), color.Dim(syncStdout, "print the plan; no Jira update and no local write"))

	fmt.Fprintf(syncStdout, "%s\n", color.Cyan(syncStdout, `Run "jirafs sync --help" for more information about flags.`))
}
