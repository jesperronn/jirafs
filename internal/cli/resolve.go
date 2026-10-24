package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/jirafs/jirafs/internal/context"
	"github.com/jirafs/jirafs/internal/merge"
	"github.com/jirafs/jirafs/internal/mirror"
	"github.com/jirafs/jirafs/internal/schema"
	"gopkg.in/yaml.v3"
)

var (
	resolveStdout io.Writer = os.Stdout
	resolveStderr io.Writer = os.Stderr
	resolveNow              = time.Now
)

// docConflicts returns the entries of the `conflicts:` frontmatter key of
// doc; nil when the key is absent, empty or unreadable.
func docConflicts(doc string) []conflictEntry {
	front, ok := schema.FrontmatterText(doc)
	if !ok {
		return nil
	}
	var parsed struct {
		Conflicts []conflictEntry `yaml:"conflicts"`
	}
	if err := yaml.Unmarshal([]byte(front), &parsed); err != nil {
		return nil
	}
	return parsed.Conflicts
}

// conflictFieldNames returns the field names of entries, in order.
func conflictFieldNames(entries []conflictEntry) []string {
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Field
	}
	return names
}

// conflictEntriesBlock renders entries as the `conflicts:` frontmatter key.
func conflictEntriesBlock(entries []conflictEntry) string {
	out, _ := yaml.Marshal(map[string][]conflictEntry{"conflicts": entries}) // plain strings always marshal
	return string(out)
}

// applyRemoteValue sets the field of issue named by e to the remote value
// recorded in e. Keys of a custom_fields conflict that were set in the base
// but are absent from the remote object are removed.
func applyRemoteValue(issue *schema.Issue, e conflictEntry) error {
	v := e.Remote
	switch schema.EditableField(e.Field) {
	case schema.EditableFieldSummary:
		issue.Summary = v
	case schema.EditableFieldDescription:
		issue.Description = v
		if issue.Sections == nil {
			issue.Sections = map[schema.FixedSectionName]string{}
		}
		if v == "" {
			delete(issue.Sections, schema.SecDescription)
		} else {
			issue.Sections[schema.SecDescription] = v
		}
	case schema.EditableFieldLabels:
		issue.Labels = splitList(v)
	case schema.EditableFieldFixVersions:
		issue.FixVersions = splitList(v)
	case schema.EditableFieldAssignee:
		issue.Assignee = nil
		if v != "" {
			issue.Assignee = &v
		}
	case schema.EditableFieldSprint:
		issue.Sprint = v
	case schema.EditableFieldStatus:
		issue.Status, issue.Context.Status = v, v
	case schema.EditableFieldPriority:
		issue.Context.Priority = v
	case merge.FieldLinkedIssues:
		issue.LinkedIssues = nil
		for _, line := range splitLines(v) {
			p := strings.SplitN(line, "|", 4)
			for len(p) < 4 {
				p = append(p, "")
			}
			issue.LinkedIssues = append(issue.LinkedIssues, schema.LinkedIssue{Key: schema.IssueKey(p[0]), Relation: p[1], LinkType: p[2], Direction: p[3]})
		}
	case schema.EditableFieldCustomFields:
		return applyRemoteCustomFields(issue, e)
	default:
		return fmt.Errorf("cannot resolve unknown field %q", e.Field)
	}
	return nil
}

