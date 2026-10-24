// Package cli provides command implementations for the jirafs CLI.
package cli

import (
	stdcontext "context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jirafs/jirafs/internal/archive"
	"github.com/jirafs/jirafs/internal/color"
	"github.com/jirafs/jirafs/internal/config"
	"github.com/jirafs/jirafs/internal/context"
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
	mirrorNow                       = time.Now
	mirrorStderr          io.Writer = os.Stderr
	mirrorClientFactory             = newJiraClient
	archiveServiceFactory           = buildArchiveService
)

// RunMirror dispatches the `jirafs mirror` subcommand to the appropriate
// sub-subcommand. It returns an exit code (0 on success, 1 on error).
func RunMirror(args []string) int {
	cmdCtx, stop := commandContext()
	defer stop()
	if len(args) == 0 {
		return usageError(mirrorStderr, "jirafs mirror", "missing subcommand")
	}

	// Help and usage errors never need settings.
	if wantsHelpOrWord(args[:1]) {
		printMirrorHelp()
		return 0
	}
	switch args[0] {
	case "refresh", "current-sprint", "my-issues":
		if wantsHelp(args[1:]) {
			printMirrorRefreshHelp()
			return 0
		}
	case "archive-sweep":
		if wantsHelp(args[1:]) {
			printMirrorArchiveSweepHelp()
			return 0
		}
	default:
		return usageError(mirrorStderr, "jirafs mirror", "unknown subcommand %q", args[0])
	}

	// Load settings and create resolver.
	settings, ok := loadSettingsOrHint(mirrorStderr, "mirror")
	if !ok {
		return 1
	}
	resolver := context.NewResolver(settings, "")

	switch args[0] {
	case "refresh":
		return runMirrorRefresh(cmdCtx, args[1:], settings, resolver)
	case "archive-sweep":
		return runMirrorArchiveSweep(args[1:], settings, resolver)
	default: // current-sprint, my-issues
		return runMirrorRefresh(cmdCtx, append(args[1:], args[0]), settings, resolver)
	}
}

func runMirrorRefresh(cmdCtx stdcontext.Context, args []string, settings *config.Settings, resolver *context.Resolver) int {
	fs := quietFlagSet("mirror refresh")
	projectFlag := fs.String("project", "", "project key or name to refresh")
	cwdFlag := fs.String("cwd", "", "working directory to use for project resolution")
	dryRunFlag := fs.Bool("dry-run", false, "list the issue files that would be written; change nothing")
	fullFlag := fs.Bool("full", false, "ignore the last-sync cursor and refetch every issue of the scope")
	if err := parseFlags(fs, args); err != nil {
		return flagError(mirrorStderr, "jirafs mirror refresh", err)
	}

	if len(fs.Args()) == 0 {
		return usageError(mirrorStderr, "jirafs mirror refresh", "missing scope name")
	}
	if len(fs.Args()) > 1 {
		return usageError(mirrorStderr, "jirafs mirror refresh", "too many positional arguments")
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
		builtin, ok := mirror.BuiltinScope(scopeName)
		if !ok {
			printUnknownScope(scopeName, m)
			return 1
		}
		scope = builtin
	}
	if *fullFlag {
		scope.LastSynced = ""
	}
	jql, err := scope.SearchJQL()
	if err != nil {
		fmt.Fprintf(mirrorStderr, "jirafs mirror refresh: %v\n", err)
		return 1
	}

	client, err := mirrorClientFactory(settings, ctx, cwd)
	if err != nil {
		fmt.Fprintf(mirrorStderr, "jirafs mirror refresh: cannot create Jira client: %v\n", err)
		return 1
	}

	if *dryRunFlag {
		return mirrorRefreshDryRun(client, settings, ctx, scopeName, jql)
	}

	startedAt := mirrorNow()
	added, err := mirror.RefreshScope(stdcontext.Background(), client, scope, m)
	if err != nil {
		fmt.Fprintf(mirrorStderr, "jirafs mirror refresh: %v\n", err)
		return 1
	}

	res, err := materializeScopeIssues(cmdCtx, client, settings, ctx, jql)
	if err != nil {
		fmt.Fprintf(mirrorStderr, "jirafs mirror refresh: cannot write issue snapshots: %v\n", err)
		return 1
	}
	// Newly added members whose file failed to write are not recorded.
	failures := res.Failures
	failedKeys := make(map[schema.IssueKey]bool, len(failures))
	for _, f := range failures {
		failedKeys[f.Key] = true
	}
	kept := added[:0]
	for _, k := range added {
		if failedKeys[k] {
			m.RemoveScopeMember(k)
			continue
		}
		kept = append(kept, k)
	}
	added = kept

	// writeSyncedIssue recorded remote timestamps on disk; mirror in memory so
	// saving m below does not discard them.
	for key, updated := range res.RemoteUpdated {
		m.SetRemoteUpdated(key, updated)
	}
	// The cursor only advances after a refresh without any failure.
	if len(failures) == 0 {
		m.SetScopeLastSynced(scope, startedAt)
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
	} else {
		fmt.Fprintf(mirrorStdout, "jirafs mirror refresh: %d changed issue key(s) for scope %q in %q:\n", len(added), scopeName, ctx.Name)
		for _, key := range added {
			fmt.Fprintf(mirrorStdout, "  %s\n", key)
		}
	}
	summary := fmt.Sprintf("fetched %d, updated %d, unchanged %d", res.Fetched, res.Updated, res.Unchanged)
	if len(res.LocalEdits) > 0 {
		summary += fmt.Sprintf(", kept local edits %d", len(res.LocalEdits))
	}
	fmt.Fprintf(mirrorStdout, "jirafs mirror refresh: %s\n", summary)
	for _, key := range res.LocalEdits {
		fmt.Fprintf(mirrorStderr, "%s: local edits kept, not overwritten (inspect: jirafs status; overwrite: jirafs export issue %s)\n", key, key)
	}
	if len(failures) == 0 {
		return 0
	}
	for _, f := range failures {
		fmt.Fprintf(mirrorStderr, "%s: %s\n", f.Key, f.Reason)
	}
	fmt.Fprintf(mirrorStderr, "refreshed %d, failed %d\n", res.Updated, len(failures))
	fmt.Fprintln(mirrorStderr, "retry each failed issue:")
	for _, f := range failures {
		fmt.Fprintf(mirrorStderr, "  jirafs export issue %s\n", f.Key)
	}
	return 1
}

