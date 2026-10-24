// Package cli provides command implementations for the jirafs CLI.
package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jirafs/jirafs/internal/archive"
	"github.com/jirafs/jirafs/internal/color"
	"github.com/jirafs/jirafs/internal/config"
	"github.com/jirafs/jirafs/internal/context"
	"github.com/jirafs/jirafs/internal/mirror"
	"github.com/jirafs/jirafs/internal/schema"
)

var (
	archiveStdout io.Writer = os.Stdout
	archiveStderr io.Writer = os.Stderr

	archiveServiceFactory = buildArchiveService
)

// runArchiveIssue handles `jirafs archive issue <KEY>`. It resolves the
// project context, locates the live issue file, and moves it into the
// project's _archive directory via the archive service.
func runArchiveIssue(args []string, settings *config.Settings) int {
	// Go's flag package stops at the first non-flag argument, so pull a
	// leading positional key off before parsing flags and fall back to any
	// trailing positional after parsing. This lets callers write either
	// "issue ABC-1 --project test" or "issue --project test ABC-1".
	var keyPos string
	var flagArgs []string
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		keyPos = args[0]
		flagArgs = args[1:]
	} else {
		flagArgs = args
	}

	fs := quietFlagSet("archive issue")
	projectFlag := fs.String("project", "", "name of a configured project")
	cwdFlag := fs.String("cwd", "", "working directory to resolve the project from")
	dryRunFlag := fs.Bool("dry-run", false, "print what would be archived; move nothing")
	if err := parseFlags(fs, flagArgs); err != nil {
		return flagError(archiveStderr, "jirafs archive issue", err)
	}
	if keyPos == "" && fs.NArg() > 0 {
		keyPos = fs.Arg(0)
	}
	if keyPos == "" {
		return usageError(archiveStderr, "jirafs archive issue", "missing issue key")
	}
	key := keyPos

	cwd := *cwdFlag
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			fmt.Fprintf(archiveStderr, "jirafs archive issue: cannot determine working directory: %v\n", err)
			return 1
		}
	}

	ctx, ok := resolveArchiveContext(settings, *projectFlag, cwd)
	if !ok {
		return 1
	}

	proj, ok := settings.Projects[ctx.Name]
	if !ok || len(proj.LocalDirs) == 0 {
		fmt.Fprintln(archiveStderr, "jirafs archive issue: no local directories configured for this project")
		fmt.Fprintln(archiveStderr, `run "jirafs pull my-issues" to create local dirs`)
		return 1
	}

	issuePath, found := findIssuePath(proj.LocalDirs, schema.IssueKey(key))
	if !found {
		fmt.Fprintf(archiveStderr, "jirafs archive issue: issue %s not found in local directories\n", key)
		return 1
	}

	archiveDir := filepath.Join(proj.LocalDirs[0], "_archive")
	if *dryRunFlag {
		if err := (archive.FileService{ArchiveDir: archiveDir}).CheckEligible(key, ctx.MirrorDir, issuePath); err != nil {
			fmt.Fprintf(archiveStdout, "would skip %s: %v\n", issuePath, err)
			fmt.Fprintln(archiveStdout, "dry run: nothing would be archived; nothing was moved")
			return 0
		}
		fmt.Fprintf(archiveStdout, "would archive %s -> %s (%s)\n", issuePath, filepath.Join(archiveDir, filepath.Base(issuePath)), writeVerb(filepath.Join(archiveDir, filepath.Base(issuePath))))
		dryRunDone(archiveStdout, "jirafs archive issue "+key)
		return 0
	}
	if err := os.MkdirAll(archiveDir, 0o755); err != nil {
		fmt.Fprintf(archiveStderr, "jirafs archive issue %s: %v\n", key, err)
		return 1
	}

	archiveErr := archive.FileService{ArchiveDir: archiveDir}.Archive(key, ctx.MirrorDir, proj.LocalDirs[0], issuePath)
	if archiveErr != nil {
		fmt.Fprintf(archiveStderr, "jirafs archive issue %s: %v\n", key, archiveErr)
		return 1
	}

	fmt.Fprintf(archiveStdout, "jirafs: archived %s -> %s\n", issuePath, archiveDir)
	return 0
}

