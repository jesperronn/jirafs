// Package cli provides command implementations for the jirafs CLI.
package cli

import (
	stdcontext "context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jirafs/jirafs/internal/color"
	"github.com/jirafs/jirafs/internal/config"
	"github.com/jirafs/jirafs/internal/context"
	"github.com/jirafs/jirafs/internal/export"
	"github.com/jirafs/jirafs/internal/jira"
)

var (
	exportStdout        io.Writer = os.Stdout
	exportStderr        io.Writer = os.Stderr
	exportClientFactory           = newJiraClient
)

// RunExport dispatches the `jirafs export` subcommand. It supports:
//
//	"jirafs export issue KEY" → fetch and export one issue
//	"jirafs export help"      → show help text
func RunExport(args []string) int {
	cmdCtx, stop := commandContext()
	defer stop()
	if len(args) == 0 {
		return usageError(exportStderr, "jirafs export", "missing subcommand")
	}

	// Help and usage errors never need settings.
	if wantsHelpOrWord(args[:1]) {
		printExportHelp()
		return 0
	}
	switch args[0] {
	case "issue":
		if wantsHelp(args[1:]) {
			printExportIssueHelp()
			return 0
		}
	default:
		return usageError(exportStderr, "jirafs export", "unknown subcommand %q", args[0])
	}

	// Load settings and create resolver.
	settings, ok := loadSettingsOrHint(exportStderr, "export")
	if !ok {
		return 1
	}
	resolver := context.NewResolver(settings, "")

	return runExportIssue(cmdCtx, args[1:], settings, resolver)
}

