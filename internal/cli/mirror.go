// Package cli provides command implementations for the jirafs CLI.
package cli

import (
	stdcontext "context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jirafs/jirafs/internal/archive"
	"github.com/jirafs/jirafs/internal/config"
	"github.com/jirafs/jirafs/internal/context"
	"github.com/jirafs/jirafs/internal/export"
	"github.com/jirafs/jirafs/internal/jira"
	"github.com/jirafs/jirafs/internal/mirror"
	"github.com/jirafs/jirafs/internal/schema"
	"gopkg.in/yaml.v3"
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

var (
	mirrorStdout          io.Writer = os.Stdout
	mirrorStderr          io.Writer = os.Stderr
	mirrorClientFactory             = buildMirrorClient
	archiveServiceFactory           = buildArchiveService
)

// RunMirror dispatches the `jirafs mirror` subcommand to the appropriate
// sub-subcommand. It returns an exit code (0 on success, 1 on error).
func RunMirror(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(mirrorStderr, "jirafs mirror: missing subcommand. Use --help for usage.")
		return 1
	}

	// Check for help before loading settings.
	if args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		printMirrorHelp()
		return 0
	}

	// Load settings and create resolver.
	settings, err := config.Load()
	if err != nil {
		fmt.Fprintf(mirrorStderr, "jirafs mirror: cannot load settings: %v\n", err)
		return 1
	}
	resolver := context.NewResolver(settings, "")

	switch args[0] {
	case "refresh":
		return runMirrorRefresh(args[1:], settings, resolver)
	case "archive-sweep":
		return runMirrorArchiveSweep(args[1:], settings, resolver)
	case "current-sprint", "my-issues", "backlog", "next-sprint":
		return runMirrorRefresh(append(args[1:], args[0]), settings, resolver)
	default:
		fmt.Fprintf(mirrorStderr, "jirafs mirror: unknown subcommand %q. Use --help for usage.\n", args[0])
		return 1
	}
}

func runMirrorRefresh(args []string, settings *config.Settings, resolver *context.Resolver) int {
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			printMirrorRefreshHelp()
			return 0
		}
	}

	fs := flag.NewFlagSet("mirror refresh", flag.ContinueOnError)
	fs.SetOutput(mirrorStderr)
	projectFlag := fs.String("project", "", "project key or name to refresh")
	cwdFlag := fs.String("cwd", "", "working directory to use for project resolution")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(mirrorStderr, "jirafs mirror refresh: invalid flags: %v\n", err)
		return 1
	}

	if len(fs.Args()) == 0 {
		fmt.Fprintln(mirrorStderr, "jirafs mirror refresh: missing scope name")
		return 1
	}
	if len(fs.Args()) > 1 {
		fmt.Fprintln(mirrorStderr, "jirafs mirror refresh: too many positional arguments")
		return 1
	}
	scopeName := fs.Args()[0]

	cwd := "."
	if *cwdFlag != "" {
		cwd = *cwdFlag
	}

	ctx, ok := resolveMirrorContext(settings, *projectFlag, cwd, "refresh")
	if !ok {
		return 1
	}

	m, mirrorPath, err := loadMirrorYAML(ctx.MirrorDir)
	if err != nil {
		fmt.Fprintf(mirrorStderr, "jirafs mirror refresh: cannot load mirror: %v\n", err)
		return 1
	}

	scope := m.ScopeFor(scopeName)
	if scope.IsZero() {
		fmt.Fprintf(mirrorStderr, "jirafs mirror refresh: scope %q not found in mirror\n", scopeName)
		return 1
	}

	client, err := mirrorClientFactory(settings, ctx, cwd)
	if err != nil {
		fmt.Fprintf(mirrorStderr, "jirafs mirror refresh: cannot create Jira client: %v\n", err)
		return 1
	}

	added, err := mirror.RefreshScope(stdcontext.Background(), client, scope, m)
	if err != nil {
		fmt.Fprintf(mirrorStderr, "jirafs mirror refresh: %v\n", err)
		return 1
	}

	if err := materializeScopeIssues(stdcontext.Background(), client, settings, ctx, scopeName); err != nil {
		fmt.Fprintf(mirrorStderr, "jirafs mirror refresh: cannot write issue snapshots: %v\n", err)
		return 1
	}

	if err := saveMirrorYAML(mirrorPath, m); err != nil {
		fmt.Fprintf(mirrorStderr, "jirafs mirror refresh: cannot save mirror: %v\n", err)
		return 1
	}

	sort.Slice(added, func(i, j int) bool {
		return added[i] < added[j]
	})
	if len(added) == 0 {
		fmt.Fprintf(mirrorStdout, "jirafs mirror refresh: no changed issue keys for scope %q in %q\n", scopeName, ctx.Name)
		return 0
	}

	fmt.Fprintf(mirrorStdout, "jirafs mirror refresh: %d changed issue key(s) for scope %q in %q:\n", len(added), scopeName, ctx.Name)
	for _, key := range added {
		fmt.Fprintf(mirrorStdout, "  %s\n", key)
	}
	return 0
}

