# Development Rules

## Goal

These rules define how `jirafs` work should be prepared for another agent,
reviewer, or integrator when multiple workers are active at once.

They extend, but do not replace:

- [Verification Policy](verification-policy.md)
- [Parallel Workstreams](parallel-workstreams.md)

When these documents overlap, keep the stricter rule.

## Core Operating Rules

- Respect path ownership. Do not edit outside your assigned scope unless an
  explicit integration task says otherwise.
- Assume other workers may change other files at any time. Do not revert,
  rewrite, or "clean up" unrelated work.
- Keep changes legible for handoff. Another worker should be able to see what
  changed, why it changed, and how it was verified without reconstructing your
  intent from raw diffs.
- Treat contract changes as coordination events, not incidental edits.

## Commit Chunking

Commits should be small enough to review and large enough to leave the tree in
a coherent state.

### Required Shape

- One commit should represent one logical step.
- Keep commits path-local to the owning workstream whenever possible.
- Separate pure renames, mechanical refactors, behavior changes, and test
  additions unless they are inseparable.
- Add or update tests in the same commit as the behavior they prove when
  practical.
- Do not mix unrelated fixes just because they were nearby in the editor.

### Preferred Order

For non-trivial work, chunk commits in this order when possible:

1. Contract or seam introduction.
2. Implementation behind that contract.
3. Tests proving the behavior, if not already included with the implementation.
4. Docs or follow-on cleanup tied directly to the same change.

If a contract change affects another stream, the owning stream should land the
contract-facing commit first so dependent work can rebase cleanly.

### Bad Chunking Patterns

- one commit that mixes interface churn, feature logic, and opportunistic
  cleanup
- one commit that edits multiple ownership zones without an explicit integration
  reason
- one commit that changes behavior but leaves proof for a later "test follow-up"
- one commit that hides a risky change inside formatting noise

## Agent Handoff Expectations

A handoff must let the next worker continue without guessing.

### Every Handoff Must State

- current objective
- exact owned paths and explicit out-of-scope paths
- current branch name
- whether the change is a leaf change or architecture-affecting change under
  [Verification Policy](verification-policy.md)
- what is finished
- what is still pending
- any contract dependencies on other streams
- known risks, open questions, or blockers

### Every Handoff Must Include

- the exact files changed
- the exact commands already run
- the results of those commands
- the next smallest useful step for the receiving worker

### Handoff Rules

- Do not say "mostly done" without naming the unfinished edge cases.
- Do not imply verification that was not actually run.
- If you had to stop before full verification, label the handoff as partial and
  list the missing gates explicitly.
- If another stream needs to change first, say that directly instead of pushing
  speculative edits into their ownership zone.

## Branch And PR Hygiene

Branches and PRs must preserve review clarity and parallel safety.

### Branch Rules

- Keep branches short-lived and focused on one workstream or one intentional
  integration change.
- Rebase or merge from `main` after contract landings, not only at the end of a
  long branch.
- Do not accumulate unrelated local commits before opening or updating a PR.

### PR Rules

- Keep PRs path-local unless the change is explicitly an integration task.
- Name the affected workstream or contract boundary in the PR description when
  relevant.
- Call out cross-stream interface changes before merge.
- Update docs in the same PR when behavior, contracts, or operator workflows
  changed and the docs fall inside your assigned scope.
- Do not bundle unrelated follow-up cleanup into a PR that is supposed to prove
  one behavior change.

### Reviewer Clarity

Every PR description should make it obvious:

- what changed
- why it changed
- what remained intentionally untouched
- how the author verified the final diff

## Verification Evidence For Handoff Or Review

Verification evidence must be concrete enough that another agent or reviewer can
trust it and, if needed, replay it.

### Required Evidence Format

When handing off work or requesting review, record:

- exact command lines, not paraphrases
- whether each command passed or failed
- the scope of each command: targeted test, related tests, full lint, full
  suite, coverage, manual check
- any manual verification performed and what was observed
- any required gate that was not run, with the reason

### Evidence Rules

- "Tests pass locally" is not sufficient.
- "Linted" is not sufficient without the command.
- Coverage claims must include the command and reported outcome when coverage is
  required by policy.
- Manual verification should name the user-visible behavior checked, such as CLI
  output, filesystem layout, or round-trip diffs.
- If the diff changed after review feedback, evidence must reflect the final
  diff, not an earlier revision.

### Partial Verification

If work is handed to another agent before all gates are complete, separate the
evidence into:

- verified now
- still required before review
- still required before merge

This keeps a reviewer from mistaking an incomplete local loop for merge-ready
work.

## Default Standard

- chunk commits by logical proof, not by convenience
- make handoffs explicit enough for another worker to continue safely
- keep branches and PRs narrow enough for parallel development
- record verification evidence with exact commands and outcomes
- do not interfere with unrelated in-flight work
