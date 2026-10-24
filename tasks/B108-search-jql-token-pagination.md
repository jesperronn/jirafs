# B108: Use Jira Cloud `/rest/api/3/search/jql` with token pagination.

## Dependencies

B107

## Owned paths

`internal/jira/**`

## Context

`SearchIssues` POSTs to `/rest/api/2/search` and pages with `startAt` / `maxResults 50` (`internal/jira/client.go:405-425`), reading `Total` from the response (`client.go:132-134, 459`). Atlassian is removing the old `/search` endpoints on Cloud in favour of `/rest/api/3/search/jql`, which has no `startAt`/`total` and pages with `nextPageToken`. A whole-project mirror (B107) cannot depend on the old endpoint.

## Acceptance

- New request: `POST /rest/api/3/search/jql` with `{"jql", "fields", "maxResults", "nextPageToken"?}`; loop until the response has no `nextPageToken` (or `isLast: true`).
- The `expand: names` handling for custom fields (`client.go:422-424`) keeps working or is replaced by the equivalent in the new endpoint; custom fields still land in `custom_fields`.
- Fallback: if the new endpoint answers 404/410 (Jira Server / Data Center, detectable also by `/rest/api/2/serverInfo` `deploymentType`), use the existing `/rest/api/2/search` `startAt` path. The choice is made once per client and cached.
- Signature stays `([]*schema.Issue, int, error)`; the int becomes the number of issues returned when `total` is unavailable.
- The fake transport (`FakeTransport`) learns the new endpoint, with a multi-page fixture.

## Verification

- `bin/verify` passes.
- New tests in `internal/jira/client_test.go`: three pages chained by `nextPageToken` return all issues in order; 404 on the new endpoint falls back to the v2 path and the second call does not retry the new endpoint; custom fields round-trip on both paths.
- Paste the request log of the multi-page test (method, path, token) showing the chaining.

## Rules

- No placeholder code, no TODO comments, no partial implementations.
- Add tests in the same task.
- Final gates: `bin/verify` must pass.
- Commit with conventional commit wording after gates pass.
- Move this file to `tasks/done/` as part of the same commit.
