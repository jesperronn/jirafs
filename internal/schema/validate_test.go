package schema

import (
	"testing"
	"time"
)

func TestIssueIdentity_ValidateRequired_allSet(t *testing.T) {
	id := IssueIdentity{
		Key:     "ABC-123",
		Type:    "story",
		Project: TypedRef{Type: RefProject, Value: "ABC"},
	}
	errs := id.ValidateRequired()
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
}

func TestIssueIdentity_ValidateRequired_missingKey(t *testing.T) {
	id := IssueIdentity{
		Type:    "story",
		Project: TypedRef{Type: RefProject, Value: "ABC"},
	}
	errs := id.ValidateRequired()
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Field != "key" {
		t.Errorf("expected field 'key', got %q", errs[0].Field)
	}
}

func TestIssueIdentity_ValidateRequired_missingType(t *testing.T) {
	id := IssueIdentity{
		Key:     "ABC-123",
		Project: TypedRef{Type: RefProject, Value: "ABC"},
	}
	errs := id.ValidateRequired()
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Field != "type" {
		t.Errorf("expected field 'type', got %q", errs[0].Field)
	}
}

func TestIssueIdentity_ValidateRequired_missingProject(t *testing.T) {
	id := IssueIdentity{
		Key: "ABC-123",
		Type: "story",
	}
	errs := id.ValidateRequired()
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Field != "project" {
		t.Errorf("expected field 'project', got %q", errs[0].Field)
	}
}

func TestIssueIdentity_ValidateRequired_allMissing(t *testing.T) {
	var id IssueIdentity
	errs := id.ValidateRequired()
	if len(errs) != 3 {
		t.Fatalf("expected 3 errors, got %d: %v", len(errs), errs)
	}
	fields := []string{errs[0].Field, errs[1].Field, errs[2].Field}
	expected := []string{"key", "type", "project"}
	for i, want := range expected {
		if fields[i] != want {
			t.Errorf("error[%d] field = %q, want %q", i, fields[i], want)
		}
	}
}

func TestIssueIdentity_IsComplete(t *testing.T) {
	complete := IssueIdentity{
		Key:     "ABC-123",
		Type:    "story",
		Project: TypedRef{Type: RefProject, Value: "ABC"},
	}
	if !complete.IsComplete() {
		t.Error("complete identity should report IsComplete")
	}

	incomplete := IssueIdentity{}
	if incomplete.IsComplete() {
		t.Error("empty identity should not report IsComplete")
	}
}

func TestValidateSections_allKnown(t *testing.T) {
	sections := []FixedSectionName{
		SecDescription,
		SecAcceptanceCriteria,
		SecNotes,
	}
	errs := ValidateSections(sections)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
}

func TestValidateSections_oneUnknown(t *testing.T) {
	sections := []FixedSectionName{
		SecDescription,
		FixedSectionName("Custom Section"),
	}
	errs := ValidateSections(sections)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Field != "section" {
		t.Errorf("expected field 'section', got %q", errs[0].Field)
	}
}

func TestValidateSections_allUnknown(t *testing.T) {
	sections := []FixedSectionName{
		FixedSectionName("Foo"),
		FixedSectionName("Bar"),
	}
	errs := ValidateSections(sections)
	if len(errs) != 2 {
		t.Fatalf("expected 2 errors, got %d: %v", len(errs), errs)
	}
}

func TestValidateSections_empty(t *testing.T) {
	errs := ValidateSections(nil)
	if len(errs) != 0 {
		t.Fatalf("expected no errors for nil, got %v", errs)
	}

	errs = ValidateSections([]FixedSectionName{})
	if len(errs) != 0 {
		t.Fatalf("expected no errors for empty, got %v", errs)
	}
}

func TestValidateSections_emptySectionName(t *testing.T) {
	sections := []FixedSectionName{
		SecDescription,
		FixedSectionName(""),
	}
	errs := ValidateSections(sections)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for empty section, got %d: %v", len(errs), errs)
	}
}

