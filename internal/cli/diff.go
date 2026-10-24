// Package cli provides command implementations for the jirafs CLI.
package cli

import (
	stdcontext "context"
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
)

var (
	diffStdout        io.Writer = os.Stdout
	diffStderr        io.Writer = os.Stderr
	diffClientFactory           = newJiraClient
)

// RunDiff dispatches the `jirafs diff [KEY...]` subcommand. It resolves the
// project context, fetches each remote issue from Jira, reads the local issue
// from the file system, and prints the operations that would bring the remote
// in line with the local copy. It never writes anything.
//
// Without keys it diffs every local issue file that changed since its last
// sync (files without a recorded synced_hash are always included). The exit
// code is 0 whether or not there are differences, unless --exit-code is given,
// in which case it is 1 when any issue differs (like git diff --exit-code).
func RunDiff(args []string) int {
	cmdCtx, stop := commandContext()
	defer stop()
	// Check for help before loading settings.
	if wantsHelpOrWord(args) {
		printDiffHelp()
		return 0
	}

	// Parse flags first to extract --project and --cwd.
	fs := quietFlagSet("diff")
	projectFlag := fs.String("project", "", "project key or name to use")
	cwdFlag := fs.String("cwd", "", "working directory to use for project resolution")
	exitCodeFlag := fs.Bool("exit-code", false, "exit 1 when any issue differs from Jira")
	if err := parseFlags(fs, args); err != nil {
		return flagError(diffStderr, "jirafs diff", err)
	}

	// Load settings and create resolver.
	settings, ok := loadSettingsOrHint(diffStderr, "diff")
	if !ok {
		return 1
	}

	var changed int
	var rc int
	if len(fs.Args()) == 0 {
		changed, rc = runDiffAll(cmdCtx, settings, *projectFlag, *cwdFlag)
	} else {
		changed, rc = runDiffKeys(cmdCtx, fs.Args(), *projectFlag, *cwdFlag, settings)
	}
	if rc != 0 {
		return rc
	}
	if *exitCodeFlag && changed > 0 {
		return 1
	}
	return 0
}

// diffSummary prints the closing one-line summary.
func diffSummary(changed int) {
	fmt.Fprintf(diffStdout, "jirafs diff: %d issue(s) changed\n", changed)
}

// runDiffKeys handles `jirafs diff KEY...`. It resolves the project context
// once, then for each key fetches the remote issue from Jira, reads the local
// issue from the file system, and prints the operations. It returns the
// number of issues with differences and a non-zero code on any error.
func runDiffKeys(cmdCtx stdcontext.Context, keys []string, project, cwd string, settings *config.Settings) (int, int) {
	if cwd == "" {
		cwd = "."
	}

	ctx, ok := resolveDiffContext(settings, project, cwd)
	if !ok {
		return 0, 1
	}

	// Create the Jira client.
	client, err := diffClientFactory(settings, ctx, cwd)
	if err != nil {
		fmt.Fprintf(diffStderr, "jirafs diff: cannot create Jira client: %v\n", err)
		return 0, 1
	}

	changed, rc := 0, 0
	for _, key := range keys {
		n, ok := diffIssueKey(cmdCtx, client, ctx, settings, key)
		if !ok {
			rc = 1
			continue
		}
		if n > 0 {
			changed++
		}
	}
	if rc == 0 {
		diffSummary(changed)
	}
	return changed, rc
}

// diffIssueKey diffs one issue and prints the result. ok is false on error.
func diffIssueKey(cmdCtx stdcontext.Context, client jira.Client, ctx *context.Context, settings *config.Settings, key string) (int, bool) {
	// Fetch the remote issue from Jira.
	remote, err := client.FetchIssue(cmdCtx, key)
	if err != nil {
		fmt.Fprintf(diffStderr, "jirafs diff: %v\n", err)
		return 0, false
	}

	// Read the local issue from the file system using the project's local_dirs.
	localIssue, localPath, err := readLocalIssue(ctx, settings, key)
	if err != nil {
		fmt.Fprintf(diffStderr, "jirafs diff: %v\n", err)
		return 0, false
	}

	// Frontmatter status is context; empty means "no change".
	localIssue.Status = localIssue.Context.Status
	if remote.Status == "" && localIssue.Status != "" {
		fmt.Fprintf(diffStderr, "jirafs diff: warning: remote status of %s is unknown; not planning a status change\n", key)
	}

	// Build the plan by comparing local and remote.
	ops, _, err := plan.BuildPlan(localIssue, *remote)
	if err != nil {
		fmt.Fprintf(diffStderr, "jirafs diff: %v\n", err)
		return 0, false
	}

	if len(ops) == 0 {
		fmt.Fprintf(diffStdout, "jirafs diff: issue %s in %s: no changes needed\n",
			key, localPath)
		return 0, true
	}

	// Sort operations deterministically by field type.
	sort.Slice(ops, func(i, j int) bool {
		return ops[i].Field < ops[j].Field
	})

	fmt.Fprintf(diffStdout, "jirafs diff: %d operation(s) for issue %s in %s:\n",
		len(ops), key, localPath)
	for _, op := range ops {
		fmt.Fprintf(diffStdout, "  %s %s = %s\n", op.Type, op.Field, op.Value)
	}

	return len(ops), true
}

