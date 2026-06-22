// Package cli provides command implementations for the jirafs CLI.
package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

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
func RunPlan(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(planStderr, "jirafs plan: missing issue key. Use --help for usage.")
		return 1
	}

	// Check for help before loading settings.
	if args[0] == "help" {
		printPlanHelp()
		return 0
	}

	// Load settings and create resolver.
	settings, err := config.Load()
	if err != nil {
		fmt.Fprintf(planStderr, "jirafs plan: cannot load settings: %v\n", err)
		return 1
	}
	resolver := context.NewResolver(settings, "")

	return runPlanIssue(args, settings, resolver)
}

// runPlanIssue handles `jirafs plan KEY`. It resolves the project context,
// fetches the remote issue from Jira through the real service path, reads
// the local issue from the file system, and displays the plan of operations.
func runPlanIssue(args []string, settings *config.Settings, resolver *context.Resolver) int {
	fs := flag.NewFlagSet("plan", flag.ExitOnError)
	// --project overrides the auto-detected project.
	projectFlag := fs.String("project", "", "project key or name to use")
	// --cwd overrides the working directory for project resolution.
	cwdFlag := fs.String("cwd", "", "working directory to use for project resolution")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(planStderr, "jirafs plan: invalid flags: %v\n", err)
		return 1
	}

	if len(fs.Args()) == 0 {
		fmt.Fprintln(planStderr, "jirafs plan: missing issue key")
		return 1
	}
	if len(fs.Args()) > 1 {
		fmt.Fprintln(planStderr, "jirafs plan: too many positional arguments")
		return 1
	}
	key := fs.Args()[0]

	cwd := "."
	if *cwdFlag != "" {
		cwd = *cwdFlag
	}

	ctx, ok := resolvePlanContext(settings, *projectFlag, cwd)
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
	fmt.Fprintln(planStderr, `Usage:
  jirafs plan <issue-key> [flags]

Fetches the remote issue from Jira, reads the local copy from the file
system, and displays the plan of operations needed to bring the remote
in line with the local copy.

Flags:
  --project KEY   project key or name to use
  --cwd DIR       working directory for project resolution

Run "jirafs plan <issue-key> --help" for more information about flags.`)
}