// runArchiveList handles `jirafs archive list`. It prints the basenames of
// every archived issue snapshot in the project's _archive directory.
func runArchiveList(args []string, settings *config.Settings) int {
	fs := quietFlagSet("archive list")
	projectFlag := fs.String("project", "", "name of a configured project")
	cwdFlag := fs.String("cwd", "", "working directory to resolve the project from")
	if err := parseFlags(fs, args); err != nil {
		return flagError(archiveStderr, "jirafs archive list", err)
	}

	cwd := *cwdFlag
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			fmt.Fprintf(archiveStderr, "jirafs archive list: cannot determine working directory: %v\n", err)
			return 1
		}
	}

	ctx, ok := resolveArchiveContext(settings, *projectFlag, cwd)
	if !ok {
		return 1
	}

	proj, ok := settings.Projects[ctx.Name]
	if !ok || len(proj.LocalDirs) == 0 {
		fmt.Fprintln(archiveStderr, "jirafs archive list: no local directories configured for this project")
		fmt.Fprintln(archiveStderr, `run "jirafs pull my-issues" to create local dirs`)
		return 1
	}

	archiveDir := filepath.Join(proj.LocalDirs[0], "_archive")
	return listArchive(archiveDir)
}

// listArchive prints the basenames of every *.md file in archiveDir. When the
// directory is missing or empty it prints a friendly empty-state message.
func listArchive(archiveDir string) int {
	entries, err := os.ReadDir(archiveDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintln(archiveStdout, "jirafs: no archived issues")
			return 0
		}
		fmt.Fprintf(archiveStderr, "jirafs archive list: %v\n", err)
		return 1
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(entry.Name(), ".md") {
			names = append(names, entry.Name())
		}
	}

	if len(names) == 0 {
		fmt.Fprintln(archiveStdout, "jirafs: no archived issues")
		return 0
	}

	sort.Strings(names)
	for _, name := range names {
		fmt.Fprintln(archiveStdout, name)
	}
	return 0
}

// resolveArchiveContext resolves the active project context for archive
// operations. It mirrors resolveMirrorContext but emits archive-named messages
// to archiveStderr. On failure it prints an error plus a next-step hint and
// returns false.
func resolveArchiveContext(settings *config.Settings, project, cwd string) (*context.Context, bool) {
	res := context.NewResolver(settings, project)
	ctx, err := res.Resolve(cwd)
	if err != nil {
		var ce *context.Error
		if context.IsContextError(err, &ce) {
			if ce.Code == config.ErrNoProjectResolved {
				fmt.Fprintf(archiveStderr, "jirafs archive: no project resolved for cwd %q\n", cwd)
				if len(ce.Candidates) > 0 {
					fmt.Fprintln(archiveStderr, "Available projects:")
					for _, name := range ce.Candidates {
						fmt.Fprintf(archiveStderr, "  - %s\n", name)
					}
				}
				return nil, false
			}
			fmt.Fprintf(archiveStderr, "jirafs archive: %v\n", err)
			return nil, false
		}
		fmt.Fprintf(archiveStderr, "jirafs archive: %v\n", err)
		return nil, false
	}
	return ctx, true
}

