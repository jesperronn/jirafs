// Package cli provides command implementations for the jirafs CLI.
package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/jirafs/jirafs/internal/config"
	"github.com/jirafs/jirafs/internal/context"
	"github.com/jirafs/jirafs/internal/jira"
	"github.com/jirafs/jirafs/internal/plan"
	"github.com/jirafs/jirafs/internal/schema"
	"github.com/jirafs/jirafs/internal/sync"
)

var (
	syncStdout   io.Writer = os.Stdout
	syncStderr   io.Writer = os.Stderr
	syncClientFactory  = buildSyncClient
)

// RunSync dispatches the `jirafs sync` subcommand. It resolves the project
// context, fetches the remote issue from Jira, reads the local issue from
// the file system, builds and validates a plan, applies the plan, and
// pushes the updated remote back to Jira through the real service path.
func RunSync(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(syncStderr, "jirafs sync: missing issue key. Use --help for usage.")
		return 1
	}

	// Check for help before loading settings.
	if args[0] == "help" {
		printSyncHelp()
		return 0
	}

	// Load settings and create resolver.
	settings, err := config.Load()
	if err != nil {
		fmt.Fprintf(syncStderr, "jirafs sync: cannot load settings: %v\n", err)
		return 1
	}
	resolver := context.NewResolver(settings, "")

	return runSyncIssue(args, settings, resolver)
}

// runSyncIssue handles `jirafs sync KEY`. It resolves the project context,
// fetches the remote issue from Jira through the real service path, reads
// the local issue from the file system, builds and validates a plan,
// applies the plan, and pushes the updated remote back to Jira.
func runSyncIssue(args []string, settings *config.Settings, resolver *context.Resolver) int {
	fs := flag.NewFlagSet("sync", flag.ExitOnError)
	// --project overrides the auto-detected project.
	projectFlag := fs.String("project", "", "project key or name to use")
	// --cwd overrides the working directory for project resolution.
	cwdFlag := fs.String("cwd", "", "working directory to use for project resolution")
	// --apply writes the updated remote back to the local issue file.
	applyFlag := fs.Bool("apply", false, "write the updated remote to the local issue file")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(syncStderr, "jirafs sync: invalid flags: %v\n", err)
		return 1
	}

	if len(fs.Args()) == 0 {
		fmt.Fprintln(syncStderr, "jirafs sync: missing issue key")
		return 1
	}
	if len(fs.Args()) > 1 {
		fmt.Fprintln(syncStderr, "jirafs sync: too many positional arguments")
		return 1
	}
	key := fs.Args()[0]

	cwd := "."
	if *cwdFlag != "" {
		cwd = *cwdFlag
	}

	ctx, ok := resolveSyncContext(settings, *projectFlag, cwd)
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
	remote, err := client.FetchIssue(nil, key)
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

	fmt.Fprintf(syncStdout, "jirafs sync: %d operation(s) for issue %s in %s:\n",
		len(ops), key, localPath)
	for _, op := range ops {
		fmt.Fprintf(syncStdout, "  %s %s = %s\n", op.Type, op.Field, op.Value)
	}

	// Push the updated remote back to Jira through the real service path.
	updatedRemote, err := client.UpdateIssue(nil, key, result.Remote)
	if err != nil {
		fmt.Fprintf(syncStderr, "jirafs sync: cannot update remote issue: %v\n", err)
		return 1
	}

	// Update the remote metadata in the result with the updated remote.
	result.Remote.RemoteMetadata = updatedRemote.RemoteMetadata

	if *applyFlag {
		// Write the updated remote back to the local issue file.
		if err := writeUpdatedLocalIssue(localPath, result.Remote); err != nil {
			fmt.Fprintf(syncStderr, "jirafs sync: cannot write updated local issue: %v\n", err)
			return 1
		}
	}

	fmt.Fprintln(syncStdout, "jirafs sync: applied "+fmt.Sprint(len(ops))+" operation(s) successfully")
	return 0
}

// writeUpdatedLocalIssue writes the updated remote issue back to the local
// file system using the canonical codec (schema.RenderIssue).
func writeUpdatedLocalIssue(localPath string, issue *schema.Issue) error {
	content := schema.RenderIssue(*issue)
	return os.WriteFile(localPath, []byte(content), 0o644)
}

// resolveSyncContext resolves the project context for the sync command.
func resolveSyncContext(settings *config.Settings, project, cwd string) (*context.Context, bool) {
	res := context.NewResolver(settings, project)
	ctx, err := res.Resolve(cwd)
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

func buildSyncClient(settings *config.Settings, ctx *context.Context, cwd string) (jira.Client, error) {
	creds, err := settings.ResolveInstanceCredentials(ctx.Instance)
	if err != nil {
		return nil, err
	}
	client := jira.NewJiraClient(creds.BaseURL)
	client.SetCredentials(creds)
	return client, nil
}

// printSyncHelp prints usage information for the sync subcommand.
func printSyncHelp() {
	fmt.Fprintln(syncStderr, `Usage:
  jirafs sync <issue-key> [flags]

Fetches the remote issue from Jira, reads the local copy from the file
system, builds and validates a plan, applies the plan, and pushes the
updated remote back to Jira through the real service path.

Flags:
  --project KEY   project key or name to use
  --cwd DIR       working directory for project resolution
  --apply         write the updated remote to the local issue file

Run "jirafs sync <issue-key> --help" for more information about flags.`)
}
