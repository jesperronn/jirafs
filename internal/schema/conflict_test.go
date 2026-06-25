package schema

import (
	"testing"
)

func TestConflict(t *testing.T) {
	// Test zero value
	c := Conflict{}
	if !c.IsZero() {
		t.Errorf("Expected zero conflict to be zero, got %v", c)
	}

	// Test non-zero value
	c = Conflict{
		Field:       "summary",
		Type:        ConflictBothEdited,
		LocalValue:  "local value",
		RemoteValue: "remote value",
	}
	if c.IsZero() {
		t.Errorf("Expected non-zero conflict to not be zero, got %v", c)
	}

	// Test String method
	expectedString := "both_edited:summary:local value:remote value"
	actualString := c.String()
	if actualString != expectedString {
		t.Errorf("Expected %s, got %s", expectedString, actualString)
	}

	// Test Equals method
	c2 := Conflict{
		Field:       "summary",
		Type:        ConflictBothEdited,
		LocalValue:  "local value",
		RemoteValue: "remote value",
	}
	if !c.Equals(c2) {
		t.Errorf("Expected conflicts to be equal, got %v and %v", c, c2)
	}

	// Test Equals with different values
	c3 := Conflict{
		Field:       "summary",
		Type:        ConflictBothEdited,
		LocalValue:  "different local value",
		RemoteValue: "remote value",
	}
	if c.Equals(c3) {
		t.Errorf("Expected conflicts to be different, got %v and %v", c, c3)
	}

	// Test IsValidConflictType
	if !IsValidConflictType(ConflictBothEdited) {
		t.Errorf("Expected ConflictBothEdited to be valid")
	}
	if !IsValidConflictType(ConflictLocalDeleteRemoteEdit) {
		t.Errorf("Expected ConflictLocalDeleteRemoteEdit to be valid")
	}
	if !IsValidConflictType(ConflictRemoteDeleteLocalEdit) {
		t.Errorf("Expected ConflictRemoteDeleteLocalEdit to be valid")
	}
	if !IsValidConflictType(ConflictLocalAddRemoteEdit) {
		t.Errorf("Expected ConflictLocalAddRemoteEdit to be valid")
	}
	if !IsValidConflictType(ConflictArchivePathInvalid) {
		t.Errorf("Expected ConflictArchivePathInvalid to be valid")
	}
	if !IsValidConflictType(ConflictUnresolvedRef) {
		t.Errorf("Expected ConflictUnresolvedRef to be valid")
	}
	if !IsValidConflictType(ConflictInvalidTransition) {
		t.Errorf("Expected ConflictInvalidTransition to be valid")
	}
	if IsValidConflictType("invalid_type") {
		t.Errorf("Expected invalid_type to not be valid")
	}
}