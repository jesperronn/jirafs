# Ralph Task Archive

Completed task IDs stay here so active ralph ledgers can stay small.

## Foundation

- [x] B001 | none | `go.mod`, `cmd/jirafs/**`, `internal/**`, `tests/**`, `bin/**` | Go module, `jirafs` builds, `bin/test` and `bin/lint` pass.
- [x] B002 | B001 | `cmd/jirafs/**`, `internal/cli/**`, `tests/cli/**` | Help lists `init`, `export`, `plan`, `sync`, `new`, `registry`, `board`, `archive`.
- [x] B003 | B001 | `tests/**`, `internal/testutil/**` | Fixture loading and golden-output diff helpers exist.

## Settings And Context

- [x] B010 | B001 | `internal/config/**`, `tests/config/**` | Settings errors expose stable codes and messages.
- [x] B011 | B010 | `internal/config/**`, `tests/config/**` | Parse `~/.jirafs/settings.toml`; load instances/projects; expand paths.
- [x] B012 | B011 | `internal/config/**`, `tests/config/**` | Reject duplicate keys, missing instances, bad project refs, bad mirror dirs.
- [x] B013 | B011 | `internal/context/**`, `tests/context/**` | Explicit `--project` beats every other source.
- [x] B014 | B013 | `internal/context/**`, `tests/context/**` | Cwd mapping uses most-specific match; ambiguity fails clearly.

## Schema

- [x] B020a | B001 | `internal/schema/**`, `tests/schema/**` | Define typed-ref type constants and validation.
- [x] B020b | B020a | `internal/schema/**`, `tests/schema/**` | Parse and render typed refs with structured invalid-format errors.
- [x] B020c | B020b | `internal/schema/**`, `tests/schema/**` | Add typed-ref equality, zero-value, and round-trip tests.