// RunArchive dispatches the `jirafs archive` subcommand to the appropriate
// handler. It returns an exit code (0 on success, 1 on error).
func RunArchive(args []string) int {
	if len(args) == 0 {
		return usageError(archiveStderr, "jirafs archive", "missing subcommand")
	}

	// Help and usage errors never need settings.
	if wantsHelpOrWord(args[:1]) {
		printArchiveHelp()
		return 0
	}
	switch args[0] {
	case "issue":
		if wantsHelp(args[1:]) {
			printArchiveIssueHelp()
			return 0
		}
		settings, ok := loadSettingsOrHint(archiveStderr, "archive issue")
		if !ok {
			return 1
		}
		return runArchiveIssue(args[1:], settings)
	case "sweep":
		if wantsHelp(args[1:]) {
			printArchiveSweepHelp()
			return 0
		}
		settings, ok := loadSettingsOrHint(archiveStderr, "archive sweep")
		if !ok {
			return 1
		}
		return runArchiveSweep(args[1:], settings)
	case "list":
		if wantsHelp(args[1:]) {
			printArchiveListHelp()
			return 0
		}
		settings, ok := loadSettingsOrHint(archiveStderr, "archive list")
		if !ok {
			return 1
		}
		return runArchiveList(args[1:], settings)
	default:
		return usageError(archiveStderr, "jirafs archive", "unknown subcommand %q", args[0])
	}
}

func printArchiveIssueHelp() {
	PrintUsage(archiveStdout, Usage{
		Synopsis:    []string{"archive issue <KEY> [flags]"},
		Description: []string{"Moves one resolved issue file into the project's _archive directory."},
		Sections: []UsageSection{{Title: "Flags", Rows: [][2]string{
			{"--project KEY", "name of a configured project"},
			{"--cwd DIR", "working directory to resolve the project from"},
			{"--dry-run", "print what would be archived; move nothing"},
			{"--help, -h", "show this help message"},
		}}},
	})
}

func printArchiveSweepHelp() {
	PrintUsage(archiveStdout, Usage{
		Synopsis:    []string{"archive sweep [flags]"},
		Description: []string{"Reports archive-eligible issues in the project's local directories; with --apply, archives them."},
		Sections: []UsageSection{{Title: "Flags", Rows: [][2]string{
			{"--project KEY", "name of a configured project"},
			{"--cwd DIR", "working directory for project resolution"},
			{"--apply", "actually archive eligible issues"},
			{"--dry-run", "list eligible issues only (the default)"},
			{"--help, -h", "show this help message"},
		}}},
	})
}

func printArchiveListHelp() {
	PrintUsage(archiveStdout, Usage{
		Synopsis:    []string{"archive list [flags]"},
		Description: []string{"Lists archived issue snapshots in the project's _archive directory."},
		Sections: []UsageSection{{Title: "Flags", Rows: [][2]string{
			{"--project KEY", "name of a configured project"},
			{"--cwd DIR", "working directory to resolve the project from"},
			{"--help, -h", "show this help message"},
		}}},
	})
}

// printArchiveHelp prints usage information for the archive subcommand.
func printArchiveHelp() {
	fmt.Fprintf(archiveStdout, "%s\n", color.BoldBlue(archiveStdout, "Usage:"))
	fmt.Fprintf(archiveStdout, "  jirafs %s <subcommand> [flags]\n\n", color.Blue(archiveStdout, "archive"))

	fmt.Fprintf(archiveStdout, "%s\n", color.Dim(archiveStdout, "Moves resolved issues out of the live workspace into a per-project"))
	fmt.Fprintf(archiveStdout, "%s\n\n", color.Dim(archiveStdout, "_archive directory and lists what has been archived."))

	fmt.Fprintf(archiveStdout, "%s:\n", color.BoldGreen(archiveStdout, "Subcommands"))
	fmt.Fprintf(archiveStdout, "  %s   %s\n", color.Yellow(archiveStdout, "issue"), color.Dim(archiveStdout, "archive a single issue by key"))
	fmt.Fprintf(archiveStdout, "  %s   %s\n", color.Yellow(archiveStdout, "sweep"), color.Dim(archiveStdout, "report (or with --apply, archive) archive-eligible issues"))
	fmt.Fprintf(archiveStdout, "  %s   %s\n", color.Yellow(archiveStdout, "list"), color.Dim(archiveStdout, "list archived issue snapshots"))

	fmt.Fprintf(archiveStdout, "%s:\n", color.BoldGreen(archiveStdout, "Flags"))
	fmt.Fprintf(archiveStdout, "  %s   %s\n", color.Yellow(archiveStdout, "--project"), color.Dim(archiveStdout, "name of a configured project"))
	fmt.Fprintf(archiveStdout, "  %s   %s\n", color.Yellow(archiveStdout, "--cwd"), color.Dim(archiveStdout, "working directory to resolve the project from"))
	fmt.Fprintf(archiveStdout, "  %s   %s\n", color.Yellow(archiveStdout, "--help, -h"), color.Dim(archiveStdout, "show this help message"))
}

