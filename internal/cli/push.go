package cli

import (
	stdcontext "context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jirafs/jirafs/internal/codec"
	"github.com/jirafs/jirafs/internal/config"
	"github.com/jirafs/jirafs/internal/context"
	"github.com/jirafs/jirafs/internal/jira"
	"github.com/jirafs/jirafs/internal/mirror"
	"github.com/jirafs/jirafs/internal/plan"
	"github.com/jirafs/jirafs/internal/schema"
	"github.com/jirafs/jirafs/internal/sync"
)

var (
	pushStdout        io.Writer = os.Stdout
	pushStderr        io.Writer = os.Stderr
	pushClientFactory           = newJiraClient
	pushNow                     = time.Now
)

func printPushHelp() {
	PrintUsage(pushStdout, Usage{
		Synopsis: []string{"push [KEY|file...] [flags]"},
		Description: []string{
			"Writes local issue files to Jira. A draft file (state: draft) is created as a",
			"new issue and rewritten as KEY.md; an existing issue (a KEY, or its file) is",
			"updated, including a status change through Jira's transitions.",
			"Without arguments it pushes every locally modified file and every draft, as",
			"listed by `jirafs status`. Files whose remote copy changed since the last",
			"pull are skipped and reported as conflicts.",
			"Without --apply (or with --dry-run) it only prints what would be written.",
			"Every issue is reported as ok, skipped or failed; one failure never stops",
			"the others, and the exit code is 1 when any issue failed.",
		},
		Sections: []UsageSection{{Title: "Flags", Rows: [][2]string{
			{"--apply", "perform the Jira writes (without it, push is a dry run)"},
			{"--dry-run", "print what would be written and write nothing (the default)"},
			{"--write-local", "after a successful update, write the refreshed remote to the local file"},
			{"--project NAME", "name of a configured project"},
			{"--cwd DIR", "working directory to resolve the project from"},
			{"--help, -h", "show this help message"},
		}}},
		Footer: `Run "jirafs push --help" for more information about flags.`,
	})
}

// pushOptions are the mutation flags of one `jirafs push` run. Jira is only
// written when apply is set; otherwise the run is a dry run.
type pushOptions struct {
	apply      bool
	writeLocal bool
	project    string
	cwd        string
}

// Result states of one pushed issue.
const (
	pushOK      = "ok"
	pushSkipped = "skipped (conflict)"
	pushFailed  = "failed"
)

// pushResult is the outcome of one target: a line of the final summary.
type pushResult struct {
	name   string
	status string
	reason string
}

// pushRun carries the state shared by all targets of one invocation.
type pushRun struct {
	cmdCtx   stdcontext.Context
	settings *config.Settings
	ctx      *context.Context
	opts     pushOptions
	mirror   mirror.Mirror
	client   jira.Client
	cerr     error
	madeCli  bool
}

// jiraClient creates the Jira client on first use.
func (r *pushRun) jiraClient() (jira.Client, error) {
	if !r.madeCli {
		r.madeCli = true
		r.client, r.cerr = pushClientFactory(r.settings, r.ctx, r.opts.cwd)
	}
	return r.client, r.cerr
}

// fail prints msg as an error and returns the failed result for name. The
// first line of msg is the reason shown in the summary.
func pushFail(name, msg string) pushResult {
	if !strings.HasSuffix(msg, "\n") {
		msg += "\n"
	}
	fmt.Fprint(pushStderr, "jirafs push: "+msg)
	first, _, _ := strings.Cut(msg, "\n")
	return pushResult{name, pushFailed, first}
}

