package plan

import (
	"testing"

	"github.com/jirafs/jirafs/internal/schema"
)

func TestBuildPlan_unchangedSummaryAndDescription(t *testing.T) {
	// B060a: unchanged local/remote summary and description produce empty plan.
	local := schema.Issue{
		Summary:     "Test issue",
		Description: "Some description",
	}
	remote := schema.Issue{
		Summary:     "Test issue",
		Description: "Some description",
	}

	ops, conflicts, err := BuildPlan(local, remote)
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}

	if len(ops) != 0 {
		t.Errorf("expected 0 operations, got %d: %v", len(ops), ops)
	}

	if len(conflicts) != 0 {
		t.Errorf("expected 0 conflicts, got %d: %v", len(conflicts), conflicts)
	}
}

func TestBuildPlan_emptyIssues(t *testing.T) {
	// Both zero-value issues should also produce an empty plan.
	var local, remote schema.Issue

	ops, conflicts, err := BuildPlan(local, remote)
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}

	if len(ops) != 0 {
		t.Errorf("expected 0 operations, got %d: %v", len(ops), ops)
	}

	if len(conflicts) != 0 {
		t.Errorf("expected 0 conflicts, got %d: %v", len(conflicts), conflicts)
	}
}

func TestBuildPlan_unchangedSummaryDifferentDescription(t *testing.T) {
	local := schema.Issue{
		Summary:     "Test issue",
		Description: "Old description",
	}
	remote := schema.Issue{
		Summary:     "Test issue",
		Description: "New description",
	}

	ops, conflicts, err := BuildPlan(local, remote)
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}

	if len(ops) != 1 {
		t.Fatalf("expected 1 operation, got %d: %v", len(ops), ops)
	}

	op := ops[0]
	if op.Field != schema.EditableFieldDescription {
		t.Errorf("expected field %q, got %q", schema.EditableFieldDescription, op.Field)
	}
	if op.Type != schema.OpSet {
		t.Errorf("expected type %q, got %q", schema.OpSet, op.Type)
	}
	if op.Value != "Old description" {
		t.Errorf("expected value %q, got %q", "Old description", op.Value)
	}

	if len(conflicts) != 0 {
		t.Errorf("expected 0 conflicts, got %d: %v", len(conflicts), conflicts)
	}
}

func TestBuildPlan_differentSummaryUnchangedDescription(t *testing.T) {
	local := schema.Issue{
		Summary:     "New title",
		Description: "Some description",
	}
	remote := schema.Issue{
		Summary:     "Old title",
		Description: "Some description",
	}

	ops, conflicts, err := BuildPlan(local, remote)
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}

	if len(ops) != 1 {
		t.Fatalf("expected 1 operation, got %d: %v", len(ops), ops)
	}

	op := ops[0]
	if op.Field != schema.EditableFieldSummary {
		t.Errorf("expected field %q, got %q", schema.EditableFieldSummary, op.Field)
	}
	if op.Type != schema.OpSet {
		t.Errorf("expected type %q, got %q", schema.OpSet, op.Type)
	}
	if op.Value != "New title" {
		t.Errorf("expected value %q, got %q", "New title", op.Value)
	}

	if len(conflicts) != 0 {
		t.Errorf("expected 0 conflicts, got %d: %v", len(conflicts), conflicts)
	}
}

func TestBuildPlan_unchangedRefsAndMetadata(t *testing.T) {
	// B060b: unchanged refs and metadata produce empty plan.
	assignee := "jdoe"
	local := schema.Issue{
		Summary:     "Test issue",
		Description: "Some description",
		Labels:      []string{"bug", "priority"},
		Assignee:    &assignee,
		LinkedIssues: []schema.LinkedIssue{
			{Key: "PROJ-1", Type: "blocks"},
		},
		RemoteMetadata: schema.RemoteMetadata{
			RemoteVersion: "1",
			ContentHash:   "abc123",
		},
	}
	remote := schema.Issue{
		Summary:     "Test issue",
		Description: "Some description",
		Labels:      []string{"bug", "priority"},
		Assignee:    &assignee,
		LinkedIssues: []schema.LinkedIssue{
			{Key: "PROJ-1", Type: "blocks"},
		},
		RemoteMetadata: schema.RemoteMetadata{
			RemoteVersion: "1",
			ContentHash:   "abc123",
		},
	}

	ops, conflicts, err := BuildPlan(local, remote)
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}

	if len(ops) != 0 {
		t.Errorf("expected 0 operations, got %d: %v", len(ops), ops)
	}

	if len(conflicts) != 0 {
		t.Errorf("expected 0 conflicts, got %d: %v", len(conflicts), conflicts)
	}
}