func splitList(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func applyRemoteCustomFields(issue *schema.Issue, e conflictEntry) error {
	var remote, base map[string]interface{}
	if err := json.Unmarshal([]byte(e.Remote), &remote); err != nil {
		return fmt.Errorf("custom_fields conflict has an unreadable remote value: %v", err)
	}
	if e.Base != "" {
		if err := json.Unmarshal([]byte(e.Base), &base); err != nil {
			return fmt.Errorf("custom_fields conflict has an unreadable base value: %v", err)
		}
	}
	if issue.CustomFields == nil {
		issue.CustomFields = map[string]interface{}{}
	}
	for k := range base {
		if _, ok := remote[k]; !ok {
			delete(issue.CustomFields, k)
		}
	}
	for k, v := range remote {
		issue.CustomFields[k] = v
	}
	if len(issue.CustomFields) == 0 {
		issue.CustomFields = nil
	}
	return nil
}

// resolvePlan is what resolving one issue will do.
type resolvePlan struct {
	key       schema.IssueKey
	path      string
	doc       string
	local     schema.Issue
	keepLocal bool
	resolved  []conflictEntry
	remaining []conflictEntry
}

// newResolvePlan selects the conflicts to resolve: all of them, or those
// named in fields.
func newResolvePlan(key schema.IssueKey, path, doc string, local schema.Issue, keepLocal bool, fields []string) (*resolvePlan, error) {
	all := docConflicts(doc)
	if len(all) == 0 {
		return nil, fmt.Errorf("%s has no recorded conflicts; nothing to resolve (see: jirafs status)", key)
	}
	p := &resolvePlan{key: key, path: path, doc: doc, local: local, keepLocal: keepLocal}
	want := map[string]bool{}
	for _, f := range fields {
		want[f] = true
	}
	for _, e := range all {
		if len(fields) == 0 || want[e.Field] {
			p.resolved = append(p.resolved, e)
			delete(want, e.Field)
		} else {
			p.remaining = append(p.remaining, e)
		}
	}
	if len(want) > 0 {
		unknown := make([]string, 0, len(want))
		for f := range want {
			unknown = append(unknown, f)
		}
		sort.Strings(unknown)
		return nil, fmt.Errorf("%s has no conflict on field %s; conflicted fields: %s", key, strings.Join(unknown, ", "), strings.Join(conflictFieldNames(all), ", "))
	}
	return p, nil
}

// newDoc returns the file content after resolution.
func (p *resolvePlan) newDoc() (string, error) {
	doc := p.doc
	if !p.keepLocal {
		issue := p.local
		issue.Sections = copySections(issue.Sections)
		for _, e := range p.resolved {
			if err := applyRemoteValue(&issue, e); err != nil {
				return "", err
			}
		}
		doc = schema.RenderIssue(issue)
		if synced := schema.SyncedHash(p.doc); synced != "" {
			doc = schema.StampSyncedHashAs(doc, synced)
		}
	} else {
		doc = schema.RemoveFrontmatterKey(doc, "conflicts")
	}
	if len(p.remaining) > 0 {
		doc = schema.AppendFrontmatter(doc, conflictEntriesBlock(p.remaining))
	}
	return doc, nil
}

func copySections(in map[schema.FixedSectionName]string) map[schema.FixedSectionName]string {
	out := make(map[schema.FixedSectionName]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// updatedBase returns the base snapshot to store after resolution: the
// resolved fields take the remote value, so a kept local value is a plain
// local edit that `push` may send and a taken remote value is in sync. Without
// a stored base one is built from the local issue with every recorded
// conflict field set to its remote value.
func (p *resolvePlan) updatedBase(mirrorDir string) (*schema.Issue, error) {
	base, ok := mirror.LoadBase(mirrorDir, p.key)
	entries := p.resolved
	if !ok {
		clone := p.local
		clone.Sections = copySections(clone.Sections)
		base = &clone
		entries = append(append([]conflictEntry(nil), p.resolved...), p.remaining...)
	}
	for _, e := range entries {
		if err := applyRemoteValue(base, e); err != nil {
			return nil, err
		}
	}
	return base, nil
}

// apply writes the resolution: a backup of the local file when its values are
// replaced, the file itself and the base snapshot. It returns the backup path.
func (p *resolvePlan) apply(mirrorDir string, now time.Time) (string, error) {
	doc, err := p.newDoc()
	if err != nil {
		return "", err
	}
	base, err := p.updatedBase(mirrorDir)
	if err != nil {
		return "", err
	}
	backup := ""
	if !p.keepLocal {
		if backup, err = mirror.SaveBackup(mirrorDir, p.key, p.doc, now); err != nil {
			return "", fmt.Errorf("save backup: %w", err)
		}
	}
	if err := os.WriteFile(p.path, []byte(doc), 0o644); err != nil {
		return backup, err
	}
	return backup, mirror.SaveBase(mirrorDir, base)
}

// RunResolve implements `jirafs resolve KEY --keep local|remote [--field NAME] [--apply]`.
//
// In a git-backed mirror resolve does not commit: it only edits a local file
// the user is still working on, exactly like editing the file by hand, and
// `push --apply` commits it once it has been written to Jira.
func RunResolve(args []string) int {
	if wantsHelpOrWord(args) {
		PrintCommandHelp(resolveStdout, "resolve")
		return 0
	}
	fs := quietFlagSet("resolve")
	keep := fs.String("keep", "", "which value to keep: local or remote")
	field := fs.String("field", "", "resolve only these conflicted fields (comma-separated)")
	applyFlag := fs.Bool("apply", false, "write the resolution")
	dryRunFlag := fs.Bool("dry-run", false, "show what would be written (the default)")
	scope := addScopeFlags(fs)
	if err := parseFlags(fs, args); err != nil {
		return flagError(resolveStderr, "jirafs resolve", err)
	}
	if dryRunConflicts(resolveStderr, "jirafs resolve", *dryRunFlag, *applyFlag) {
		return 2
	}
	if fs.NArg() != 1 {
		return usageError(resolveStderr, "jirafs resolve", "expected exactly one issue key, got %d", fs.NArg())
	}
	if *keep != "local" && *keep != "remote" {
		return usageError(resolveStderr, "jirafs resolve", "--keep must be local or remote (got %q); e.g. jirafs resolve %s --keep local", *keep, fs.Arg(0))
	}
	settings, ok := loadSettingsOrHint(resolveStderr, "resolve")
	if !ok {
		return 1
	}
	cwd, err := scope.workdir()
	if err != nil {
		fmt.Fprintf(resolveStderr, "jirafs resolve: cannot determine working directory: %v\n", err)
		return 1
	}
	ctx, err := context.NewResolver(settings, *scope.project).Resolve(cwd)
	if err != nil {
		fmt.Fprintf(resolveStderr, "jirafs resolve: %v\n", err)
		return 1
	}
	key := fs.Arg(0)
	local, path, err := readLocalIssue(ctx, settings, key)
	if err != nil {
		fmt.Fprintf(resolveStderr, "jirafs resolve: %v\n", err)
		return 1
	}
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(resolveStderr, "jirafs resolve: cannot read %s: %v\n", path, err)
		return 1
	}
	var fields []string
	if *field != "" {
		fields = strings.Split(*field, ",")
	}
	plan, err := newResolvePlan(schema.IssueKey(key), path, string(data), local, *keep == "local", fields)
	if err != nil {
		fmt.Fprintf(resolveStderr, "jirafs resolve: %v\n", err)
		return 1
	}

	printResolvePlan(plan, *applyFlag)
	if *applyFlag {
		backup, err := plan.apply(ctx.MirrorDir, resolveNow())
		if backup != "" {
			fmt.Fprintf(resolveStdout, "jirafs resolve: local file saved to %s\n", backup)
		}
		if err != nil {
			fmt.Fprintf(resolveStderr, "jirafs resolve: %s: %v\n", key, err)
			return 1
		}
		fmt.Fprintf(resolveStdout, "jirafs resolve: wrote %s\n", path)
		printResolveNext(plan)
		return 0
	}
	dryRunDone(resolveStdout, resolveApplyHint(key, *keep, *field, *scope.project, *scope.cwd))
	return 0
}

// printResolvePlan describes, per field, what resolving does.
func printResolvePlan(p *resolvePlan, applied bool) {
	verb := "would resolve"
	if applied {
		verb = "resolving"
	}
	side := "remote"
	if p.keepLocal {
		side = "local"
	}
	fmt.Fprintf(resolveStdout, "jirafs resolve: %s %s: keep %s for %s\n", verb, p.key, side, strings.Join(conflictFieldNames(p.resolved), ", "))
	for _, e := range p.resolved {
		if p.keepLocal {
			fmt.Fprintf(resolveStdout, "  %s: keep the local value; drop the recorded remote value %q\n", e.Field, e.Remote)
		} else {
			fmt.Fprintf(resolveStdout, "  %s: replace the local value with the remote value %q (local file is saved to .jirafs/backup first)\n", e.Field, e.Remote)
		}
	}
}

// printResolveNext says what is left to do after an applied resolution.
func printResolveNext(p *resolvePlan) {
	if len(p.remaining) > 0 {
		fmt.Fprintf(resolveStdout, "jirafs resolve: %d conflict(s) left on %s: %s; run: jirafs resolve %s --keep local|remote --field NAME\n",
			len(p.remaining), p.key, strings.Join(conflictFieldNames(p.remaining), ", "), p.key)
		return
	}
	fmt.Fprintf(resolveStdout, "jirafs resolve: no conflicts left on %s; review it: jirafs diff %s, then run: jirafs push %s --apply\n", p.key, p.key, p.key)
}

// resolveApplyHint builds the exact `jirafs resolve --apply` command.
func resolveApplyHint(key, keep, field, project, cwd string) string {
	parts := []string{"jirafs", "resolve", shellQuote(key), "--keep", keep}
	if field != "" {
		parts = append(parts, "--field", shellQuote(field))
	}
	parts = append(parts, "--apply")
	if project != "" {
		parts = append(parts, "--project", shellQuote(project))
	}
	if cwd != "" {
		parts = append(parts, "--cwd", shellQuote(cwd))
	}
	return strings.Join(parts, " ")
}
