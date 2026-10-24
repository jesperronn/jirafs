# jirafs

`jirafs` is a local-first Jira workspace.

The project goal is to make Jira issues editable as structured Markdown files,
sync those files safely back to Jira, and retain a durable archive that can be
used for analysis, agent workflows, and team process improvement.

## Current Workflow

The current CLI is usable as a narrow operator workflow:

1. Configure one Jira instance and one project with `jirafs setup`.
2. Select or confirm the active project with `jirafs use`.
3. Refresh one live mirror scope with `jirafs mirror refresh`.
4. Export or inspect one issue locally with `jirafs export issue`.
5. Edit the mirrored Markdown file.
6. Review the planned write-back with `jirafs plan`.
7. Apply the write path with `jirafs sync`.

Example:

```bash
jirafs setup \
  --project platform \
  --key PLAT \
  --instance work \
  --base-url https://jira.example.com \
  --auth-type atlassian_api_token \
  --credential-ref file://~/.jirafs/credentials/work-user.toml \
  --credential-ref env://JIRAFS_WORK_API_TOKEN \
  --set-current

jirafs use
jirafs mirror refresh my-issues
jirafs export issue PLAT-123            # writes <local_dir>/PLAT-123.md
jirafs export issue PLAT-123 --stdout > /tmp/PLAT-123.md
jirafs plan PLAT-123
jirafs sync PLAT-123
```

## Mirror scopes

Scopes live in `<mirror_dir>/mirror.yml`. Refresh one by name with
`jirafs mirror refresh <scope-name>`.

```yaml
scopes:
  - name: plat
    type: project        # project = "PLAT" ORDER BY key
    target: PLAT
  - name: urgent
    type: jql            # target is sent to Jira verbatim
    target: project = PLAT AND priority = Highest ORDER BY created
  - name: plat-api
    type: component      # needs "project"; project = "PLAT" AND component = "API"
    project: PLAT
    target: API
```

`my-issues` and `current-sprint` also work without a `mirror.yml` entry.
Unknown scope names print the configured names and an example to add.

## Current Status

What works today:

- user-level setup writes `~/.jirafs/settings.toml`
- project selection and remembered current-project state
- mirror refresh wiring for named scopes such as `my-issues`
- export, plan, and sync command paths for the current safe field set
- credential refs from `env://`, `file://`, and `op://`
- auth support for `basic`, `atlassian_api_token`, and `bearer_token`

What is still rough:

- the README has historically assumed the deeper docs carry most of the
  operator story
- the CLI does not yet present one obvious “show me status” command
- troubleshooting output should keep "useful next step" hints as concrete
  copy-pasteable commands, not just descriptive text
- live Jira behavior still needs real-instance shakeout and better diagnostics

## Obvious Next Steps

If you are trying `jirafs` for the first time:

1. Run `jirafs setup ... --set-current`.
2. Run `jirafs use` to confirm the active project.
3. Run `jirafs mirror refresh my-issues`.
4. Inspect the mirror directory and `mirror.yml`.
5. Run `jirafs export issue ABC-123` or open a mirrored issue file.
6. Run `jirafs plan ABC-123` before any write attempt.

If something fails:

- no settings yet (first run): run
  `jirafs setup --project <name> --key <KEY> --instance <name> --base-url https://<site>.atlassian.net --credential-ref env://JIRAFS_API_TOKEN --set-current`
- config or auth issue: inspect `~/.jirafs/settings.toml`
- scope issue: inspect `<mirror_dir>/mirror.yml`
- project resolution issue: run `jirafs use --project <name>`
- live Jira issue: retry with a single issue export before a whole-scope refresh
- empty sync scope: run `jirafs mirror refresh current-sprint` or
  `jirafs mirror refresh my-issues`, then `jirafs sync <KEY>` per issue
  (multi-issue `jirafs sync` is not implemented yet)
- not sure what a command will change: add `--dry-run` (sync, push, export
  issue, new, archive issue, mirror refresh, mirror archive-sweep); it prints
  the plan and the command for the real run, and writes nothing