// RunPush implements `jirafs push [KEY|file...] [--apply]`.
func RunPush(args []string) int {
	cmdCtx, stop := commandContext()
	defer stop()
	if wantsHelpOrWord(args) {
		printPushHelp()
		return 0
	}
	fs := quietFlagSet("push")
	applyFlag := fs.Bool("apply", false, "perform the Jira writes")
	dryRunFlag := fs.Bool("dry-run", false, "show what would be written (the default)")
	writeLocalFlag := fs.Bool("write-local", false, "after a successful update, write the refreshed remote to the local file")
	projectFlag := fs.String("project", "", "name of a configured project")
	cwdFlag := fs.String("cwd", "", "working directory to resolve the project from")
	if err := parseFlags(fs, args); err != nil {
		return flagError(pushStderr, "jirafs push", err)
	}
	if dryRunConflicts(pushStderr, "jirafs push", *dryRunFlag, *applyFlag) {
		return 2
	}

	settings, ok := loadSettingsOrHint(pushStderr, "push")
	if !ok {
		return 1
	}
	cwd := *cwdFlag
	if cwd == "" {
		var err error
		if cwd, err = os.Getwd(); err != nil {
			fmt.Fprintf(pushStderr, "jirafs push: %v\n", err)
			return 1
		}
	}
	ctx, ok := resolvePushContext(settings, *projectFlag, cwd)
	if !ok {
		fmt.Fprintln(pushStderr, `run "jirafs init <name> ..." to configure a project`)
		return 1
	}
	run := &pushRun{
		cmdCtx:   cmdCtx,
		settings: settings,
		ctx:      ctx,
		opts:     pushOptions{apply: *applyFlag, writeLocal: *writeLocalFlag, project: *projectFlag, cwd: *cwdFlag},
	}
	run.opts.cwd = cwd
	if m, err := readMirrorFile(filepath.Join(ctx.MirrorDir, "mirror.yml")); err == nil {
		run.mirror = *m
	}

	targets := fs.Args()
	var results []pushResult
	if len(targets) == 0 {
		changes := collectLocalChanges(settings.Projects[ctx.Name].LocalDirs, run.mirror)
		if len(changes) == 0 {
			fmt.Fprintln(pushStdout, "jirafs push: nothing to push: no modified or draft files")
			return 0
		}
		for _, c := range changes {
			if c.State == ChangeConflicted {
				results = append(results, conflictResult(c.Key))
				continue
			}
			label := c.Key
			if c.State == ChangeDraft {
				label = c.Path
			}
			results = append(results, run.pushTarget(c.Path, label))
		}
	} else {
		for _, t := range targets {
			results = append(results, run.pushTarget(t, ""))
		}
	}

	failed := printPushSummary(results)
	if !run.opts.apply {
		dryRunDone(pushStdout, run.realCommand(targets, *cwdFlag))
	}
	if failed > 0 {
		return 1
	}
	return 0
}

// conflictResult is the result of a file whose remote copy moved on.
func conflictResult(key string) pushResult {
	return pushResult{key, pushSkipped, "remote changed since the last pull; review it: jirafs diff " + key}
}

// printPushSummary prints one line per target and a closing count, and
// returns the number of failed targets.
func printPushSummary(results []pushResult) int {
	var ok, skipped, failed int
	fmt.Fprintln(pushStdout, "jirafs push: summary:")
	for _, r := range results {
		switch r.status {
		case pushOK:
			ok++
		case pushSkipped:
			skipped++
		default:
			failed++
		}
		line := fmt.Sprintf("  %s  %s", r.name, r.status)
		if r.reason != "" {
			line += "  " + r.reason
		}
		fmt.Fprintln(pushStdout, line)
	}
	fmt.Fprintf(pushStdout, "jirafs push: %d ok, %d skipped, %d failed\n", ok, skipped, failed)
	return failed
}

// realCommand builds the exact command that performs the run for real.
func (r *pushRun) realCommand(targets []string, cwdFlag string) string {
	return pushApplyHint(targets, r.opts.project, cwdFlag, r.opts.writeLocal)
}

// pushApplyHint builds the exact `jirafs push --apply` command for targets
// as given, preserving the flags needed to resolve the same project.
func pushApplyHint(targets []string, project, cwd string, writeLocal bool) string {
	parts := []string{"jirafs", "push"}
	for _, t := range targets {
		parts = append(parts, shellQuote(t))
	}
	parts = append(parts, "--apply")
	if writeLocal {
		parts = append(parts, "--write-local")
	}
	if project != "" {
		parts = append(parts, "--project", shellQuote(project))
	}
	if cwd != "" {
		parts = append(parts, "--cwd", shellQuote(cwd))
	}
	return strings.Join(parts, " ")
}

