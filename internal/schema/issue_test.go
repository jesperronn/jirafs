package schema

import "testing"

func TestIssueIdentity_IsZero(t *testing.T) {
	var zero IssueIdentity
	if !zero.IsZero() {
		t.Error("zero IssueIdentity should be IsZero")
	}

	nonZero := IssueIdentity{
		Key:     "ABC-123",
		Type:    "story",
		Project: TypedRef{Type: RefProject, Value: "ABC"},
	}
	if nonZero.IsZero() {
		t.Error("non-zero IssueIdentity should not be IsZero")
	}

	partial := IssueIdentity{Key: "ABC-123"}
	if partial.IsZero() {
		t.Error("partial IssueIdentity should not be IsZero")
	}
}

func TestMachineOwned_IsZero(t *testing.T) {
	var zero MachineOwned
	if !zero.IsZero() {
		t.Error("zero MachineOwned should be IsZero")
	}

	nonZero := MachineOwned{SchemaVersion: "1"}
	if nonZero.IsZero() {
		t.Error("non-zero MachineOwned should not be IsZero")
	}
}
