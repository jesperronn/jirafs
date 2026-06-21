// Package plan builds typed sync plans by comparing local and remote issue state.
package plan

import (
	"sort"
	"strings"

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
//
// When the local remote_version differs from the remote remote_version,
// BuildPlan reports a stale_remote_version conflict and returns no operations.
func BuildPlan(local, remote schema.Issue) ([]schema.PlanOperation, []schema.Conflict, error) {
	var ops []schema.PlanOperation
	var conflicts []schema.Conflict

	// Check for stale remote version.
	if local.RemoteMetadata.RemoteVersion != "" &&
		remote.RemoteMetadata.RemoteVersion != "" &&
		local.RemoteMetadata.RemoteVersion != remote.RemoteMetadata.RemoteVersion {
		conflicts = append(conflicts, schema.Conflict{
			Field:       schema.EditableFieldSummary,
			Type:        schema.ConflictRemoteDeleteLocalEdit,
			LocalValue:  local.RemoteMetadata.RemoteVersion,
			RemoteValue: remote.RemoteMetadata.RemoteVersion,
		})
		return ops, conflicts, nil
	}

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

	// Compare labels.
	if !labelsEqual(local.Labels, remote.Labels) {
		ops = append(ops, schema.PlanOperation{
			Field: schema.EditableFieldLabels,
			Type:  schema.OpSet,
			Value: labelsToString(local.Labels),
		})
	}

	// Compare assignee.
	if assigneeString(local.Assignee) != assigneeString(remote.Assignee) {
		ops = append(ops, schema.PlanOperation{
			Field: schema.EditableFieldAssignee,
			Type:  schema.OpSet,
			Value: assigneeString(local.Assignee),
		})
	}

	// Compare status.
	if local.Status != remote.Status {
		ops = append(ops, schema.PlanOperation{
			Field: schema.EditableFieldStatus,
			Type:  schema.OpSet,
			Value: local.Status,
		})
	}

	// Compare sprint.
	if local.Sprint != remote.Sprint {
		ops = append(ops, schema.PlanOperation{
			Field: schema.EditableFieldSprint,
			Type:  schema.OpSet,
			Value: local.Sprint,
		})
	}

	// Compare fix versions.
	if !fixVersionsEqual(local.FixVersions, remote.FixVersions) {
		ops = append(ops, schema.PlanOperation{
			Field: schema.EditableFieldFixVersions,
			Type:  schema.OpSet,
			Value: fixVersionsToString(local.FixVersions),
		})
	}

	return ops, conflicts, nil
}

// labelsToString converts a slice of labels to a comma-separated string.
// Labels are sorted for deterministic comparison.
func labelsToString(labels []string) string {
	sorted := make([]string, len(labels))
	copy(sorted, labels)
	sort.Strings(sorted)
	return strings.Join(sorted, ",")
}

// labelsEqual compares two label slices for equality, ignoring order.
func labelsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	sortedA := make([]string, len(a))
	copy(sortedA, a)
	sort.Strings(sortedA)
	sortedB := make([]string, len(b))
	copy(sortedB, b)
	sort.Strings(sortedB)
	for i := range sortedA {
		if sortedA[i] != sortedB[i] {
			return false
		}
	}
	return true
}

// assigneeString converts an assignee pointer to a string.
func assigneeString(a *string) string {
	if a == nil {
		return ""
	}
	return *a
}

// fixVersionsToString converts a slice of fix versions to a comma-separated string.
// Fix versions are sorted for deterministic comparison.
func fixVersionsToString(versions []string) string {
	sorted := make([]string, len(versions))
	copy(sorted, versions)
	sort.Strings(sorted)
	return strings.Join(sorted, ",")
}

// fixVersionsEqual compares two fix version slices for equality, ignoring order.
func fixVersionsEqual(a, b []string) bool {
	return labelsEqual(a, b)
}