## Testing Policy

Tests and agent verification must stay offline unless a task explicitly says
otherwise.

- mock 1Password and Jira calls by default
- use fake transports or injected providers for network-bound code
- do not add live endpoint tests as a shortcut for missing test seams
- if a live probe is genuinely required, state that in the task and the test
  name

## TLS Certificates

`jirafs` always verifies Jira server certificates against the system trust
store; there is no option to skip verification. For a Jira instance signed by a
private CA, add that CA certificate to the operating system trust store (for
example `update-ca-certificates` on Debian/Ubuntu, or Keychain Access on
macOS) and `jirafs` will trust it.

## Command Surface

The most important commands right now are:

- `jirafs setup`
- `jirafs use`
- `jirafs mirror refresh`
- `jirafs export issue`
- `jirafs plan`
- `jirafs sync`

## Creating Issues With An AI

`jirafs` has no LLM dependency. `new --prompt` prints a template plus
instructions, and `new --from` validates and saves the filled document as a
local draft:

```sh
jirafs new story --prompt --idea "users want dark mode" | llm | jirafs new --from -
```

`--from` accepts a file path or `-` for stdin, validates the document (issue
type and summary are required), applies `--project`/`--assignee`, and prints
`jirafs: created <path>`. Invalid documents are rejected with the offending
field and a retry command.

Drafts are created in Jira with `jirafs push`. It is a dry run by default;
`--apply` creates the issue, renames the file to `KEY.md`, marks it synced and
prints the browse URL:

```sh
jirafs push jira/add-dark-mode.md            # show what would be created
jirafs push jira/add-dark-mode.md --apply    # POST /rest/api/2/issue
```

Only files with `state: draft` can be pushed; use `jirafs sync KEY` to update
existing issues.

### Custom fields on create

Some Jira instances require custom fields when creating an issue. Add them to
the draft's frontmatter as `custom_fields:`. Keys are `customfield_NNNNN`;
values may be a string, number, list, or a map such as `{value: "X"}` or
`{id: "123"}`, and are sent to Jira as JSON unchanged:

```yaml
custom_fields:
  customfield_12601:
    value: Platform Team
  customfield_10002: 5
  customfield_10003:
    - id: "10100"
```

To avoid repeating them in every draft, set per-project defaults in
`~/.jirafs/settings.toml`. A draft's own `custom_fields:` win over
`create_defaults` for the same key; templates may include `custom_fields:` too.

```toml
[projects.platform.create_defaults]
customfield_12601 = { value = "Platform Team" }
customfield_10002 = 5
```

Custom fields are sent on create only; they are not synced back.

`jirafs push` (dry run) and `jirafs push --apply` ask Jira's createmeta
endpoint which fields are required and list the ones the payload lacks, with
allowed values and a copy-pasteable `custom_fields:` / `create_defaults`
snippet. `--apply` refuses to create until they are set. If createmeta cannot
be reached, a warning is printed and the push continues.

## Initial Scope

- Export Jira issues, sprints, and related metadata into a local filesystem.
- Edit current and future issues locally, then sync changes back to Jira.
- Create new issues locally from typed templates, then upload them.
- Keep historical issues and sprint snapshots as a structured archive.
- Preserve enough metadata to support later features such as kanban views,
  release planning, and archive-driven process analysis.

## Design Principles

- Local files are the main human editing surface.
- Structured frontmatter is the canonical machine-readable layer.
- Markdown sections are used for long-form text fields.
- Sync must be explicit, inspectable, and conflict-aware.
- References to Jira entities must be typed and resolvable.
- The core implementation should work from the command line without requiring
  a Codex skill or another agent layer.

## Documents