// readLocalIssue reads the local issue file from the project's local_dirs
// and parses it using schema.ParseIssue. It returns the parsed issue and
// the file's path inside the matching local dir.
func readLocalIssue(ctx *context.Context, settings *config.Settings, key string) (schema.Issue, string, error) {
	proj, ok := settings.Projects[ctx.Name]
	if !ok {
		return schema.Issue{}, "", fmt.Errorf("project %q not found in settings", ctx.Name)
	}

	fileName := key + ".md"

	for _, localDir := range proj.LocalDirs {
		path := filepath.Join(localDir, fileName)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		issue, pe := schema.ParseIssue(string(data))
		if pe != nil {
			continue
		}
		if string(issue.Identity.Key) == key {
			return issue, path, nil
		}
	}

	return schema.Issue{}, "", fmt.Errorf("issue %s not found in local directories", key)
}

// resolveDiffContext resolves the project context for the diff command.
func resolveDiffContext(settings *config.Settings, project, cwd string) (*context.Context, bool) {
	res := context.NewResolver(settings, project)
	ctx, err := res.Resolve(cwd)
	if err != nil {
		var ce *context.Error
		if context.IsContextError(err, &ce) {
			if ce.Code == config.ErrNoProjectResolved {
				fmt.Fprintf(diffStderr, "jirafs diff: no project resolved for cwd %q\n", cwd)
				if len(ce.Candidates) > 0 {
					fmt.Fprintln(diffStderr, "Available projects:")
					for _, name := range ce.Candidates {
						fmt.Fprintf(diffStderr, "  - %s\n", name)
					}
				}
				return nil, false
			}
			fmt.Fprintf(diffStderr, "jirafs diff: %v\n", err)
			return nil, false
		}
		fmt.Fprintf(diffStderr, "jirafs diff: %v\n", err)
		return nil, false
	}
	return ctx, true
}

