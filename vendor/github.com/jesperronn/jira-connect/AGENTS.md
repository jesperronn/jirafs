# AGENTS

Read `README.md` first. Keep the adapter generic and follow the
[read/write decisions](docs/2026-11-15-generic-issue-write-design.md).

- Use the versioned Data Center references in `docs/datacenter-api.md`, never
  Cloud v3 routes or capability probes.
- Tests and verification are offline. Inject HTTP and credential providers;
  never query Jira, read real secrets or invoke a real credential tool.
- Run `bin/lint` and `bin/test`; report their output before claiming success.
- The example collector is manual only. Its dry-run may be checked offline;
  keep real captures local/ignored and commit only reviewed sanitized fixtures.
- Keep dependencies few. Use conventional commits and stage explicit paths.
  Never commit binaries/caches/secrets or infer authorization to publish.
- CI calls the maintainer entrypoints documented in `docs/workflows.md`.
- On macOS use `gsed`, not BSD `sed`.
