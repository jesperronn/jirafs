package main

import (
	"fmt"
	"io"
	"os"

	"github.com/jirafs/jirafs/internal/cli"
	"github.com/jirafs/jirafs/internal/color"
)

// version is set at build time via -ldflags "-X main.version=<value>".
var version = "dev"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run dispatches one jirafs invocation and returns its exit code. It never
// calls os.Exit. Output of cmd/jirafs's own commands goes to stdout/stderr;
// the internal/cli handlers still write to the process-wide os.Stdout and
// os.Stderr, which in-process tests redirect.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		printHelp(stdout)
		return 0
	}
	rest := args[1:]
	switch args[0] {
	case "help", "--help", "-h":
		printHelp(stdout)
		return 0
	case "--version", "-v":
		fmt.Fprintln(stdout, "jirafs", version)
		return 0
	case "board":
		return cli.RunBoard(rest)
	case "init":
		return cli.RunInit(rest)
	case "new":
		return cli.RunNew(rest)
	case "push":
		return cli.RunPush(rest)
	case "registry":
		return cli.RunRegistry(rest)
	case "archive":
		return cli.RunArchive(rest)
	case "use":
		return cli.RunUse(rest)
	case "pull":
		return cli.RunPull(rest)
	case "diff":
		return cli.RunDiff(rest)
	case "status":
		return cli.RunStatus(rest)
	case "doctor":
		return cli.RunDoctor(rest)
	default:
		fmt.Fprintf(stderr, "jirafs: unknown command: %q\nUse --help for usage.\n", args[0])
		return 2
	}
}

func printHelp(stderr io.Writer) {
	fmt.Fprintf(stderr, "%s\n", color.BoldBlue(stderr, "Usage:"))
	fmt.Fprintf(stderr, "  %s <command> [arguments]\n\n", color.Bold(stderr, "jirafs"))

	fmt.Fprintf(stderr, "%s:\n", color.BoldGreen(stderr, "Commands"))
	commands := [][2]string{
		{"init", "configure a Jira instance and project, and create its mirror directory"},
		{"pull", "fetch Jira issues into local Markdown files (one issue or a scope)"},
		{"diff", "show what push would change in Jira (read-only)"},
		{"status", "show workspace status and next-step hints"},
		{"doctor", "run health checks on config, credentials, and live connectivity"},
		{"new", "create a new issue from a template"},
		{"push", "create drafts and update modified issues in Jira (dry run unless --apply)"},
		{"registry", "manage local registry files for typed references"},
		{"board", "show a local kanban-style board view"},
		{"archive", "archive resolved issues and list archived files"},
		{"use", "update remembered project context"},
		{"help", "show this help message"},
	}
	for _, cmd := range commands {
		fmt.Fprintf(stderr, "  %s  %s\n", color.Blue(stderr, cmd[0]), color.Dim(stderr, cmd[1]))
	}

	fmt.Fprintf(stderr, "\n%s\n", color.Cyan(stderr, `Run "jirafs <command> --help" for more information about a command.`))
}