func TestBuildPlan_differingRefsMetadataNoOps(t *testing.T) {
	// B060b: differing refs and metadata do not produce operations
	// when summary, description, labels, and assignee are unchanged
	// and remote versions match.
	assignee := "jdoe"
	local := schema.Issue{
		Summary:     "Test issue",
		Description: "Some description",
		Labels:      []string{"bug"},
		Assignee:    &assignee,
		LinkedIssues: []schema.LinkedIssue{
			{Key: "PROJ-1", Type: "blocks"},
		},
		RemoteMetadata: schema.RemoteMetadata{
			RemoteVersion: "1",
			ContentHash:   "abc123",
		},
	}
	remote := schema.Issue{
		Summary:     "Test issue",
		Description: "Some description",
		Labels:      []string{"bug"},
		Assignee:    &assignee,
		LinkedIssues: []schema.LinkedIssue{
			{Key: "PROJ-2", Type: "relates to"},
		},
		RemoteMetadata: schema.RemoteMetadata{
			RemoteVersion: "1", // same version
			ContentHash:   "abc123", // same content hash
		},
	}

	ops, conflicts, err := BuildPlan(local, remote)
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}

	if len(ops) != 0 {
		t.Errorf("expected 0 operations (refs/metadata diffs ignored), got %d: %v", len(ops), ops)
	}

	if len(conflicts) != 0 {
		t.Errorf("expected 0 conflicts, got %d: %v", len(conflicts), conflicts)
	}
}

func TestBuildPlan_bothChanged(t *testing.T) {
	local := schema.Issue{
		Summary:     "New title",
		Description: "New description",
	}
	remote := schema.Issue{
		Summary:     "Old title",
		Description: "Old description",
	}

	ops, conflicts, err := BuildPlan(local, remote)
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}

	if len(ops) != 2 {
		t.Fatalf("expected 2 operations, got %d: %v", len(ops), ops)
	}

	// Verify both operations are present.
	var hasSummary, hasDescription bool
	for _, op := range ops {
		if op.Field == schema.EditableFieldSummary {
			hasSummary = true
			if op.Value != "New title" {
				t.Errorf("summary op value = %q, want %q", op.Value, "New title")
			}
		}
		if op.Field == schema.EditableFieldDescription {
			hasDescription = true
			if op.Value != "New description" {
				t.Errorf("description op value = %q, want %q", op.Value, "New description")
			}
		}
	}

	if !hasSummary {
		t.Error("missing summary operation")
	}
	if !hasDescription {
		t.Error("missing description operation")
	}

	if len(conflicts) != 0 {
		t.Errorf("expected 0 conflicts, got %d: %v", len(conflicts), conflicts)
	}
}

func TestBuildPlan_labelsChange(t *testing.T) {
	// B061b: labels change produces a typed operation.
	local := schema.Issue{
		Summary:  "Test issue",
		Labels:   []string{"bug", "priority"},
	}
	remote := schema.Issue{
		Summary:  "Test issue",
		Labels:   []string{"bug"},
	}

	ops, conflicts, err := BuildPlan(local, remote)
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}

	if len(ops) != 1 {
		t.Fatalf("expected 1 operation, got %d: %v", len(ops), ops)
	}

	op := ops[0]
	if op.Field != schema.EditableFieldLabels {
		t.Errorf("expected field %q, got %q", schema.EditableFieldLabels, op.Field)
	}
	if op.Type != schema.OpSet {
		t.Errorf("expected type %q, got %q", schema.OpSet, op.Type)
	}
	// Labels are sorted: "bug,priority"
	if op.Value != "bug,priority" {
		t.Errorf("expected value %q, got %q", "bug,priority", op.Value)
	}

	if len(conflicts) != 0 {
		t.Errorf("expected 0 conflicts, got %d: %v", len(conflicts), conflicts)
	}
}

