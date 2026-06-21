// Package plan builds typed sync plans by comparing local and remote issue state.
package plan

import (
	"github.com/jirafs/jirafs/internal/schema"
)

// BuildPlan compares local and remote issue state and returns the operations
// needed to bring the remote in line with local, plus any conflicts detected.
//
// When both summary and description are unchanged between local and remote,
// BuildPlan returns an empty plan (no operations, no conflicts).
//
// When summary or description differ, BuildPlan produces typed PlanOperation
// entries describing the required changes.
func BuildPlan(local, remote schema.Issue) ([]schema.PlanOperation, []schema.Conflict, error) {
	var ops []schema.PlanOperation
	var conflicts []schema.Conflict

	// Compare summary.
	if local.Summary != remote.Summary {
		ops = append(ops, schema.PlanOperation{
			Field: schema.EditableFieldSummary,
			Type:  schema.OpSet,
			Value: local.Summary,
		})
	}

	// Compare description.
	if local.Description != remote.Description {
		ops = append(ops, schema.PlanOperation{
			Field: schema.EditableFieldDescription,
			Type:  schema.OpSet,
			Value: local.Description,
		})
	}

	return ops, conflicts, nil
}