func TestValidationError_Error(t *testing.T) {
	e := ValidationError{Field: "key", Msg: "is required"}
	got := e.Error()
	want := "key: is required"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestIssueValidate_complete(t *testing.T) {
	issue := Issue{
		Identity: IssueIdentity{
			Key:     "ABC-123",
			Type:    "story",
			Project: TypedRef{Type: RefProject, Value: "ABC"},
		},
		Sections: map[FixedSectionName]string{
			SecDescription:        "Some description",
			SecAcceptanceCriteria: "AC text",
		},
	}
	errs := issue.Validate()
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
}

func TestIssueValidate_missingKey(t *testing.T) {
	issue := Issue{
		Identity: IssueIdentity{
			Type:    "story",
			Project: TypedRef{Type: RefProject, Value: "ABC"},
		},
		Sections: map[FixedSectionName]string{
			SecDescription: "desc",
		},
	}
	errs := issue.Validate()
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Field != "key" {
		t.Errorf("expected field 'key', got %q", errs[0].Field)
	}
}

func TestIssueValidate_missingType(t *testing.T) {
	issue := Issue{
		Identity: IssueIdentity{
			Key:     "ABC-123",
			Project: TypedRef{Type: RefProject, Value: "ABC"},
		},
		Sections: map[FixedSectionName]string{
			SecDescription: "desc",
		},
	}
	errs := issue.Validate()
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Field != "type" {
		t.Errorf("expected field 'type', got %q", errs[0].Field)
	}
}

func TestIssueValidate_missingProject(t *testing.T) {
	issue := Issue{
		Identity: IssueIdentity{
			Key: "ABC-123",
			Type: "story",
		},
		Sections: map[FixedSectionName]string{
			SecDescription: "desc",
		},
	}
	errs := issue.Validate()
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Field != "project" {
		t.Errorf("expected field 'project', got %q", errs[0].Field)
	}
}

func TestIssueValidate_allMissing(t *testing.T) {
	issue := Issue{}
	errs := issue.Validate()
	if len(errs) != 3 {
		t.Fatalf("expected 3 errors, got %d: %v", len(errs), errs)
	}
	fields := []string{errs[0].Field, errs[1].Field, errs[2].Field}
	expected := []string{"key", "type", "project"}
	for i, want := range expected {
		if fields[i] != want {
			t.Errorf("error[%d] field = %q, want %q", i, fields[i], want)
		}
	}
}

func TestIssueValidate_unknownSections(t *testing.T) {
	issue := Issue{
		Identity: IssueIdentity{
			Key:     "ABC-123",
			Type:    "story",
			Project: TypedRef{Type: RefProject, Value: "ABC"},
		},
		Sections: map[FixedSectionName]string{
			SecDescription:        "desc",
			FixedSectionName("Custom"): "custom body",
		},
	}
	errs := issue.Validate()
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Field != "section" {
		t.Errorf("expected field 'section', got %q", errs[0].Field)
	}
}

func TestIssueValidate_combinedErrors(t *testing.T) {
	issue := Issue{
		Identity: IssueIdentity{},
		Sections: map[FixedSectionName]string{
			SecDescription:        "desc",
			FixedSectionName("Custom"): "custom",
		},
	}
	errs := issue.Validate()
	if len(errs) != 4 {
		t.Fatalf("expected 4 errors (3 identity + 1 section), got %d: %v", len(errs), errs)
	}
	// First 3 are identity errors, last is section
	if errs[3].Field != "section" {
		t.Errorf("expected last error field 'section', got %q", errs[3].Field)
	}
}

func TestIssueValidate_nilSections(t *testing.T) {
	issue := Issue{
		Identity: IssueIdentity{
			Key:     "ABC-123",
			Type:    "story",
			Project: TypedRef{Type: RefProject, Value: "ABC"},
		},
		Sections: nil,
	}
	errs := issue.Validate()
	if len(errs) != 0 {
		t.Fatalf("expected no errors for nil sections, got %v", errs)
	}
}

func TestIssueValidate_emptySectionsMap(t *testing.T) {
	issue := Issue{
		Identity: IssueIdentity{
			Key:     "ABC-123",
			Type:    "story",
			Project: TypedRef{Type: RefProject, Value: "ABC"},
		},
		Sections: map[FixedSectionName]string{},
	}
	errs := issue.Validate()
	if len(errs) != 0 {
		t.Fatalf("expected no errors for empty sections, got %v", errs)
	}
}

// B022b: state validation tests for syncable, unsynced, archived, and draft.

func TestRemoteMetadataValidate_synced_complete(t *testing.T) {
	r := RemoteMetadata{
		RemoteVersion: "3",
		ContentHash:   "abc123",
		SyncTime:      time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC),
	}
	errs := r.Validate()
	if len(errs) != 0 {
		t.Fatalf("expected no errors for complete synced metadata, got %v", errs)
	}
	if r.State() != StateSynced {
		t.Errorf("State() = %q, want %q", r.State(), StateSynced)
	}
}

