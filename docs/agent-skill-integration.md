# Agent Integration

## Goal

Agents should help with drafting, summarizing, and analysis without owning the
core Jira sync logic.

`jirafs` remains CLI-first.

## Agent Responsibilities

Good agent tasks:

- generate a draft issue from chat or notes
- suggest template choice
- suggest labels, assignee, fix version, sprint, or epic
- summarize sync plans
- analyze archive data for recurring process gaps
- propose stronger Definition of Ready or acceptance criteria

## Non-Agent Responsibilities

The agent layer should not be the sole owner of:

- Jira auth
- Jira HTTP operations
- schema validation
- reference resolution
- conflict detection
- sync application

Those belong in the CLI and core library.

## Example Flows

### Draft From Conversation

1. Agent receives unstructured text.
2. Agent selects a template.
3. Agent fills a draft Markdown issue file.
4. User reviews or edits locally.
5. `jirafs plan` and `jirafs sync` handle the write path.

### Archive Analysis

1. Agent reads archived issue files and sprint snapshots.
2. Agent extracts repeated failure modes.
3. Agent proposes process changes or template updates.

## Skill Integration

A Codex skill for `jirafs` should:

- call CLI commands
- interpret plan output
- generate drafts against real templates
- avoid bypassing the CLI for writes

This preserves one source of truth for behavior and reduces divergence between
interactive agent use and ordinary terminal use.
