# Changelog

## [Unreleased]

- Add generic Data Center issue/search/release reads, JSON CLI and credential references.
- Add explicit field/link writes with previews and a stdio MCP interface.
- Add offline verification and maintainer workflows.
- Add a build command for both executables and list workflows in lifecycle order.
- Add a manual read-only MCP live test for search, issue fetching and releases.
- Add verbose live-test diagnostics and local Story/Activity/release selections with capture comparisons.
- Colorize live-test terminal output and reject incomplete searches, captures and dropped configured fields.
- Save private successful-read JSON and compact previews in verbose live tests.
- Preserve standard consumer read fields and expanded names through library, CLI and MCP without losing null/empty values or numeric precision.
- Add an opt-in jiggit read bridge with config/response translation, complete search pagination and offline consumer regression tests.
- Add safe server-side fix-version add/remove through the library, CLI and MCP, with credential-free previews and explicit apply.
- Add Data Center metadata, creation/comment/transition operations, real Agile sprint reads, and a complete opt-in jirafs Client bridge.
- Expand the MCP interface to 22 tools with a checked machine-readable specification.
