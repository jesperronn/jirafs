package cli

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/jirafs/jirafs/internal/config"
	"github.com/jirafs/jirafs/internal/context"
)

var (
	useStdout io.Writer = os.Stdout
	useStderr io.Writer = os.Stderr
)

// UseSnapshot represents the result of project selection for the
// `jirafs use` command. It is a lightweight, read-only view of the
// resolved project.
type UseSnapshot struct {
	// ProjectName is the config name of the resolved project.
	ProjectName string

	// ProjectKey is the Jira project key (e.g. "PROJ").
	ProjectKey string

	// MirrorDir is the expanded mirror directory path.
	MirrorDir string

	// Instance is the Jira instance name.
	Instance string

	// Resolved reports whether a project was successfully resolved.
	Resolved bool
}

// IsZero reports whether s is the zero value.
func (s UseSnapshot) IsZero() bool {
	return s.ProjectName == "" && s.ProjectKey == "" &&
		s.MirrorDir == "" && s.Instance == "" && !s.Resolved
}

// BuildUseSnapshot builds a project-selection snapshot for the given
// settings and working directory. It resolves the project using the same
// precedence as other commands: explicit flag, cwd mapping, remembered state.
func BuildUseSnapshot(settings *config.Settings, cwd string) UseSnapshot {
	snap := UseSnapshot{}

	if settings == nil {
		return snap
	}

	resolver := context.NewResolver(settings, "")
	ctx, err := resolver.Resolve(cwd)
	if err == nil {
		snap.Resolved = true
		snap.ProjectName = ctx.Name
		snap.ProjectKey = ctx.Key
		snap.MirrorDir = ctx.MirrorDir
		snap.Instance = ctx.Instance
	}

	return snap
}

// RunUse handles the `jirafs use` subcommand. It supports three modes:
//
//  1. `jirafs use` (no args) → show the current project via a snapshot.
//  2. `jirafs use --clear` → clear the remembered current project.
//  3. `jirafs use <project_key>` or `jirafs use --project <KEY>` → set it.
//
// It loads the user's settings, validates the project exists, updates the
// remembered current_project in state, writes it back, and prints a message.
// Returns an exit code (0 on success, 1 on error).
func RunUse(args []string) int {
	fs := flag.NewFlagSet("use", flag.ExitOnError)
	project := fs.String("project", "", "project key to set as current")
	clear := fs.Bool("clear", false, "clear the remembered current project")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(useStderr, "jirafs use: invalid flags: %v\n", err)
		return 1
	}

	// --clear takes priority.
	if *clear {
		if *project != "" {
			fmt.Fprintf(useStderr, "jirafs use: --project and --clear are mutually exclusive\n")
			return 1
		}
		s, err := config.Load()
		if err != nil {
			fmt.Fprintf(useStderr, "jirafs use: cannot load settings: %v\n", err)
			return 1
		}
		s.State.CurrentProject = ""
		if err := s.SaveState(); err != nil {
			fmt.Fprintf(useStderr, "jirafs use: cannot save state: %v\n", err)
			return 1
		}
		fmt.Fprintln(useStdout, "jirafs: current project cleared")
		return 0
	}

	// Determine the project key from --project flag or positional args.
	var key string
	if *project != "" {
		key = *project
	} else if len(fs.Args()) == 1 {
		key = fs.Args()[0]
	} else if len(fs.Args()) > 1 {
		fmt.Fprintf(useStderr, "jirafs use: too many positional arguments\n")
		return 1
	}

	// No project given → show current state via snapshot.
	if key == "" {
		s, err := config.Load()
		if err != nil {
			fmt.Fprintf(useStderr, "jirafs use: cannot load settings: %v\n", err)
			return 1
		}
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(useStderr, "jirafs use: cannot determine working directory: %v\n", err)
			return 1
		}
		snap := BuildUseSnapshot(s, cwd)
		if !snap.Resolved {
			fmt.Fprintln(useStdout, "jirafs: no current project set")
			return 0
		}
		fmt.Fprintf(useStdout, "jirafs: current project is %q (key: %s)\n", snap.ProjectName, snap.ProjectKey)
		return 0
	}

	s, err := config.Load()
	if err != nil {
		fmt.Fprintf(useStderr, "jirafs use: cannot load settings: %v\n", err)
		return 1
	}

	if _, ok := s.Projects[key]; !ok {
		fmt.Fprintf(useStderr, "jirafs use: project %q not found in settings\n", key)
		return 1
	}

	s.State.CurrentProject = key
	if err := s.SaveState(); err != nil {
		fmt.Fprintf(useStderr, "jirafs use: cannot save state: %v\n", err)
		return 1
	}

	fmt.Fprintf(useStdout, "jirafs: current project set to %q\n", key)
	return 0
}
