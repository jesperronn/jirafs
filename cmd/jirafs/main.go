package main

import (
	"fmt"
	"os"
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
	case "--help", "-h":
		printHelp()
	default:
		fmt.Fprintf(os.Stderr, "jirafs: unknown command: %q\n\n", os.Args[1])
		printHelp()
		os.Exit(1)
	}
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
  help       show this help message

Run "jirafs <command> --help" for more information about a command.`)
}
