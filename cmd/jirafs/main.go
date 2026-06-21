package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/jirafs/jirafs/internal/config"
)

func main() {
	if len(os.Args) < 2 {
		printHelp()
		os.Exit(0)
	}

	switch os.Args[1] {
	case "help":
		printHelp()
	case "init", "export", "plan", "sync", "new", "registry", "board", "archive":
		fmt.Fprintf(os.Stderr, "jirafs %s: not yet implemented\n", os.Args[1])
		os.Exit(1)
	case "use":
		os.Exit(runUse(os.Args[2:]))
	case "--help", "-h":
		printHelp()
	default:
		fmt.Fprintf(os.Stderr, "jirafs: unknown command: %q\n\n", os.Args[1])
		printHelp()
		os.Exit(1)
	}
}

// runUse handles the `jirafs use --project <KEY>` command.
// It loads the user's settings, validates the project exists, updates the
// remembered current_project in state, writes it back, and prints a confirmation.
// Returns an exit code (0 on success, 1 on error).
func runUse(args []string) int {
	fs := flag.NewFlagSet("use", flag.ExitOnError)
	project := fs.String("project", "", "project key to set as current")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "jirafs use: invalid flags: %v\n", err)
		return 1
	}

	if *project == "" {
		fmt.Fprintf(os.Stderr, "jirafs use: --project is required\n")
		return 1
	}

	s, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "jirafs use: cannot load settings: %v\n", err)
		return 1
	}

	if _, ok := s.Projects[*project]; !ok {
		fmt.Fprintf(os.Stderr, "jirafs use: project %q not found in settings\n", *project)
		return 1
	}

	s.State.CurrentProject = *project
	if err := s.SaveState(); err != nil {
		fmt.Fprintf(os.Stderr, "jirafs use: cannot save state: %v\n", err)
		return 1
	}

	fmt.Printf("jirafs: current project set to %q\n", *project)
	return 0
}

func printHelp() {
	fmt.Fprintln(os.Stderr, `Usage:
  jirafs <command> [arguments]

Commands:
  init       initialize a new jirafs project in the current directory
  export     export Jira issues into local Markdown files
  plan       show a sync plan without applying changes
  sync       apply a sync plan and push changes to Jira
  new        create a new issue from a template
  registry   manage local registry files for typed references
  board      show a local kanban-style board view
  archive    manage archived issue files
  use        update remembered project context
  help       show this help message

Run "jirafs <command> --help" for more information about a command.`)
}