// refreshFailure records one issue that could not be written during a refresh.
type refreshFailure struct {
	Key    schema.IssueKey
	Reason string
}

// mirrorRefreshDryRun lists the issue files a refresh of scopeName would
// write, using only read calls, and changes nothing locally or remotely.
func mirrorRefreshDryRun(client jira.Client, settings *config.Settings, ctx *context.Context, scopeName, jql string) int {
	proj, ok := settings.Projects[ctx.Name]
	if !ok || len(proj.LocalDirs) == 0 {
		fmt.Fprintf(mirrorStderr, "jirafs mirror refresh: project %q has no local_dirs configured\n", ctx.Name)
		return 1
	}
	issues, _, err := client.SearchIssues(stdcontext.Background(), jql)
	if err != nil {
		fmt.Fprintf(mirrorStderr, "jirafs mirror refresh: %v\n", err)
		return 1
	}
	var lines []string
	for _, issue := range issues {
		if issue == nil || issue.Identity.Key == "" {
			continue
		}
		dest := filepath.Join(proj.LocalDirs[0], string(issue.Identity.Key)+".md")
		lines = append(lines, fmt.Sprintf("  would %s %s", writeVerb(dest), dest))
	}
	sort.Strings(lines)
	fmt.Fprintf(mirrorStdout, "jirafs mirror refresh: would write %d issue file(s) for scope %q in %q and update mirror.yml:\n", len(lines), scopeName, ctx.Name)
	for _, l := range lines {
		fmt.Fprintln(mirrorStdout, l)
	}
	dryRunDone(mirrorStdout, "jirafs mirror refresh "+scopeName)
	return 0
}

// refreshResult summarizes one materializeScopeIssues run.
type refreshResult struct {
	// Failures are the issues whose file could not be written.
	Failures []refreshFailure
	// Fetched is the number of issues the search returned.
	Fetched int
	// Updated is the number of issue files written.
	Updated int
	// Unchanged is the number of issues whose local file already carries the
	// remote updated timestamp; they are not rewritten.
	Unchanged int
	// LocalEdits are issues with unsynced local edits; they are never
	// overwritten by a refresh.
	LocalEdits []schema.IssueKey
	// RemoteUpdated holds the remote updated timestamp to record per issue.
	RemoteUpdated map[schema.IssueKey]string
}

// localIssueState classifies an existing local issue file against the remote
// updated timestamp.
type localIssueState int

const (
	localAbsent localIssueState = iota
	localStale
	localCurrent
	localEdited
)

