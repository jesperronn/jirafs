// Package cli provides command implementations for the jirafs CLI.
package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
	syncStdout   io.Writer = os.Stdout
	syncStderr   io.Writer = os.Stderr
	syncClientFactory  = buildSyncClient
	syncBuildClient  = buildSyncClient
)

// RunSync dispatches the `jirafs sync` subcommand. When an issue key is
// provided, it resolves the project context, fetches the remote issue from
// Jira, reads the local issue from the file system, builds and validates a
// plan, applies the plan, and pushes the updated remote back to Jira through
// the real service path. When no issue key is provided, it resolves the
// project context and lists all local issues with their pending sync
// operations, allowing the user to preview what would be synced.
func RunSync(args []string) int {
	// Check for help before loading settings.
	if len(args) > 0 && (args[0] == "help" || args[0] == "--help" || args[0] == "-h") {
		printSyncHelp()
		return 0
	}

	// Load settings.
	settings, err := config.Load()
	if err != nil {
		fmt.Fprintf(syncStderr, "jirafs sync: cannot load settings: %v\n", err)
		return 1
	}

	// Parse flags to extract --project and --cwd regardless of whether
	// an issue key is provided. This allows "jirafs sync --project TEST"
	// (no key) to resolve the project and list pending syncs.
	fs := flag.NewFlagSet("sync", flag.ExitOnError)
	projectFlag := fs.String("project", "", "project key or name to use")
	cwdFlag := fs.String("cwd", "", "working directory to use for project resolution")
	applyFlag := fs.Bool("apply", false, "write the updated remote to the local issue file")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(syncStderr, "jirafs sync: invalid flags: %v\n", err)
		return 1
	}

	// If no positional arguments (no issue key), resolve the project from
	// the parsed flags and list pending syncs.
	if len(fs.Args()) == 0 {
		return runSyncAll(settings, *projectFlag, *cwdFlag)
	}

	return runSyncIssue(fs.Args(), settings, context.NewResolver(settings, *projectFlag), *applyFlag)
}

