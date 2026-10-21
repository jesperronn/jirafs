package schema

import (
	"testing"
)

// B021a: tests for issue identity and machine-owned frontmatter fields.

func TestIssueIdentity_PartialValues(t *testing.T) {
	// Partial identity (only Key) should not be IsZero.
	partial := IssueIdentity{Key: "PROJ-42"}
	if partial.IsZero() {
		t.Error("partial IssueIdentity (key only) should not be IsZero")
	}

	// Partial identity (only Type) should not be IsZero.
	partialType := IssueIdentity{Type: "bug"}
	if partialType.IsZero() {
		t.Error("partial IssueIdentity (type only) should not be IsZero")
	}

	// Partial identity (only Project) should not be IsZero.
	partialProj := IssueIdentity{Project: TypedRef{Type: RefProject, Value: "PROJ"}}
	if partialProj.IsZero() {
		t.Error("partial IssueIdentity (project only) should not be IsZero")
	}
}

func TestMachineOwned_SchemaVersion(t *testing.T) {
	// Test that SchemaVersion is properly set and read.
	m := MachineOwned{SchemaVersion: "v1"}
	if m.SchemaVersion != "v1" {
		t.Errorf("SchemaVersion = %q, want %q", m.SchemaVersion, "v1")
	}
	if m.IsZero() {
		t.Error("MachineOwned with SchemaVersion should not be IsZero")
	}

	// Empty SchemaVersion should be IsZero.
	empty := MachineOwned{}
	if !empty.IsZero() {
		t.Error("MachineOwned with empty SchemaVersion should be IsZero")
	}
}

func TestLinkedIssue_IsZero(t *testing.T) {
	var zero LinkedIssue
	if !zero.IsZero() {
		t.Error("zero LinkedIssue should be IsZero")
	}

	nonZero := LinkedIssue{Key: "PROJ-1", Relation: "blocks"}
	if nonZero.IsZero() {
		t.Error("non-zero LinkedIssue should not be IsZero")
	}

	// Partial (only Key) should not be IsZero.
	partial := LinkedIssue{Key: "PROJ-1"}
	if partial.IsZero() {
		t.Error("partial LinkedIssue (key only) should not be IsZero")
	}
}

func TestLinkedIssue_Equals(t *testing.T) {
	a := LinkedIssue{Key: "PROJ-1", Relation: "blocks"}
	b := LinkedIssue{Key: "PROJ-1", Relation: "blocks"}
	c := LinkedIssue{Key: "PROJ-2", Relation: "blocks"}
	d := LinkedIssue{Key: "PROJ-1", Relation: "relates to"}

	if !a.Equals(b) {
		t.Error("equal LinkedIssues should be Equals")
	}
	if a.Equals(c) {
		t.Error("different key should not be Equals")
	}
	if a.Equals(d) {
		t.Error("different type should not be Equals")
	}
}

func TestPermissionCategoryConstants(t *testing.T) {
	// Verify PermissionCategory constants have expected string values.
	if PermissionEditable != "editable" {
		t.Errorf("PermissionEditable = %q, want %q", PermissionEditable, "editable")
	}
	if PermissionAppendOnly != "append_only" {
		t.Errorf("PermissionAppendOnly = %q, want %q", PermissionAppendOnly, "append_only")
	}
	if PermissionReadOnly != "read_only" {
		t.Errorf("PermissionReadOnly = %q, want %q", PermissionReadOnly, "read_only")
	}
}

func TestIssue_IsZero_identityOnly(t *testing.T) {
	// Issue with only identity set should not be IsZero.
	issue := Issue{
		Identity: IssueIdentity{
			Key:     "PROJ-42",
			Type:    "story",
			Project: TypedRef{Type: RefProject, Value: "PROJ"},
		},
	}
	if issue.IsZero() {
		t.Error("Issue with identity should not be IsZero")
	}
}

func TestIssue_IsZero_machineOwnedOnly(t *testing.T) {
	// Issue with only MachineOwned set should not be IsZero.
	issue := Issue{
		MachineOwned: MachineOwned{SchemaVersion: "1"},
	}
	if issue.IsZero() {
		t.Error("Issue with MachineOwned should not be IsZero")
	}
}

func TestIssue_IsZero_partialIdentity(t *testing.T) {
	// Issue with partial identity (only Key) should not be IsZero.
	issue := Issue{
		Identity: IssueIdentity{Key: "PROJ-42"},
	}
	if issue.IsZero() {
		t.Error("Issue with partial identity should not be IsZero")
	}
}

func TestIssue_IsZero_empty(t *testing.T) {
	var issue Issue
	if !issue.IsZero() {
		t.Error("empty Issue should be IsZero")
	}
}

func TestLinkedIssue_EqualsIgnoresReadOnlyContext(t *testing.T) {
	a := LinkedIssue{Key: "PROJ-1", Relation: "blocks", Summary: "x", Status: "Open"}
	b := LinkedIssue{Key: "PROJ-1", Relation: "blocks", Summary: "y", Status: "Done"}
	if !a.Equals(b) {
		t.Error("Equals must compare only key and relation")
	}
}
