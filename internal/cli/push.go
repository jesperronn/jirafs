package cli

import (
	stdcontext "context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jirafs/jirafs/internal/codec"
	"github.com/jirafs/jirafs/internal/jira"
	"github.com/jirafs/jirafs/internal/schema"
)

var (
	pushStdout        io.Writer = os.Stdout
	pushStderr        io.Writer = os.Stderr
	pushClientFactory           = newJiraClient
	pushNow                     = time.Now
)

func printPushHelp() {
	fmt.Fprint(pushStdout, `jirafs push - create a local draft issue in Jira

Usage:
  jirafs push <draft.md> [--apply|--dry-run] [--project <key>] [--cwd <dir>]

Without --apply (or with --dry-run), shows what would be created and checks Jira's createmeta
for required fields the draft is missing; nothing is created.
With --apply, first runs the same required-field check (refusing when fields
are missing), then creates the issue in Jira, rewrites the file with the new key
(renamed to KEY.md, state no longer draft) and prints the new path and URL.
`)
}

// RunPush implements `jirafs push <draft.md> [--apply]`.
func RunPush(args []string) int {
	cmdCtx, stop := commandContext()
	defer stop()
	if len(args) > 0 && (args[0] == "help" || args[0] == "--help" || args[0] == "-h") {
		printPushHelp()
		return 0
	}
	var path string
	var flagArgs []string
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		path, flagArgs = args[0], args[1:]
	} else {
		flagArgs = args
	}
	fs := flag.NewFlagSet("push", flag.ContinueOnError)
	fs.SetOutput(pushStderr)
	applyFlag := fs.Bool("apply", false, "create the issue in Jira")
	dryRunFlag := fs.Bool("dry-run", false, "show what would be created (the default)")
	projectFlag := fs.String("project", "", "project key or name to use")
	cwdFlag := fs.String("cwd", "", "working directory to resolve the project from")
	if err := parseFlags(fs, flagArgs); err != nil {
		return 1
	}
	if dryRunConflicts(pushStderr, "jirafs push", *dryRunFlag, *applyFlag) {
		return 2
	}
	if path == "" && fs.NArg() > 0 {
		path = fs.Arg(0)
	}
	if path == "" {
		fmt.Fprintln(pushStderr, "jirafs push: missing draft file; try: jirafs push <draft.md>")
		return 1
	}

	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(pushStderr, "jirafs push: cannot read %s: %v\n", path, err)
		return 1
	}
	issue, pe := codec.ParseIssue(string(data))
	if pe != nil {
		fmt.Fprintf(pushStderr, "jirafs push: invalid document: %v\n", pe)
		return 1
	}
	if issue.RemoteMetadata.StateFile != "draft" || issue.Identity.Key != "" {
		fmt.Fprintf(pushStderr, "jirafs push: %s is not a draft (it already is a synced issue)%s\n", path, syncHint(issue))
		return 1
	}
	if msg := validateFilled(issue); msg != "" {
		fmt.Fprintf(pushStderr, "jirafs push: invalid draft: %s\n", msg)
		return 1
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
	ctx, ok := resolveMirrorContext(settings, *projectFlag, cwd, "push")
	if !ok {
		fmt.Fprintln(pushStderr, `run "jirafs setup ..." to configure a project`)
		return 1
	}
	if *projectFlag != "" || issue.Identity.Project.IsZero() || issue.Identity.Project.Value == "PROJECT" {
		issue.Identity.Project = schema.TypedRef{Type: "project", Value: ctx.Key}
	}
	baseURL := settings.Instances[ctx.Instance].BaseURL
	defaults, derr := schema.NormalizeCustomFields(settings.Projects[ctx.Name].CreateDefaults)
	if derr != nil {
		fmt.Fprintf(pushStderr, "jirafs push: invalid create_defaults for project %s: %v\n", ctx.Name, derr)
		return 1
	}
	issue.CustomFields = schema.MergeCustomFields(defaults, issue.CustomFields)

	createFields := jira.CreateFields(issue)
	payload, _ := json.MarshalIndent(map[string]interface{}{"fields": createFields}, "  ", "  ")

	// Ask Jira which fields are required on create and report the ones the
	// payload lacks. A failure to ask is a warning, never fatal.
	var client jira.Client
	var missing []jira.CreateMetaField
	client, cerr := pushClientFactory(settings, ctx, cwd)
	if cerr != nil {
		if *applyFlag {
			fmt.Fprintf(pushStderr, "jirafs push: cannot create Jira client: %v\n", cerr)
			return 1
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
		if *applyFlag {
			fmt.Fprintf(pushStderr, "jirafs push: refusing to create: Jira requires fields this draft does not set\n%s", msg)
			return 1
		}
		fmt.Fprint(pushStdout, "jirafs push: warning: Jira requires fields this draft does not set\n"+msg)
	}
	if !*applyFlag {
		fmt.Fprintf(pushStdout, "jirafs push: would create %s in project %s: %s\n", jira.IssueTypeName(string(issue.Identity.Type)), issue.Identity.Project.Value, issue.Summary)
		fmt.Fprintf(pushStdout, "  POST %s/rest/api/2/issue\n  %s\n", baseURL, payload)
		fmt.Fprintf(pushStdout, "dry run: nothing was created; to create it run: %s\n", pushApplyHint(path, *projectFlag, *cwdFlag))
		return 0
	}

	key, err := client.CreateIssue(cmdCtx, issue)
	if err != nil {
		fmt.Fprintf(pushStderr, "jirafs push: cannot create issue: %v\n", err)
		return 1
	}

	issue.Identity.Key = schema.IssueKey(key)
	issue.RemoteMetadata.StateFile = "synced"
	issue.RemoteMetadata.SyncTime = pushNow().UTC()
	newPath := filepath.Join(filepath.Dir(path), key+".md")
	if err := os.WriteFile(newPath, []byte(schema.RenderIssue(*issue)), 0o644); err != nil {
		fmt.Fprintf(pushStderr, "jirafs push: created %s in Jira (%s/browse/%s) but cannot write %s: %v\n", key, baseURL, key, newPath, err)
		return 1
	}
	if newPath != path {
		if err := os.Remove(path); err != nil {
			fmt.Fprintf(pushStderr, "jirafs push: created %s; cannot remove old draft %s: %v\n", key, path, err)
		}
	}
	fmt.Fprintf(pushStdout, "jirafs: created %s\n  file: %s\n  url:  %s/browse/%s\n", key, newPath, baseURL, key)
	return 0
}

// syncHint returns a copy-pasteable follow-up for an already-synced file, or
// an empty string when the file carries no issue key to sync.
func syncHint(issue *schema.Issue) string {
	if issue.Identity.Key == "" {
		return "; it has no issue key, so there is nothing to push or sync"
	}
	return "; to update an existing issue run: jirafs sync " + string(issue.Identity.Key)
}

// pushApplyHint builds the exact `jirafs push --apply` command for path as
// given, preserving the flags needed to resolve the same project.
func pushApplyHint(path, project, cwd string) string {
	parts := []string{"jirafs", "push", shellQuote(path), "--apply"}
	if project != "" {
		parts = append(parts, "--project", shellQuote(project))
	}
	if cwd != "" {
		parts = append(parts, "--cwd", shellQuote(cwd))
	}
	return strings.Join(parts, " ")
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
