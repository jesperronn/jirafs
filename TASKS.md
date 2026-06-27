# Tasks: Credential Display & Timeout Feature

## Overview
Implement credential display messaging and timeout handling for `jirafs sync` command with special support for 1password approval flow.

---

## Tasks

### 1. Implement credential display abstraction for 1password and other sources
Create a helper function that formats credential display messages:
- For 1password: `reading credential "op://JIRA_API_TOKEN_NINE_JRJ" waiting for 1password approval...`
- For other sources: `reading credential "[name]"...`
- Support dim/muted styling for the message

Should handle credential source detection and formatting consistently.

**Status:** pending

---

### 2. Add credential validation (non-empty, non-blank)
Validate that retrieved credentials are not empty or whitespace-only:
- Check credential value after retrieval
- Return validation result with clear error message
- Should be called before attempting to use the credential

Fail fast with a clear error if validation fails.

**Status:** pending

---

### 3. Implement 30-second timeout for credential retrieval
Add timeout mechanism for credential operations:
- 30-second timeout for all credential operations
- Cancel/abort if timeout is exceeded
- Return timeout error with special handling for 1password (user approval may have been denied)

Should be reusable across different credential sources.

**Status:** pending

---

### 4. Create failure messaging with helpful next steps
Implement error message formatting for credential failures:
- Timeout scenario: suggest checking 1password app/prompt, trying again
- Empty credential: suggest checking credential configuration
- Other errors: generic "unable to retrieve credential" message

Messages should guide user toward resolution.

**Status:** pending

---

### 5. Integrate credential display & validation into sync command
Wire up the new credential handling into `jirafs sync`:
- Call credential display before retrieval
- Apply timeout to retrieval operation
- Validate credential after retrieval
- Show appropriate error messages on failure
- Exit with failure status if any validation fails

This ties together all the previous components.

**Depends on:** Tasks 1–4

**Status:** pending

---

### 6. Write tests for credential handling
Create test coverage for:
- Credential display formatting (1password vs other sources)
- Timeout behavior (success and timeout scenarios)
- Empty/blank credential validation
- Failure messaging (all error scenarios)
- Integration with sync command

Should cover happy path and all failure modes.

**Status:** pending

---
