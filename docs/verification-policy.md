# Verification Policy

## Goal

`jirafs` changes must be self-verified before review and before merge.

A change is not ready because it compiles, looks correct, or passes one happy
path. The author must prove the change works, does not regress adjacent
behavior, and still fits the intended architecture.

## Change Classes

Every change must be classified before verification starts.

### Leaf Change

A leaf change is narrow and local:

- bug fix in one module
- formatting or parsing adjustment with unchanged interfaces
- isolated CLI output change
- test-only change with no production behavior change

### Architecture-Affecting Change

An architecture-affecting change alters or risks altering system boundaries:

- new command family or new sync workflow
- schema or filesystem layout change
- changes to core document model or round-trip rules
- changes to sync planning, conflict handling, or Jira write behavior
- changes to public interfaces between CLI, library, and agent layers

When uncertain, treat the change as architecture-affecting.

Architecture-affecting changes should also be checked against:

- [Architecture](architecture.md)
- [Implementation Roadmap](implementation-roadmap.md)
- [Parallel Workstreams](parallel-workstreams.md)

## Required Local Loop

The minimum local loop for every non-trivial code change is:

1. Write or update the failing test first when practical.
2. Run the smallest relevant test scope and make it pass.
3. Run related tests for the touched area.
4. Run full lint.
5. Run the full test suite.
6. Run coverage and confirm total coverage is at least 90%.
7. Review the diff and remove debug code, dead code, and accidental changes.

The author should repeat the narrowest possible loop while editing:

- targeted test command after each logic change
- targeted lint or typecheck after interface changes
- full suite before asking for review

Do not rely on review or CI to perform first-pass debugging.

## Required Checks

The repository must provide or grow toward explicit verification commands. The
expected gates are:

- `bin/lint` or equivalent project lint command
- `bin/test` or equivalent full test command
- focused test commands for the changed area
- coverage command that reports total line coverage
- language-native static analysis appropriate for Go, such as formatting,
  vetting, and any selected linter wrapper used by `bin/lint`

If a dedicated wrapper does not exist yet, the author must run the equivalent
native commands and record them in the change description. Missing automation is
not a reason to skip the gate.

## Coverage Policy

The target is at least 90% total coverage on the main testable codebase.

Coverage rules:

- new behavior must include tests
- bug fixes must include a regression test
- architecture-affecting changes must add or update integration coverage
- untestable code paths are a design smell and must be justified in review

If total coverage drops below 90%, the change is not ready to merge unless the
change itself raises coverage and a documented exception is explicitly approved.

For Go specifically, the same policy applies:

- package-level tests are required for new behavior
- integration tests are required across read/plan/sync boundaries when touched
- coverage must be measured from the actual Go test suite, not approximated

## Verification By Change Class

### Leaf Changes

Leaf changes must verify:

- targeted unit tests covering the changed behavior
- nearby regression tests for the touched module or command
- full lint
- full test suite
- coverage gate at or above 90%

The author must also manually inspect any user-visible output changed by the
patch.

### Architecture-Affecting Changes

Architecture-affecting changes must verify everything required for leaf changes
plus broader proof:

- integration tests across module boundaries
- round-trip or end-to-end tests for affected workflows
- migration or compatibility tests when data shape or interfaces change
- explicit review against [Architecture](architecture.md) and related docs
- negative-path tests for conflicts, invalid input, and partial failure modes

The author must explain why the new shape belongs in the existing architecture
or update the architecture docs in the same change set.

## Pre-Merge Checks

Before merge, the change owner must confirm:

- all required checks passed on the final diff
- no failing or skipped tests were ignored without explanation
- no TODOs, debug logging, or temporary flags remain unintentionally
- documentation is updated when behavior, interfaces, or workflows changed
- reviewer feedback was re-verified locally after each substantive update

Re-run the relevant local loop after every meaningful review-driven code change.

## Review Gates

A reviewer should reject the change if any of the following are missing:

- exact commands run
- test evidence for new behavior or bug fixes
- coverage result meeting the 90% target
- proof that architecture-affecting changes were validated across boundaries
- updated docs when the public or architectural contract changed

“CI will catch it” is not an acceptable review argument.

## Failure Handling

When any check fails, stop the merge path and loop immediately:

1. Treat the failure as real until disproven.
2. Reduce to the smallest reproducing command or test.
3. Fix the code or fix the test only if the test is wrong for a stated reason.
4. Re-run the failing check.
5. Re-run the related local scope.
6. Re-run the full suite and coverage gate before review or merge.

Never merge with a known failing check, an unexplained flaky test, or a
coverage drop below policy.

If the failure exposes missing invariants or missing regression coverage, add
the test before closing the loop.

## Manual Verification

Some behaviors require manual confirmation in addition to automated checks:

- CLI text and JSON output
- filesystem writes and generated file layout
- diff readability for round-tripped issue documents
- error messages and conflict presentation

Manual checks do not replace automated checks. They supplement them.

## Default Standard

The default standard for `jirafs` is strict:

- verify locally first
- prove changes with tests
- keep total coverage at or above 90%
- widen verification when the change touches architecture
- do not ask for review or merge while any required check is unresolved
