// Package cli provides command implementations for the jirafs CLI.
package cli

import (
	stdcontext "context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/jirafs/jirafs/internal/config"
	"github.com/jirafs/jirafs/internal/context"
	"github.com/jirafs/jirafs/internal/export"
	"github.com/jirafs/jirafs/internal/jira"
	"github.com/jirafs/jirafs/internal/mirror"
	"github.com/jirafs/jirafs/internal/schema"
	"gopkg.in/yaml.v3"
)

var (
	pullStdout        io.Writer = os.Stdout
	pullStderr        io.Writer = os.Stderr
	pullNow                     = time.Now
	pullClientFactory           = newJiraClient
)

// issueKeyPattern matches a Jira issue key such as PLAT-123.
var issueKeyPattern = regexp.MustCompile(`^[A-Z][A-Z0-9]+-\d+$`)

// isIssueArg reports whether arg names a single issue: an issue key or a
// Jira URL. Anything else is treated as a scope name.
func isIssueArg(arg string) bool {
	return issueKeyPattern.MatchString(arg) || strings.Contains(arg, "://")
}

// RunPull handles `jirafs pull [scope|KEY]`. A key (or issue URL) pulls that
// one issue; a scope name from mirror.yml (or a built-in scope) is refreshed
// incrementally; no argument pulls every configured scope.
func RunPull(args []string) int {
	cmdCtx, stop := commandContext()
	defer stop()
	if wantsHelpOrWord(args) {
		printPullHelp()
		return 0
	}

	settings, ok := loadSettingsOrHint(pullStderr, "pull")
	if !ok {
		return 1
	}

	fs := quietFlagSet("pull")
	scope := addScopeFlags(fs)
	stdoutFlag := fs.Bool("stdout", false, "print one issue to stdout and write no file")
	dryRunFlag := fs.Bool("dry-run", false, "print what would be fetched and written; change nothing")
	fullFlag := fs.Bool("full", false, "ignore the last-sync cursor and refetch every issue of the scope")
	if err := parseFlags(fs, args); err != nil {
		return flagError(pullStderr, "jirafs pull", err)
	}
	if len(fs.Args()) > 1 {
		return usageError(pullStderr, "jirafs pull", "too many positional arguments")
	}
	arg := ""
	if len(fs.Args()) == 1 {
		arg = fs.Args()[0]
	}
	issueMode := arg != "" && isIssueArg(arg)
	if *stdoutFlag && !issueMode {
		return usageError(pullStderr, "jirafs pull", "--stdout only applies when pulling one issue KEY")
	}
	if *fullFlag && issueMode {
		return usageError(pullStderr, "jirafs pull", "--full only applies to scopes, not to one issue KEY")
	}

	cwd, err := scope.workdir()
	if err != nil {
		fmt.Fprintf(pullStderr, "jirafs pull: cannot determine working directory: %v\n", err)
		return 1
	}
	ctx, ok := resolveMirrorContext(settings, *scope.project, cwd, "pull")
	if !ok {
		return 1
	}
	m, mirrorPath, err := loadMirrorYAML(ctx.MirrorDir)
	if err != nil {
		fmt.Fprintf(pullStderr, "jirafs pull: cannot load mirror: %v\n", err)
		return 1
	}

	if issueMode {
		if !m.ScopeFor(arg).IsZero() {
			fmt.Fprintf(pullStderr, "jirafs pull: %q is both an issue key and a scope name in mirror.yml; rename the scope in mirror.yml (e.g. %q) and re-run\n", arg, strings.ToLower(arg))
			return 1
		}
		return runPullIssue(cmdCtx, arg, settings, ctx, m, mirrorPath, cwd, *stdoutFlag, *dryRunFlag)
	}

	var names []string
	if arg != "" {
		names = []string{arg}
	} else {
		for _, s := range m.Scopes {
			names = append(names, s.Name)
		}
		if len(names) == 0 {
			fmt.Fprint(pullStderr, noScopesMessage())
			return 1
		}
	}
	for _, name := range names {
		if m.ScopeFor(name).IsZero() {
			if _, ok := mirror.BuiltinScope(name); !ok {
				printUnknownScope(name, m)
				return 1
			}
		}
	}

	client, err := pullClientFactory(settings, ctx, cwd)
	if err != nil {
		fmt.Fprintf(pullStderr, "jirafs pull: cannot create Jira client: %v\n", err)
		return 1
	}
	exit := 0
	for _, name := range names {
		if code := runPullScope(cmdCtx, client, settings, ctx, m, mirrorPath, name, *fullFlag, *dryRunFlag); code != 0 {
			exit = code
		}
	}
	if *dryRunFlag && exit == 0 {
		dryRunDone(pullStdout, "jirafs pull"+realArgs(arg, *fullFlag))
	}
	return exit
}