- [Architecture](docs/architecture.md)
- [Issue Format](docs/issue-format.md)
- [Sync Model](docs/sync-model.md)
- [References](docs/references.md)
- [Templates](docs/templates.md)
- [CLI](docs/cli.md)
- [Agent Integration](docs/agent-skill-integration.md)
- [Implementation Roadmap](docs/implementation-roadmap.md)
- [Parallel Workstreams](docs/parallel-workstreams.md)
- [Orchestration Model](docs/orchestration-model.md)
- [Verification Policy](docs/verification-policy.md)
- [Development Rules](docs/development-rules.md)
- [Code Style](docs/code-style.md)
- [Validator Contract](docs/validator-contract.md)
- [Mirror Model](docs/mirror-model.md)
- [Settings And Context](docs/settings-and-context.md)
- [Credential Sources](docs/credential-sources.md)
- [Project Selection CLI](docs/project-selection-cli.md)
- [Implementation Packets](docs/implementation-packets.md)
- [Ralph Task Archive](docs/ralph-task-archive.md)
- [Pre-Live Parallel Plan](docs/pre-live-parallel-plan.md)

Read these after the README:

- [CLI](docs/cli.md) for command-level intent
- [Settings And Context](docs/settings-and-context.md) for config shape
- [Credential Sources](docs/credential-sources.md) for auth references
- [Mirror Model](docs/mirror-model.md) for live/archive behavior
- [Sync Model](docs/sync-model.md) for plan/sync behavior

## Implementation Direction

The preferred implementation order is:

1. Define the schema and Markdown codec.
2. Implement registry loading and reference resolution.
3. Implement Jira read/export.
4. Implement local diff/plan.
5. Implement safe sync for a small editable field set.
6. Add templates, bulk export, archive snapshots, and board projections.
7. Add global settings, project selection, and mirror scope management as
   first-class behavior.

The execution plan for that work now lives in:

- [Implementation Roadmap](docs/implementation-roadmap.md)
- [Parallel Workstreams](docs/parallel-workstreams.md)
- [Orchestration Model](docs/orchestration-model.md)
- [Verification Policy](docs/verification-policy.md)
- [Development Rules](docs/development-rules.md)
- [Code Style](docs/code-style.md)
- [Validator Contract](docs/validator-contract.md)
- [Mirror Model](docs/mirror-model.md)
- [Settings And Context](docs/settings-and-context.md)
- [Credential Sources](docs/credential-sources.md)
- [Project Selection CLI](docs/project-selection-cli.md)
- [Implementation Packets](docs/implementation-packets.md)
- [Ralph Task Archive](docs/ralph-task-archive.md)
- [Pre-Live Parallel Plan](docs/pre-live-parallel-plan.md)

## Language Direction

`jirafs` is now planned as a Go implementation.

Why Go:

- single static binary distribution
- minimal runtime assumptions on the operator machine
- lower packaging and interpreter drift risk
- strong long-term fit for a durable CLI utility

The architecture and workflow docs remain valid; the implementation language
changes, not the product model.

## Dependency Policy

`jirafs` should stay conservative about dependencies.

Default rules:

- prefer the Go standard library when it is sufficient
- add a third-party dependency only when it removes meaningful complexity or
  avoids building risky parsing or CLI behavior ourselves
- prefer small, stable, well-maintained libraries over large frameworks
- prefer libraries with narrow scope and low transitive dependency counts
- avoid dependencies that force a large application framework shape onto the
  codebase

Initial likely third-party categories:

- TOML parsing
- YAML parsing when needed for local mirror config files
- Markdown frontmatter handling only if the standard library plus a small
  parser is not enough
- CLI command parsing if the standard library proves too bare for predictable UX

What we want to avoid:

- heavy framework-style CLI stacks
- large indirect dependency trees for simple parsing problems
- generator-driven codebases where handwritten code would stay clearer
- dependencies that make static builds or cross-platform release packaging
  harder

The repository should keep the same verification standard regardless of
language choice: high test coverage, strict local linting, and green CI before
merge.

### Epic link field

Set `epic_link_field = "customfield_10014"` under `[projects.<name>]` in
`~/.jirafs/settings.toml` to export epic links. `JIRAFS_EPIC_LINK_FIELD`
overrides it; `jirafs doctor` shows the effective value.