func TestBuildPlan_labelsUnchanged(t *testing.T) {
	// B061b: unchanged labels produce no operation.
	local := schema.Issue{
		Summary: "Test issue",
		Labels:  []string{"priority", "bug"},
	}
	remote := schema.Issue{
		Summary: "Test issue",
		Labels:  []string{"bug", "priority"},
	}

	ops, _, err := BuildPlan(local, remote)
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}

	if len(ops) != 0 {
		t.Errorf("expected 0 operations for unchanged labels, got %d: %v", len(ops), ops)
	}
}

func TestBuildPlan_assigneeChange(t *testing.T) {
	// B061b: assignee change produces a typed operation.
	localAssignee := "jdoe"
	remoteAssignee := "jsmith"
	local := schema.Issue{
		Summary:  "Test issue",
		Assignee: &localAssignee,
	}
	remote := schema.Issue{
		Summary:  "Test issue",
		Assignee: &remoteAssignee,
	}

	ops, conflicts, err := BuildPlan(local, remote)
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}

	if len(ops) != 1 {
		t.Fatalf("expected 1 operation, got %d: %v", len(ops), ops)
	}

	op := ops[0]
	if op.Field != schema.EditableFieldAssignee {
		t.Errorf("expected field %q, got %q", schema.EditableFieldAssignee, op.Field)
	}
	if op.Type != schema.OpSet {
		t.Errorf("expected type %q, got %q", schema.OpSet, op.Type)
	}
	if op.Value != "jdoe" {
		t.Errorf("expected value %q, got %q", "jdoe", op.Value)
	}

	if len(conflicts) != 0 {
		t.Errorf("expected 0 conflicts, got %d: %v", len(conflicts), conflicts)
	}
}

func TestBuildPlan_assigneeNilToLocal(t *testing.T) {
	// B061b: assignee from nil to a value produces a typed operation.
	localAssignee := "jdoe"
	local := schema.Issue{
		Summary:  "Test issue",
		Assignee: &localAssignee,
	}
	remote := schema.Issue{
		Summary:  "Test issue",
		Assignee: nil,
	}

	ops, _, err := BuildPlan(local, remote)
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}

	if len(ops) != 1 {
		t.Fatalf("expected 1 operation, got %d: %v", len(ops), ops)
	}

	op := ops[0]
	if op.Field != schema.EditableFieldAssignee {
		t.Errorf("expected field %q, got %q", schema.EditableFieldAssignee, op.Field)
	}
	if op.Value != "jdoe" {
		t.Errorf("expected value %q, got %q", "jdoe", op.Value)
	}
}

func TestBuildPlan_statusChange(t *testing.T) {
	// B061b: status change produces a typed operation.
	local := schema.Issue{
		Summary: "Test issue",
		Status:  "In Progress",
	}
	remote := schema.Issue{
		Summary: "Test issue",
		Status:  "To Do",
	}

	ops, conflicts, err := BuildPlan(local, remote)
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}

	if len(ops) != 1 {
		t.Fatalf("expected 1 operation, got %d: %v", len(ops), ops)
	}

	op := ops[0]
	if op.Field != schema.EditableFieldStatus {
		t.Errorf("expected field %q, got %q", schema.EditableFieldStatus, op.Field)
	}
	if op.Type != schema.OpSet {
		t.Errorf("expected type %q, got %q", schema.OpSet, op.Type)
	}
	if op.Value != "In Progress" {
		t.Errorf("expected value %q, got %q", "In Progress", op.Value)
	}

	if len(conflicts) != 0 {
		t.Errorf("expected 0 conflicts, got %d: %v", len(conflicts), conflicts)
	}
}

func TestBuildPlan_sprintChange(t *testing.T) {
	// B061b: sprint change produces a typed operation.
	local := schema.Issue{
		Summary: "Test issue",
		Sprint:  "Sprint 42",
	}
	remote := schema.Issue{
		Summary: "Test issue",
		Sprint:  "Sprint 41",
	}

	ops, conflicts, err := BuildPlan(local, remote)
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}

	if len(ops) != 1 {
		t.Fatalf("expected 1 operation, got %d: %v", len(ops), ops)
	}

	op := ops[0]
	if op.Field != schema.EditableFieldSprint {
		t.Errorf("expected field %q, got %q", schema.EditableFieldSprint, op.Field)
	}
	if op.Type != schema.OpSet {
		t.Errorf("expected type %q, got %q", schema.OpSet, op.Type)
	}
	if op.Value != "Sprint 42" {
		t.Errorf("expected value %q, got %q", "Sprint 42", op.Value)
	}

	if len(conflicts) != 0 {
		t.Errorf("expected 0 conflicts, got %d: %v", len(conflicts), conflicts)
	}
}

