package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jirafs/jirafs/internal/codec"
	"github.com/jirafs/jirafs/internal/config"
	"github.com/jirafs/jirafs/internal/jira"
	"github.com/jirafs/jirafs/internal/schema"
)

var (
	pushStdout        io.Writer = os.Stdout
	pushStderr        io.Writer = os.Stderr
	pushClientFactory           = buildSyncClient
	pushNow                     = time.Now
)

func printPushHelp() {
	fmt.Fprint(pushStdout, `jirafs push - create a local draft issue in Jira

Usage:
  jirafs push <draft.md> [--apply] [--project <key>] [--cwd <dir>]

Without --apply, shows what would be created and contacts nothing.
With --apply, creates the issue in Jira, rewrites the file with the new key
(renamed to KEY.md, state no longer draft) and prints the new path and URL.
`)
}

// RunPush implements `jirafs push <draft.md> [--apply]`.
func RunPush(args []string) int {
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
	projectFlag := fs.String("project", "", "project key or name to use")
	cwdFlag := fs.String("cwd", "", "working directory to resolve the project from")
	if err := fs.Parse(flagArgs); err != nil {
		return 1
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
		fmt.Fprintf(pushStderr, "jirafs push: %s is not a draft (it already is a synced issue); to update an existing issue run: jirafs sync %s\n", path, syncHintKey(issue, path))
		return 1
	}
	if msg := validateFilled(issue); msg != "" {
		fmt.Fprintf(pushStderr, "jirafs push: invalid draft: %s\n", msg)
		return 1
	}

	settings, err := config.Load()
	if err != nil {
		fmt.Fprintf(pushStderr, "jirafs push: %v\n", err)
		return 1
	}
	cwd := *cwdFlag
	if cwd == "" {
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

	payload, _ := json.MarshalIndent(map[string]interface{}{"fields": jira.CreateFields(issue)}, "  ", "  ")
	if !*applyFlag {
		fmt.Fprintf(pushStdout, "jirafs push: would create %s in project %s: %s\n", jira.IssueTypeName(string(issue.Identity.Type)), issue.Identity.Project.Value, issue.Summary)
		fmt.Fprintf(pushStdout, "  POST %s/rest/api/2/issue\n  %s\n", baseURL, payload)
		fmt.Fprintf(pushStdout, "dry run: nothing was created; to create it run: jirafs push %s --apply\n", path)
		return 0
	}

	client, err := pushClientFactory(settings, ctx, cwd)
	if err != nil {
		fmt.Fprintf(pushStderr, "jirafs push: cannot create Jira client: %v\n", err)
		return 1
	}
	key, err := client.CreateIssue(nil, issue)
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

// syncHintKey returns the issue key for a hint, falling back to the file stem.
func syncHintKey(issue *schema.Issue, path string) string {
	if issue.Identity.Key != "" {
		return string(issue.Identity.Key)
	}
	return strings.TrimSuffix(filepath.Base(path), ".md")
}
