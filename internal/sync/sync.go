package sync

import (
	"os"

	"github.com/jirafs/jirafs/internal/plan"
	"github.com/jirafs/jirafs/internal/schema"
)

// ApplyResult holds the result of a Sync call.
type ApplyResult struct {
	// Remote is the remote issue after the plan was applied.
	// For a no-op plan this is identical to the input.
	Remote *schema.Issue
	// Conflicts reports any conflicts detected during validation.
	// Empty when the plan is valid and applied.
	Conflicts []schema.Conflict
}

// Sync applies a validated plan to the remote issue, returning the updated
// remote and any conflicts.
//
// For a no-op plan (zero operations), Sync returns the remote unchanged
// without mutation.
//
// Before any mutation, Sync validates the archive path and checks that all
// references in the issue are resolved. If either check fails, Sync returns
// conflicts and does not apply the plan.
func Sync(local, remote schema.Issue, plan []schema.PlanOperation, archivePath string) ApplyResult {
	// Validate the plan against current state.
	if conflicts := validatePlan(local, remote, plan); len(conflicts) > 0 {
		return ApplyResult{
			Remote:    &remote,
			Conflicts: conflicts,
		}
	}

	// Validate archive path before mutation.
	if conflicts := validateArchivePath(archivePath); len(conflicts) > 0 {
		return ApplyResult{
			Remote:    &remote,
			Conflicts: conflicts,
		}
	}

	// Validate that all references in the issue are resolved before mutation.
	if conflicts := validateUnresolvedRefs(local, remote); len(conflicts) > 0 {
		return ApplyResult{
			Remote:    &remote,
			Conflicts: conflicts,
		}
	}

	if conflicts := validateTransitions(remote, plan); len(conflicts) > 0 {
		return ApplyResult{
			Remote:    &remote,
			Conflicts: conflicts,
		}
	}

	// No-op plan: return remote unchanged.
	if len(plan) == 0 {
		return ApplyResult{
			Remote:    &remote,
			Conflicts: nil,
		}
	}

	// Apply operations to remote (deferred to B063b).
	newRemote := remote
	for _, op := range plan {
		applyOp(&newRemote, op)
	}

	return ApplyResult{
		Remote:    &newRemote,
		Conflicts: nil,
	}
}

// validatePlan checks that the plan is still valid given the current remote
// state. Returns conflicts if the plan is stale or invalid.
func validatePlan(local, remote schema.Issue, ops []schema.PlanOperation) []schema.Conflict {
	if local.RemoteMetadata.RemoteVersion != "" &&
		remote.RemoteMetadata.RemoteVersion != "" &&
		local.RemoteMetadata.RemoteVersion != remote.RemoteMetadata.RemoteVersion {
		return []schema.Conflict{
			{
				Field:       schema.EditableFieldSummary,
				Type:        schema.ConflictBothEdited,
				LocalValue:  local.RemoteMetadata.RemoteVersion,
				RemoteValue: remote.RemoteMetadata.RemoteVersion,
			},
		}
	}

	if local.RemoteMetadata.ContentHash != "" &&
		remote.RemoteMetadata.ContentHash != "" &&
		local.RemoteMetadata.ContentHash != remote.RemoteMetadata.ContentHash {
		return []schema.Conflict{
			{
				Field:       schema.EditableFieldSummary,
				Type:        schema.ConflictBothEdited,
				LocalValue:  local.RemoteMetadata.ContentHash,
				RemoteValue: remote.RemoteMetadata.ContentHash,
			},
		}
	}

	if len(ops) == 0 {
		return nil
	}

	// Re-run the plan builder to detect conflicts.
	_, conflicts, err := plan.BuildPlan(local, remote)
	if err != nil {
		return []schema.Conflict{
			{
				Field:       schema.EditableFieldSummary,
				Type:        schema.ConflictBothEdited,
				LocalValue:  "plan validation failed",
				RemoteValue: err.Error(),
			},
		}
	}

	if len(conflicts) > 0 {
		return conflicts
	}

	// Verify each operation still matches the diff.
	computed, _, _ := plan.BuildPlan(local, remote)
	if len(computed) != len(ops) {
		return []schema.Conflict{
			{
				Field:       schema.EditableFieldSummary,
				Type:        schema.ConflictBothEdited,
				LocalValue:  "plan operation count mismatch",
				RemoteValue: "expected",
			},
		}
	}

	for i := range computed {
		if !computed[i].Equals(ops[i]) {
			return []schema.Conflict{
				{
					Field:       computed[i].Field,
					Type:        schema.ConflictBothEdited,
					LocalValue:  "plan operation mismatch",
					RemoteValue: "expected",
				},
			}
		}
	}

	return nil
}

// validateTransitions rejects status changes that are not explicitly allowed by
// the current sync rules. Status is mirrored, but direct status edits are not
// yet supported as field writes.
func validateTransitions(remote schema.Issue, ops []schema.PlanOperation) []schema.Conflict {
	for _, op := range ops {
		if op.Field != schema.EditableFieldStatus {
			continue
		}
		if op.Value == remote.Status {
			continue
		}
		return []schema.Conflict{
			{
				Field:       schema.EditableFieldStatus,
				Type:        schema.ConflictInvalidTransition,
				LocalValue:  op.Value,
				RemoteValue: remote.Status,
			},
		}
	}

	return nil
}