// runExportIssue handles `jirafs export issue KEY`. It resolves the project
// context, fetches the issue from Jira through the real service path, and
// exports it through the canonical codec.
func runExportIssue(cmdCtx stdcontext.Context, args []string, settings *config.Settings, resolver *context.Resolver) int {
	fs := quietFlagSet("export issue")
	// --project overrides the auto-detected project.
	projectFlag := fs.String("project", "", "project key or name to use")
	// --cwd overrides the working directory for project resolution.
	cwdFlag := fs.String("cwd", "", "working directory to use for project resolution")
	// --stdout prints the document instead of writing a file.
	stdoutFlag := fs.Bool("stdout", false, "print the document to stdout and write no file")
	dryRunFlag := fs.Bool("dry-run", false, "print the target path and whether it would be overwritten; write nothing")
	if err := parseFlags(fs, args); err != nil {
		return flagError(exportStderr, "jirafs export issue", err)
	}

	if len(fs.Args()) == 0 {
		return usageError(exportStderr, "jirafs export issue", "missing issue key")
	}
	if len(fs.Args()) > 1 {
		return usageError(exportStderr, "jirafs export issue", "too many positional arguments")
	}
	arg := fs.Args()[0]

	cwd := "."
	if *cwdFlag != "" {
		cwd = *cwdFlag
	}

	ctx, ok := resolveExportContext(settings, *projectFlag, cwd)
	if !ok {
		return 1
	}

	key, err := jira.ParseIssueRef(arg, settings.Instances[ctx.Instance].BaseURL)
	if err != nil {
		fmt.Fprintf(exportStderr, "jirafs export issue: %s\n", strings.ReplaceAll(err.Error(), "<name>", ctx.Name))
		return 1
	}

	// Create the Jira client.
	client, err := exportClientFactory(settings, ctx, cwd)
	if err != nil {
		fmt.Fprintf(exportStderr, "jirafs export issue: cannot create Jira client: %v\n", err)
		return 1
	}

	// Fetch the issue from Jira.
	issue, err := client.FetchIssue(cmdCtx, key)
	if err != nil {
		fmt.Fprintf(exportStderr, "jirafs export issue: %v\n", err)
		return 1
	}

	jira.AttachComments(cmdCtx, client, issue, exportStderr)

	// Export through the canonical codec.
	output := export.ExportIssue(issue)
	if *stdoutFlag {
		fmt.Fprint(exportStdout, output)
		return 0
	}

	proj, ok := settings.Projects[ctx.Name]
	if !ok || len(proj.LocalDirs) == 0 {
		fmt.Fprintf(exportStderr, "jirafs export issue: project %q has no local_dirs configured; use --stdout to print instead\n", ctx.Name)
		return 1
	}
	if *dryRunFlag {
		dest, err := filepath.Abs(filepath.Join(proj.LocalDirs[0], key+".md"))
		if err != nil {
			fmt.Fprintf(exportStderr, "jirafs export issue: %v\n", err)
			return 1
		}
		fmt.Fprintf(exportStdout, "would %s %s (%d bytes)\n", writeVerb(dest), dest, len(output))
		dryRunDone(exportStdout, "jirafs export issue "+key)
		return 0
	}
	if err := os.MkdirAll(proj.LocalDirs[0], 0o755); err != nil {
		fmt.Fprintf(exportStderr, "jirafs export issue: create local directory %s: %v\n", proj.LocalDirs[0], err)
		return 1
	}
	dest, err := filepath.Abs(filepath.Join(proj.LocalDirs[0], key+".md"))
	if err != nil {
		fmt.Fprintf(exportStderr, "jirafs export issue: %v\n", err)
		return 1
	}
	if err := writeSyncedIssue(dest, ctx.MirrorDir, issue); err != nil {
		fmt.Fprintf(exportStderr, "jirafs export issue: write %s: %v\n", dest, err)
		return 1
	}
	fmt.Fprintf(exportStdout, "wrote %s → %s\n", key, dest)
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

// printExportHelp prints usage information for the export subcommand.
func printExportHelp() {
	fmt.Fprintf(exportStdout, "%s\n", color.BoldBlue(exportStdout, "Usage:"))
	fmt.Fprintf(exportStdout, "  jirafs %s <%s> [flags]\n", color.Blue(exportStdout, "export"), color.Yellow(exportStdout, "subcommand"))

	fmt.Fprintf(exportStdout, "  %s\n\n", color.Dim(exportStdout, "issue accepts a key (PROJ-123) or a Jira URL (<base_url>/browse/PROJ-123)"))

	fmt.Fprintf(exportStdout, "%s:\n", color.BoldGreen(exportStdout, "Subcommands"))
	fmt.Fprintf(exportStdout, "  %s    %s\n", color.Blue(exportStdout, "issue"), color.Dim(exportStdout, "export one issue through the real service path"))
	fmt.Fprintf(exportStdout, "  %s    %s\n\n", color.Blue(exportStdout, "help"), color.Dim(exportStdout, "show this help message"))

	fmt.Fprintf(exportStdout, "%s:\n", color.BoldGreen(exportStdout, "Flags"))
	fmt.Fprintf(exportStdout, "  %s KEY   %s\n", color.Yellow(exportStdout, "--project"), color.Dim(exportStdout, "project key or name to use"))
	fmt.Fprintf(exportStdout, "  %s DIR   %s\n", color.Yellow(exportStdout, "--cwd"), color.Dim(exportStdout, "working directory for project resolution"))
	fmt.Fprintf(exportStdout, "  %s     %s\n", color.Yellow(exportStdout, "--stdout"), color.Dim(exportStdout, "print the document to stdout instead of writing <local_dirs[0]>/<KEY>.md"))
	fmt.Fprintf(exportStdout, "  %s    %s\n\n", color.Yellow(exportStdout, "--dry-run"), color.Dim(exportStdout, "print the target path and whether it would be overwritten; write nothing"))

	fmt.Fprintf(exportStdout, "%s\n", color.Cyan(exportStdout, `Run "jirafs export <subcommand> --help" for more information about a subcommand.`))
}

// printExportIssueHelp prints usage information for `jirafs export issue`.
func printExportIssueHelp() {
	PrintUsage(exportStdout, Usage{
		Synopsis: []string{"export issue <KEY|URL> [flags]"},
		Description: []string{
			"Fetches one issue from Jira and writes it to <local_dirs[0]>/<KEY>.md.",
			"KEY is an issue key (PROJ-123); URL is <base_url>/browse/PROJ-123 or a URL with selectedIssue=PROJ-123.",
		},
		Sections: []UsageSection{{Title: "Flags", Rows: [][2]string{
			{"--project KEY", "project key or name to use"},
			{"--cwd DIR", "working directory for project resolution"},
			{"--stdout", "print the document to stdout and write no file"},
			{"--dry-run", "print the target path and whether it would be overwritten; write nothing"},
			{"--help, -h", "show this help message"},
		}}},
	})
}