// runSyncIssue handles `jirafs sync KEY`. It resolves the project context,
// fetches the remote issue from Jira through the real service path, reads
// the local issue from the file system, builds and validates a plan,
// applies the plan, and pushes the updated remote back to Jira.
func runSyncIssue(args []string, settings *config.Settings, resolver *context.Resolver, apply bool) int {
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

	if apply {
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
	fmt.Fprintf(syncStderr, "%s\n", color.BoldBlue(syncStderr, "Usage:"))
	fmt.Fprintf(syncStderr, "  jirafs %s [flags]\n", color.Blue(syncStderr, "sync"))
	fmt.Fprintf(syncStderr, "  jirafs %s <%s> [flags]\n\n", color.Blue(syncStderr, "sync"), color.Yellow(syncStderr, "issue-key"))

	fmt.Fprintf(syncStderr, "%s\n", color.Dim(syncStderr, "When called without an issue key, resolves the project context and lists all"))
	fmt.Fprintf(syncStderr, "%s\n", color.Dim(syncStderr, "local issues with their pending sync operations, allowing preview of what"))
	fmt.Fprintf(syncStderr, "%s\n\n", color.Dim(syncStderr, "would be synced."))

	fmt.Fprintf(syncStderr, "%s\n", color.Dim(syncStderr, "When called with an issue key, fetches the remote issue from Jira, reads"))
	fmt.Fprintf(syncStderr, "%s\n", color.Dim(syncStderr, "the local copy from the file system, builds and validates a plan, applies"))
	fmt.Fprintf(syncStderr, "%s\n\n", color.Dim(syncStderr, "the plan, and pushes the updated remote back to Jira through the real"))
	fmt.Fprintf(syncStderr, "%s\n\n", color.Dim(syncStderr, "service path."))

	fmt.Fprintf(syncStderr, "%s:\n", color.BoldGreen(syncStderr, "Flags"))
	fmt.Fprintf(syncStderr, "  %s KEY    %s\n", color.Yellow(syncStderr, "--project"), color.Dim(syncStderr, "project key or name to use"))
	fmt.Fprintf(syncStderr, "  %s DIR    %s\n", color.Yellow(syncStderr, "--cwd"), color.Dim(syncStderr, "working directory for project resolution"))
	fmt.Fprintf(syncStderr, "  %s        %s\n\n", color.Yellow(syncStderr, "--apply"), color.Dim(syncStderr, "write the updated remote to the local issue file"))

	fmt.Fprintf(syncStderr, "%s\n", color.Cyan(syncStderr, `Run "jirafs sync --help" for more information about flags.`))
}

// runSyncAll resolves the project context and lists all local issues with
// their pending sync operations. It resolves the project using the same
// resolution order as runSyncIssue (explicit --project, cwd mapping,
// remembered state), then iterates over all local directories to find
// issue files, compares each against the remote, and reports what would
// be synced.
func runSyncAll(settings *config.Settings, project, cwd string) int {
	// Resolve the project context.
	if cwd == "" {
		cwd = "."
	}
	resolver := context.NewResolver(settings, project)
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
				return 1
			}
			fmt.Fprintf(syncStderr, "jirafs sync: %v\n", err)
			return 1
		}
		fmt.Fprintf(syncStderr, "jirafs sync: %v\n", err)
		return 1
	}

	// Get the project from settings.
	proj, ok := settings.Projects[ctx.Name]
	if !ok {
		fmt.Fprintf(syncStderr, "jirafs sync: project %q not found in settings\n", ctx.Name)
		return 1
	}

	if len(proj.LocalDirs) == 0 {
		fmt.Fprintf(syncStderr, "jirafs sync: project %q has no local directories configured\n", ctx.Name)
		return 1
	}

	// Collect all issue files from local directories.
	type issueInfo struct {
		key     string
		path    string
		hasDiff bool
		ops     []schema.PlanOperation
	}

	var issues []issueInfo

	for _, localDir := range proj.LocalDirs {
		entries, err := os.ReadDir(localDir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			if !strings.HasSuffix(entry.Name(), ".md") {
				continue
			}
			filePath := filepath.Join(localDir, entry.Name())
			data, err := os.ReadFile(filePath)
			if err != nil {
				continue
			}
			issue, pe := schema.ParseIssue(string(data))
			if pe != nil {
				continue
			}
			key := string(issue.Identity.Key)
			if key == "" {
				continue
			}

			// Fetch the remote issue to compare.
			client, err := syncBuildClient(settings, ctx, cwd)
			if err != nil {
				fmt.Fprintf(syncStderr, "DEBUG: buildSyncClient error: %v\n", err)
				// Skip issues we can't fetch remotely.
				continue
			}
			remote, err := client.FetchIssue(nil, key)
			if err != nil {
				// Remote not found — still report it as needing attention.
				issues = append(issues, issueInfo{
					key:     key,
					path:    entry.Name(),
					hasDiff: true,
				})
				continue
			}

			// Build the plan by comparing local and remote.
			ops, _, err := plan.BuildPlan(issue, *remote)
			if err != nil {
				// Unparseable plan — skip.
				continue
			}

			issues = append(issues, issueInfo{
				key:     key,
				path:    entry.Name(),
				hasDiff: len(ops) > 0,
				ops:     ops,
			})
		}
	}

	if len(issues) == 0 {
		fmt.Fprintf(syncStdout, "jirafs sync: project %s: no local issues found\n", ctx.Name)
		return 0
	}

	// Report each issue.
	var needsSync int
	for _, info := range issues {
		if info.hasDiff {
			needsSync++
		}
		if len(info.ops) > 0 {
			fmt.Fprintf(syncStdout, "  %s (%s): %d operation(s) pending\n",
				info.key, info.path, len(info.ops))
		} else {
			fmt.Fprintf(syncStdout, "  %s (%s): up to date\n",
				info.key, info.path)
		}
	}

	fmt.Fprintf(syncStdout, "jirafs sync: project %s: %d of %d issue(s) have pending operations\n",
		ctx.Name, needsSync, len(issues))
	return 0
}