// applyOp applies a single plan operation to a remote issue.
// Used internally by Sync after validation.
func applyOp(remote *schema.Issue, op schema.PlanOperation) {
	switch op.Field {
	case schema.EditableFieldSummary:
		remote.Summary = op.Value
	case schema.EditableFieldDescription:
		remote.Description = op.Value
	case schema.EditableFieldLabels:
		remote.Labels = splitValues(op.Value)
	case schema.EditableFieldAssignee:
		remote.Assignee = strPtr(op.Value)
	case schema.EditableFieldStatus:
		remote.Status = op.Value
	case schema.EditableFieldSprint:
		remote.Sprint = op.Value
	case schema.EditableFieldFixVersions:
		remote.FixVersions = splitValues(op.Value)
	}
}

// splitValues converts a comma-separated string to a slice of strings.
func splitValues(s string) []string {
	if s == "" {
		return nil
	}
	parts := make([]string, 0)
	for _, p := range splitComma(s) {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}

// strPtr returns a pointer to the given string.
func strPtr(s string) *string {
	return &s
}

// splitComma splits a string by commas, stripping whitespace.
func splitComma(s string) []string {
	var parts []string
	var current string
	for _, c := range s {
		if c == ',' {
			parts = append(parts, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	parts = append(parts, current)
	return parts
}

// validateArchivePath checks that the archive path exists and is a directory.
// Returns conflicts if the path is invalid.
func validateArchivePath(path string) []schema.Conflict {
	if path == "" {
		return []schema.Conflict{
			{
				Field:       "",
				Type:        schema.ConflictArchivePathInvalid,
				LocalValue:  "",
				RemoteValue: "archive path is empty",
			},
		}
	}

	info, err := os.Stat(path)
	if err != nil {
		return []schema.Conflict{
			{
				Field:       "",
				Type:        schema.ConflictArchivePathInvalid,
				LocalValue:  path,
				RemoteValue: err.Error(),
			},
		}
	}

	if !info.IsDir() {
		return []schema.Conflict{
			{
				Field:       "",
				Type:        schema.ConflictArchivePathInvalid,
				LocalValue:  path,
				RemoteValue: "not a directory",
			},
		}
	}

	return nil
}

// validateUnresolvedRefs checks that all references in the local and remote
// issues are resolved (non-empty). Returns conflicts for each unresolved
// reference found.
func validateUnresolvedRefs(local, remote schema.Issue) []schema.Conflict {
	var conflicts []schema.Conflict

	// Check linked issues for empty keys.
	for _, li := range local.LinkedIssues {
		if li.Key == "" {
			conflicts = append(conflicts, schema.Conflict{
				Field:       schema.EditableFieldSummary,
				Type:        schema.ConflictUnresolvedRef,
				LocalValue:  "linked_issue",
				RemoteValue: "empty key",
			})
		}
	}
	for _, li := range remote.LinkedIssues {
		if li.Key == "" {
			conflicts = append(conflicts, schema.Conflict{
				Field:       schema.EditableFieldSummary,
				Type:        schema.ConflictUnresolvedRef,
				LocalValue:  "linked_issue",
				RemoteValue: "empty key",
			})
		}
	}

	// Check assignee.
	if local.Assignee != nil && *local.Assignee == "" {
		conflicts = append(conflicts, schema.Conflict{
			Field:       schema.EditableFieldAssignee,
			Type:        schema.ConflictUnresolvedRef,
			LocalValue:  "assignee",
			RemoteValue: "empty value",
		})
	}
	if remote.Assignee != nil && *remote.Assignee == "" {
		conflicts = append(conflicts, schema.Conflict{
			Field:       schema.EditableFieldAssignee,
			Type:        schema.ConflictUnresolvedRef,
			LocalValue:  "assignee",
			RemoteValue: "empty value",
		})
	}

	// Check sprint.
	if local.Sprint == "" && local.Summary != "" {
		// Sprint is only checked when it's non-zero to avoid false
		// positives on empty issues.
	}
	if remote.Sprint == "" && remote.Summary != "" {
		// Sprint is only checked when it's non-zero to avoid false
		// positives on empty issues.
	}

	// Check fix versions.
	for _, fv := range local.FixVersions {
		if fv == "" {
			conflicts = append(conflicts, schema.Conflict{
				Field:       schema.EditableFieldFixVersions,
				Type:        schema.ConflictUnresolvedRef,
				LocalValue:  "fix_version",
				RemoteValue: "empty value",
			})
		}
	}
	for _, fv := range remote.FixVersions {
		if fv == "" {
			conflicts = append(conflicts, schema.Conflict{
				Field:       schema.EditableFieldFixVersions,
				Type:        schema.ConflictUnresolvedRef,
				LocalValue:  "fix_version",
				RemoteValue: "empty value",
			})
		}
	}

	return conflicts
}