// runDiffAll resolves the project context and diffs every local issue file
// that changed since its last sync (no issue key provided). Files without a
// recorded synced_hash cannot be classified and are always diffed.
func runDiffAll(cmdCtx stdcontext.Context, settings *config.Settings, project, cwd string) (int, int) {
	ctx, ok := resolveDiffContext(settings, project, cwd)
	if !ok {
		return 0, 1
	}

	proj, ok := settings.Projects[ctx.Name]
	if !ok {
		fmt.Fprintf(diffStderr, "jirafs diff: project %q not found in settings\n", ctx.Name)
		return 0, 1
	}

	if len(proj.LocalDirs) == 0 {
		fmt.Fprintln(diffStdout, "jirafs diff: no local directories configured for project", ctx.Name)
		return 0, 0
	}

	// Collect all local issue files.
	var issueFiles []string
	for _, localDir := range proj.LocalDirs {
		entries, err := os.ReadDir(localDir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
				continue
			}
			issueFiles = append(issueFiles, filepath.Join(localDir, entry.Name()))
		}
	}

	if len(issueFiles) == 0 {
		fmt.Fprintf(diffStdout, "jirafs diff: no issue files found in %s\n",
			strings.Join(proj.LocalDirs, ", "))
		return 0, 0
	}

	// Sort files deterministically by path.
	sort.Strings(issueFiles)

	var totalOps, totalIssues, changedIssues, parsed int

	for _, filePath := range issueFiles {
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}
		doc := string(data)
		localIssue, pe := schema.ParseIssue(doc)
		if pe != nil {
			// Skip unparseable files.
			continue
		}

		parsed++

		// Files unchanged since their last sync cannot differ locally.
		if synced := schema.SyncedHash(doc); synced != "" && synced == schema.ContentHash(doc) {
			continue
		}

		// Frontmatter status is context; empty means "no change".
		localIssue.Status = localIssue.Context.Status

		// Fetch the remote issue.
		client, err := diffClientFactory(settings, ctx, cwd)
		if err != nil {
			fmt.Fprintf(diffStderr, "jirafs diff: %s: cannot create client: %v\n", filePath, err)
			continue
		}

		remote, err := client.FetchIssue(cmdCtx, string(localIssue.Identity.Key))
		if err != nil {
			// Remote issue not found or fetch error - skip.
			continue
		}

		if remote.Status == "" && localIssue.Status != "" {
			fmt.Fprintf(diffStderr, "jirafs diff: %s: warning: remote status unknown; not planning a status change\n", filePath)
		}

		ops, _, err := plan.BuildPlan(localIssue, *remote)
		if err != nil {
			fmt.Fprintf(diffStderr, "jirafs diff: %s: %v\n", filePath, err)
			continue
		}

		totalIssues++
		totalOps += len(ops)

		if len(ops) == 0 {
			fmt.Fprintf(diffStdout, "  %s: no changes needed\n", localIssue.Identity.Key)
			continue
		}
		changedIssues++
		// Sort operations deterministically by field type.
		sort.Slice(ops, func(i, j int) bool {
			return ops[i].Field < ops[j].Field
		})
		fmt.Fprintf(diffStdout, "  %s: %d operation(s)\n", localIssue.Identity.Key, len(ops))
		for _, op := range ops {
			fmt.Fprintf(diffStdout, "    %s %s = %s\n", op.Type, op.Field, op.Value)
		}
	}

	if parsed == 0 {
		fmt.Fprintln(diffStdout, "jirafs diff: no parseable issue files found")
		return 0, 0
	}

	if totalIssues == 0 {
		fmt.Fprintf(diffStdout, "jirafs diff: no locally changed issues in %s\n", ctx.Name)
	} else if totalOps == 0 {
		fmt.Fprintf(diffStdout, "jirafs diff: %d issue(s) in %s: all up to date\n",
			totalIssues, ctx.Name)
	} else {
		fmt.Fprintf(diffStdout, "jirafs diff: %d issue(s) in %s: %d total operation(s)\n",
			totalIssues, ctx.Name, totalOps)
	}
	diffSummary(changedIssues)

	return changedIssues, 0
}

// printDiffHelp prints usage information for the diff subcommand.
func printDiffHelp() {
	fmt.Fprintf(diffStdout, "%s\n", color.BoldBlue(diffStdout, "Usage:"))
	fmt.Fprintf(diffStdout, "  jirafs %s [%s...] [flags]\n\n", color.Blue(diffStdout, "diff"), color.Yellow(diffStdout, "issue-key"))

	fmt.Fprintf(diffStdout, "%s\n", color.Dim(diffStdout, "Fetches each remote issue from Jira, reads the local copy from the file"))
	fmt.Fprintf(diffStdout, "%s\n", color.Dim(diffStdout, "system, and prints the operations needed to bring the remote in line"))
	fmt.Fprintf(diffStdout, "%s\n", color.Dim(diffStdout, "with the local copy. Without a key, diffs every locally changed issue."))
	fmt.Fprintf(diffStdout, "%s\n\n", color.Dim(diffStdout, "Read-only: nothing is written to Jira or to local files."))

	fmt.Fprintf(diffStdout, "%s:\n", color.BoldGreen(diffStdout, "Flags"))
	fmt.Fprintf(diffStdout, "  %s KEY   %s\n", color.Yellow(diffStdout, "--project"), color.Dim(diffStdout, "project key or name to use"))
	fmt.Fprintf(diffStdout, "  %s DIR   %s\n", color.Yellow(diffStdout, "--cwd"), color.Dim(diffStdout, "working directory for project resolution"))
	fmt.Fprintf(diffStdout, "  %s   %s\n\n", color.Yellow(diffStdout, "--exit-code"), color.Dim(diffStdout, "exit 1 when any issue differs (default exit 0)"))

	fmt.Fprintf(diffStdout, "%s\n", color.Cyan(diffStdout, `Run "jirafs diff --help" for more information about flags.`))
}