// resolvePushContext resolves the project context for the push command.
func resolvePushContext(settings *config.Settings, project, cwd string) (*context.Context, bool) {
	ctx, err := context.NewResolver(settings, project).Resolve(cwd)
	if err != nil {
		var ce *context.Error
		if context.IsContextError(err, &ce) && ce.Code == config.ErrNoProjectResolved {
			fmt.Fprintf(pushStderr, "jirafs push: no project resolved for cwd %q\n", cwd)
			if len(ce.Candidates) > 0 {
				fmt.Fprintln(pushStderr, "Available projects:")
				for _, name := range ce.Candidates {
					fmt.Fprintf(pushStderr, "  - %s\n", name)
				}
			}
			return nil, false
		}
		fmt.Fprintf(pushStderr, "jirafs push: %v\n", err)
		return nil, false
	}
	return ctx, true
}

// looksLikePath reports whether a push target names a file rather than an
// issue key.
func looksLikePath(arg string) bool {
	return strings.HasSuffix(arg, ".md") || strings.ContainsRune(arg, filepath.Separator)
}

// pushTarget resolves one KEY or file (arg), then creates or updates it.
// label overrides the name used in the summary when non-empty.
func (r *pushRun) pushTarget(arg, label string) pushResult {
	name := arg
	if label != "" {
		name = label
	}
	var issue schema.Issue
	var path string
	if looksLikePath(arg) {
		path = arg
		data, err := os.ReadFile(path)
		if err != nil {
			return pushFail(name, fmt.Sprintf("cannot read %s: %v", path, err))
		}
		var pe *schema.ParseError
		if issue, pe = schema.ParseIssue(string(data)); pe != nil {
			return pushFail(name, fmt.Sprintf("invalid document %s: %v", path, pe))
		}
	} else {
		var err error
		if issue, path, err = readLocalIssue(r.ctx, r.settings, arg); err != nil {
			return pushFail(name, err.Error())
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return pushFail(name, fmt.Sprintf("cannot read %s: %v", path, err))
	}
	doc := string(data)

	switch {
	case issue.RemoteMetadata.StateFile == "draft":
		return r.createDraft(path, doc)
	case issue.RemoteMetadata.State() == schema.StateArchived:
		return pushFail(name, fmt.Sprintf("%s is archived; there is nothing to push", path))
	case issue.Identity.Key == "":
		return pushFail(name, fmt.Sprintf("%s has no issue key and is not a draft, so there is nothing to push", path))
	}
	key := string(issue.Identity.Key)
	if synced := schema.SyncedHash(doc); (synced == "" || synced != schema.ContentHash(doc)) &&
		remoteAhead(r.mirror.RemoteUpdatedFor(issue.Identity.Key), issue.Context.Updated) {
		return conflictResult(key)
	}
	return r.updateIssue(key, issue, path)
}

// createDraft creates the draft at path as a new Jira issue.
func (r *pushRun) createDraft(path, doc string) pushResult {
	name := path
	issue, pe := codec.ParseIssue(doc)
	if pe != nil {
		return pushFail(name, fmt.Sprintf("invalid document: %v", pe))
	}
	if issue.Identity.Key != "" {
		return pushFail(name, fmt.Sprintf("%s is a draft but already has issue key %s; to update that issue set state: synced in the file, then run: jirafs push %s", path, issue.Identity.Key, issue.Identity.Key))
	}
	if msg := validateFilled(issue); msg != "" {
		return pushFail(name, "invalid draft: "+msg)
	}
	ctx, settings, opts := r.ctx, r.settings, r.opts
	if opts.project != "" || issue.Identity.Project.IsZero() || issue.Identity.Project.Value == "PROJECT" {
		issue.Identity.Project = schema.TypedRef{Type: "project", Value: ctx.Key}
	}
	baseURL := settings.Instances[ctx.Instance].BaseURL
	defaults, derr := schema.NormalizeCustomFields(settings.Projects[ctx.Name].CreateDefaults)
	if derr != nil {
		return pushFail(name, fmt.Sprintf("invalid create_defaults for project %s: %v", ctx.Name, derr))
	}
	issue.CustomFields = schema.MergeCustomFields(defaults, issue.CustomFields)

	createFields := jira.CreateFields(issue)
	payload, _ := json.MarshalIndent(map[string]interface{}{"fields": createFields}, "  ", "  ")

	// Ask Jira which fields are required on create and report the ones the
	// payload lacks. A failure to ask is a warning, never fatal.
	var missing []jira.CreateMetaField
	client, cerr := r.jiraClient()
	if cerr != nil {
		if opts.apply {
			return pushFail(name, fmt.Sprintf("cannot create Jira client: %v", cerr))
		}
		fmt.Fprintf(pushStderr, "jirafs push: warning: cannot check required fields (no Jira client: %v)\n", cerr)
	} else {
		typeName := jira.IssueTypeName(string(issue.Identity.Type))
		meta, merr := client.FetchCreateMeta(stdcontext.Background(), issue.Identity.Project.Value, typeName)
		if merr != nil {
			fmt.Fprintf(pushStderr, "jirafs push: warning: cannot check required fields: %v\n", merr)
		} else {
			missing = missingRequired(meta, createFields)
		}
	}
	if len(missing) > 0 {
		msg := formatMissingRequired(missing, ctx.Name, path)
		if opts.apply {
			return pushFail(name, "refusing to create: Jira requires fields this draft does not set\n"+msg)
		}
		fmt.Fprint(pushStdout, "jirafs push: warning: Jira requires fields this draft does not set\n"+msg)
	}
	if !opts.apply {
		fmt.Fprintf(pushStdout, "jirafs push: would create %s in project %s: %s\n", jira.IssueTypeName(string(issue.Identity.Type)), issue.Identity.Project.Value, issue.Summary)
		fmt.Fprintf(pushStdout, "  POST %s/rest/api/2/issue\n  %s\n", baseURL, payload)
		return pushResult{name, pushOK, "would create " + jira.IssueTypeName(string(issue.Identity.Type))}
	}

	key, err := client.CreateIssue(r.cmdCtx, issue)
	if err != nil {
		return pushFail(name, fmt.Sprintf("cannot create issue: %v", err))
	}

	issue.Identity.Key = schema.IssueKey(key)
	issue.RemoteMetadata.StateFile = "synced"
	issue.RemoteMetadata.SyncTime = pushNow().UTC()
	newPath := filepath.Join(filepath.Dir(path), key+".md")
	if err := writeSyncedIssue(newPath, ctx.MirrorDir, issue); err != nil {
		return pushFail(key, fmt.Sprintf("created %s in Jira (%s/browse/%s) but cannot write %s: %v", key, baseURL, key, newPath, err))
	}
	if newPath != path {
		if err := os.Remove(path); err != nil {
			fmt.Fprintf(pushStderr, "jirafs push: created %s; cannot remove old draft %s: %v\n", key, path, err)
		}
	}
	fmt.Fprintf(pushStdout, "jirafs: created %s\n  file: %s\n  url:  %s/browse/%s\n", key, newPath, baseURL, key)
	return pushResult{key, pushOK, "created from " + path}
}

// updateIssue fetches the remote issue key, builds and validates a plan from
// the local issue, and (with --apply) pushes the changes to Jira, including
// a status transition.
func (r *pushRun) updateIssue(key string, localIssue schema.Issue, localPath string) pushResult {
	client, err := r.jiraClient()
	if err != nil {
		return pushFail(key, fmt.Sprintf("cannot create Jira client: %v", err))
	}
	remote, err := client.FetchIssue(r.cmdCtx, key)
	if err != nil {
		return pushFail(key, err.Error())
	}

	// The local status is the frontmatter `status:` value (parsed into
	// Context). A missing status means "no change", never a status edit.
	localIssue.Status = localIssue.Context.Status
	if localIssue.Status == "" {
		localIssue.Status = remote.Status
	}
	if remote.Status == "" && localIssue.Context.Status != "" {
		fmt.Fprintf(pushStderr, "jirafs push: warning: remote status of %s is unknown; not changing status\n", key)
	}

	ops, _, err := plan.BuildPlan(localIssue, *remote)
	if err != nil {
		return pushFail(key, err.Error())
	}
	if len(ops) == 0 {
		fmt.Fprintf(pushStdout, "jirafs push: issue %s in %s: no changes needed\n", key, localPath)
		return pushResult{key, pushOK, "no changes needed"}
	}

	// Validate and apply the plan through the sync service. A temp directory
	// stands in for the archive path to avoid validation errors.
	result := sync.Sync(localIssue, *remote, ops, os.TempDir())
	if len(result.Conflicts) > 0 {
		fmt.Fprintln(pushStdout, "jirafs push: conflicts detected, no changes applied:")
		for _, c := range result.Conflicts {
			fmt.Fprintf(pushStdout, "  %s: %s (local=%q, remote=%q)\n",
				c.Type, c.Field, c.LocalValue, c.RemoteValue)
		}
		return pushResult{key, pushSkipped, "local and remote changed the same fields; review it: jirafs diff " + key}
	}

	sort.Slice(ops, func(i, j int) bool {
		return ops[i].Field < ops[j].Field
	})

	// A status change needs the transitions API. Resolve it before anything
	// is written so a missing transition fails without a partial update.
	var statusTr *jira.Transition
	var transitionFields map[string]interface{}
	for _, op := range ops {
		if op.Field != schema.EditableFieldStatus {
			continue
		}
		transitions, err := client.FetchTransitions(r.cmdCtx, key)
		if err != nil {
			return pushFail(key, fmt.Sprintf("cannot fetch transitions for %s: %v", key, err))
		}
		statusTr = findTransition(transitions, op.Value)
		if statusTr == nil {
			return pushFail(key, noTransitionMessage(key, remote.Status, op.Value, transitions, localPath))
		}
		var missing []jira.RequiredField
		for _, rf := range statusTr.RequiredFields() {
			if v, ok := localIssue.CustomFields[rf.ID]; ok && v != nil && v != "" {
				if transitionFields == nil {
					transitionFields = map[string]interface{}{}
				}
				transitionFields[rf.ID] = v
			} else {
				missing = append(missing, rf)
			}
		}
		if len(missing) > 0 {
			return pushFail(key, gatedTransitionMessage(key, statusTr, missing, localPath))
		}
	}

	fmt.Fprintf(pushStdout, "jirafs push: %d operation(s) for issue %s in %s:\n", len(ops), key, localPath)
	for _, op := range ops {
		if op.Field == schema.EditableFieldStatus && statusTr != nil {
			fmt.Fprintf(pushStdout, "  transition status: %s \u2192 %s (via %q)\n",
				remote.Status, statusTr.To.Name, statusTr.Name)
			continue
		}
		fmt.Fprintf(pushStdout, "  %s %s = %s\n", op.Type, op.Field, op.Value)
	}

	if !r.opts.apply {
		return pushResult{key, pushOK, fmt.Sprintf("would apply %d operation(s)", len(ops))}
	}

	updatedRemote, err := client.UpdateIssue(r.cmdCtx, key, result.Remote)
	if err != nil {
		return pushFail(key, fmt.Sprintf("cannot update remote issue: %v", err))
	}
	if statusTr != nil {
		if err := client.DoTransition(r.cmdCtx, key, statusTr.ID, transitionFields); err != nil {
			return pushFail(key, fmt.Sprintf("fields updated, but status transition %q failed (the field update was already applied): %v", statusTr.Name, err))
		}
	}

	result.Remote.RemoteMetadata = updatedRemote.RemoteMetadata
	if r.opts.writeLocal {
		if err := writeSyncedIssue(localPath, r.ctx.MirrorDir, result.Remote); err != nil {
			return pushFail(key, fmt.Sprintf("cannot write updated local issue: %v", err))
		}
	}

	fmt.Fprintf(pushStdout, "jirafs push: applied %d operation(s) successfully\n", len(ops))
	return pushResult{key, pushOK, fmt.Sprintf("applied %d operation(s)", len(ops))}
}

// findTransition returns the transition whose target status name matches
// target case-insensitively, or nil.
func findTransition(ts []jira.Transition, target string) *jira.Transition {
	for i := range ts {
		if strings.EqualFold(ts[i].To.Name, target) {
			return &ts[i]
		}
	}
	return nil
}

// noTransitionMessage explains that no transition leads to target and lists
// the reachable statuses with a copy-pasteable next step.
func noTransitionMessage(key, from, target string, ts []jira.Transition, localPath string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "cannot change status of %s from %q to %q: no available Jira transition leads to it\n", key, from, target)
	if len(ts) == 0 {
		b.WriteString("available target statuses: (none)\n")
	} else {
		b.WriteString("available target statuses:\n")
		for _, t := range ts {
			fmt.Fprintf(&b, "  %s (via %q)\n", t.To.Name, t.Name)
		}
	}
	fmt.Fprintf(&b, "fix: set status in %s to one of the above, or revert it: jirafs push %s --dry-run\n", localPath, key)
	return b.String()
}

