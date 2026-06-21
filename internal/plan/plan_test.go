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
	// when summary and description are unchanged.
	localAssignee := "jdoe"
	remoteAssignee := "jsmith"
	local := schema.Issue{
		Summary:     "Test issue",
		Description: "Some description",
		Labels:      []string{"bug"},
		Assignee:    &localAssignee,
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
		Labels:      []string{"enhancement"},
		Assignee:    &remoteAssignee,
		LinkedIssues: []schema.LinkedIssue{
			{Key: "PROJ-2", Type: "relates to"},
		},
		RemoteMetadata: schema.RemoteMetadata{
			RemoteVersion: "2",
			ContentHash:   "def456",
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
