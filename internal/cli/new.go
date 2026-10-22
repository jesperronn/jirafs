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
	newStdin  io.Reader = os.Stdin
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
  jirafs new <story|bug|epic> --prompt [--idea <text>]
  jirafs new --from <file|-> [flags]

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
  --prompt           Print the template plus instructions for an AI; no draft is written.
  --idea <text>      With --prompt: the idea the AI should turn into the issue.
  --force            Overwrite the draft file if it already exists.
  --from <file|->    Create a draft from a filled document (- reads stdin).

AI pipe:
  jirafs new story --prompt --idea "..." | llm | jirafs new --from -
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
	promptFlag := fs.Bool("prompt", false, "print the template plus AI instructions")
	ideaFlag := fs.String("idea", "", "idea text to include in the prompt")
	fromFlag := fs.String("from", "", "filled document to create a draft from (- for stdin)")
	forceFlag := fs.Bool("force", false, "overwrite an existing draft file")
	dryRunFlag := fs.Bool("dry-run", false, "print the target path and whether it would be overwritten; write nothing")

	if err := parseFlags(fs, flagArgs); err != nil {
		return 1
	}
	if templateName == "" && len(fs.Args()) > 0 {
		templateName = fs.Args()[0]
	}

	if *promptFlag {
		return runNewPrompt(templateName, *templateFlag, *ideaFlag)
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
	if *fromFlag != "" {
		data, err := readFromSource(*fromFlag)
		if err != nil {
			fmt.Fprintf(newStderr, "jirafs new: cannot read %s: %v\n", *fromFlag, err)
			return 1
		}
		content = stripCodeFence(string(data))
	} else if *templateFlag != "" {
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
		if *fromFlag != "" {
			fmt.Fprintf(newStderr, "jirafs new: invalid document: %v\n%s\n", pe, retryHint(*fromFlag))
			return 1
		}
		fmt.Fprintf(newStderr, "jirafs new: invalid template: %v\n", pe)
		return 1
	}
	if *fromFlag != "" && (issue.Identity.Project.IsZero() || issue.Identity.Project.Value == "PROJECT") {
		issue.Identity.Project = schema.TypedRef{Type: "project", Value: ctx.Key}
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

	if *fromFlag != "" {
		if msg := validateFilled(issue); msg != "" {
			fmt.Fprintf(newStderr, "jirafs new: invalid document: %s\n%s\n", msg, retryHint(*fromFlag))
			return 1
		}
	}
	if issue.Summary == "" {
		fmt.Fprintln(newStderr, "jirafs new: summary is required (pass --summary)")
		return 1
	}
	if issue.Identity.Type == "" {
		fmt.Fprintln(newStderr, "jirafs new: template must set an issue type")
		return 1
	}

	path := filepath.Join(proj.LocalDirs[0], slugify(issue.Summary)+".md")
	_, statErr := os.Stat(path)
	exists := statErr == nil
	if exists && !*forceFlag && *dryRunFlag {
		fmt.Fprintf(newStdout, "would refuse: %s already exists\nto overwrite it run: jirafs new %s\n", path, shellJoin(append(withoutFlag(args, "--dry-run"), "--force")))
		return 0
	}
	if exists && !*forceFlag {
		fmt.Fprintf(newStderr, "jirafs new: %s already exists; refusing to overwrite\nto overwrite it run: jirafs new %s\nor choose a different --summary to get a different file name\n", path, shellJoin(append(append([]string{}, args...), "--force")))
		return 1
	}
	if *dryRunFlag {
		fmt.Fprintf(newStdout, "would %s %s\n", writeVerb(path), path)
		dryRunDone(newStdout, "re-run the same jirafs new command without --dry-run")
		return 0
	}
	if err := os.WriteFile(path, []byte(schema.RenderIssue(*issue)), 0o644); err != nil {
		fmt.Fprintf(newStderr, "jirafs new: %v\n", err)
		return 1
	}
	fmt.Fprintf(newStdout, "jirafs: created %s\nnext: edit it, then run: jirafs push %s\n", path, shellQuote(path))
	return 0
}

// promptInstructions is the AI-facing preamble printed by `new --prompt`.
const promptInstructions = `You are filling in a Jira issue document.
Rules:
- Fill every section from the idea below; replace every TODO placeholder.
- Keep the frontmatter keys exactly as they are; set "summary" to a concise one-line title.
- Do not add sections that are not in the template.
- Output only the completed document (frontmatter and sections), with no commentary and no code fences.
`

// buildPrompt turns a template and an idea into the text handed to an AI.
// It is the seam for a future built-in LLM call: replace it with a function
// that sends this prompt to a model and returns the filled document, which
// createDraft-style validation (`new --from`) then consumes unchanged.
func buildPrompt(tmpl, idea string) string {
	var b strings.Builder
	b.WriteString(promptInstructions)
	b.WriteString("\nTemplate:\n\n")
	b.WriteString(tmpl)
	if idea != "" {
		b.WriteString("\nIdea:\n\n")
		b.WriteString(idea)
		b.WriteString("\n")
	}
	return b.String()
}

// runNewPrompt implements `jirafs new <type> --prompt`.
func runNewPrompt(templateName, templatePath, idea string) int {
	var tmpl string
	if templatePath != "" {
		data, err := os.ReadFile(templatePath)
		if err != nil {
			fmt.Fprintf(newStderr, "jirafs new: cannot read template: %v\n", err)
			return 1
		}
		tmpl = string(data)
	} else {
		t, ok := templateFor(templateName)
		if !ok {
			fmt.Fprintf(newStderr, "jirafs new: --prompt needs a template; try: jirafs new story --prompt --idea \"...\"\n")
			return 1
		}
		tmpl = t
	}
	fmt.Fprint(newStdout, buildPrompt(tmpl, idea))
	return 0
}

// readFromSource reads a document from a file path, or stdin when path is "-".
func readFromSource(path string) ([]byte, error) {
	if path == "-" {
		return io.ReadAll(newStdin)
	}
	return os.ReadFile(path)
}

// stripCodeFence removes a wrapping Markdown code fence that LLMs often add.
func stripCodeFence(s string) string {
	t := strings.TrimSpace(s)
	if !strings.HasPrefix(t, "```") {
		return s
	}
	lines := strings.Split(t, "\n")
	lines = lines[1:]
	if n := len(lines); n > 0 && strings.TrimSpace(lines[n-1]) == "```" {
		lines = lines[:n-1]
	}
	return strings.Join(lines, "\n")
}

// validateFilled checks the fields a filled document must carry and returns
// a message naming the offending field, or "" when valid.
func validateFilled(issue *schema.Issue) string {
	if issue.Identity.Type == "" {
		return "field \"type\" is required (set type: story, bug, or epic in the frontmatter)"
	}
	if strings.TrimSpace(issue.Summary) == "" {
		return "field \"summary\" is required (set a one-line summary: in the frontmatter)"
	}
	return ""
}

// retryHint returns a copy-pasteable command for retrying after a bad document.
func retryHint(from string) string {
	if from == "-" {
		return `retry: jirafs new story --prompt --idea "..." | llm | jirafs new --from -`
	}
	return fmt.Sprintf("retry: fix the document, then run: jirafs new --from %s", from)
}

// shellQuote quotes s for a POSIX shell only when it needs it.
func shellQuote(s string) string {
	if s != "" && strings.Trim(s, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_@%+=:,./-") == "" {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func shellJoin(args []string) string {
	q := make([]string, len(args))
	for i, a := range args {
		q[i] = shellQuote(a)
	}
	return strings.Join(q, " ")
}

func withoutFlag(args []string, name string) []string {
	var out []string
	for _, a := range args {
		if a != name {
			out = append(out, a)
		}
	}
	return out
}
