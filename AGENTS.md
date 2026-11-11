# AGENTS

`jirafs`: a local-first, explicit, conflict-aware CLI that mirrors Jira as files. Read `README.md` first.

## Definition of done

`bin/workflows` lists every command. Run `bin/verify` (lint, `gendocs --check`, tests) and show its output before saying "done".

Before reporting a task commit ready for integration, run `bin/handoff`. Report the task filename, commit hash, verify result, and the `clean|done|blocked` line it prints.

## Conventions

- Small Go code, few dependencies, strong tests. Shell tests are `tests/fixtures/*.sh`, run by `bin/test`.
- Conventional commits (`feat|fix|test|docs|chore(scope): subject`); stage files by path.
- On macOS use `gsed`, not BSD `sed`.
- When `jirafs` prints a "useful next step" or similar troubleshooting hint, make it a copy-pasteable command the user can run immediately, not a vague description.

## Hard rules

- `bin/handoff` only rebases the branch onto `main` and runs `bin/verify`. Never merge or move `main`; the user or orchestrator merges.
- `bin/live-throwaway` writes to a live Jira story: run by hand only.
- Do not commit built binaries or caches (`jirafs`, `.gocache/`, `.jumbo/`).

## Ralph loop

The active loop lives in `ralphs/execute-tasks/`. Tasks are files under `tasks/`; move one to `tasks/done/` when complete.

## Docs

[docs/README.md](docs/README.md) indexes the design and process docs; `bin/gendocs` regenerates the generated regions of `README.md` and `docs/cli.md`.
