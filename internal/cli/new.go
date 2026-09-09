package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/jirafs/jirafs/internal/codec"
	"github.com/jirafs/jirafs/internal/config"
	"github.com/jirafs/jirafs/internal/schema"
)

var (
	newStdout io.Writer = os.Stdout
	newStderr io.Writer = os.Stderr
)

// slugRe collapses any run of non-alphanumeric characters into a single hyphen.
var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

// Built-in issue templates. Each is a codec-parseable Markdown document whose
// frontmatter sets the issue type (and a PROJECT placeholder that --project
// overrides at runtime).
const (
	templateStory = `---
type: story
project: "project:PROJECT"
schema_version: "1"
state: draft
summary: New Story
---

## Description

TODO: describe this story.

## Acceptance Criteria

TODO: acceptance criteria.
`

	templateBug = `---
type: bug
project: "project:PROJECT"
schema_version: "1"
state: draft
summary: New Bug
---

## Description

TODO: describe the bug. Where relevant, include:

- **Reproduction:** steps to reproduce the bug.
- **Expected behavior:** what should have happened.
- **Actual behavior:** what actually happened.
- **Impact:** who or what is affected.

## Acceptance Criteria

TODO: acceptance criteria for the fix.
`

	templateEpic = `---
type: epic
project: "project:PROJECT"
schema_version: "1"
state: draft
summary: New Epic
---

## Description

TODO: describe this epic.

- **Goal:** the goal of this epic.
- **Scope:** what is in scope.
- **Non-Goals:** what is out of scope.

## Acceptance Criteria

TODO: how success will be measured.
`
)

// templateFor returns the built-in template named by type, if known.
func templateFor(typeName string) (string, bool) {
	switch typeName {
	case "story":
		return templateStory, true
	case "bug":
		return templateBug, true
	case "epic":
		return templateEpic, true
	default:
		return "", false
	}
}

// printNewHelp writes the `jirafs new` usage to newStdout.
func printNewHelp() {
	fmt.Fprint(newStdout, `jirafs new - create a local draft issue from a template

Usage:
  jirafs new <story|bug|epic> [flags]
  jirafs new --template <path.md> [flags]

The command writes a new draft issue file to the project's local directory.
It does not contact Jira.

Arguments:
  <template>    Built-in template name: story, bug, or epic.

Flags:
  --summary <text>   Set the issue summary.
  --project <key>    Override the project (key or name).
  --assignee <name>  Set the assignee.
  --cwd <dir>        Working directory to resolve the project from.
  --template <path>  Use a user-supplied template file instead of a built-in.
`)
}

// slugify lowercases s, replaces every run of non-alphanumeric characters with
// a single hyphen, trims leading/trailing hyphens, and falls back to
// "new-issue" when the result is empty.
func slugify(s string) string {
	s = slugRe.ReplaceAllString(strings.ToLower(s), "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return "new-issue"
	}
	return s
}

// RunNew implements `jirafs new`. It returns a process exit code.
func RunNew(args []string) int {
	if len(args) == 0 {
		fmt.Fprintf(newStderr, "jirafs new: missing subcommand; use --help\n")
		return 1
	}
	switch args[0] {
	case "help", "--help", "-h":
		printNewHelp()
		return 0
	}

	// The template name is the first non-flag positional. Go's flag package
	// stops at the first non-flag argument, so pull it off before parsing the
	// remaining flags.
	var templateName string
	var flagArgs []string
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		templateName = args[0]
		flagArgs = args[1:]
	} else {
		flagArgs = args
	}

	fs := flag.NewFlagSet("new", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(newStderr, "jirafs new: invalid flags\n")
		fs.PrintDefaults()
	}
	summaryFlag := fs.String("summary", "", "issue summary")
	projectFlag := fs.String("project", "", "project key or name to override")
	assigneeFlag := fs.String("assignee", "", "assignee to set")
	cwdFlag := fs.String("cwd", "", "working directory to resolve the project from")
	templateFlag := fs.String("template", "", "path to a template file")

	if err := fs.Parse(flagArgs); err != nil {
		return 1
	}
	if templateName == "" && len(fs.Args()) > 0 {
		templateName = fs.Args()[0]
	}

	settings, err := config.Load()
	if err != nil {
		fmt.Fprintf(newStderr, "jirafs new: %v\n", err)
		return 1
	}

	cwd := *cwdFlag
	if cwd == "" {
		cwd, err = os.Getwd()
		if err != nil {
			fmt.Fprintf(newStderr, "jirafs new: %v\n", err)
			return 1
		}
	}

	ctx, ok := resolveMirrorContext(settings, *projectFlag, cwd, "new")
	if !ok {
		fmt.Fprintln(newStderr, `run "jirafs setup ..." to configure a project`)
		return 1
	}
	proj, ok := settings.Projects[ctx.Name]
	if !ok || len(proj.LocalDirs) == 0 {
		fmt.Fprintln(newStderr, `run "jirafs setup ..." to configure a project`)
		return 1
	}

	var content string
	if *templateFlag != "" {
		data, err := os.ReadFile(*templateFlag)
		if err != nil {
			fmt.Fprintf(newStderr, "jirafs new: cannot read template: %v\n", err)
			return 1
		}
		content = string(data)
	} else {
		tmpl, ok := templateFor(templateName)
		if !ok {
			fmt.Fprintf(newStderr, "jirafs new: unknown template %q (use story, bug, or epic)\n", templateName)
			return 1
		}
		content = tmpl
	}

	issue, pe := codec.ParseIssue(content)
	if pe != nil {
		fmt.Fprintf(newStderr, "jirafs new: invalid template: %v\n", pe)
		return 1
	}

	if *summaryFlag != "" {
		issue.Summary = *summaryFlag
	}
	if *projectFlag != "" {
		issue.Identity.Project = schema.TypedRef{Type: "project", Value: ctx.Key}
	}
	if *assigneeFlag != "" {
		v := *assigneeFlag
		issue.Assignee = &v
	}

	issue.RemoteMetadata.StateFile = "draft"

	if issue.Summary == "" {
		fmt.Fprintln(newStderr, "jirafs new: summary is required (pass --summary)")
		return 1
	}
	if issue.Identity.Type == "" {
		fmt.Fprintln(newStderr, "jirafs new: template must set an issue type")
		return 1
	}

	path := filepath.Join(proj.LocalDirs[0], slugify(issue.Summary)+".md")
	if err := os.WriteFile(path, []byte(schema.RenderIssue(*issue)), 0o644); err != nil {
		fmt.Fprintf(newStderr, "jirafs new: %v\n", err)
		return 1
	}
	fmt.Fprintf(newStdout, "jirafs: created %s\n", path)
	return 0
}