func TestBuildPlan_fixVersionsChange(t *testing.T) {
	// B061b: fix-version change produces a typed operation.
	local := schema.Issue{
		Summary:     "Test issue",
		FixVersions: []string{"1.0", "2.0"},
	}
	remote := schema.Issue{
		Summary:     "Test issue",
		FixVersions: []string{"1.0"},
	}

	ops, conflicts, err := BuildPlan(local, remote)
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}

	if len(ops) != 1 {
		t.Fatalf("expected 1 operation, got %d: %v", len(ops), ops)
	}

	op := ops[0]
	if op.Field != schema.EditableFieldFixVersions {
		t.Errorf("expected field %q, got %q", schema.EditableFieldFixVersions, op.Field)
	}
	if op.Type != schema.OpSet {
		t.Errorf("expected type %q, got %q", schema.OpSet, op.Type)
	}
	// Fix versions are sorted: "1.0,2.0"
	if op.Value != "1.0,2.0" {
		t.Errorf("expected value %q, got %q", "1.0,2.0", op.Value)
	}

	if len(conflicts) != 0 {
		t.Errorf("expected 0 conflicts, got %d: %v", len(conflicts), conflicts)
	}
}

func TestBuildPlan_allFieldsChanged(t *testing.T) {
	// B061b: multiple field changes produce multiple operations.
	localAssignee := "jdoe"
	local := schema.Issue{
		Summary:     "New title",
		Description: "New desc",
		Labels:      []string{"bug", "priority"},
		Assignee:    &localAssignee,
		Status:      "In Progress",
		Sprint:      "Sprint 42",
		FixVersions: []string{"1.0", "2.0"},
	}
	remoteAssignee := "jsmith"
	remote := schema.Issue{
		Summary:     "Old title",
		Description: "Old desc",
		Labels:      []string{"enhancement"},
		Assignee:    &remoteAssignee,
		Status:      "To Do",
		Sprint:      "Sprint 41",
		FixVersions: []string{"1.0"},
	}

	ops, conflicts, err := BuildPlan(local, remote)
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}

	// All 7 fields changed.
	if len(ops) != 7 {
		t.Fatalf("expected 7 operations, got %d: %v", len(ops), ops)
	}

	// Verify all field types are present.
	fieldTypes := make(map[schema.EditableField]bool)
	for _, op := range ops {
		fieldTypes[op.Field] = true
		if op.Type != schema.OpSet {
			t.Errorf("expected type %q for field %q, got %q", schema.OpSet, op.Field, op.Type)
		}
	}

	expectedFields := []schema.EditableField{
		schema.EditableFieldSummary,
		schema.EditableFieldDescription,
		schema.EditableFieldLabels,
		schema.EditableFieldAssignee,
		schema.EditableFieldStatus,
		schema.EditableFieldSprint,
		schema.EditableFieldFixVersions,
	}
	for _, ef := range expectedFields {
		if !fieldTypes[ef] {
			t.Errorf("missing operation for field %q", ef)
		}
	}

	if len(conflicts) != 0 {
		t.Errorf("expected 0 conflicts, got %d: %v", len(conflicts), conflicts)
	}
}