// classifyLocalIssue reads dest and reports whether it is missing, stale,
// already at remoteUpdated, or carries local edits (its content no longer
// matches its synced_hash).
func classifyLocalIssue(dest, remoteUpdated string) localIssueState {
	data, err := os.ReadFile(dest)
	if err != nil {
		return localAbsent
	}
	doc := string(data)
	if h := schema.SyncedHash(doc); h != "" && h != schema.ContentHash(doc) {
		return localEdited
	}
	if remoteUpdated == "" {
		return localStale
	}
	local, perr := schema.ParseIssue(doc)
	if perr == nil && local.Context.Updated == remoteUpdated {
		return localCurrent
	}
	return localStale
}

// materializeScopeIssues writes one file per issue matching jql. Issues whose
// local file already has the remote updated timestamp are skipped, and files
// with local edits are never overwritten. A failure writing a single issue is
// recorded and processing continues; only fatal errors (settings, directory
// creation, search) are returned as err.
func materializeScopeIssues(ctx stdcontext.Context, client jira.Client, settings *config.Settings, projectCtx *context.Context, jql string) (*refreshResult, error) {
	proj, ok := settings.Projects[projectCtx.Name]
	if !ok {
		return nil, fmt.Errorf("project %q not found in settings", projectCtx.Name)
	}
	if len(proj.LocalDirs) == 0 {
		return nil, fmt.Errorf("project %q has no local_dirs configured", projectCtx.Name)
	}
	if err := os.MkdirAll(proj.LocalDirs[0], 0o755); err != nil {
		return nil, fmt.Errorf("create local directory %s: %w", proj.LocalDirs[0], err)
	}

	issues, _, err := client.SearchIssues(ctx, jql)
	if err != nil {
		return nil, err
	}
	// Only issues that will actually be written count towards progress.
	writable := make([]*schema.Issue, 0, len(issues))
	for _, issue := range issues {
		if issue != nil && issue.Identity.Key != "" {
			writable = append(writable, issue)
		}
	}
	total := len(writable)
	res := &refreshResult{Fetched: total, RemoteUpdated: map[schema.IssueKey]string{}}
	writeMirrorProgress(mirrorStdout, "fetching", 0, total)
	if total == 0 {
		writeMirrorProgress(mirrorStdout, "fetched", 0, total)
		fmt.Fprintln(mirrorStdout)
		return res, nil
	}
	completed := 0
	for _, issue := range writable {
		completed++
		key := issue.Identity.Key
		dest := filepath.Join(proj.LocalDirs[0], string(key)+".md")
		switch classifyLocalIssue(dest, issue.Context.Updated) {
		case localCurrent:
			res.Unchanged++
			writeMirrorProgress(mirrorStdout, "fetching", completed, total)
			continue
		case localEdited:
			res.LocalEdits = append(res.LocalEdits, key)
			if issue.Context.Updated != "" {
				res.RemoteUpdated[key] = issue.Context.Updated
			}
			writeMirrorProgress(mirrorStdout, "fetching", completed, total)
			continue
		}
		renderIssue := issue
		fullIssue, err := client.FetchIssue(ctx, string(key))
		if err == nil && fullIssue != nil && fullIssue.Identity.Key != "" {
			renderIssue = fullIssue
		}
		jira.AttachComments(ctx, client, renderIssue, mirrorStderr)
		dest = filepath.Join(proj.LocalDirs[0], string(renderIssue.Identity.Key)+".md")
		if err := writeSyncedIssue(dest, projectCtx.MirrorDir, renderIssue); err != nil {
			res.Failures = append(res.Failures, refreshFailure{
				Key:    key,
				Reason: fmt.Sprintf("write issue file %s: %v", dest, err),
			})
		} else {
			res.Updated++
			if renderIssue.Context.Updated != "" {
				res.RemoteUpdated[renderIssue.Identity.Key] = renderIssue.Context.Updated
			}
		}
		writeMirrorProgress(mirrorStdout, "fetching", completed, total)
	}
	writeMirrorProgress(mirrorStdout, "fetched", total, total)
	fmt.Fprintln(mirrorStdout)
	return res, nil
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
	fs := quietFlagSet("mirror archive-sweep")
	// --project overrides the auto-detected project.
	projectFlag := fs.String("project", "", "project key or name to scan")
	// --cwd overrides the working directory for project resolution.
	cwdFlag := fs.String("cwd", "", "working directory to use for project resolution")
	// --apply actually moves eligible issues through the archive service.
	applyFlag := fs.Bool("apply", false, "actually archive eligible issues")
	dryRunFlag := fs.Bool("dry-run", false, "list eligible issues only (the default)")
	if err := parseFlags(fs, args); err != nil {
		return flagError(mirrorStderr, "jirafs mirror archive-sweep", err)
	}
	if dryRunConflicts(mirrorStderr, "jirafs mirror archive-sweep", *dryRunFlag, *applyFlag) {
		return 2
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
		fmt.Fprintln(mirrorStdout, "jirafs mirror archive-sweep: no eligible issues found")
		return 0
	}

	fmt.Fprintf(mirrorStdout, "jirafs mirror archive-sweep: %d eligible issue(s) for %q:\n", len(eligible), ctx.Name)
	for _, e := range eligible {
		fmt.Fprintf(mirrorStdout, "  %s (resolved: %s)\n", e.Key, e.ResolvedStatus)
	}

	// With --apply, call the archive service for each eligible issue.
	if !*applyFlag {
		if *dryRunFlag {
			dryRunDone(mirrorStdout, "jirafs mirror archive-sweep --apply")
		}
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
	fmt.Fprintf(mirrorStdout, "%s\n", color.BoldBlue(mirrorStdout, "Usage:"))
	fmt.Fprintf(mirrorStdout, "  jirafs %s <%s> [flags]\n\n", color.Blue(mirrorStdout, "mirror"), color.Yellow(mirrorStdout, "subcommand"))

	fmt.Fprintf(mirrorStdout, "%s:\n", color.BoldGreen(mirrorStdout, "Subcommands"))
	fmt.Fprintf(mirrorStdout, "  %s  %s\n", color.Blue(mirrorStdout, "current-sprint"), color.Dim(mirrorStdout, "shorthand for \"refresh current-sprint\""))
	fmt.Fprintf(mirrorStdout, "  %s       %s\n", color.Blue(mirrorStdout, "my-issues"), color.Dim(mirrorStdout, "shorthand for \"refresh my-issues\""))
	fmt.Fprintf(mirrorStdout, "  %s         %s\n", color.Blue(mirrorStdout, "refresh"), color.Dim(mirrorStdout, "refresh one named live mirror scope"))
	fmt.Fprintf(mirrorStdout, "  %s   %s\n", color.Blue(mirrorStdout, "archive-sweep"), color.Dim(mirrorStdout, "report archive-eligible issues without mutation"))
	fmt.Fprintf(mirrorStdout, "  %s            %s\n\n", color.Blue(mirrorStdout, "help"), color.Dim(mirrorStdout, "show this help message"))

	fmt.Fprintf(mirrorStdout, "%s:\n", color.BoldGreen(mirrorStdout, "Live mirror scopes"))
	fmt.Fprintf(mirrorStdout, "  %s  %s\n", color.Magenta(mirrorStdout, "current-sprint"), color.Dim(mirrorStdout, "active sprint issues"))
	fmt.Fprintf(mirrorStdout, "  %s       %s\n", color.Magenta(mirrorStdout, "my-issues"), color.Dim(mirrorStdout, "issues assigned to the current user"))
	fmt.Fprintf(mirrorStdout, "  %s  %s\n\n", color.Magenta(mirrorStdout, "<name>"), color.Dim(mirrorStdout, "any scope (project, component or jql) defined in mirror.yml"))

	fmt.Fprintf(mirrorStdout, "%s:\n", color.BoldGreen(mirrorStdout, "Flags"))
	fmt.Fprintf(mirrorStdout, "  %s KEY   %s\n", color.Yellow(mirrorStdout, "--project"), color.Dim(mirrorStdout, "project key or name to use"))
	fmt.Fprintf(mirrorStdout, "  %s DIR   %s\n", color.Yellow(mirrorStdout, "--cwd"), color.Dim(mirrorStdout, "working directory for project resolution"))
	fmt.Fprintf(mirrorStdout, "  %s         %s\n", color.Yellow(mirrorStdout, "--apply"), color.Dim(mirrorStdout, "actually archive eligible issues"))
	fmt.Fprintf(mirrorStdout, "  %s       %s\n", color.Yellow(mirrorStdout, "--dry-run"), color.Dim(mirrorStdout, "list only; on refresh also list the files that would be written"))
	fmt.Fprintf(mirrorStdout, "  %s          %s\n\n", color.Yellow(mirrorStdout, "--full"), color.Dim(mirrorStdout, "on refresh, ignore the last-sync cursor and refetch every issue"))

	fmt.Fprintf(mirrorStdout, "%s\n", color.Cyan(mirrorStdout, `Run "jirafs mirror <subcommand> --help" for more information about a subcommand.`))
}

func printMirrorRefreshHelp() {
	fmt.Fprintf(mirrorStdout, "%s\n", color.BoldBlue(mirrorStdout, "Usage:"))
	fmt.Fprintf(mirrorStdout, "  jirafs %s %s [flags] <%s>\n", color.Blue(mirrorStdout, "mirror"), color.Blue(mirrorStdout, "refresh"), color.Yellow(mirrorStdout, "scope"))
	fmt.Fprintf(mirrorStdout, "  jirafs %s <%s> [flags]\n\n", color.Blue(mirrorStdout, "mirror"), color.Yellow(mirrorStdout, "scope"))

	fmt.Fprintf(mirrorStdout, "%s\n\n", color.Dim(mirrorStdout, "Refreshes one named live mirror scope."))

	fmt.Fprintf(mirrorStdout, "%s:\n", color.BoldGreen(mirrorStdout, "Live mirror scopes"))
	fmt.Fprintf(mirrorStdout, "  %s\n", color.Magenta(mirrorStdout, "current-sprint"))
	fmt.Fprintf(mirrorStdout, "  %s\n", color.Magenta(mirrorStdout, "my-issues"))
	fmt.Fprintf(mirrorStdout, "  %s\n", color.Magenta(mirrorStdout, "<name> (any scope in mirror.yml)"))
	fmt.Fprintln(mirrorStdout)

	fmt.Fprintf(mirrorStdout, "%s:\n", color.BoldGreen(mirrorStdout, "Examples"))
	fmt.Fprintf(mirrorStdout, "  %s\n", color.Cyan(mirrorStdout, "jirafs mirror my-issues"))
	fmt.Fprintf(mirrorStdout, "  %s\n", color.Cyan(mirrorStdout, "jirafs mirror refresh my-issues"))
	fmt.Fprintf(mirrorStdout, "  %s\n\n", color.Cyan(mirrorStdout, "jirafs mirror refresh current-sprint"))

	fmt.Fprintf(mirrorStdout, "%s:\n", color.BoldGreen(mirrorStdout, "Flags"))
	fmt.Fprintf(mirrorStdout, "  %s KEY   %s\n", color.Yellow(mirrorStdout, "--project"), color.Dim(mirrorStdout, "project key or name to use"))
	fmt.Fprintf(mirrorStdout, "  %s DIR   %s\n", color.Yellow(mirrorStdout, "--cwd"), color.Dim(mirrorStdout, "working directory for project resolution"))
	fmt.Fprintf(mirrorStdout, "  %s          %s\n", color.Yellow(mirrorStdout, "--help"), color.Dim(mirrorStdout, "show this help message"))
}

func printMirrorArchiveSweepHelp() {
	PrintUsage(mirrorStdout, Usage{
		Synopsis:    []string{"mirror archive-sweep [flags]"},
		Description: []string{"Reports archive-eligible issues in the project's local directories; with --apply, archives them."},
		Sections: []UsageSection{{Title: "Flags", Rows: [][2]string{
			{"--project KEY", "project key or name to scan"},
			{"--cwd DIR", "working directory for project resolution"},
			{"--apply", "actually archive eligible issues"},
			{"--dry-run", "list eligible issues only (the default)"},
			{"--help, -h", "show this help message"},
		}}},
	})
}

// printUnknownScope reports an unknown scope name, listing the configured
// scope names and a copy-pasteable mirror.yml example for adding one.
func printUnknownScope(name string, m *mirror.Mirror) {
	fmt.Fprint(mirrorStderr, unknownScopeMessage(name, m))
}

func unknownScopeMessage(name string, m *mirror.Mirror) string {
	var b strings.Builder
	fmt.Fprintf(&b, "jirafs mirror refresh: scope %q not found in mirror\n", name)
	names := []string{"my-issues (built-in)", "current-sprint (built-in)"}
	for _, s := range m.Scopes {
		names = append(names, s.Name)
	}
	fmt.Fprintf(&b, "available scopes: %s\n", strings.Join(names, ", "))
	fmt.Fprintf(&b, "to add one, append this to mirror.yml and re-run:\n\nscopes:\n  - name: %s\n    type: jql\n    target: project = PLAT AND status != Done ORDER BY key\n", name)
	return b.String()
}