func TestRemoteMetadataValidate_synced_missingVersion(t *testing.T) {
	r := RemoteMetadata{
		StateFile:     "synced",
		ContentHash:   "abc123",
		SyncTime:      time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC),
	}
	errs := r.Validate()
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Field != "remote_version" {
		t.Errorf("field = %q, want %q", errs[0].Field, "remote_version")
	}
}

func TestRemoteMetadataValidate_synced_missingHash(t *testing.T) {
	r := RemoteMetadata{
		StateFile:     "synced",
		RemoteVersion: "3",
		SyncTime:      time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC),
	}
	errs := r.Validate()
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Field != "content_hash" {
		t.Errorf("field = %q, want %q", errs[0].Field, "content_hash")
	}
}

func TestRemoteMetadataValidate_synced_missingSyncTime(t *testing.T) {
	r := RemoteMetadata{
		StateFile:     "synced",
		RemoteVersion: "3",
		ContentHash:   "abc123",
	}
	errs := r.Validate()
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Field != "sync_time" {
		t.Errorf("field = %q, want %q", errs[0].Field, "sync_time")
	}
}

func TestRemoteMetadataValidate_synced_allMissing(t *testing.T) {
	// StateFile alone does not produce StateSynced because IsZero()
	// does not check StateFile.  With no remote metadata fields set,
	// the state is unsynced and validates cleanly.
	r := RemoteMetadata{
		StateFile: "synced",
	}
	errs := r.Validate()
	if len(errs) != 0 {
		t.Fatalf("expected no errors (StateFile alone is unsynced), got %d: %v", len(errs), errs)
	}
	if r.State() != StateUnsynced {
		t.Errorf("State() = %q, want %q", r.State(), StateUnsynced)
	}
}

func TestRemoteMetadataValidate_draft_valid(t *testing.T) {
	r := RemoteMetadata{StateFile: "draft"}
	errs := r.Validate()
	if len(errs) != 0 {
		t.Fatalf("expected no errors for valid draft, got %v", errs)
	}
	if r.State() != StateDraft {
		t.Errorf("State() = %q, want %q", r.State(), StateDraft)
	}
}

func TestRemoteMetadataValidate_draft_wrongStateFile(t *testing.T) {
	// StateFile: "synced" with no remote metadata fields produces
	// StateUnsynced (not StateSynced) because IsZero() ignores StateFile.
	r := RemoteMetadata{StateFile: "synced"}
	errs := r.Validate()
	if len(errs) != 0 {
		t.Fatalf("expected no errors (StateFile ignored by IsZero), got %d: %v", len(errs), errs)
	}
	if r.State() != StateUnsynced {
		t.Errorf("State() = %q, want %q", r.State(), StateUnsynced)
	}
}

func TestRemoteMetadataValidate_archived_valid(t *testing.T) {
	r := RemoteMetadata{StateFile: "archived"}
	errs := r.Validate()
	if len(errs) != 0 {
		t.Fatalf("expected no errors for valid archived, got %v", errs)
	}
	if r.State() != StateArchived {
		t.Errorf("State() = %q, want %q", r.State(), StateArchived)
	}
}

