## Instructions: Break monolithic task ledger into individual task files

**Input:** `docs/ralph-loop-implementation-tasks.md`
**Output:** files in `tasks/` and `tasks/done/`

### Step 1 — Create directories

```bash
mkdir -p tasks/done
```

### Step 2 — For each task line in the ledger, create one file

Each task line looks like:
```
- [x] B024a | B021b | `internal/schema/**` | Define typed plan operation model
- [ ] B024b | B024a | `internal/schema/**` | Define conflict model
```

Parse each line into these fields:
1. **status** — `[x]` = done, `[ ]` = pending
2. **id** — e.g. `B024b`
3. **deps** — everything between the first and second `|`, copied verbatim
4. **paths** — everything between the second and third `|`, copied verbatim
5. **objective** — everything after the third `|`

For each task, write a file with this exact template:

```markdown
# <id>: <objective>

## Dependencies

<deps — pipe-separated as written in ledger, e.g. `B024a | B024b` — write "none" if empty>

## Owned paths

<paths — pipe-separated as written in ledger>

## Acceptance

<objective>

## Rules

- No placeholder code, no TODO comments, no partial implementations.
- Add tests in the same task.
- Final gates: `bin/test` and `bin/lint` must both pass.
- Commit with conventional commit wording after gates pass.
- Move this file to `tasks/done/` as part of the same commit.
```

**Filename:** `tasks/<id>.md` for pending tasks, `tasks/done/<id>.md` for done tasks.

### Step 3 — Handle multi-line entries

Some task lines have sub-bullets (indented lines starting with `-`). Append those lines verbatim under a `## Notes` section at the bottom of the task file.

### Step 4 — Handle archived tasks

The ledger header lists archived IDs (e.g. `B001`, `B002`…) as already-completed. For each archived ID, write a minimal file in `tasks/done/`:

```markdown
# <id>: archived

## Status

Archived — completed before individual task files were introduced.
```

### Step 5 — Verify the count

```bash
echo "Pending:" && ls tasks/*.md | wc -l
echo "Done:" && ls tasks/done/*.md | wc -l
```

Cross-check: pending + done should equal the total task count from the ledger (including archived IDs).

### Step 6 — Do NOT delete the ledger yet

Leave `docs/ralph-loop-implementation-tasks.md` in place until the new RALPH.md is wired up and tested.