// gatedTransitionMessage explains that the transition needs screen fields
// the issue has no value for, with a copy-pasteable next step.
func gatedTransitionMessage(key string, tr *jira.Transition, missing []jira.RequiredField, localPath string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "transition %q (to %q) on %s requires fields with no value; nothing was written\n", tr.Name, tr.To.Name, key)
	b.WriteString("required fields:\n")
	for _, f := range missing {
		fmt.Fprintf(&b, "  %s (%s)\n", f.ID, f.Name)
	}
	fmt.Fprintf(&b, "fix: add the value(s) under custom_fields in %s, then run: jirafs push %s --dry-run\n", localPath, key)
	return b.String()
}

// skipRequired lists required fields that are never reported as missing:
// project and issuetype are always sent, and Jira fills in the reporter.
var skipRequired = map[string]bool{"project": true, "issuetype": true, "reporter": true}

// missingRequired returns the required, default-less create-meta fields that
// are absent (or empty) in the create payload.
func missingRequired(meta []jira.CreateMetaField, fields map[string]interface{}) []jira.CreateMetaField {
	var out []jira.CreateMetaField
	for _, f := range meta {
		if !f.Required || f.HasDefault || skipRequired[f.ID] {
			continue
		}
		if v, ok := fields[f.ID]; ok && v != nil && v != "" {
			continue
		}
		out = append(out, f)
	}
	return out
}