func materializeScopeIssues(ctx stdcontext.Context, client jira.Client, settings *config.Settings, projectCtx *context.Context, scopeName string) error {
	proj, ok := settings.Projects[projectCtx.Name]
	if !ok {
		return fmt.Errorf("project %q not found in settings", projectCtx.Name)
	}
	if len(proj.LocalDirs) == 0 {
		return fmt.Errorf("project %q has no local_dirs configured", projectCtx.Name)
	}
	if err := os.MkdirAll(proj.LocalDirs[0], 0o755); err != nil {
		return fmt.Errorf("create local directory %s: %w", proj.LocalDirs[0], err)
	}

	issues, total, err := client.SearchIssues(ctx, scopeName)
	if err != nil {
		return err
	}
	if total < len(issues) {
		total = len(issues)
	}
	writeMirrorProgress(mirrorStdout, "fetching", 0, total)
	if len(issues) == 0 {
		writeMirrorProgress(mirrorStdout, "fetched", 0, total)
		fmt.Fprintln(mirrorStdout)
		return nil
	}
	completed := 0
	for _, issue := range issues {
		if issue == nil || issue.Identity.Key == "" {
			continue
		}
		renderIssue := issue
		fullIssue, err := client.FetchIssue(ctx, string(issue.Identity.Key))
		if err == nil && fullIssue != nil && fullIssue.Identity.Key != "" {
			renderIssue = fullIssue
		}
		output := export.ExportIssue(renderIssue)
		dest := filepath.Join(proj.LocalDirs[0], string(renderIssue.Identity.Key)+".md")
		if err := os.WriteFile(dest, []byte(output), 0o644); err != nil {
			return fmt.Errorf("write issue file %s: %w", dest, err)
		}
		completed++
		writeMirrorProgress(mirrorStdout, "fetching", completed, total)
	}
	writeMirrorProgress(mirrorStdout, "fetched", total, total)
	fmt.Fprintln(mirrorStdout)
	return nil
}

func writeMirrorProgress(w io.Writer, label string, current, total int) {
	fmt.Fprintf(w, "\r%s %d/%d", label, current, total)
}

