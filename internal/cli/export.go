// Package cli provides command implementations for the jirafs CLI.
package cli

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/jirafs/jirafs/internal/config"
	"github.com/jirafs/jirafs/internal/context"
	"github.com/jirafs/jirafs/internal/export"
	"github.com/jirafs/jirafs/internal/jira"
)

var (
	exportStdout   io.Writer = os.Stdout
	exportStderr   io.Writer = os.Stderr
	exportClientFactory = buildExportClient
)

// RunExport dispatches the `jirafs export` subcommand. It supports:
//
//	"jirafs export issue KEY" → fetch and export one issue
//	"jirafs export help"      → show help text
func RunExport(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(exportStderr, "jirafs export: missing subcommand. Use --help for usage.")
		return 1
	}

	// Check for help before loading settings.
	if args[0] == "help" {
		printExportHelp()
		return 0
	}

	// Load settings and create resolver.
	settings, err := config.Load()
	if err != nil {
		fmt.Fprintf(exportStderr, "jirafs export: cannot load settings: %v\n", err)
		return 1
	}
	resolver := context.NewResolver(settings, "")

	switch args[0] {
	case "issue":
		return runExportIssue(args[1:], settings, resolver)
	default:
		fmt.Fprintf(exportStderr, "jirafs export: unknown subcommand %q. Use --help for usage.\n", args[0])
		return 1
	}
}

// runExportIssue handles `jirafs export issue KEY`. It resolves the project
// context, fetches the issue from Jira through the real service path, and
// exports it through the canonical codec.
func runExportIssue(args []string, settings *config.Settings, resolver *context.Resolver) int {
	fs := flag.NewFlagSet("export issue", flag.ExitOnError)
	// --project overrides the auto-detected project.
	projectFlag := fs.String("project", "", "project key or name to use")
	// --cwd overrides the working directory for project resolution.
	cwdFlag := fs.String("cwd", "", "working directory to use for project resolution")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(exportStderr, "jirafs export issue: invalid flags: %v\n", err)
		return 1
	}

	if len(fs.Args()) == 0 {
		fmt.Fprintln(exportStderr, "jirafs export issue: missing issue key")
		return 1
	}
	if len(fs.Args()) > 1 {
		fmt.Fprintln(exportStderr, "jirafs export issue: too many positional arguments")
		return 1
	}
	key := fs.Args()[0]

	cwd := "."
	if *cwdFlag != "" {
		cwd = *cwdFlag
	}

	ctx, ok := resolveExportContext(settings, *projectFlag, cwd)
	if !ok {
		return 1
	}

	// Create the Jira client.
	client, err := exportClientFactory(settings, ctx, cwd)
	if err != nil {
		fmt.Fprintf(exportStderr, "jirafs export issue: cannot create Jira client: %v\n", err)
		return 1
	}

	// Fetch the issue from Jira.
	issue, err := client.FetchIssue(nil, key)
	if err != nil {
		fmt.Fprintf(exportStderr, "jirafs export issue: %v\n", err)
		return 1
	}

	// Export through the canonical codec.
	output := export.ExportIssue(issue)
	fmt.Fprint(exportStdout, output)
	return 0
}

// resolveExportContext resolves the project context for the export command.
func resolveExportContext(settings *config.Settings, project, cwd string) (*context.Context, bool) {
	res := context.NewResolver(settings, project)
	ctx, err := res.Resolve(cwd)
	if err != nil {
		var ce *context.Error
		if context.IsContextError(err, &ce) {
			if ce.Code == config.ErrNoProjectResolved {
				fmt.Fprintf(exportStderr, "jirafs export issue: no project resolved for cwd %q\n", cwd)
				if len(ce.Candidates) > 0 {
					fmt.Fprintln(exportStderr, "Available projects:")
					for _, name := range ce.Candidates {
						fmt.Fprintf(exportStderr, "  - %s\n", name)
					}
				}
				return nil, false
			}
			fmt.Fprintf(exportStderr, "jirafs export issue: %v\n", err)
			return nil, false
		}
		fmt.Fprintf(exportStderr, "jirafs export issue: %v\n", err)
		return nil, false
	}
	return ctx, true
}

func buildExportClient(settings *config.Settings, ctx *context.Context, cwd string) (jira.Client, error) {
	creds, err := settings.ResolveInstanceCredentials(ctx.Instance)
	if err != nil {
		return nil, err
	}
	client := jira.NewJiraClient(creds.BaseURL)
	client.SetCredentials(creds)
	return client, nil
}

// printExportHelp prints usage information for the export subcommand.
func printExportHelp() {
	fmt.Fprintln(exportStderr, `Usage:
  jirafs export <subcommand> [flags]

Subcommands:
  issue       export one issue through the real service path
  help        show this help message

Flags:
  --project KEY   project key or name to use
  --cwd DIR       working directory for project resolution

Run "jirafs export <subcommand> --help" for more information about a subcommand.`)
}