const maxAllowedShown = 10

// formatMissingRequired renders the missing fields with their allowed values
// and copy-pasteable snippets for custom_fields and create_defaults.
func formatMissingRequired(missing []jira.CreateMetaField, project, path string) string {
	var sb strings.Builder
	var draftSnip, tomlSnip strings.Builder
	for _, f := range missing {
		fmt.Fprintf(&sb, "  missing required field: %s (%s)\n", f.Name, f.ID)
		if n := len(f.AllowedValues); n > 0 {
			shown := f.AllowedValues
			if n > maxAllowedShown {
				shown = shown[:maxAllowedShown]
			}
			labels := make([]string, len(shown))
			for i, a := range shown {
				labels[i] = a.Label()
				if a.ID != "" && a.ID != a.Label() {
					labels[i] += " [id " + a.ID + "]"
				}
			}
			more := ""
			if n > len(shown) {
				more = fmt.Sprintf(", ... (%d more)", n-len(shown))
			}
			fmt.Fprintf(&sb, "    allowed values: %s%s\n", strings.Join(labels, ", "), more)
		}
		if !schema.IsCustomFieldKey(f.ID) {
			fmt.Fprintf(&sb, "    %s is a standard field that jirafs cannot set from a draft yet\n", f.ID)
			continue
		}
		yamlVal, tomlVal := `"TODO"`, `"TODO"`
		if len(f.AllowedValues) > 0 {
			a := f.AllowedValues[0]
			switch {
			case a.Value != "":
				yamlVal, tomlVal = fmt.Sprintf("{value: %q}", a.Value), fmt.Sprintf("{ value = %q }", a.Value)
			case a.ID != "":
				yamlVal, tomlVal = fmt.Sprintf("{id: %q}", a.ID), fmt.Sprintf("{ id = %q }", a.ID)
			default:
				yamlVal, tomlVal = fmt.Sprintf("{name: %q}", a.Name), fmt.Sprintf("{ name = %q }", a.Name)
			}
		}
		fmt.Fprintf(&draftSnip, "    %s: %s\n", f.ID, yamlVal)
		fmt.Fprintf(&tomlSnip, "    %s = %s\n", f.ID, tomlVal)
	}
	if draftSnip.Len() > 0 {
		fmt.Fprintf(&sb, "  add to the frontmatter of %s:\n    custom_fields:\n%s", path, strings.ReplaceAll(draftSnip.String(), "    customfield", "      customfield"))
		fmt.Fprintf(&sb, "  or to ~/.jirafs/settings.toml for every draft in the project:\n    [projects.%s.create_defaults]\n%s", project, tomlSnip.String())
	}
	return sb.String()
}