// runMirrorArchiveSweep handles `jirafs mirror archive-sweep`.
// It resolves the project context, loads the mirror, scans all issue files
// in the project's local directories, and reports eligible issues without
// mutation. With --apply, it also calls the archive service interface
// to move eligible issues.
func runMirrorArchiveSweep(args []string, settings *config.Settings, resolver *context.Resolver) int {
	fs := flag.NewFlagSet("mirror archive-sweep", flag.ExitOnError)
	// --project overrides the auto-detected project.
	projectFlag := fs.String("project", "", "project key or name to scan")
	// --cwd overrides the working directory for project resolution.
	cwdFlag := fs.String("cwd", "", "working directory to use for project resolution")
	// --apply actually moves eligible issues through the archive service.
	applyFlag := fs.Bool("apply", false, "actually archive eligible issues")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "jirafs mirror archive-sweep: invalid flags: %v\n", err)
		return 1
	}

	// Resolve the current working directory.
	cwd := "."
	if *cwdFlag != "" {
		cwd = *cwdFlag
	}

	ctx, ok := resolveMirrorContext(settings, *projectFlag, cwd, "archive-sweep")
	if !ok {
		return 1
	}

	// Load the mirror from the mirror directory.
	m, _, err := loadMirrorYAML(ctx.MirrorDir)
	if err != nil {
		fmt.Fprintf(mirrorStderr, "jirafs mirror archive-sweep: cannot load mirror: %v\n", err)
		return 1
	}

	// Scan all issue files in the project's local directories.
	eligible, err := scanEligibleIssues(ctx, m, settings)
	if err != nil {
		fmt.Fprintf(mirrorStderr, "jirafs mirror archive-sweep: %v\n", err)
		return 1
	}

	// Report results.
	if len(eligible) == 0 {
		fmt.Fprintln(mirrorStdout, "jirajs mirror archive-sweep: no eligible issues found")
		return 0
	}

	fmt.Fprintf(mirrorStdout, "jirafs mirror archive-sweep: %d eligible issue(s) for %q:\n", len(eligible), ctx.Name)
	for _, e := range eligible {
		fmt.Fprintf(mirrorStdout, "  %s (resolved: %s)\n", e.Key, e.ResolvedStatus)
	}

	// With --apply, call the archive service for each eligible issue.
	if !*applyFlag {
		return 0
	}

	// Build the archive service.
	archiveSvc, err := archiveServiceFactory(settings, ctx, cwd)
	if err != nil {
		fmt.Fprintf(mirrorStderr, "jirafs mirror archive-sweep: cannot create archive service: %v\n", err)
		return 1
	}

	// Archive each eligible issue.
	proj, ok := settings.Projects[ctx.Name]
	if !ok {
		fmt.Fprintf(mirrorStderr, "jirafs mirror archive-sweep: project %q not found in settings\n", ctx.Name)
		return 1
	}

	var successCount int
	var failCount int
	var errors []string

	for _, e := range eligible {
		// Find the issue file path.
		issuePath, found := findIssuePath(proj.LocalDirs, e.Key)
		if !found {
			errMsg := fmt.Sprintf("%s: file not found in local directories", e.Key)
			errors = append(errors, errMsg)
			failCount++
			continue
		}

		err := archiveSvc.Archive(string(e.Key), ctx.MirrorDir, proj.LocalDirs[0], issuePath)
		if err != nil {
			errMsg := fmt.Sprintf("%s: %v", e.Key, err)
			errors = append(errors, errMsg)
			failCount++
		} else {
			fmt.Fprintf(mirrorStdout, "  archived: %s\n", e.Key)
			successCount++
		}
	}

	if failCount > 0 {
		fmt.Fprintf(mirrorStderr, "jirafs mirror archive-sweep: %d/%d issues archived successfully\n", successCount, len(eligible))
		for _, e := range errors {
			fmt.Fprintf(mirrorStderr, "  error: %s\n", e)
		}
		return 1
	}

	fmt.Fprintf(mirrorStdout, "jirafs mirror archive-sweep: archived %d issue(s)\n", successCount)
	return 0
}

// findIssuePath searches local directories for a file matching the given issue key.
func findIssuePath(localDirs []string, key schema.IssueKey) (string, bool) {
	for _, dir := range localDirs {
		path := filepath.Join(dir, string(key)+".md")
		if _, err := os.Stat(path); err == nil {
			return path, true
		}
	}
	return "", false
}

// loadMirrorYAML loads the mirror YAML file from the mirror directory.
// It looks for mirror.yml or mirror.yaml in the mirror directory.
func loadMirrorYAML(mirrorDir string) (*mirror.Mirror, string, error) {
	// Try mirror.yml first, then mirror.yaml.
	for _, name := range []string{"mirror.yml", "mirror.yaml"} {
		path := filepath.Join(mirrorDir, name)
		if _, err := os.Stat(path); err == nil {
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, "", fmt.Errorf("cannot read mirror file %s: %w", path, err)
			}
			m, err := mirror.UnmarshalMirror(data)
			if err != nil {
				return nil, "", fmt.Errorf("cannot parse mirror file %s: %w", path, err)
			}
			return m, path, nil
		}
	}
	// No mirror file found: return an empty mirror (all issues are eligible).
	return &mirror.Mirror{}, filepath.Join(mirrorDir, "mirror.yml"), nil
}

