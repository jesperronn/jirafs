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
)

var (
	planStdout   io.Writer = os.Stdout
	planStderr   io.Writer = os.Stderr
	planClientFactory  = buildPlanClient
)

// RunPlan dispatches the `jirafs plan` subcommand. It resolves the project
// context, fetches the remote issue from Jira, reads the local issue from
// the file system, and displays the plan of operations needed to bring the
// remote in line with the local copy.
//
// When called without an issue key, RunPlan resolves the project context
// and lists all local issues with their planned operations.
func RunPlan(args []string) int {
	// Check for help before loading settings.
	if len(args) > 0 && (args[0] == "help" || args[0] == "--help" || args[0] == "-h") {
		printPlanHelp()
		return 0
	}

	// Load settings and create resolver.
	settings, err := config.Load()
	if err != nil {
		fmt.Fprintf(planStderr, "jirafs plan: cannot load settings: %v\n", err)
		return 1
	}

	// Parse flags first to extract --project and --cwd.
	fs := flag.NewFlagSet("plan", flag.ContinueOnError)
	projectFlag := fs.String("project", "", "project key or name to use")
	cwdFlag := fs.String("cwd", "", "working directory to use for project resolution")
	_ = fs.Parse(args)

	// When no issue key is provided, resolve the project and list all local issues.
	if len(fs.Args()) == 0 {
		return runPlanList(settings, *projectFlag, *cwdFlag)
	}

	key := ""
	if len(fs.Args()) > 0 {
		key = fs.Args()[0]
	}
	if len(fs.Args()) > 1 {
		fmt.Fprintln(planStderr, "jirafs plan: too many positional arguments")
		return 1
	}
	return runPlanIssue(key, *projectFlag, *cwdFlag, settings)
}

// runPlanIssue handles `jirafs plan KEY`. It resolves the project context,
// fetches the remote issue from Jira through the real service path, reads
// the local issue from the file system, and displays the plan of operations.
// project and cwd are already parsed by the caller.
func runPlanIssue(key, project, cwd string, settings *config.Settings) int {
	if key == "" {
		fmt.Fprintln(planStderr, "jirafs plan: missing issue key")
		return 1
	}

	if cwd == "" {
		cwd = "."
	}

	ctx, ok := resolvePlanContext(settings, project, cwd)
	if !ok {
		return 1
	}

	// Create the Jira client.
	client, err := planClientFactory(settings, ctx, cwd)
	if err != nil {
		fmt.Fprintf(planStderr, "jirafs plan: cannot create Jira client: %v\n", err)
		return 1
	}

	// Fetch the remote issue from Jira.
	remote, err := client.FetchIssue(nil, key)
	if err != nil {
		fmt.Fprintf(planStderr, "jirafs plan: %v\n", err)
		return 1
	}

	// Read the local issue from the file system using the project's local_dirs.
	localIssue, localPath, err := readLocalIssue(ctx, settings, key)
	if err != nil {
		fmt.Fprintf(planStderr, "jirafs plan: %v\n", err)
		return 1
	}

	// Build the plan by comparing local and remote.
	ops, _, err := plan.BuildPlan(localIssue, *remote)
	if err != nil {
		fmt.Fprintf(planStderr, "jirafs plan: %v\n", err)
		return 1
	}

	if len(ops) == 0 {
		fmt.Fprintf(planStdout, "jirafs plan: issue %s in %s: no changes needed\n",
			key, localPath)
		return 0
	}

	// Sort operations deterministically by field type.
	sort.Slice(ops, func(i, j int) bool {
		return ops[i].Field < ops[j].Field
	})

	fmt.Fprintf(planStdout, "jirafs plan: %d operation(s) for issue %s in %s:\n",
		len(ops), key, localPath)
	for _, op := range ops {
		fmt.Fprintf(planStdout, "  %s %s = %s\n", op.Type, op.Field, op.Value)
	}

	return 0
}

// readLocalIssue reads the local issue file from the project's local_dirs
// and parses it using schema.ParseIssue. It returns the parsed issue and
// the relative file path.
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
			return issue, fileName, nil
		}
	}

	return schema.Issue{}, "", fmt.Errorf("issue %s not found in local directories", key)
}

// resolvePlanContext resolves the project context for the plan command.
func resolvePlanContext(settings *config.Settings, project, cwd string) (*context.Context, bool) {
	res := context.NewResolver(settings, project)
	ctx, err := res.Resolve(cwd)
	if err != nil {
		var ce *context.Error
		if context.IsContextError(err, &ce) {
			if ce.Code == config.ErrNoProjectResolved {
				fmt.Fprintf(planStderr, "jirafs plan: no project resolved for cwd %q\n", cwd)
				if len(ce.Candidates) > 0 {
					fmt.Fprintln(planStderr, "Available projects:")
					for _, name := range ce.Candidates {
						fmt.Fprintf(planStderr, "  - %s\n", name)
					}
				}
				return nil, false
			}
			fmt.Fprintf(planStderr, "jirafs plan: %v\n", err)
			return nil, false
		}
		fmt.Fprintf(planStderr, "jirafs plan: %v\n", err)
		return nil, false
	}
	return ctx, true
}

