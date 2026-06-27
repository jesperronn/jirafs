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

	"github.com/jirafs/jirafs/internal/board"
	"github.com/jirafs/jirafs/internal/color"
	"github.com/jirafs/jirafs/internal/config"
	"github.com/jirafs/jirafs/internal/registry"
	"github.com/jirafs/jirafs/internal/schema"
)

var (
	boardStdout io.Writer = os.Stdout
	boardStderr io.Writer = os.Stderr
)

// RunBoard dispatches the `jirafs board` subcommand. It resolves the project
// context, reads all issues from the mirror directory, loads the registry
// (statuses, users), groups issues according to the board model, and
// renders a kanban-style board view to stdout.
func RunBoard(args []string) int {
	// Check for help before loading settings.
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			printBoardHelp()
			return 0
		}
	}

	// Load settings and create resolver.
	settings, err := config.Load()
	if err != nil {
		fmt.Fprintf(boardStderr, "jirafs board: cannot load settings: %v\n", err)
		return 1
	}

	// Parse flags first to extract --project, --group-by, and --cwd.
	fs := flag.NewFlagSet("board", flag.ContinueOnError)
	projectFlag := fs.String("project", "", "project key or name to use")
	groupByFlag := fs.String("group-by", "status", "grouping mode: status, assignee, or epic")
	cwdFlag := fs.String("cwd", "", "working directory to use for project resolution")
	_ = fs.Parse(args)

	// Validate --group-by.
	if *groupByFlag != "status" && *groupByFlag != "assignee" && *groupByFlag != "epic" {
		fmt.Fprintf(boardStderr, "jirafs board: invalid --group-by %q: must be status, assignee, or epic\n", *groupByFlag)
		return 1
	}

	ctx, ok := resolvePlanContext(settings, *projectFlag, *cwdFlag)
	if !ok {
		return 1
	}

	proj, ok := settings.Projects[ctx.Name]
	if !ok {
		fmt.Fprintf(boardStderr, "jirafs board: project %q not found in settings\n", ctx.Name)
		return 1
	}

	if len(proj.LocalDirs) == 0 {
		fmt.Fprintln(boardStdout, "jirafs board: no local directories configured for project "+ctx.Name)
		return 0
	}

	// Read all issue files from local directories.
	issues, err := readAllIssues(proj.LocalDirs)
	if err != nil {
		fmt.Fprintf(boardStderr, "jirafs board: %v\n", err)
		return 1
	}

	groupMode := board.GroupMode(*groupByFlag)
	if len(issues) == 0 {
		fmt.Fprintf(boardStdout, "jirafs board: no issues found in %s (group=%s)\n", strings.Join(proj.LocalDirs, ", "), groupMode)
		return 0
	}

	// Load registry data from mirror directory.
	var statuses map[string]registry.Status
	var users map[string]registry.User

	mirrorDir := ctx.MirrorDir
	statuses, _ = registry.LoadStatuses(mirrorDir)
	users, _ = registry.LoadUsers(mirrorDir)

	// Build the board.
	b := board.NewBoard()

	switch groupMode {
	case board.GroupModeStatus:
		b.GroupByStatus(issues, statuses)
	case board.GroupModeAssignee:
		b.GroupByAssignee(issues, users)
	case board.GroupModeEpic:
		b.GroupByEpic(issues)
	}

	// Render the board.
	renderBoard(boardStdout, b)

	return 0
}

// readAllIssues reads all .md files from the given local directories and
// parses them into schema.Issue slices. Returns all parseable issues.
func readAllIssues(localDirs []string) ([]*schema.Issue, error) {
	var allIssues []*schema.Issue
	seen := make(map[schema.IssueKey]bool)

	for _, localDir := range localDirs {
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
			path := filepath.Join(localDir, name)
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			issue, pe := schema.ParseIssue(string(data))
			if pe != nil {
				continue
			}
			key := issue.Identity.Key
			if seen[key] {
				continue
			}
			seen[key] = true
			allIssues = append(allIssues, &issue)
		}
	}

	return allIssues, nil
}

// renderBoard renders a board model to stdout as a kanban-style view with
// stable column ordering. Each column header is followed by the issues
// in that column, grouped by status.
func renderBoard(w io.Writer, b *board.Board) {
	fmt.Fprintln(w, color.Cyan(w, "jirafs board: group="+string(b.GroupMode)))
	fmt.Fprintln(w)

	for _, colName := range b.ColumnOrder {
		var issues []*schema.Issue
		switch b.GroupMode {
		case board.GroupModeStatus:
			issues = b.StatusColumns[colName]
		case board.GroupModeAssignee:
			issues = b.AssigneeColumns[colName]
		case board.GroupModeEpic:
			issues = b.EpicColumns[colName]
		}

		fmt.Fprintf(w, color.BoldGreen(w, "  %s (%d):\n"), colName, len(issues))

		// Sort issues by key within each column for deterministic output.
		sort.Slice(issues, func(i, j int) bool {
			return issues[i].Identity.Key < issues[j].Identity.Key
		})

		for _, issue := range issues {
			label := color.Yellow(w, string(issue.Identity.Key))
			summary := issue.Summary
			if summary == "" {
				summary = "(no summary)"
			}
			fmt.Fprintf(w, "    %s  %s\n", label, color.Dim(w, summary))
		}
		fmt.Fprintln(w)
	}
}

// printBoardHelp prints usage information for the board subcommand.
func printBoardHelp() {
	fmt.Fprintf(boardStderr, "%s\n", color.BoldBlue(boardStderr, "Usage:"))
	fmt.Fprintf(boardStderr, "  jirafs %s [flags]\n\n", color.Blue(boardStderr, "board"))

	fmt.Fprintf(boardStderr, "%s\n", color.Dim(boardStderr, "Displays a local kanban-style board view of issues grouped by"))
	fmt.Fprintf(boardStderr, "%s\n\n", color.Dim(boardStderr, "status, assignee, or epic."))

	fmt.Fprintf(boardStderr, "%s:\n", color.BoldGreen(boardStderr, "Flags"))
	fmt.Fprintf(boardStderr, "  %s MODE  %s\n", color.Yellow(boardStderr, "--group-by"), color.Dim(boardStderr, "status, assignee, or epic (default: status)"))
	fmt.Fprintf(boardStderr, "  %s KEY   %s\n", color.Yellow(boardStderr, "--project"), color.Dim(boardStderr, "project key or name to use"))
	fmt.Fprintf(boardStderr, "  %s DIR   %s\n\n", color.Yellow(boardStderr, "--cwd"), color.Dim(boardStderr, "working directory for project resolution"))

	fmt.Fprintf(boardStderr, "%s\n", color.Cyan(boardStderr, `Run "jirafs board --help" for more information about flags.`))
}