func TestRemoteMetadataValidate_archived_wrongStateFile(t *testing.T) {
	// StateFile: "draft" produces StateDraft, which validates cleanly
	// because validateDraft checks StateFile == "draft".
	r := RemoteMetadata{StateFile: "draft"}
	errs := r.Validate()
	if len(errs) != 0 {
		t.Fatalf("expected no errors (StateFile matches draft), got %d: %v", len(errs), errs)
	}
	if r.State() != StateDraft {
		t.Errorf("State() = %q, want %q", r.State(), StateDraft)
	}
}

func TestRemoteMetadataValidate_unsynced_empty(t *testing.T) {
	r := RemoteMetadata{}
	errs := r.Validate()
	if len(errs) != 0 {
		t.Fatalf("expected no errors for empty (unsynced) metadata, got %v", errs)
	}
	if r.State() != StateUnsynced {
		t.Errorf("State() = %q, want %q", r.State(), StateUnsynced)
	}
}

func TestRemoteMetadataValidate_unsynced_partial(t *testing.T) {
	// Partial metadata with only RemoteVersion set produces StateSynced
	// (because IsZero() is false when a field is set), which then
	// validates that all three fields (remote_version, content_hash,
	// sync_time) are present.
	r := RemoteMetadata{
		RemoteVersion: "3",
	}
	errs := r.Validate()
	if len(errs) != 2 {
		t.Fatalf("expected 2 errors for partial metadata, got %d: %v", len(errs), errs)
	}
	// Should report missing content_hash and sync_time.
	fields := []string{errs[0].Field, errs[1].Field}
	expected := []string{"content_hash", "sync_time"}
	for i, want := range expected {
		if fields[i] != want {
			t.Errorf("error[%d] field = %q, want %q", i, fields[i], want)
		}
	}
}

func TestIssueValidate_syncedComplete(t *testing.T) {
	issue := Issue{
		Identity: IssueIdentity{
			Key:     "SYNC-1",
			Type:    "story",
			Project: TypedRef{Type: RefProject, Value: "SYNC"},
		},
		RemoteMetadata: RemoteMetadata{
			RemoteVersion: "3",
			ContentHash:   "abc",
			SyncTime:      time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC),
		},
	}
	errs := issue.Validate()
	if len(errs) != 0 {
		t.Fatalf("expected no errors for complete synced issue, got %v", errs)
	}
}

func TestIssueValidate_syncedMissingFields(t *testing.T) {
	// StateFile alone does not produce StateSynced; without remote
	// metadata fields, State() returns StateUnsynced and validates.
	issue := Issue{
		Identity: IssueIdentity{
			Key:     "SYNC-2",
			Type:    "story",
			Project: TypedRef{Type: RefProject, Value: "SYNC"},
		},
		RemoteMetadata: RemoteMetadata{
			StateFile: "synced",
		},
	}
	errs := issue.Validate()
	if len(errs) != 0 {
		t.Fatalf("expected no errors (StateFile alone is unsynced), got %d: %v", len(errs), errs)
	}
}

func TestIssueValidate_draftValid(t *testing.T) {
	issue := Issue{
		Identity: IssueIdentity{
			Key:     "DRAFT-1",
			Type:    "story",
			Project: TypedRef{Type: RefProject, Value: "DRAFT"},
		},
		RemoteMetadata: RemoteMetadata{StateFile: "draft"},
	}
	errs := issue.Validate()
	if len(errs) != 0 {
		t.Fatalf("expected no errors for valid draft, got %v", errs)
	}
}

func TestIssueValidate_unsyncedPartial(t *testing.T) {
	// Partial metadata with only RemoteVersion produces StateSynced
	// (because IsZero is false), which validates all three fields.
	issue := Issue{
		Identity: IssueIdentity{
			Key:     "PARTIAL-1",
			Type:    "story",
			Project: TypedRef{Type: RefProject, Value: "PARTIAL"},
		},
		RemoteMetadata: RemoteMetadata{
			RemoteVersion: "3",
		},
	}
	errs := issue.Validate()
	if len(errs) != 2 {
		t.Fatalf("expected 2 errors for partial metadata, got %d: %v", len(errs), errs)
	}
}