// realArgs rebuilds the argument tail of the real command for dry-run hints.
func realArgs(arg string, full bool) string {
	tail := ""
	if arg != "" {
		tail += " " + arg
	}
	if full {
		tail += " --full"
	}
	return tail
}

// runPullIssue pulls one issue: it fetches it from Jira and writes it to
// <local_dirs[0]>/<KEY>.md, or prints it with --stdout.
func runPullIssue(cmdCtx stdcontext.Context, arg string, settings *config.Settings, ctx *context.Context, m *mirror.Mirror, mirrorPath, cwd string, toStdout, dryRun bool) int {
	key, err := jira.ParseIssueRef(arg, settings.Instances[ctx.Instance].BaseURL)
	if err != nil {
		fmt.Fprintf(pullStderr, "jirafs pull: %s\n", strings.ReplaceAll(err.Error(), "<name>", ctx.Name))
		return 1
	}

	client, err := pullClientFactory(settings, ctx, cwd)
	if err != nil {
		fmt.Fprintf(pullStderr, "jirafs pull: cannot create Jira client: %v\n", err)
		return 1
	}
	issue, err := client.FetchIssue(cmdCtx, key)
	if err != nil {
		if !toStdout && jira.IsClientError(err, jira.ErrNotFound) {
			pc := &pullGitCommit{Label: key, Host: hostOf(settings.Instances[ctx.Instance].BaseURL)}
			gs := beginPullGit(ctx.MirrorDir, dryRun, pullStderr)
			if code, handled := pullGoneIssue(pc, m, mirrorPath, settings, ctx, key, dryRun); handled {
				gs.commit(*pc, pullNow(), pullStdout, pullStderr)
				return code
			}
		}
		fmt.Fprintf(pullStderr, "jirafs pull: %v\n", err)
		return 1
	}
	if !toStdout && issue.Identity.Key != "" && !strings.EqualFold(string(issue.Identity.Key), key) {
		pc := &pullGitCommit{Label: key, Host: hostOf(settings.Instances[ctx.Instance].BaseURL)}
		gs := beginPullGit(ctx.MirrorDir, dryRun, pullStderr)
		code := pullMovedIssue(pc, m, mirrorPath, settings, ctx, key, issue, dryRun)
		gs.commit(*pc, pullNow(), pullStdout, pullStderr)
		return code
	}
	jira.AttachComments(cmdCtx, client, issue, pullStderr)

	output := export.ExportIssue(issue)
	if toStdout {
		fmt.Fprint(pullStdout, output)
		return 0
	}

	proj, ok := settings.Projects[ctx.Name]
	if !ok || len(proj.LocalDirs) == 0 {
		fmt.Fprintf(pullStderr, "jirafs pull: project %q has no local_dirs configured; use --stdout to print instead\n", ctx.Name)
		return 1
	}
	dest, err := filepath.Abs(filepath.Join(proj.LocalDirs[0], key+".md"))
	if err != nil {
		fmt.Fprintf(pullStderr, "jirafs pull: %v\n", err)
		return 1
	}
	if dryRun {
		fmt.Fprintf(pullStdout, "would %s %s (%d bytes)\n", writeVerb(dest), dest, len(output))
		dryRunDone(pullStdout, "jirafs pull "+key)
		return 0
	}
	if err := os.MkdirAll(proj.LocalDirs[0], 0o755); err != nil {
		fmt.Fprintf(pullStderr, "jirafs pull: create local directory %s: %v\n", proj.LocalDirs[0], err)
		return 1
	}
	gitState := beginPullGit(ctx.MirrorDir, dryRun, pullStderr)
	if err := writeSyncedIssue(dest, ctx.MirrorDir, issue); err != nil {
		fmt.Fprintf(pullStderr, "jirafs pull: write %s: %v\n", dest, err)
		return 1
	}
	fmt.Fprintf(pullStdout, "wrote %s → %s\n", key, dest)
	gitState.commit(pullGitCommit{Label: key, Host: hostOf(settings.Instances[ctx.Instance].BaseURL), Written: map[string]schema.IssueKey{dest: schema.IssueKey(key)}}, pullNow(), pullStdout, pullStderr)
	return 0
}

