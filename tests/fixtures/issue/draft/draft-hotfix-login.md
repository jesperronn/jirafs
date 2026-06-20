---
schema: jirafs/jira-issue-v1
remote:
  jira_base_url: https://jira.example.com
  issue_id: null
  issue_key: null
sync:
  mode: create
workflow:
  project: XYZ
  issue_type: Bug
template: bug
relations:
  assignee: null
  reporter: user:jesper
  parent: null
  epic: null
  sprint: null
  fix_versions: []
  affects_versions: []
  labels: []
  components: []
links:
  blocks: []
  blocked_by: []
  relates_to: []
  duplicates: []
permissions:
  editable:
    - summary
    - description
    - labels
    - fix_versions
    - assignee
    - parent
    - epic
    - sprint
  append_only:
    - comments
  read_only:
    - reporter
    - created
    - status
---

# draft-hotfix-login-timeout

## Description

Users report intermittent login timeouts during peak hours. Investigation
needed on session store configuration.

## Acceptance Criteria

- Reproduce the timeout locally
- Identify root cause in session middleware
- Add fix and regression test

## Comments To Add

- Assigned to on-call engineer for initial triage.