// runArchiveSweep handles `jirafs archive sweep`.
// It resolves the project context, loads the mirror, scans all issue files
// in the project's local directories, and reports eligible issues without
// mutation. With --apply, it also calls the archive service interface
// to move eligible issues.
func runArchiveSweep(args []string, settings *config.Settings) int {
	fs := quietFlagSet("archive sweep")
	// --project overrides the auto-detected project.
	projectFlag := fs.String("project", "", "name of a configured project")
	// --cwd overrides the working directory for project resolution.
	cwdFlag := fs.String("cwd", "", "working directory to use for project resolution")
	// --apply actually moves eligible issues through the archive service.
	applyFlag := fs.Bool("apply", false, "actually archive eligible issues")
	dryRunFlag := fs.Bool("dry-run", false, "list eligible issues only (the default)")
	if err := parseFlags(fs, args); err != nil {
		return flagError(archiveStderr, "jirafs archive sweep", err)
	}
	if dryRunConflicts(archiveStderr, "jirafs archive sweep", *dryRunFlag, *applyFlag) {
		return 2
	}

	// Resolve the current working directory.
	cwd := "."
	if *cwdFlag != "" {
		cwd = *cwdFlag
	}

	ctx, ok := resolveArchiveContext(settings, *projectFlag, cwd)
	if !ok {
		return 1
	}

	// Load the mirror from the mirror directory.
	m, _, err := loadMirrorYAML(ctx.MirrorDir)
	if err != nil {
		fmt.Fprintf(archiveStderr, "jirafs archive sweep: cannot load mirror: %v\n", err)
		return 1
	}

	// Scan all issue files in the project's local directories.
	eligible, err := scanEligibleIssues(ctx, m, settings)
	if err != nil {
		fmt.Fprintf(archiveStderr, "jirafs archive sweep: %v\n", err)
		return 1
	}

	// Report results.
	if len(eligible) == 0 {
		fmt.Fprintln(archiveStdout, "jirafs archive sweep: no eligible issues found")
		return 0
	}

	fmt.Fprintf(archiveStdout, "jirafs archive sweep: %d eligible issue(s) for %q:\n", len(eligible), ctx.Name)
	for _, e := range eligible {
		fmt.Fprintf(archiveStdout, "  %s (resolved: %s)\n", e.Key, e.ResolvedStatus)
	}

	// With --apply, call the archive service for each eligible issue.
	if !*applyFlag {
		if *dryRunFlag {
			dryRunDone(archiveStdout, "jirafs archive sweep --apply")
		}
		return 0
	}

	// Build the archive service.
	archiveSvc, err := archiveServiceFactory(settings, ctx, cwd)
	if err != nil {
		fmt.Fprintf(archiveStderr, "jirafs archive sweep: cannot create archive service: %v\n", err)
		return 1
	}

	// Archive each eligible issue.
	proj, ok := settings.Projects[ctx.Name]
	if !ok {
		fmt.Fprintf(archiveStderr, "jirafs archive sweep: project %q not found in settings\n", ctx.Name)
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
			fmt.Fprintf(archiveStdout, "  archived: %s\n", e.Key)
			successCount++
		}
	}

	if failCount > 0 {
		fmt.Fprintf(archiveStderr, "jirafs archive sweep: %d/%d issues archived successfully\n", successCount, len(eligible))
		for _, e := range errors {
			fmt.Fprintf(archiveStderr, "  error: %s\n", e)
		}
		return 1
	}

	fmt.Fprintf(archiveStdout, "jirafs archive sweep: archived %d issue(s)\n", successCount)
	return 0
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
