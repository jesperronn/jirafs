package schema

import "testing"

func TestConflictTypeConstants(t *testing.T) {
	expected := []ConflictType{
		ConflictBothEdited,
		ConflictLocalDeleteRemoteEdit,
		ConflictRemoteDeleteLocalEdit,
		ConflictLocalAddRemoteEdit,
		ConflictArchivePathInvalid,
		ConflictUnresolvedRef,
		ConflictInvalidTransition,
	}
	for _, ct := range expected {
		if ct == "" {
			t.Errorf("conflict type should not be empty: %q", ct)
		}
	}
}

func TestIsValidConflictType(t *testing.T) {
	for _, ct := range ValidConflictTypes {
		if !IsValidConflictType(ct) {
			t.Errorf("expected %q to be valid", ct)
		}
	}
	if IsValidConflictType("unknown") {
		t.Error("unknown conflict type should be invalid")
	}
	if IsValidConflictType("") {
		t.Error("empty conflict type should be invalid")
	}
}

func TestConflict_IsZero(t *testing.T) {
	var zero Conflict
	if !zero.IsZero() {
		t.Error("zero Conflict should be IsZero")
	}

	nonZero := Conflict{
		Field: EditableFieldSummary,
		Type:  ConflictBothEdited,
	}
	if nonZero.IsZero() {
		t.Error("non-zero Conflict should not be IsZero")
	}
}

func TestConflict_IsZero_partial(t *testing.T) {
	partial := Conflict{Field: EditableFieldSummary, Type: ConflictBothEdited}
	if partial.IsZero() {
		t.Error("partial Conflict should not be IsZero")
	}

	partial2 := Conflict{Field: EditableFieldSummary, LocalValue: "x"}
	if partial2.IsZero() {
		t.Error("partial Conflict should not be IsZero")
	}

	partial3 := Conflict{Type: ConflictBothEdited, LocalValue: "x"}
	if partial3.IsZero() {
		t.Error("partial Conflict should not be IsZero")
	}

	partial4 := Conflict{Field: EditableFieldSummary, Type: ConflictBothEdited, LocalValue: "x"}
	if partial4.IsZero() {
		t.Error("partial Conflict should not be IsZero")
	}
}

func TestConflict_String(t *testing.T) {
	c := Conflict{
		Field:       EditableFieldSummary,
		Type:        ConflictBothEdited,
		LocalValue:  "local title",
		RemoteValue: "remote title",
	}
	want := "both_edited:summary:local title:remote title"
	got := c.String()
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestConflict_String_empty(t *testing.T) {
	c := Conflict{}
	want := ":::"
	got := c.String()
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestConflict_String_delete_type(t *testing.T) {
	c := Conflict{
		Field:       EditableFieldDescription,
		Type:        ConflictLocalDeleteRemoteEdit,
		LocalValue:  "",
		RemoteValue: "was here",
	}
	want := "local_delete_remote_edit:description::was here"
	got := c.String()
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestConflict_Equals(t *testing.T) {
	a := Conflict{Field: EditableFieldSummary, Type: ConflictBothEdited, LocalValue: "x", RemoteValue: "y"}
	b := Conflict{Field: EditableFieldSummary, Type: ConflictBothEdited, LocalValue: "x", RemoteValue: "y"}
	c := Conflict{Field: EditableFieldDescription, Type: ConflictBothEdited, LocalValue: "x", RemoteValue: "y"}
	d := Conflict{Field: EditableFieldSummary, Type: ConflictLocalDeleteRemoteEdit, LocalValue: "", RemoteValue: "y"}
	e := Conflict{Field: EditableFieldSummary, Type: ConflictBothEdited, LocalValue: "z", RemoteValue: "y"}
	f := Conflict{Field: EditableFieldSummary, Type: ConflictBothEdited, LocalValue: "x", RemoteValue: "w"}

	if !a.Equals(b) {
		t.Error("a and b should be equal")
	}
	if a.Equals(c) {
		t.Error("a and c should not be equal (different field)")
	}
	if a.Equals(d) {
		t.Error("a and d should not be equal (different type)")
	}
	if a.Equals(e) {
		t.Error("a and e should not be equal (different local value)")
	}
	if a.Equals(f) {
		t.Error("a and f should not be equal (different remote value)")
	}
}

func TestConflict_Equals_zero(t *testing.T) {
	var zero Conflict
	zero2 := Conflict{}
	if !zero.Equals(zero2) {
		t.Error("two zero Conflicts should be equal")
	}
}
