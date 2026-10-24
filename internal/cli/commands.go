package cli

import (
	"fmt"
	"io"
	"strings"
)

// CommandDoc is the single definition of one command's (or subcommand's)
// help: the --help printers, the top-level command list and the generated
// README/docs sections all render from it.
type CommandDoc struct {
	// Name is the command word, or "command subcommand" for a subcommand
	// page such as "archive sweep".
	Name string
	// Summary is the one-line description shown in command lists.
	Summary string
	// Usage is the full help page.
	Usage Usage
	// Hidden commands are dispatched but left out of help and generated docs.
	Hidden bool
}

// helpFlag is the row every help page ends its flag list with.
var helpFlag = [2]string{"--help, -h", "show this help message"}

var (
	flagProject = [2]string{"--project NAME", "name of a configured project (default: resolved from --cwd or the remembered project)"}
	flagCwd     = [2]string{"--cwd DIR", "working directory to resolve the project from (default: current directory)"}
)

func flags(rows ...[2]string) UsageSection {
	return UsageSection{Title: "Flags", Rows: append(rows, helpFlag)}
}

func footerFor(name string) string {
	return fmt.Sprintf(`Run "jirafs %s --help" for more information about flags.`, name)
}

// commandDocs lists every command and subcommand, top-level commands first
// in the order they appear in `jirafs --help`.
var commandDocs = []CommandDoc{
	{
		Name:    "init",
		Summary: "configure a Jira instance and project, and create its mirror directory",
		Usage: Usage{
			Synopsis: []string{"init [flags] <project-name>"},
			Description: []string{
				"Configures one Jira instance and one project: records them in",
				"~/.jirafs/settings.toml, creates the mirror directory and writes",
				"mirror.yml plus the six registry files (users, projects, statuses,",
				"sprints, fix_versions, issue_types). Existing files are left untouched.",
				"Missing values are prompted for in a terminal.",
			},
			Sections: []UsageSection{flags(
				[2]string{"--auth-type string", "auth type: basic, atlassian_api_token, bearer_token or oauth1 (default: atlassian_api_token)"},
				[2]string{"--base-url string", "Jira base URL (http:// or https://)"},
				[2]string{"--credential-ref value", "credential reference (env://..., file://... or op://...); may be repeated"},
				[2]string{"--instance string", "settings name for the Jira instance"},
				[2]string{"--key string", "Jira project key (e.g. PLAT)"},
				[2]string{"--mirror-dir string", "mirror directory (default: ~/jira/<project>)"},
				[2]string{"--set-current", "also set the remembered current project"},
			)},
		},
	},
	{
		Name:    "pull",
		Summary: "fetch Jira issues into local Markdown files (one issue or a scope)",
		Usage: Usage{
			Synopsis: []string{"pull [scope|KEY] [flags]"},
			Description: []string{
				"Fetches issues from Jira into the local mirror.",
				"KEY (PROJ-123, or a Jira URL) pulls that one issue to <local_dirs[0]>/<KEY>.md.",
				"A scope name from mirror.yml (or the built-ins my-issues and current-sprint) is refreshed incrementally; files with local edits are three-way merged with the remote (conflicting fields keep the local value; the pre-merge file is saved under .jirafs/backup).",
				"With no argument, every scope configured in mirror.yml is pulled.",
			},
			Sections: []UsageSection{
				{Title: "Examples", Rows: [][2]string{
					{"jirafs pull", "pull every configured scope"},
					{"jirafs pull my-issues", "refresh one scope"},
					{"jirafs pull PLAT-123", "pull one issue"},
					{"jirafs pull PLAT-123 --stdout", "print one issue instead of writing a file"},
				}},
				flags(
					[2]string{"--project NAME", "name of a configured project (default: resolved from --cwd or the remembered project)"},
					[2]string{"--cwd DIR", "working directory for project resolution"},
					[2]string{"--full", "scope only: ignore the last-sync cursor and refetch every issue"},
					[2]string{"--stdout", "KEY only: print the document and write no file"},
					[2]string{"--dry-run", "print what would be fetched and the real command; write nothing"},
				),
			},
		},
	},
	{
		Name:    "diff",
		Summary: "show what push would change in Jira (read-only)",
		Usage: Usage{
			Synopsis: []string{"diff [issue-key...] [flags]"},
			Description: []string{
				"Fetches each remote issue from Jira, reads the local copy from the file",
				"system, and prints the operations needed to bring the remote in line",
				"with the local copy. Without a key, diffs every locally changed issue.",
				"Read-only: nothing is written to Jira or to local files.",
			},
			Sections: []UsageSection{flags(
				[2]string{"--project NAME", "name of a configured project"},
				[2]string{"--cwd DIR", "working directory for project resolution"},
				[2]string{"--exit-code", "exit 1 when any issue differs (default exit 0)"},
			)},
			Footer: footerFor("diff"),
		},
	},
	{
		Name:    "status",
		Summary: "show workspace status and next-step hints",
		Usage: Usage{
			Synopsis: []string{"status [flags]"},
			Description: []string{
				"Reports the current state of a jirafs workspace: config, mirror files,",
				"scopes, local changes (modified, draft, conflicted; offline), and a",
				"next-step hint for onboarding.",
			},
			Sections: []UsageSection{flags(
				flagProject, flagCwd,
				[2]string{"--porcelain", "print STATE<TAB>KEY<TAB>PATH lines only, for scripts"},
			)},
		},
	},
	{
		Name:    "doctor",
		Summary: "run health checks on config, credentials, and live connectivity",
		Usage: Usage{
			Synopsis: []string{"doctor [flags]"},
			Description: []string{
				"Reports the health of a jirafs workspace: config, credential resolution,",
				"and live-probe connectivity for each configured Jira instance.",
			},
			Sections: []UsageSection{flags(
				flagProject, flagCwd,
				[2]string{"--verbose", "print request URLs during checks"},
			)},
		},
	},
	{
		Name:    "new",
		Summary: "create a new issue from a template",
		Usage: Usage{
			Synopsis: []string{
				"new <story|bug|epic> [flags]",
				"new --template <path.md> [flags]",
				"new <story|bug|epic> --prompt [--idea <text>]",
				"new --from <file|-> [flags]",
			},
			Description: []string{
				"Writes a new draft issue file to the project's local directory.",
				"It does not contact Jira. Built-in templates: story, bug, epic.",
			},
			Sections: []UsageSection{
				flags(
					[2]string{"--summary <text>", "set the issue summary"},
					[2]string{"--project <name>", "name of a configured project"},
					[2]string{"--assignee <name>", "set the assignee"},
					[2]string{"--cwd <dir>", "working directory to resolve the project from"},
					[2]string{"--template <path>", "use a user-supplied template file instead of a built-in"},
					[2]string{"--prompt", "print the template plus instructions for an AI; no draft is written"},
					[2]string{"--idea <text>", "with --prompt: the idea the AI should turn into the issue"},
					[2]string{"--force", "overwrite the draft file if it already exists"},
					[2]string{"--from <file|->", "create a draft from a filled document (- reads stdin)"},
				),
				{Title: "AI pipe", Rows: [][2]string{
					{`jirafs new story --prompt --idea "..." | llm | jirafs new --from -`, ""},
				}},
			},
		},
	},
	{
		Name:    "push",
		Summary: "create drafts and update modified issues in Jira (dry run unless --apply)",
		Usage: Usage{
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
				"With git: true in mirror.yml, --apply commits what Jira accepted as",
				"\"push to <host> at <time>\".",
			},
			Sections: []UsageSection{flags(
				[2]string{"--apply", "perform the Jira writes (without it, push is a dry run)"},
				[2]string{"--dry-run", "print what would be written and write nothing (the default)"},
				[2]string{"--write-local", "after a successful update, write the refreshed remote to the local file"},
				[2]string{"--project NAME", "name of a configured project"},
				[2]string{"--cwd DIR", "working directory to resolve the project from"},
			)},
			Footer: footerFor("push"),
		},
	},
	{
		Name:    "registry",
		Summary: "manage local registry files for typed references",
		Usage: Usage{
			Synopsis: []string{"registry [flags] <subcommand> [args]"},
			Description: []string{
				"Manage local typed-reference registry files (users, projects, statuses,",
				"sprints, fix_versions). These live in the active project's mirror directory.",
			},
			Sections: []UsageSection{
				{Title: "Subcommands", Rows: [][2]string{
					{"show <type>", "display a registry; add --json for JSON output"},
					{"init <type>", "create an empty registry file"},
					{"refresh <type>", "populate a registry from Jira (requires network access)"},
				}},
				{Title: "Types", Rows: [][2]string{
					{"users, projects, statuses, sprints, fix_versions", ""},
				}},
				flags(
					[2]string{"--project", "name of a configured project"},
					[2]string{"--cwd", "working directory for project resolution"},
					[2]string{"--json", "output registries as JSON (show)"},
				),
			},
		},
	},
	{
		Name:    "board",
		Summary: "show a local kanban-style board view",
		Usage: Usage{
			Synopsis: []string{"board [flags]"},
			Description: []string{
				"Displays a local kanban-style board view of issues grouped by",
				"status, assignee, or epic.",
			},
			Sections: []UsageSection{flags(
				[2]string{"--group-by MODE", "status, assignee, or epic (default: status)"},
				[2]string{"--project KEY", "name of a configured project"},
				[2]string{"--cwd DIR", "working directory for project resolution"},
			)},
			Footer: footerFor("board"),
		},
	},
	{
		Name:    "archive",
		Summary: "archive resolved issues and list archived files",
		Usage: Usage{
			Synopsis: []string{"archive <subcommand> [flags]"},
			Description: []string{
				"Moves resolved issues out of the live workspace into a per-project",
				"_archive directory and lists what has been archived.",
			},
			Sections: []UsageSection{
				{Title: "Subcommands", Rows: [][2]string{
					{"issue", "archive a single issue by key"},
					{"sweep", "report (or with --apply, archive) archive-eligible issues"},
					{"list", "list archived issue snapshots"},
				}},
				flags(
					[2]string{"--project", "name of a configured project"},
					[2]string{"--cwd", "working directory to resolve the project from"},
				),
			},
		},
	},
	{
		Name:    "archive issue",
		Summary: "archive a single resolved issue by key",
		Usage: Usage{
			Synopsis:    []string{"archive issue <KEY> [flags]"},
			Description: []string{"Moves one resolved issue file into the project's _archive directory."},
			Sections: []UsageSection{flags(
				[2]string{"--project KEY", "name of a configured project"},
				[2]string{"--cwd DIR", "working directory to resolve the project from"},
				[2]string{"--dry-run", "print what would be archived; move nothing"},
			)},
		},
	},
	{
		Name:    "archive sweep",
		Summary: "report (or with --apply, archive) archive-eligible issues",
		Usage: Usage{
			Synopsis:    []string{"archive sweep [flags]"},
			Description: []string{"Reports archive-eligible issues in the project's local directories; with --apply, archives them."},
			Sections: []UsageSection{flags(
				[2]string{"--project KEY", "name of a configured project"},
				[2]string{"--cwd DIR", "working directory for project resolution"},
				[2]string{"--apply", "actually archive eligible issues"},
				[2]string{"--dry-run", "list eligible issues only (the default)"},
			)},
		},
	},
	{
		Name:    "archive list",
		Summary: "list archived issue snapshots",
		Usage: Usage{
			Synopsis:    []string{"archive list [flags]"},
			Description: []string{"Lists archived issue snapshots in the project's _archive directory."},
			Sections: []UsageSection{flags(
				[2]string{"--project KEY", "name of a configured project"},
				[2]string{"--cwd DIR", "working directory to resolve the project from"},
			)},
		},
	},
	{
		Name:    "use",
		Summary: "update remembered project context",
		Usage: Usage{
			Synopsis: []string{"use [flags] [project]"},
			Description: []string{
				"Shows, sets or clears the remembered current project.",
				"With no arguments it shows the current project.",
			},
			Sections: []UsageSection{flags(
				[2]string{"--clear", "clear the remembered current project"},
				[2]string{"--project string", "name of the configured project to set as current"},
			)},
		},
	},
	{
		Name:    "help",
		Summary: "show this help message",
		Usage: Usage{
			Synopsis:    []string{"help"},
			Description: []string{`Lists all commands. Run "jirafs <command> --help" for one command.`},
		},
	},
	{
		Name:    "docs",
		Summary: "render generated documentation regions (maintainers only)",
		Hidden:  true,
		Usage:   Usage{Synopsis: []string{"docs --markdown | --render FILE"}},
	},
}

// Commands returns every documented command and subcommand, including
// hidden ones, in registry order.
func Commands() []CommandDoc {
	return append([]CommandDoc(nil), commandDocs...)
}

// TopLevelCommands returns the visible top-level commands (no subcommands)
// in help order.
func TopLevelCommands() []CommandDoc {
	var out []CommandDoc
	for _, c := range commandDocs {
		if !c.Hidden && !strings.Contains(c.Name, " ") {
			out = append(out, c)
		}
	}
	return out
}

// LookupCommand finds a registry entry by name.
func LookupCommand(name string) (CommandDoc, bool) {
	for _, c := range commandDocs {
		if c.Name == name {
			return c, true
		}
	}
	return CommandDoc{}, false
}

// PrintCommandHelp writes the help page of the named command to w. An
// unregistered name is a programming error and panics.
func PrintCommandHelp(w io.Writer, name string) {
	c, ok := LookupCommand(name)
	if !ok {
		panic("cli: no registry entry for command " + name)
	}
	PrintUsage(w, c.Usage)
}
