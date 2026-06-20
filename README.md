# jirafs

`jirafs` is a local-first Jira workspace.

The project goal is to make Jira issues editable as structured Markdown files,
sync those files safely back to Jira, and retain a durable archive that can be
used for analysis, agent workflows, and team process improvement.

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
- [Verification Policy](docs/verification-policy.md)
- [Development Rules](docs/development-rules.md)
- [Mirror Model](docs/mirror-model.md)
- [Settings And Context](docs/settings-and-context.md)
- [Project Selection CLI](docs/project-selection-cli.md)
- [Implementation Packets](docs/implementation-packets.md)

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
- [Verification Policy](docs/verification-policy.md)
- [Development Rules](docs/development-rules.md)
- [Mirror Model](docs/mirror-model.md)
- [Settings And Context](docs/settings-and-context.md)
- [Project Selection CLI](docs/project-selection-cli.md)
- [Implementation Packets](docs/implementation-packets.md)

## Language Direction

The current preference is Python for the first implementation:

- strong YAML/JSON/Markdown tooling
- fast iteration while the schema is still moving
- simple CLI packaging

Go remains a plausible future choice if single-binary distribution becomes a
higher priority than iteration speed.