func TestBuildPlan_staleRemoteVersion(t *testing.T) {
	// B062a: stale remote version produces conflict, not operations.
	local := schema.Issue{
		Summary: "New title",
		RemoteMetadata: schema.RemoteMetadata{
			RemoteVersion: "42",
			ContentHash:   "abc123",
		},
	}
	remote := schema.Issue{
		Summary: "Old title",
		RemoteMetadata: schema.RemoteMetadata{
			RemoteVersion: "43", // stale: remote was updated
			ContentHash:   "def456",
		},
	}

	ops, conflicts, err := BuildPlan(local, remote)
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}

	if len(ops) != 0 {
		t.Errorf("expected 0 operations for stale remote, got %d: %v", len(ops), ops)
	}

	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict for stale remote, got %d: %v", len(conflicts), conflicts)
	}

	c := conflicts[0]
	if c.Type != schema.ConflictBothEdited {
		t.Errorf("conflict type = %q, want %q", c.Type, schema.ConflictBothEdited)
	}
	if c.LocalValue != "42" {
		t.Errorf("conflict local value = %q, want %q", c.LocalValue, "42")
	}
	if c.RemoteValue != "43" {
		t.Errorf("conflict remote value = %q, want %q", c.RemoteValue, "43")
	}
}

func TestBuildPlan_matchingRemoteVersionNoConflict(t *testing.T) {
	// B062a: matching remote version should not produce a conflict.
	local := schema.Issue{
		Summary: "New title",
		RemoteMetadata: schema.RemoteMetadata{
			RemoteVersion: "42",
			ContentHash:   "abc123",
		},
	}
	remote := schema.Issue{
		Summary: "Old title",
		RemoteMetadata: schema.RemoteMetadata{
			RemoteVersion: "42", // matching
			ContentHash:   "abc123",
		},
	}

	ops, conflicts, err := BuildPlan(local, remote)
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}

	if len(ops) != 1 {
		t.Fatalf("expected 1 operation, got %d: %v", len(ops), ops)
	}

	if len(conflicts) != 0 {
		t.Errorf("expected 0 conflicts for matching version, got %d: %v", len(conflicts), conflicts)
	}
}

func TestBuildPlan_emptyLocalRemoteVersionNoConflict(t *testing.T) {
	// B062a: empty local remote version should not trigger conflict check.
	local := schema.Issue{
		Summary: "New title",
		RemoteMetadata: schema.RemoteMetadata{
			ContentHash: "abc123",
		},
	}
	remote := schema.Issue{
		Summary: "Old title",
		RemoteMetadata: schema.RemoteMetadata{
			RemoteVersion: "42",
			ContentHash:   "abc123",
		},
	}

	ops, conflicts, err := BuildPlan(local, remote)
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}

	if len(ops) != 1 {
		t.Fatalf("expected 1 operation, got %d: %v", len(ops), ops)
	}

	if len(conflicts) != 0 {
		t.Errorf("expected 0 conflicts when local has no remote version, got %d: %v", len(conflicts), conflicts)
	}
}

func TestBuildPlan_emptyRemoteRemoteVersionNoConflict(t *testing.T) {
	// B062a: empty remote remote version should not trigger conflict check.
	local := schema.Issue{
		Summary: "New title",
		RemoteMetadata: schema.RemoteMetadata{
			RemoteVersion: "42",
			ContentHash:   "abc123",
		},
	}
	remote := schema.Issue{
		Summary: "Old title",
		RemoteMetadata: schema.RemoteMetadata{
			ContentHash:   "abc123", // matching content hash
		},
	}

	ops, conflicts, err := BuildPlan(local, remote)
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}

	if len(ops) != 1 {
		t.Fatalf("expected 1 operation, got %d: %v", len(ops), ops)
	}

	if len(conflicts) != 0 {
		t.Errorf("expected 0 conflicts when remote has no remote version, got %d: %v", len(conflicts), conflicts)
	}
}

func TestBuildPlan_staleContentHash(t *testing.T) {
	// B062b: stale content hash produces conflict, not operations.
	local := schema.Issue{
		Summary: "New title",
		RemoteMetadata: schema.RemoteMetadata{
			RemoteVersion: "42",
			ContentHash:   "abc123",
		},
	}
	remote := schema.Issue{
		Summary: "Old title",
		RemoteMetadata: schema.RemoteMetadata{
			RemoteVersion: "42", // same version
			ContentHash:   "def456", // stale: content hash differs
		},
	}

	ops, conflicts, err := BuildPlan(local, remote)
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}

	if len(ops) != 0 {
		t.Errorf("expected 0 operations for stale content hash, got %d: %v", len(ops), ops)
	}

	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict for stale content hash, got %d: %v", len(conflicts), conflicts)
	}

	c := conflicts[0]
	if c.Type != schema.ConflictBothEdited {
		t.Errorf("conflict type = %q, want %q", c.Type, schema.ConflictBothEdited)
	}
	if c.LocalValue != "abc123" {
		t.Errorf("conflict local value = %q, want %q", c.LocalValue, "abc123")
	}
	if c.RemoteValue != "def456" {
		t.Errorf("conflict remote value = %q, want %q", c.RemoteValue, "def456")
	}
}