func saveMirrorYAML(path string, m *mirror.Mirror) error {
	data, err := yaml.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshal mirror: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write mirror file %s: %w", path, err)
	}
	return nil
}

func buildMirrorClient(settings *config.Settings, ctx *context.Context, cwd string) (jira.Client, error) {
	creds, err := settings.ResolveInstanceCredentials(ctx.Instance)
	if err != nil {
		return nil, err
	}
	client := jira.NewJiraClient(creds.BaseURL)
	client.SetCredentials(creds)
	return client, nil
}

func buildArchiveService(settings *config.Settings, ctx *context.Context, cwd string) (archive.Service, error) {
	// Find the archive directory.
	proj, ok := settings.Projects[ctx.Name]
	if !ok {
		return nil, fmt.Errorf("project %q not found in settings", ctx.Name)
	}

	var archiveDir string
	for _, dir := range proj.LocalDirs {
		candidate := filepath.Join(dir, "_archive")
		if _, err := os.Stat(candidate); err == nil {
			archiveDir = candidate
			break
		}
	}

	if archiveDir == "" {
		// Create the archive directory.
		for _, dir := range proj.LocalDirs {
			candidate := filepath.Join(dir, "_archive")
			if err := os.MkdirAll(candidate, 0o755); err == nil {
				archiveDir = candidate
				break
			}
		}
		if archiveDir == "" {
			return nil, fmt.Errorf("cannot create archive directory")
		}
	}
	return archive.FileService{ArchiveDir: archiveDir}, nil
}

func resolveMirrorContext(settings *config.Settings, project, cwd, subcommand string) (*context.Context, bool) {
	res := context.NewResolver(settings, project)
	ctx, err := res.Resolve(cwd)
	if err != nil {
		var ce *context.Error
		if context.IsContextError(err, &ce) {
			if ce.Code == config.ErrNoProjectResolved {
				fmt.Fprintf(mirrorStderr, "jirafs mirror %s: no project resolved for cwd %q\n", subcommand, cwd)
				if len(ce.Candidates) > 0 {
					fmt.Fprintln(mirrorStderr, "Available projects:")
					for _, name := range ce.Candidates {
						fmt.Fprintf(mirrorStderr, "  - %s\n", name)
					}
				}
				return nil, false
			}
			fmt.Fprintf(mirrorStderr, "jirafs mirror %s: %v\n", subcommand, err)
			return nil, false
		}
		fmt.Fprintf(mirrorStderr, "jirafs mirror %s: %v\n", subcommand, err)
		return nil, false
	}
	return ctx, true
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
	fmt.Fprintln(mirrorStderr, `Usage:
  jirafs mirror <subcommand> [flags]

Subcommands:
  current-sprint  shorthand for "refresh current-sprint"
  my-issues       shorthand for "refresh my-issues"
  backlog         shorthand for "refresh backlog"
  next-sprint     shorthand for "refresh next-sprint"
  refresh         refresh one named live mirror scope
  archive-sweep   report archive-eligible issues without mutation
  help            show this help message

Live mirror scopes:
  current-sprint  active sprint issues
  my-issues       issues assigned to the current user
  backlog         bounded backlog window
  next-sprint     next planned sprint when configured

Flags:
  --project KEY   project key or name to use
  --cwd DIR       working directory for project resolution
  --apply         actually archive eligible issues

Run "jirafs mirror <subcommand> --help" for more information about a subcommand.`)
}

func printMirrorRefreshHelp() {
	fmt.Fprintln(mirrorStderr, `Usage:
  jirafs mirror refresh [flags] <scope>
  jirafs mirror <scope> [flags]

Refreshes one named live mirror scope.

Live mirror scopes:
  current-sprint
  my-issues
  backlog
  next-sprint

Examples:
  jirafs mirror my-issues
  jirafs mirror refresh my-issues
  jirafs mirror next-sprint
  jirafs mirror refresh current-sprint

Flags:
  --project KEY   project key or name to use
  --cwd DIR       working directory for project resolution
  --help          show this help message`)
}