// runPlanList resolves the project context and lists all local issues
// with their planned operations (no issue key provided).
func runPlanList(settings *config.Settings, project, cwd string) int {
	ctx, ok := resolvePlanContext(settings, project, cwd)
	if !ok {
		return 1
	}

	proj, ok := settings.Projects[ctx.Name]
	if !ok {
		fmt.Fprintf(planStderr, "jirafs plan: project %q not found in settings\n", ctx.Name)
		return 1
	}

	if len(proj.LocalDirs) == 0 {
		fmt.Fprintln(planStdout, "jirafs plan: no local directories configured for project", ctx.Name)
		return 0
	}

	// Collect all local issue files.
	var issueFiles []string
	for _, localDir := range proj.LocalDirs {
		entries, err := os.ReadDir(localDir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if len(name) <= 3 || name[len(name)-3:] != ".md" {
				continue
			}
			issueFiles = append(issueFiles, filepath.Join(localDir, name))
		}
	}

	if len(issueFiles) == 0 {
		fmt.Fprintf(planStdout, "jirafs plan: no issue files found in %s\n",
			strings.Join(proj.LocalDirs, ", "))
		return 0
	}

	// Sort files deterministically by path.
	sort.Strings(issueFiles)

	var totalOps int
	var totalIssues int

	for _, filePath := range issueFiles {
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}
		localIssue, pe := schema.ParseIssue(string(data))
		if pe != nil {
			// Skip unparseable files.
			continue
		}

		// Fetch the remote issue.
		client, err := planClientFactory(settings, ctx, cwd)
		if err != nil {
			fmt.Fprintf(planStderr, "jirafs plan: %s: cannot create client: %v\n", filePath, err)
			continue
		}

		remote, err := client.FetchIssue(nil, string(localIssue.Identity.Key))
		if err != nil {
			// Remote issue not found or fetch error - skip.
			continue
		}

		// Build the plan.
		ops, _, err := plan.BuildPlan(localIssue, *remote)
		if err != nil {
			fmt.Fprintf(planStderr, "jirafs plan: %s: %v\n", filePath, err)
			continue
		}

		totalIssues++
		totalOps += len(ops)

		if len(ops) == 0 {
			fmt.Fprintf(planStdout, "  %s: no changes needed\n", localIssue.Identity.Key)
		} else {
			// Sort operations deterministically by field type.
			sort.Slice(ops, func(i, j int) bool {
				return ops[i].Field < ops[j].Field
			})
			fmt.Fprintf(planStdout, "  %s: %d operation(s)\n", localIssue.Identity.Key, len(ops))
			for _, op := range ops {
				fmt.Fprintf(planStdout, "    %s %s = %s\n", op.Type, op.Field, op.Value)
			}
		}
	}

	if totalIssues == 0 {
		fmt.Fprintln(planStdout, "jirafs plan: no parseable issue files found")
		return 0
	}

	if totalOps == 0 {
		fmt.Fprintf(planStdout, "jirafs plan: %d issue(s) in %s: all up to date\n",
			totalIssues, ctx.Name)
	} else {
		fmt.Fprintf(planStdout, "jirafs plan: %d issue(s) in %s: %d total operation(s)\n",
			totalIssues, ctx.Name, totalOps)
	}

	return 0
}

func buildPlanClient(settings *config.Settings, ctx *context.Context, cwd string) (jira.Client, error) {
	creds, err := settings.ResolveInstanceCredentials(ctx.Instance)
	if err != nil {
		return nil, err
	}
	client := jira.NewJiraClient(creds.BaseURL)
	client.SetCredentials(creds)
	return client, nil
}

// printPlanHelp prints usage information for the plan subcommand.
func printPlanHelp() {
	fmt.Fprintf(planStderr, "%s\n", color.BoldBlue(planStderr, "Usage:"))
	fmt.Fprintf(planStderr, "  jirafs %s <%s> [flags]\n\n", color.Blue(planStderr, "plan"), color.Yellow(planStderr, "issue-key"))

	fmt.Fprintf(planStderr, "%s\n", color.Dim(planStderr, "Fetches the remote issue from Jira, reads the local copy from the file"))
	fmt.Fprintf(planStderr, "%s\n", color.Dim(planStderr, "system, and displays the plan of operations needed to bring the remote"))
	fmt.Fprintf(planStderr, "%s\n\n", color.Dim(planStderr, "in line with the local copy."))

	fmt.Fprintf(planStderr, "%s:\n", color.BoldGreen(planStderr, "Flags"))
	fmt.Fprintf(planStderr, "  %s KEY   %s\n", color.Yellow(planStderr, "--project"), color.Dim(planStderr, "project key or name to use"))
	fmt.Fprintf(planStderr, "  %s DIR   %s\n\n", color.Yellow(planStderr, "--cwd"), color.Dim(planStderr, "working directory for project resolution"))

	fmt.Fprintf(planStderr, "%s\n", color.Cyan(planStderr, `Run "jirafs plan <issue-key> --help" for more information about flags.`))
}