func TestBuildPlan_matchingContentHashNoConflict(t *testing.T) {
	// B062b: matching content hash should not produce a conflict.
	local := schema.Issue{
		Summary: "New title",
		RemoteMetadata: schema.RemoteMetadata{
			RemoteVersion: "42",
			ContentHash:   "abc123",
		},
	}
	remote := schema.Issue{
		Summary: "Old title",
		RemoteMetadata: schema.RemoteMetadata{
			RemoteVersion: "42",
			ContentHash:   "abc123", // matching
		},
	}

	ops, conflicts, err := BuildPlan(local, remote)
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}

	if len(ops) != 1 {
		t.Fatalf("expected 1 operation, got %d: %v", len(ops), ops)
	}

	if len(conflicts) != 0 {
		t.Errorf("expected 0 conflicts for matching content hash, got %d: %v", len(conflicts), conflicts)
	}
}

func TestBuildPlan_emptyLocalContentHashNoConflict(t *testing.T) {
	// B062b: empty local content hash should not trigger conflict check.
	local := schema.Issue{
		Summary: "New title",
		RemoteMetadata: schema.RemoteMetadata{
			RemoteVersion: "42",
			ContentHash:   "", // empty
		},
	}
	remote := schema.Issue{
		Summary: "Old title",
		RemoteMetadata: schema.RemoteMetadata{
			RemoteVersion: "42",
			ContentHash:   "abc123",
		},
	}

	ops, conflicts, err := BuildPlan(local, remote)
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}

	if len(ops) != 1 {
		t.Fatalf("expected 1 operation, got %d: %v", len(ops), ops)
	}

	if len(conflicts) != 0 {
		t.Errorf("expected 0 conflicts when local has no content hash, got %d: %v", len(conflicts), conflicts)
	}
}

func TestBuildPlan_emptyRemoteContentHashNoConflict(t *testing.T) {
	// B062b: empty remote content hash should not trigger conflict check.
	local := schema.Issue{
		Summary: "New title",
		RemoteMetadata: schema.RemoteMetadata{
			RemoteVersion: "42",
			ContentHash:   "abc123",
		},
	}
	remote := schema.Issue{
		Summary: "Old title",
		RemoteMetadata: schema.RemoteMetadata{
			RemoteVersion: "42",
			ContentHash:   "", // empty
		},
	}

	ops, conflicts, err := BuildPlan(local, remote)
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}

	if len(ops) != 1 {
		t.Fatalf("expected 1 operation, got %d: %v", len(ops), ops)
	}

	if len(conflicts) != 0 {
		t.Errorf("expected 0 conflicts when remote has no content hash, got %d: %v", len(conflicts), conflicts)
	}
}

func TestBuildPlan_allFieldsUnchanged(t *testing.T) {
	// B061b: all fields unchanged produce empty plan.
	localAssignee := "jdoe"
	local := schema.Issue{
		Summary:     "Test issue",
		Description: "Some description",
		Labels:      []string{"bug", "priority"},
		Assignee:    &localAssignee,
		Status:      "To Do",
		Sprint:      "Sprint 41",
		FixVersions: []string{"1.0"},
	}
	remoteAssignee := "jdoe"
	remote := schema.Issue{
		Summary:     "Test issue",
		Description: "Some description",
		Labels:      []string{"priority", "bug"},
		Assignee:    &remoteAssignee,
		Status:      "To Do",
		Sprint:      "Sprint 41",
		FixVersions: []string{"1.0"},
	}

	ops, conflicts, err := BuildPlan(local, remote)
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}

	if len(ops) != 0 {
		t.Errorf("expected 0 operations, got %d: %v", len(ops), ops)
	}

	if len(conflicts) != 0 {
		t.Errorf("expected 0 conflicts, got %d: %v", len(conflicts), conflicts)
	}
}
