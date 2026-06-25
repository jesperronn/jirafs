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
jirafs export issue PLAT-123 > /tmp/PLAT-123.md
jirafs plan PLAT-123
jirafs sync PLAT-123
```

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
- the CLI does not yet present one obvious “next step” hint after each command
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

- config or auth issue: inspect `~/.jirafs/settings.toml`
- scope issue: inspect `<mirror_dir>/mirror.yml`
- project resolution issue: run `jirafs use --project <name>`
- live Jira issue: retry with a single issue export before a whole-scope refresh

## Command Surface

The most important commands right now are:

- `jirafs setup`
- `jirafs use`
- `jirafs mirror refresh`
- `jirafs export issue`
- `jirafs plan`
- `jirafs sync`

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
- [Ralph Parallel Workflow](docs/ralph-parallel-workflow.md)
- [Pre-Live Parallel Plan](docs/pre-live-parallel-plan.md)
- [Ralph Stream WS4: Plan And Sync](docs/ralph-stream-ws4-plan-sync.md)

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
- [Ralph Parallel Workflow](docs/ralph-parallel-workflow.md)
- [Pre-Live Parallel Plan](docs/pre-live-parallel-plan.md)
- [Ralph Stream WS4: Plan And Sync](docs/ralph-stream-ws4-plan-sync.md)

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
