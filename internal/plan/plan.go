// Package plan builds typed sync plans by comparing local and remote issue state.
package plan

import (
	"encoding/json"
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
	// A status change needs a known remote status: never transition blindly.
	if remote.Status != "" && local.Status != "" && !strings.EqualFold(local.Status, remote.Status) {
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

	// Compare priority; an empty local priority means "not set locally".
	if local.Context.Priority != "" && local.Context.Priority != remote.Context.Priority {
		ops = append(ops, schema.PlanOperation{
			Field: schema.EditableFieldPriority,
			Type:  schema.OpSet,
			Value: local.Context.Priority,
		})
	}

	// Compare custom fields: only keys present locally, only when changed.
	if changed := ChangedCustomFields(local.CustomFields, remote.CustomFields); len(changed) > 0 {
		if b, err := json.Marshal(changed); err == nil {
			ops = append(ops, schema.PlanOperation{
				Field: schema.EditableFieldCustomFields,
				Type:  schema.OpSet,
				Value: string(b),
			})
		}
	}

	return ops, conflicts, nil
}

// FieldValue returns the comparable string form of one editable field of
// issue: lists are sorted and comma-joined, an unset assignee is empty, and
// custom_fields is the JSON object of all custom fields. Fields without a
// value in this model (parent, epic) yield "".
func FieldValue(issue schema.Issue, field schema.EditableField) string {
	switch field {
	case schema.EditableFieldSummary:
		return issue.Summary
	case schema.EditableFieldDescription:
		return issue.Description
	case schema.EditableFieldLabels:
		return labelsToString(issue.Labels)
	case schema.EditableFieldAssignee:
		return assigneeString(issue.Assignee)
	case schema.EditableFieldStatus:
		if issue.Status != "" {
			return issue.Status
		}
		return issue.Context.Status
	case schema.EditableFieldSprint:
		return issue.Sprint
	case schema.EditableFieldFixVersions:
		return fixVersionsToString(issue.FixVersions)
	case schema.EditableFieldPriority:
		return issue.Context.Priority
	case schema.EditableFieldCustomFields:
		b, err := json.Marshal(issue.CustomFields)
		if err != nil {
			return ""
		}
		return string(b)
	}
	return ""
}

// BuildPlanWithBase is BuildPlan limited to what the local side changed: a
// field whose local value equals base is left out, so a change made only on
// the remote is never overwritten. A nil base builds the plain BuildPlan.
func BuildPlanWithBase(base *schema.Issue, local, remote schema.Issue) ([]schema.PlanOperation, []schema.Conflict, error) {
	ops, conflicts, err := BuildPlan(local, remote)
	if err != nil || base == nil {
		return ops, conflicts, err
	}
	kept := ops[:0]
	for _, op := range ops {
		if op.Field == schema.EditableFieldCustomFields {
			changed := ChangedCustomFields(ChangedCustomFields(local.CustomFields, base.CustomFields), remote.CustomFields)
			if len(changed) == 0 {
				continue
			}
			b, merr := json.Marshal(changed)
			if merr != nil {
				return nil, nil, merr
			}
			op.Value = string(b)
		} else if SameFieldValue(op.Field, FieldValue(local, op.Field), FieldValue(*base, op.Field)) {
			continue
		}
		kept = append(kept, op)
	}
	return kept, conflicts, nil
}

// SameFieldValue reports whether two FieldValue strings of field are equal;
// status compares case-insensitively like BuildPlan does.
func SameFieldValue(field schema.EditableField, a, b string) bool {
	if field == schema.EditableFieldStatus {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// ChangedCustomFields returns the entries of local whose JSON form differs
// from the same key in remote. Nil when nothing changed.
func ChangedCustomFields(local, remote map[string]interface{}) map[string]interface{} {
	var out map[string]interface{}
	for k, lv := range local {
		lb, _ := json.Marshal(lv)
		rv, ok := remote[k]
		rb, _ := json.Marshal(rv)
		if ok && string(lb) == string(rb) {
			continue
		}
		if !ok && string(lb) == "null" {
			continue
		}
		if out == nil {
			out = map[string]interface{}{}
		}
		out[k] = lv
	}
	return out
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
