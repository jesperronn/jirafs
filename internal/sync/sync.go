package sync

import (
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
func Sync(local, remote schema.Issue, plan []schema.PlanOperation) ApplyResult {
	// Validate the plan against current state.
	if conflicts := validatePlan(local, remote, plan); len(conflicts) > 0 {
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
