# AGENTS

- Read `README.md` first.
- Keep `jirafs` local-first, explicit, and conflict-aware.
- Prefer small Go code, few dependencies, and strong tests.
- To verify work is complete, always run `bin/verify` (lint + tests).
- The active ralph loop lives in `ralphs/execute-tasks/`; tasks are files
  under `tasks/` and move to `tasks/done/` when complete.
- Before reporting a task commit ready for integration, run `bin/handoff`.
- Report the task filename, commit hash, verify result, and the
  `clean|done|blocked` line from `bin/handoff`.
- When `jirafs` prints a "useful next step" or similar troubleshooting hint,
  prefer a copy-pasteable command the user can run immediately, not a vague
  description.
