# jira-connect

A generic Jira Data Center REST v2 adapter with a Go library, JSON CLI and
stdio MCP server. It reads any issue type and updates explicitly supplied fields
or links. Business interpretation stays in the consuming application; see the
[design decisions](docs/2026-11-15-generic-issue-write-design.md).

## Install

For a release archive, extract it and use `./jira-connect` or
`./jira-connect-mcp` in place of the `.bin/` paths below.

Use Go 1.26+ from this checkout:

```sh
bin/build
```

The local binaries are ignored by Git. Alternatively, `go install ./cmd/jira-connect ./cmd/jira-connect-mcp` installs into `GOBIN`, normally `$(go env GOPATH)/bin`;
add that directory to PATH. `go run ./cmd/jira-connect --help` also works from
the checkout without installing.

## Set up

Keep working values in ignored `config.local.json`; use the fictional
[config.example.json](config.example.json) as the template for a new checkout.
Select credential references using the [configuration guide](docs/cli.md#configure). Configuration
is explicit; generic commands do not load jirafs or jiggit settings.

## First run

```sh
.bin/jira-connect --config config.local.json versions EXAMPLE
```

This is a live read when run by you. For external agents, follow
[MCP connection and calling instructions](docs/mcp.md).

## Use

- [Read/search issues and releases, preview/apply changes](docs/cli.md).
- [MCP tool reference for humans and agents](docs/mcp.md).
- [Use the Go library and find the versioned REST references](docs/datacenter-api.md).
- [Manually capture and review an Activity example](docs/activity-evidence.md).
- [Understand browser release-page API differences](docs/release-page-evidence.md).
- [Current work queue and deferred decisions](docs/tasks.md).
- [Opt-in jirafs Client bridge](docs/jirafs-bridge.md).
- [Opt-in jiggit read bridge](docs/jiggit-bridge.md).
- [Consumer compatibility evidence and migration gaps](docs/consumer-compatibility.md).
- [Implementation and verification history](docs/status.md).

## Develop

Go, Bash and Python 3.9+ are required for the offline checks:

```sh
bin/workflows
bin/lint
bin/test
```

See [maintainer workflows and releases](docs/workflows.md).
Workflow flags/selectors are documented by each command's help. Tests use fake
HTTP/credential providers and credential-free MCP subprocesses, including the
race detector. No live Jira or local listening port is required. The wrappers
originated from the maintainer-workflow skill and are maintained in this repo.

## Troubleshoot

Run `.bin/jira-connect --help`, then use the [error guide](docs/cli.md#handle-errors)
or [MCP connection checks](docs/mcp.md#connect).
