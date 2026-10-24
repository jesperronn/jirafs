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
	case "resolve":
		return cli.RunResolve(rest)
	case "status":
		return cli.RunStatus(rest)
	case "doctor":
		return cli.RunDoctor(rest)
	case "docs":
		return runDocs(rest, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "jirafs: unknown command: %q\nUse --help for usage.\n", args[0])
		return 2
	}
}

func printHelp(stderr io.Writer) {
	fmt.Fprintf(stderr, "%s\n", color.BoldBlue(stderr, "Usage:"))
	fmt.Fprintf(stderr, "  %s <command> [arguments]\n\n", color.Bold(stderr, "jirafs"))

	fmt.Fprintf(stderr, "%s:\n", color.BoldGreen(stderr, "Commands"))
	var commands [][2]string
	for _, c := range cli.TopLevelCommands() {
		commands = append(commands, [2]string{c.Name, c.Summary})
	}
	for _, cmd := range commands {
		fmt.Fprintf(stderr, "  %s  %s\n", color.Blue(stderr, cmd[0]), color.Dim(stderr, cmd[1]))
	}

	fmt.Fprintf(stderr, "\n%s\n", color.Cyan(stderr, `Run "jirafs <command> --help" for more information about a command.`))
}
