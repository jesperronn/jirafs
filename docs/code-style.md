# Code Style

## Goal

`jirafs` code should stay simple to read, easy to refactor, easy to test, and
easy to lint automatically.

This document defines the default style target for production code, tests, and
automation wrappers.

Read this together with:

- [Verification Policy](verification-policy.md)
- [Development Rules](development-rules.md)
- [Validator Contract](validator-contract.md)

## Primary Standard

Prefer simple code over clever code.

Default expectations:

- small functions with one clear purpose
- explicit names over compressed logic
- straight-line control flow where practical
- data shapes that are easy to inspect in tests
- no hidden global mutation unless the boundary requires it
- no speculative abstractions before the second real use

## Formatting Standard

The repo should standardize on tools with deterministic output and built-in
autofix.

For Go code, the default formatting and lint baseline should be:

- `gofmt` for formatting
- `goimports` for import normalization
- `golangci-lint --fix` as the main lint autofixer wrapper

`bin/lint` should eventually run the autofix-capable formatter/linter in a
non-destructive CI-safe mode plus any repo-specific checks.

Local developer flow should also support an explicit autofix command or mode.
The standard is not "lint only"; it is "lint with an approved autofix path".

## Simplicity Rules

- keep public interfaces narrow
- prefer plain structs and functions over framework-heavy patterns
- isolate IO from pure transformation logic
- keep parsing, planning, and syncing as separate layers
- return structured errors instead of stringly-typed failure states
- remove dead branches and temporary compatibility code quickly

## Testability Rules

- write code so the core behavior can be tested without the network
- keep Jira transport behind explicit boundaries
- prefer table-driven tests for input-output rules
- prefer golden files for stable document rendering
- add regression tests for every bug fix
- avoid logic that only works through end-to-end manual execution

## Refactoring Rules

Refactoring is required when code becomes harder to understand than the
behavior it implements.

Safe refactor expectations:

- keep behavior unchanged unless the task explicitly changes behavior
- add or keep proof before moving code
- separate mechanical moves from logic changes when practical
- leave modules smaller or clearer than before

## Readability Rules

- one file should usually have one obvious reason to exist
- one function should usually answer one question or perform one action
- comments should explain why, not restate obvious code
- branch conditions should be readable without mental inversion chains
- avoid helper layers that only rename existing calls without adding meaning

## Lint Policy

Linting should be strict but boring.

The preferred lint standard:

- use widely understood rules
- keep rule count small enough that violations are actionable
- prefer autofixable rules when choosing between equivalent style checks
- avoid project-specific style rules unless they protect a real architecture
  boundary

If a lint rule produces frequent low-value churn, remove or narrow the rule.

## Review Standard

Review should be able to ask:

- is this the simplest code that satisfies the requirement
- is the control flow obvious
- is the code easy to test
- can the linter or formatter keep this style stable automatically

If the answer is no, refactor before adding more features around it.

## Default Standard

- simple code first
- deterministic formatting
- lint rules with an autofix path
- code structured for tests and refactors
- readability over cleverness
