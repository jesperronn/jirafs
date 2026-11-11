# Docs index

Each line is the file's title and its opening sentence. `cli.md` also holds
generated regions: run `bin/gendocs`, not a hand edit.

## Product and design

- [Architecture](architecture.md): standalone CLI-first project that turns Jira into a local mirror.
- [CLI](cli.md): the CLI is the primary interface.
- [Issue Format](issue-format.md): rules for issue files.
- [Mirror Model](mirror-model.md): the local mirror keeps a small working set of Jira issues.
- [Sync Model](sync-model.md): sync must be explicit, inspectable, and conflict-aware.
- [References](references.md): references to Jira entities are typed and stable.
- [Templates](templates.md): local-first creation of new issues.
- [Validator Contract](validator-contract.md): every edit is followed by explicit validation.
- [Settings And Context](settings-and-context.md): the global settings file that resolves Jira context.
- [Credential Sources](credential-sources.md): shared operator configuration is separate from secrets.
- [Project Selection CLI](project-selection-cli.md): predictable project selection for humans and scripts.
- [Agent Integration](agent-skill-integration.md): what agents help with and what they do not own.

## Working rules

- [Code Style](code-style.md): keep code simple to read, refactor and test.
- [Development Rules](development-rules.md): how work is prepared for another agent.
- [Verification Policy](verification-policy.md): changes are self-verified before review and merge.

## Planning and orchestration

- [Implementation Roadmap](implementation-roadmap.md)
- [Implementation Packets](implementation-packets.md): handoff units for implementation agents.
- [Parallel Workstreams](parallel-workstreams.md): the intended parallel split.
- [Pre-Live Parallel Plan](pre-live-parallel-plan.md): remaining MVP work as worker-sized streams.
- [Orchestration Model](orchestration-model.md): one orchestrator coordinating many small tasks.
- [Ralph Task Archive](ralph-task-archive.md): completed task IDs, so active ledgers stay small.

## Archive

- [archive/](archive/): old task handoff and report files, kept unchanged.