// runPullScope refreshes one named scope (configured or built-in)
// incrementally and saves the mirror. With dryRun it only lists the files
// that would be written.
func runPullScope(cmdCtx stdcontext.Context, client jira.Client, settings *config.Settings, ctx *context.Context, m *mirror.Mirror, mirrorPath, scopeName string, full, dryRun bool) int {
	scope := m.ScopeFor(scopeName)
	if scope.IsZero() {
		scope, _ = mirror.BuiltinScope(scopeName)
	}
	if full {
		scope.LastSynced = ""
	}
	jql, err := scope.SearchJQL()
	if err != nil {
		fmt.Fprintf(pullStderr, "jirafs pull: %v\n", err)
		return 1
	}

	if dryRun {
		return pullScopeDryRun(cmdCtx, client, settings, ctx, m, scopeName, jql, full)
	}

	startedAt := pullNow()
	added, err := mirror.RefreshScope(stdcontext.Background(), client, scope, m)
	if err != nil {
		fmt.Fprintf(pullStderr, "jirafs pull: %v\n", err)
		return 1
	}

	gitState := beginPullGit(ctx.MirrorDir, dryRun, pullStderr)
	res, err := materializeScopeIssues(cmdCtx, client, settings, ctx, jql)
	if err != nil {
		fmt.Fprintf(pullStderr, "jirafs pull: cannot write issue snapshots: %v\n", err)
		return 1
	}
	checkFailed := 0
	pc := &pullGitCommit{Label: scopeName, Scope: true, Host: hostOf(settings.Instances[ctx.Instance].BaseURL), Written: res.Written}
	if full {
		_, checkFailed = checkVanishedMembers(pc, cmdCtx, client, m, scope.Name, settings.Projects[ctx.Name].LocalDirs[0], ctx.MirrorDir, res.Seen, false)
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
	// The cursor only advances after a pull without any failure.
	if len(failures) == 0 && checkFailed == 0 {
		m.SetScopeLastSynced(scope, startedAt)
	}

	if err := saveMirrorYAML(mirrorPath, m); err != nil {
		fmt.Fprintf(pullStderr, "jirafs pull: cannot save mirror: %v\n", err)
		return 1
	}

	sort.Slice(added, func(i, j int) bool {
		return added[i] < added[j]
	})
	if len(added) == 0 {
		fmt.Fprintf(pullStdout, "jirafs pull: no changed issue keys for scope %q in %q\n", scopeName, ctx.Name)
	} else {
		fmt.Fprintf(pullStdout, "jirafs pull: %d changed issue key(s) for scope %q in %q:\n", len(added), scopeName, ctx.Name)
		for _, key := range added {
			fmt.Fprintf(pullStdout, "  %s\n", key)
		}
	}
	summary := fmt.Sprintf("fetched %d, updated %d, unchanged %d", res.Fetched, res.Updated, res.Unchanged)
	if len(res.LocalEdits) > 0 {
		summary += fmt.Sprintf(", kept local edits %d", len(res.LocalEdits))
	}
	fmt.Fprintf(pullStdout, "jirafs pull: %s\n", summary)
	for _, key := range res.LocalEdits {
		fmt.Fprintf(pullStderr, "%s: local edits kept, not overwritten (inspect: jirafs status; overwrite: jirafs pull %s)\n", key, key)
	}
	gitState.commit(*pc, pullNow(), pullStdout, pullStderr)
	if len(failures) == 0 {
		if checkFailed > 0 {
			return 1
		}
		return 0
	}
	for _, f := range failures {
		fmt.Fprintf(pullStderr, "%s: %s\n", f.Key, f.Reason)
	}
	fmt.Fprintf(pullStderr, "pulled %d, failed %d\n", res.Updated, len(failures))
	fmt.Fprintln(pullStderr, "retry each failed issue:")
	for _, f := range failures {
		fmt.Fprintf(pullStderr, "  jirafs pull %s\n", f.Key)
	}
	return 1
}

// pullScopeDryRun lists the issue files a pull of scopeName would write,
// using only read calls, and changes nothing locally or remotely.
func pullScopeDryRun(cmdCtx stdcontext.Context, client jira.Client, settings *config.Settings, ctx *context.Context, m *mirror.Mirror, scopeName, jql string, full bool) int {
	proj, ok := settings.Projects[ctx.Name]
	if !ok || len(proj.LocalDirs) == 0 {
		fmt.Fprintf(pullStderr, "jirafs pull: project %q has no local_dirs configured\n", ctx.Name)
		return 1
	}
	issues, _, err := client.SearchIssues(cmdCtx, jql)
	if err != nil {
		fmt.Fprintf(pullStderr, "jirafs pull: %v\n", err)
		return 1
	}
	var lines []string
	seen := map[schema.IssueKey]bool{}
	for _, issue := range issues {
		if issue == nil || issue.Identity.Key == "" {
			continue
		}
		seen[issue.Identity.Key] = true
		dest := filepath.Join(proj.LocalDirs[0], string(issue.Identity.Key)+".md")
		lines = append(lines, fmt.Sprintf("  would %s %s", writeVerb(dest), dest))
	}
	sort.Strings(lines)
	fmt.Fprintf(pullStdout, "jirafs pull: would write %d issue file(s) for scope %q in %q and update mirror.yml:\n", len(lines), scopeName, ctx.Name)
	for _, l := range lines {
		fmt.Fprintln(pullStdout, l)
	}
	if full {
		if _, failed := checkVanishedMembers(&pullGitCommit{}, cmdCtx, client, m, scopeName, proj.LocalDirs[0], ctx.MirrorDir, seen, true); failed > 0 {
			return 1
		}
	}
	return 0
}

// pullFailure records one issue that could not be written during a refresh.
type pullFailure struct {
	Key    schema.IssueKey
	Reason string
}

// pullResult summarizes one materializeScopeIssues run.
type pullResult struct {
	// Failures are the issues whose file could not be written.
	Failures []pullFailure
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
	// Written maps every file this run wrote to its issue key.
	Written map[string]schema.IssueKey
	// RemoteUpdated holds the remote updated timestamp to record per issue.
	RemoteUpdated map[schema.IssueKey]string
	// Seen holds every issue key the search returned.
	Seen map[schema.IssueKey]bool
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
func materializeScopeIssues(ctx stdcontext.Context, client jira.Client, settings *config.Settings, projectCtx *context.Context, jql string) (*pullResult, error) {
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
	res := &pullResult{Fetched: total, RemoteUpdated: map[schema.IssueKey]string{}, Written: map[string]schema.IssueKey{}, Seen: map[schema.IssueKey]bool{}}
	writeMirrorProgress(pullStdout, "fetching", 0, total)
	if total == 0 {
		writeMirrorProgress(pullStdout, "fetched", 0, total)
		fmt.Fprintln(pullStdout)
		return res, nil
	}
	completed := 0
	for _, issue := range writable {
		completed++
		key := issue.Identity.Key
		res.Seen[key] = true
		dest := filepath.Join(proj.LocalDirs[0], string(key)+".md")
		switch classifyLocalIssue(dest, issue.Context.Updated) {
		case localCurrent:
			res.Unchanged++
			writeMirrorProgress(pullStdout, "fetching", completed, total)
			continue
		case localEdited:
			res.LocalEdits = append(res.LocalEdits, key)
			if issue.Context.Updated != "" {
				res.RemoteUpdated[key] = issue.Context.Updated
			}
			writeMirrorProgress(pullStdout, "fetching", completed, total)
			continue
		}
		renderIssue := issue
		fullIssue, err := client.FetchIssue(ctx, string(key))
		if err == nil && fullIssue != nil && fullIssue.Identity.Key != "" {
			renderIssue = fullIssue
		}
		jira.AttachComments(ctx, client, renderIssue, pullStderr)
		dest = filepath.Join(proj.LocalDirs[0], string(renderIssue.Identity.Key)+".md")
		if err := writeSyncedIssue(dest, projectCtx.MirrorDir, renderIssue); err != nil {
			res.Failures = append(res.Failures, pullFailure{
				Key:    key,
				Reason: fmt.Sprintf("write issue file %s: %v", dest, err),
			})
		} else {
			res.Updated++
			res.Written[dest] = renderIssue.Identity.Key
			if renderIssue.Context.Updated != "" {
				res.RemoteUpdated[renderIssue.Identity.Key] = renderIssue.Context.Updated
			}
		}
		writeMirrorProgress(pullStdout, "fetching", completed, total)
	}
	writeMirrorProgress(pullStdout, "fetched", total, total)
	fmt.Fprintln(pullStdout)
	return res, nil
}

func writeMirrorProgress(w io.Writer, label string, current, total int) {
	fmt.Fprintf(w, "\r%s %d/%d", label, current, total)
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

func resolveMirrorContext(settings *config.Settings, project, cwd, subcommand string) (*context.Context, bool) {
	res := context.NewResolver(settings, project)
	ctx, err := res.Resolve(cwd)
	if err != nil {
		var ce *context.Error
		if context.IsContextError(err, &ce) {
			if ce.Code == config.ErrNoProjectResolved {
				fmt.Fprintf(pullStderr, "jirafs %s: no project resolved for cwd %q\n", subcommand, cwd)
				if len(ce.Candidates) > 0 {
					fmt.Fprintln(pullStderr, "Available projects:")
					for _, name := range ce.Candidates {
						fmt.Fprintf(pullStderr, "  - %s\n", name)
					}
				}
				return nil, false
			}
		}
		fmt.Fprintf(pullStderr, "jirafs %s: %v\n", subcommand, err)
		return nil, false
	}
	return ctx, true
}

func printPullHelp() {
	PrintCommandHelp(pullStdout, "pull")
}

// printUnknownScope reports an unknown scope name, listing the configured
// scope names and a copy-pasteable mirror.yml example for adding one.
func printUnknownScope(name string, m *mirror.Mirror) {
	fmt.Fprint(pullStderr, unknownScopeMessage(name, m))
}

func unknownScopeMessage(name string, m *mirror.Mirror) string {
	var b strings.Builder
	fmt.Fprintf(&b, "jirafs pull: scope %q not found in mirror\n", name)
	names := []string{"my-issues (built-in)", "current-sprint (built-in)"}
	for _, s := range m.Scopes {
		names = append(names, s.Name)
	}
	fmt.Fprintf(&b, "available scopes: %s\n", strings.Join(names, ", "))
	fmt.Fprintf(&b, "to add one, append this to mirror.yml and re-run:\n\nscopes:\n  - name: %s\n    type: jql\n    target: project = PLAT AND status != Done ORDER BY key\n", name)
	return b.String()
}

// noScopesMessage explains how to configure a scope when `jirafs pull` has
// nothing to pull.
func noScopesMessage() string {
	return "jirafs pull: no scopes configured in mirror.yml\n" +
		"pull a built-in scope: jirafs pull my-issues\n" +
		"or add one to mirror.yml and re-run:\n\nscopes:\n  - name: my-work\n    type: jql\n    target: project = PLAT AND status != Done ORDER BY key\n"
}
