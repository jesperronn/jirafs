// Package cli provides command implementations for the jirafs CLI.
package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jirafs/jirafs/internal/config"
	"github.com/jirafs/jirafs/internal/context"
	"github.com/jirafs/jirafs/internal/mirror"
	"github.com/jirafs/jirafs/internal/schema"
)

// MirrorHandler handles the `jirafs mirror` subcommand and its sub-commands.
type MirrorHandler struct {
	// Settings is the loaded user settings.
	Settings *config.Settings
	// Resolver resolves the active project context.
	Resolver *context.Resolver
	// Reader is the stdin reader for interactive prompts (optional).
	Reader context.PromptReader
}

// RunMirror dispatches the `jirafs mirror` subcommand to the appropriate
// sub-subcommand. It returns an exit code (0 on success, 1 on error).
func RunMirror(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "jirafs mirror: missing subcommand. Use --help for usage.")
		return 1
	}

	// Check for help before loading settings.
	if args[0] == "help" {
		printMirrorHelp()
		return 0
	}

	// Load settings and create resolver.
	settings, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "jirafs mirror: cannot load settings: %v\n", err)
		return 1
	}
	resolver := context.NewResolver(settings, "")

	switch args[0] {
	case "archive-sweep":
		return runMirrorArchiveSweep(args[1:], settings, resolver)
	default:
		fmt.Fprintf(os.Stderr, "jirafs mirror: unknown subcommand %q. Use --help for usage.\n", args[0])
		return 1
	}
}

// runMirrorArchiveSweep handles `jirafs mirror archive-sweep`.
// It resolves the project context, loads the mirror, scans all issue files
// in the project's local directories, and reports eligible issues without
// mutation.
func runMirrorArchiveSweep(args []string, settings *config.Settings, resolver *context.Resolver) int {
	fs := flag.NewFlagSet("mirror archive-sweep", flag.ExitOnError)
	// --project overrides the auto-detected project.
	projectFlag := fs.String("project", "", "project key or name to scan")
	// --cwd overrides the working directory for project resolution.
	cwdFlag := fs.String("cwd", "", "working directory to use for project resolution")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "jirafs mirror archive-sweep: invalid flags: %v\n", err)
		return 1
	}

	// Resolve the current working directory.
	cwd := "."
	if *cwdFlag != "" {
		cwd = *cwdFlag
	}

	// Resolve the project context.
	res := context.NewResolver(settings, *projectFlag)
	ctx, err := res.Resolve(cwd)
	if err != nil {
		var ce *context.Error
		if context.IsContextError(err, &ce) {
			if ce.Code == config.ErrNoProjectResolved {
				fmt.Fprintf(os.Stderr, "jirafs mirror archive-sweep: no project resolved for cwd %q\n", cwd)
				if len(ce.Candidates) > 0 {
					fmt.Fprintln(os.Stderr, "Available projects:")
					for _, name := range ce.Candidates {
						fmt.Fprintf(os.Stderr, "  - %s\n", name)
					}
				}
				return 1
			}
			fmt.Fprintf(os.Stderr, "jirafs mirror archive-sweep: %v\n", err)
			return 1
		}
		fmt.Fprintf(os.Stderr, "jirafs mirror archive-sweep: %v\n", err)
		return 1
	}

	// Load the mirror from the mirror directory.
	m, err := loadMirrorYAML(ctx.MirrorDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "jirafs mirror archive-sweep: cannot load mirror: %v\n", err)
		return 1
	}

	// Scan all issue files in the project's local directories.
	eligible, err := scanEligibleIssues(ctx, m, settings)
	if err != nil {
		fmt.Fprintf(os.Stderr, "jirafs mirror archive-sweep: %v\n", err)
		return 1
	}

	// Report results.
	if len(eligible) == 0 {
		fmt.Println("jirafs mirror archive-sweep: no eligible issues found")
		return 0
	}

	fmt.Printf("jirafs mirror archive-sweep: %d eligible issue(s) for %q:\n", len(eligible), ctx.Name)
	for _, e := range eligible {
		fmt.Printf("  %s (resolved: %s)\n", e.Key, e.ResolvedStatus)
	}

	return 0
}

// loadMirrorYAML loads the mirror YAML file from the mirror directory.
// It looks for mirror.yml or mirror.yaml in the mirror directory.
func loadMirrorYAML(mirrorDir string) (*mirror.Mirror, error) {
	// Try mirror.yml first, then mirror.yaml.
	for _, name := range []string{"mirror.yml", "mirror.yaml"} {
		path := filepath.Join(mirrorDir, name)
		if _, err := os.Stat(path); err == nil {
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("cannot read mirror file %s: %w", path, err)
			}
			m, err := mirror.UnmarshalMirror(data)
			if err != nil {
				return nil, fmt.Errorf("cannot parse mirror file %s: %w", path, err)
			}
			return m, nil
		}
	}
	// No mirror file found: return an empty mirror (all issues are eligible).
	return &mirror.Mirror{}, nil
}

// scanEligibleIssues walks all local directories of the project and checks
// each issue file for archive eligibility.
func scanEligibleIssues(ctx *context.Context, m *mirror.Mirror, settings *config.Settings) ([]mirror.ArchiveEligible, error) {
	proj, ok := settings.Projects[ctx.Name]
	if !ok {
		return nil, fmt.Errorf("project %q not found in settings", ctx.Name)
	}

	var eligible []mirror.ArchiveEligible

	for _, localDir := range proj.LocalDirs {
		issues, err := scanLocalDir(localDir, m)
		if err != nil {
			return nil, fmt.Errorf("scanning %s: %w", localDir, err)
		}
		eligible = append(eligible, issues...)
	}

	return eligible, nil
}

// scanLocalDir scans all .md files in a local directory for archive eligibility.
func scanLocalDir(dir string, m *mirror.Mirror) ([]mirror.ArchiveEligible, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("cannot read directory %s: %w", dir, err)
	}

	var eligible []mirror.ArchiveEligible

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		issue, pe := schema.ParseIssue(string(data))
		if pe != nil {
			// Skip files that cannot be parsed (not an issue file).
			continue
		}

		// Check archive eligibility.
		resolved := mirror.ResolvedStatus(issue.RemoteMetadata.ResolvedStatus)
		if m.IsArchiveEligible(issue.Identity.Key, resolved, issue.RemoteMetadata) {
			eligible = append(eligible, mirror.ArchiveEligible{
				Key:            issue.Identity.Key,
				ResolvedStatus: resolved,
			})
		}
	}

	return eligible, nil
}

// printMirrorHelp prints usage information for the mirror subcommand.
func printMirrorHelp() {
	fmt.Fprintln(os.Stderr, `Usage:
  jirafs mirror <subcommand> [flags]

Subcommands:
  archive-sweep   report archive-eligible issues without mutation
  help            show this help message

Flags:
  --project KEY   project key or name to use
  --cwd DIR       working directory for project resolution

Run "jirafs mirror <subcommand> --help" for more information about a subcommand.`)
}
