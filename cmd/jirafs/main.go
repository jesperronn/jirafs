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

// runUse handles the `jirafs use` command. It supports three modes:
//
//  1. `jirafs use` (no args) → show the current project.
//  2. `jirafs use --clear` → clear the remembered current project.
//  3. `jirafs use <project_key>` or `jirafs use --project <KEY>` → set it.
//
// It loads the user's settings, validates the project exists, updates the
// remembered current_project in state, writes it back, and prints a message.
// Returns an exit code (0 on success, 1 on error).
func runUse(args []string) int {
	fs := flag.NewFlagSet("use", flag.ExitOnError)
	project := fs.String("project", "", "project key to set as current")
	clear := fs.Bool("clear", false, "clear the remembered current project")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "jirafs use: invalid flags: %v\n", err)
		return 1
	}

	// --clear takes priority.
	if *clear {
		if *project != "" {
			fmt.Fprintf(os.Stderr, "jirafs use: --project and --clear are mutually exclusive\n")
			return 1
		}
		s, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "jirafs use: cannot load settings: %v\n", err)
			return 1
		}
		s.State.CurrentProject = ""
		if err := s.SaveState(); err != nil {
			fmt.Fprintf(os.Stderr, "jirafs use: cannot save state: %v\n", err)
			return 1
		}
		fmt.Println("jirafs: current project cleared")
		return 0
	}

	// Determine the project key from --project flag or positional args.
	var key string
	if *project != "" {
		key = *project
	} else if len(fs.Args()) == 1 {
		key = fs.Args()[0]
	} else if len(fs.Args()) > 1 {
		fmt.Fprintf(os.Stderr, "jirafs use: too many positional arguments\n")
		return 1
	}

	// No project given → show current state.
	if key == "" {
		s, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "jirafs use: cannot load settings: %v\n", err)
			return 1
		}
		if s.State.CurrentProject == "" {
			fmt.Println("jirafs: no current project set")
		} else {
			fmt.Printf("jirafs: current project is %q\n", s.State.CurrentProject)
		}
		return 0
	}

	s, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "jirafs use: cannot load settings: %v\n", err)
		return 1
	}

	if _, ok := s.Projects[key]; !ok {
		fmt.Fprintf(os.Stderr, "jirafs use: project %q not found in settings\n", key)
		return 1
	}

	s.State.CurrentProject = key
	if err := s.SaveState(); err != nil {
		fmt.Fprintf(os.Stderr, "jirafs use: cannot save state: %v\n", err)
		return 1
	}

	fmt.Printf("jirafs: current project set to %q\n", key)
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
